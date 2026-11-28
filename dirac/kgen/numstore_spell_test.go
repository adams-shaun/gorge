package kgen

import (
	"strings"
	"testing"
)

// offerProbe is a tempting-offer shaped sorcery of its own: each opponent is
// offered a draw (RepeatOptionalForEachPlayer$), the accepted offers are
// counted in X, and the controller draws X; the chain resets X last.
const offerProbe = `Name:Offer Probe
ManaCost:2 W
Types:Sorcery
A:SP$ Draw | SubAbility$ DBRepeat | SpellDescription$ probe
SVar:DBRepeat:DB$ RepeatEach | RepeatSubAbility$ DBDrawOpp | RepeatPlayers$ Player.Opponent | RepeatOptionalForEachPlayer$ True | RepeatOptionalMessage$ Take it? | SubAbility$ DBDrawYou
SVar:DBDrawOpp:DB$ Draw | Defined$ Player.IsRemembered | SubAbility$ DBTally
SVar:DBTally:DB$ StoreSVar | Type$ CountSVar | SVar$ X | Expression$ X/Plus.1
SVar:DBDrawYou:DB$ Draw | NumCards$ X | SubAbility$ DBClear
SVar:DBClear:DB$ StoreSVar | Type$ Number | SVar$ X | Expression$ 0
SVar:X:Number$0
`

// TestOfferLowers: the probe lowers whole: the opt-in election heads the
// repeated player's body and skips exactly the body's other instructions, the
// accumulator is an increment of the stored name, and the chain's last
// instruction resets it to its printed default.
func TestOfferLowers(t *testing.T) {
	c := lowerScriptCard(t, "Offer Probe", offerProbe)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	if len(c.Abilities) != 1 {
		t.Fatalf("%d abilities, want 1", len(c.Abilities))
	}
	prog := c.Abilities[0].Prog
	rep, opt := -1, -1
	for i, in := range prog {
		switch in.Op {
		case "RepeatEach":
			rep = i
		case "OptInSeat":
			opt = i
		}
	}
	if rep < 0 || opt != rep+1 {
		t.Fatalf("RepeatEach at %d, OptInSeat at %d: the election must head the loop's body", rep, opt)
	}
	if prog[rep].Skip != prog[opt].Skip+1 {
		t.Errorf("loop skips %d, the election %d: a no must skip the body's other instructions", prog[rep].Skip, prog[opt].Skip)
	}
	if end := opt + prog[opt].Skip; prog[end].Op != "ExtStoreNum" || prog[end].B.Kind != vSVarNum && prog[end].B.Add != 1 {
		t.Errorf("the body ends with %v, want the accumulator's increment", prog[end])
	}
	last := prog[len(prog)-1]
	if last.Op != "ExtStoreNum" || !last.B.isConst() || last.B.N != 0 || last.C != 0<<1|1 {
		t.Errorf("chain ends with %v, want the reset to the printed 0", last)
	}
}

// TestOfferLowersWithoutReset: a spell's stored number does not need an
// in-chain reset. The object becomes a new one when the spell leaves the stack
// for the graveyard (CR 400.7), and both engines clear the runtime SVar store
// there (engine/events/apply_zone.go, dirac/act_storenum.go numStackExit).
func TestOfferLowersWithoutReset(t *testing.T) {
	src := strings.Replace(offerProbe, "SVar:DBDrawYou:DB$ Draw | NumCards$ X | SubAbility$ DBClear\nSVar:DBClear:DB$ StoreSVar | Type$ Number | SVar$ X | Expression$ 0\n",
		"SVar:DBDrawYou:DB$ Draw | NumCards$ X\n", 1)
	if src == offerProbe {
		t.Fatal("the edit changed nothing")
	}
	c := lowerScriptCard(t, "Offer Probe", src)
	for _, rf := range c.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
}

// TestOfferRefusals: the offer only lowers over opponents, and a condition
// that names the stored SVar stays unread.
func TestOfferRefusals(t *testing.T) {
	for _, tc := range []struct{ name, from, to, want string }{
		{"conditional reset", "SVar:DBClear:DB$ StoreSVar | Type$ Number", "SVar:DBClear:DB$ StoreSVar | ConditionCheckSVar$ X | ConditionSVarCompare$ GE1 | Type$ Number", "CheckSVar"},
		{"every player", "RepeatPlayers$ Player.Opponent", "RepeatPlayers$ Player", "RepeatOptionalForEachPlayer$"},
	} {
		src := strings.Replace(offerProbe, tc.from, tc.to, 1)
		if src == offerProbe {
			t.Fatalf("%s: the edit changed nothing", tc.name)
		}
		c := lowerScriptCard(t, "Offer Probe", src)
		var all []string
		for _, rf := range c.Refusals {
			all = append(all, rf.Reason)
		}
		if !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, all, tc.want)
		}
	}
}
