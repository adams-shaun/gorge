package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// The std-mkm evidence cards whose only unreachable key is their
// "when <card> is turned face up" trigger. Each deck pairs the card with a
// mana floater (Iridescent Tiger's enter trigger adds {W}{U}{B}{R}{G}, the
// Burning-Tree Emissary's {R}{G}): the face-down cast and the turn-up special
// action are both floating-pool-gated offers, so a clean game only reaches
// the trigger through a floated pool.
var kturnupCards = []struct {
	card  string
	upKey string // the card's turn-face-up trigger key
	deck  []deck.Entry
}{{
	card:  "Nervous Gardener",
	upKey: "f0/t0",
	deck: []deck.Entry{
		{Name: "Nervous Gardener", Count: 12},
		{Name: "Iridescent Tiger", Count: 10},
		{Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 12},
	},
}, {
	card:  "Forum Familiar",
	upKey: "f0/t0",
	deck: []deck.Entry{
		{Name: "Forum Familiar", Count: 12},
		{Name: "Iridescent Tiger", Count: 10},
		{Name: "Mountain", Count: 6},
		{Name: "Forest", Count: 12},
	},
}, {
	card:  "Offender at Large",
	upKey: "f0/t1",
	deck: []deck.Entry{
		{Name: "Offender at Large", Count: 10},
		{Name: "Iridescent Tiger", Count: 10},
		{Name: "Mountain", Count: 10},
		{Name: "Forest", Count: 10},
	},
}, {
	card:  "Aurelia's Vindicator",
	upKey: "f0/t0",
	deck: []deck.Entry{
		{Name: "Aurelia's Vindicator", Count: 10},
		{Name: "Iridescent Tiger", Count: 10},
		{Name: "Mountain", Count: 10},
		{Name: "Forest", Count: 10},
	},
}}

// kturnupPolicy plays lands, casts the floater (its enter trigger floats
// mana), casts the Disguise card face down from the floated pool, and turns
// it face up whenever the special action is offered. picked counts the
// face-down casts, floater casts, turn-up offers seen and turn-up picks made.
func kturnupPolicy(card string, seed uint64, picked *[4]int) Policy {
	rnd := RandomPolicy(seed)
	tu, _ := canon.SpecialCode("turn_face_up")
	return PolicyFunc(func(c *PickCtx) Canon {
		var land, faceDown, turnUp, float, cast Canon
		var haveLand, haveFaceDown, haveTurnUp, haveFloat, haveCast bool
		for _, o := range c.D.Options {
			n := optionFaceName(c, o)
			switch {
			case o.Kind() == canon.ActPlayLand:
				land, haveLand = o, true
			case o.Kind() == canon.ActCastFace && n == card:
				faceDown, haveFaceDown = o, true
			case o.Kind() == canon.ActSpecial && o.Mode()&0xf == tu:
				turnUp, haveTurnUp = o, true
				picked[2]++
			case o.Kind() == canon.ActCast && (n == "Iridescent Tiger" || n == "Burning-Tree Emissary"):
				if !haveFloat {
					float, haveFloat = o, true
				}
			case o.Kind() == canon.ActCast && n == card:
				cast, haveCast = o, true
			}
		}
		switch {
		case haveTurnUp:
			picked[3]++
			return turnUp
		case haveFaceDown:
			picked[0]++
			return faceDown
		case haveFloat:
			picked[1]++
			return float
		case haveLand:
			return land
		case haveCast:
			return cast
		}
		return rnd.Pick(c)
	})
}

// TestCoverageCreditsTurnFaceUpTrigger: each std-mkm Disguise card whose
// evidence key is its "when turned face up" trigger earns that key in a clean
// lockstep game the card is cast face down in and then turned face up.
func TestCoverageCreditsTurnFaceUpTrigger(t *testing.T) {
	c, _ := testCorpus(t)
	seeds := 8
	if testing.Short() {
		seeds = 3
	}
	for _, tc := range kturnupCards {
		tc := tc
		t.Run(tc.card, func(t *testing.T) {
			t.Parallel()
			cd, ok := c.Reg.Lookup(tc.card)
			if !ok {
				t.Fatalf("%q not in the corpus", tc.card)
			}
			keys := strings.Join(CoverageKeys(cd), ",")
			if !strings.Contains(keys, tc.upKey) {
				t.Fatalf("%s's keys [%s] have no %s", tc.card, keys, tc.upKey)
			}
			es := append([]deck.Entry(nil), tc.deck...)
			cc := &Corpus{Reg: c.Reg, decks: map[string][]*cards.Card{}}
			cc.AddDecks(map[string]deck.File{
				"tu-a": {Name: "tu-a", Cards: es},
				"tu-b": {Name: "tu-b", Cards: es},
			})
			var pairs []Pairing
			for i := 0; i < seeds; i++ {
				pairs = append(pairs, Pairing{Index: i, A: "tu-a", B: "tu-b", Seed: uint64(i + 1)})
			}
			var credited int
			picked := [4]int{}
			cfg := FuzzConfig{Policy: func(seed uint64) Policy {
				return kturnupPolicy(tc.card, seed, &picked)
			}, Coverage: true}
			s := Fuzz(cc, pairs, GorgeKernel(Config{}), cfg, nil)
			if s.Diverged != 0 || s.Errors != 0 || s.Coverage == nil {
				t.Fatalf("gorge mirror: %d diverged, %d errors (%v)", s.Diverged, s.Errors, s.ByClass)
			}
			rc := s.Coverage.Cards[tc.card]
			if rc == nil {
				t.Fatalf("no %s row", tc.card)
			}
			credited = rc.Used[tc.upKey]
			t.Logf("%s: cast %d, used %v, unused %v", tc.card, rc.Cast, rc.Used, rc.Unused())
			t.Logf("%s: face-down casts %d, floater casts %d, turn-up offers %d, turn-up picks %d", tc.card, picked[0], picked[1], picked[2], picked[3])
			for _, fl := range []string{"Iridescent Tiger", "Burning-Tree Emissary"} {
				if fc := s.Coverage.Cards[fl]; fc != nil && fc.Cast > 0 {
					t.Logf("%s: cast %d used %v", fl, fc.Cast, fc.Used)
				}
			}
			if rc.Cast == 0 {
				t.Fatalf("%s was never cast", tc.card)
			}
			if picked[0] == 0 || picked[3] == 0 {
				t.Fatalf("%s: the deck never reached the play (face-down casts %d, turn-up picks %d)", tc.card, picked[0], picked[3])
			}
			if credited == 0 {
				t.Errorf("%s: the turn-face-up key %s was never credited (used %v)", tc.card, tc.upKey, rc.Used)
			}
		})
	}
}
