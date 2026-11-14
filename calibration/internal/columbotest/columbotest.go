// Package columbotest builds the columbo binary once per test process for tests that run it on fixtures.
package columbotest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	once sync.Once
	bin  string
	err  error
	log  []byte
)

func Binary(t testing.TB) string {
	t.Helper()
	once.Do(func() {
		dir, mkErr := os.MkdirTemp("", "calibration-columbo-")
		if mkErr != nil {
			err = mkErr
			return
		}
		bin = filepath.Join(dir, "columbo")
		_, self, _, _ := runtime.Caller(0)
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/columbo")
		cmd.Dir = filepath.Join(filepath.Dir(self), "..", "..", "..")
		log, err = cmd.CombinedOutput()
	})
	if err != nil {
		t.Fatalf("build columbo: %v\n%s", err, log)
	}
	return bin
}

// ResolveTempDir points TMPDIR past macOS's /var symlink, since columbo refuses a symlinked source root.
func ResolveTempDir(t testing.TB) {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(os.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", resolved)
}
