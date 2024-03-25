package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
)

//go:embed static
var staticContent embed.FS
var prefix = os.Getenv("PATH_PREFIX")

type PostPageRequest struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	TextContent string `json:"text"`
}

// See https://web.dev/articles/cross-origin-resource-sharing?utm_source=devtools#preflight-requests.
func scrapePageOptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "https://icebox.spencerjp.dev")
	w.Header().Set("Access-Control-Allow-Methods", "POST")
	w.WriteHeader(http.StatusOK)
}

func scrapePage(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var content PostPageRequest
	if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		log.Infof("POST /pages: Failed to decode JSON: %v", err)
		return
	}
	if content.URL == "" || content.Title == "" || content.TextContent == "" {
		http.Error(w, "Incomplete request", http.StatusBadRequest)
		log.Infof("POST /pages: Incomplete request. URL=%t, title=%t, content=%t",
			content.URL == "", content.Title == "", content.TextContent == "")
		return
	}

	location, err := url.Parse(content.URL)
	if err != nil {
		http.Error(w, "Bad URL", http.StatusBadRequest)
		log.Infof("POST /pages: Bad URL %q", content.URL)
		return
	}
	minLocation := url.URL{
		Scheme: location.Scheme,
		Host:   location.Host,
		Path:   location.Path,
		// Some websites use the query to distinguish pages.
		// E.g. Hacker News uses /items?id=X for posts.
		RawQuery: location.RawQuery,
	}

	col := DataColumn{
		ScrapedAt:   time.Now(),
		URL:         minLocation.String(),
		SafeTitle:   html.EscapeString(content.Title),
		SafeContent: html.EscapeString(content.TextContent),
	}

	id, err := db.Save(col)
	if err != nil {
		http.Error(w, "Failed to commit data", http.StatusInternalServerError)
		log.Infof("POST /pages: Failed to %q save in DB: %v", col.URL, err)
		return
	}

	log.Infof("Scraped %d: %s", id, col.URL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `{"id":%d}`, id) // Now that's fast JSON.
}

func makeSearch() func(w http.ResponseWriter, r *http.Request) {
	searchTemplate := template.Must(template.ParseFS(staticContent, "static/search.template.html"))
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.FormValue("q")
		page := 0
		if parsed, err := strconv.Atoi(r.FormValue("page")); err == nil {
			page = parsed
		}

		var results []SearchResult
		if query != "" {
			var err error
			results, err = db.Search(query, page)
			if err != nil {
				http.Error(w, "Failed to query database", http.StatusInternalServerError)
				log.Infof("search: failed to query for %q: %v", query, err)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		if err := searchTemplate.Execute(w, map[string]any{
			"PageNum":    page,
			"NextPage":   withPage(prefix, r.URL, +1),
			"PrevPage":   withPage(prefix, r.URL, -1),
			"Query":      query,
			"NumResults": len(results),
			"Results":    results,
		}); err != nil {
			log.Errorf("failed to render search: %v", err)
		}
	}
}

func withPage(prefix string, in *url.URL, diff int) string {
	u := &url.URL{}
	*u = *in
	vals := u.Query()
	page := 0
	if parsed, err := strconv.Atoi(vals.Get("page")); err == nil {
		page = parsed
	}
	page += diff
	if page < 0 {
		return ""
	}
	vals.Set("page", fmt.Sprintf("%d", page))
	u.RawQuery = vals.Encode()
	u.Path = filepath.Join(prefix, u.Path)
	return u.String()
}
