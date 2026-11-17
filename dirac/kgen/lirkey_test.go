package kgen

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestLIRKeyIsBlockHash(t *testing.T) {
	cs := lowerNamed(t, []string{"Banishing Light", "Sol Ring"})
	if len(cs) != 2 {
		t.Fatalf("lowered %d cards, want 2", len(cs))
	}
	keys := LIRKeys(cs)
	if len(keys) != 2 {
		t.Fatalf("LIRKeys has %d entries, want 2: %v", len(keys), keys)
	}
	seen := map[string]bool{}
	for _, c := range cs {
		var b bytes.Buffer
		dumpLIR(&b, c)
		if !strings.HasPrefix(b.String(), "CARD "+c.Name+" token=") {
			t.Fatalf("%s: unexpected dump head %q", c.Name, b.String())
		}
		sum := sha256.Sum256(b.Bytes())
		want := hex.EncodeToString(sum[:])[:16]
		if keys[c.Name] != want {
			t.Errorf("%s: key %q, want %q", c.Name, keys[c.Name], want)
		}
		seen[keys[c.Name]] = true
	}
	if len(seen) != 2 {
		t.Errorf("the two cards share a key: %v", keys)
	}
}

func TestLIRKeysFlagSorted(t *testing.T) {
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Sol Ring\nBanishing Light\nAcademy Ruins\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out.txt")
	f, err := os.Create(out)
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = f
	status := Main([]string{"-lir-keys", "-card-list", list, "-cards", cardsDir})
	os.Stdout = old
	f.Close()
	if status != 0 {
		t.Fatalf("status %d", status)
	}
	data, _ := os.ReadFile(out)
	lines := strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines, want 3: %q", len(lines), lines)
	}
	if !sort.StringsAreSorted(lines) {
		t.Errorf("not sorted: %q", lines)
	}
	for _, l := range lines {
		f := strings.Split(l, "\t")
		if len(f) != 2 || len(f[1]) != 16 {
			t.Errorf("bad line %q", l)
		}
	}
}
