package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spencer-p/palace/pkg/auth"
)

var db DB

func main() {
	if len(os.Args) > 1 {
		fmt.Printf("MY_PASSWORD=%s\n", b64(auth.SaltAndHash(os.Args[1])))
	}

	var err error
	db, err = NewDB(os.Getenv("DB_FILE"))
	if err != nil {
		log.Fatalf("Prepare database: %v", err)
	}

	usersDB := fakeUsersDB{}
	authhandle := func(path string, f func(w http.ResponseWriter, r *http.Request)) {
		http.Handle(path, auth.OnlyAuthenticated(usersDB, http.HandlerFunc(f)))
	}

	http.HandleFunc("GET /login", auth.GetLogin)
	http.Handle("POST /login", auth.PostLogin(usersDB))

	http.Handle("/{$}", http.RedirectHandler(filepath.Join(os.Getenv("PATH_PREFIX"), "/search"), http.StatusFound))
	authhandle("/search", search)

	authhandle("POST /pages", scrapePage)
	authhandle("GET /pages/{id}", notImpl)
	authhandle("DELETE /pages/{id}", notImpl)

	log.Errorf("listen and serve: %v", http.ListenAndServe(":6844", nil))
}

func notImpl(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

func b64(in []byte) []byte {
	var b bytes.Buffer
	enc := base64.NewEncoder(base64.URLEncoding, &b)
	if _, err := enc.Write(in); err != nil {
		panic(err)
	}
	if err := enc.Close(); err != nil {
		panic(err)
	}
	return b.Bytes()
}
