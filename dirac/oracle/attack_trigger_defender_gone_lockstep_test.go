package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAttackTriggerDefenderGoneReprosLockstep replays Fathom Fleet
// Swordjack's attack trigger after Devouring Light exiles the attacker in
// response: gorge binds the defending player as the trigger is queued
// (CR 603.10, 608.2h) and resolves the damage from the attacker's last known
// information; the kernel must bind the same seat at two seats and not stop
// with a defenderpw stall.
func TestAttackTriggerDefenderGoneReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "attack_trigger_defender_gone")
}

// atkDefenderGoneDeck reaches the mechanism at four seats: an attack trigger
// that names the defending player (Fathom Fleet Swordjack's
// TriggeredDefender damage, Mage Slayer's DamageSource$ form) and an instant
// that removes the attacker before the trigger resolves (Devouring Light
// exiles it, Cloudshift blinks it out and back as a new object).
var atkDefenderGoneDeck = deckOf(4, "Mage Slayer",
	deckOf(4, "Fathom Fleet Swordjack",
		deckOf(4, "Devouring Light",
			deckOf(4, "Cloudshift",
				deckOf(8, "Raging Goblin",
					deckOf(5, "Plains",
						deckOf(6, "Mountain", deckOf(5, "Forest")...)...)...)...)...)...)...)

// atkDefenderGoneTally replays the gorge event log's stack: a TriggerPush
// whose trigger object is the attack trigger's attacker pushes a frame; a
// MoveZone off the battlefield makes it gone; a Resolve pops it. The tally
// counts resolves whose attacker left while the trigger waited, and the
// attack triggers of the two carriers, so the test cannot pass empty.
type atkDefenderGoneTally struct {
	last         int
	frames       []atkDefenderGoneFrame
	mage, fathom atkDefenderGoneCount
}

type atkDefenderGoneCount struct {
	triggers int
	gone     int
}

type atkDefenderGoneFrame struct {
	name string
	ids  []state.ObjID
	gone bool
}

func (a *atkDefenderGoneTally) reset() {
	a.last = 0
	a.frames = a.frames[:0]
	a.mage, a.fathom = atkDefenderGoneCount{}, atkDefenderGoneCount{}
}

func (a *atkDefenderGoneTally) check(g *GorgeSide, _ Side) *Divergence {
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
				for _, id := range a.frames[i].ids {
					if id == e.Obj {
						a.frames[i].gone = true
					}
				}
			}
		case events.TriggerPush:
			name := atkDefenderGoneCarrier(g.E.G, e.Obj)
			ids := append([]state.ObjID{e.Obj}, e.IDs...)
			a.frames = append(a.frames, atkDefenderGoneFrame{name: name, ids: ids})
			switch name {
			case "Mage Slayer":
				a.mage.triggers++
			case "Fathom Fleet Swordjack":
				a.fathom.triggers++
			}
		case events.PutOnStack:
			a.frames = append(a.frames, atkDefenderGoneFrame{})
		case events.Resolve:
			if len(a.frames) == 0 {
				break
			}
			f := a.frames[len(a.frames)-1]
			a.frames = a.frames[:len(a.frames)-1]
			if !f.gone {
				break
			}
			switch f.name {
			case "Mage Slayer":
				a.mage.gone++
			case "Fathom Fleet Swordjack":
				a.fathom.gone++
			}
		}
	}
	return nil
}

// atkDefenderGoneCarrier names obj when it is one of the two attack triggers
// the test carries, else "".
func atkDefenderGoneCarrier(g *state.Game, obj state.ObjID) string {
	o := g.Obj(obj)
	if o == nil || o.Face() == nil {
		return ""
	}
	switch o.Face().Name {
	case "Mage Slayer", "Fathom Fleet Swordjack":
		return o.Face().Name
	}
	return ""
}

// atkDefenderGoneInstant reports whether a cast option is an instant, which
// the policy holds for the carrier trigger's response window instead of
// spending on its own turn.
func atkDefenderGoneInstant(c *PickCtx, a Canon) bool {
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return false
	}
	ob := c.G.E.G.Obj(o.Obj)
	if ob == nil || ob.Face() == nil {
		return false
	}
	for _, t := range ob.Face().Types {
		if t == "Instant" {
			return true
		}
	}
	return false
}

// atkDefenderGoneTriggerOnStack reports whether a carrier's triggered ability
// (Mage Slayer's or Fathom Fleet Swordjack's attack trigger) is on the stack,
// so the policy saves its response spells for the window the test needs.
func atkDefenderGoneTriggerOnStack(g *state.Game) bool {
	if g == nil {
		return false
	}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || state.StackKindOf(g, o) != state.StackKindTriggered {
			continue
		}
		if src := g.Obj(o.Source); src != nil && src.Face() != nil {
			switch src.Face().Name {
			case "Mage Slayer", "Fathom Fleet Swordjack":
				return true
			}
		}
	}
	return false
}

// atkDefenderGonePolicy answers yes, casts Devouring Light or Cloudshift in
// response to a trigger (Cloudshift on its own attacking creature, Devouring
// Light on an attacking creature), attacks with every offered creature, and
// otherwise casts, activates and plays lands at random.
func atkDefenderGonePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var atk, rest []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActYes {
				return o
			}
		}
		inResponse := c.G != nil && atkDefenderGoneTriggerOnStack(c.G.E.G)
		if inResponse {
			for _, o := range c.D.Options {
				if o.Kind() == ActCast || o.Kind() == ActCastFace {
					return o
				}
			}
			if c.D.Kind == DTarget {
				for _, o := range c.D.Options {
					if o.Kind() != ActTarget || int(o.Tgt()) >= int(RefPlayer) {
						continue
					}
					if ob := c.G.E.G.Obj(state.ObjID(o.Tgt())); ob != nil && ob.IsAttacking {
						return o
					}
				}
			}
		}
		atk, rest = atk[:0], rest[:0]
		for _, o := range c.D.Options {
			switch o.Kind() {
			case ActAttack:
				atk = append(atk, o)
			case ActCast, ActCastFace:
				if atkDefenderGoneInstant(c, o) {
					continue // saved for the carrier trigger's response window
				}
				rest = append(rest, o)
			case ActActivate, ActPlayLand:
				rest = append(rest, o)
			}
		}
		if len(atk) > 0 {
			d := *c.D
			d.Options = atk
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if len(rest) > 0 {
			d := *c.D
			d.Options = rest
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		// never spend a held instant outside its response window: pass
		// instead when that is an option.
		var safe []Canon
		for _, o := range c.D.Options {
			if (o.Kind() == ActCast || o.Kind() == ActCastFace) && atkDefenderGoneInstant(c, o) {
				continue
			}
			safe = append(safe, o)
		}
		if len(safe) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = safe
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// TestAttackTriggerDefenderGoneFourSeat plays four-seat pods of the carriers
// and their response spells on a table built from the decks: an attack
// trigger whose attacker left before it resolved must deal the defending
// player's damage in both engines, with no stall, and at least one game must
// reach the mechanism.
func TestAttackTriggerDefenderGoneFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	decks := [4][]string{atkDefenderGoneDeck, atkDefenderGoneDeck, atkDefenderGoneDeck, atkDefenderGoneDeck}
	var all []string
	for _, d := range decks {
		all = append(all, d...)
	}
	set := printedDelveSet(t, [2][]string{all, all})
	tally := &atkDefenderGoneTally{}
	play := fourSeatSetPlay(t, set, decks, nil, tally.check, func(g *GorgeSide) {
		tally.check(g, nil)
	}, atkDefenderGonePolicy)
	const seeds = 20
	var mage, fathom atkDefenderGoneCount
	for seed := uint64(1); seed <= seeds; seed++ {
		tally.reset()
		res := play(seed)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %q\nker %q\n%s last %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		mage.triggers += tally.mage.triggers
		mage.gone += tally.mage.gone
		fathom.triggers += tally.fathom.triggers
		fathom.gone += tally.fathom.gone
	}
	triggers, gone := mage.triggers+fathom.triggers, mage.gone+fathom.gone
	if triggers == 0 {
		t.Fatalf("no carrier attack trigger fired in %d games", seeds)
	}
	if gone == 0 {
		t.Fatalf("%d carrier attack triggers fired in %d games, none resolved after its attacker left", triggers, seeds)
	}
	if mage.gone == 0 {
		t.Fatalf("Mage Slayer fired %d times in %d games, none resolved after its attacker left", mage.triggers, seeds)
	}
	if fathom.gone == 0 {
		t.Fatalf("Fathom Fleet Swordjack fired %d times in %d games, none resolved after its attacker left", fathom.triggers, seeds)
	}
	t.Logf("%d seeds: Mage Slayer %d/%d, Fathom Fleet Swordjack %d/%d (gone/triggers)", seeds, mage.gone, mage.triggers, fathom.gone, fathom.triggers)
}

// TestAttackTriggerDefenderGoneMutation restores the old two-seat reading
// (no queue-time defender) and requires the recorded game to fail: the
// attacker is gone when its TriggeredDefender damage resolves.
func TestAttackTriggerDefenderGoneMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "attack_trigger_defender_gone.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugAtkTriggerNoDefenderBind
	dirac.DebugAtkTriggerNoDefenderBind = true
	defer func() { dirac.DebugAtkTriggerNoDefenderBind = old }()
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil && !strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("mutation: the recorded game neither diverged nor stopped the kernel (%q)", res.Stall)
	}
	t.Logf("mutation: divergence=%v stall=%q", res.Div != nil, res.Stall)
}
