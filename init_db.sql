CREATE TABLE IF NOT EXISTS web_data
	( id INTEGER PRIMARY KEY AUTOINCREMENT
	, url TEXT NOT NULL
	, scraped_at TIME NOT NULL
	, content TEXT NOT NULL
	, title TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS web_data_uniq ON web_data(content, title, url);

CREATE VIRTUAL TABLE IF NOT EXISTS search_index USING fts5
	( content = 'web_data'
	, content_rowid = 'id'
	, tokenize = 'porter unicode61'
	, content
	, title
);

CREATE TRIGGER IF NOT EXISTS wd_ai AFTER INSERT ON web_data BEGIN
	INSERT INTO search_index(rowid, content, title) VALUES (new.id, new.content, new.title);
	-- This would be a good place to delete extra stuff.
END;
CREATE TRIGGER IF NOT EXISTS wd_ad AFTER DELETE ON web_data BEGIN
	INSERT INTO search_index(search_index, rowid, content, title) VALUES('delete', old.id, old.content, old.title);
END;

-- https://kerkour.com/sqlite-for-servers
PRAGMA journal_mode = WAL;
PRAGMA busy_timeout = 30000; -- 30s.
PRAGMA synchronous = NORMAL;
PRAGMA cache_size = 1000000000; -- 1e9 pages.
PRAGMA foreign_keys = true;
PRAGMA temp_store = memory;
