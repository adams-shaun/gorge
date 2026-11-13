package kgen

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// poolDecks is where a named pool's decks live: $GORGE_POOLS/<name>/decks, else
// .pools/<name>/decks at the module root found by walking up from the working
// directory to go.mod. "" when there is neither. It is dirac/oracle's PoolDecks
// without the per-pool env, because kgen does not import the oracle.
func poolDecks(name string) string {
	root := os.Getenv("GORGE_POOLS")
	if root == "" {
		dir, err := os.Getwd()
		if err != nil {
			return ""
		}
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				root = filepath.Join(dir, ".pools")
				break
			}
			up := filepath.Dir(dir)
			if up == dir {
				return ""
			}
			dir = up
		}
	}
	return filepath.Join(root, name, "decks")
}

// The generator's inputs: the compiled corpus (gitignored; link it into the
// worktree as .cards) and the FDN deck pool (KGEN_DECKS, else poolDecks). Both
// are overridable.
func testInputs(t *testing.T) (decks, cardsDir string) {
	t.Helper()
	decks = os.Getenv("KGEN_DECKS")
	if decks == "" {
		decks = poolDecks("fdn")
	}
	cardsDir = os.Getenv("KGEN_CARDS")
	if cardsDir == "" {
		var err error
		if cardsDir, err = cardset.CorpusDir(); err != nil {
			t.Skipf("kgen input missing (%v): link the corpus as .cards here", err)
		}
	}
	for _, d := range []string{decks, cardsDir} {
		if _, err := os.Stat(d); err != nil {
			t.Skipf("kgen input %s missing (%v): link the corpus as .cards and set GORGE_POOLS or KGEN_DECKS", d, err)
		}
	}
	return decks, cardsDir
}

var (
	poolOnce sync.Once
	poolVal  *pool
	poolErr  error
)

func testPool(t *testing.T) *pool {
	t.Helper()
	decks, cardsDir := testInputs(t)
	poolOnce.Do(func() { poolVal, poolErr = loadPool(decks, cardsDir) })
	if poolErr != nil {
		t.Fatal(poolErr)
	}
	return poolVal
}

// TestStaticRecordsMatchIR checks every pool card's static record (cost,
// types, P/T, keyword mask, colours) against the compiled IR, derived here
// independently of the lowering.
func TestStaticRecordsMatchIR(t *testing.T) {
	p := testPool(t)
	set := cardsettest.Set(t, "fdn")
	if len(p.names) != set.NumCards() {
		t.Fatalf("pool has %d cards, table %d", len(p.names), set.NumCards())
	}
	for i, n := range p.names {
		if set.Names()[i] != n {
			t.Fatalf("pool index %d: table %q, pool %q", i, set.Names()[i], n)
		}
		f := p.byName[n].Faces[0]
		d := set.Def(i)
		// Cost.
		var want kop.ManaCost
		for _, tok := range strings.Fields(f.ManaCost) {
			if v, err := strconv.Atoi(tok); err == nil {
				want.Generic += uint8(v)
				continue
			}
			switch tok {
			case "X":
				want.X++
			case "no", "cost":
			default:
				a, b := -1, -1
				if len(tok) == 2 {
					a, b = strings.IndexByte("WUBRG", tok[0]), strings.IndexByte("WUBRG", tok[1])
				}
				if i := strings.Index("WUBRGC", tok); len(tok) == 1 && i >= 0 {
					want.Pips[i]++
				} else if a >= 0 && b >= 0 && a != b && want.Hyb[1] == 0 {
					j := 0
					if want.Hyb[0] != 0 {
						j = 1
					}
					want.Hyb[j] = kop.ColorMask(1<<a | 1<<b)
					if a > b { // printed out of WUBRG order: gorge announces A first
						want.HybOrd |= uint8(1) << j
					}
				} else {
					want.Pips[kop.C] = 99 // hybrid: no kop form; the card must be refused
				}
			}
		}
		if want.Pips[kop.C] == 99 {
			if set.Supported()[i] {
				t.Errorf("%s: hybrid cost but supported", n)
			}
		} else if d.Cost != want {
			t.Errorf("%s: cost %+v, IR %q", n, d.Cost, f.ManaCost)
		}
		// Types.
		var tm kop.TypeMask
		for _, w := range f.Types {
			switch w {
			case "Land":
				tm |= kop.TLand
			case "Creature":
				tm |= kop.TCreature
			case "Artifact":
				tm |= kop.TArtifact
			case "Enchantment":
				tm |= kop.TEnchantment
			case "Planeswalker":
				tm |= kop.TPlaneswalker
			case "Instant":
				tm |= kop.TInstant
			case "Sorcery":
				tm |= kop.TSorcery
			case "Battle":
				tm |= kop.TBattle
			case "Legendary":
				tm |= kop.TLegendary
			case "Basic":
				tm |= kop.TBasic
			case "Aura":
				tm |= kop.TAura
			case "Equipment":
				tm |= kop.TEquipment
			}
		}
		if d.Types != tm {
			t.Errorf("%s: types %#x, IR %v (%#x)", n, d.Types, f.Types, tm)
		}
		// P/T and loyalty.
		if int(d.Power) != f.Power() || int(d.Toughness) != f.Toughness() {
			if !strings.Contains(f.PT, "*") {
				t.Errorf("%s: P/T %d/%d, IR %q", n, d.Power, d.Toughness, f.PT)
			}
		}
		if f.Loyalty != "" && strconv.Itoa(int(d.Loyalty)) != f.Loyalty {
			t.Errorf("%s: loyalty %d, IR %q", n, d.Loyalty, f.Loyalty)
		}
		// Colours: the coloured symbols of the printed cost.
		var cm kop.ColorMask
		for _, r := range f.ManaCost {
			if i := strings.IndexRune("WUBRG", r); i >= 0 {
				cm |= 1 << i
			}
		}
		if d.Colors != cm {
			t.Errorf("%s: colours %#x, IR cost %q", n, d.Colors, f.ManaCost)
		}
		// Keywords: the printed simple keywords, plus the keyword-shaped
		// expansions the generator folds back (only on supported cards).
		var km kop.KwMask
		for _, k := range f.Keywords {
			head, param, _ := strings.Cut(k, ":")
			switch {
			case head == "Hexproof" && param == "Instant":
				km |= kop.KwHexproofFromInstants
			case head == "Flash":
				km |= kop.KwFlash
			case head == "Kicker":
				km |= kop.KwKicker
			case head == "Flashback":
				km |= kop.KwFlashback
			case head == "Ward" && set.Supported()[i]:
				km |= kop.KwWard
			case head == "Prowess" && set.Supported()[i]:
				km |= kop.KwProwess
			default:
				if b, ok := kwBits[head]; ok && param == "" {
					km |= b
				}
			}
		}
		if d.Keywords != km {
			t.Errorf("%s: keywords %#x, IR %v (%#x)", n, d.Keywords, f.Keywords, km)
		}
	}
}

// TestMatchupsBuild assembles a deterministic sample of matchups from the
// fully supported decks and checks the tables.
func TestMatchupsBuild(t *testing.T) {
	heavyParallel(t)
	p := testPool(t)
	var ok [][]string
	set := cardsettest.Set(t, "fdn")
	for _, d := range p.decks {
		good := true
		for _, n := range d {
			i, found := set.Lookup(n)
			if !found || !set.Supported()[i] {
				good = false
				break
			}
		}
		if good {
			ok = append(ok, d)
		}
	}
	if len(ok) == 0 {
		t.Fatal("no supported decks")
	}
	entries := func(d []string) []cardset.Entry {
		var out []cardset.Entry
		for _, n := range d {
			out = append(out, cardset.Entry{Name: n, Count: 1})
		}
		return out
	}
	maxSubs, maxCards := 0, 0
	const pairs = 400
	for k := 0; k < pairs; k++ {
		a := ok[(k*7919)%len(ok)]
		b := ok[(k*104729+13)%len(ok)]
		tbl, da, db, err := set.Matchup(entries(a), entries(b))
		if err != nil {
			t.Fatalf("matchup %d: %v", k, err)
		}
		if len(da) != len(a) || len(db) != len(b) {
			t.Fatalf("matchup %d: deck sizes %d/%d", k, len(da), len(db))
		}
		for _, id := range append(da, db...) {
			if int(id) >= len(tbl.Cards) || tbl.Cards[id].Name == "" {
				t.Fatalf("matchup %d: card id %d", k, id)
			}
		}
		for _, c := range tbl.Cards {
			for _, ab := range c.Abilities {
				for _, in := range ab.Prog {
					if (in.Op == kop.OpCreateToken || in.Op == kop.OpCreateTokenEx) && (int(in.C) >= len(tbl.Cards) || tbl.Cards[in.C].Types&kop.TToken == 0) {
						t.Fatalf("matchup %d: %s creates card %d which is not a token", k, c.Name, in.C)
					}
				}
			}
		}
		if len(tbl.Subtypes) > maxSubs {
			maxSubs = len(tbl.Subtypes)
		}
		if len(tbl.Cards) > maxCards {
			maxCards = len(tbl.Cards)
		}
	}
	t.Logf("%d supported decks; %d matchups built; max %d cards, %d subtypes", len(ok), pairs, maxCards, maxSubs)
	// Every supported deck alone: its subtype count bounds any pairing
	// (the union of two decks is at most the sum).
	maxDeck := 0
	for _, d := range ok {
		tbl, _, _, err := set.Matchup(entries(d), nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(tbl.Subtypes) > maxDeck {
			maxDeck = len(tbl.Subtypes)
		}
	}
	t.Logf("max subtypes in one deck's table: %d (pairs stay within 64 while this is <= 32)", maxDeck)
	if maxDeck > 32 {
		t.Errorf("a single deck needs %d subtypes: some pairings may exceed 64", maxDeck)
	}
}

// TestOpsDocCoversAppendedOps fails when an op appended after the core's
// vocabulary has no OPS.md entry.
func TestOpsDocCoversAppendedOps(t *testing.T) {
	have := map[string]bool{}
	for _, e := range opsEntries {
		have[e.label] = true
	}
	for o := kop.OpSkipUnless; o < kop.OpHighEnd; o++ {
		if o.Known() && !have["op:"+o.String()] {
			t.Errorf("op %s has no OPS.md entry (dirac/kgen/opsdoc.go)", o)
		}
	}
}

func TestCardListPool(t *testing.T) {
	_, cardsDir := testInputs(t)
	path := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(path, []byte("# test list\nAbrade\n\nMountain\nAbrade\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(path, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"Abrade", "Mountain"}; strings.Join(p.names, ",") != strings.Join(want, ",") {
		t.Fatalf("names %v, want %v", p.names, want)
	}
	if len(p.decks) != 0 {
		t.Fatalf("card-list pool has %d decks", len(p.decks))
	}
	for _, n := range p.names {
		if p.nDecks[n] != 1 || p.byName[n] == nil {
			t.Fatalf("%s: nDecks %d, byName %v", n, p.nDecks[n], p.byName[n] != nil)
		}
	}
}
