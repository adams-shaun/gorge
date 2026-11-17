package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The P/T switch lockstep (dirac/kgen/lower_kwmark.go's KwMarkSwitchPT,
// kernel kw_kwmark.go and chars_gen.go ptGen): Pump / PumpAll KW$ HIDDEN
// CARDNAME's power and toughness are switched on a target (Twisted Image,
// About Face, Invert), on itself (Aeromoeba, Aquamoeba, Calcite Snapper's
// landfall) and on every creature (Mannichi), against gorge's layer-7
// SubSwitch effect (applied after pumps and counters; an even count
// cancels). The asymmetric creatures make every switch observable, and a
// 0/4 Wall of Omens switched to 4/0 dies to the state-based action.
func ptSwitchDecks() [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: "Twisted Image", Count: 4}, {Name: "About Face", Count: 3}, {Name: "Invert", Count: 2},
			{Name: "Aeromoeba", Count: 2}, {Name: "Aquamoeba", Count: 3}, {Name: "Calcite Snapper", Count: 2},
			{Name: "Mannichi, the Fevered Dream", Count: 2}, {Name: "Shock", Count: 3},
			{Name: "Island", Count: 10}, {Name: "Mountain", Count: 9},
		},
		{
			{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Wall of Omens", Count: 4},
			{Name: "Giant Growth", Count: 3}, {Name: "Travel Preparations", Count: 2}, {Name: "Twisted Image", Count: 2},
			{Name: "Forest", Count: 12}, {Name: "Island", Count: 5}, {Name: "Plains", Count: 4},
		},
	}
}

// ptSwitchCarriers are the deck's switch carriers (each card's name as gorge
// spells its face).
var ptSwitchCarriers = map[string]bool{
	"Twisted Image": true, "About Face": true, "Invert": true, "Aeromoeba": true,
	"Aquamoeba": true, "Mannichi, the Fevered Dream": true,
}

// ptSwitchPolicy is castFirstPolicy that, from gorge's options, counts the
// priority decisions offering a carrier's cast or activation.
func ptSwitchPolicy(st *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		counted := map[int]bool{}
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DPriority && !counted[c.Seq] {
				for _, a := range c.D.Options {
					if k := a.Kind(); k != ActCast && k != ActCastFace && k != ActActivate {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok {
						continue
					}
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ptSwitchCarriers[ob.Face().Name] {
						*st++
						counted[c.Seq] = true
						break
					}
				}
			}
			return inner.Pick(c)
		})
	}
}

func TestPTSwitchLockstep(t *testing.T) {
	t.Parallel()
	offers := new(int)
	lockstepRunPolicy(t, "ptswitch", ptSwitchDecks(), 48, true, nil, nil, ptSwitchPolicy(offers))
	if *offers == 0 {
		t.Fatal("no switch carrier was ever castable or activatable; the lockstep proves nothing")
	}
	t.Logf("switch carrier offers: %d", *offers)
}

// TestPTSwitchLockstepMutation: with the switch mark dropped from every
// ExtKwMark (the kernel never switches), some seed diverges.
func TestPTSwitchLockstepMutation(t *testing.T) {
	t.Parallel()
	if n := lockstepRun(t, "ptswitch", ptSwitchDecks(), 48, false, ptSwitchDropMark); n == 0 {
		t.Fatal("no seed diverged with the P/T switch mark dropped")
	}
}

// ptSwitchDropMark clears kop.KwMarkSwitchPT from every ExtKwMark.
func ptSwitchDropMark(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		abs := append([]kop.Ability(nil), cards[i].Abilities...)
		for j := range abs {
			prog := append([]kop.Instr(nil), abs[j].Prog...)
			for p := range prog {
				if prog[p].Ext() == kop.ExtKwMark {
					prog[p].C &^= kop.KwMarkSwitchPT
				}
			}
			abs[j].Prog = prog
		}
		cards[i].Abilities = abs
	}
	tbl.Cards = cards
}
