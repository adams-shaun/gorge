package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Probe B of the Spirit Guide scoping (dirac: lower Simian / Elvish Spirit
// Guide). gorge's own decisions are driven through GorgeSide on a deck whose
// hand holds Elvish Spirit Guides, {1}{G} Grizzly Bears and few Forests, on
// both option surfaces, and every decision that could reach the guide's
// hand-zone mana ability is tallied:
//
//  1. a canonical priority option list that offers a cast while no land is in
//     play and the only mana is a hand guide (gorge's planner prices no hand
//     source: the cast must not be offered);
//  2. a raw priority decision that offers the guide's "activate" (gorge does;
//     the oracle's canonical list never does, doc.go "Autopay priority
//     vocabulary");
//  3. a payment window or colour ask the side answers beneath the surface that
//     lists the guide (as a mana/activate option) and the answers that pick it.
type spiritGuideObs struct {
	games, decisions int
	// priorities that held a hand guide, by what they offered.
	guideInHand       int
	rawActivateOffer  int // raw priority decisions listing the guide's activate
	castNoLandOneGuid int // canonical cast offered with no untapped land and no pool: only a guide could pay
	castWithLand      int
	noSourceGuide     int // priorities with a hand guide, no untapped caster source, an empty pool and another card in hand (the state castNoLandOneGuid needs)
	windows           int // internal asks answered beneath the surface
	windowListsGuide  int
	windowPicksGuide  int
	fallbacks         int
	taps              int // caster battlefield sources tapped out of band at a target ask (tapOne / tapAll runs)
	fallbackNoBF      int // fallback windows with no untapped caster battlefield source (only the guide can pay)
	fallbackBF        int // fallback windows with an untapped caster battlefield source left
	fallbackBFPicks   int // of those, the windows the oracle answered with the guide anyway
	sample            []string
}

func (o *spiritGuideObs) note(s string) {
	if len(o.sample) < 8 {
		o.sample = append(o.sample, s)
	}
}

func isHandGuide(g *GorgeSide, id state.ObjID) bool {
	ob := g.E.G.Obj(id)
	return ob != nil && ob.Zone == state.ZHand && ob.Face() != nil && strings.HasSuffix(ob.Face().Name, "Spirit Guide")
}

func spiritGuideDeck(guide string) []string {
	var d []string
	for range 12 {
		d = append(d, guide)
	}
	for range 14 {
		d = append(d, "Grizzly Bears")
	}
	for len(d) < 40 {
		d = append(d, "Forest")
	}
	return d
}

// spiritGuideTargetDeck is a targeted one-pip spell of the guide's colour (its
// plan executes after the target ask) beside basic lands and the guide:
// Lightning Bolts and Mountains for the Simian, Giant Growths, Grizzly Bears
// and Forests for the Elvish.
func spiritGuideTargetDeck(guide string) []string {
	spell, land := "Lightning Bolt", "Mountain"
	var d []string
	if strings.HasPrefix(guide, "Elvish") {
		spell, land = "Giant Growth", "Forest"
		for range 6 {
			d = append(d, "Grizzly Bears")
		}
	}
	for range 12 {
		d = append(d, guide)
	}
	for range 10 {
		d = append(d, spell)
	}
	for len(d) < 40 {
		d = append(d, land)
	}
	return d
}

// tapMode is the out-of-band change a probe run makes at a target ask (after the
// cast was announced and its plan chosen, before the plan executes).
type tapMode int

const (
	tapNone tapMode = iota
	tapOne          // the caster's first untapped battlefield source (a planned source, or not)
	tapAll          // every untapped battlefield source of the caster: the forced state, no battlefield source left
)

// casterSources lists me's own untapped battlefield mana sources (not the
// other seats' permanents pay.BattlefieldManaSourceIDs appends).
func casterSources(g *GorgeSide, me state.PlayerID) []state.ObjID {
	var ids []state.ObjID
	for _, id := range pay.BattlefieldManaSourceIDs(g.E.G, me) {
		if ob := g.E.G.Obj(id); ob != nil && ob.Controller == me && !ob.Tapped {
			ids = append(ids, id)
		}
	}
	return ids
}

func runSpiritGuideProbe(t *testing.T, guide string, seeds int, mode tapMode) *spiritGuideObs {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	obs := &spiritGuideObs{}
	deck := spiritGuideDeck(guide)
	if mode != tapNone {
		deck = spiritGuideTargetDeck(guide)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, n := range deck {
			c, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sg-a", "sg-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		g.OnInternal = func(d *decision.Decision, idx int) {
			obs.windows++
			if d.PaymentFallback != nil {
				obs.fallbacks++
			}
			bf := len(casterSources(g, d.Player)) > 0
			listed := false
			for _, o := range d.Options {
				if o.Obj != 0 && isHandGuide(g, o.Obj) {
					listed = true
					if o.Index == idx {
						if d.PaymentFallback != nil && bf {
							obs.fallbackBFPicks++
						}
						obs.windowPicksGuide++
						obs.note("window pick: " + d.Prompt + " -> " + o.Label)
					}
				}
			}
			if d.PaymentFallback != nil {
				if bf {
					obs.fallbackBF++
				} else {
					obs.fallbackNoBF++
				}
			}
			if listed {
				obs.windowListsGuide++
				obs.note("window lists guide: " + d.Prompt)
			}
		}
		pol := RandomPolicy(seed)
		taps := 0
		for n := 0; n < 600; n++ {
			cd := g.Step()
			if cd.Kind == DNone {
				break
			}
			obs.decisions++
			if mode != tapNone && cd.Kind == DTarget {
				// A source of the caster tapped after the cast was announced
				// (the payment-plan tests tap the plan's own source, which
				// this probe cannot name: tapOne taps the first source, tapAll
				// every one): a plan whose source changed stops and the manual
				// window opens. Only the caster's own sources are touched.
				me := state.PlayerID(cd.Player)
				hasGuide := false
				for _, id := range g.E.G.Zone(state.ZHand, me) {
					hasGuide = hasGuide || isHandGuide(g, id)
				}
				if hasGuide {
					for _, id := range casterSources(g, me) {
						g.E.Emit(events.Event{Kind: events.Tap, Obj: id})
						taps++
						if mode == tapOne {
							break
						}
					}
				}
			}
			if cd.Kind == DPriority {
				me := state.PlayerID(cd.Player)
				guides := 0
				for _, id := range g.E.G.Zone(state.ZHand, me) {
					if isHandGuide(g, id) {
						guides++
					}
				}
				if guides > 0 {
					obs.guideInHand++
					if raw := g.Pending(); raw != nil {
						for _, o := range raw.Options {
							if o.Kind == "activate" && isHandGuide(g, o.Obj) {
								obs.rawActivateOffer++
								break
							}
						}
					}
					lands := len(casterSources(g, me))
					if lands == 0 && g.E.G.Players[me].Pool.Total() == 0 && len(g.E.G.Zone(state.ZHand, me)) > guides {
						obs.noSourceGuide++
					}
					for _, o := range cd.Options {
						if o.Kind() == ActCast {
							if lands == 0 && g.E.G.Players[me].Pool.Total() == 0 {
								obs.castNoLandOneGuid++
								obs.note("cast offered with no mana source in play and only a hand guide")
							} else {
								obs.castWithLand++
							}
						}
					}
				}
			}
			g.Apply(pol.Pick(&PickCtx{D: &cd, G: g, Seq: n}))
		}
		if err := g.Err(); err != nil {
			t.Fatalf("seed %d: %v", seed, err)
		}
		obs.games++
		obs.taps += taps
	}
	return obs
}

// TestSpiritGuideGorgeWindow records, for both guides, whether gorge's
// payment windows ever list or are answered with a hand-zone Spirit Guide:
// first in natural play, then with a caster source tapped after the cast was
// announced, once the first source only (a window can open with other
// battlefield sources left) and once every source (the forced state: nothing
// on the battlefield can pay).
func TestSpiritGuideGorgeWindow(t *testing.T) {
	for _, guide := range []string{"Elvish Spirit Guide", "Simian Spirit Guide"} {
		nat := runSpiritGuideProbe(t, guide, 120, tapNone)
		t.Logf("natural   %s: %+v", guide, *nat)
		if nat.guideInHand == 0 || nat.rawActivateOffer != nat.guideInHand {
			t.Fatalf("%s: gorge's raw priority offered the guide's activate %d of %d times", guide, nat.rawActivateOffer, nat.guideInHand)
		}
		if nat.noSourceGuide == 0 {
			t.Fatalf("%s: the state castNoLandOneGuid needs (a hand guide, no caster source, empty pool, another card in hand) never occurred: the assertion below would be vacuous", guide)
		}
		if nat.castNoLandOneGuid != 0 {
			t.Errorf("%s: a cast was offered with only a hand guide to pay it", guide)
		}
		one := runSpiritGuideProbe(t, guide, 120, tapOne)
		t.Logf("tapOne    %s: %+v", guide, *one)
		all := runSpiritGuideProbe(t, guide, 120, tapAll)
		t.Logf("tapAll    %s: %+v", guide, *all)
		for name, o := range map[string]*spiritGuideObs{"tapOne": one, "tapAll": all} {
			if o.taps == 0 || o.fallbacks == 0 {
				t.Fatalf("%s %s: %d taps, %d fallback windows: the probe measured nothing", guide, name, o.taps, o.fallbacks)
			}
		}
		// The forced state: no caster battlefield source left, the fallback
		// window lists the hand guide and the oracle's answer takes it.
		if all.fallbackNoBF == 0 || all.windowListsGuide == 0 || all.windowPicksGuide == 0 {
			t.Errorf("%s tapAll: %d no-source fallback windows, the guide listed %d and answered %d times, want all > 0", guide, all.fallbackNoBF, all.windowListsGuide, all.windowPicksGuide)
		}
	}
}
