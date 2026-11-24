package dirac

import (
	"sort"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// cardKindsFixture is the choosecardkinds pool's matchup table and its card ids
// by name.
func cardKindsFixture(t *testing.T) (*kop.Table, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "choosecardkinds")
	deck := []cardset.Entry{
		{Name: "Archon of Valor's Reach", Count: 2}, {Name: "Cloud Key", Count: 2}, {Name: "Stenn, Paranoid Partisan", Count: 2},
		{Name: "Searing Spear", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Goblin Piker", Count: 4},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Short Sword", Count: 2}, {Name: "Glorious Anthem", Count: 2},
		{Name: "Mountain", Count: 6}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 3}, {Name: "Plains", Count: 3},
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return tbl, id
}

// cardKindsCast casts o (its mana already on the battlefield) and returns the
// as-enters ask it poses, nil when it enters unasked.
func (sc *scen) cardKindsCast(t *testing.T, o Obj) *Decision {
	t.Helper()
	sc.do(t, AkCast, TCode(o), scAny)
	for i := 0; i < 40; i++ {
		d := sc.s.Step()
		switch d.Kind {
		case DkChoose:
			return &d
		case DkPriority:
			if sc.slot(o) != NoSlot {
				return nil
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			t.Fatalf("unexpected decision kind %d while the card resolves", d.Kind)
		}
	}
	t.Fatal("no ask and no entry")
	return nil
}

// answerCardKind answers ask with the card type named, then passes until o is
// on the battlefield.
func (sc *scen) answerCardKind(t *testing.T, ask *Decision, o Obj, name string) {
	t.Helper()
	want := -1
	for i, c := range cardTypeChoice {
		if c.name == name {
			want = i
		}
	}
	if want < 0 {
		t.Fatalf("%s is no card type", name)
	}
	for i := 0; i < int(ask.N); i++ {
		a := ask.Options[i]
		if uint16(a.Target())<<8|uint16(a.Mode()) == cardTypeLabelH[want] {
			sc.s.Apply(a)
			for j := 0; j < 20 && sc.slot(o) == NoSlot; j++ {
				if d := sc.s.Step(); d.Kind == DkPriority {
					sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
				}
			}
			if sc.slot(o) == NoSlot {
				t.Fatalf("%s never entered", name)
			}
			return
		}
	}
	t.Fatalf("the ask does not offer %s", name)
}

// cardKindNames are the card types an ask offers, sorted by name (the decision's
// option order is its label values', which the oracle's canonical form sorts).
func cardKindNames(t *testing.T, ask *Decision) (out []string) {
	t.Helper()
	for i := 0; i < int(ask.N); i++ {
		a := ask.Options[i]
		v := uint16(a.Target())<<8 | uint16(a.Mode())
		found := ""
		for j := range cardTypeLabelH {
			if cardTypeLabelH[j] == v {
				found = cardTypeChoice[j].name
			}
		}
		if found == "" {
			t.Fatalf("option %d (%#x) is no card-type label", i, v)
		}
		out = append(out, found)
	}
	sort.Strings(out)
	return out
}

func sameNames(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestChooseTypeCardKindsPlay: Cloud Key, Archon of Valor's Reach and Stenn ask
// for a card type as they enter (gorge effChooseType over TypeChoiceLabels(
// "Card", ValidTypes$, InvalidTypes$)) and store the answer on the permanent,
// where their Card.ChosenType statics read it as "has that card type": an
// Instant costs {1} less beside Cloud Key's Instant, and a Creature does not;
// no Sorcery can be cast beside Archon's Sorcery, and an Instant can. The
// answer is asked again when the permanent leaves and re-enters.
func TestChooseTypeCardKindsPlay(t *testing.T) {
	tbl, id := cardKindsFixture(t)
	newScen := func(lands map[string]int) *scen {
		sc := newScenTableLib(t, tbl, id["Mountain"], Config{})
		for n, c := range lands {
			sc.lands(0, id[n], c)
		}
		return sc
	}
	chosenIdx := func(name string) objWord {
		for i, c := range cardTypeChoice {
			if c.name == name {
				return objWord(i)
			}
		}
		t.Fatalf("%s is no card type", name)
		return 255
	}

	t.Run("Cloud Key", func(t *testing.T) {
		// the controls: one Mountain pays neither two-mana spell with no Key
		bare := newScen(map[string]int{"Mountain": 1})
		bareSpear, barePiker := bare.hand(id["Searing Spear"], 0), bare.hand(id["Goblin Piker"], 0)
		if bare.castOffered(0, bareSpear) || bare.castOffered(0, barePiker) {
			t.Fatal("one Mountain pays a {1}{R} spell with no Cloud Key out: the reduction below proves nothing")
		}

		sc := newScen(map[string]int{"Mountain": 3})
		key := sc.hand(id["Cloud Key"], 0)
		ask := sc.cardKindsCast(t, key)
		if ask == nil || ask.Sub != ChGenType {
			t.Fatalf("Cloud Key posed no type ask: %+v", ask)
		}
		if got, want := cardKindNames(t, ask), []string{"Artifact", "Creature", "Enchantment", "Instant", "Sorcery"}; !sameNames(got, want) {
			t.Fatalf("Cloud Key offers %v, want %v", got, want)
		}
		sc.answerCardKind(t, ask, key, "Instant")
		sl := sc.slot(key)
		if got := sc.s.perms[sl].chosen; got != chosenIdx("Instant") {
			t.Fatalf("Cloud Key stored %d, want the Instant index %d", got, chosenIdx("Instant"))
		}

		sc.perm(id["Mountain"], 0)
		spear, piker := sc.hand(id["Searing Spear"], 0), sc.hand(id["Goblin Piker"], 0)
		if !sc.castOffered(0, spear) {
			t.Errorf("with Instant chosen, Searing Spear {1}{R} is not castable off one Mountain")
		}
		if sc.castOffered(0, piker) {
			t.Errorf("with Instant chosen, Goblin Piker {1}{R} is castable off one Mountain")
		}
		sc.s.perms[sl].chosen = chosenIdx("Creature")
		sc.s.touchSl(sl)
		if sc.castOffered(0, spear) || !sc.castOffered(0, piker) {
			t.Errorf("with Creature chosen, Spear castable %v, Piker castable %v: want false, true", sc.castOffered(0, spear), sc.castOffered(0, piker))
		}

		// the answer is asked again after the Key leaves and re-enters
		sc.s.perms[sl].chosen = chosenIdx("Instant")
		sc.s.leave(sc.k, sl, kop.ZoneHand, false)
		if sc.slot(key) != NoSlot || sc.s.objs[key].zone != kop.ZoneHand {
			t.Fatalf("Cloud Key did not return to the hand: zone %d", sc.s.objs[key].zone)
		}
		sc.lands(0, id["Mountain"], 3)
		again := sc.cardKindsCast(t, key)
		if again == nil || again.Sub != ChGenType {
			t.Fatalf("Cloud Key re-entered without asking: %+v", again)
		}
		sc.answerCardKind(t, again, key, "Sorcery")
		if got := sc.s.perms[sc.slot(key)].chosen; got != chosenIdx("Sorcery") {
			t.Errorf("the re-entered Key stored %d, want the Sorcery index %d", got, chosenIdx("Sorcery"))
		}
	})

	t.Run("Archon", func(t *testing.T) {
		// the control: with no Archon, a Sorcery and an Instant are both castable
		bare := newScen(map[string]int{"Mountain": 2, "Island": 1})
		bareDiv, bareSpear := bare.hand(id["Divination"], 0), bare.hand(id["Searing Spear"], 0)
		if !bare.castOffered(0, bareDiv) || !bare.castOffered(0, bareSpear) {
			t.Fatal("the control hand cannot cast Divination and Searing Spear with no Archon out")
		}

		sc := newScen(map[string]int{"Mountain": 4, "Forest": 1, "Plains": 1})
		ar := sc.hand(id["Archon of Valor's Reach"], 0)
		ask := sc.cardKindsCast(t, ar)
		if ask == nil || ask.Sub != ChGenType {
			t.Fatalf("Archon posed no type ask: %+v", ask)
		}
		if got, want := cardKindNames(t, ask), []string{"Artifact", "Enchantment", "Instant", "Planeswalker", "Sorcery"}; !sameNames(got, want) {
			t.Fatalf("Archon offers %v, want %v", got, want)
		}
		sc.answerCardKind(t, ask, ar, "Sorcery")
		sc.lands(0, id["Mountain"], 2)
		sc.lands(0, id["Island"], 1)
		div, spear := sc.hand(id["Divination"], 0), sc.hand(id["Searing Spear"], 0)
		if sc.castOffered(0, div) {
			t.Errorf("Divination (a Sorcery) is castable beside Archon's Sorcery")
		}
		if !sc.castOffered(0, spear) {
			t.Errorf("Searing Spear (an Instant) is not castable beside Archon's Sorcery")
		}
		sl := sc.slot(ar)
		sc.s.perms[sl].chosen = chosenIdx("Instant")
		sc.s.touchSl(sl)
		if !sc.castOffered(0, div) || sc.castOffered(0, spear) {
			t.Errorf("with Instant chosen, Divination castable %v, Searing Spear castable %v: want true, false", sc.castOffered(0, div), sc.castOffered(0, spear))
		}
	})

	t.Run("Stenn", func(t *testing.T) {
		sc := newScen(map[string]int{"Mountain": 1, "Plains": 1, "Island": 1})
		st := sc.hand(id["Stenn, Paranoid Partisan"], 0)
		ask := sc.cardKindsCast(t, st)
		if ask == nil || ask.Sub != ChGenType {
			t.Fatalf("Stenn posed no type ask: %+v", ask)
		}
		want := []string{"Artifact", "Battle", "Enchantment", "Instant", "Kindred", "Planeswalker", "Sorcery"}
		if got := cardKindNames(t, ask); !sameNames(got, want) {
			t.Fatalf("Stenn offers %v, want %v (no Creature, no Land)", got, want)
		}
		sc.answerCardKind(t, ask, st, "Instant")
		sc.perm(id["Mountain"], 0)
		if spear := sc.hand(id["Searing Spear"], 0); !sc.castOffered(0, spear) {
			t.Errorf("Searing Spear is not castable off one Mountain beside Stenn's Instant")
		}
	})
}
