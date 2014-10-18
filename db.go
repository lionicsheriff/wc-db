package main

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
	"time"
)

var schema = []string{
	// v1-v3
	// needs to be split into separate statements due to https://github.com/mattn/go-sqlite3/issues/60
	// otherwise only the first statement will execute
	`
CREATE TABLE config (
    key varchar PRIMARY KEY,
    value varchar
);
`,
	`
CREATE TABLE documents (
    id integer PRIMARY KEY,
    path string
    );

`,
	`
CREATE TABLE word_count (
    id integer PRIMARY KEY,
    path string,
    words integer,
    timestamp integer,
    UNIQUE (path, timestamp)
);
`,
}

type SchemaVersionNotFound struct{}

func (s SchemaVersionNotFound) Error() string {
	return "no schema version found"
}

func openDb(dsn string) (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", dsn)
	return
}

func getSchemaVersion(db *sql.DB) (version int, err error) {

	row := db.QueryRow("pragma schema_version")

	var schema_version string
	err = row.Scan(&schema_version)
	if err != nil {
		err = SchemaVersionNotFound{}
		return
	}

	version, err = strconv.Atoi(schema_version)
	return
}

func updateSchema(db *sql.DB) (err error) {
	version, err := getSchemaVersion(db)
	if err != nil {
		return
	}
	if version < len(schema) {
		tx, err := db.Begin()
		for _, cmd := range schema[version:] {
			_, err = db.Exec(cmd)
			if err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		err = tx.Commit()
	}

	if err == nil {
		_, err = db.Exec("pragma schema_version = " + strconv.Itoa(len(schema)))
	}
	return
}

func getPreviousDayWordCount(db *sql.DB, path string) (count int, err error) {
	cmd := `
SELECT words
FROM word_count
WHERE path = ?
AND timestamp < ?
ORDER BY timestamp DESC
LIMIT 1,1
`

	y, m, d := time.Now().UTC().Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	row := db.QueryRow(cmd, path, today)
	var wordsColumn string
	err = row.Scan(&wordsColumn)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return
	}
	if wordsColumn == "" {
		return 0, nil
	} else {
		count, err = strconv.Atoi(wordsColumn)
		return
	}
}

func addWordCount(db *sql.DB, path string, count int) (err error) {
	cmd := `
INSERT INTO word_count (path, words, timestamp)
VALUES (?, ?, ?)
`

	time := time.Now().UTC().Unix()
	_, err = db.Exec(cmd, path, count, time)
	return
}

func getPreviousWordCount(db *sql.DB, path string) (count int, err error) {
	cmd := `
SELECT words
FROM word_count
WHERE path = ?
ORDER BY timestamp DESC
LIMIT 1,1
`
	row := db.QueryRow(cmd, path)
	var wordsColumn string
	err = row.Scan(&wordsColumn)
	if err == sql.ErrNoRows {
		return 0, nil
	} else if err != nil {
		return
	}
	if wordsColumn == "" {
		return 0, nil
	} else {
		count, err = strconv.Atoi(wordsColumn)
		return
	}
}
