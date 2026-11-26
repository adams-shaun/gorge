package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// sp3Run plays seeds 1..n of one constructed matchup over a fixture pool,
// gorge leading, failing on each divergence or unmodelled stall (unless
// mutated: the first divergence is returned instead). hit is called at every
// pick with gorge's side and reports whether the scenario under test is live
// there; the count of games where it was is returned. mutate, when set,
// rewrites the matchup's table first.
func sp3Run(t *testing.T, fixture string, kd [2][]cardset.Entry, pol func(uint64) Policy, n uint64, mutated bool, mutate func(*kop.Table), hit func(*GorgeSide) bool) (*Divergence, int) {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kd {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for range e.Count {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(kd[0], kd[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	hits := 0
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sp3-a", "sp3-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		seen := false
		check := func(ref *GorgeSide, _ Side) *Divergence {
			if !seen && hit(ref) {
				seen = true
			}
			return nil
		}
		res := LockstepLed(m, seed, g, k, pol(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 30, Check: check})
		CloseSide(k)
		if seen {
			hits++
		}
		if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil && mutated {
			return d, hits
		} else if d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return nil, hits
}

// topOfStackName is the face name of the object on top of gorge's stack.
func topOfStackName(ref *GorgeSide) string {
	st := ref.E.G.Stack
	if len(st) == 0 {
		return ""
	}
	o := ref.E.G.Obj(st[len(st)-1])
	if o != nil && o.Face() == nil && o.Source != 0 {
		o = ref.E.G.Obj(o.Source) // an ability: its source's name
	}
	if o != nil && o.Face() != nil {
		return o.Face().Name
	}
	return ""
}

// The ImmediateTrigger OptionalDecider$ lockstep: Dream Eater's "When you
// do, you may return target nonland permanent an opponent controls" is an
// optional reflexive trigger (gorge effImmediateTrigger's
// TriggerContext.OptionalSpec, CR 603.5): its controller is asked as it
// resolves. kgen marks the latent kop.TfOptional; the kernel's resolve asks
// it (raOptional).
var sp3ReflexDecks = [2][]cardset.Entry{
	{{Name: "Dream Eater", Count: 10}, {Name: "Wind Drake", Count: 8}, {Name: "Island", Count: 22}},
	{{Name: "Grizzly Bears", Count: 22}, {Name: "Forest", Count: 18}},
}

// sp3ReflexAsked: gorge poses the optional ask with Dream Eater's reflexive
// ability on top of the stack.
func sp3ReflexAsked(ref *GorgeSide) bool {
	d := ref.E.Pending()
	return d != nil && d.Kind == decision.KTriggerOptional && topOfStackName(ref) == "Dream Eater"
}

// TestReflexOptionalLowers: random and explore policies, no divergence and
// no unmodelled stall, and the reflexive "you may" is asked on gorge's side.
func TestReflexOptionalLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	_, a := sp3Run(t, "reflexoptional", sp3ReflexDecks, pkPolicy, 24, false, nil, sp3ReflexAsked)
	_, b := sp3Run(t, "reflexoptional", sp3ReflexDecks, ExplorePolicy, 24, false, nil, sp3ReflexAsked)
	t.Logf("Dream Eater's reflexive ask posed in %d random and %d explore games", a, b)
	if a+b < 4 {
		t.Errorf("Dream Eater's optional reflexive trigger was asked in %d games, want at least 4", a+b)
	}
}

// TestReflexOptionalMutation: with kop.TfOptional cleared from Dream Eater's
// reflexive latent the kernel resolves it unasked and diverges, so
// TestReflexOptionalLowers does not pass vacuously.
func TestReflexOptionalMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	mut := func(tbl *kop.Table) {
		if mutateCard(t, tbl, "Dream Eater", func(a *kop.Ability, _ *kop.Instr) {
			if a.AFlags&kop.AfLatent != 0 {
				a.Trig.Flags &^= kop.TfOptional
			}
		}) == 0 {
			t.Fatal("no TfOptional latent on Dream Eater")
		}
	}
	if d, _ := sp3Run(t, "reflexoptional", sp3ReflexDecks, ExplorePolicy, 24, true, mut, sp3ReflexAsked); d == nil {
		t.Error("no divergence with the reflexive trigger made mandatory")
	}
}

// The NoColoredMana$ lockstep: Void Mirror's SpellCast trigger fires only for
// a cast that spent no coloured mana (gorge trigmatch SpellCastEval over
// Object.ManaColorSpent; kernel kop.Tf5CastNoColoredMana over the spell
// entry's spc). Both decks mix Wastes with a coloured basic and colourless
// with coloured spells, so a mirror sees both kinds of cast.
var sp3MirrorDecks = [2][]cardset.Entry{
	{{Name: "Void Mirror", Count: 6}, {Name: "Memnite", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Wastes", Count: 10}, {Name: "Forest", Count: 10}},
	{{Name: "Void Mirror", Count: 4}, {Name: "Ornithopter", Count: 4}, {Name: "Wind Drake", Count: 8}, {Name: "Memnite", Count: 4}, {Name: "Wastes", Count: 10}, {Name: "Island", Count: 10}},
}

// sp3MirrorStats are the scenario counts read from gorge's events.
type sp3MirrorStats struct{ fired, colouredCasts int }

// sp3MirrorObserver counts, from gorge's events, Void Mirror's triggers put
// on the stack and coloured-spell casts made while a Void Mirror was on the
// battlefield (casts the gate must turn away).
func sp3MirrorObserver(st *sp3MirrorStats) func(*GorgeSide) bool {
	seen := 0
	var last *GorgeSide
	return func(ref *GorgeSide) bool {
		if ref != last {
			last, seen = ref, 0
		}
		g := ref.E.G
		evs := ref.E.L.Events
		hit := false
		for ; seen < len(evs); seen++ {
			ev := evs[seen]
			o := g.Obj(ev.Obj)
			if o == nil || o.Face() == nil {
				continue
			}
			switch {
			case ev.Kind == events.TriggerPush && o.Face().Name == "Void Mirror":
				st.fired++
				hit = true
			case ev.Kind == events.PutOnStack && (o.Face().Name == "Grizzly Bears" || o.Face().Name == "Wind Drake") && sp3MirrorOut(g):
				st.colouredCasts++
			}
		}
		return hit
	}
}

// sp3MirrorOut reports whether a Void Mirror is on gorge's battlefield.
func sp3MirrorOut(g *state.Game) bool {
	for _, p := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, p) {
			if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Void Mirror" {
				return true
			}
		}
	}
	return false
}

// TestNoColoredManaLowers: random and explore policies, no divergence and no
// unmodelled stall; Void Mirror's trigger fired, and coloured spells were
// cast past a Void Mirror, on gorge's side.
func TestNoColoredManaLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	var st sp3MirrorStats
	obs := sp3MirrorObserver(&st)
	sp3Run(t, "nocoloredmana", sp3MirrorDecks, pkPolicy, 24, false, nil, obs)
	sp3Run(t, "nocoloredmana", sp3MirrorDecks, ExplorePolicy, 24, false, nil, obs)
	t.Logf("Void Mirror triggers %d, coloured casts past a mirror %d", st.fired, st.colouredCasts)
	if st.fired < 4 || st.colouredCasts < 4 {
		t.Errorf("Void Mirror fired %d times and saw %d coloured casts, want at least 4 of each", st.fired, st.colouredCasts)
	}
}

// TestNoColoredManaMutation: with the gate off (dirac.DebugNoColoredManaOff)
// a coloured cast triggers the mirror in the kernel alone, so the lockstep
// diverges and TestNoColoredManaLowers does not pass vacuously.
func TestNoColoredManaMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoColoredManaOff = true
	defer func() { dirac.DebugNoColoredManaOff = false }()
	var st sp3MirrorStats
	if d, _ := sp3Run(t, "nocoloredmana", sp3MirrorDecks, ExplorePolicy, 24, true, nil, sp3MirrorObserver(&st)); d == nil {
		t.Error("no divergence with the NoColoredMana$ gate off")
	}
}

// The LethalDamageByPower$ lockstep: under Zilortha, Strength Incarnate a
// creature its controller controls dies to damage equal to its power, not its
// toughness (gorge LethalDamageThreshold; kernel kop.StatLethalByPower):
// Zilortha itself (7/3) survives a Lightning Bolt, Wall of Wood (0/3) dies to
// any damage. The red-green decks fight with burn and creatures of unequal
// power and toughness.
var sp3LethalDecks = [2][]cardset.Entry{
	{{Name: "Zilortha, Strength Incarnate", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Wall of Wood", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9}},
	{{Name: "Lightning Bolt", Count: 8}, {Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9}},
}

// sp3LethalLive: on gorge's battlefield a creature carries marked damage at
// least its toughness and is still alive (its lethal threshold is its power).
func sp3LethalLive(ref *GorgeSide) bool {
	e := ref.E
	for _, p := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if o := e.G.Obj(id); o != nil && o.Damage > 0 && e.IsCreature(id) && o.Damage >= e.Toughness(id) {
				return true
			}
		}
	}
	return false
}

// TestLethalByPowerLowers: random and explore policies, no divergence and no
// unmodelled stall, and on gorge's side a creature survived damage at least
// its toughness under the static.
func TestLethalByPowerLowers(t *testing.T) {
	if isolate(t) {
		return
	}
	_, a := sp3Run(t, "lethalbypower", sp3LethalDecks, pkPolicy, 24, false, nil, sp3LethalLive)
	_, b := sp3Run(t, "lethalbypower", sp3LethalDecks, ExplorePolicy, 24, false, nil, sp3LethalLive)
	t.Logf("a creature survived toughness-lethal damage in %d random and %d explore games", a, b)
	if a+b < 3 {
		t.Errorf("the power threshold was live in %d games, want at least 3", a+b)
	}
}

// TestLethalByPowerMutation: with the static ignored
// (dirac.DebugLethalByPowerOff) the kernel reads toughness and diverges, so
// TestLethalByPowerLowers does not pass vacuously.
func TestLethalByPowerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugLethalByPowerOff = true
	defer func() { dirac.DebugLethalByPowerOff = false }()
	if d, _ := sp3Run(t, "lethalbypower", sp3LethalDecks, ExplorePolicy, 24, true, nil, sp3LethalLive); d == nil {
		t.Error("no divergence with the power threshold off")
	}
}
