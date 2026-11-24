package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/trigmatch"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestSpellCastValidSAKickedWarpNumTargets pins the three SpellCast ValidSA$
// heads the corpus carries (Verazol's Spell.Kicked, Cantor of the Refrain's
// Spell.Warp, Voracious Bibliophile's Spell.numTargets GE1) against a spell on
// the stack whose cast flags and targets the case sets, and the census
// (effects.UnknownPredicates) that must agree with the matcher.
func TestSpellCastValidSAKickedWarpNumTargets(t *testing.T) {
	t.Parallel()
	spellCard := card(t, "Name:Probe Bolt\nTypes:Instant\nManaCost:R\nOracle:x\n")
	setup := func(flags uint64, targets, subTargets []state.Target) (*Engine, state.ObjID, state.ObjID, events.Event) {
		e := handEngine(t, spellCard)
		src := battlePerm(t, e, 0, "Name:Probe Watcher\nTypes:Creature Human\nPT:1/1\nOracle:x\n")
		other := battlePerm(t, e, 1, "Name:Probe Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
		id := e.G.Zone(state.ZHand, 0)[0]
		o := e.G.Obj(id)
		o.Zone = state.ZStack
		e.G.SetZone(state.ZHand, 0, nil)
		e.G.SetZone(state.ZStack, 0, []state.ObjID{id})
		o.CastFlags = flags
		for i := range targets {
			if !targets[i].IsPlayer {
				targets[i].Obj = other
			}
		}
		o.Targets, o.SubTargets = targets, subTargets
		return e, src, id, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack}
	}
	obj := state.Target{}
	pl := state.Target{Player: 1, IsPlayer: true}
	cases := []struct {
		name       string
		clause     string
		flags      uint64
		targets    []state.Target
		subTargets []state.Target
		want       bool
	}{
		{"kicked", "Spell.Kicked", state.FlagKicked, nil, nil, true},
		{"unkicked", "Spell.Kicked", 0, nil, nil, false},
		{"warped", "Spell.Warp", state.FlagWarped, nil, nil, true},
		{"unwarped", "Spell.Warp", state.FlagKicked, nil, nil, false},
		{"zero targets GE1", "Spell.numTargets GE1", 0, nil, nil, false},
		{"one target GE1", "Spell.numTargets GE1", 0, []state.Target{obj}, nil, true},
		{"one sub-target GE1", "Spell.numTargets GE1", 0, nil, []state.Target{pl}, true},
		{"zero targets EQ0", "Spell.numTargets EQ0", 0, nil, nil, true},
		{"one target EQ1", "Spell.numTargets EQ1", 0, []state.Target{obj}, nil, true},
		{"two targets EQ2", "Spell.numTargets EQ2", 0, []state.Target{obj}, []state.Target{pl}, true},
		{"two targets EQ1", "Spell.numTargets EQ1", 0, []state.Target{obj}, []state.Target{pl}, false},
		{"malformed comparison", "Spell.numTargets GEx", 0, []state.Target{obj}, nil, false},
	}
	for _, c := range cases {
		e, src, id, ev := setup(c.flags, append([]state.Target(nil), c.targets...), c.subTargets)
		o := e.G.Obj(id)
		if o.Zone != state.ZStack || o.CastFlags != c.flags || len(o.Targets)+len(o.SubTargets) != len(c.targets)+len(c.subTargets) {
			t.Fatalf("%s: precondition: spell on stack with flags %b and %d targets: %+v", c.name, c.flags, len(c.targets)+len(c.subTargets), o)
		}
		if got := trigmatch.ValidSAMatches(boardOf(e), src, ev, 0, c.clause); got != c.want {
			t.Errorf("%s: ValidSAMatches(%q) = %v, want %v", c.name, c.clause, got, c.want)
		}
	}
	for _, spec := range []string{"Spell.Kicked", "Spell.Warp", "Spell.numTargets GE1"} {
		if un := effects.UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty", spec, un)
		}
	}
}
