package main

import (
	"log"
	"net/http"
)

var db DB

func main() {
	var err error
	db, err = NewDB()
	if err != nil {
		log.Fatalf("Prepare database: %v", err)
	}

	http.Handle("/{$}", http.RedirectHandler("/search", http.StatusFound))
	http.HandleFunc("/search", search)

	http.HandleFunc("POST /pages", scrapePage)
	http.HandleFunc("GET /pages/{id}", notImpl)
	http.HandleFunc("DELETE /pages/{id}", notImpl)

	log.Println(http.ListenAndServe(":6844", nil))
}

func notImpl(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "Not Implemented", http.StatusNotImplemented)
}
