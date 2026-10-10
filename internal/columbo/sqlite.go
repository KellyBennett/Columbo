package columbo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
)

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

func OpenSnapshot(path string) (*snapshotdb.Snapshot, error) {
	name, err := validateSnapshotPath(path)
	if err != nil {
		return nil, err
	}
	return snapshotdb.Open(name)
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
	return snapshotdb.ValidateRegularFile(path)
}
func snapshotNoSidecars(path string) error {
	return snapshotdb.ValidateSidecars(path)
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
func buildSnapshot(path string, contents snapshotContents) error {
	writer, err := snapshotdb.Create(path)
	if err != nil {
		return err
	}
	err = contents.populate(writer)
	return errors.Join(err, writer.Close())
}
func (contents snapshotContents) populate(writer *snapshotdb.Writer) error {
	if err := newSnapshotWriter(writer.Queries()).write(contents.report, contents.version); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	return writer.Complete()
}
