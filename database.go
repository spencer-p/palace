package main

import (
	"database/sql"
	_ "embed"
	"fmt"
	"html/template"
	"time"

	"github.com/charmbracelet/log"
	_ "modernc.org/sqlite"
)

type DataColumn struct {
	ScrapedAt   time.Time
	URL         string
	SafeTitle   string
	SafeContent string
}

type SearchResult struct {
	DataColumn
	ID        int
	SafeBlurb template.HTML
}

type Database interface {
	Save(DataColumn) (int, error)
	Fetch(int) (DataColumn, error)
	Search(string) ([]SearchResult, error)
}

type DB struct {
	*sql.DB
}

//go:embed init_db.sql
var initDB string

func NewDB(filename string) (DB, error) {
	db, err := sql.Open("sqlite", filename)
	if err != nil {
		return DB{}, fmt.Errorf("failed to open %q: %v", filename, err)
	}

	_, err = db.Exec(initDB)
	if err != nil {
		db.Close()
		return DB{}, fmt.Errorf("failed to run database init script: %v", err)
	}

	return DB{db}, nil
}

const ISO8601 = "2006-01-02 15:04:05.000"

func (db *DB) Save(col DataColumn) (int64, error) {
	res, err := db.Exec(`INSERT INTO web_data(url, scraped_at, title, content) VALUES (?, ?, ?, ?) RETURNING id`,
		col.URL,
		col.ScrapedAt.Format(ISO8601),
		col.SafeTitle,
		col.SafeContent,
	)
	if err != nil {
		return 0, err
	}

	err = db.Evict(col.URL)
	if err != nil {
		log.Warnf("failed to evict old entries for %q: %v", col.URL, err)
	}

	return res.LastInsertId()
}

func (db *DB) evictID(url string) (int64, bool, error) {
	var id int64
	rows, err := db.Query(`SELECT id FROM web_data WHERE url = ? ORDER BY id DESC LIMIT 5`, url)
	if err != nil {
		return id, false, fmt.Errorf("failed to query old ids: %v", err)
	}
	defer rows.Close()

	total := 0
	for rows.Next() {
		if err := rows.Scan(&id); err != nil {
			return id, false, fmt.Errorf("failed to scan col: %v", err)
		}
		total++
	}
	if total < 5 {
		return id, false, nil
	}

	return id, true, nil
}

func (db *DB) Evict(url string) error {
	id, ok, err := db.evictID(url)
	if err != nil || !ok {
		return err
	}

	log.Infof("Dropping %q items below id %d", url, id)

	for i := 0; i < 5; i++ {
		if _, err = db.Exec(`DELETE FROM web_data WHERE id < ?`, id); err == nil {
			return nil
		}
		log.Warnf("eviction for %q failed, will retry: %v", url, err)
		time.Sleep(time.Duration(i) * 5 * time.Millisecond)
	}
	return fmt.Errorf("failed to delete: %v", err)
}

func (db *DB) Search(query string) ([]SearchResult, error) {
	rows, err := db.Query(`
	SELECT
		id, url, scraped_at, search_index.title, search_index.content,
		snippet(search_index, 0, '<b>', '</b>', '...', 16)
	FROM web_data
	INNER JOIN search_index ON web_data.id = search_index.rowid
	WHERE search_index MATCH ?
	ORDER BY rank`,
		query,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var scrapeTime string
		if err := rows.Scan(&r.ID, &r.URL, &scrapeTime, &r.SafeTitle, &r.SafeContent, &r.SafeBlurb); err != nil {
			return nil, fmt.Errorf("column %d: scan: %w", len(results), err)
		}
		t, err := time.Parse(ISO8601, scrapeTime)
		if err != nil {
			return nil, fmt.Errorf("column %d: parse scrape time: %w", len(results), err)
		}
		r.ScrapedAt = t
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}
