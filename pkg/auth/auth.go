package auth

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/gob"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/charmbracelet/log"
	"github.com/gorilla/sessions"
	"golang.org/x/crypto/pbkdf2"
)

var (
	store  *sessions.CookieStore
	salt   []byte
	prefix = os.Getenv("PATH_PREFIX")
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

// OnlyAuthenticated decorates a handler to redirect to a login page if there is
// no auth data.
func OnlyAuthenticated(db UsersDB, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rc := r.Clone(context.Background())
		if err := checkAuth(db, r); err != nil {
			noAuth(w, r, err)
			return
		}

		// TODO: Will this work for the web extension?
		// TODO: Refresh the token before calling the inner handler?

		// Allow the request.
		next.ServeHTTP(w, rc)
	})
}

func noAuth(w http.ResponseWriter, r *http.Request, authErr error) {
	log.Errorf("%s %s: failed to auth user: %v", r.Method, r.URL.Path, authErr)
	session, err := store.Get(r, sessionName)
	if err == nil {
		session.AddFlash(fmt.Sprintf("Logged out: %v", authErr))
		session.Values["redirect"] = r.URL.String()
		session.Save(r, w)
	} else {
		log.Warnf("failed to create session: %v", err)
	}
	http.Redirect(w, r, prefix+"/login", http.StatusFound) // This is easily a bug, it needs to know its prefix.
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
		flashes := session.Flashes()
		log.Infof("flashes: %v", flashes)
		for _, f := range flashes {
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
	<input type="text" placeholder="password" name="password" required>
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
