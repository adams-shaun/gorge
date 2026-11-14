package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newTriggerOrderState is a two-seat scenario over set, with deck's first
// entry seated as seat 0's commander (in the command zone). It empties both
// hands so the test places exactly the permanents it needs.
func newTriggerOrderState(t *testing.T, set cardset.Set, deck []cardset.Entry) (*State, *prep, map[string]CardID) {
	t.Helper()
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id[deck[len(deck)-1].Name] // the last entry is a basic land
	}
	lib[0] = id[deck[0].Name] // the first entry is the commander
	cfg := Config{NoElide: true, Commander: true, Commanders: noCommanders()}
	cfg.Commanders[0][0] = 0
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, cfg)
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn = 2
	s.active = 0
	s.step = kop.StepMain1
	s.prio = 0
	s.passes = 0
	s.ctl = ctlPriority
	return s, s.k(), id
}

// placeInGrave puts a fresh object for card c in p's graveyard.
func placeInGrave(s *State, c CardID, p uint8) Obj {
	o := Obj(s.nobj)
	s.nobj++
	s.objs[o] = object{card: c, chosen: 255, slot: NoSlot}
	s.place(o, kop.ZoneGraveyard, false)
	return o
}

// pendingIndex returns the first pending trigger whose card is name, -1 if
// none.
func pendingIndex(k *prep, s *State, name string) int {
	for i := 0; i < int(s.npend); i++ {
		if k.t.Cards[s.pend[i].card].Name == name {
			return i
		}
	}
	return -1
}

// TestCommandTriggerQueuesAfterGraveyard pins the spell-cast queue order to
// gorge's object walk: library, hand, battlefield, graveyard, exile, stack,
// command (rules forEachObject). A commander's printed command-zone trigger is
// therefore queued after a graveyard card's trigger on the same event, not
// before it.
func TestCommandTriggerQueuesAfterGraveyard(t *testing.T) {
	set := cardsettest.Set(t, "strictlyself")
	s, k, id := newTriggerOrderState(t, set, []cardset.Entry{
		{Name: "Edgar Markov", Count: 1},
		{Name: "Lingering Phantom", Count: 1},
		{Name: "Plains", Count: 1},
	})
	if o := s.cmdObj(0, 0); o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("Edgar Markov is not in seat 0's command zone: %v", o)
	}
	placeInGrave(s, id["Lingering Phantom"], 0)

	// A legendary Vampire spell: Edgar's command-zone eminence and Lingering
	// Phantom's graveyard historic-spell trigger are both on this event.
	spell := Obj(s.nobj)
	s.nobj++
	s.objs[spell] = object{card: id["Edgar Markov"], chosen: 255, slot: NoSlot, zone: kop.ZoneStack}
	e := stackEntry{kind: seSpell, obj: spell, card: id["Edgar Markov"], ctrl: 0}
	s.onCast(k, &e)

	graveAt := pendingIndex(k, s, "Lingering Phantom")
	commandAt := pendingIndex(k, s, "Edgar Markov")
	if graveAt < 0 || commandAt < 0 {
		t.Fatalf("same-event triggers not queued: Lingering Phantom at %d, Edgar Markov at %d (%d pending)",
			graveAt, commandAt, s.npend)
	}
	if graveAt > commandAt {
		t.Fatalf("command-zone trigger queued at %d before the graveyard trigger at %d: want graveyard first",
			commandAt, graveAt)
	}
}

// TestCommandAttackTriggerQueuesAfterGraveyard is the same pin for the
// DeclareAttackers walk: Sidar Jabari's command-zone eminence (a Knight
// attacks) and Interceptor, Shadow's Hound's graveyard trigger (a legendary
// creature attacks) fire on one event.
func TestCommandAttackTriggerQueuesAfterGraveyard(t *testing.T) {
	set := cardsettest.Set(t, "strictlyself")
	s, k, id := newTriggerOrderState(t, set, []cardset.Entry{
		{Name: "Sidar Jabari of Zhalfir", Count: 1},
		{Name: "Interceptor, Shadow's Hound", Count: 1},
		{Name: "Edgar Markov", Count: 1},
		{Name: "Plains", Count: 1},
	})
	if o := s.cmdObj(0, 0); o == NoObj || s.objs[o].zone != kop.ZoneCommand {
		t.Fatalf("Sidar Jabari is not in seat 0's command zone: %v", o)
	}
	placeInGrave(s, id["Interceptor, Shadow's Hound"], 0)
	edgar := Obj(s.nobj)
	s.nobj++
	s.objs[edgar] = object{card: id["Edgar Markov"], chosen: 255, slot: NoSlot}
	sl := s.enter(k, edgar, 0, false, kop.ZoneNone)
	s.perms[sl].flags &^= pfSick
	s.perms[sl].flags |= pfAttacking
	s.perms[sl].atk = TCode(PlayerTarget1)
	s.nAttackers = 1
	s.npend = 0
	s.fireAttacksGen(k)

	graveAt := pendingIndex(k, s, "Interceptor, Shadow's Hound")
	commandAt := pendingIndex(k, s, "Sidar Jabari of Zhalfir")
	if graveAt < 0 || commandAt < 0 {
		t.Fatalf("same-event triggers not queued: Interceptor at %d, Sidar at %d (%d pending)",
			graveAt, commandAt, s.npend)
	}
	if graveAt > commandAt {
		t.Fatalf("command-zone trigger queued at %d before the graveyard trigger at %d: want graveyard first",
			commandAt, graveAt)
	}
}
