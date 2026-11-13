package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// copyScen is a generated-table scenario: player 0 controls Thousand-Year
// Storm and has cast one instant or sorcery this turn, so the next one is
// copied once as Storm's trigger resolves.
func copyScen(t *testing.T, deck []cardset.Entry, cfg Config) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, cfg)
	sc.perm(id["Thousand-Year Storm"], 0)
	sc.s.g.isCast[0] = 1
	return sc, id
}

// TestCopyMultiTargetAsk: a copy of Run Away Together ("two target
// creatures controlled by different players") is asked its new targets as
// one pick per object (gorge AskCopyTargets' Min 2 / Max 2 subset, one
// group per controller): the inherited targets first, then the other
// creatures; the second pick offers only the other controller's; at the
// maximum the answer closes and the copy resolves with the new pair. The ask
// comes inside Storm's trigger's resolution (CR 707.10c), so no player has
// passed over the copy.
func TestCopyMultiTargetAsk(t *testing.T) {
	deck := []cardset.Entry{{Name: "Thousand-Year Storm", Count: 1}, {Name: "Run Away Together", Count: 1},
		{Name: "Eager Trufflesnout", Count: 4}, {Name: "Island", Count: 10}}
	sc, id := copyScen(t, deck, Config{NoElide: true})
	s := sc.s
	sc.lands(0, id["Island"], 2)
	rat := sc.hand(id["Run Away Together"], 0)
	a0 := sc.perm(id["Eager Trufflesnout"], 0)
	a1 := sc.perm(id["Eager Trufflesnout"], 0)
	b0 := sc.perm(id["Eager Trufflesnout"], 1)
	b1 := sc.perm(id["Eager Trufflesnout"], 1)
	sc.do(t, AkCast, TCode(rat), scAny)
	sc.do(t, AkTarget, scAny, TCode(a0))
	sc.do(t, AkTarget, scAny, TCode(b0))
	// Storm's trigger on the cast; it resolves and puts the copy above
	d := passToCopyAsk(t, s)
	if d.Kind != DkTarget || d.Sub != SubCopyTarget || d.Player != 0 {
		t.Fatalf("want the copy's new-target ask, got kind %d sub %d player %d", d.Kind, d.Sub, d.Player)
	}
	if s.res.ask != raGenCopyTargets {
		t.Fatalf("ask %d, want the multi-object ask", s.res.ask)
	}
	if s.sp != 3 || s.stack[s.sp-2].kind != seTrigger || !s.isSpellCopy(s.stack[s.sp-1].obj) {
		t.Fatalf("stack at the ask: %+v, want the copy above the resolving trigger", s.stack[:s.sp])
	}
	// inherited a0, b0 first; no Done below the minimum of two
	if d.N != 4 || s.FeatCopyInherited(TCode(a0)) != 0 || s.FeatCopyInherited(TCode(b0)) != 1 || s.FeatCopyInherited(TCode(b1)) != -1 {
		t.Fatalf("first pick options %v, inherited positions %d %d %d", d.Options[:d.N],
			s.FeatCopyInherited(TCode(a0)), s.FeatCopyInherited(TCode(b0)), s.FeatCopyInherited(TCode(b1)))
	}
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkDone {
			t.Fatalf("Done offered below the minimum: %v", d.Options[:d.N])
		}
	}
	s.Apply(MakeAction(AkTarget, 0, TCode(b1), 0))
	d = s.Step()
	if d.Kind != DkTarget || d.N != 2 {
		t.Fatalf("second pick: kind %d options %v", d.Kind, d.Options[:d.N])
	}
	for i := 0; i < int(d.N); i++ {
		if c := d.Options[i].Target(); c != TCode(uint8(a0)) && c != TCode(uint8(a1)) {
			t.Fatalf("second pick offers %d (same controller as the first)", c)
		}
	}
	s.Apply(MakeAction(AkTarget, 0, TCode(a1), 0))
	passToCopyResolved(t, s)
	if sc.onBF(a1) || sc.onBF(b1) {
		t.Fatalf("the copy did not return its new targets")
	}
	if !sc.onBF(a0) || !sc.onBF(b0) {
		t.Fatalf("the copy returned an inherited target it did not keep")
	}
	if s.prio != s.active || s.passes != 0 {
		t.Fatalf("after the copy: priority %d passes %d, want the active player with none (CR 117.3b)", s.prio, s.passes)
	}
}

// passToCopyAsk passes priority until the copy's new-target ask.
func passToCopyAsk(t *testing.T, s *State) Decision {
	t.Helper()
	for i := 0; i < 8; i++ {
		d := s.Step()
		if d.Kind == DkTarget && d.Sub == SubCopyTarget {
			return d
		}
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d sub %d before the copy ask", d.Kind, d.Sub)
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("no copy ask")
	return Decision{}
}

// passToCopyResolved passes priority until only the original is on the stack,
// after a copy's election closed.
func passToCopyResolved(t *testing.T, s *State) {
	t.Helper()
	for i := 0; i < 8; i++ {
		d := s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d sub %d before the copy resolved", d.Kind, d.Sub)
		}
		if s.sp == 1 {
			return
		}
		s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("the copy did not resolve")
}

// TestCopyAbSyntheticNoDeclaration: an ability copy whose stack entry names a
// kernel-native ability (ab >= abCascade: cascade, storm, madness, delayed, ward,
// prowess) has no CardDef.Abilities entry, so copyDeclSlot reports no
// declaration (-1) instead of indexing the card's printed abilities. The live
// shape is Vantress Visions copying warp's delayed exile trigger (CR 702.185a,
// 603.7, 707.10): the copy is still minted and keeps its inherited targets
// (CR 707.10c), so only the new-target election is skipped.
func TestCopyAbSyntheticNoDeclaration(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	// A generated table (prepareGen is nil for the hand-written vanilla one).
	tbl, _, _, err := set.Matchup([]cardset.Entry{{Name: "Island", Count: 1}},
		[]cardset.Entry{{Name: "Island", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	// A card with a TgtCopyAb slot makes prepareGen set gen.anyCopyAb.
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Copy Ability", Types: kop.TInstant,
		Abilities: []kop.Ability{{
			Targets:  [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtCopyAb}},
			NTargets: 1,
		}}})
	one := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test One Trigger", Types: kop.TCreature,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Targets:  [kop.MaxTargets]kop.TargetSpec{{TFlags: kop.TgfDecl}},
			NTargets: 1}}})
	sc := newScenTableLib(t, tbl, 0, Config{})
	if sc.k.gen == nil || !sc.k.gen.anyCopyAb {
		t.Fatal("test table did not set anyCopyAb")
	}
	for _, ab := range []uint8{abCascade, abStorm, abMadness, abDelayed, abWard, abProwess} {
		e := &stackEntry{kind: seTrigger, card: one, ab: ab}
		if slot := sc.s.copyDeclSlot(sc.k, e); slot != -1 {
			t.Errorf("copyDeclSlot(ab %d) = %d, want -1 (a kernel-native ability has no declaration)", ab, slot)
		}
	}
	// A printed ability still reports its declaration (the guard is not a
	// blanket refusal to copy abilities).
	if slot := sc.s.copyDeclSlot(sc.k, &stackEntry{kind: seTrigger, card: one, ab: 0}); slot != 0 {
		t.Errorf("copyDeclSlot(printed ability) = %d, want 0", slot)
	}
}
