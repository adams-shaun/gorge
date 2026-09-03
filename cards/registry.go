package cards

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// Registry is the compiled corpus: every card, indexed by normalised name.
type Registry struct {
	Cards  []*Card
	byName map[string]*Card
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]*Card{}}
}

// NormalizeName folds case, collapses whitespace and drops punctuation so
// catalogue names from Scryfall match Forge script names. A "Front // Back"
// name resolves to the front face, which is how Forge names the file.
func NormalizeName(s string) string {
	if i := strings.Index(s, "//"); i > 0 {
		s = s[:i]
	}
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.ToLower(s) {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			prevSpace = false
		case unicode.IsSpace(r):
			if !prevSpace {
				b.WriteByte(' ')
				prevSpace = true
			}
		default:
			// Apostrophes, commas, colons and hyphens are dropped, not turned
			// into spaces: "Urza's Tower" and "Urzas Tower" must agree.
		}
	}
	return strings.TrimSpace(b.String())
}

func (r *Registry) Add(c *Card) {
	r.Cards = append(r.Cards, c)
	if r.byName == nil {
		r.byName = map[string]*Card{}
	}
	for _, f := range c.Faces {
		if k := NormalizeName(f.Name); k != "" {
			if _, exists := r.byName[k]; !exists {
				r.byName[k] = c
			}
		}
	}
}

func (r *Registry) Lookup(name string) (*Card, bool) {
	c, ok := r.byName[NormalizeName(name)]
	return c, ok
}

// cacheFile is the on-disk shape. Only Cards is encoded; the index is rebuilt
// on load so the two can never disagree.
type cacheFile struct {
	Version int
	Cards   []*Card
}

const cacheVersion = 1

func (r *Registry) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	zw, err := gzip.NewWriterLevel(f, gzip.BestSpeed)
	if err != nil {
		f.Close()
		return err
	}
	if err := gob.NewEncoder(zw).Encode(cacheFile{Version: cacheVersion, Cards: r.Cards}); err != nil {
		zw.Close()
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func LoadRegistry(path string) (*Registry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	var cf cacheFile
	if err := gob.NewDecoder(zr).Decode(&cf); err != nil {
		zr.Close()
		return nil, err
	}
	if err := zr.Close(); err != nil {
		return nil, err
	}
	if cf.Version != cacheVersion {
		return nil, fmt.Errorf("IR cache version %d, want %d — run `make compile-cards`", cf.Version, cacheVersion)
	}
	r := NewRegistry()
	for _, c := range cf.Cards {
		r.Add(c)
	}
	return r, nil
}

// CompileDir walks a cardsfolder tree, parses, links and applies intrinsics.
// Paths are sorted so the resulting Cards slice — and therefore the cache
// bytes — are byte-identical across runs.
func CompileDir(dir string) (*Registry, []Diag, error) {
	var paths []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(p, ".txt") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no card scripts under %s — run `make fetch-cards`", dir)
	}
	sort.Strings(paths)

	r := NewRegistry()
	var diags []Diag
	for _, p := range paths {
		c, d := Parse(p)
		diags = append(diags, d...)
		if c == nil {
			continue
		}
		diags = append(diags, c.Link()...)
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		r.Add(c)
	}
	return r, diags, nil
}
