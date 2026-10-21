package neutral_test

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// combatObs is kernelObs in step at turn turn with a p0 Voldaren Epicure
// "o-att" attacking target; blocked puts p1's Epicure in the observation
// blocking it, and !blocked leaves that blocker off the battlefield.
func combatObs(t *testing.T, turn uint32, step, target string, blocked bool) *protocol.Observation {
	t.Helper()
	var o protocol.Observation
	if err := json.Unmarshal([]byte(kernelObs), &o); err != nil {
		t.Fatal(err)
	}
	o.Turn, o.PhaseStep = turn, step
	epi := o.Players[1].Battlefield[0]
	att := epi
	att.ObjectID, att.OwnerSeat, att.ControllerSeat = "o-att", "p0", "p0"
	pm := *epi.Permanent
	pm.Attacking, pm.AttackTarget = true, &protocol.TargetRef{Player: &target}
	att.Permanent = &pm
	o.Players[0].Battlefield = append(o.Players[0].Battlefield, att)
	if blocked {
		bp := *epi.Permanent
		bp.Blocking, bp.BlockedAttackers = true, []protocol.ObjectRef{att.ObjectRef}
		o.Players[1].Battlefield[0].Permanent = &bp
	} else {
		o.Players[1].Battlefield = nil
	}
	return &o
}

func attackerBlocks(t *testing.T, b *neutral.Builder, o *protocol.Observation) int {
	t.Helper()
	v, err := b.View(o)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range v.Players {
		for _, cv := range p.Battlefield {
			if cv.Attacking {
				return len(cv.BlockedBy)
			}
		}
	}
	t.Fatal("no attacker in the view")
	return 0
}

// TestBlockedAttackerStaysBlockedAfterItsBlockerLeaves: CR 509.1h. Once the
// builder has seen an attacker blocked, it stays blocked for the rest of
// that combat after the blocker leaves the battlefield (gorge's view keeps a
// zero tombstone), but not into a later combat or after it is re-pointed.
func TestBlockedAttackerStaysBlockedAfterItsBlockerLeaves(t *testing.T) {
	b := neutral.NewBuilder(testcorpus.Registry(t))
	steps := []struct {
		turn         uint32
		step, target string
		blocked      bool
		want         int
	}{
		{3, "declare_blockers", "p1", true, 1},
		{3, "combat_damage", "p1", false, 1}, // the blocker died: still blocked
		{3, "end_of_combat", "p1", false, 1},
		{3, "postcombat_main", "p1", false, 0}, // out of combat: the memory resets
		{3, "end_of_combat", "p1", false, 0},
		{5, "declare_blockers", "p1", true, 1},
		{5, "combat_damage", "p0", false, 0}, // re-pointed: gorge clears its blocks
		{7, "declare_blockers", "p1", true, 1},
		{9, "combat_damage", "p1", false, 0}, // a new turn resets
	}
	for _, s := range steps {
		if got := attackerBlocks(t, b, combatObs(t, s.turn, s.step, s.target, s.blocked)); got != s.want {
			t.Errorf("turn %d %s (target %s, blocker present %v): blocked by %d, want %d",
				s.turn, s.step, s.target, s.blocked, got, s.want)
		}
	}
}
