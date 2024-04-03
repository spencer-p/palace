package main

import (
	"net/http"
	"os"
	"path/filepath"
	"time"

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

	mux := http.NewServeMux()
	usersDB := fakeUsersDB{}
	authhandle := func(path string, f func(w http.ResponseWriter, r *http.Request)) {
		mux.Handle(path, auth.OnlyAuthenticated(usersDB, http.HandlerFunc(f)))
	}

	mux.HandleFunc("GET /login", auth.GetLogin)
	mux.Handle("POST /login", auth.PostLogin(usersDB))

	mux.Handle("/{$}", http.RedirectHandler(filepath.Join(os.Getenv("PATH_PREFIX"), "/search"), http.StatusFound))
	authhandle("/search", makeSearch())

	mux.HandleFunc("OPTIONS /pages", scrapePageOptions)
	authhandle("POST /pages", scrapePage)
	authhandle("GET /pages/{id}", notImpl)
	authhandle("DELETE /pages/{id}", notImpl)

	http.Handle("GET /static/", http.FileServer(http.FS(staticContent)))

	log.Infof("Starting server")
	log.Errorf("listen and serve: %v", http.ListenAndServe(":6844", logWrap(mux)))
}

func notImpl(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}

func logWrap(next http.Handler) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tstart := time.Now()
		method, path := r.Method, r.URL
		next.ServeHTTP(w, r)
		log.Info("Served HTTP", "method", method, "path", path, "useragent", r.Header.Get("User-Agent"), "latency", time.Now().Sub(tstart))
	})
}
