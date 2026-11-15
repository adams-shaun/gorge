package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The SpellCast trigger's ValidCard$ Card.NamedCard reads the listener's own
// chosen card name (gorge namePredicate reads g.Obj(sc.Source).ChosenName
// with the trigger's source, Silverquill Silencer): the opponent casting the
// named spell queues it and drains 3 life, any other spell does not. This is
// the Card.NamedCard read outside a resolution (a dig's Valid$ reads its own
// resolution's answer instead), so castFilterOK matches the spell with the
// listener as the filter's source context (trig_castbound.go).
func TestCastNamedCardSpellFiresTheListener(t *testing.T) {
	deckA := []cardset.Entry{{Name: "Silverquill Silencer", Count: 1}, {Name: "Plains", Count: 2}, {Name: "Swamp", Count: 2}}
	deckB := []cardset.Entry{{Name: "Dark Ritual", Count: 2}, {Name: "Swamp", Count: 3}}
	set := cardsettest.Set(t, "namecast")
	tbl, da, _, err := set.Matchup(deckA, deckB)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sil, ok := id["Silverquill Silencer"]
	if !ok {
		t.Fatal("precondition: no Silverquill Silencer in the table")
	}
	ritual := id["Dark Ritual"]
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{})
	silo := sc.perm(sil, 0)
	// answer the ETB name ask with Dark Ritual (the entering controller's
	// ChGenName options, mode = the card's CardID)
	if d := sc.s.Step(); d.Kind == DkChoose {
		hit := false
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkChoose && a.Mode() == uint8(ritual) {
				sc.s.Apply(a)
				hit = true
				break
			}
		}
		if !hit {
			t.Fatalf("the name ask offered no Dark Ritual: %v", d.Options[:d.N])
		}
	}
	if got := sc.s.perms[sc.slot(silo)].chosen; got != objWord(ritual) {
		t.Fatalf("precondition: Silencer's chosen byte is %d, want Dark Ritual's %d", got, ritual)
	}
	sc.perm(id["Swamp"], 1)
	rit := sc.hand(ritual, 1)
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0)) // seat 1's priority
	// the cast queues Silencer's trigger, which resolves with no decision:
	// seat 1 loses 3 life and seat 0 draws a card
	life, hand := sc.s.pl[1].life, sc.s.pl[0].handN
	sc.do(t, AkCast, TCode(rit), scAny)
	for i := 0; i < 8; i++ {
		sc.s.Step()
	}
	if sc.s.pl[1].life != life-3 || sc.s.pl[0].handN != hand+1 {
		t.Fatalf("the named cast went life %d hand %d, want life %d and hand %d", sc.s.pl[1].life, sc.s.pl[0].handN, life-3, hand+1)
	}
	// an unnamed cast queues nothing: seat 1's second Dark Ritual with the
	// byte wiped (ClearNamedCard) matches nothing, as gorge's wiped
	// ChosenName does
	sc.s.perms[sc.slot(silo)].chosen = 255
	for i := 0; i < 40; i++ {
		if sc.s.step == kop.StepMain2 || sc.s.step == kop.StepEnd {
			break
		}
		d := sc.s.Step()
		switch d.Kind {
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		case DkAttackers, DkBlockers:
			sc.s.Apply(MakeAction(AkDone, 0, 0, 0))
		case DkNone:
			t.Fatal("the game ended before the second cast")
		}
	}
	if sc.s.step != kop.StepMain2 && sc.s.step != kop.StepEnd {
		t.Fatalf("never reached main 2 (step %d)", sc.s.step)
	}
	rit2 := sc.hand(ritual, 1)
	if sc.s.prio == 0 {
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	life, hand = sc.s.pl[1].life, sc.s.pl[0].handN
	sc.do(t, AkCast, TCode(rit2), scAny)
	for i := 0; i < 8; i++ {
		sc.s.Step()
	}
	if sc.s.pl[1].life != life || sc.s.pl[0].handN != hand {
		t.Fatalf("the unnamed cast went life %d hand %d, want life %d and hand %d", sc.s.pl[1].life, sc.s.pl[0].handN, life, hand)
	}
}
