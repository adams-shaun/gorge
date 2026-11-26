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

// TestGraveAbilityLossLeaveLookBack: a card leaving the graveyard for another
// zone is judged before the move (CR 603.10a), where the removal still
// applies; one moving to the battlefield is judged on the new object. After
// the move the plain predicate no longer sees a graveyard card, which is why
// the look-back reads first.
func TestGraveAbilityLossLeaveLookBack(t *testing.T) {
	tbl, jailer := graveLossTable()
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	card := sc.grave(v.ThinkTwice, 0, 1)[0]
	if _, lost := s.graveLeaveLost(k, card, kop.ZoneHand); lost {
		t.Fatal("look-back lost with no Jailer on the battlefield")
	}
	sc.perm(jailer, 1)
	for _, z := range []kop.Zone{kop.ZoneHand, kop.ZoneExile, kop.ZoneLibrary, kop.ZoneGraveyard} {
		if _, lost := s.graveLeaveLost(k, card, z); !lost {
			t.Errorf("a card leaving for zone %v is judged lost before the move", z)
		}
	}
	if _, lost := s.graveLeaveLost(k, card, kop.ZoneBattlefield); lost {
		t.Error("a card moving to the battlefield is judged on the new object, not lost")
	}
	DebugNoGraveLoss = true
	_, lost := s.graveLeaveLost(k, card, kop.ZoneHand)
	DebugNoGraveLoss = false
	if lost {
		t.Error("DebugNoGraveLoss did not silence the look-back")
	}
	// the card has left: the live predicate answers for a graveyard card only
	s.detach(card)
	if _, lost := s.graveAbilitiesLost(k, card); lost {
		t.Error("the live predicate read a card that already left the graveyard")
	}
}

// TestGraveAbilityLossHidesTrigger: a graveyard card's printed trigger
// (TriggerZones$ Graveyard) is queued with no Jailer and suppressed, and
// counted, while one is; the other listeners' order is untouched.
func TestGraveAbilityLossHidesTrigger(t *testing.T) {
	tbl, jailer := graveLossTable()
	tbl.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	tbl.Cards = append(tbl.Cards, kop.CardDef{
		Name:      "Grave Drawn Test",
		Types:     kop.TCreature,
		Power:     1,
		Toughness: 1,
		Abilities: []kop.Ability{{
			Kind: kop.AbTriggered,
			Trig: kop.Trigger{Event: kop.TrigDrawn, Zone: kop.ZoneGraveyard},
		}},
	})
	watcher := CardID(len(tbl.Cards) - 1)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	s, k := sc.s, sc.k
	if len(k.gen.graveTrig) == 0 {
		t.Skip("the table lists no graveyard trigger card")
	}
	sc.grave(watcher, 0, 1)
	queued := func() int {
		b := s.npend
		s.fireGraveDrawn(k, 0, 1)
		return int(s.npend - b)
	}
	if queued() != 1 {
		t.Fatal("precondition: the graveyard Drawn trigger is not queued with no Jailer")
	}
	j := sc.perm(jailer, 1)
	before := GraveLossStats.Hits.Load()
	if n := queued(); n != 0 {
		t.Errorf("%d trigger(s) queued although the card lost all abilities", n)
	}
	if GraveLossStats.Hits.Load() == before {
		t.Error("GraveLossStats counted no hit for the suppressed trigger")
	}
	s.destroy(k, sc.slot(j))
	if queued() != 1 {
		t.Error("the trigger is not queued again once the Jailer left")
	}
}
