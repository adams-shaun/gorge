package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestTurnUpAttachEffect plays Gift of Doom's turn-up attach replacement
// (dirac/face_turnupattach.go) when an EFFECT turns the permanent up, in
// lockstep with gorge, over the turnupattacheff fixture pool:
//
//   - Break Open turns up one targeted face-down creature (kop.ExtTurnFaceUp
//     C 0) and Zimone's landfall picks one of its controller's face-down
//     permanents (C 1, gorge's "setstate_pick"). When the turned-up
//     permanent is Gift of Doom the attach election is posed while it is
//     still face down, and its answer continues the resolution: the
//     permanent turns up, a Zimone pick's program goes on exactly once.
//   - Every other face-down permanent (Manifest Dread / Soul Summons
//     manifests, a Bubble Smuggler cast face down) turns up with NO
//     election, though the table's turn-up flag is set by Gift of Doom,
//     Karlov Watchdog and Bubble Smuggler alike: the kernel poses the
//     election only for a card holding the replacement.
//
// The policy never takes the turn-face-up special action, so every turn-up
// in these games is an effect's. Preconditions: Gift of Doom and other
// permanents are turned up by effects, and Gift of Doom's election is
// answered both ways. The mutation swaps the replacement for an empty
// program and must diverge.
func TestTurnUpAttachEffect(t *testing.T) {
	if isolate(t) {
		return
	}
	t.Run("lockstep", func(t *testing.T) {
		run := runTurnUpAttachEffect(t, 160, true, nil)
		t.Logf("%d games, %d divergences, %d stalls; %v", run.games, run.divs, run.stalls, run.casts)
		for _, c := range []struct {
			key  string
			want int
		}{
			{"Gift of Doom#up", 6}, {"Gift of Doom#attached", 2}, {"Gift of Doom#bare", 2}, {"other#up", 10},
			{"Break Open#cast", 4},
		} {
			if run.casts[c.key] < c.want {
				t.Errorf("%s = %d, want at least %d", c.key, run.casts[c.key], c.want)
			}
		}
	})
	t.Run("TurnUpAttach/mutation", func(t *testing.T) {
		run := runTurnUpAttachEffect(t, 48, false, func(t *testing.T, tbl *kop.Table) {
			n := 0
			for i := range tbl.Cards {
				cd := &tbl.Cards[i]
				if cd.Name != "Gift of Doom" {
					continue
				}
				cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
				for j := range cd.Abilities {
					if cd.Abilities[j].Kind == kop.AbReplacement && cd.Abilities[j].Repl.Kind == kop.ReplTurnUpAttach {
						cd.Abilities[j].Repl.Kind = kop.ReplTurnUpProgram // an empty program: no election
						n++
					}
				}
			}
			if n != 1 {
				t.Fatalf("Gift of Doom: %d ReplTurnUpAttach replacements, want 1", n)
			}
		})
		run.requireDivergence(t, "an effect turn-up with no attach election")
	})
}

// turnUpAttachEffectDecks is the one matchup: both seats play the same 40.
var turnUpAttachEffectDecks = []fixtureDecks{{
	{{Name: "Gift of Doom", Count: 4}, {Name: "Break Open", Count: 3}, {Name: "Zimone, Mystery Unraveler", Count: 3}, {Name: "Karlov Watchdog", Count: 3}, {Name: "Bubble Smuggler", Count: 3}, {Name: "Manifest Dread", Count: 3}, {Name: "Soul Summons", Count: 2}, {Name: "Memnite", Count: 2}, {Name: "Rampant Growth", Count: 2}, {Name: "Evolving Wilds", Count: 2}, {Name: "Mox Emerald", Count: 1}, {Name: "Mox Ruby", Count: 1}, {Name: "Mox Pearl", Count: 1}, {Name: "Mox Sapphire", Count: 1}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 3}},
	{{Name: "Gift of Doom", Count: 4}, {Name: "Break Open", Count: 3}, {Name: "Zimone, Mystery Unraveler", Count: 3}, {Name: "Karlov Watchdog", Count: 3}, {Name: "Bubble Smuggler", Count: 3}, {Name: "Manifest Dread", Count: 3}, {Name: "Soul Summons", Count: 2}, {Name: "Memnite", Count: 2}, {Name: "Rampant Growth", Count: 2}, {Name: "Evolving Wilds", Count: 2}, {Name: "Mox Emerald", Count: 1}, {Name: "Mox Ruby", Count: 1}, {Name: "Mox Pearl", Count: 1}, {Name: "Mox Sapphire", Count: 1}, {Name: "Forest", Count: 3}, {Name: "Island", Count: 3}, {Name: "Mountain", Count: 3}},
}}

// turnUpAttachEffectPolicy never takes the turn-face-up special action;
// seven times in eight it takes the first offered cast of the Moxen, then a
// face-down cast, then any other cast (so the face-down permanents and Break
// Open's targets exist), else it plays randomly. Each turn-up gorge logged
// since the last decision is counted by the card turned up, and a turned-up
// Gift of Doom is counted attached or bare as of the next decision.
func turnUpAttachEffectPolicy(seed uint64, casts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7461656666)) // "taeff"
	seen := 0
	var gifts []events.Event
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return rnd.Pick(c)
		}
		evs := c.G.E.L.Events
		for _, g := range gifts {
			if o := c.G.E.G.Obj(g.Obj); o != nil && o.AttachedTo != 0 {
				casts["Gift of Doom#attached"]++
			} else {
				casts["Gift of Doom#bare"]++
			}
		}
		gifts = gifts[:0]
		for ; seen < len(evs); seen++ {
			if evs[seen].Kind != events.TurnFaceUp {
				continue
			}
			o := c.G.E.G.Obj(evs[seen].Obj)
			if o != nil && o.Face() != nil && o.Face().Name == "Gift of Doom" {
				casts["Gift of Doom#up"]++
				gifts = append(gifts, evs[seen])
			} else {
				casts["other#up"]++
			}
		}
		var opts []Canon
		for _, a := range c.D.Options {
			if a.Kind() == ActSpecial {
				continue // the special action turn-up is the morphpick test's
			}
			opts = append(opts, a)
		}
		if len(opts) == 0 {
			return rnd.Pick(c)
		}
		if c.D.Kind == DPriority && prng.IntN(8) != 0 {
			for _, w := range [...]struct {
				k    CanonKind
				only func(n string) bool
			}{
				{ActCast, func(n string) bool { return len(n) > 4 && n[:4] == "Mox " }},
				{ActCastFace, func(string) bool { return true }},
				{ActCast, func(string) bool { return true }},
			} {
				for _, a := range opts {
					if a.Kind() != w.k {
						continue
					}
					if n := refCardName(c.G, a.Src()); n != "" && w.only(n) {
						if n == "Break Open" {
							casts["Break Open#cast"]++
						}
						return a
					}
				}
			}
		}
		return opts[prng.IntN(len(opts))]
	})
}

// runTurnUpAttachEffect plays seeds 1..seeds of the matchup; mutate, when
// set, rewrites the table first.
func runTurnUpAttachEffect(t *testing.T, seeds int, report bool, mutate func(*testing.T, *kop.Table)) fixtureRun {
	t.Helper()
	var mut func(*kop.Table)
	if mutate != nil {
		mut = func(tbl *kop.Table) { mutate(t, tbl) }
	}
	return runFixtureLockstepPolicy(t, "turnupattacheff", turnUpAttachEffectDecks, seeds, report, mut, turnUpAttachEffectPolicy)
}
