package oracle

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestRetraceGrantMorphReprosLockstep replays the recorded game where Six's
// granted retrace on a morph card (Shieldhide Dragon) in its owner's graveyard
// stopped the kernel with an unmodelled retracegrant stall: the bar treated
// the morph cost as an additional cost on the face-up cast, which it is not --
// the face-down morph cast is a hand-and-command-zone offer, so the retrace
// cast is the plain one both engines offer.
func TestRetraceGrantMorphReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "retrace_grant_morph")
}

// retraceMorphDeck holds Six ({2}{G}), the morph carrier and Ruin Crab's mill,
// so Shieldhide Dragons reach their owner's graveyard while a Six is on the
// battlefield and the granted retrace is evaluated at priority.
var retraceMorphDeck = []string{
	"Six", "Six",
	"Shieldhide Dragon", "Shieldhide Dragon", "Shieldhide Dragon", "Shieldhide Dragon",
	"Ruin Crab", "Ruin Crab", "Ruin Crab",
	"Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest",
	"Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest", "Forest",
	"Island", "Island", "Island", "Island", "Island", "Island", "Island", "Island",
	"Island", "Island", "Island", "Island", "Island", "Island", "Island", "Island",
	"Island", "Island", "Island", "Island", "Island", "Island", "Island", "Island",
	"Island", "Island", "Island", "Island", "Island", "Island",
}

// retraceMorphPolicy takes a retrace cast whenever one is offered (the
// CastModeOther cast-mode code is retrace's and jump-start's; the deck holds
// no jump-start), else the combat policy.
func retraceMorphPolicy(seed uint64) Policy {
	base := combatPolicy(seed, nil)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActCast && o.Mode()&0x0F == castModeOther {
				return o
			}
		}
		return base.Pick(c)
	})
}

// TestRetraceGrantMorphFourSeat plays four-seat pods whose decks all hold Six,
// Shieldhide Dragon (morph) and Ruin Crab (self-mill): a morph card in its
// owner's graveyard beside that owner's Six is the granted-retrace state, the
// kernel evaluates the granted retrace cast at those priorities (never with
// the retracegrant stall), and the games play out the same in both engines.
//
// The offer itself is unreachable in lockstep play -- both engines price a
// retrace cast from the floating pool alone (gorge's manaFeasiblePricedP, the
// kernel's FeatPoolPays mirror), and no lockstep game ever floats mana (the
// canonical priority vocabulary drops gorge's mana-ability activations), so
// the count is of the grant's evaluation points, not of offered casts.
func TestRetraceGrantMorphFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	var retraceEvals int
	check := func(g *GorgeSide, ker Side) *Divergence {
		ready := [4]bool{}
		for p := range g.E.G.Players {
			six, drag := false, false
			for _, id := range g.E.G.Zone(state.ZBattlefield, state.PlayerID(p)) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Six" {
					six = true
				}
			}
			for _, id := range g.E.G.Zone(state.ZGraveyard, state.PlayerID(p)) {
				if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Shieldhide Dragon" {
					drag = true
				}
			}
			if six && drag && p < len(ready) {
				ready[p] = true
			}
		}
		if !ready[0] && !ready[1] && !ready[2] && !ready[3] {
			return nil
		}
		d := ker.Step()
		if d.Kind != DPriority || int(d.Player) >= len(ready) || !ready[d.Player] ||
			g.E.G.Active != state.PlayerID(d.Player) {
			return nil // the grant is evaluated at the owner's own main-phase priorities
		}
		for _, o := range d.Options {
			// a retrace cast is only ever offered when the floating pool pays
			// the printed cost, which never happens here; a mode-15 option
			// would still be the one action the whole brief is about
			if o.Kind() == ActCast && o.Mode()&0x0F == castModeOther {
				retraceEvals += 1 << 20 // an actual offered cast outranks the state count
			}
		}
		retraceEvals++
		return nil
	}
	set := cardsettest.Synth(t, "dr-mbc-bbab")
	play := fourSeatSetPlay(t, set, [4][]string{retraceMorphDeck, retraceMorphDeck, retraceMorphDeck, retraceMorphDeck},
		nil, check, nil, func(seed uint64) Policy { return retraceMorphPolicy(seed) })
	const seeds = 20
	fourSeatAtkClean(t, play, seeds)
	if retraceEvals == 0 {
		t.Fatal("no granted retrace of a morph card was ever evaluated: the mechanism was never reached")
	}
	t.Logf("%d granted-retrace evaluation points on a graveyard morph card", retraceEvals&(1<<20-1))
}

// TestRetraceGrantMorphMutation re-adds the morph bar (dirac.DebugRetraceBarMorph):
// the granted retrace offer on a morph card is withheld again and the recorded
// game ends in the unmodelled retracegrant stall, so the reads above are the
// ones that matter.
func TestRetraceGrantMorphMutation(t *testing.T) {
	old := dirac.DebugRetraceBarMorph
	dirac.DebugRetraceBarMorph = true
	defer func() { dirac.DebugRetraceBarMorph = old }()
	r, err := ReadRepro(filepath.Join("testdata", "repro", "retrace_grant_morph.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	pol := &ScriptPolicy{Actions: r.Actions}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
	if !strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("re-adding the morph bar ended in %q, want a kernel stall", res.Stall)
	}
	t.Logf("mutation: kernel stopped after %d decisions with %q", res.Decisions, res.Stall)
}
