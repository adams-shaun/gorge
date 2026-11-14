package cards

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// WalkScripts calls fn with the path of every .txt file under root, in
// filepath.WalkDir order. It is the one directory walk over a script tree
// (cards, tokens, kgen's ALL list), because filepath.WalkDir does not follow
// a symlinked root: it yields the link as one non-directory entry, so a root
// that is a symlink — what cards-overlay.sh makes of cardsfolder and
// tokenscripts — silently reads as an empty tree. So:
//
//   - a symlinked root is resolved before the walk; the paths fn sees are still
//     under root as the caller spelled it, so Rel(root, path) and a Card's Path
//     read the same as for a plain directory;
//   - a symlinked directory below the root is an error naming it, since the
//     walk would skip its scripts without a word. A symlinked file is fine:
//     it is read through.
//
// A missing root returns an error that satisfies errors.Is(err, fs.ErrNotExist).
func WalkScripts(root string, fn func(path string) error) error {
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	return filepath.WalkDir(resolved, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(resolved, p)
		if err != nil {
			return err
		}
		if d.Type()&fs.ModeSymlink != 0 {
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				return fmt.Errorf("cards: symlinked directory %q under %s is not followed: its scripts would be skipped; replace it with a real directory", rel, root)
			}
		}
		if !strings.HasSuffix(p, ".txt") {
			return nil
		}
		return fn(filepath.Join(root, rel))
	})
}
