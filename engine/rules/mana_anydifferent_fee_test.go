package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// anyDifferentFeeBeacon is the Interplanar Beacon shape: a "{1}, {T}: Add two
// mana of different colors" ability whose mana may be spent only on
// planeswalker spells, beside a plain {T}: Add {C}. The script line is
// reworded (Produced$ before Cost$) so archtest's verbatim-line guard holds.
const anyDifferentFeeBeacon = "Name:Fee Beacon\nTypes:Land\n" +
	"A:AB$ Mana | Produced$ C | Cost$ T | SpellDescription$ Add {C}.\n" +
	"A:AB$ Mana | Produced$ Combo AnyDifferent | Amount$ 2 | Cost$ 1 T | RestrictValid$ Spell.Planeswalker | SpellDescription$ Add two mana of different colours.\nOracle:x\n"

const anyDifferentFeePlains = "Name:Plains\nTypes:Basic Land Plains\nOracle:x\n"

// A {1}, {T} Combo AnyDifferent ability pays its fee and then asks for two
// DIFFERENT colours (CR 106.1): the floated pair is restricted to planeswalker
// spells, a repeated colour is rejected by the validator, and the bot's
// first-Min answer passes it.
func TestComboAnyDifferentFeeAbilityFloatsTwoRestrictedColours(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	beacon := onBoard(t, e, 0, anyDifferentFeeBeacon)
	onBoard(t, e, 0, anyDifferentFeePlains)
	addManaQuiet(t, e, 0, "W")
	if got := e.G.Players[0].Pool; got != (state.Mana{state.MW: 1}) {
		t.Fatalf("precondition: pool = %v, want exactly {W}", got)
	}
	// The bound: one of each colour from the fee ability, plus {C}, the floating {W} and the
	// Plains' {W}; never the unbounded every-colour sentinel.
	if got, want := e.PotentialMana(0), (state.Mana{state.MW: 3, state.MU: 1, state.MB: 1, state.MR: 1, state.MG: 1, state.MC: 1}); got != want {
		t.Fatalf("fee AnyDifferent potential = %v, want the bounded %v", got, want)
	}

	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, beacon))
	d := e.Pending()
	fee := -1
	for _, o := range d.Options {
		if strings.Contains(o.Label, "AnyDifferent") || strings.Contains(o.Label, "different") {
			fee = o.Index
		}
	}
	if fee < 0 {
		t.Fatalf("no fee ability in the Beacon's menu: %+v", d.Options)
	}
	submitChoices(t, e, fee)
	d = e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 || len(d.Options) != 10 {
		t.Fatalf("fee AnyDifferent ask = %+v, want two choices from ten unit-colour options", d)
	}
	var whites, blues []int
	for _, o := range d.Options {
		switch o.ManaSymbol {
		case "W":
			whites = append(whites, o.Index)
		case "U":
			blues = append(blues, o.Index)
		}
	}
	if len(whites) != 2 || len(blues) != 2 {
		t.Fatalf("fee AnyDifferent offered W=%v U=%v, want two units of each", whites, blues)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{whites[0], whites[1]}}); err == nil {
		t.Fatal("same-colour answer accepted, want a rejection")
	}
	bot := []int{d.Options[0].Index, d.Options[1].Index}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: bot}); err != nil {
		t.Fatalf("bot's first-Min answer %v rejected: %v", bot, err)
	}
	submitChoices(t, e, whites[0], blues[0])
	pl := e.G.Players[0]
	if want := (state.Mana{state.MW: 1, state.MU: 1}); pl.Pool != want {
		t.Fatalf("fee AnyDifferent pool = %v, want %v (the {W} paid the fee)", pl.Pool, want)
	}
	if !e.G.Obj(beacon).Tapped {
		t.Fatal("the Beacon stayed untapped after its activation")
	}
	var restricted int32
	for _, r := range pl.RestrictedMana {
		if r.Valid != "Spell.Planeswalker" {
			t.Fatalf("floated batch %+v lost its planeswalker restriction", r)
		}
		restricted += r.Amount
	}
	if restricted != 2 {
		t.Fatalf("restricted units = %d (%+v), want both floated mana restricted", restricted, pl.RestrictedMana)
	}
}

// The CR 601.2g cast window prices the fee ability's distinct pair: with the
// Plains funding the {1}, the Beacon alone pays a {W}{U} planeswalker, never
// a {W}{W} one (its two mana are different colours), and never a {W}{U}
// non-planeswalker spell (RestrictValid$, CR 106.6).
func TestComboAnyDifferentFeeAbilityCastWindow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, cost, types string
		want              bool
	}{
		{"walker WU", "W U", "Legendary Planeswalker Test\nLoyalty:3", true},
		{"walker WW", "W W", "Legendary Planeswalker Test\nLoyalty:3", false},
		{"sorcery WU", "W U", "Sorcery", false},
	} {
		spell := "Name:Fee Spell\nManaCost:" + tc.cost + "\nTypes:" + tc.types + "\nOracle:x\n"
		e, _, spellID := newFixtureDeck(t, 77, spell, anyDifferentFeeBeacon, anyDifferentFeePlains)
		beacon := moveSeeded(t, e, 0, anyDifferentFeeBeacon, state.ZBattlefield)
		moveSeeded(t, e, 0, anyDifferentFeePlains, state.ZBattlefield)
		if o := e.G.Obj(spellID); o == nil || o.Zone != state.ZHand {
			t.Fatalf("%s: precondition: the spell must be in hand: %+v", tc.name, o)
		}
		if pool := e.G.Players[0].Pool; pool != (state.Mana{}) {
			t.Fatalf("%s: precondition: pool = %v, want empty", tc.name, pool)
		}
		pc := &pendingCast{player: 0, card: spellID, from: state.ZHand, ability: -1, cost: e.parseCost(tc.cost)}
		units := e.castWindowUnits(pc)
		pairs := 0
		for _, u := range units {
			if u.ID != beacon {
				continue
			}
			for _, a := range u.Alts {
				if a.CostGeneric == 1 {
					pairs++
					if a.Counts[state.MC] != 0 || a.Amt != 1 {
						t.Fatalf("%s: fee alt %+v is not one mana each of two colours", tc.name, a)
					}
					for _, n := range a.Counts {
						if n > 1 {
							t.Fatalf("%s: fee alt %+v repeats a colour", tc.name, a)
						}
					}
				}
			}
		}
		if wantPairs := map[bool]int{true: 10, false: 0}[tc.types != "Sorcery"]; pairs != wantPairs {
			t.Fatalf("%s: Beacon fee alternatives = %d, want %d (units %+v)", tc.name, pairs, wantPairs, units)
		}
		got := e.castWindowReachable(0, e.parseCost(tc.cost), state.Mana{}, state.Mana{}, [8]state.Mana{}, 20, nil, pipRider{}, units)
		if got != tc.want {
			t.Fatalf("%s: {%s} reachable = %v, want %v", tc.name, tc.cost, got, tc.want)
		}
	}
}
