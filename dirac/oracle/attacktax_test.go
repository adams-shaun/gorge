package oracle

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The attack-tax lockstep (S:Mode$ CantAttackUnless with a generic mana
// Cost$; CR 508.1d, 508.1g-i; gorge rules/attack_cost.go, kernel
// combat_attacktax.go): Propaganda, Ghostly Prison and Windborn Muse tax an
// attack on their controller, Baird and Sphere of Safety (its tax the
// number of enchantments its controller controls) one on that player or a
// planeswalker they control (so do Archon of Absolution and Collective
// Restraint, its tax the domain count), Onakke Oathkeeper one on such a
// planeswalker (Ajani, Caller of the Pride), Brainwash every attack of the
// creature it enchants and Phyrexian Marauder its own, {1} per +1/+1
// counter on it. The payers' sources cover the attack
// window's shapes: basic lands, a dual's two intrinsic abilities, a painland and a Talisman (a fixed {C} first, the painful choice second), Sol Ring's
// two mana, the choice-shaped Tranquil Cove and Birds of Paradise, the
// chosen-colour Thriving Isle, Arcane Signet's colour identity outside a
// commander game, and Llanowar Elves, a mana creature that may attack (its
// own mana then folds into the budget and CR 508.1f keeps it from paying),
// and Warden of Geometries, whose vigilance leaves it free to pay.
// The taxer deck plays most taxes, the payer deck most sources; the seats
// swap every other pair of seeds.

var katTaxer = []cardset.Entry{
	{Name: "Propaganda", Count: 2}, {Name: "Ghostly Prison", Count: 2}, {Name: "Windborn Muse", Count: 2},
	{Name: "Baird, Steward of Argive", Count: 2}, {Name: "Sphere of Safety", Count: 2}, {Name: "Onakke Oathkeeper", Count: 2},
	{Name: "Archon of Absolution", Count: 1}, {Name: "Brainwash", Count: 2}, {Name: "Collective Restraint", Count: 1},
	{Name: "Ajani, Caller of the Pride", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Serra Angel", Count: 1},
	{Name: "Mind Stone", Count: 1}, {Name: "Plains", Count: 7}, {Name: "Island", Count: 4}, {Name: "Tundra", Count: 2},
	{Name: "Tranquil Cove", Count: 2}, {Name: "Thriving Isle", Count: 1}, {Name: "Adarkar Wastes", Count: 2},
}

var katPayer = []cardset.Entry{
	{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Birds of Paradise", Count: 3},
	{Name: "Warden of Geometries", Count: 2},
	{Name: "Serra Angel", Count: 1}, {Name: "Phyrexian Marauder", Count: 2}, {Name: "Koskun Falls", Count: 1},
	{Name: "Propaganda", Count: 1}, {Name: "Ghostly Prison", Count: 1}, {Name: "Onakke Oathkeeper", Count: 1},
	{Name: "Ajani, Caller of the Pride", Count: 1}, {Name: "Sol Ring", Count: 2}, {Name: "Arcane Signet", Count: 1},
	{Name: "Mind Stone", Count: 1}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Swamp", Count: 1},
	{Name: "Tundra", Count: 1}, {Name: "Tranquil Cove", Count: 1}, {Name: "Thriving Isle", Count: 2}, {Name: "Talisman of Progress", Count: 2},
}

// katTaxes are the newly lowered carriers each game counts the casts of:
// the class the Commander precons hold and the corpus's other generic mana
// taxes the lowering admits (Archon of Absolution, Brainwash's enchanted
// creature, Collective Restraint's domain, Koskun Falls, Phyrexian
// Marauder's own counters).
var katTaxes = []string{"Propaganda", "Ghostly Prison", "Windborn Muse", "Baird, Steward of Argive", "Sphere of Safety",
	"Onakke Oathkeeper", "Archon of Absolution", "Brainwash", "Collective Restraint", "Koskun Falls", "Phyrexian Marauder"}

// katScan counts each tax card's casts in gorge's log.
func katScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		if ev.Kind != events.PutOnStack || ev.From != state.ZHand {
			continue
		}
		if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil {
			for _, name := range katTaxes {
				if ob.Face().Name == name {
					n["cast "+name]++
				}
			}
		}
	}
}

// katMutation names a dirac mutation switch the run turns on.
type katMutation int

const (
	katNone  katMutation = iota
	katFree              // dirac.DebugAtkTaxFree: no attack is taxed
	katOrder             // dirac.DebugAtkTaxChoiceFirst: the window taps the choice-shaped sources first
	katVigil             // dirac.DebugAtkTaxWithholdVigilant: the window withholds a declared attacker with vigilance too
)

// runAttackTax plays the seeded games; a mutation stops at the first
// divergence. The tally counts the taxed picks (gorge's option label names
// the {N} per creature), those at a planeswalker, and each source the
// payment window tapped.
func runAttackTax(t *testing.T, seeds uint64, mut katMutation) (games, divs int, tally map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("attacktax")
	if err != nil {
		t.Fatal(err)
	}
	switch mut {
	case katFree:
		old := dirac.DebugAtkTaxFree
		dirac.DebugAtkTaxFree = true
		defer func() { dirac.DebugAtkTaxFree = old }()
	case katOrder:
		old := dirac.DebugAtkTaxChoiceFirst
		dirac.DebugAtkTaxChoiceFirst = true
		defer func() { dirac.DebugAtkTaxChoiceFirst = old }()
	case katVigil:
		old := dirac.DebugAtkTaxWithholdVigilant
		dirac.DebugAtkTaxWithholdVigilant = true
		defer func() { dirac.DebugAtkTaxWithholdVigilant = old }()
	}
	tally = map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		ents := [2][]cardset.Entry{katTaxer, katPayer}
		if (seed/2)%2 == 1 {
			ents[0], ents[1] = ents[1], ents[0]
		}
		var decks [2][]*cards.Card
		for s := range decks {
			for _, e := range ents[s] {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], c)
				}
			}
			if len(decks[s]) != 40 {
				t.Fatalf("deck %d: %d cards", s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(ents[0], ents[1])
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kat-a", "kat-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		declared := map[state.ObjID]bool{} // the attackers picked in the declaration being paid
		played := map[string]int{}
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && gs == nil {
				gs = c.G
				gs.OnInternal = func(d *decision.Decision, idx int) {
					if len(d.Options) > 0 && d.Options[0].Kind == "attack_mana" {
						tally["window taps"]++
						if o := optByIndex(d, idx); o.Obj != 0 {
							if ob := gs.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
								tally["tap "+ob.Face().Name]++
							}
							if declared[o.Obj] {
								tally["tap a declared attacker"]++
							}
						}
					}
				}
			}
			a := inner.Pick(c)
			if c.D.Kind != DAttackers {
				clear(declared)
			}
			if a.Kind() == ActAttack && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok {
					declared[o.Obj] = true
				}
				if o, ok := c.G.GorgeOption(a); ok && strings.Contains(o.Label, " per creature)") {
					tally["taxed picks"]++
					if o.Battle != 0 {
						tally["taxed planeswalker picks"]++
					}
					katLive(c.G.E.G, o, tally)
				}
			}
			return a
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		katScan(gs, tally)
		if d := res.Div; d != nil {
			divs++
			if mut != katNone {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// katPlayerTaxes tax an attack on their controller only; katWalkerTaxes
// one on that player's planeswalkers (Baird, Sphere of Safety and Archon of
// Absolution tax both).
var (
	katPlayerTaxes = []string{"Propaganda", "Ghostly Prison", "Windborn Muse", "Collective Restraint", "Koskun Falls", "Baird, Steward of Argive", "Sphere of Safety", "Archon of Absolution"}
	katWalkerTaxes = []string{"Onakke Oathkeeper", "Baird, Steward of Argive", "Sphere of Safety", "Archon of Absolution"}
)

// katLive tallies, for one charged pick o, the newly lowered taxes in force
// on it: each carrier of the right scope on the defender's battlefield, a
// Brainwash enchanting the attacker, an attacking Phyrexian Marauder.
func katLive(g *state.Game, o decision.Option, tally map[string]int) {
	scope := katPlayerTaxes
	if o.Battle != 0 {
		scope = katWalkerTaxes
	}
	seen := map[string]bool{}
	for _, id := range g.Zone(state.ZBattlefield, o.Player) {
		if ob := g.Obj(id); ob != nil && ob.Face() != nil && slices.Contains(scope, ob.Face().Name) && !seen[ob.Face().Name] {
			seen[ob.Face().Name] = true
			tally["live "+ob.Face().Name]++
		}
	}
	if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Phyrexian Marauder" {
		tally["live Phyrexian Marauder"]++
	}
	for p := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(p)) {
			if ob := g.Obj(id); ob != nil && ob.Face() != nil && ob.Face().Name == "Brainwash" && ob.AttachedTo == o.Obj {
				tally["live Brainwash"]++
			}
		}
	}
}

func katTally(tally map[string]int) string {
	keys := make([]string, 0, len(tally))
	for k := range tally {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, tally[k]))
	}
	return strings.Join(parts, " ")
}

// TestAttackTaxLockstep plays every newly lowered attack tax against a payer
// whose sources span the attack window's shapes, gorge and dirac decision by
// decision.
func TestAttackTaxLockstep(t *testing.T) {
	t.Parallel()
	games, divs, tally := runAttackTax(t, 160, katNone)
	t.Logf("%d games, %d divergences; %s", games, divs, katTally(tally))
	for _, name := range katTaxes {
		if tally["cast "+name] < 3 {
			t.Errorf("%s cast %d times, want at least 3", name, tally["cast "+name])
		}
		if tally["live "+name] < 2 {
			t.Errorf("%s taxed %d charged picks, want at least 2", name, tally["live "+name])
		}
	}
	for key, min := range map[string]int{"taxed picks": 40, "taxed planeswalker picks": 2, "window taps": 40,
		"tap Sol Ring": 2, "tap Tundra": 2, "tap Adarkar Wastes": 2, "tap Talisman of Progress": 2, "tap Llanowar Elves": 2, "tap a declared attacker": 1} {
		if tally[key] < min {
			t.Errorf("%s %d, want at least %d", key, tally[key], min)
		}
	}
	if choice := tally["tap Birds of Paradise"] + tally["tap Tranquil Cove"] + tally["tap Thriving Isle"]; choice < 2 {
		t.Errorf("the window tapped a choice-shaped source %d times, want at least 2", choice)
	}
}

// TestAttackTaxFreeMutation: a kernel that taxes no attack diverges.
func TestAttackTaxFreeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runAttackTax(t, 160, katFree)
	if divs == 0 {
		t.Fatalf("an untaxed kernel played %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}

// TestAttackTaxOrderMutation: a window that taps the choice-shaped sources
// before the fixed-production ones diverges.
func TestAttackTaxOrderMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runAttackTax(t, 160, katOrder)
	if divs == 0 {
		t.Fatalf("a choice-first window played %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}

// TestAttackTaxVigilantMutation: a window that withholds a declared
// attacker with vigilance (which CR 508.1f leaves untapped) diverges.
func TestAttackTaxVigilantMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runAttackTax(t, 160, katVigil)
	if divs == 0 {
		t.Fatalf("a window withholding vigilant attackers played %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
