package trials

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

const devNull = "/dev/null"

// hunk is one @@ block: where it starts in the old file and its lines, each still carrying its ' ', '-', or '+' mark.
type hunk struct {
	start, oldLen int
	lines         []string
}

// applyPatch replays a diffTrees patch on root. Every context and removed line must match, so a patch never lands on
// a tree it was not made from.
func applyPatch(root string, patch []byte) error {
	lines := strings.SplitAfter(string(patch), "\n")
	for i := 0; i < len(lines); {
		if lines[i] == "" {
			i++
			continue
		}
		if i+1 >= len(lines) || !strings.HasPrefix(lines[i], "--- ") || !strings.HasPrefix(lines[i+1], "+++ ") {
			return fmt.Errorf("patch line %d: want a --- and +++ header, got %q", i+1, lines[i])
		}
		from, to := strings.TrimSpace(lines[i][4:]), strings.TrimSpace(lines[i+1][4:])
		i += 2
		var hunks []hunk
		for i < len(lines) && strings.HasPrefix(lines[i], "@@ ") {
			h, n, err := readHunk(lines[i:])
			if err != nil {
				return fmt.Errorf("patch line %d: %w", i+1, err)
			}
			hunks = append(hunks, h)
			i += n
		}
		if err := patchFile(root, from, to, hunks); err != nil {
			return err
		}
	}
	return nil
}

func readHunk(lines []string) (hunk, int, error) {
	var oldSpec, newSpec string
	if _, err := fmt.Sscanf(lines[0], "@@ -%s +%s @@", &oldSpec, &newSpec); err != nil {
		return hunk{}, 0, fmt.Errorf("bad hunk header %q", lines[0])
	}
	start, oldLen, err := hunkRange(oldSpec)
	if err != nil {
		return hunk{}, 0, err
	}
	_, newLen, err := hunkRange(newSpec)
	if err != nil {
		return hunk{}, 0, err
	}
	h := hunk{start: start, oldLen: oldLen}
	n := 1
	for olds, news := 0, 0; olds < oldLen || news < newLen; n++ {
		if n >= len(lines) || lines[n] == "" {
			return hunk{}, 0, fmt.Errorf("hunk %q ends early", strings.TrimSpace(lines[0]))
		}
		switch lines[n][0] {
		case ' ':
			olds, news = olds+1, news+1
		case '-':
			olds++
		case '+':
			news++
		default:
			return hunk{}, 0, fmt.Errorf("bad hunk line %q", lines[n])
		}
		h.lines = append(h.lines, lines[n])
	}
	if oldLen > 0 {
		h.start--
	}
	return h, n, nil
}

// hunkRange reads difflib's "start,length" range; a lone number has length 1, and an empty range starts at the line before it.
func hunkRange(spec string) (start, length int, err error) {
	first, rest, hasLength := strings.Cut(spec, ",")
	if start, err = strconv.Atoi(first); err != nil {
		return 0, 0, fmt.Errorf("bad hunk range %q", spec)
	}
	length = 1
	if hasLength {
		if length, err = strconv.Atoi(rest); err != nil {
			return 0, 0, fmt.Errorf("bad hunk range %q", spec)
		}
	}
	return start, length, nil
}

// patchFile works on difflib's lines, which always end with a newline line that the file itself does not hold.
func patchFile(root, from, to string, hunks []hunk) error {
	rel := strings.TrimPrefix(to, "b/")
	if to == devNull {
		rel = strings.TrimPrefix(from, "a/")
	}
	path := filepath.Join(root, filepath.FromSlash(rel))
	var old []string
	if from != devNull {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		old = difflib.SplitLines(string(data))
	}
	var out []string
	pos := 0
	for _, h := range hunks {
		if h.start < pos || h.start+h.oldLen > len(old) {
			return fmt.Errorf("%s: hunk at line %d is outside the file", rel, h.start+1)
		}
		out = append(out, old[pos:h.start]...)
		at := h.start
		for _, l := range h.lines {
			mark, text := l[0], l[1:]
			if mark != '+' {
				if old[at] != text {
					return fmt.Errorf("%s: line %d is %q, the patch expects %q", rel, at+1, old[at], text)
				}
				at++
			}
			if mark != '-' {
				out = append(out, text)
			}
		}
		pos = at
	}
	out = append(out, old[pos:]...)
	if to == devNull {
		return os.Remove(path)
	}
	if n := len(out); n > 0 && out[n-1] == "\n" {
		out = out[:n-1]
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(out, "")), 0o644)
}
