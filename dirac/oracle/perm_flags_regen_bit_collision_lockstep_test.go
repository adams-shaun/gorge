package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPermFlagsRegenBitCollisionReprosLockstep replays the recorded game where
// Rabbit Battery reconfigures onto a creature holding three or more
// regeneration shields: the shields beyond the first no longer share
// perm.flags bits with pfBatchGone and pfLastBear (CR 701.19a: shields stack
// until used or cleanup).
func TestPermFlagsRegenBitCollisionReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "perm_flags_regen_bit_collision")
}

// permFlagsRegenDeck holds the attachments that can hold shields (Rabbit
// Battery reconfigures; Gaea's Embrace gives repeatable regeneration), green
// mana for the shields and red for the reconfigure. Unicycle and Specter's Shroud are not in
// the dr-mbc-aaba pool.
var permFlagsRegenDeck = []string{
	"Rabbit Battery", "Rabbit Battery", "Rabbit Battery", "Rabbit Battery", "Gaea's Embrace", "Gaea's Embrace", "Gaea's Embrace", "Gaea's Embrace",
	"Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest",
	"Mountain", "Mountain", "Mountain", "Mountain", "Mountain", "Mountain",
}

// permFlagsRegenChooser stacks regeneration shields with Gaea's Embrace until
// the creature it enchants holds three, then activates that creature's own
// ability (Rabbit Battery's reconfigure) so it re-attaches holding them.
func permFlagsRegenChooser(c *PickCtx) (Canon, bool) {
	if c.G == nil || c.G.E.G == nil {
		return 0, false
	}
	g := c.G.E.G
	for _, o := range c.D.Options {
		if o.Kind() != ActActivate {
			continue
		}
		opt, ok := c.G.GorgeOption(o)
		if !ok || opt.Obj == 0 {
			continue
		}
		src := g.Obj(opt.Obj)
		if src == nil || src.Face() == nil {
			continue
		}
		switch src.Face().Name {
		case "Gaea's Embrace":
			if host := g.Obj(src.AttachedTo); host != nil && host.Counter("Shield") < 3 {
				return o, true
			}
		case "Rabbit Battery":
			if src.AttachedTo == 0 && src.Counter("Shield") >= 3 {
				return o, true
			}
		}
	}
	return 0, false
}

// TestPermFlagsRegenBitCollisionFourSeat plays four-seat pods whose decks all
// hold Rabbit Battery and Gaea's Embrace: an
// attachment holding three or more shields that re-attaches must keep them in
// both engines, and the games reach that.
func TestPermFlagsRegenBitCollisionFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var reached int
	var last = map[state.ObjID]int32{}
	check := func(g *GorgeSide, _ Side) *Divergence {
		for i := range g.E.G.Objs {
			o := &g.E.G.Objs[i]
			if o == nil || o.Zone != state.ZBattlefield {
				continue
			}
			n := o.Counter("Shield")
			if o.AttachedTo != 0 && n >= 3 && last[o.ID] != int32(o.AttachedTo) {
				reached++
			}
			last[o.ID] = int32(o.AttachedTo)
		}
		return nil
	}
	set := cardsettest.Synth(t, "dr-mbc-aaba")
	play := fourSeatSetPlay(t, set, [4][]string{permFlagsRegenDeck, permFlagsRegenDeck, permFlagsRegenDeck, permFlagsRegenDeck},
		nil, check, nil, func(seed uint64) Policy { return combatPolicy(seed, permFlagsRegenChooser) })
	for seed := uint64(1); seed <= 20; seed++ {
		if res := play(seed); res.Div != nil {
			d := res.Div
			t.Errorf("seed %d: diverged at decision %d: %s %s %v", seed, d.At, d.What, d.Field, d.Diff)
		}
	}
	if reached == 0 {
		t.Fatal("no attachment holding three shields re-attached: the mechanism was never reached")
	}
	t.Logf("%d re-attachments of an attachment holding three or more shields", reached)
}

// TestPermFlagsRegenBitCollisionMutation keeps the extra shields in
// perm.flags again: the recorded game then diverges.
func TestPermFlagsRegenBitCollisionMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	old := dirac.DebugRegenShieldsInFlags
	dirac.DebugRegenShieldsInFlags = true
	defer func() { dirac.DebugRegenShieldsInFlags = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "perm_flags_regen_bit_collision.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if res.Div == nil {
		t.Fatal("flag-bit mutation replayed the recorded game with no divergence")
	}
	t.Logf("mutation diverged at %d: %s", res.Div.At, res.Div.What)
}
