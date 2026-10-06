package snapshotdb

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

const SchemaVersion = 4
const SQLiteApplicationID = 0x434c4d42

//go:embed schema.sql
var sqliteSchema string

type Snapshot struct {
	db      *sql.DB
	queries Querier
}

func Open(path string) (*Snapshot, error) {
	if err := validateSnapshotFile(path); err != nil {
		return nil, err
	}
	db, err := openSnapshotDatabase(path, true)
	if err != nil {
		return nil, err
	}
	if err = validateSnapshot(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Snapshot{db: db, queries: New(db)}, nil
}
func (s *Snapshot) Queries() Querier { return s.queries }
func (s *Snapshot) Close() error     { return s.db.Close() }

type Writer struct {
	db      *sql.DB
	tx      *sql.Tx
	queries Querier
}

func Create(path string) (*Writer, error) {
	db, err := openSnapshotDatabase(path, false)
	if err != nil {
		return nil, err
	}
	writer := &Writer{db: db}
	if err = writer.begin(); err != nil {
		writer.Close()
		return nil, err
	}
	return writer, nil
}
func (w *Writer) begin() error {
	if _, err := w.db.Exec("PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL"); err != nil {
		return err
	}
	tx, err := w.db.Begin()
	if err != nil {
		return err
	}
	w.tx = tx
	w.bindQueries()
	return createSnapshotSchema(tx)
}
func (w *Writer) Queries() Querier { return w.queries }
func (w *Writer) Complete() error {
	if err := validateSnapshotReport(w.queries); err != nil {
		return err
	}
	if err := validateSnapshotData(w.tx); err != nil {
		return err
	}
	return w.tx.Commit()
}
func (w *Writer) Close() error {
	if w.tx != nil {
		_ = w.tx.Rollback()
	}
	return w.db.Close()
}

func snapshotDatabaseURI(path string, readonly bool) string {
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	query := url.Values{"_pragma": {"foreign_keys(1)"}}
	if readonly {
		query.Set("mode", "ro")
		query.Set("immutable", "1")
		query.Add("_pragma", "query_only(1)")
	}
	uri.RawQuery = query.Encode()
	return uri.String()
}
func openSnapshotDatabase(path string, readonly bool) (*sql.DB, error) {
	db, err := sql.Open("sqlite", snapshotDatabaseURI(path, readonly))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func createSnapshotSchema(db snapshotQuery) error {
	_, err := db.Exec(sqliteSchema)
	if err != nil {
		return err
	}
	_, err = db.Exec(fmt.Sprintf("PRAGMA application_id=%d; PRAGMA user_version=%d", SQLiteApplicationID, SchemaVersion))
	return err
}

type snapshotQuery interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func validateSnapshot(db *sql.DB) error {
	if err := validateSnapshotIdentity(db); err != nil {
		return err
	}
	if err := validateSnapshotSchema(db); err != nil {
		return err
	}
	return validateSnapshotData(db)
}
func validateSnapshotIdentity(db *sql.DB) error {
	var applicationID, userVersion int
	if err := db.QueryRow("PRAGMA application_id").Scan(&applicationID); err != nil {
		return err
	}
	if applicationID != SQLiteApplicationID {
		return fmt.Errorf("not a Columbo SQLite snapshot")
	}
	if err := db.QueryRow("PRAGMA user_version").Scan(&userVersion); err != nil {
		return err
	}
	if userVersion != SchemaVersion {
		return fmt.Errorf("unsupported snapshot schema version %d", userVersion)
	}
	return validateSnapshotReport(snapshotQueries(db))
}
func validateSnapshotReport(queries Querier) error {
	identity, err := queries.ReportIdentity(context.Background())
	if err != nil {
		return err
	}
	if identity.ReportCount != 1 || identity.SchemaVersion != SchemaVersion {
		return fmt.Errorf("snapshot report identity does not match schema version")
	}
	return nil
}
func validateSnapshotData(db snapshotQuery) error {
	check := snapshotValidation{db: db}
	if err := check.integrity(); err != nil {
		return err
	}
	return check.foreignKeys()
}

type snapshotValidation struct{ db snapshotQuery }

func (v *snapshotValidation) integrity() error {
	rows, err := v.db.Query("PRAGMA integrity_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	result := snapshotIntegrity{}
	for rows.Next() {
		result.accept(rows)
	}
	if err = rows.Err(); err != nil {
		return err
	}
	return result.finish()
}

type snapshotIntegrity struct {
	count int
	err   error
}

func (v *snapshotIntegrity) accept(rows *sql.Rows) {
	v.count++
	var result string
	if err := rows.Scan(&result); err != nil {
		v.err = err
		return
	}
	if result != "ok" {
		v.err = fmt.Errorf("snapshot integrity validation failed: %s", result)
	}
}
func (v *snapshotIntegrity) finish() error {
	if v.err != nil {
		return v.err
	}
	if v.count != 1 {
		return fmt.Errorf("snapshot integrity validation did not return ok")
	}
	return nil
}
func (v *snapshotValidation) foreignKeys() error {
	rows, err := v.db.Query("PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("snapshot foreign key validation failed")
	}
	return rows.Err()
}
func expectedSnapshotSchema() (map[string]string, error) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	if err = createSnapshotSchema(db); err != nil {
		return nil, err
	}
	return snapshotSchemaObjects(db)
}
func validateSnapshotSchema(db snapshotQuery) error {
	want, err := expectedSnapshotSchema()
	if err != nil {
		return err
	}
	got, err := snapshotSchemaObjects(db)
	if err != nil {
		return err
	}
	return snapshotSchemaMatch(want, got)
}
func snapshotSchemaMatch(want, got map[string]string) error {
	if len(want) != len(got) {
		return fmt.Errorf("snapshot schema does not match supported schema")
	}
	for key, definition := range want {
		if got[key] != definition {
			return fmt.Errorf("snapshot schema object %s does not match supported schema", key)
		}
	}
	return nil
}
func snapshotSchemaObjects(db snapshotQuery) (map[string]string, error) {
	rows, err := db.Query("SELECT type,name,sql FROM sqlite_schema WHERE sql IS NOT NULL AND name NOT GLOB 'sqlite_*' ORDER BY type,name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	schema := snapshotSchemaReader{objects: map[string]string{}}
	for rows.Next() {
		schema.accept(rows)
	}
	return schema.objects, errors.Join(schema.err, rows.Err())
}

type snapshotSchemaReader struct {
	objects map[string]string
	err     error
}

func (s *snapshotSchemaReader) accept(rows *sql.Rows) {
	var kind, name, definition string
	if err := rows.Scan(&kind, &name, &definition); err != nil {
		s.err = err
		return
	}
	s.objects[kind+":"+name] = strings.Join(strings.Fields(definition), " ")
}

func (w *Writer) bindQueries()           { w.queries = New(w.tx) }
func snapshotQueries(db *sql.DB) Querier { return New(db) }

func validateSnapshotFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("snapshot destination must be a regular non-symlink file: %s", path)
	}
	return validateSnapshotSidecars(path)
}
func validateSnapshotSidecars(path string) error {
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err := os.Lstat(path + suffix); err == nil {
			return fmt.Errorf("snapshot has an unsupported sidecar: %s", path+suffix)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
