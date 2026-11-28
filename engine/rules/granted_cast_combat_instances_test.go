package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedCastAndCombatInstancesTriggerSeparately: every instance of Storm,
// Exploit or Afflict on one object triggers on its own (CR 113.2c, 702.40b),
// a grant identical to a printed line included. A printed line owns one
// derived entry, its own expansion queuing that instance; every other equal
// entry is a granted instance and queues too. The printed-only and
// granted-once rows hold the count from the other side.
func TestGrantedCastAndCombatInstancesTriggerSeparately(t *testing.T) {
	t.Parallel()
	keywords := []string{"Storm", "Exploit", "Afflict:1"}
	shapes := []struct {
		name            string
		printed, grants int
	}{
		{"printed and granted", 1, 1},
		{"granted twice", 0, 2},
		{"printed once", 1, 0},
		{"granted once", 0, 1},
	}
	seed := uint64(7130)
	for _, kw := range keywords {
		for _, sh := range shapes {
			seed++
			seed := seed
			t.Run(kw+"/"+sh.name, func(t *testing.T) {
				t.Parallel()
				grantedCastCombatCase(t, seed, kw, sh.printed, sh.grants)
			})
		}
	}
}

// grantedCastCombatCase seeds a 2/2 printing kw printed times in seat 0's
// deck, puts grants enchantments granting it on the battlefield, fires the
// keyword's event once and requires one queued trigger per instance.
func grantedCastCombatCase(t *testing.T, seed uint64, kw string, printed, grants int) {
	t.Helper()
	want := printed + grants
	bearSrc := "Name:Scratch Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\n" +
		strings.Repeat("K:"+kw+"\n", printed) + "Oracle:x\n"
	bearCard := card(t, bearSrc)
	e, _ := tokenReplGame(t, seed, bearCard)
	toMain1(t, e)
	affected := "Creature.YouCtrl"
	if kw == "Storm" {
		affected = "Card.YouCtrl | AffectedZone$ Stack"
	}
	for range grants {
		onBoard(t, e, 0, "Name:Keyword Grant\nManaCost:2\nTypes:Enchantment\n"+
			"S:Mode$ Continuous | Affected$ "+affected+" | AddKeyword$ "+kw+" | Description$ x\nOracle:x\n")
	}
	held := func(id state.ObjID) {
		t.Helper()
		n := 0
		for _, k := range e.Derived(id).Keywords {
			if k == kw {
				n++
			}
		}
		if n != want {
			t.Fatalf("precondition: derived keywords %q hold %q %d times, want %d", e.Derived(id).Keywords, kw, n, want)
		}
	}
	var bear state.ObjID
	var from int
	switch kw {
	case "Storm":
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand} {
			for _, id := range e.G.Zone(z, 0) {
				if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Scratch Bear" && bear == 0 {
					bear = id
					from = len(e.L.Events)
					e.emit(events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: z, To: state.ZStack})
				}
			}
		}
		if bear == 0 {
			t.Fatalf("seeded Scratch Bear not found")
		}
		held(bear)
	case "Exploit":
		from = len(e.L.Events)
		bear = moveSeededCard(t, e, 0, bearCard, state.ZBattlefield)
		held(bear)
	case "Afflict:1":
		bear = moveSeededCard(t, e, 0, bearCard, state.ZBattlefield)
		blk := onBoard(t, e, 1, "Name:Wall Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:0/4\nOracle:x\n")
		e.G.Obj(bear).SummonSick = false
		held(bear)
		declareAttackersOnly(t, e, 0, 1, bear)
		from = len(e.L.Events)
		e.emit(events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{bear, blk}}})
	}
	if got := queuedSince(e, bear, from); got != want {
		t.Fatalf("%s printed %d times and granted %d times queued %d triggers, want %d", kw, printed, grants, got, want)
	}
}
