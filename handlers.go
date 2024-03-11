package main

import (
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"time"
)

type PostPageRequest struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	TextContent string `json:"text"`
}

func scrapePage(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var content PostPageRequest
	if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.Printf("POST /pages: Failed to decode JSON: %v", err)
		return
	}
	if content.URL == "" || content.Title == "" || content.TextContent == "" {
		http.Error(w, "Incomplete request", http.StatusBadRequest)
		log.Printf("POST /pages: Incomplete request")
		return
	}

	col := DataColumn{
		ScrapedAt:   time.Now(),
		URL:         content.URL,
		SafeTitle:   html.EscapeString(content.TextContent),
		SafeContent: html.EscapeString(content.TextContent),
	}

	id, err := db.Save(col)
	if err != nil {
		http.Error(w, "Failed to commit data", http.StatusInternalServerError)
		log.Printf("POST /pages: Failed to save in DB: %v", err)
		return
	}
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"id":%d}`, id) // Now that's fast JSON.
}

func search(w http.ResponseWriter, r *http.Request) {
	query := r.FormValue("q")
	if query == "" {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
		return
	}

	results, err := db.Search(query)
	if err != nil {
		http.Error(w, "Failed to query database", http.StatusInternalServerError)
		log.Printf("search: failed to query for %q: %v", query, err)
		return
	}

	w.WriteHeader(http.StatusOK)
	for _, r := range results {
		fmt.Fprintf(w, "%+v\n", r)
	}
}
