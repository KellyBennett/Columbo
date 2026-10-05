package columbo

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type sqliteFaultIO struct {
	snapshotFilesystem
	phase string
}

func (f sqliteFaultIO) Build(path string, contents snapshotContents) error {
	switch f.phase {
	case "disk":
		return sqliteLimitedBuild(path, contents)
	case "interruption":
		return sqliteInterruptedBuild(path)
	default:
		return f.snapshotFilesystem.Build(path, contents)
	}
}
func (f sqliteFaultIO) Sync(path string) error {
	if f.phase == "sync" {
		return errors.New("injected snapshot sync failure")
	}
	return f.snapshotFilesystem.Sync(path)
}
func (f sqliteFaultIO) Publish(target snapshotPublicationTarget) error {
	if f.phase == "publish" {
		return errors.New("injected publication failure")
	}
	return f.snapshotFilesystem.Publish(target)
}
func (f sqliteFaultIO) SyncDirectory(path string) error {
	if f.phase == "directory" {
		return errors.New("injected directory sync failure")
	}
	return f.snapshotFilesystem.SyncDirectory(path)
}
func sqliteLimitedBuild(path string, contents snapshotContents) error {
	db, err := openSnapshotDatabase(path, false)
	if err != nil {
		return err
	}
	defer db.Close()
	if _, err = db.Exec("PRAGMA max_page_count=1"); err != nil {
		return err
	}
	return populateTestSnapshot(db, contents)
}
func sqliteInterruptedBuild(path string) error {
	db, err := openSnapshotDatabase(path, false)
	if err != nil {
		return err
	}
	defer db.Close()
	return sqliteInterruptedTransaction(db)
}
func sqliteInterruptedTransaction(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = createSnapshotSchema(tx); err != nil {
		return err
	}
	return context.Canceled
}
func TestSQLiteStorageAndPublicationFailures(t *testing.T) {
	for _, phase := range []string{"disk", "interruption", "sync", "publish"} {
		t.Run(phase, func(t *testing.T) { newSQLiteHarness(t).storageFailure(phase) })
	}
}
func (h *sqliteHarness) storageFailure(phase string) {
	previous := h.previousSnapshot()
	before := previous.bytes()
	err := h.fault(phase)
	require.Error(h.T, err)
	require.Equal(h.T, before, previous.bytes())
	h.noOutput()
	h.checkStorageFailure(phase, err)
	require.NoError(h.T, snapshotNoSidecars(h.path))
	h.noTemporarySnapshots()
}
func (h *sqliteHarness) previousSnapshot() *sqliteHarness {
	previous := &sqliteHarness{T: h.T, path: h.path + ".previous.sqlite"}
	previous.writeEmpty()
	return previous
}
func (h *sqliteHarness) checkStorageFailure(phase string, err error) {
	if phase == "disk" {
		require.True(h.T, strings.Contains(err.Error(), "database or disk is full"), err)
	}
}
func (h *sqliteHarness) noTemporarySnapshots() {
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(h.path), ".columbo-*"))
	require.NoError(h.T, err)
	require.Empty(h.T, matches)
}
func TestSQLitePostPublicationDirectoryFailure(t *testing.T) {
	newSQLiteHarness(t).checkPostPublicationDirectoryFailure()
}
func (h *sqliteHarness) checkPostPublicationDirectoryFailure() {
	err := h.fault("directory")
	require.Error(h.T, err)
	db := h.open()
	h.caseCount(db, 1)
	h.noTemporarySnapshots()
	require.NoError(h.T, snapshotNoSidecars(h.path))
}
func (h *sqliteHarness) caseCount(db *sql.DB, want int) {
	var count int
	require.NoError(h.T, db.QueryRow("SELECT COUNT(*) FROM cases").Scan(&count))
	require.Equal(h.T, want, count)
}
func TestSQLiteNonexistentFailureCreatesNoOutput(t *testing.T) {
	newSQLiteHarness(t).checkNonexistentFailureCreatesNoOutput()
}
func (h *sqliteHarness) checkNonexistentFailureCreatesNoOutput() {
	require.Error(h.T, h.fault("disk"))
	h.noOutput()
	h.noTemporarySnapshots()
}

func (h *sqliteHarness) fault(phase string) error {
	return writeSnapshotWithIO(h.path, snapshotContents{report: sqliteTestReport(), version: "replacement"}, sqliteFaultIO{phase: phase})
}
func (h *sqliteHarness) noOutput() {
	_, err := os.Lstat(h.path)
	require.True(h.T, errors.Is(err, os.ErrNotExist))
}

// A competing creator arrives after validation, immediately before publication.
type sqliteConcurrentCreator struct{ snapshotFilesystem }

func (c sqliteConcurrentCreator) Publish(target snapshotPublicationTarget) error {
	if err := os.WriteFile(target.to, []byte("unrelated concurrent file"), 0600); err != nil {
		return err
	}
	return c.snapshotFilesystem.Publish(target)
}
func TestSQLiteAbsentPublicationNeverClobbersNewFile(t *testing.T) {
	newSQLiteHarness(t).checkConcurrentCreator()
}
func (h *sqliteHarness) checkConcurrentCreator() {
	err := writeSnapshotWithIO(h.path, snapshotContents{version: "test"}, sqliteConcurrentCreator{})
	require.Error(h.T, err)
	require.Equal(h.T, []byte("unrelated concurrent file"), h.bytes())
	h.noTemporarySnapshots()
}

// A competing snapshot arrives and is replaced by an unrelated file before
// our publication. No-clobber creation must honor the target present then.
type sqliteConcurrentReplacement struct{ snapshotFilesystem }

func (r sqliteConcurrentReplacement) Publish(target snapshotPublicationTarget) error {
	if err := WriteSnapshot(target.to, Report{}, "competing"); err != nil {
		return err
	}
	if err := replaceConcurrentSnapshot(target.to); err != nil {
		return err
	}
	return r.snapshotFilesystem.Publish(target)
}
func replaceConcurrentSnapshot(path string) error {
	incoming := path + ".incoming"
	if err := os.WriteFile(incoming, []byte("unrelated replacement file"), 0600); err != nil {
		return err
	}
	return os.Rename(incoming, path)
}
func TestSQLitePublicationNeverClobbersConcurrentReplacement(t *testing.T) {
	newSQLiteHarness(t).checkConcurrentReplacement()
}
func (h *sqliteHarness) checkConcurrentReplacement() {
	err := writeSnapshotWithIO(h.path, snapshotContents{version: "test"}, sqliteConcurrentReplacement{})
	require.Error(h.T, err)
	require.Equal(h.T, []byte("unrelated replacement file"), h.bytes())
	h.noTemporarySnapshots()
}
