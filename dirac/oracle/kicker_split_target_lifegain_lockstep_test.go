package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Expel the Unworthy's chained ChangeZone shape: the root's TargetMin$/Max$
// resolve to 0 when kicked, so the Kicked-only sub link chooses the creature
// the trailing D BGainLife must read through Defined$ TargetedController and
// Targeted$CardManaCost. The recordings replay games where the gain read the
// wrong target (kicked: the root's empty slot; unkicked: the sub's absent
// slot), so the two-seat lockstep pins both arms.
func TestKickerSplitTargetLifeGainReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "kicker_split_target_lifegain")
	stdReproNamed(t, "kicker_split_target_lifegain_b")
}

// kickerSplitKickDeck is seats 0/2: Expel the Unworthy with kicker-capable
// mana and vanilla creatures spanning mana values 1-6 (the exiled creature's
// controller gains its mana value). kickerSplitPlainDeck is seats 1/3: the
// same Expel beside lands only, so the plain cast is what those seats spend
// their mana on while the other seats furnish the targets.
func kickerSplitKickDeck() []string {
	return deckOf(8, "Expel the Unworthy",
		deckOf(2, "Narnam Renegade",
			deckOf(2, "Griffin Sentinel",
				deckOf(2, "Master Trinketeer",
					deckOf(2, "Cat Burglar",
						deckOf(2, "Nessian Game Warden",
							deckOf(2, "Gargos, Vicious Watcher",
								deckOf(10, "Plains",
									deckOf(8, "Forest",
										deckOf(2, "Swamp")...)...)...)...)...)...)...)...)...)
}

func kickerSplitPlainDeck() []string {
	return deckOf(8, "Expel the Unworthy",
		deckOf(16, "Plains",
			deckOf(16, "Forest")...)...)
}

// kickerSplitTally counts the mechanism's resolutions per game and splits
// them by the cast's kicked flag: an Expel put on the stack is classified by
// its target event ("sub-target" is the Kicked-only second ChangeZone link;
// the plain cast's root target is the other spelling). A gain is paired with
// the oldest pending cast: nothing else in these pods gains life.
type kickerSplitTally struct {
	cur         *GorgeSide
	seen        int
	pushes      map[state.ObjID]bool
	pending     []bool
	kicked      int
	unkicked    int
	allKicked   int
	allUnkicked int
}

func (k *kickerSplitTally) reset(side *GorgeSide) {
	if k.cur != nil {
		k.allKicked += k.kicked
		k.allUnkicked += k.unkicked
	}
	k.cur, k.seen = side, 0
	k.pushes = map[state.ObjID]bool{}
	k.pending = nil
	k.kicked, k.unkicked = 0, 0
}

func (k *kickerSplitTally) totals() (int, int) {
	return k.allKicked + k.kicked, k.allUnkicked + k.unkicked
}

func (k *kickerSplitTally) check(ref *GorgeSide, _ Side) *Divergence {
	if ref != k.cur {
		k.reset(ref)
	}
	evs := ref.E.L.Events
	for ; k.seen < len(evs); k.seen++ {
		ev := evs[k.seen]
		switch ev.Kind {
		case events.PutOnStack:
			if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && o.Face().Name == "Expel the Unworthy" {
				k.pushes[ev.Obj] = true
			}
		case events.TargetsChosen:
			if k.pushes[ev.Obj] {
				k.pending = append(k.pending, ev.Text == "sub-target")
				delete(k.pushes, ev.Obj)
			}
		case events.LifeChange:
			if ev.Amount > 0 && len(k.pending) > 0 {
				kicked := k.pending[0]
				k.pending = k.pending[1:]
				if kicked {
					k.kicked++
				} else {
					k.unkicked++
				}
			}
		}
	}
	return nil
}

// kickerSplitPolicy casts Expel the Unworthy whenever it can, forcing the
// kicked cast on seats 0/2 (canonical cast mode 1, which only Expel offers in
// this pod) and the plain cast on seats 1/3, so both gain arms are reached
// whether gorge or the kernel leads the decision. On a kernel-led seat the
// plain arm is the mode-0 cast of the same source that also offers its
// mode-1 (kicked) cast; everything else is combatPolicy's answer.
func kickerSplitPolicy(seed uint64) Policy {
	base := combatPolicy(seed, nil)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DPriority {
			if c.D.Player%2 == 0 {
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && a.Mode()&0x0f == 1 {
						return a
					}
				}
			} else {
				if c.G != nil {
					for _, a := range c.D.Options {
						if a.Kind() == ActCast && a.Mode()&0x0f == 0 && optionFaceName(c, a) == "Expel the Unworthy" {
							return a
						}
					}
				}
				kicked := map[Ref]bool{}
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && a.Mode()&0x0f == 1 {
						kicked[a.Src()] = true
					}
				}
				for _, a := range c.D.Options {
					if a.Kind() == ActCast && a.Mode()&0x0f == 0 && kicked[a.Src()] {
						return a
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// TestKickerSplitTargetLifeGainFourSeat plays 20 four-seat pods and requires
// no divergence. The check hook counts each resolution of the mechanism, so
// the no-divergence claim cannot pass without Expel being cast both kicked
// and unkicked.
func TestKickerSplitTargetLifeGainFourSeat(t *testing.T) {
	if isolate(t) {
		return
	}
	kick, plain := kickerSplitKickDeck(), kickerSplitPlainDeck()
	set := cardsettest.Synth(t, "tail-25")
	tally := &kickerSplitTally{}
	play := fourSeatSetPlay(t, set, [4][]string{kick, plain, kick, plain},
		nil, tally.check, nil, kickerSplitPolicy)
	fourSeatAtkClean(t, play, 20)
	kicked, unkicked := tally.totals()
	if kicked == 0 || unkicked == 0 {
		t.Fatalf("the kicked and unkicked Expel gain were not both reached in 20 pods: kicked=%d unkicked=%d", kicked, unkicked)
	}
	t.Logf("Expel gains resolved: kicked=%d unkicked=%d", kicked, unkicked)
}

// TestKickerSplitTargetLifeGainMutation rebuilds the recording's table with
// the pre-fix lowering (DebugExpelGainTarget1) and requires the divergence
// the recording captured: the gain reads target slot 1 whatever the kicker
// decided.
func TestKickerSplitTargetLifeGainMutation(t *testing.T) {
	r, err := ReadRepro(filepath.Join("testdata", "repro", "kicker_split_target_lifegain.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugExpelGainTarget1
	dirac.DebugExpelGainTarget1 = true
	defer func() { dirac.DebugExpelGainTarget1 = old }()
	res := replayDecodedScript(t, synthReg(t), printedDelveSet(t, r.Cards), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div == nil {
		t.Fatalf("mutation: the pre-fix lowering did not diverge")
	}
	t.Logf("mutation: diverged at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
