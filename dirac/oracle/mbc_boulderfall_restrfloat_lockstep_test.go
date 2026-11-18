package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 106.6 with CR 601.2b, 601.2c, 601.2f-h, 733.1: an Abstract Paintmage on
// the battlefield adds {U}{R} at the beginning of its controller's first main
// phase, floating it restricted to instants and sorceries. A two-target Boulderfall cast beside the float pays through tapForSpellPlain's window
// fallback: the planner arm has no witness for a payment the pool's admitted
// float would fund (State.rb refuses it), so the cast's CR 601.2g fallback
// window runs — whose settle (windowSpellSettled) discounts the admitted
// floating batches itself. The caller handed that window the cost already
// shortened by restrAdmShortCost, so the window discounted the float twice:
// it tapped one Mountain short of the payment gorge's window funds (gorge
// taps the last generic; the kernel's walk had settled at the twice-shortened
// cost). The recording pins the settle's shape.
const mbcFBFloatRepro = "boulderfall_paid_over_a_restricted_float"

var mbcFBFloatWhy = "an Abstract Paintmage's first-main-phase trigger leaves {U}{R} floating restricted to instants and sorceries, and the two-target Boulderfall cast beside it pays through the CR 601.2g fallback window over that float"

var mbcFBFloatDecks = [2][]string{
	append(deckOf(6, "Island"),
		append(deckOf(12, "Mountain"),
			append(deckOf(12, "Abstract Paintmage"), deckOf(10, "Boulderfall")...)...)...),
	deckOf(40, "Mountain"),
}

const (
	mbcFBCastNone uint8 = iota
	mbcFBCastPaintmage
	mbcFBCastBoulder
)

// mbcFBFloatPolicy steers seat 0's plays toward the float: a Paintmage down,
// lands beside it, then one Violent Eruption cast with two targets in a first
// main phase whose float is still in the pool (the trigger floats {U}{R}
// there and nothing else spends it). Everything else — including both
// payments and every decision of seat 1 — falls to the heuristic bot.
type mbcFBFloatPolicy struct {
	heur Policy
	kind uint8
	// pm, fired, tgts: the Paintmage and Boulderfalls the policy has started and
	// the targets it has picked on the live Boulderfall's ask.
	pm, fired, tgts int
	// sawPM, sawFB, tookPM, tookFB: the record loop's diagnostics.
	sawPM, sawFB, tookPM, tookFB int
}

func mbcFBFloatPolicyFor(seed uint64) Policy {
	return &mbcFBFloatPolicy{heur: HeuristicPolicy(seed)}
}

func (p *mbcFBFloatPolicy) Pick(c *PickCtx) Canon {
	if c.D.Kind == DPriority {
		if a, ok := p.steer(c); ok {
			return a
		}
	} else if p.kind == mbcFBCastBoulder {
		if a, ok := p.answer(c); ok {
			return a
		}
	}
	return p.heur.Pick(c)
}

// steer takes one seat-0 priority action in the float's order, or waits.
func (p *mbcFBFloatPolicy) steer(c *PickCtx) (Canon, bool) {
	p.kind = mbcFBCastNone
	if c.D.Player != 0 {
		return 0, false
	}
	um, restr := mbcFBFloatPool(c.G)
	pms, islands := 0, 0
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
		o := c.G.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Abstract Paintmage":
			pms++
		case "Island":
			islands++
		}
	}
	var island, land, pm, fb Canon
	var haveIsland, haveLand, havePM, haveFB bool
	for _, o := range c.D.Options {
		switch {
		case o.Kind() == ActPlayLand:
			if optionFaceName(c, o) == "Island" {
				island, haveIsland = o, true
			} else {
				land, haveLand = o, true
			}
		case o.Kind() == ActCast && optionFaceName(c, o) == "Abstract Paintmage":
			pm, havePM = o, true
			p.sawPM++
		case o.Kind() == ActCast && optionFaceName(c, o) == "Boulderfall":
			fb, haveFB = o, true
			p.sawFB++
		}
	}
	switch {
	case havePM && pms == 0:
		// the float's maker: its first-main-phase trigger starts floating the
		// next turn's {U}{R} (this turn's already passed when it entered)
		p.kind = mbcFBCastPaintmage
		p.pm++
		p.tookPM++
		return pm, true
	case haveIsland && islands == 0:
		// the Paintmage's {U}: one Island prints it, the Mountains do not
		return island, true
	case haveLand:
		p.tookPM++ // a diagnostic reuse: the lands the steering played
		return land, true
	case pms != 0 && p.fired < 2 && haveFB:
		// the cast must run short: {6}{R}{R} over the floating {U}{R} leaves
		// six units for six Mountain taps (the settle's own discount takes
		// two, the double discount four more), and no plain unit lies beside
		// the float
		if restr < 2 || um < 6 {
			return Pack(ActPass, RefNone, RefNone, 0), true
		}
		p.kind = mbcFBCastBoulder
		p.tgts = 0
		p.fired++
		p.tookFB++
		return fb, true
	}
	return Pack(ActPass, RefNone, RefNone, 0), true
}

func (p *mbcFBFloatPolicy) answer(c *PickCtx) (Canon, bool) {
	if c.D.Kind == DTarget {
		// a per-target-priced ask (gorge's Decision.AffordableTargets): stop
		// picking once the caster can provably pay no further target, so the
		// choice stands and the cast is not reversed (CR 601.2h, 733.1)
		if p.tgts < 2 && (!c.D.TargetLimited || c.D.TargetLeft > 0) {
			for _, o := range c.D.Options {
				if o.Kind() == ActTarget {
					p.tgts++
					return o, true
				}
			}
		}
		for _, o := range c.D.Options {
			if o.Kind() == ActDone {
				return o, true
			}
		}
	}
	return 0, false
}

// mbcFBFloatPool counts seat 0's untapped Mountains and the restricted batches
// in its pool: the float the Boulderfall pays over is restricted, and the
// scenario keeps plain units out of the pool so the window's settle is what
// moves.
func mbcFBFloatPool(g *GorgeSide) (mountains int, restr int32) {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		if o.Face().Name == "Mountain" && !o.Tapped {
			mountains++
		}
	}
	for _, b := range g.E.G.Players[0].RestrictedMana {
		restr += b.Amount
	}
	return mountains, restr
}

// mbcFBFloatMarks is a gorge log's scenario marks: the record loop's
// diagnostics read them to see how far a game's steering got.
type mbcFBFloatMarks struct {
	pms    int // Paintmages that entered seat 0's battlefield
	fbs    int // Boulderfalls put on the stack
	tgts   int // Boulderfall target choices of two or more
	floats int // restricted units floating when a two-target ask ran
}

// mbcFBFloatScan reads the finished gorge log for the scenario: a Boulderfall chose two targets on a step whose pool held admitted restricted
// units (the mana-restriction ManaAdds of a Paintmage trigger, less their
// spends), and resolved.
func mbcFBFloatScan(g *GorgeSide) (mk mbcFBFloatMarks, reached bool) {
	var pm bool
	var batch [8]int32
	var fbStack state.ObjID
	var fbTgts int
	fire := false
	for _, e := range g.E.L.Events {
		switch e.Kind {
		case events.MoveZone:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
				switch {
				case e.To == state.ZBattlefield && o.Face().Name == "Abstract Paintmage":
					pm = true
					mk.pms++
				case e.To == state.ZGraveyard && e.Obj == fbStack && fire:
					return mk, true
				}
			}
		case events.PutOnStack:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Boulderfall" {
				fbStack, fbTgts, fire = e.Obj, 0, false
				mk.fbs++
			}
		case events.ManaAdd:
			c := mbcFBFloatCol(e.Counter)
			if c < 0 {
				continue
			}
			if e.Amount > 0 && pm && strings.HasPrefix(e.Text, "mana-restriction:") {
				batch[c] += e.Amount
			} else if e.Amount < 0 && batch[c] > 0 {
				batch[c] -= min(batch[c], -e.Amount)
			}
		case events.ManaClear, events.StepChange, events.TurnChange:
			if !fire {
				batch = [8]int32{}
			}
		case events.TargetsChosen:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Boulderfall" &&
				e.Obj == fbStack {
				n := len(e.IDs)
				if e.Amount == 1 || e.Amount == 3 { // a single player target rides Player
					n = 1
				}
				fbTgts += n
				if fbTgts >= 2 && mbcFBFloatFloating(batch) {
					mk.tgts++
					mk.floats = int(mbcFBFloatFloatingN(batch))
					fire = true
				}
			}
		case events.SpellCastReversed:
			if e.Obj == fbStack {
				fbStack, fbTgts, fire = 0, 0, false
			}
		}
	}
	return mk, false
}

func mbcFBFloatCol(counter string) int {
	if counter == "" {
		return int(state.MC)
	}
	if counter[0] == 'S' && len(counter) == 2 {
		return int(state.ManaIndex(counter[1]))
	}
	return int(state.ManaIndex(counter[0]))
}

func mbcFBFloatFloating(batch [8]int32) bool {
	return mbcFBFloatFloatingN(batch) > 0
}

func mbcFBFloatFloatingN(batch [8]int32) int32 {
	var n int32
	for _, v := range batch {
		n += v
	}
	return n
}

// mbcFBFloatReached reports the scan's reach.
func mbcFBFloatReached(g *GorgeSide) bool {
	_, reached := mbcFBFloatScan(g)
	return reached
}

// mbcFBFloatPlay plays one lockstep game of repro r under pol and reports
// whether the game reached the float beside the taxed cast. finish, when set,
// runs on the gorge side after the game.
func mbcFBFloatPlay(t *testing.T, reg *cards.Registry, set cardset.Set, r *Repro, pol Policy, finish func(*GorgeSide)) (Result, bool) {
	t.Helper()
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = 3000
	rc.MaxTurns = 40
	var reached bool
	rc.Finish = func(g *GorgeSide) {
		reached = mbcFBFloatReached(g)
		if finish != nil {
			finish(g)
		}
	}
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, pol, rc), reached
}

// TestMBCBoulderfallRestrFloatReprosLockstep replays the recorded game: no
// divergence, every scripted action offered — the CR 601.2g fallback window
// funds the two-target cast exactly once over the admitted floating batch.
func TestMBCBoulderfallRestrFloatReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, mbcFBFloatRepro)
}

// TestMBCBoulderfallRestrFloatRecord plays the policy over seeds until one
// reaches the float beside the two-target Violent Eruption cast, and writes the
// repro. It runs only with PINS_RECORD=1.
func TestMBCBoulderfallRestrFloatRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	set := printedDelveSet(t, mbcFBFloatDecks)
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{mbcFBFloatRepro + "_0", mbcFBFloatRepro + "_1"},
			Cards: mbcFBFloatDecks, Seed: seed, PassElision: true, Surface: "full",
			Kernel: "custom", Pool: "custom", Why: mbcFBFloatWhy}
		res, reached := mbcFBFloatPlay(t, reg, set, r, mbcFBFloatPolicyFor(seed), nil)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %v\nref %v\nker %v\n%s", seed, res.Div.At,
				res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail)
		}
		if !reached {
			continue
		}
		r.setActions(res.Actions)
		if err := r.Write(filepath.Join("testdata", "repro", mbcFBFloatRepro+".repro.json")); err != nil {
			t.Fatal(err)
		}
		t.Logf("seed %d: %d decisions", seed, len(res.Actions))
		return
	}
	t.Fatalf("no seed reached a Paintmage float beside a two-target Boulderfall cast")
}
