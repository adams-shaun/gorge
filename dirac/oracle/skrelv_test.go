package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// gorgeDefinedKWFixTime is gorge cr-pump-definedkw-chosencolor (5026a0910):
// before it Skrelv's Pump granted no keyword.
const gorgeDefinedKWFixTime = "20261026105853"

// The Skrelv lockstep: Skrelv, Defector Mite's activation (a
// colour, then toxic 1, hexproof from it and "can't be blocked by creatures
// of that colour" on another creature until end of turn), against green and
// white blockers, red Shocks and blue Prodigal Sorcerers (the hexproof). The expected answer at every decision is
// gorge's.
var kskrelvDecks = [2][]cardset.Entry{
	{
		{Name: "Skrelv, Defector Mite", Count: 12}, {Name: "Grizzly Bears", Count: 6}, {Name: "Suntail Hawk", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 8},
	},
	{
		{Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4}, {Name: "Suntail Hawk", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Prodigal Sorcerer", Count: 6}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3}, {Name: "Island", Count: 5},
	},
}

func TestSkrelvLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeDefinedKWFixTime) {
		t.Skip("gorge before cr-pump-definedkw-chosencolor (5026a0910)")
	}
	lockstepSet(t, "skrelv", kskrelvDecks, 600, "KSKRELV_SEEDS")
}

// TestSkrelvLockstepMutation: with the grants dropped, some seed diverges.
func TestSkrelvLockstepMutation(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeDefinedKWFixTime) {
		t.Skip("gorge before cr-pump-definedkw-chosencolor (5026a0910)")
	}
	n := lockstepRun(t, "skrelv", kskrelvDecks, 200, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				p := append([]kop.Instr(nil), abs[j].Prog...)
				for x := range p {
					if p[x].Ext() == kop.ExtColorGrant {
						p[x] = kop.Instr{Op: kop.OpNop}
					}
				}
				abs[j].Prog = p
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the grants dropped")
	}
}
