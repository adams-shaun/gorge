package kgen

import (
	"strings"
	"testing"
)

// Processor, Minion of the Wastes and Nameless Race: "as CARDNAME enters, pay
// any amount of life" (R:Event$ Moved with a StoreSVar body costing PayLife<X>).
const payLifeProcessor = `Name:Pay Life Processor
ManaCost:4
Types:Artifact
R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ PayAny | ReplacementResult$ Updated | Description$ Pay any amount of life as this enters.
SVar:PayAny:AB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ X | Cost$ Mandatory PayLife<X>
A:AB$ Token | Cost$ 4 T | TokenScript$ b_x_x_phyrexian_minion | TokenOwner$ You | TokenPower$ LifePaidOnETB | TokenToughness$ LifePaidOnETB | SpellDescription$ Create an X/X token.
SVar:X:Count$xPaid
SVar:LifePaidOnETB:Number$0
`

const payLifeRace = `Name:Pay Life Race
ManaCost:3 B
Types:Creature
PT:*/*
K:Trample
R:Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplaceWith$ PayAny | ReplacementResult$ Updated | Description$ Pay any amount of life as this enters.
SVar:PayAny:AB$ StoreSVar | SVar$ LifePaidOnETB | Type$ Calculate | Expression$ X | Cost$ Mandatory PayLife<X> | XMax$ Cap
S:Mode$ Continuous | CharacteristicDefining$ True | SetPower$ LifePaidOnETB | SetToughness$ LifePaidOnETB | Description$ power and toughness are each equal to the life paid.
SVar:X:Count$xPaid
SVar:Cap:SVar$Held/Plus.Yard
SVar:Held:Count$Valid Permanent.White+!token+OppCtrl
SVar:Yard:Count$ValidGraveyard Card.White+OppCtrl
SVar:LifePaidOnETB:Number$0
`

// TestPayLifeLowers: the as-enters pay-any-amount card lowers with no refusal
// to one ExtPayLifeX at the head of its enters program, and the token and CDA
// readers read the stored name.
func TestPayLifeLowers(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		capped    bool
	}{{"Pay Life Processor", payLifeProcessor, false}, {"Pay Life Race", payLifeRace, true}} {
		c := lowerScriptCard(t, tc.name, tc.src)
		for _, rf := range c.Refusals {
			t.Errorf("%s refused: %s", tc.name, rf.Reason)
		}
		n := 0
		for _, r := range c.Repls {
			for i, in := range r.Prog {
				if in.Op != "ExtPayLifeX" {
					continue
				}
				n++
				if i != 0 || in.CS != "LifePaidOnETB" || r.S != "Updated" {
					t.Errorf("%s: ExtPayLifeX at %d name %q result %q, want the head, LifePaidOnETB, Updated", tc.name, i, in.CS, r.S)
				}
				if capped := !(in.B.isConst() && in.B.N == payLifeNoCap); capped != tc.capped {
					t.Errorf("%s: cap value %v, want capped=%v", tc.name, in.B, tc.capped)
				}
			}
		}
		if n != 1 {
			t.Errorf("%s: %d ExtPayLifeX, want 1", tc.name, n)
		}
	}
}

// TestPayLifeRefusals: shapes the ask does not model refuse by name.
func TestPayLifeRefusals(t *testing.T) {
	for _, tc := range []struct{ name, from, to, want string }{
		{"fixed cost", "PayLife<X>", "PayLife<2>", "PayLife"},
		{"no Count$xPaid", "SVar:X:Count$xPaid", "SVar:X:Number$2", "PayLife"},
		{"extra cost part", "Mandatory PayLife<X>", "Mandatory PayLife<X> Discard<1/Card>", "PayLife"},
		{"Type Number", "Type$ Calculate", "Type$ Number", "PayLife"},
	} {
		src := strings.Replace(payLifeProcessor, tc.from, tc.to, 1)
		c := lowerScriptCard(t, "Pay Life Processor", src)
		var all []string
		for _, rf := range c.Refusals {
			all = append(all, rf.Reason)
		}
		if !strings.Contains(strings.Join(all, "\n"), tc.want) {
			t.Errorf("%s: refusals %v, want one containing %q", tc.name, all, tc.want)
		}
	}
}
