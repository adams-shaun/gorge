package env_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	maxGoldenFileSize = 512 << 10
	maxGoldenDirSize  = 4 << 20
)

// TestSpellbenchGoldensAreSmall keeps the scenario fixtures committable. The
// protocol_v2/ tree is the vendored external corpus (byte-identical to the
// upstream SpellBench goldens), so it is exempt from the per-file cap; its
// bytes are still counted in the directory total.
func TestSpellbenchGoldensAreSmall(t *testing.T) {
	root := filepath.Join("..", "testdata")
	protocolV2 := filepath.Join("goldens", "protocol_v2") + string(filepath.Separator)
	var total int64
	var files int
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		size := info.Size()
		total += size
		files++
		if strings.HasPrefix(rel, protocolV2) {
			return nil
		}
		if size >= maxGoldenFileSize {
			t.Errorf("%s is %d bytes, want under %d", rel, size, maxGoldenFileSize)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total >= maxGoldenDirSize {
		t.Errorf("%s totals %d bytes across %d files, want under %d", root, total, files, maxGoldenDirSize)
	}
	t.Logf("%s: %d files, %d bytes", root, files, total)
}
