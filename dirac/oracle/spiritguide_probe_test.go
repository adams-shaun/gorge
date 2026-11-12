package oracle

import (
	"fmt"
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
	windowsHoldGuide  int // of those, the windows whose payer held a guide in hand
	windowListsGuide  int
	windowPicksGuide  int
	fallbacks         int
	taps              int // caster battlefield sources tapped out of band at a target ask (tapOne / tapAll runs)
	fallbackNoBF      int // fallback windows with no untapped caster battlefield source (only the guide can pay)
	fallbackBF        int // fallback windows with an untapped caster battlefield source left
	fallbackBFPicks   int // of those, the windows the oracle answered with the guide anyway
	// shapes tallies each window that listed the guide by the order of its
	// options and the option the oracle's answer took: a letter per option (A
	// auto-fill, B a mana/activate option of a battlefield or other source, H
	// the hand guide's, x any other: done, cancel, undo), then "->" and the
	// answer's letter.
	shapes map[string]int
	// unlisted tallies the windows that listed the guide but no battlefield
	// option while a caster battlefield source was untapped: the source's
	// name, and whether a payment (owed mana) rode the window.
	unlisted map[string]int
	// byPool tallies the windows whose payer held a guide, by whether the
	// window listed one and the mana already in the payer's pool.
	byPool map[string]int
	sample []string
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

// windowShape is the shape key of a window d that listed a hand guide (see
// spiritGuideObs.shapes); idx is the option index the answer took.
func windowShape(g *GorgeSide, d *decision.Decision, idx int) string {
	letter := func(o decision.Option) byte {
		switch {
		case o.Kind == decision.OptAutoFill:
			return 'A'
		case o.Obj != 0 && isHandGuide(g, o.Obj):
			return 'H'
		case o.Kind == "mana" || o.Kind == "activate":
			return 'B'
		}
		return 'x'
	}
	var b strings.Builder
	ans := byte('?')
	for _, o := range d.Options {
		b.WriteByte(letter(o))
		if o.Index == idx {
			ans = letter(o)
		}
	}
	b.WriteString("->")
	b.WriteByte(ans)
	return b.String()
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

// casterSources lists me's own untapped battlefield permanents (not the
// other seats' permanents pay.BattlefieldManaSourceIDs appends). It is every
// untapped permanent, mana source or not: a Grizzly Bears, or a Spirit Guide
// cast as a creature (its mana ability works from the hand only), counts.
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
	deck := spiritGuideDeck(guide)
	if mode != tapNone {
		deck = spiritGuideTargetDeck(guide)
	}
	return runSpiritGuideDeck(t, deck, seeds, mode)
}

// runSpiritGuideDeck is runSpiritGuideProbe on an explicit deck list.
func runSpiritGuideDeck(t *testing.T, deck []string, seeds int, mode tapMode) *spiritGuideObs {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	obs := &spiritGuideObs{shapes: map[string]int{}, unlisted: map[string]int{}, byPool: map[string]int{}}
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
			for _, id := range g.E.G.Zone(state.ZHand, d.Player) {
				if isHandGuide(g, id) {
					obs.windowsHoldGuide++
					hasListed := false
					for _, o := range d.Options {
						hasListed = hasListed || o.Obj != 0 && isHandGuide(g, o.Obj)
					}
					obs.byPool[fmt.Sprintf("listed=%v pool=%d", hasListed, g.E.G.Players[d.Player].Pool.Total())]++
					break
				}
			}
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
				obs.shapes[windowShape(g, d, idx)]++
				if shape := windowShape(g, d, idx); bf && !strings.Contains(shape, "B") {
					for _, id := range casterSources(g, d.Player) {
						obs.unlisted[fmt.Sprintf("%s owed=%v fallback=%v", g.E.G.Obj(id).Face().Name, d.ManaPayment != nil, d.PaymentFallback != nil)]++
					}
				}
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

// TestSpiritGuideWindowOrder measures how gorge orders a payment window that
// holds a hand guide and how the oracle's answer (payexec.WindowAnswer) takes
// between the options, in the forced state (tapAll) and with one caster
// permanent tapped (tapOne). Every window listed the guide, so the shape keys
// cover every window the run opened. The claims it asserts: the listed options
// are battlefield sources first (B), then one H per guide in the hand, then the
// non-source rows (x); the answer is the first B, else the first H, so the
// guide is answered only when no battlefield option is listed; and a window
// that lists no B while the caster has an untapped permanent has no untapped
// land among them (the permanents are Bears and creature-cast guides, which
// have no mana ability on the battlefield).
func TestSpiritGuideWindowOrder(t *testing.T) {
	for _, guide := range []string{"Elvish Spirit Guide", "Simian Spirit Guide"} {
		land := "Forest"
		if strings.HasPrefix(guide, "Simian") {
			land = "Mountain"
		}
		for _, m := range []struct {
			name string
			mode tapMode
		}{{"tapOne", tapOne}, {"tapAll", tapAll}} {
			o := runSpiritGuideProbe(t, guide, 120, m.mode)
			t.Logf("%s %s: windows=%d lists=%d picks=%d shapes=%v unlisted=%v", m.name, guide, o.windows, o.windowListsGuide, o.windowPicksGuide, o.shapes, o.unlisted)
			if o.windowListsGuide == 0 || o.windowPicksGuide == 0 {
				t.Fatalf("%s %s: the guide was listed %d and answered %d times: the probe measured nothing", m.name, guide, o.windowListsGuide, o.windowPicksGuide)
			}
			sawB := false
			for shape, n := range o.shapes {
				opts, ans, _ := strings.Cut(shape, "->")
				sawB = sawB || strings.Contains(opts, "B")
				if strings.Contains(opts, "H") && strings.Contains(opts[strings.Index(opts, "H"):], "B") {
					t.Errorf("%s %s: shape %q (%d windows) lists a battlefield option after a hand guide", m.name, guide, shape, n)
				}
				if first := strings.TrimLeft(opts, "x"); first == "" || ans != first[:1] {
					t.Errorf("%s %s: shape %q (%d windows): the answer is not the first source listed", m.name, guide, shape, n)
				}
			}
			if m.mode == tapOne && !sawB {
				t.Errorf("%s %s: no window listed a battlefield source beside the guide", m.name, guide)
			}
			for key := range o.unlisted {
				if strings.HasPrefix(key, land+" ") {
					t.Errorf("%s %s: an untapped %s was not listed as a source: %q", m.name, guide, land, key)
				}
			}
		}
	}
}

// TestSpiritGuideOffColour measures whether gorge lists a guide whose colour
// the cast cannot use, and whether the oracle answers with it: a Simian Spirit
// Guide (red) beside Giant Growth ({G}) and Forests, an Elvish Spirit Guide
// (green) beside Lightning Bolt ({R}) and Mountains, every caster permanent
// tapped at the target ask (tapAll). The Giant Growth deck adds Grizzly Bears,
// the only creatures it can target.
func TestSpiritGuideOffColour(t *testing.T) {
	for _, c := range []struct {
		guide, spell, land string
		listed             bool // the guide's colour can pay what the cast owes
	}{
		{"Simian Spirit Guide", "Giant Growth", "Forest", false},
		{"Elvish Spirit Guide", "Lightning Bolt", "Mountain", false},
		{"Elvish Spirit Guide", "Arc Lightning", "Mountain", true}, // {2}{R}: green pays the generic part
	} {
		var deck []string
		if c.spell == "Giant Growth" {
			for range 6 {
				deck = append(deck, "Grizzly Bears") // Giant Growth needs a creature to target
			}
		}
		for range 12 {
			deck = append(deck, c.guide)
		}
		for range 10 {
			deck = append(deck, c.spell)
		}
		for len(deck) < 40 {
			deck = append(deck, c.land)
		}
		o := runSpiritGuideDeck(t, deck, 120, tapAll)
		t.Logf("%s with %s: windows=%d fallbacks=%d windowsHoldGuide=%d lists=%d picks=%d byPool=%v", c.guide, c.spell, o.windows, o.fallbacks, o.windowsHoldGuide, o.windowListsGuide, o.windowPicksGuide, o.byPool)
		if o.taps == 0 || o.windowsHoldGuide == 0 {
			t.Fatalf("%s with %s: %d taps, %d windows with a guide in hand: the probe measured nothing", c.guide, c.spell, o.taps, o.windowsHoldGuide)
		}
		if c.listed && (o.byPool["listed=false pool=2"] == 0 || o.byPool["listed=false pool=0"] != 0 || o.byPool["listed=false pool=1"] != 0) {
			// the green guide is withheld once the generic {2} is paid and only {R} is owed
			t.Errorf("%s with %s: unlisted windows by pool %v, want some at pool=2 (the generic paid) and none below", c.guide, c.spell, o.byPool)
		}
		if c.listed != (o.windowListsGuide > 0) {
			t.Errorf("%s with %s: the guide was listed %d times of %d windows holding one, want listed=%v", c.guide, c.spell, o.windowListsGuide, o.windowsHoldGuide, c.listed)
		}
	}
}
