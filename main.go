package main

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/charmbracelet/log"
	"github.com/spencer-p/palace/pkg/auth"
)

var db DB

func main() {
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
	authhandle("/search", makeSearch())

	http.HandleFunc("OPTIONS /pages", scrapePageOptions)
	authhandle("POST /pages", scrapePage)
	authhandle("GET /pages/{id}", notImpl)
	authhandle("DELETE /pages/{id}", notImpl)

	http.Handle("GET /static/", http.FileServer(http.FS(staticContent)))

	log.Infof("Starting server")
	log.Errorf("listen and serve: %v", http.ListenAndServe(":6844", nil))
}

func notImpl(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}
