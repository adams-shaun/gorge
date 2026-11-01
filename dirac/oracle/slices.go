package oracle

import (
	"archive/tar"
	"compress/gzip"
	"embed"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// A slice is what a clone has instead of a pool tree: the first SlicePairs pairs of
// a pool at SliceSeed (pairs.tsv, a -pairs-file with each pair's seed written out)
// and only the decks they name, with the matching pool.tsv rows. Playing it gives
// the same games as the same pairs of the whole pool; TestSliceMatchesPool holds
// them equal where a pool is on disk.

// The pairs the committed slices hold.
const (
	SliceSeed  = 1
	SlicePairs = 1500
)

//go:embed testdata/slices
var sliceFS embed.FS

// ExtractSlice writes the committed slice of pool name into a new temporary
// directory, as decks/, pool.tsv and pairs.tsv, and returns it with the function
// that removes it.
func ExtractSlice(name string) (dir string, remove func(), err error) {
	src := path.Join("testdata/slices", name)
	pairs, err := sliceFS.ReadFile(path.Join(src, "pairs.tsv"))
	if err != nil {
		return "", nil, fmt.Errorf("no committed slice of pool %q: %w", name, err)
	}
	f, err := sliceFS.Open(path.Join(src, "decks.tar.gz"))
	if err != nil {
		return "", nil, fmt.Errorf("no committed slice of pool %q: %w", name, err)
	}
	defer f.Close()
	dir, err = os.MkdirTemp("", "dirac-slice-"+name+"-")
	if err != nil {
		return "", nil, err
	}
	remove = func() { os.RemoveAll(dir) }
	if err := os.WriteFile(filepath.Join(dir, "pairs.tsv"), pairs, 0o644); err != nil {
		remove()
		return "", nil, err
	}
	if err := untar(f, dir); err != nil {
		remove()
		return "", nil, fmt.Errorf("slice %s: %w", name, err)
	}
	return dir, remove, nil
}

func untar(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	if err := os.Mkdir(filepath.Join(dir, "decks"), 0o755); err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !strings.HasPrefix(h.Name, "decks/") && h.Name != "pool.tsv" || strings.Contains(h.Name, "..") {
			return fmt.Errorf("unexpected member %q", h.Name)
		}
		out, err := os.Create(filepath.Join(dir, filepath.FromSlash(h.Name)))
		if err != nil {
			return err
		}
		_, err = io.Copy(out, tr)
		if cerr := out.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return err
		}
	}
}
