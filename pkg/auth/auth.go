package auth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/pbkdf2"
)

var (
	store   *sessions.CookieStore
	salt    []byte
	apiKeys []string
	prefix  = os.Getenv("PATH_PREFIX")
)

const (
	sessionName = "palace_auth"
)

func init() {
	setupCookiesOrDie()
	gob.Register(authToken{})
	gob.Register(time.Time{})
}

// UsersDB is the API surface required to validate username+password pairs.
// The passwords passed in will be SaltAndHash of the plaintext.
type UsersDB interface {
	ValidatePassword(user string, password []byte) error
}

type authToken struct {
	Username          string
	PasswordHash      []byte
	CreationTimestamp time.Time
}

func setupCookiesOrDie() {
	salt = MustDecodeBase64([]byte(os.Getenv("AUTH_SALT")))
	if len(salt) == 0 {
		panic(fmt.Errorf("AUTH_SALT must be non-empty"))
	}

	blockKey := MustDecodeBase64([]byte(os.Getenv("AUTH_BLOCK_KEY")))
	switch len(blockKey) {
	case 16, 24, 32:
		// OK.
	default:
		panic(fmt.Errorf("AUTH_BLOCK_KEY is invalid length %d", len(blockKey)))
	}

	hashKey := MustDecodeBase64([]byte(os.Getenv("AUTH_HASH_KEY")))
	if len(hashKey) == 0 {
		panic(fmt.Errorf("AUTH_HASH_KEY must be non-empty"))
	}

	store = sessions.NewCookieStore(hashKey, blockKey)
	maxAge := 400 * 24 * 60 * 60 // 400 days.
	store.Options = &sessions.Options{
		Path:     prefix, // Potential bug.
		MaxAge:   maxAge, // Does not take effect yet.
		Secure:   true,
		HttpOnly: false, // Allows JavaScript to read the cookie.
	}
	store.MaxAge(maxAge)

	for _, token := range strings.Split(os.Getenv("AUTH_API_KEYS"), ",") {
		apiKeys = append(apiKeys, token)
	}
}

func checkToken(db UsersDB, token authToken) error {
	// Validate that the password provided is valid.
	if err := db.ValidatePassword(token.Username, token.PasswordHash); err != nil {
		return err
	}

	// Verify the timestamp is not too old.
	if time.Now().Sub(token.CreationTimestamp) >= 30*24*time.Hour {
		return fmt.Errorf("token created at %s is expired", token.CreationTimestamp)
	}
	return nil
}

func checkAuth(db UsersDB, r *http.Request) error {
	session, err := store.Get(r, sessionName)
	if err != nil {
		return err
	}
	token, ok := session.Values["token"].(authToken)
	if !ok {
		return fmt.Errorf("no authentication")
	}
	return checkToken(db, token)
}

type jsonToken struct {
	Token string `json:"token"`
}

func checkAuthJSON(db UsersDB, r *http.Request) error {
	var packet jsonToken
	if err := json.NewDecoder(r.Body).Decode(&packet); err != nil {
		return fmt.Errorf("no JSON token: %v", err)
	}

	// Allow apiKeys.
	if slices.Contains(apiKeys, packet.Token) {
		return nil
	}

	// TODO: The remainder can be removed when all clients are using an api key.
	values := make(map[any]any)
	err := securecookie.DecodeMulti(sessionName, packet.Token, &values, store.Codecs...)
	if err != nil {
		return fmt.Errorf("failed to decode json token: %v", err)
	}

	token, ok := values["token"].(authToken)
	if !ok {
		return fmt.Errorf("no authentication")
	}
	return checkToken(db, token)
}

// OnlyAuthenticated decorates a handler to redirect to a login page if there is
// no auth data.
func OnlyAuthenticated(db UsersDB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// rc is used for auth, the original request will be given to the inner
		// handler.
		rc := r.Clone(context.Background())
		if authErr := checkAuth(db, rc); authErr != nil {

			// We'll try finding a JSON token, so we need a seperate copy of the
			// body for the inner handler.
			body, err := io.ReadAll(r.Body)
			if err != nil {
				noAuth(w, rc, authErr)
				return
			}
			rc.Body = io.NopCloser(bytes.NewBuffer(body))
			r.Body = io.NopCloser(bytes.NewBuffer(slices.Clone(body)))

			if jsonErr := checkAuthJSON(db, rc); jsonErr != nil {
				log.Infof("No JSON auth: %v", jsonErr)
				log.Infof("No cookie auth: %v", authErr)
				noAuth(w, rc, authErr)
				return
			}
		}

		// Refresh the auth token before calling the inner handler.
		if err := refreshToken(w, r); err != nil {
			log.Warnf("Failed to refresh auth token: %v", err)
		}

		// Allow the request.
		next.ServeHTTP(w, r)
	})
}

func noAuth(w http.ResponseWriter, r *http.Request, authErr error) {
	log.Errorf("%s %s: failed to auth user: %v", r.Method, r.URL.Path, authErr)
	session, err := store.Get(r, sessionName)
	if err == nil {
		session.AddFlash(fmt.Sprintf("Logged out: %v", authErr))
		session.Values["redirect"] = filepath.Join(prefix, r.URL.String())
		session.Save(r, w)
	} else {
		log.Warnf("failed to create session: %v", err)
	}
	http.Redirect(w, r, filepath.Join(prefix, "/login"), http.StatusFound) // This is easily a bug, it needs to know its prefix.
}

// PostLogin creates a handler to receive login form results (checking the
// form values "username" and "password") and grant the user an auth token.
func PostLogin(db UsersDB) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username := r.FormValue("username")
		password := r.FormValue("password")
		salted := SaltAndHash(password)
		// TODO: Hash that password with salt.
		if err := db.ValidatePassword(username, salted); err != nil {
			noAuth(w, r, err)
			return
		}

		session, err := store.Get(r, sessionName)
		if err != nil {
			log.Errorf("failed to create session: %v", err)
			http.Error(w, fmt.Sprintf("Error: failed to create session: %v", err), http.StatusInternalServerError)
			return
		}

		// User is authenticated, set up a token.
		session.Values["token"] = authToken{
			Username:          username,
			PasswordHash:      salted,
			CreationTimestamp: time.Now(),
		}
		if err := session.Save(r, w); err != nil {
			log.Errorf("User logged in but we failed to save their session: %v", err)
		}

		redirect := prefix // Prefix bug!!!
		if sessionRedirect, ok := session.Values["redirect"]; ok {
			redirect = sessionRedirect.(string)
		}
		http.Redirect(w, r, redirect, http.StatusFound) // Again, prefix bug.
	})
}

// GetLogin serves an HTTP login page.
func GetLogin(w http.ResponseWriter, r *http.Request) {
	var flashtext []string
	session, err := store.Get(r, sessionName)
	if err == nil {
		for _, f := range session.Flashes() {
			flashtext = append(flashtext, fmt.Sprintf("%s", f))
		}
		session.Save(r, w) // Clears the flashes.
	} else {
		log.Warnf("no session on login page: %v", err)
	}

	fmt.Fprintf(w, `<body>`)
	for _, f := range flashtext {
		fmt.Fprintf(w, `<p>%s</p>`, html.EscapeString(f))
	}
	fmt.Fprintf(w, `
	<form method="post">
	<input type="text" placeholder="username" name="username" required>
	<input type="password" placeholder="password" name="password" required>
	<button type="submit">Login</button>
	</form>
</body>
	`)
}

// SaltAndHash hashes the password with a salt and dervies a key from it.
func SaltAndHash(password string) []byte {
	return pbkdf2.Key([]byte(password), salt, 4096, 32, sha1.New)
}

// MustDecodeBase64 decodes the URL-encoded base 64 or panics.
func MustDecodeBase64(in []byte) []byte {
	b := bytes.NewBuffer(in)
	dec := base64.NewDecoder(base64.URLEncoding, b)
	output, err := io.ReadAll(dec)
	if err != nil {
		panic(err)
	}
	return output
}

func refreshToken(w http.ResponseWriter, r *http.Request) error {
	if r.Method != "GET" {
		// We don't want to attempt to refresh the extension/javascript,
		// just the user browsing. We can proxy that by filtering just GET
		// requests.
		return nil
	}
	session, err := store.Get(r, sessionName)
	if err != nil {
		return err
	}
	token, ok := session.Values["token"].(authToken)
	if !ok {
		return fmt.Errorf("no authentication")
	}

	token.CreationTimestamp = time.Now()
	session.Values["token"] = token
	if err := session.Save(r, w); err != nil {
		return fmt.Errorf("save refreshed token: %w", err)
	}
	return nil
}
