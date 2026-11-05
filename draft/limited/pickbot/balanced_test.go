package pickbot_test

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/build"
	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
)

// wu reports whether a card's colour identity is inside white-blue.
func wu(c *cards.Card) bool { return c.ColourIdentity()&^0b00011 == 0 }

// bucketOf mirrors pickbot's curveBucket for the tests that measure a curve.
func bucketOf(mv int) int {
	switch {
	case mv <= 1:
		return 0
	case mv >= 6:
		return 5
	default:
		return mv - 1
	}
}

// fdnPick returns the first n FDN sheet names that resolve, are not lands,
// carry a raw rating and pass keep, sorted by GIH (descending unless
// ascending) and then by name.
func fdnPick(t *testing.T, reg *cards.Registry, keep func(c *cards.Card, gih float64) bool, ascending bool, n int) []string {
	t.Helper()
	type pick struct {
		name string
		gih  float64
	}
	var picks []pick
	for _, name := range fdnMainSetNames(t) {
		c, ok := reg.Lookup(name)
		if !ok || c == nil || len(c.Faces) == 0 {
			continue
		}
		if c.Faces[0].IsLand() {
			continue
		}
		e, ok := pb.RawRating("FDN", name)
		if !ok {
			continue
		}
		if keep != nil && !keep(c, e.GihWr) {
			continue
		}
		picks = append(picks, pick{name: name, gih: e.GihWr})
	}
	sort.SliceStable(picks, func(i, j int) bool {
		if picks[i].gih != picks[j].gih {
			if ascending {
				return picks[i].gih < picks[j].gih
			}
			return picks[i].gih > picks[j].gih
		}
		return picks[i].name < picks[j].name
	})
	if len(picks) > n {
		picks = picks[:n]
	}
	out := make([]string, len(picks))
	for i, p := range picks {
		out[i] = p.name
	}
	return out
}

// fdnPool turns names into CardRefs with the FDN sheet rarities.
func fdnPool(t *testing.T, reg *cards.Registry, names []string) []limited.CardRef {
	t.Helper()
	spec, err := setdata.Load("FDN")
	if err != nil {
		t.Fatalf("setdata FDN: %v", err)
	}
	rarity := sheetRarities(t, spec)
	pool := make([]limited.CardRef, 0, len(names))
	for i, name := range names {
		lookupCard(t, reg, name)
		r, ok := rarity[name]
		if !ok {
			t.Fatalf("%q is not on any FDN sheet", name)
		}
		pool = append(pool, limited.CardRef{
			ID: "b" + itoa(i), Name: name, Set: "FDN", Number: "1", Rarity: r,
		})
	}
	return pool
}

// balancedTypePool builds the pool TestBalancedPlaysEveryTypeItCan uses: 25
// white-blue creatures plus two of each of instant, sorcery, noncreature
// enchantment and noncreature artifact.
func balancedTypePool(t *testing.T, reg *cards.Registry) (pool []limited.CardRef, typeRatings []float64, creatureCut float64) {
	t.Helper()
	creatures := fdnPick(t, reg, func(c *cards.Card, gih float64) bool {
		return wu(c) && c.Faces[0].IsCreature()
	}, false, 25)
	if len(creatures) != 25 {
		t.Fatalf("FDN has %d white-blue rated creatures, want 25", len(creatures))
	}
	creatureCut = 1
	for i, name := range creatures {
		e, _ := pb.RawRating("FDN", name)
		if i == 22 {
			creatureCut = e.GihWr
		}
	}
	names := append([]string{}, creatures...)
	filters := []func(c *cards.Card) bool{
		func(c *cards.Card) bool { return c.Faces[0].IsInstant() },
		func(c *cards.Card) bool { return c.Faces[0].IsSorcery() },
		func(c *cards.Card) bool {
			f := c.Faces[0]
			return f.IsEnchantment() && !f.IsCreature()
		},
		func(c *cards.Card) bool {
			f := c.Faces[0]
			return f.IsArtifact() && !f.IsCreature()
		},
	}
	for _, isType := range filters {
		picks := fdnPick(t, reg, func(c *cards.Card, gih float64) bool {
			return wu(c) && isType(c) && gih >= 0.46
		}, true, 2)
		if len(picks) != 2 {
			t.Fatalf("FDN yields %d rated white-blue cards of a quota type, want 2", len(picks))
		}
		for _, name := range picks {
			e, _ := pb.RawRating("FDN", name)
			if e.GihWr >= creatureCut {
				t.Fatalf("quota card %q rates %.4f, not below the 23rd creature's %.4f", name, e.GihWr, creatureCut)
			}
			typeRatings = append(typeRatings, e.GihWr)
		}
		names = append(names, picks...)
	}
	return fdnPool(t, reg, names), typeRatings, creatureCut
}

// TestBalancedPlaysEveryTypeItCan pins the type quotas: on a pool whose
// best 23 are all creatures, Balanced still plays two of each spell type
// while keeping the creature band, where Best23 plays none of them.
func TestBalancedPlaysEveryTypeItCan(t *testing.T) {
	reg := corpus(t)
	pool, _, _ := balancedTypePool(t, reg)
	b := &pb.Balanced{Lookup: lookupOf(reg)}

	deck := b.Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	if total := deck.Total(); total != 40 {
		t.Fatalf("built a %d-card deck, want 40", total)
	}
	if len(deck.Main) != 23 {
		t.Fatalf("built deck maindecks %d cards, want 23", len(deck.Main))
	}

	counts := map[string]int{}
	for _, id := range deck.Main {
		f := lookupCard(t, reg, poolName(t, pool, id)).Faces[0]
		switch {
		case f.IsCreature():
			counts["creature"]++
		case f.IsInstant():
			counts["instant"]++
		case f.IsSorcery():
			counts["sorcery"]++
		case f.IsEnchantment():
			counts["enchantment"]++
		case f.IsArtifact():
			counts["artifact"]++
		}
	}
	for _, class := range []string{"instant", "sorcery", "enchantment", "artifact"} {
		if counts[class] < 2 {
			t.Errorf("deck plays %d %ss, want at least 2", counts[class], class)
		}
	}
	if counts["creature"] > 18 {
		t.Errorf("deck plays %d creatures, want at most 18", counts["creature"])
	}

	off := (&pb.Best23{Lookup: lookupOf(reg)}).Build("FDN", pool)
	instants := 0
	for _, id := range off.Main {
		if lookupCard(t, reg, poolName(t, pool, id)).Faces[0].IsInstant() {
			instants++
		}
	}
	if instants != 0 {
		t.Fatalf("Best23 plays %d instants on this pool, want 0 so the test discriminates", instants)
	}
}

// TestBalancedCapsTheTopOfTheCurve pins the curve bands on an all-creature
// pool, where Best23's only curve rule (the 6+ cap of 3) is not enough.
func TestBalancedCapsTheTopOfTheCurve(t *testing.T) {
	reg := corpus(t)
	names := fdnPick(t, reg, func(c *cards.Card, gih float64) bool {
		return wu(c) && c.Faces[0].IsCreature()
	}, false, 1000)
	pool := fdnPool(t, reg, names)

	counts := [6]int{}
	for _, name := range names {
		counts[bucketOf(int(lookupCard(t, reg, name).Faces[0].Cmc()))]++
	}
	capacity := 0
	for b := 0; b < 6; b++ {
		if counts[b] < pb.DefaultDeckShape.CurveMax[b] {
			capacity += counts[b]
		} else {
			capacity += pb.DefaultDeckShape.CurveMax[b]
		}
	}
	if capacity < 23 {
		t.Fatalf("pool curve capacity %d < 23: the build would have to relax the maxima", capacity)
	}

	s := pb.DefaultDeckShape
	s.CreatureMax = 23
	deck := (&pb.Balanced{Lookup: lookupOf(reg), Shape: s}).Build("FDN", pool)
	got := [6]int{}
	for _, id := range deck.Main {
		got[bucketOf(int(lookupCard(t, reg, poolName(t, pool, id)).Faces[0].Cmc()))]++
	}
	if got[1] < 5 {
		t.Errorf("deck plays %d two-drops, want at least 5", got[1])
	}
	if got[2] < 4 {
		t.Errorf("deck plays %d three-drops, want at least 4", got[2])
	}
	if got[3] > 5 {
		t.Errorf("deck plays %d four-drops, want at most 5", got[3])
	}
	if got[4] > 3 {
		t.Errorf("deck plays %d five-drops, want at most 3", got[4])
	}
	if got[5] > 2 {
		t.Errorf("deck plays %d six-plus-drops, want at most 2", got[5])
	}

	best23 := (&pb.Best23{Lookup: lookupOf(reg)}).Build("FDN", pool)
	off := [6]int{}
	for _, id := range best23.Main {
		off[bucketOf(int(lookupCard(t, reg, poolName(t, pool, id)).Faces[0].Cmc()))]++
	}
	if off[1] >= 5 && off[2] >= 4 && off[3] <= 5 && off[4] <= 3 && off[5] <= 2 {
		t.Fatalf("Best23 meets every curve bound on this pool (curve %v), so the test does not discriminate", off)
	}
}

// TestBalancedStaysInItsPair pins the pair: on the thin-pair pool, Balanced
// maindecks only white-green cards (26 cover 23 slots).
func TestBalancedStaysInItsPair(t *testing.T) {
	reg := corpus(t)
	pool := shortPairPool(t)
	deck := (&pb.Balanced{Lookup: lookupOf(reg)}).Build("FDN", pool)
	if len(deck.Main) != 23 {
		t.Fatalf("built deck maindecks %d cards, want 23", len(deck.Main))
	}
	outside := 0
	for _, id := range deck.Main {
		name := poolName(t, pool, id)
		card := lookupCard(t, reg, name)
		if card.ColourIdentity()&^(1<<0|1<<4) != 0 {
			outside++
			t.Logf("maindeck %q is outside white-green", name)
		}
	}
	if outside != 0 {
		t.Fatalf("built deck maindecks %d cards outside white-green, want 0", outside)
	}
}

// TestBalancedWithoutLookup pins the no-Lookup path: no facts means every
// card is classOther at mana value 0, so the phases relax and the deck is
// still 23 distinct in-pool IDs with no basics.
func TestBalancedWithoutLookup(t *testing.T) {
	pool := shortPairPool(t)
	deck := (&pb.Balanced{}).Build("FDN", pool)
	if len(deck.Main) != 23 {
		t.Fatalf("built deck maindecks %d cards, want 23", len(deck.Main))
	}
	if deck.Basics != ([5]int{}) {
		t.Fatalf("deck basics = %v, want all zero", deck.Basics)
	}
	inPool := map[string]bool{}
	for _, c := range pool {
		inPool[c.ID] = true
	}
	seen := map[string]bool{}
	for _, id := range deck.Main {
		if !inPool[id] {
			t.Fatalf("deck maindecks %q, not in the pool", id)
		}
		if seen[id] {
			t.Fatalf("deck maindecks %q twice", id)
		}
		seen[id] = true
	}
}

// TestBalancedIsDeterministic pins that two builds are identical.
func TestBalancedIsDeterministic(t *testing.T) {
	reg := corpus(t)
	pool, _, _ := balancedTypePool(t, reg)
	b := &pb.Balanced{Lookup: lookupOf(reg)}
	first := b.Build("FDN", pool)
	second := b.Build("FDN", pool)
	if len(first.Main) != len(second.Main) {
		t.Fatalf("builds differ in maindeck size: %d vs %d", len(first.Main), len(second.Main))
	}
	for i := range first.Main {
		if first.Main[i] != second.Main[i] {
			t.Fatalf("rebuild differs at card %d: %q vs %q", i, first.Main[i], second.Main[i])
		}
	}
	if first.Basics != second.Basics {
		t.Fatalf("rebuild differs in basics: %v vs %v", first.Basics, second.Basics)
	}
}

// TestDefaultDeckShape pins the default target's values and the arithmetic
// that keeps its minima, quotas and maindeck size compatible.
func TestDefaultDeckShape(t *testing.T) {
	want := pb.DeckShape{
		MainSize:    23,
		CurveMin:    [6]int{0, 5, 4, 3, 1, 0},
		CurveMax:    [6]int{3, 8, 7, 5, 3, 2},
		CreatureMin: 14,
		CreatureMax: 18,
		TypeMin:     [8]int{0, 0, 2, 2, 2, 2, 1, 1},
		QuotaFloor:  -10,

		SplashMax:     3,
		SplashSources: 3,
		DualMax:       4,
		ThinMainSize:  22,
	}
	got := pb.DefaultDeckShape
	if got != want {
		t.Fatalf("DefaultDeckShape = %+v, want %+v", got, want)
	}
	minSum, maxSum := 0, 0
	for b := 0; b < 6; b++ {
		minSum += got.CurveMin[b]
		maxSum += got.CurveMax[b]
	}
	if minSum > got.MainSize || got.MainSize > maxSum {
		t.Fatalf("curve minima %d / maxima %d do not contain MainSize %d", minSum, maxSum, got.MainSize)
	}
	if got.CreatureMin+2*4 > got.MainSize {
		t.Fatalf("creature floor %d plus four type quotas exceeds MainSize %d", got.CreatureMin, got.MainSize)
	}
}

// monoOf returns the first n FDN nonland cards whose colour identity is
// exactly colour bit c, restricted by pips (the pip count of colour c).
func monoOf(t *testing.T, reg *cards.Registry, c uint, n int, pips func(p int) bool) []string {
	t.Helper()
	return fdnPick(t, reg, func(card *cards.Card, gih float64) bool {
		if card.ColourIdentity() != 1<<c {
			return false
		}
		p := 0
		for _, f := range card.Faces {
			p += int(cards.ManaCostChroma(f.ManaCost, uint8(1<<c)))
		}
		return pips(p)
	}, false, n)
}

func anyPips(int) bool   { return true }
func onePip(p int) bool  { return p == 1 }
func twoPlus(p int) bool { return p >= 2 }

// wuPool is nW mono-white plus nU mono-blue nonland cards, then extra names.
func wuPool(t *testing.T, reg *cards.Registry, nW, nU int) []string {
	t.Helper()
	w := monoOf(t, reg, 0, nW, anyPips)
	u := monoOf(t, reg, 1, nU, anyPips)
	if len(w) != nW || len(u) != nU {
		t.Fatalf("FDN yields %d white and %d blue cards, want %d and %d", len(w), len(u), nW, nU)
	}
	return append(w, u...)
}

// withLands appends land refs (one per name, IDs prefix+index) to a pool.
func withLands(pool []limited.CardRef, prefix string, names ...string) []limited.CardRef {
	for i, n := range names {
		pool = append(pool, limited.CardRef{ID: prefix + itoa(i), Name: n, Set: "FDN", Number: "1", Rarity: limited.Common})
	}
	return pool
}

// countNamed counts Main entries whose card is called name.
func countNamed(t *testing.T, pool []limited.CardRef, deck build.Deck, name string) int {
	t.Helper()
	n := 0
	for _, id := range deck.Main {
		if poolName(t, pool, id) == name {
			n++
		}
	}
	return n
}

func basicsTotal(d build.Deck) int {
	n := 0
	for _, b := range d.Basics {
		n += b
	}
	return n
}

// spellsOf counts the nonland Main entries and those of colour bit c.
func spellsOf(t *testing.T, reg *cards.Registry, pool []limited.CardRef, deck build.Deck, c uint) (spells, ofColour int) {
	t.Helper()
	for _, id := range deck.Main {
		card := lookupCard(t, reg, poolName(t, pool, id))
		if card.Faces[0].IsLand() {
			continue
		}
		spells++
		if card.ColourIdentity()&(1<<c) != 0 {
			ofColour++
		}
	}
	return
}

// TestBalancedSplashesAThinPair pins the splash: a 20-card white-blue pair
// takes the three one-red-pip cards and gives red three sources, and skips
// the black cards whose two pips no splash covers.
func TestBalancedSplashesAThinPair(t *testing.T) {
	reg := corpus(t)
	names := wuPool(t, reg, 10, 10)
	red := monoOf(t, reg, 3, 3, onePip)
	black := monoOf(t, reg, 2, 3, twoPlus)
	if len(red) != 3 || len(black) != 3 {
		t.Fatalf("FDN yields %d one-pip red and %d two-pip black cards, want 3 and 3", len(red), len(black))
	}
	pool := fdnPool(t, reg, append(append(names, red...), black...))

	deck := (&pb.Balanced{Lookup: lookupOf(reg)}).Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	spells, redN := spellsOf(t, reg, pool, deck, 3)
	_, blackN := spellsOf(t, reg, pool, deck, 2)
	if spells != 23 || redN != 3 || blackN != 0 {
		t.Fatalf("deck plays %d spells, %d red, %d black; want 23, 3, 0", spells, redN, blackN)
	}
	if deck.Basics[3] < 3 {
		t.Fatalf("red basics = %d (basics %v), want at least 3 sources", deck.Basics[3], deck.Basics)
	}
	if total := deck.Total(); total != 40 {
		t.Fatalf("built a %d-card deck, want 40", total)
	}

	// Without the splash (dpool-2's any-colour padding, which the zero
	// SplashMax and ThinMainSize turn back on) the precondition above fails.
	s := pb.DefaultDeckShape
	s.SplashMax, s.ThinMainSize = 0, 0
	off := (&pb.Balanced{Lookup: lookupOf(reg), Shape: s}).Build("FDN", pool)
	_, offBlack := spellsOf(t, reg, pool, off, 2)
	if off.Basics[3] >= 3 && offBlack == 0 {
		t.Fatalf("splash-off build also meets the assertions (basics %v), so the test does not discriminate", off.Basics)
	}
}

func twentySixPool(t *testing.T, reg *cards.Registry) []limited.CardRef {
	return fdnPool(t, reg, wuPool(t, reg, 13, 13))
}

// TestBalancedPlaysOnColourDuals pins the nonbasic scan: the white-blue
// dual is played, the blue-red and black-green ones are not.
func TestBalancedPlaysOnColourDuals(t *testing.T) {
	reg := corpus(t)
	pool := withLands(twentySixPool(t, reg), "land", "Tranquil Cove", "Swiftwater Cliffs", "Jungle Hollow")
	deck := (&pb.Balanced{Lookup: lookupOf(reg)}).Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	spells, _ := spellsOf(t, reg, pool, deck, 0)
	if spells != 23 || countNamed(t, pool, deck, "Tranquil Cove") != 1 ||
		countNamed(t, pool, deck, "Swiftwater Cliffs") != 0 || countNamed(t, pool, deck, "Jungle Hollow") != 0 {
		t.Fatalf("deck plays %d spells, Cove %d, Cliffs %d, Hollow %d; want 23, 1, 0, 0", spells,
			countNamed(t, pool, deck, "Tranquil Cove"), countNamed(t, pool, deck, "Swiftwater Cliffs"),
			countNamed(t, pool, deck, "Jungle Hollow"))
	}
	if basicsTotal(deck) != 16 || deck.Total() != 40 {
		t.Fatalf("basics %d, total %d; want 16, 40", basicsTotal(deck), deck.Total())
	}
	s := pb.DefaultDeckShape
	s.DualMax = 0
	off := (&pb.Balanced{Lookup: lookupOf(reg), Shape: s}).Build("FDN", pool)
	if len(off.Main) != 23 {
		t.Fatalf("dual-off build maindecks %d cards, want 23 so the test discriminates", len(off.Main))
	}
}

// TestBalancedCapsDuals pins DualMax: six Coves in the pool, four played.
func TestBalancedCapsDuals(t *testing.T) {
	reg := corpus(t)
	pool := withLands(twentySixPool(t, reg), "cove", "Tranquil Cove", "Tranquil Cove", "Tranquil Cove",
		"Tranquil Cove", "Tranquil Cove", "Tranquil Cove")
	deck := (&pb.Balanced{Lookup: lookupOf(reg)}).Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	if n := countNamed(t, pool, deck, "Tranquil Cove"); n != 4 {
		t.Fatalf("deck plays %d Coves, want 4", n)
	}
	if basicsTotal(deck) != 13 || deck.Total() != 40 {
		t.Fatalf("basics %d, total %d; want 13, 40", basicsTotal(deck), deck.Total())
	}
}

// TestBalancedPrefersAnEighteenthLand pins the thin-pair main size: 22
// in-pair spells and 18 basics beat a 23rd card that needs two black pips.
func TestBalancedPrefersAnEighteenthLand(t *testing.T) {
	reg := corpus(t)
	black := monoOf(t, reg, 2, 3, twoPlus)
	if len(black) != 3 {
		t.Fatalf("FDN yields %d two-pip black cards, want 3", len(black))
	}
	pool := fdnPool(t, reg, append(wuPool(t, reg, 11, 11), black...))
	deck := (&pb.Balanced{Lookup: lookupOf(reg)}).Build("FDN", pool)
	if err := build.Validate(pool, deck); err != nil {
		t.Fatalf("built deck invalid: %v", err)
	}
	spells, blackN := spellsOf(t, reg, pool, deck, 2)
	if spells != 22 || blackN != 0 || basicsTotal(deck) != 18 {
		t.Fatalf("deck plays %d spells, %d black, %d basics; want 22, 0, 18", spells, blackN, basicsTotal(deck))
	}
	s := pb.DefaultDeckShape
	s.ThinMainSize = 0
	off := (&pb.Balanced{Lookup: lookupOf(reg), Shape: s}).Build("FDN", pool)
	offSpells, offBlack := spellsOf(t, reg, pool, off, 2)
	if offSpells != 23 || offBlack != 1 {
		t.Fatalf("rule-off build plays %d spells, %d black; want 23, 1", offSpells, offBlack)
	}
}
