package oracle

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Two restricted mana abilities whose spend restriction names activated
// abilities (CR 106.6, 601.2g-h, 602.2b):
//
//   - Sunken Citadel: {T}: Add one mana of the chosen colour; {T}: Add two of
//     that colour, spendable only to activate abilities of land sources.
//     Safe Haven's {2}, {T}: Exile a creature you control is a land source's
//     activation and only Citadel can pay its {2}, so the pair's larger
//     alternative must be planned.
//   - Steelswarm Operator: {T}: Add {U} for artifact spells; {T}: Add {U}{U}
//     for abilities of artifact sources. Wayfarer's Bauble is the artifact
//     whose {2}, {T}, sacrifice ability the second pays; the Bauble itself is
//     the artifact spell the first pays.
//
// Both decks hold only the sources the case names, so a game that reaches the
// activation proves the new planner path: the pre-change kernel refused both
// cards in kgen. The games are recorded by TestActivatedOnlyPairRecord
// (ACTONLY_PAIR_RECORD=1) and replayed by TestStdSetReprosLockstep.
type activatedOnlyCase struct {
	effectLongCase
	activate string // the card whose activation the policy takes
	color    int    // the choose-a-colour answer, -1 none
}

func activatedOnlyCases() []activatedOnlyCase {
	return []activatedOnlyCase{
		{
			effectLongCase: effectLongCase{
				name:    "sunken_citadel_activation_only_pair",
				decks:   [2][]string{append(append(deckOf(1, "Sunken Citadel"), deckOf(1, "Kher Keep")...), deckOf(18, "Plains")...), deckOf(20, "Plains")},
				surface: "autopay",
				reached: citadelSecondAbilityUsed,
			},
			activate: "Kher Keep",
			color:    3, // red: Kher Keep's {1}{R} activation
		},
		{
			effectLongCase: effectLongCase{
				name:    "steelswarm_operator_two_restricted_abilities",
				prefer:  []string{"Steelswarm Operator", "Wayfarer's Bauble"},
				decks:   [2][]string{append(append(deckOf(8, "Island"), deckOf(8, "Steelswarm Operator")...), deckOf(4, "Wayfarer's Bauble")...), deckOf(20, "Plains")},
				surface: "autopay",
				reached: operatorSecondAbilityUsed,
			},
			activate: "Wayfarer's Bauble",
			color:    -1,
		},
	}
}

// citadelSecondAbilityUsed: Kher Keep's {1}{R} token activation was taken (its
// only nonmana activation), and Citadel was tapped beside it. Kher Keep needs
// two mana and Citadel's first ability makes one, so the tap that paid it was
// the activation-only second.
func citadelSecondAbilityUsed(g *GorgeSide) bool {
	in := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	citadel, haven := false, false
	for i := range g.E.L.Events {
		if e := &g.E.L.Events[i]; e.Kind == events.Tap {
			switch in(e) {
			case "Sunken Citadel":
				citadel = true
			case "Kher Keep":
				haven = true
			}
		}
	}
	return citadel && haven
}

// operatorSecondAbilityUsed: Wayfarer's Bauble was cast (the spell-only {U}
// pays its {1} beside an Island at most) and then sacrificed to its {2}
// activation, which only the activation-only {U}{U} can fund; the Operator
// was tapped.
func operatorSecondAbilityUsed(g *GorgeSide) bool {
	in := func(e *events.Event) string {
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
			return o.Face().Name
		}
		return ""
	}
	cast, sac, tapped := false, false, false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.MoveZone && in(e) == "Wayfarer's Bauble" && e.To == state.ZBattlefield:
			cast = true
		case e.Kind == events.MoveZone && in(e) == "Wayfarer's Bauble" && e.From == state.ZBattlefield && events.IsSacrifice(*e):
			sac = true
		case e.Kind == events.Tap && in(e) == "Steelswarm Operator":
			tapped = true
		}
	}
	return cast && sac && tapped
}

// activatedOnlyPolicy plays lands, casts the case's cards in order, then takes
// the activation; the random tail answers its target and everything else.
func activatedOnlyPolicy(m *Matchup, c activatedOnlyCase, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind == DChoose && len(p.D.Options) > 0 {
			if c.color >= 0 && c.color < len(p.D.Options) {
				return p.D.Options[c.color] // every Citadel chooses the same colour
			}
			return p.D.Options[0]
		}
		if p.D.Kind == DPriority {
			for _, o := range p.D.Options {
				if o.Kind() == ActPlayLand {
					return o
				}
			}
			for _, want := range c.prefer {
				for _, o := range p.D.Options {
					if o.Kind() == ActCast && m.InstanceCard(int(o.Src())) == want {
						return o
					}
				}
			}
			for _, o := range p.D.Options {
				if o.Kind() == ActActivate && m.InstanceCard(int(o.Src())) == c.activate {
					return o
				}
			}
		}
		return rnd.Pick(p)
	})
}

// TestActivatedOnlyPairReprosReach replays the recorded games: no divergence,
// every scripted action offered, and the activation reached.
func TestActivatedOnlyPairReprosReach(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	for _, c := range activatedOnlyCases() {
		t.Run(c.name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", c.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			script := &ScriptPolicy{Actions: r.Actions}
			res, reached := effectLongPlay(t, reg, c.effectLongCase, r, printedDelveSet(t, r.Cards), script)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if script.Skipped != 0 {
				t.Errorf("%d scripted actions were not offered", script.Skipped)
			}
			if !reached {
				t.Fatalf("the recorded game never reaches %s", c.activate)
			}
		})
	}
}

// TestActivatedOnlyPairRecord plays the case's policy over seeds until one
// reaches the activation, and writes the repro. It runs only with
// ACTONLY_PAIR_RECORD=1; the games it writes are the committed repros.
func TestActivatedOnlyPairRecord(t *testing.T) {
	if os.Getenv("ACTONLY_PAIR_RECORD") == "" {
		return // a recording tool, not coverage
	}
	reg := synthReg(t)
	for _, c := range activatedOnlyCases() {
		t.Run(c.name, func(t *testing.T) {
			for seed := uint64(1); seed <= 400; seed++ {
				r := &Repro{Format: reproFormat, Decks: [2]string{c.name + "_0", c.name + "_1"}, Cards: c.decks,
					Seed: seed, PassElision: true, Surface: c.surface, Kernel: "custom"}
				set := reproSet(t, "custom", r)
				m, err := r.Matchup(reg)
				if err != nil {
					t.Fatal(err)
				}
				res, reached := effectLongPlay(t, reg, c.effectLongCase, r, set, activatedOnlyPolicy(m, c, seed))
				if res.Div != nil {
					t.Fatalf("seed %d: divergence at %d: %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
				}
				if !reached {
					continue
				}
				r.setActions(res.Actions)
				if err := r.Write(filepath.Join("testdata", "repro", c.name+".repro.json")); err != nil {
					t.Fatal(err)
				}
				t.Logf("seed %d: %d decisions", seed, len(res.Actions))
				return
			}
			t.Fatalf("no seed reached %s", c.activate)
		})
	}
}
