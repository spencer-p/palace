package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"html/template"
	"time"

	"github.com/charmbracelet/log"
	"github.com/spencer-p/palace/pkg/backoff"
	"github.com/spencer-p/palace/pkg/prettytime"
	"modernc.org/sqlite"
	_ "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type DataColumn struct {
	ScrapedAt   time.Time
	URL         string
	SafeTitle   template.HTML
	SafeContent template.HTML
}

type SearchResult struct {
	DataColumn
	ID         int
	SafeBlurb  template.HTML
	ScrapedAgo string
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

const (
	ISO8601   = "2006-01-02 15:04:05.000"
	ISO8601TZ = "2006-01-02 15:04:05.000-07:00"
)

func retryBusy(err error) bool {
	sqliteErr := &sqlite.Error{}
	return errors.As(err, &sqliteErr) && sqliteErr.Code() == sqlite3.SQLITE_BUSY
}

func (db *DB) Save(col DataColumn) (int64, error) {
	var res sql.Result
	if err := backoff.Retry(5, retryBusy, func() error {
		var err error
		res, err = db.Exec(`INSERT INTO web_data(url, scraped_at, title, content) VALUES (?, ?, ?, ?) RETURNING id`,
			col.URL,
			col.ScrapedAt.Format(ISO8601TZ),
			col.SafeTitle,
			col.SafeContent,
		)
		return err
	}); err != nil {
		return 0, err
	}

	err := db.Evict(col.URL)
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

	result, deleteErr := db.Exec(`DELETE FROM web_data WHERE url = ? AND id < ?`, url, id)
	if affected, err := result.RowsAffected(); err == nil {
		log.Infof("Dropped %d rows for %q", affected, url)
	}
	if deleteErr != nil {
		return fmt.Errorf("failed to delete: %v", err)
	}
	return nil
}

func (db *DB) Search(query string, page int) ([]SearchResult, error) {
	rows, err := db.Query(`
	SELECT
		id, url, scraped_at, search_index.title, search_index.content,
		snippet(search_index, 0, '<b>', '</b>', '...', 40)
	FROM web_data
	INNER JOIN search_index ON web_data.id = search_index.rowid
	WHERE search_index MATCH ?
	ORDER BY rank
	LIMIT 50 OFFSET ?`,
		query, page*50,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	now := time.Now()
	var results []SearchResult
	for rows.Next() {
		var r SearchResult
		var scrapeTime string
		if err := rows.Scan(&r.ID, &r.URL, &scrapeTime, &r.SafeTitle, &r.SafeContent, &r.SafeBlurb); err != nil {
			return nil, fmt.Errorf("column %d: scan: %w", len(results), err)
		}
		t, err := timeFromDB(scrapeTime)
		if err != nil {
			return nil, fmt.Errorf("column %d: %w", len(results), err)
		}
		r.ScrapedAt = t
		r.ScrapedAgo = prettytime.DurationBetween(now, t)
		results = append(results, r)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (db *DB) Fetch(id int64) (SearchResult, error) {
	rows, err := db.Query(`
	SELECT
		url, scraped_at, title, content
	FROM web_data
	WHERE id = ?`,
		id,
	)
	if err != nil {
		return SearchResult{}, err
	}
	defer rows.Close()

	now := time.Now()
	rows.Next()
	var r SearchResult
	var scrapeTime string
	if err := rows.Scan(&r.URL, &scrapeTime, &r.SafeTitle, &r.SafeContent); err != nil {
		return r, fmt.Errorf("scan: %w", err)
	}
	t, err := timeFromDB(scrapeTime)
	if err != nil {
		return r, err
	}
	r.ScrapedAt = t
	r.ScrapedAgo = prettytime.DurationBetween(now, t)

	if err := rows.Err(); err != nil {
		return r, err
	}
	return r, nil
}

func timeFromDB(tstring string) (time.Time, error) {
	t, err := time.Parse(ISO8601TZ, tstring)
	if err != nil {
		if t, err = time.Parse(ISO8601, tstring); err != nil {
			return time.Time{}, fmt.Errorf("parse time %v: %w", tstring, err)
		}
	}
	return t.Local(), nil
}
