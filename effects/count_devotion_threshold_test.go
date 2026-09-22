package effects

// count_devotion_threshold_test.go pins the two Count$ heads the 2026-09-22
// repo-deck audit found unmodelled (both degraded to the dispatch's
// (0, false) fallthrough, so Cabal Ritual added NO mana and Aspect of Hydra
// pumped +0/+0):
//
//   - Count$Threshold.<yes>.<no> — CR 702.24's Threshold: active while the
//     resolving controller has seven or more cards in their graveyard.
//   - Count$Devotion.<Colour> — CR 700.5's devotion: the number of mana
//     symbols of that colour among the mana costs of the permanents the
//     resolving controller controls. Every printed pip spelling counts:
//     plain pips, two-colour hybrid ("GW"), monocolour hybrid ("2G"/"2/G"),
//     Phyrexian ("GP", CR 107.4f) and the compleated tricolour ("GUP").
//   - Count$DevotionDual.<A>.<B> — the two-colour sum; each hybrid symbol
//     counts once toward EACH of its colours.
//   - Count$Devotion.Chosen — the colour read off the source object's own
//     recorded colour choice (events.Choose's "color" fold); unchosen fails
//     closed to the unresolvable verdict, never a fake zero.

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// devotionBoard builds a two-seat game and places every fixture on seat 0's
// battlefield, returning the ids grouped by what the assertions need. The
// face-down card carries a GREEN printed cost on purpose: a face-down
// battlefield permanent has no mana cost (CR 708.5), so its printed face
// must contribute nothing to the count.
func devotionBoard(t *testing.T) (g *state.Game, src, forest state.ObjID) {
	t.Helper()
	g = state.NewGame([]string{"devout", "other"})
	mk := func(src string, facedown bool) state.ObjID {
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, 0)
		o.Zone = state.ZBattlefield
		o.FaceDown = facedown
		g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
		return o.ID
	}
	src = mk("Name:DevotionSource\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n", false)
	// The devotion corpus, one pip spelling each:
	mk("Name:Plains\nTypes:Basic Land Plains\nOracle:x\n", false)                                            // no mana cost: 0
	mk("Name:HybridElf\nManaCost:2 GW\nTypes:Creature Elf\nPT:2/2\nOracle:x\n", false)                       // {2}{G/W}: 1 green
	mk("Name:PhyxBear\nManaCost:GP\nTypes:Creature Bear\nPT:2/2\nOracle:x\n", false)                         // {G/P}: 1 green
	mk("Name:TwobridOgre\nManaCost:2G\nTypes:Creature Ogre\nPT:3/3\nOracle:x\n", false)                      // {2/G}: 1 green
	mk("Name:CompleatedTamiyo\nManaCost:2 G GUP U\nTypes:Planeswalker Tamiyo\nLoyalty:5\nOracle:x\n", false) // {G/U/P}: 1 green + 1 blue
	mk("Name:BlackBeetle\nManaCost:1 B B\nTypes:Creature Insect\nPT:1/1\nOracle:x\n", false)                 // 0 green
	forest = mk("Name:Forest\nTypes:Basic Land Forest\nOracle:x\n", false)
	mk("Name:FacedownElf\nManaCost:G\nTypes:Creature Elf\nPT:1/1\nOracle:x\n", true) // face down: 0
	return g, src, forest
}

// TestDevotionCountsEveryPrintedPipSpelling is the count leaf over the
// devotion corpus above: five green pips, one blue pip, zero red. The
// preconditions (every fixture on seat 0's battlefield, the face-down marker
// set) are asserted first so a vacuous setup fails loudly.
func TestDevotionCountsEveryPrintedPipSpelling(t *testing.T) {
	g, src, forest := devotionBoard(t)
	if len(g.Zone(state.ZBattlefield, 0)) != 9 {
		t.Fatalf("fixture broken: seat 0 battlefield holds %d objects, want 9", len(g.Zone(state.ZBattlefield, 0)))
	}
	if fd := g.Obj(g.Zone(state.ZBattlefield, 0)[8]); fd == nil || !fd.FaceDown || fd.Zone != state.ZBattlefield {
		t.Fatalf("face-down fixture broken: %+v", fd)
	}
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Source: src}

	if got, ok := EvalCountOK(h, c, "Count$Devotion.Green"); !ok || got != 6 {
		t.Fatalf("Count$Devotion.Green = (%d, %v), want (6, true) — source pip 1 + hybrid 1 + Phyrexian 1 + twobrid 1 + compleated 2, Forest 0, face-down 0", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$Devotion.Blue"); !ok || got != 2 {
		t.Fatalf("Count$Devotion.Blue = (%d, %v), want (2, true) — the compleated cost's {G/U/P} and {U}", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$Devotion.Red"); !ok || got != 0 {
		t.Fatalf("Count$Devotion.Red = (%d, %v), want (0, true) — a modelled head that counts zero", got, ok)
	}
	// An unknown colour is not a colour this build can count: the
	// unresolvable verdict, not a fake zero.
	if got, ok := EvalCountOK(h, c, "Count$Devotion.Purple"); ok {
		t.Fatalf("Count$Devotion.Purple = (%d, %v), want unresolvable", got, ok)
	}
	// The player scoping: seat 1's board is empty, so the same head from a
	// seat-1 controller counts 0 — the read is the resolving controller's
	// permanents, not the whole battlefield.
	if got, ok := EvalCountOK(h, &Ctx{Controller: 1, Source: src}, "Count$Devotion.Green"); !ok || got != 0 {
		t.Fatalf("seat-1 Count$Devotion.Green = (%d, %v), want (0, true)", got, ok)
	}
	_ = forest
}

// TestDevotionChosenReadsTheRecordedChoice pins the Chosen spelling on the
// source's own colour choice: with one recorded ("G", the letter the colour
// ask records) it counts that colour; with none recorded it is UNRESOLVABLE,
// never a fake zero.
func TestDevotionChosenReadsTheRecordedChoice(t *testing.T) {
	g, src, _ := devotionBoard(t)
	h := &fakeHost{g: g}
	if got, ok := EvalCountOK(h, &Ctx{Controller: 0, Source: src}, "Count$Devotion.Chosen"); ok {
		t.Fatalf("Count$Devotion.Chosen with no recorded choice = (%d, %v), want unresolvable", got, ok)
	}
	g.Obj(src).ChosenColor = "G"
	if got, ok := EvalCountOK(h, &Ctx{Controller: 0, Source: src}, "Count$Devotion.Chosen"); !ok || got != 6 {
		t.Fatalf("Count$Devotion.Chosen after choosing Green = (%d, %v), want (6, true)", got, ok)
	}
	// A full colour word records just as well (the ask may store either
	// spelling) and normalizes to the same letter.
	g.Obj(src).ChosenColor = "blue"
	if got, ok := EvalCountOK(h, &Ctx{Controller: 0, Source: src}, "Count$Devotion.Chosen"); !ok || got != 2 {
		t.Fatalf("Count$Devotion.Chosen after choosing Blue = (%d, %v), want (2, true)", got, ok)
	}
}

// TestDevotionDualSumsBothColours pins the two-colour head: two {B/R}
// hybrids (each counts once for EACH colour) plus a plain {B} creature gives
// 3 black + 2 red = 5; an unreadable colour is unresolvable.
func TestDevotionDualSumsBothColours(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	mk := func(src string) {
		t.Helper()
		c, d := cards.ParseBytes("t.txt", []byte(src))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		for _, f := range c.Faces {
			f.ApplyIntrinsics()
		}
		o := g.AddObject(c, 0)
		o.Zone = state.ZBattlefield
		g.SetZone(state.ZBattlefield, 0, append(g.Zone(state.ZBattlefield, 0), o.ID))
	}
	mk("Name:HybridOne\nManaCost:B R\nTypes:Creature Horror\nPT:2/2\nOracle:x\n")  // {B}{R}: 1+1
	mk("Name:HybridTwo\nManaCost:2 BR\nTypes:Creature Horror\nPT:2/2\nOracle:x\n") // {B/R}: 1+1
	mk("Name:PlainBlack\nManaCost:1 B\nTypes:Creature Rat\nPT:2/1\nOracle:x\n")    // {B}: 1
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Source: g.Zone(state.ZBattlefield, 0)[0]}
	if len(g.Zone(state.ZBattlefield, 0)) != 3 {
		t.Fatalf("fixture broken: %d battlefield objects, want 3", len(g.Zone(state.ZBattlefield, 0)))
	}
	// Each hybrid's B/R pips plus the plain black: 3 black, 2 red.
	if got, ok := EvalCountOK(h, c, "Count$DevotionDual.Black.Red"); !ok || got != 5 {
		t.Fatalf("Count$DevotionDual.Black.Red = (%d, %v), want (5, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$Devotion.Black"); !ok || got != 3 {
		t.Fatalf("Count$Devotion.Black = (%d, %v), want (3, true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "Count$DevotionDual.Black.Purple"); ok {
		t.Fatalf("Count$DevotionDual.Black.Purple = (%d, %v), want unresolvable", got, ok)
	}
}

// TestThresholdBranchFlipsAtSevenGraveyardCards pins CR 702.24's boundary:
// six cards in the graveyard is NOT Threshold (the <no> branch), seven is
// (the <yes> branch), and the branch tokens are read literally. The
// precondition (exactly 6 then 7 cards) is asserted before each read.
func TestThresholdBranchFlipsAtSevenGraveyardCards(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	src := state.ObjID(0)
	// The resolving source is seat 0's spell on the stack; the counted
	// graveyard is seat 0's.
	o := g.AddObject(cardOf(t, "Name:Cabal Ritual\nManaCost:1 B\nTypes:Instant\nOracle:x\n"), 0)
	o.Zone = state.ZStack
	g.Stack = append(g.Stack, o.ID)
	src = o.ID
	fill := func(n int) {
		t.Helper()
		c, d := cards.ParseBytes("t.txt", []byte("Name:Fodder\nTypes:Creature\nOracle:x\n"))
		if len(d) != 0 {
			t.Fatalf("diags: %v", d)
		}
		c.Link()
		var ids []state.ObjID
		for i := 0; i < n; i++ {
			oo := g.AddObject(c, 0)
			oo.Zone = state.ZGraveyard
			ids = append(ids, oo.ID)
		}
		g.SetZone(state.ZGraveyard, 0, ids)
	}
	h := &fakeHost{g: g}
	c := &Ctx{Controller: 0, Source: src}

	fill(6)
	if got := len(g.Zone(state.ZGraveyard, 0)); got != 6 {
		t.Fatalf("fixture broken: graveyard holds %d cards, want 6", got)
	}
	if got, ok := EvalCountOK(h, c, "Count$Threshold.5.3"); !ok || got != 3 {
		t.Fatalf("Count$Threshold.5.3 with 6 in the graveyard = (%d, %v), want (3, true)", got, ok)
	}
	fill(7)
	if got := len(g.Zone(state.ZGraveyard, 0)); got != 7 {
		t.Fatalf("fixture broken: graveyard holds %d cards, want 7", got)
	}
	if got, ok := EvalCountOK(h, c, "Count$Threshold.5.3"); !ok || got != 5 {
		t.Fatalf("Count$Threshold.5.3 with 7 in the graveyard = (%d, %v), want (5, true)", got, ok)
	}
	// The boundary is "seven or more": eight stays on the yes branch.
	fill(8)
	if got, ok := EvalCountOK(h, c, "Count$Threshold.5.3"); !ok || got != 5 {
		t.Fatalf("Count$Threshold.5.3 with 8 in the graveyard = (%d, %v), want (5, true)", got, ok)
	}
	// Another controller's graveyard does not count.
	fill(0)
	var ids []state.ObjID
	for i := 0; i < 9; i++ {
		oo := g.AddObject(cardOf(t, "Name:Fodder\nTypes:Creature\nOracle:x\n"), 1)
		oo.Zone = state.ZGraveyard
		ids = append(ids, oo.ID)
	}
	g.SetZone(state.ZGraveyard, 1, ids)
	if got, ok := EvalCountOK(h, c, "Count$Threshold.5.3"); !ok || got != 3 {
		t.Fatalf("Count$Threshold.5.3 with 9 in seat 1's graveyard = (%d, %v), want (3, true)", got, ok)
	}
}

// cardOf is the effects-package twin of rules' card(): parse, link, apply
// intrinsics.
func cardOf(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, d := cards.ParseBytes("t.txt", []byte(src))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}
