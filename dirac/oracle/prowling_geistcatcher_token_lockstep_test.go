package oracle

// Prowling Geistcatcher's token-sacrifice lockstep, the proof its
// dirac/support/unrefused.tsv row cites: the trigger's "If that creature was a
// token" gate must put the +1/+1 counter exactly when the sacrificed creature
// WAS a token, judged after its own ChangeZone sub has already exiled it
// (CR 608.2h). The probe casts Vampiric Rites, Kozilek's Predator (its enters
// trigger makes two Eldrazi Spawn tokens, so the deck holds both arms' raw
// material) and Prowling Geistcatcher, then sacrifices an Eldrazi Spawn token
// to the Rites and Kozilek's Predator itself, asserting gorge puts one counter
// for the token and none for the nontoken while the kernel agrees throughout
// (no divergence: the kernel's RefTriggerObj read of the exiled creature and
// gorge's gate answer identically on both arms).

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

func geistLockstepDecks() [2][]cardset.Entry {
	d0 := []cardset.Entry{
		{Name: "Prowling Geistcatcher", Count: 1},
		{Name: "Kozilek's Predator", Count: 4},
		{Name: "Vampiric Rites", Count: 4},
		{Name: "Forest", Count: 15},
		{Name: "Swamp", Count: 16},
	}
	return [2][]cardset.Entry{d0, {{Name: "Mountain", Count: 40}}}
}

// geistProbe is what the policy observed about the two arms.
type geistProbe struct {
	pendingSac     bool // a Rites activation was picked: its sacrifice ask is next
	tokenPick      bool // the sacrifice ask was answered with an Eldrazi Spawn token
	nontokenPick   bool // ... with Kozilek's Predator (a nontoken creature)
	counters       int  // P1P1 counters on Prowling Geistcatcher, last observed
	sawCounter     bool // a counter was observed after the token pick
	held           bool // no counter was observed after the nontoken pick
	beforeNontoken int  // the counter count when the nontoken sacrifice was picked
	castRites      int  // casts and activations the policy picked, for the reach log
	castPredator   int
	castGeist      int
	sacs           int
}

// geistLockstepName is the face name of a gorge object, "" when it has none.
func geistLockstepName(c *PickCtx, id state.ObjID) string {
	if c.G == nil {
		return ""
	}
	ob := c.G.E.G.Obj(id)
	if ob == nil || ob.Face() == nil {
		return ""
	}
	return ob.Face().Name
}

// geistLockstepFind is p's first battlefield object matching pred.
func geistLockstepFind(c *PickCtx, p state.PlayerID, pred func(*state.Object) bool) (state.ObjID, bool) {
	if c.G == nil {
		return 0, false
	}
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
		if ob := c.G.E.G.Obj(id); ob != nil && pred(ob) {
			return id, true
		}
	}
	return 0, false
}

func geistLockstepCounters(c *PickCtx, id state.ObjID) int {
	if c.G == nil {
		return 0
	}
	ob := c.G.E.G.Obj(id)
	if ob == nil {
		return 0
	}
	n := 0
	for _, ct := range ob.Counters {
		if ct.Kind == "P1P1" {
			n += int(ct.N)
		}
	}
	return n
}

// geistLockstepOption is the canon whose gorge option matches pred.
func geistLockstepOption(c *PickCtx, pred func(*decision.Option) bool) (Canon, bool) {
	for _, a := range c.D.Options {
		o, ok := c.G.GorgeOption(a)
		if ok && pred(&o) {
			return a, true
		}
	}
	return 0, false
}

func geistLockstepPass(c *PickCtx) Canon {
	for _, a := range c.D.Options {
		if k := a.Kind(); k == ActPass || k == ActDone {
			return a
		}
	}
	if len(c.D.Options) > 0 {
		return c.D.Options[0]
	}
	return 0
}

// geistLockstepSacTarget is the creature the pending Rites sacrifice names:
// an Eldrazi Spawn token until the token arm is picked, then Kozilek's
// Predator for the nontoken arm (Geistcatcher itself is never a target).
func geistLockstepSacTarget(c *PickCtx, probe *geistProbe) (state.ObjID, bool) {
	spawn, hasSpawn := geistLockstepFind(c, 0, func(ob *state.Object) bool {
		return ob.IsToken && ob.Face() != nil && ob.Face().Name == "Eldrazi Spawn Token"
	})
	if !probe.tokenPick && hasSpawn {
		return spawn, true
	}
	pred, hasPred := geistLockstepFind(c, 0, func(ob *state.Object) bool {
		return ob.Face() != nil && ob.Face().Name == "Kozilek's Predator"
	})
	if hasPred {
		return pred, false
	}
	if hasSpawn {
		return spawn, true
	}
	return 0, false
}

func geistLockstepPolicy(probe *geistProbe) Policy {
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G == nil {
			return geistLockstepPass(c)
		}
		// The sacrifice ask a Rites activation's Sac<1/Creature> cost poses:
		// the token arm answers with an Eldrazi Spawn, the nontoken arm with
		// Kozilek's Predator itself.
		if probe.pendingSac && (c.D.Kind == DChoose || c.D.Kind == DTarget) {
			want, isSpawn := geistLockstepSacTarget(c, probe)
			if want != 0 {
				for _, a := range c.D.Options {
					if o, ok := c.G.GorgeOption(a); ok && o.Obj == want {
						probe.pendingSac = false
						if isSpawn {
							probe.tokenPick = true
						} else {
							probe.beforeNontoken = probe.counters
							probe.nontokenPick = true
						}
						probe.sacs++
						return a
					}
				}
			}
			probe.pendingSac = false
			return geistLockstepPass(c)
		}
		if c.D.Kind != DPriority || c.D.Player != 0 {
			return geistLockstepPass(c)
		}
		geist, hasGeist := geistLockstepFind(c, 0, func(ob *state.Object) bool {
			return ob.Face() != nil && ob.Face().Name == "Prowling Geistcatcher"
		})
		if hasGeist {
			probe.counters = geistLockstepCounters(c, geist)
			if probe.tokenPick && probe.counters > 0 {
				probe.sawCounter = true
			}
			if probe.nontokenPick && probe.counters == probe.beforeNontoken {
				probe.held = true
			}
		}
		if probe.nontokenPick {
			// both arms observed: the game has served its purpose
			return geistLockstepPass(c)
		}
		rites, hasRites := geistLockstepFind(c, 0, func(ob *state.Object) bool {
			return ob.Face() != nil && ob.Face().Name == "Vampiric Rites"
		})
		_, hasPred := geistLockstepFind(c, 0, func(ob *state.Object) bool {
			return ob.Face() != nil && ob.Face().Name == "Kozilek's Predator"
		})
		_, hasSpawn := geistLockstepFind(c, 0, func(ob *state.Object) bool {
			return ob.IsToken && ob.Face() != nil && ob.Face().Name == "Eldrazi Spawn Token"
		})
		if hasGeist && hasRites && hasSpawn && probe.counters == 0 && !probe.tokenPick {
			if a, ok := geistLockstepOption(c, func(o *decision.Option) bool {
				return o.Obj == rites
			}); ok {
				probe.pendingSac = true
				return a
			}
		}
		if hasGeist && hasRites && hasPred && probe.sawCounter && probe.counters == 1 {
			if a, ok := geistLockstepOption(c, func(o *decision.Option) bool {
				return o.Obj == rites
			}); ok {
				probe.pendingSac = true
				return a
			}
		}
		if !hasRites {
			if a, ok := geistLockstepCast(c, "Vampiric Rites"); ok {
				probe.castRites++
				return a
			}
		}
		if !hasPred {
			if a, ok := geistLockstepCast(c, "Kozilek's Predator"); ok {
				probe.castPredator++
				return a
			}
		}
		if !hasGeist {
			if a, ok := geistLockstepCast(c, "Prowling Geistcatcher"); ok {
				probe.castGeist++
				return a
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		return geistLockstepPass(c)
	})
}

// geistLockstepCast is the cast option of the hand card named want.
func geistLockstepCast(c *PickCtx, want string) (Canon, bool) {
	for _, a := range c.D.Options {
		if k := a.Kind(); k != ActCast && k != ActCastFace {
			continue
		}
		o, ok := c.G.GorgeOption(a)
		if ok && geistLockstepName(c, o.Obj) == want {
			return a, true
		}
	}
	return 0, false
}

func TestProwlingGeistcatcherTokenSacrificeAgrees(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	decks := geistLockstepDecks()
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(4); seed <= 60; seed += 8 {
		m, err := NewMatchup(reg, [2]string{"prowling-geistcatcher-token-a", "prowling-geistcatcher-token-b"}, gd, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: true}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &geistProbe{}
		res := Lockstep(m, seed, k, geistLockstepPolicy(probe), RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 60})
		CloseSide(k)
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		if res.Div != nil {
			t.Fatalf("seed %d diverged (tokenPick=%v nontokenPick=%v): %+v", seed, probe.tokenPick, probe.nontokenPick, res.Div)
		}
		if probe.tokenPick && probe.sawCounter && probe.nontokenPick && probe.held {
			t.Logf("seed %d: token sacrifice put one counter, nontoken held, kernel agreed", seed)
			return
		}
		t.Logf("seed %d: reach log: rites=%d predator=%d geist=%d sacs=%d tokenPick=%v nontokenPick=%v sawCounter=%v held=%v counters=%d",
			seed, probe.castRites, probe.castPredator, probe.castGeist, probe.sacs, probe.tokenPick, probe.nontokenPick, probe.sawCounter, probe.held, probe.counters)
	}
	t.Fatal("no seed reached both arms: the probe proves nothing")
}
