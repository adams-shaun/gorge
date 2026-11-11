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
}

func readSupport(src fs.FS) (*supportLists, error) {
	entries, err := cardset.ReadRefused(src)
	if err != nil {
		return nil, err
	}
	s := &supportLists{refused: map[string]*cardset.RefusedEntry{}, evidenced: map[string]bool{}}
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
		for _, n := range parseCardList(b) {
			s.evidenced[n] = true
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

// cutLine removes the line "name\n" from b, which starts with a newline.
func cutLine(b []byte, name string) ([]byte, bool) {
	before, after, found := bytes.Cut(b, []byte("\n"+name+"\n"))
	if !found {
		return b, false
	}
	return append(append(before, '\n'), after...), true
}

// unrefuseCard deletes card's refused entry under root and restores the evidence rows
// refuseCard took.
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
	for _, name := range e.Evidence {
		list := filepath.Join(root, filepath.FromSlash(cardset.EvidenceDir), name+".txt")
		old, err := os.ReadFile(list)
		if err != nil {
			return err
		}
		lines := parseCardList(old)
		if i, found := slices.BinarySearch(lines, card); !found {
			lines = slices.Insert(lines, i, card)
		}
		if err := os.WriteFile(list, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
			return err
		}
	}
	return os.Remove(path)
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
