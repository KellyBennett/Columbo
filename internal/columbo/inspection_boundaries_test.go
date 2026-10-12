package columbo

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspectionInventoryAcceptancePrecedesClassification(t *testing.T) {
	root := inspectionTestModule(t)
	a, err := load(root, []string{"./..."}, Defaults())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(root, "model.go"), []byte("stale"), 0600))
	link := filepath.Join(root, "a-not-go")
	require.NoError(t, os.Symlink("absent", link))
	files, err := a.inspectionFiles()
	assert.EqualError(t, err, "symlink source path refused: "+link, "earlier non-Go symlink must win")
	assert.Empty(t, files)
	require.NoError(t, os.Remove(link))
	files, err = a.inspectionFiles()
	assert.EqualError(t, err, "source changed during inspection: model.go")
	require.Len(t, files, 2, "stale selected source must retain earlier entries")
	assert.Equal(t, "generated.go", files[0].Path)
}

func TestInspectionInventoryPrunesDotFoldersAndRetainsNonSelectedBytes(t *testing.T) {
	root := inspectionTestModule(t)
	a, err := load(root, []string{"./..."}, Defaults())
	require.NoError(t, err)
	for _, name := range []string{".git", ".claude", "nested/.cache"} {
		directory := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(directory, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(directory, "notes.md"), []byte("ignored"), 0600))
		require.NoError(t, os.Symlink("absent", filepath.Join(directory, "bad.go")))
	}
	data := []byte("changed non-selected source")
	require.NoError(t, os.WriteFile(filepath.Join(root, "platform_windows.go"), data, 0600))
	files, err := a.inspectionFiles()
	require.NoError(t, err)
	byPath := map[string]inspectionFile{}
	for _, f := range files {
		byPath[f.Path] = f
	}
	assert.Len(t, files, 5)
	require.Contains(t, byPath, "platform_windows.go")
	unselected := byPath["platform_windows.go"]
	assert.Equal(t, inspectionHash(data), unselected.SHA, "non-selected physical bytes")
	assert.False(t, unselected.Selected)
	assert.False(t, unselected.Target)
	assert.Regexp(t, "^not-selected-runtime", unselected.Scope)
	require.Contains(t, byPath, "model_test.go")
	assert.False(t, byPath["model_test.go"].Selected)
	assert.Equal(t, "test source: not selected", byPath["model_test.go"].Scope)
}

func TestInspectionLexicalBindingsRemainPhysicalAndOneHop(t *testing.T) {
	root := inspectionTestModule(t)
	facts := inspectTestFacts(t, root)
	for _, d := range facts.Declarations {
		for _, c := range d.Calls {
			for _, span := range []*inspectionSpan{c.Binding, c.Receiver} {
				if span == nil {
					continue
				}
				data, err := os.ReadFile(filepath.Join(root, span.Path))
				require.NoError(t, err)
				require.GreaterOrEqual(t, span.StartOffset, 0)
				require.GreaterOrEqual(t, span.EndOffset, span.StartOffset)
				require.LessOrEqual(t, span.EndOffset, len(data))
				assert.Equal(t, inspectionHash(data), span.FileSHA, "physical binding file hash")
				assert.Equal(t, inspectionHash(data[span.StartOffset:span.EndOffset]), span.SHA, "physical binding span hash")
			}
			if c.Target != nil {
				assert.Empty(t, c.Target.Calls, "call targets must not expand beyond one hop")
			}
		}
	}
}

func TestInspectionDotFolderChangesDoNotAffectSourceIdentity(t *testing.T) {
	root := inspectionTestModule(t)
	hiddenRoot := filepath.Join(t.TempDir(), ".project")
	require.NoError(t, os.Rename(root, hiddenRoot))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenRoot, ".columbo.yml"), []byte("exclude: []\n"), 0600))
	before := inspectTestFacts(t, hiddenRoot)
	require.NoError(t, os.Mkdir(filepath.Join(hiddenRoot, ".claude"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(hiddenRoot, ".claude", "broken.go"), []byte("not Go source"), 0600))
	after := inspectTestFacts(t, hiddenRoot)
	assert.Equal(t, before.SourceSHA, after.SourceSHA)
	assert.Equal(t, before.Files, after.Files)
	assert.NotEmpty(t, after.ConfigSHA)
	assert.Equal(t, ".columbo.yml", after.ConfigFile)
}
