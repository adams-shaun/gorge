package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Scarab God's SetCreatureTypes copy exception ("except it's a 4/4 black
// Zombie", CR 707.9b; kop.Override.RemoveCreatureSub): the token's printed
// creature types go, its other subtypes stay, the named type joins, and a
// copied printed changeling keeps every creature type: its CDA runs in layer
// 4, after the copy exception (CR 613.3, 702.73a; gorge keeps it). Croaking Counterpart is the
// same form with SetPT; its pool's setlist carries no carrier, so the
// scenario uses The Scarab God, the card the gap's slice lowered with it.

// scarabTokenScen builds a two-seat mbc matchup, puts The Scarab God on the
// battlefield and the copied card in seat 1's graveyard, activates its
// {2}{U}{B} ability at the grave card, and resolves the stack. It returns
// the scenario, the copied card's CardID and the token's slot.
func scarabTokenScen(t *testing.T, copied string) (*scen, kop.CardID, Slot) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	for _, n := range []string{"The Scarab God", copied, "Island"} {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("precondition: %s does not play in the mbc pool (%v, supported %v)", n, ok, ok && set.Supported()[i])
		}
	}
	deck := []cardset.Entry{{Name: "The Scarab God", Count: 1}, {Name: copied, Count: 2}, {Name: "Island", Count: 20}}
	tbl, da, err := set.MatchupN([][]cardset.Entry{deck, deck})
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]kop.CardID{}
	for p := range da {
		for _, c := range da[p] {
			id[tbl.Cards[c].Name] = c
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	s := sc.s
	god := sc.perm(id["The Scarab God"], 0)
	victim := sc.grave(id[copied], 1, 1)[0]
	s.pl[0].pool[kop.U] = 2
	s.pl[0].pool[kop.B] = 2

	d := s.Step()
	var act Action
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(god) {
			act = a
			break
		}
	}
	if act == 0 {
		t.Fatalf("precondition: the {2}{U}{B} ability is not offered; opts %v", d.Options[:d.N])
	}
	s.Apply(act)
	sc.do(t, AkTarget, scAny, TCode(victim))
	sc.resolveAll(t)
	if got := s.objs[victim].zone; got != kop.ZoneExile {
		t.Fatalf("precondition: the copied card's zone is %d, want exile", got)
	}
	cps := sc.copiesOf(id[copied])
	if len(cps) != 1 {
		t.Fatalf("precondition: %d battlefield slots show %q, want the one token", len(cps), copied)
	}
	if s.objs[s.perms[cps[0]].obj].of&ofToken == 0 {
		t.Fatalf("precondition: the copy of %q is not a token", copied)
	}
	return sc, id[copied], cps[0]
}

// subIdx returns the pool subtype's index, failing when the vocabulary lacks
// it (the assertion would pass vacuously on a missing word).
func subIdx(t *testing.T, sc *scen, name string) uint8 {
	for i, w := range sc.k.t.Subtypes {
		if w == name {
			return uint8(i)
		}
	}
	t.Fatalf("precondition: subtype %q is not in the pool vocabulary", name)
	return 0
}

func TestScarabGodSetCreatureTypesToken(t *testing.T) {
	sc, _, sl := scarabTokenScen(t, "Accorder Paladin")
	s := sc.s
	if p, tg := s.ptGen(sc.k, sl); p != 4 || tg != 4 {
		t.Errorf("the token is %d/%d, want the exception's 4/4", p, tg)
	}
	if !s.hasSubGen(sc.k, sl, subIdx(t, sc, "Zombie")) {
		t.Error("the token lost the exception's Zombie")
	}
	// CR 707.9b: the copied creature types (Human Knight) go
	if s.hasSubGen(sc.k, sl, subIdx(t, sc, "Human")) || s.hasSubGen(sc.k, sl, subIdx(t, sc, "Knight")) {
		t.Error("the token kept the copied creature's printed creature types")
	}
	if s.hasSubGen(sc.k, sl, subOutside) {
		t.Error("the token has every creature type")
	}
}

func TestScarabGodSetCreatureTypesTokenChangeling(t *testing.T) {
	sc, _, sl := scarabTokenScen(t, "Changeling Outcast")
	s := sc.s
	if !s.hasSubGen(sc.k, sl, subIdx(t, sc, "Zombie")) {
		t.Error("the token lost the exception's Zombie")
	}
	// CR 613.3, 702.73a: the copied changeling's CDA runs after the copy
	// exception, so the token still has every creature type
	if !s.hasSubGen(sc.k, sl, subOutside) {
		t.Error("the copied changeling's token lost every creature type")
	}
	if !s.hasSubGen(sc.k, sl, subIdx(t, sc, "Shapeshifter")) {
		t.Error("the copied changeling's token lost its Shapeshifter subtype")
	}
	// some other creature type of the vocabulary
	other := ""
	for i, w := range sc.k.t.Subtypes {
		if i < int(kop.MaxSubtypes) && w != "Zombie" && w != "Shapeshifter" && sc.k.gen.creatSub.Has(i) {
			other = w
			if !s.hasSubGen(sc.k, sl, uint8(i)) {
				t.Errorf("the copied changeling's token lost %s", w)
			}
			break
		}
	}
	if other == "" {
		t.Fatal("precondition: the pool vocabulary has no creature type besides Zombie and Shapeshifter")
	}
}
