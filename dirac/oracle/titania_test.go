package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The discard-or-pay lockstep (CR 702.21a, 601.2b, 601.2g-h): Titania, Rugged
// Rumbler is a {2}{B/G} spell whose additional cost is a discard or {2}, and
// her ward is a discard or {2}: after "Pay", gorge poses one ward_alt choice of
// a hand card or the {2}. The other side's burn and bounce target her, so the
// ward resolves with a full hand, an empty one, untapped lands and none.
// Crystal Grotto's filter is at times the only way to the {B} face: the
// window taps it for {C}, runs dry, and gorge still asks the discard before
// the payment reverses the cast. Gorge's answer at every decision is the
// expected one.
var ktitaniaDecks = [2][]cardset.Entry{
	{ // A: B/G, Titania and the creatures and lands to cast her, a filter land among them
		{Name: "Titania, Rugged Rumbler", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Crystal Grotto", Count: 4},
	},
	{ // B: U/R, the targeting side, which pays the ward
		{Name: "Shock", Count: 8}, {Name: "Lightning Bolt", Count: 6}, {Name: "Unsummon", Count: 6},
		{Name: "Hill Giant", Count: 2}, {Name: "Mountain", Count: 12}, {Name: "Island", Count: 6},
	},
}

type ktitaniaStats struct {
	choices, discards, mana, manaOnly, discardOnly int
}

// ktitaniaPolicy is the cast-first policy, counting gorge's ward_alt choices
// (its pending decision when it leads) and the option taken.
func ktitaniaPolicy(st *ktitaniaStats) func(uint64) Policy {
	return func(seed uint64) Policy {
		inner := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			a := inner.Pick(c)
			if c.G == nil {
				return a
			}
			d := c.G.Pending()
			if d == nil || d.ResumeKind != "ward_alt" {
				return a
			}
			st.choices++
			hasMana, hasDisc := false, false
			for _, o := range d.Options {
				switch o.Kind {
				case "ward_mana":
					hasMana = true
				case "ward_discard":
					hasDisc = true
				}
			}
			if !hasDisc {
				st.manaOnly++
			}
			if !hasMana {
				st.discardOnly++
			}
			code, _ := chooseCode("ward_mana")
			if a.Kind() == ActChoose+CanonKind(code) {
				st.mana++
			} else {
				st.discards++
			}
			return a
		})
	}
}

// TestTitaniaLockstep plays the pool with the ward choice and the hybrid
// either-or cast modelled.
func TestTitaniaLockstep(t *testing.T) {
	t.Parallel()
	st := &ktitaniaStats{}
	if divs := lockstepRunPolicy(t, "titania", ktitaniaDecks, 200, true, nil, nil, ktitaniaPolicy(st)); divs != 0 {
		return
	}
	t.Logf("%d ward choices: %d discards, %d paid {2}; %d with no card in hand, %d with no {2} offered",
		st.choices, st.discards, st.mana, st.manaOnly, st.discardOnly)
	if st.discards < 20 || st.mana < 20 {
		t.Errorf("%d discards and %d {2} payments, want >= 20 of each", st.discards, st.mana)
	}
	if st.manaOnly < 3 || st.discardOnly < 3 {
		t.Errorf("%d choices with no card in hand and %d with no {2} offered, want >= 3 of each", st.manaOnly, st.discardOnly)
	}
}

// titaniaWardAs replaces Titania's ward choice with one cost.
func titaniaWardAs(discard bool) func(*kop.Table) {
	return func(tb *kop.Table) {
		for i := range tb.Cards {
			if c := &tb.Cards[i]; c.WardDiscOrN != 0 {
				if discard {
					c.WardDiscard = 1
				} else {
					c.WardN = c.WardDiscOrN
				}
				c.WardDiscOrN = 0
			}
		}
	}
}

// TestTitaniaLockstepMutation: with the ward a discard alone or {2} alone,
// or an unpayable either-or part reversed before the pips are announced,
// some game diverges, so the pool exercises each.
func TestTitaniaLockstepMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	for _, m := range []struct {
		name   string
		flag   *bool
		mutate func(*kop.Table)
	}{
		{"ward discard alone", nil, titaniaWardAs(true)},
		{"ward {2} alone", nil, titaniaWardAs(false)},
		{"either-or reversed before the pips", &dirac.DebugAltAddNoPipsFirst, nil},
	} {
		if m.flag != nil {
			*m.flag = true
		}
		divs := lockstepRunPolicy(t, "titania", ktitaniaDecks, 60, false, m.mutate, nil, castFirstPolicy)
		if m.flag != nil {
			*m.flag = false
		}
		if divs == 0 {
			t.Errorf("%s: no seed diverged in 60", m.name)
		}
	}
}
