package cards

import (
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
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

	// Tokens holds compiled token scripts (forge-gui/res/tokenscripts),
	// keyed by file stem — e.g. "r_1_1_goblin" — the name a card's
	// TokenScript$ parameter references. Tokens are never Add-ed to Cards
	// or byName: a token is not a card a deck can contain, and Lookup must
	// not resolve a token's printed name to one.
	Tokens map[string]*Card
}

func NewRegistry() *Registry {
	return &Registry{byName: map[string]*Card{}, Tokens: map[string]*Card{}}
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

// cacheFile is the on-disk shape. Only Cards and Tokens are encoded; the
// byName index is rebuilt on load so it can never disagree with Cards.
type cacheFile struct {
	Version int
	Cards   []*Card
	Tokens  map[string]*Card
}

// cacheVersion 2 adds Tokens: a v1 cache predates Registry.Tokens entirely,
// so LoadRegistry refuses it outright (see the version check below) rather
// than silently serving a registry with no tokens compiled in.
const cacheVersion = 2

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
	if err := gob.NewEncoder(zw).Encode(cacheFile{Version: cacheVersion, Cards: r.Cards, Tokens: r.Tokens}); err != nil {
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
	if _, err := io.Copy(io.Discard, zr); err != nil {
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
	if cf.Tokens != nil {
		r.Tokens = cf.Tokens
	}
	return r, nil
}

// CompileDir walks a cardsfolder tree, parses, links and applies intrinsics.
// Paths are sorted so the resulting Cards slice — and therefore the cache
// bytes — are byte-identical across runs.
func CompileDir(dir string) (*Registry, []Diag, error) {
	parsed, diags, err := compileScripts(dir)
	if err != nil {
		return nil, nil, err
	}
	if len(parsed) == 0 {
		return nil, nil, fmt.Errorf("no card scripts under %s — run `make fetch-cards`", dir)
	}

	r := NewRegistry()
	for _, c := range parsed {
		// A card with no named face parsed without error but carries no
		// identity — e.g. a script whose only content is a directive like
		// CopyFaceFrom that this parser doesn't resolve. That is worth a
		// diagnostic per card, not per face: an ALTERNATE face alone being
		// nameless is normal (legitimate CopyFaceFrom usage on a second
		// face), but a card with no named face at all silently drops out of
		// Coverage, and a human should be told which file did that.
		if !c.named() {
			diags = append(diags, Diag{c.Path, "card has no named face on any face; excluded from coverage (likely an unresolved CopyFaceFrom or similar directive)"})
		}
		r.Add(c)
	}

	if err := compileTokens(r, dir, &diags); err != nil {
		return nil, nil, err
	}

	return r, diags, nil
}

// compileScripts walks dir for every .txt script and parses, links and
// applies intrinsics to each, in sorted-path order — so compilation order,
// and therefore the cache bytes, is reproducible across runs. It is the one
// pipeline shared by CompileDir's card loop and compileTokens' token loop:
// the two differ only in what they do with the resulting Cards afterward
// (add to Cards/byName plus diagnose namelessness, vs. key by file stem
// into Tokens), never in how a script gets from bytes on disk to a fully
// linked Card.
func compileScripts(dir string) ([]*Card, []Diag, error) {
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
	sort.Strings(paths)

	var diags []Diag
	out := make([]*Card, 0, len(paths))
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
		out = append(out, c)
	}
	return out, diags, nil
}

// compileTokens walks dir's tokenscripts sibling — Fetch's TokensDir — and
// compiles every script into r.Tokens, keyed by file stem ("r_1_1_goblin").
// Tokens share compileScripts' parse/link/intrinsics pipeline but are never
// Add-ed to Cards or byName: a token is not a card a deck can contain, and
// a card's Lookup-by-name must not resolve to one. A missing tokenscripts
// directory is not an error — plenty of fixtures (and every pre-M2r cache)
// have no tokens at all.
func compileTokens(r *Registry, cardsDir string, diags *[]Diag) error {
	tokensDir := TokensDir(filepath.Dir(cardsDir))
	info, err := os.Stat(tokensDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}

	tokens, d, err := compileScripts(tokensDir)
	if err != nil {
		return err
	}
	*diags = append(*diags, d...)
	for _, c := range tokens {
		stem := strings.TrimSuffix(filepath.Base(c.Path), ".txt")
		r.Tokens[stem] = c
	}
	return nil
}
