package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The spend-restricted tap-another mana lockstep (CR 106.6, 601.2g): Grand
// Architect's "Tap an untapped blue creature you control: Add {C}{C}. Spend
// this mana only to cast artifact spells or activate abilities of artifacts",
// on its own (no other restricted source). The blue deck carries blue creatures to
// tap (and Grand Architect itself, a candidate of its own cost), artifact
// spells and an artifact's mana-costed abilities (Millstone, Bonesplitter's
// equip) the {C}{C} pays, and non-artifact spells it must not pay. Both halves
// of the restriction are read: the spell's (ManaRestrict) and the activated
// ability's (ManaRestrictAb).
func restrictedTapXDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Grand Architect", Count: 4}, {Name: "Merfolk Looter", Count: 4}, {Name: "Wind Drake", Count: 4},
			{Name: "Cloud Elemental", Count: 3}, {Name: "Juggernaut", Count: 3}, {Name: "Millstone", Count: 3},
			{Name: "Wurmcoil Engine", Count: 2}, {Name: "Bonesplitter", Count: 3}, {Name: "Island", Count: 14},
		},
		{
			{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 12},
			{Name: "Kamahl, Pit Fighter", Count: 4}, {Name: "Mountain", Count: 16},
		},
	}
}

// restrictedTapXCounts counts the Grand Architect mana a spell or an ability
// the same decision pushed spent.
type restrictedTapXCounts struct{ spell, ability int }

// restrictedTapXScan walks gorge's log: a restricted positive ManaAdd names its
// source (the " @id" suffix) and the next push of the decision spends it.
func restrictedTapXScan(g *GorgeSide, n *restrictedTapXCounts) {
	if g == nil {
		return
	}
	var pending []state.ObjID
	credit := func(spell bool) {
		for _, src := range pending {
			if ob := g.E.G.Obj(src); ob != nil && ob.Face() != nil && ob.Face().Name == "Grand Architect" {
				if spell {
					n.spell++
				} else {
					n.ability++
				}
			}
		}
		pending = pending[:0]
	}
	for i := range g.E.L.Events {
		ev := &g.E.L.Events[i]
		switch ev.Kind {
		case events.ManaAdd:
			if ev.Amount <= 0 {
				continue
			}
			if _, src, _, _, ok := events.ManaRestrictionFromText(restrSourceCutPersistent(ev.Text)); ok && src != 0 {
				pending = append(pending, src)
			}
		case events.ManaClear, events.TurnChange:
			pending = pending[:0]
		case events.PutOnStack:
			credit(true)
		case events.AbilityPush:
			credit(false)
		}
	}
}

type restrictedTapXPod struct {
	reg   *cards.Registry
	set   cardset.Set
	decks [2][]cardset.Entry
}

func newRestrictedTapXPod(t *testing.T) restrictedTapXPod {
	t.Helper()
	reg := synthReg(t) // the corpus skip comes before the fixture table read
	set, err := cardset.RawByName("restrictedtapx")
	if err != nil {
		t.Fatal(err)
	}
	p := restrictedTapXPod{reg: reg, set: set, decks: restrictedTapXDecks()}
	for i, d := range p.decks {
		c := 0
		for _, e := range d {
			c += e.Count
		}
		if c != 40 {
			t.Fatalf("deck %d: %d cards", i, c)
		}
	}
	return p
}

func (p restrictedTapXPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, restrictedTapXCounts) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"}, Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])},
		Seed: seed, PassElision: true, Surface: "full"}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	var c restrictedTapXCounts
	rc.Finish = func(g *GorgeSide) { restrictedTapXScan(g, &c) }
	tbl, _, _, err := p.set.Matchup(p.decks[0], p.decks[1])
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(p.set, tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	return LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc), c
}

func restrictedTapXFail(t *testing.T, seed uint64, res Result) {
	t.Helper()
	lo := max(res.Div.At-12, 0)
	t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
		res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
}

const restrictedTapXSeeds = 150

// TestRestrictedTapXLockstep plays seeds of the pod: no divergence and no stall,
// and Grand Architect's mana was spent on an artifact spell and on an artifact's
// activated ability.
func TestRestrictedTapXLockstep(t *testing.T) {
	t.Parallel()
	p := newRestrictedTapXPod(t)
	var sum restrictedTapXCounts
	for seed := uint64(1); seed <= restrictedTapXSeeds; seed++ {
		res, c := p.play(t, seed, 40)
		if res.Div != nil {
			restrictedTapXFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.spell += c.spell
		sum.ability += c.ability
	}
	t.Logf("Grand Architect mana spent: artifact spells %d, artifact abilities %d", sum.spell, sum.ability)
	if sum.spell == 0 || sum.ability == 0 {
		t.Fatalf("Grand Architect's mana spent on %d artifact spells and %d artifact abilities: want both above zero", sum.spell, sum.ability)
	}
}

// restrictedTapXMutates plays the pod's seeds under a mutation and reports
// whether one diverged.
func restrictedTapXMutates(t *testing.T) bool {
	p := newRestrictedTapXPod(t)
	for seed := uint64(1); seed <= restrictedTapXSeeds; seed++ {
		if res, _ := p.play(t, seed, 40); res.Div != nil {
			t.Logf("mutation diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return true
		}
	}
	return false
}

// TestRestrictedTapXMutationHidden: without the restricted tap-another source
// (DebugNoRestrTapX) the pod diverges: its plays need the mana.
func TestRestrictedTapXMutationHidden(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugNoRestrTapX = true
	defer func() { dirac.DebugNoRestrTapX = false }()
	if !restrictedTapXMutates(t) {
		t.Fatal("no seed diverged without the restricted tap-another source: the pool never needs the mana")
	}
}

// TestRestrictedTapXMutations: floating Grand Architect's mana unrestricted (the
// restriction half), and the census refusing every activated-ability payment
// (the ManaRestrictAb half) or every spell payment (the ManaRestrict half),
// each diverge from gorge: the pod pays artifact spells and an artifact's
// abilities with the mana and offers no non-artifact spell through it.
func TestRestrictedTapXMutations(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		name string
		v    uint8
	}{{"float unrestricted", 1}, {"activated payments refused", 2}, {"spell payments refused", 3}} {
		dirac.DebugRestrTapXMutate = m.v
		diverged := restrictedTapXMutates(t)
		dirac.DebugRestrTapXMutate = 0
		if !diverged {
			t.Errorf("mutation %q (%d): no seed diverged", m.name, m.v)
		}
	}
}
