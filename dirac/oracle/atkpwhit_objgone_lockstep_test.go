package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.3c, 608.2h: Fathom Fleet Swordjack attacked a planeswalker, then
// Devouring Light exiled it before its attack trigger resolved. gorge damages
// the DefenderObj bound as the trigger was queued (the Valid
// Planeswalker.TriggeredDefender arm); the kernel keeps that object in its
// atkObj table and must neither stall at OpAtkPWGuard (StallDefenderPW, the
// old object arm) nor spare the walker.
//
// attack_trigger_object_gone is a recorded game of the decks below; the test
// replays it and requires the mechanism: a Swordjack trigger whose attacker was
// attacking a planeswalker resolved after that attacker left the battlefield.
// It was recorded by playing seeds of these decks with a policy that attacks
// the walker and answers the trigger with Devouring Light or Cloudshift on the
// attacker; seed 1 reached it.

const atkObjGoneRepro = "attack_trigger_object_gone"

var atkObjGoneDeck = deckOf(6, "Fathom Fleet Swordjack",
	deckOf(4, "Devouring Light",
		deckOf(4, "Cloudshift",
			deckOf(3, "Ajani Goldmane",
				deckOf(10, "Plains",
					deckOf(5, "Mountain")...)...)...)...)...)

// atkObjGoneTally follows gorge's event log: a Swordjack trigger pushed while
// its attacker attacks a planeswalker pushes a frame; the attacker leaving the
// battlefield marks the frame gone; a Resolve pops it.
type atkObjGoneTally struct {
	last     int
	frames   []atkObjGoneFrame
	triggers int // Swordjack triggers on an attacked planeswalker
	gone     int // of those, resolved after the attacker left
}

type atkObjGoneFrame struct {
	atk  state.ObjID
	obj  bool // the attacker attacked a planeswalker or battle when the trigger was pushed
	gone bool
}

func (a *atkObjGoneTally) check(g *GorgeSide, _ Side) *Divergence {
	if g == nil || g.E == nil || g.E.G == nil {
		return nil
	}
	evs := g.E.L.Events
	for ; a.last < len(evs); a.last++ {
		e := &evs[a.last]
		switch e.Kind {
		case events.MoveZone:
			if e.From != state.ZBattlefield {
				break
			}
			for i := range a.frames {
				if a.frames[i].atk == e.Obj {
					a.frames[i].gone = true
				}
			}
		case events.TriggerPush:
			if atkDefenderGoneCarrier(g.E.G, e.Obj) != "Fathom Fleet Swordjack" {
				a.frames = append(a.frames, atkObjGoneFrame{})
				break
			}
			o := g.E.G.Obj(e.Obj)
			f := atkObjGoneFrame{atk: e.Obj, obj: o != nil && o.IsAttacking && o.AttackingBattle != 0}
			if f.obj {
				a.triggers++
			}
			a.frames = append(a.frames, f)
		case events.PutOnStack:
			a.frames = append(a.frames, atkObjGoneFrame{})
		case events.Resolve:
			if len(a.frames) == 0 {
				break
			}
			f := a.frames[len(a.frames)-1]
			a.frames = a.frames[:len(a.frames)-1]
			if f.obj && f.gone {
				a.gone++
			}
		}
	}
	return nil
}

// atkObjGonePlay plays r's decks on pol and tallies the mechanism.
func atkObjGonePlay(t *testing.T, r *Repro, mk func(*atkObjGoneTally) Policy) (Result, *atkObjGoneTally) {
	t.Helper()
	reg := synthReg(t)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.Raw(printedDelveSet(t, r.Cards))
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	tally := &atkObjGoneTally{}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 400
	if len(r.Actions) == 0 {
		rc.MaxDecisions = 3000
	}
	rc.Check = tally.check
	rc.Finish = func(g *GorgeSide) { tally.check(g, nil) }
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, mk(tally), rc), tally
}

// TestAtkPWHitObjectGoneReprosLockstep replays the recorded game: no
// divergence and no kernel stall, every action served, and the attacked
// planeswalker's trigger resolved after its attacker left.
func TestAtkPWHitObjectGoneReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", atkObjGoneRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res, tally := atkObjGonePlay(t, r, func(*atkObjGoneTally) Policy { return pol })
	if res.Div != nil {
		t.Fatalf("divergence at %d of %d: %s %s %v\nref %q\nker %q\n%s", res.Div.At, len(r.Actions), res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
	}
	if res.StallUnmodelled || res.Stall != "" && len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
		t.Fatalf("kernel stopped: %s", res.Stall)
	}
	if msg := reproSkipFailure(r, pol); msg != "" {
		t.Fatal(msg)
	}
	if tally.triggers == 0 || tally.gone == 0 {
		t.Fatalf("%d Swordjack triggers on an attacked planeswalker, %d resolved after the attacker left: the game does not reach the mechanism", tally.triggers, tally.gone)
	}
}
