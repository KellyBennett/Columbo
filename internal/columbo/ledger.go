package columbo

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/KellyBennett/Columbo/internal/snapshotdb"
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

type excessBand struct {
	name     string
	maxRatio float64
}

var excessBands = []excessBand{{"at-limit", 1}, {"near", 1.25}, {"over", 2}, {"far", math.Inf(1)}}

const unmeasuredExcess = "unmeasured"

func excessOf(shape snapshotdb.CaseShapesRow) int64 {
	trigger := clueTrigger(shape.Operator, shape.ValueType, shape.LimitValue)
	if trigger <= 0 {
		return int64(len(excessBands))
	}
	ratio := shape.Value / trigger
	return int64(slices.IndexFunc(excessBands, func(band excessBand) bool { return ratio <= band.maxRatio }))
}

func clueTrigger(operator, valueType string, limit float64) float64 {
	if operator == ">" && valueType == "integer" {
		return limit + 1
	}
	if operator == ">" || operator == ">=" {
		return limit
	}
	return 0
}

func excessName(ordinal int64) string {
	if ordinal < int64(len(excessBands)) {
		return excessBands[ordinal].name
	}
	return unmeasuredExcess
}

func (i Invocation) ledgerPath() (string, error) {
	if i.Home == "" {
		return "", fmt.Errorf("home directory unknown")
	}
	return filepath.Join(i.Home, ".columbo", "ledger.sqlite"), nil
}

func (i Invocation) recordLedger(snapshot snapshotdb.Querier) error {
	path, err := i.ledgerPath()
	if err != nil {
		return err
	}
	repository, err := invocationRepository(i.Dir)
	if err != nil {
		return err
	}
	shapes, err := snapshot.CaseShapes(context.Background())
	if err != nil {
		return err
	}
	return recordShapes(path, repository, shapes)
}

func invocationRepository(dir string) (string, error) {
	root, err := moduleRoot(dir)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return "", err
	}
	return repositoryIdentity(root, modfile.ModulePath(data))
}

func repositoryIdentity(root, modulePath string) (string, error) {
	shallow, err := gitOutput(root, "rev-parse", "--is-shallow-repository")
	if err != nil {
		prefix, _, _ := module.SplitPathVersion(modulePath)
		return "module\x00" + prefix, nil
	}
	if strings.TrimSpace(shallow) == "true" {
		return "", errors.New("a shallow clone does not know its root commits; fetch the full history to record it")
	}
	return gitRootCommits(root)
}

func gitRootCommits(root string) (string, error) {
	roots, err := gitOutput(root, "rev-list", "--max-parents=0", "HEAD")
	if err != nil {
		return "", errors.New("cannot list the repository's root commits; record it after its first commit")
	}
	commits := strings.Fields(roots)
	slices.Sort(commits)
	return "git\x00" + strings.Join(commits, " "), nil
}

func recordShapes(path, repository string, shapes []snapshotdb.CaseShapesRow) error {
	ledger, err := openLedgerFile(path)
	if err != nil {
		return err
	}
	defer ledger.Close()
	if err = writeShapes(ledger.Queries(), repository, shapes); err != nil {
		return err
	}
	return ledger.Complete()
}

type ledgerEntry struct {
	path, rule string
	valid      func(fs.FileMode) bool
}

func ledgerExists(path string) (bool, error) {
	for _, entry := range []ledgerEntry{
		{filepath.Dir(path), "ledger directory must be a non-symlink directory", fs.FileMode.IsDir},
		{path, "ledger must be a regular non-symlink file", fs.FileMode.IsRegular},
	} {
		if exists, err := entry.exists(); !exists || err != nil {
			return false, err
		}
	}
	return true, nil
}

func (e ledgerEntry) exists() (bool, error) {
	info, err := os.Lstat(e.path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err == nil && !e.valid(info.Mode()) {
		err = fmt.Errorf("%s: %s", e.rule, e.path)
	}
	return err == nil, err
}

func openLedgerFile(path string) (*snapshotdb.Ledger, error) {
	if _, err := ledgerExists(path); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		return nil, err
	}
	if err = file.Close(); err != nil {
		return nil, err
	}
	return snapshotdb.RecordLedger(path)
}

func writeShapes(queries snapshotdb.Querier, repository string, shapes []snapshotdb.CaseShapesRow) error {
	recorder, err := newLedgerRecorder(queries, repository)
	if err != nil {
		return err
	}
	for _, shape := range shapes {
		if err := recorder.record(shape); err != nil {
			return err
		}
	}
	return nil
}

type ledgerRecorder struct {
	queries snapshotdb.Querier
	secret  []byte
	repo    []byte
}

func newLedgerRecorder(queries snapshotdb.Querier, repository string) (*ledgerRecorder, error) {
	ctx := context.Background()
	if err := queries.InsertLedgerSecret(ctx, snapshotdb.InsertLedgerSecretParams{Secret: newLedgerSecret()}); err != nil {
		return nil, err
	}
	secret, err := queries.LedgerSecret(ctx)
	recorder := &ledgerRecorder{queries: queries, secret: secret}
	recorder.repo = recorder.key("repo", repository)
	return recorder, err
}

func newLedgerSecret() []byte {
	secret := make([]byte, sha256.Size)
	_, _ = rand.Read(secret)
	return secret
}

func (r *ledgerRecorder) record(shape snapshotdb.CaseShapesRow) error {
	return r.queries.RecordLedgerCase(context.Background(), snapshotdb.RecordLedgerCaseParams{
		Repo: r.repo, CaseKey: r.key("case", shape.ID), Smell: shape.Smell,
		Verdict: shape.Verdict, Suppressed: shape.Suppressed, Excess: excessOf(shape),
	})
}

func (r *ledgerRecorder) key(kind, value string) []byte {
	mac := hmac.New(sha256.New, r.secret)
	mac.Write([]byte(kind + "\x00" + value))
	return mac.Sum(nil)
}
