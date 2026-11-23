package kgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// supportNode is the IR node a refusal by name is recorded under. Unlike every other
// refusal it comes after lowering and building, so the card's table entry stays whole
// and a raw set can still play it.
const supportNode = "support"

// supportLists is the parsed dirac/support tree.
type supportLists struct {
	refused   map[string]*cardset.RefusedEntry
	evidenced map[string]bool
	// podEvidenced names whose evidence row carries a seats field of three or
	// more: the card has a clean pod game. It is report only — statusOf does
	// not read it, so a pod game does not evidence a card the support rule
	// (which stage 2, OD-4, may change) does not already cover.
	podEvidenced map[string]bool
}

func readSupport(src fs.FS) (*supportLists, error) {
	entries, err := cardset.ReadRefused(src)
	if err != nil {
		return nil, err
	}
	s := &supportLists{refused: map[string]*cardset.RefusedEntry{}, evidenced: map[string]bool{}, podEvidenced: map[string]bool{}}
	for i := range entries {
		s.refused[entries[i].Card] = &entries[i]
	}
	lists, err := fs.Glob(src, path.Join(cardset.EvidenceDir, "*.txt"))
	if err != nil {
		return nil, err
	}
	for _, f := range lists {
		b, err := fs.ReadFile(src, f)
		if err != nil {
			return nil, err
		}
		for _, r := range parseCardRows(b) {
			if r.col == "" {
				// A plain row is the evidence that makes a card supported.
				// A tabbed row (Name\t<seats>) is pod evidence, and a pod game
				// does not evidence a card: stage 1's support rule is
				// unchanged, so it only feeds podEvidenced.
				s.evidenced[r.name] = true
				continue
			}
			if n, err := strconv.Atoi(strings.TrimSpace(r.col)); err == nil && n >= 3 {
				s.podEvidenced[r.name] = true
			}
		}
	}
	return s, nil
}

// firstFace is a split card's left face name ("Cease" of "Cease // Desist"), or "" for a
// one-name card. Coverage rows, and so the evidence lists written from them, name a card
// by its first face, so a split card is found under either spelling.
func firstFace(name string) string {
	if i := strings.Index(name, " // "); i > 0 {
		return name[:i]
	}
	return ""
}

// evidencedFor reports whether an evidence list names the card, by its full name or, for
// a split card, its first face.
func (s *supportLists) evidencedFor(name string) bool {
	if s.evidenced[name] {
		return true
	}
	f := firstFace(name)
	return f != "" && s.evidenced[f]
}

// podEvidencedFor reports whether an evidence list names the card with a seats
// field of three or more — the card has a clean pod game — by its full name or,
// for a split card, its first face.
func (s *supportLists) podEvidencedFor(name string) bool {
	if s.podEvidenced[name] {
		return true
	}
	f := firstFace(name)
	return f != "" && s.podEvidenced[f]
}

// refusedFor is the refusal naming the card, by its full name or, for a split card, its
// first face.
func (s *supportLists) refusedFor(name string) *cardset.RefusedEntry {
	if e := s.refused[name]; e != nil {
		return e
	}
	if f := firstFace(name); f != "" {
		return s.refused[f]
	}
	return nil
}

// embeddedSupport is this binary's lists: like the pool inputs, a build never reads
// them from the working tree, so tables match their key.
var embeddedSupport = sync.OnceValues(func() (*supportLists, error) { return readSupport(gorge.TableSources()) })

// refuseByName records a refusal on each lowered card the lists name. A card that
// does not lower keeps its own refusal, which is the truer reason.
func refuseByName(lw *lowered, s *supportLists) {
	for _, c := range lw.cards {
		if e := s.refusedFor(c.Name); e != nil && len(c.Refusals) == 0 {
			c.Refusals = append(c.Refusals, refusal{Card: c.Name, Node: supportNode, Reason: e.Reason()})
		}
	}
}

// loweredOK reports whether c lowered and built: it is playable on a raw table.
func (c *lCard) loweredOK() bool {
	return len(c.Refusals) == 0 || c.Refusals[0].Node == supportNode
}

// statusOf is the one place a card's status is computed. buildPool has run, so c's
// refusals include refuseByName's. A token earns no evidence of its own: it is played
// through the card that makes it.
func statusOf(c *lCard, s *supportLists) (cardset.Status, string) {
	switch {
	case !c.loweredOK():
		return cardset.StatusNotLowered, "not lowered: " + c.Refusals[0].Reason
	case len(c.Refusals) > 0:
		return cardset.StatusRefused, c.Refusals[0].Reason
	case !c.Token && !s.evidencedFor(c.Name):
		return cardset.StatusUnverified, "unverified: in no evidence list"
	}
	return cardset.StatusSupported, ""
}

// CardSupport is one card's support as the support page computes it: statusOf
// after lowering and building, whether the lowering is two-seat only
// (kop.CardDef.TwoSeat), and LIRKeysFor's key for the same lowering.
// twoSeat-at-caller: SupportFor only reports the flag; cardaudit's run reads
// TwoSeat to rank 4-seat cards first.
type CardSupport struct {
	Status     cardset.Status
	TwoSeat    bool
	TwoSeatWhy []string // the reasons that set TwoSeat (lCard.TwoSeatWhy), for the Commander pod census
	LIRKey     string
	// Nodes is the IR node of each lowering refusal the card recorded, in the
	// order lowering recorded them ("A[0] SP$ Charm", "T[1] Mode=Phase", ...).
	Nodes []string
}

// SupportFor lowers and builds the named cards as TestSupportDocIsCurrent does
// and returns each card's support. A card that records a lowering refusal still
// has a LIR key, so a caller must read Status, not LIRKey, to know whether the
// card lowered.
func SupportFor(cardsDir string, names []string) (map[string]CardSupport, error) {
	if len(names) == 0 {
		return map[string]CardSupport{}, nil
	}
	p, err := loadCardNames(append([]string(nil), names...), cardsDir)
	if err != nil {
		return nil, err
	}
	lw := lowerPool(p)
	// The key is the pre-build block, as LIRKeysFor computes it: buildPool
	// appends build refusals, which dumpLIR would then hash.
	keys := LIRKeys(append(append([]*lCard(nil), lw.cards...), lw.tokens...))
	if _, _, _, err := buildPool(lw); err != nil {
		return nil, err
	}
	s, err := embeddedSupport()
	if err != nil {
		return nil, err
	}
	out := make(map[string]CardSupport, len(lw.cards))
	for _, c := range lw.cards {
		st, _ := statusOf(c, s)
		var nodes []string
		for _, r := range c.Refusals {
			nodes = append(nodes, r.Node)
		}
		out[c.Name] = CardSupport{Status: st, TwoSeat: c.TwoSeat, TwoSeatWhy: c.TwoSeatWhy, LIRKey: keys[c.Name], Nodes: nodes}
	}
	return out, nil
}

// parseRefusedRepro parses -repro: POOL:SURFACE:PAIR[:POLICY[:MULLIGANS]]. The seed is the
// evidence corpus's, so a game is named by its pair.
func parseRefusedRepro(s string) (cardset.RefusedRepro, error) {
	f := strings.Split(s, ":")
	bad := func(why string) (cardset.RefusedRepro, error) {
		return cardset.RefusedRepro{}, fmt.Errorf("-repro %q: %s; want POOL:SURFACE:PAIR[:POLICY[:MULLIGANS]]", s, why)
	}
	if len(f) < 3 || len(f) > 5 || f[0] == "" {
		return bad("wrong shape")
	}
	if f[1] != "autopay" && f[1] != "full" {
		return bad("surface is autopay or full")
	}
	pair, err := strconv.Atoi(f[2])
	if err != nil || pair < 0 {
		return bad("pair is a pair index")
	}
	r := cardset.RefusedRepro{Pool: f[0], Surface: f[1], Seed: 1, Pair: pair, Policy: "random", Mulligans: 1}
	if len(f) >= 4 {
		if !slices.Contains([]string{"random", "heuristic", "explore", "first"}, f[3]) {
			return bad("policy is random, heuristic, explore or first")
		}
		r.Policy = f[3]
	}
	if len(f) == 5 {
		if r.Mulligans, err = strconv.Atoi(f[4]); err != nil || r.Mulligans < 0 {
			return bad("mulligans is a count")
		}
	}
	return r, nil
}

func refusedPath(root, card string) string {
	return filepath.Join(root, filepath.FromSlash(cardset.RefusedDir), cardset.SupportSlug(card)+".json")
}

// refuseCard writes card's refused entry under root and takes it out of the evidence
// lists, which must not name a refused card. The entry records those lists, so
// unrefuseCard puts everything back as it was.
func refuseCard(root, card, class, why string, repro cardset.RefusedRepro) (string, error) {
	path := refusedPath(root, card)
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf("%s exists: -unrefuse it first", path)
	}
	if why == "" || strings.ContainsAny(why, "\r\n") {
		return "", errors.New("-why wants one line")
	}
	e := cardset.RefusedEntry{Card: card, Class: class, Why: why, Repro: repro}
	lists, err := filepath.Glob(filepath.Join(root, filepath.FromSlash(cardset.EvidenceDir), "*.txt"))
	if err != nil {
		return "", err
	}
	for _, list := range lists {
		b, err := os.ReadFile(list)
		if err != nil {
			return "", err
		}
		if out, found := cutLine(append([]byte{'\n'}, b...), card); found {
			if err := os.WriteFile(list, out[1:], 0o644); err != nil {
				return "", err
			}
			e.Evidence = append(e.Evidence, strings.TrimSuffix(filepath.Base(list), ".txt"))
		}
	}
	b, err := e.Marshal()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	return path, os.WriteFile(path, b, 0o644)
}

// cutLine removes the line "name" from b, which starts with a newline. The
// line may carry trailing tab-separated columns (an evidence row's seats
// field), which go with it.
func cutLine(b []byte, name string) ([]byte, bool) {
	before, after, found := bytes.Cut(b, []byte("\n"+name+"\n"))
	if !found {
		before, after, found = bytes.Cut(b, []byte("\n"+name+"\t"))
		if !found {
			return b, false
		}
		// The row carries columns: drop the rest of it, up to but not
		// including the newline that ends it, so no column is left behind as
		// a row of its own.
		if i := bytes.IndexByte(after, '\n'); i >= 0 {
			after = after[i+1:]
		} else {
			after = nil
		}
	}
	return append(append(before, '\n'), after...), true
}

// unrefuseCard deletes card's refused entry under root and restores the evidence rows
// refuseCard took. Every row already in the list keeps its trailing columns (an
// evidence row's seats field); only the restored card is inserted as a plain name,
// and the next pod run's landing records the card's pod games again.
func unrefuseCard(root, card string) error {
	path := refusedPath(root, card)
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var e cardset.RefusedEntry
	if err := json.Unmarshal(b, &e); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	restored := false
	for _, name := range e.Evidence {
		list := filepath.Join(root, filepath.FromSlash(cardset.EvidenceDir), name+".txt")
		old, err := os.ReadFile(list)
		if err != nil {
			return err
		}
		rows := parseCardRows(old)
		i, found := slices.BinarySearchFunc(rows, card, func(r cardRow, name string) int {
			return strings.Compare(r.name, name)
		})
		if !found {
			rows = slices.Insert(rows, i, cardRow{name: card})
		}
		var out strings.Builder
		for _, r := range rows {
			out.WriteString(r.name)
			if r.col != "" {
				out.WriteByte('\t')
				out.WriteString(r.col)
			}
			out.WriteByte('\n')
		}
		if err := os.WriteFile(list, []byte(out.String()), 0o644); err != nil {
			return err
		}
		restored = true
	}
	if restored {
		if err := legacyKeyIfNoRow(root, card); err != nil {
			return err
		}
	}
	return os.Remove(path)
}

// legacyKeyIfNoRow adds card to LEGACY.txt (kept sorted) when no evidence-keys sidecar has a
// row for it: TestEvidenceCoverageRatchet wants every evidenced card to have one or the other.
func legacyKeyIfNoRow(root, card string) error {
	rows, _, err := readCoverageRows(os.DirFS(root))
	if err != nil {
		return err
	}
	if _, ok := rows[card]; ok {
		return nil
	}
	path := filepath.Join(root, filepath.FromSlash(coverageLegacy))
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	var lines []string
	for _, l := range strings.Split(string(old), "\n") {
		if l = strings.TrimSpace(l); l != "" {
			lines = append(lines, l)
		}
	}
	i, found := slices.BinarySearch(lines, card)
	if found {
		return nil
	}
	lines = slices.Insert(lines, i, card)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

// runRefuse is kgen -refuse: cards is the corpus the name must be in.
func runRefuse(cards, card, class, why, repro string, w io.Writer) error {
	r, err := parseRefusedRepro(repro)
	if err != nil {
		return err
	}
	if _, err := loadCardNames([]string{card}, cards); err != nil {
		return err
	}
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	path, err := refuseCard(root, card, class, why, r)
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "kgen: refused %s in %s: make dirac-tables, then rebuild what plays it\n", card, path)
	return nil
}

// runUnrefuse is kgen -unrefuse.
func runUnrefuse(card string, w io.Writer) error {
	root, err := moduleRoot()
	if err != nil {
		return err
	}
	if err := unrefuseCard(root, card); err != nil {
		return err
	}
	fmt.Fprintf(w, "kgen: %s is no longer refused: make dirac-tables, then rebuild what plays it\n", card)
	fmt.Fprintf(w, "kgen: unrefuse only when the defect is fixed: add the passing test and a row in dirac/support/unrefused.tsv (TestUnrefusalNeedsProof fails without one); if only the pinned game moved, re-record it with pool custom instead\n")
	return nil
}
