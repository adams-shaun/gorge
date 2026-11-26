package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// graveLossTable is vanilla's table plus one Yixlid Jailer stand-in: a
// creature carrying one StatGraveNoAbilities static over every card.
func graveLossTable() (kop.Table, CardID) {
	tbl := *v.Table
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...), kop.CardDef{
		Name:      "Grave Loss Test",
		Types:     kop.TCreature,
		Power:     2,
		Toughness: 1,
		Abilities: []kop.Ability{{
			Kind:   kop.AbStatic,
			Static: kop.Static{Kind: kop.StatGraveNoAbilities, Zone: kop.ZoneGraveyard},
		}},
	})
	return tbl, CardID(len(tbl.Cards) - 1)
}

// TestGraveAbilityLossGenUse: the genUse bit is set for a table holding the
// static and clear for one that does not (no State field backs it).
func TestGraveAbilityLossGenUse(t *testing.T) {
	tbl, _ := graveLossTable()
	if !tableHasGraveLoss(&tbl) {
		t.Fatal("tableHasGraveLoss: false for a table carrying StatGraveNoAbilities")
	}
	if tableHasGraveLoss(v.Table) {
		t.Fatal("tableHasGraveLoss: true for the vanilla table")
	}
	on := newScenTable(t, &tbl, Config{NoElide: true})
	if !graveLossOn(on.k) {
		t.Error("useGraveLoss off for the table with the static")
	}
	off := newScen(t, Config{NoElide: true})
	if graveLossOn(off.k) {
		t.Error("useGraveLoss on for the vanilla table")
	}
}

// TestGraveAbilityLossPredicate: a graveyard card is lost while a Jailer
// static is live on the battlefield, with the removal's timestamp, and not
// once the source leaves; DebugNoGraveLoss silences it.
func TestGraveAbilityLossPredicate(t *testing.T) {
	tbl, jailer := graveLossTable()
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	card := sc.grave(v.ThinkTwice, 0, 1)[0]
	if _, lost := s.graveAbilitiesLost(k, card); lost {
		t.Fatal("lost with no Jailer on the battlefield")
	}
	j := sc.perm(jailer, 1)
	ts, lost := s.graveAbilitiesLost(k, card)
	if !lost {
		t.Fatal("not lost while a Jailer static is live (the opponent's: it reaches every graveyard)")
	}
	if want := s.layerTs(sc.slot(j)); ts != want || ts == 0 {
		t.Errorf("stamp %d, want the source's layer timestamp %d", ts, want)
	}
	if _, lost := s.graveAbilitiesLost(k, sc.perm(v.GrizzlyBears, 0)); lost {
		t.Error("a battlefield permanent read as a graveyard card")
	}
	DebugNoGraveLoss = true
	_, lost = s.graveAbilitiesLost(k, card)
	DebugNoGraveLoss = false
	if lost {
		t.Error("DebugNoGraveLoss did not silence the predicate")
	}
	// the source leaves: the removal ends
	s.destroy(sc.k, sc.slot(j))
	if _, lost := s.graveAbilitiesLost(k, card); lost {
		t.Error("still lost after the Jailer left the battlefield")
	}
}

// TestGraveAbilityLossHidesFlashback: a printed flashback offer is made while
// no Jailer is in play and withheld, and counted, while one is.
func TestGraveAbilityLossHidesFlashback(t *testing.T) {
	tbl, jailer := graveLossTable()
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s := sc.s
	for i := 0; i < 3; i++ {
		sc.perm(v.Island, 0)
	}
	card := sc.grave(v.ThinkTwice, 0, 1)[0]
	flashbackOffered := func() bool {
		d := s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(card) && a.Mode()&2 != 0 {
				return true
			}
		}
		return false
	}
	if !flashbackOffered() {
		t.Fatal("precondition: Think Twice's flashback is not offered with no Jailer")
	}
	j := sc.perm(jailer, 1)
	before := GraveLossStats.Hits.Load()
	if flashbackOffered() {
		t.Error("flashback offered although the card lost all abilities")
	}
	if GraveLossStats.Hits.Load() == before {
		t.Error("GraveLossStats counted no hit for the withheld offer")
	}
	s.destroy(sc.k, sc.slot(j))
	if !flashbackOffered() {
		t.Error("flashback not offered again once the Jailer left")
	}
}
