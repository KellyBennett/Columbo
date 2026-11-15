package feedbackdb

import (
	"context"
	"database/sql"
	_ "embed"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 1

const SQLiteApplicationID = 'C'<<24 | 'L'<<16 | 'M'<<8 | 'F'

//go:embed schema.sql
var sqliteSchema string

type Store struct {
	db      *sql.DB
	queries Querier
}

func Open(path string) (*Store, error) {
	db, err := openDatabase(path)
	if err != nil {
		return nil, err
	}
	if err = prepare(db, path); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, queries: New(db)}, nil
}

func openDatabase(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(path), RawQuery: url.Values{"_pragma": {"foreign_keys(1)"}}.Encode()}
	db, err := sql.Open("sqlite", uri.String())
	if err == nil {
		db.SetMaxOpenConns(1)
	}
	return db, err
}

func prepare(db *sql.DB, path string) error {
	var applicationID, userVersion, objects int
	if err := db.QueryRow("SELECT (SELECT application_id FROM pragma_application_id), (SELECT user_version FROM pragma_user_version), (SELECT count(*) FROM sqlite_schema)").Scan(&applicationID, &userVersion, &objects); err != nil {
		return err
	}
	switch {
	case applicationID == 0 && userVersion == 0 && objects == 0:
		return create(db)
	case applicationID != SQLiteApplicationID:
		return fmt.Errorf("%s is not a Columbo feedback store", path)
	case userVersion != SchemaVersion:
		return fmt.Errorf("%s has feedback store schema version %d; this build reads version %d", path, userVersion, SchemaVersion)
	}
	return nil
}

func create(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(sqliteSchema); err != nil {
		return err
	}
	if _, err = tx.Exec(fmt.Sprintf("PRAGMA application_id=%d; PRAGMA user_version=%d", SQLiteApplicationID, SchemaVersion)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Queries() Querier { return s.queries }

func (s *Store) Update(ctx context.Context, fn func(Querier) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(New(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }
