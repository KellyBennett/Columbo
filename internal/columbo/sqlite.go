package columbo

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// WriteSnapshot publishes a complete sibling database at a new output path.
// Atomic no-clobber creation preserves every existing destination.
func WriteSnapshot(path string, report Report, version string) error {
	return writeSnapshotWithIO(path, snapshotContents{report: report, version: version}, snapshotFilesystem{})
}

type snapshotContents struct {
	report  Report
	version string
}

func writeSnapshotWithIO(path string, contents snapshotContents, operations snapshotIO) error {
	publication := snapshotPublication{output: path, contents: contents, operations: operations}
	return publication.run()
}

// The publication seam permits bounded storage-failure tests without changing
// the production transaction or weakening destination validation.
type snapshotIO interface {
	Build(string, snapshotContents) error
	Sync(string) error
	Publish(snapshotPublicationTarget) error
	SyncDirectory(string) error
}
type snapshotPublicationTarget struct {
	from, to string
}
type snapshotFilesystem struct{}

func (snapshotFilesystem) Build(path string, contents snapshotContents) error {
	return buildSnapshot(path, contents)
}
func (snapshotFilesystem) Sync(path string) error                         { return syncSnapshot(path) }
func (snapshotFilesystem) Publish(target snapshotPublicationTarget) error { return target.publish() }
func (target snapshotPublicationTarget) publish() error {
	// Linking atomically requires an absent destination at the actual publication.
	// The deferred temporary cleanup removes the other name of this same inode.
	return os.Link(target.from, target.to)
}
func (snapshotFilesystem) SyncDirectory(path string) error { return syncSnapshotDirectory(path) }

type snapshotPublication struct {
	output, temporary string
	contents          snapshotContents
	operations        snapshotIO
}

func (p *snapshotPublication) run() error {
	if err := p.prepare(); err != nil {
		return err
	}
	defer removeSnapshotTemporary(p.temporary)
	if err := p.build(); err != nil {
		return err
	}
	return p.publish()
}
func (p *snapshotPublication) prepare() error {
	name, err := snapshotPath(p.output)
	if err != nil {
		return err
	}
	p.output = name
	if err = snapshotDestinationAbsent(name); err != nil {
		return err
	}
	p.temporary, err = newSnapshotTemporary(filepath.Dir(name))
	return err
}
func (p *snapshotPublication) build() error {
	if err := p.operations.Build(p.temporary, p.contents); err != nil {
		return err
	}
	return p.operations.Sync(p.temporary)
}
func (p *snapshotPublication) publish() error {
	if err := snapshotNoSidecars(p.output); err != nil {
		return err
	}
	if err := p.operations.Publish(snapshotPublicationTarget{from: p.temporary, to: p.output}); err != nil {
		return fmt.Errorf("publish snapshot: %w", err)
	}
	return p.operations.SyncDirectory(filepath.Dir(p.output))
}
func newSnapshotTemporary(directory string) (string, error) {
	temporary, err := os.CreateTemp(directory, ".columbo-*.sqlite")
	if err != nil {
		return "", fmt.Errorf("create snapshot: %w", err)
	}
	name := temporary.Name()
	if err = temporary.Close(); err != nil {
		removeSnapshotTemporary(name)
		return "", err
	}
	return name, nil
}

// OpenSnapshot validates and opens an immutable, read-only Columbo snapshot.
// Immutable readers do not create journals, WAL files, or shared-memory files.
func OpenSnapshot(path string) (*sql.DB, error) {
	name, err := validateSnapshotPath(path)
	if err != nil {
		return nil, err
	}
	db, err := openSnapshotDatabase(name, true)
	if err != nil {
		return nil, err
	}
	if err = validateSnapshot(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}
func validateSnapshotPath(path string) (string, error) {
	name, err := snapshotPath(path)
	if err != nil {
		return "", err
	}
	if _, err = snapshotRegularFile(name); err != nil {
		return "", err
	}
	return name, snapshotNoSidecars(name)
}
func snapshotPath(path string) (string, error) {
	if path == "" || path == "-" {
		return "", fmt.Errorf("snapshot requires a filesystem destination")
	}
	return filepath.Abs(path)
}
func snapshotRegularFile(path string) (os.FileInfo, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("snapshot destination must be a regular non-symlink file: %s", path)
	}
	return info, nil
}
func snapshotNoSidecars(path string) error {
	for _, suffix := range []string{"-journal", "-wal", "-shm"} {
		if _, err := os.Lstat(path + suffix); err == nil {
			return fmt.Errorf("snapshot has an unsupported sidecar: %s", path+suffix)
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
func snapshotDestinationAbsent(path string) error {
	_, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return snapshotNoSidecars(path)
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("snapshot destination already exists; choose a fresh output path: %s", path)
}
func removeSnapshotTemporary(path string) {
	for _, suffix := range []string{"", "-journal", "-wal", "-shm"} {
		_ = os.Remove(path + suffix)
	}
}
func syncSnapshot(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	return errors.Join(err, file.Close())
}
func syncSnapshotDirectory(path string) error {
	directory, err := os.Open(path)
	if err != nil {
		return err
	}
	err = directory.Sync()
	return errors.Join(err, directory.Close())
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
func buildSnapshot(path string, contents snapshotContents) error {
	db, err := openSnapshotDatabase(path, false)
	if err != nil {
		return err
	}
	err = populateSnapshot(db, contents)
	return errors.Join(err, db.Close())
}
func populateSnapshot(db *sql.DB, contents snapshotContents) error {
	if _, err := db.Exec("PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL"); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = writeSnapshotTransaction(tx, contents); err != nil {
		return err
	}
	return tx.Commit()
}
func writeSnapshotTransaction(tx *sql.Tx, contents snapshotContents) error {
	if err := createSnapshotSchema(tx); err != nil {
		return err
	}
	if err := newSnapshotWriter(tx).write(contents.report, contents.version); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return validateSnapshotData(tx)
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
func validateSnapshotIdentity(db snapshotQuery) error {
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
	return validateSnapshotReport(db)
}
func validateSnapshotReport(db snapshotQuery) error {
	var count, schemaVersion int
	if err := db.QueryRow("SELECT COUNT(*), COALESCE(MAX(schema_version),0) FROM report").Scan(&count, &schemaVersion); err != nil {
		return err
	}
	if count != 1 || schemaVersion != SchemaVersion {
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

// Validation cursors own their individual PRAGMA contracts and always close
// rows before another operation can reuse the single database connection.
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
