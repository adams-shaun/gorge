package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 605.2a, 601.2g: a land with two mana abilities offers both, and the
// payment window taps the one the payment needs. Sunken Citadel's second
// "{T}: Add two mana of the chosen color. Spend this mana only to activate
// abilities of land sources." is restricted to exactly that: paying
// Blighted Gorge's {4}{R} activation. gorge taps the Citadel's restricted
// ability for it; the kernel's window walk (mana_window.go windowCanTap)
// read only the source's first usable ability, tapped the chosen-1 one and
// came up short, so the game diverged at the window's projection
// (Pool ref=5 kernel=4, Cards[].Flags ref=1 kernel=0).

// laterChosenWindowDecks deals the scenario: each seat 8 Blighted Gorge and
// 32 Sunken Citadel, so every mana source is a Citadel (the Blighted Gorge
// is only the activation being paid).
func laterChosenWindowDecks() [2][]string {
	deck := append(repeatName("Blighted Gorge", 8), repeatName("Sunken Citadel", 32)...)
	return [2][]string{append([]string(nil), deck...), append([]string(nil), deck...)}
}

// laterChosenWindowPolicy plays the scenario: play a hand land each turn,
// choose red for every Sunken Citadel's ETB colour ask, and take the
// Blighted Gorge {4}{R} activation as soon as either side offers it.
func laterChosenWindowPolicy(m *Matchup) PolicyFunc {
	return func(c *PickCtx) Canon {
		if c.G == nil {
			return pickKind(c, ActPass)
		}
		g := c.G.E.G
		switch c.D.Kind {
		case DChoose:
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && o.Kind == "color" && o.Label == "Red" {
					return a
				}
			}
			return pickKind(c, ActDone)
		case DPriority:
			for _, a := range c.D.Options {
				if o, ok := c.G.GorgeOption(a); ok && a.Kind() == ActPlayLand {
					if ob := g.Obj(o.Obj); ob != nil && ob.Zone == state.ZHand {
						return a
					}
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActActivate && a.Mode() == 1 && m.InstanceCard(int(a.Src())) == "Blighted Gorge" {
					return a
				}
			}
		}
		if a, ok := findAction(c, func(a Canon) bool { return a.Kind() == ActPass }); ok {
			return a
		}
		return pickKind(c, ActPass)
	}
}

// laterChosenWindowRun plays one game of the scenario.
func laterChosenWindowRun(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro, policy Policy) Result {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(oneOfEachEntries(r.Cards[0]), oneOfEachEntries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = 800
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, policy, rc)
}

// TestMBCRestrictedLaterChosenWindowRecord records the scenario: it runs
// only with PINS_RECORD=1; the game it writes is the committed repro, whose
// first divergence is the payment window's projection after the activation.
func TestMBCRestrictedLaterChosenWindowRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	r := &Repro{Format: reproFormat, Decks: [2]string{"later_chosen_window_0", "later_chosen_window_1"},
		Cards: laterChosenWindowDecks(), Seed: 1, PassElision: true, Surface: "full",
		Kernel: "custom", Pool: "custom", MaxTurns: 10,
		Why: "a restricted later chosen-mana ability pays a land activation's window"}
	set := printedDelveSet(t, r.Cards)
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	res := laterChosenWindowRun(t, reg, set, r, laterChosenWindowPolicy(m))
	if res.Div == nil {
		t.Fatalf("the game never diverged at the window: %d actions, stall %q", len(res.Actions), res.Stall)
	}
	if res.Div.What != "projection" {
		t.Fatalf("first divergence is %s/%s, want the window's projection: %s %v", res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
	}
	r.setActions(res.Actions)
	if err := r.Write(filepath.Join("testdata", "repro", "restricted_later_chosen_ability_pays_the_activation.repro.json")); err != nil {
		t.Fatal(err)
	}
	t.Logf("recorded %d actions: divergence at %d %s %s %v", len(r.Actions), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
}
