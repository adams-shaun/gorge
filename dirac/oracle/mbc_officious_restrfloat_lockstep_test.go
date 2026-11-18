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

// CR 106.6 with CR 601.2b, 601.2c, 601.2f-h, 733.1: an Abstract Paintmage's
// "At the beginning of your first main phase, add {U}{R}. Spend this mana only
// to cast instant and sorcery spells" floats two restricted units each main
// phase (CR 106.6). Five Boomerang casts, one per main phase, each pay their
// own {U} pip out of the fresh float and bounce a basic land to hand, leaving
// the board without an untapped printed mana source. A Fireball cast with X
// announced as 0 (so the X arm's xWindowPay does not take it, mana_window.go)
// and three targets prices {2}{R} — one {1} per target beyond the first — and
// pays through tapForSpellPlain's window fallback: the plan has no witness for
// the taxed cast — the printed pool is empty and the Gemstone Mines are
// choice-shaped riders the planner's census does not price — so the cast's CR
// 601.2g fallback window runs, whose settle (windowSpellSettled) discounts the
// admitted floating batch itself and whose walk takes a Mine. The caller
// handed that window the cost already shortened by restrAdmShortCost, so the
// window discounted the float twice: its settle asked the pool for one unit
// less than the fix's, and the payment came up one unit short where gorge's
// window (the unshortened cost) funds {2}{R} over the float and the Mine, and
// gorge resolved the cast the kernel reversed. The recording pins the settle's
// shape.
const mbcFBFloatRepro = "fireball_x0_paid_over_a_restricted_float"

var mbcFBFloatWhy = "an Abstract Paintmage floats {U}{R}, five Boomerangs spend the floats' {U} pips bouncing every basic land, and a Fireball with X announced as 0 and three targets pays {2}{R} through the CR 601.2g fallback window over the next float"

var mbcFBFloatDecks = [2][]string{
	append(deckOf(1, "Island"),
		append(deckOf(2, "Plains"),
			append(deckOf(2, "Mountain"),
				append(deckOf(2, "Gemstone Mine"),
					append(deckOf(8, "Abstract Paintmage"),
						append(deckOf(8, "Boomerang"), deckOf(8, "Fireball")...)...)...)...)...)...),
	deckOf(40, "Mountain"),
}

const (
	mbcFBCastNone uint8 = iota
	mbcFBCastPaintmage
	mbcFBCastBoomerang
	mbcFBCastFireball
)

// mbcFBFloatPolicy steers seat 0's plays toward the float: an Abstract
// Paintmage down over the Island and both Mountains, five Boomerangs (one per
// main phase, each paid by the fresh float and bouncing a basic land) until no
// basic land is left, then one Fireball with X announced as 0 and three
// targets in the next main phase over the next float. Everything else —
// including both payments and every decision of seat 1 — falls to the
// heuristic bot.
type mbcFBFloatPolicy struct {
	heur Policy
	kind uint8
	// fired, tgts: the Fireball casts the policy has started and the targets
	// it has picked on the live ask.
	fired, tgts int
	// sawPM, sawBoom, sawFB, tookLand, tookFB: the record loop's diagnostics.
	sawPM, sawBoom, sawFB, tookLand, tookFB int
}

func mbcFBFloatPolicyFor(seed uint64) Policy {
	return &mbcFBFloatPolicy{heur: HeuristicPolicy(seed)}
}

func (p *mbcFBFloatPolicy) Pick(c *PickCtx) Canon {
	if c.D.Kind == DPriority {
		if a, ok := p.steer(c); ok {
			return a
		}
	} else if p.kind == mbcFBCastBoomerang || p.kind == mbcFBCastFireball {
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
	plains, mines, restr := mbcFBFloatPool(c.G)
	islands, mountains, pms, basics := 0, 0, 0, 0
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
		o := c.G.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Island":
			islands++
			basics++
		case "Mountain":
			if o.Controller == 0 {
				mountains++
				basics++
			}
		case "Plains":
			basics++
		case "Abstract Paintmage":
			pms++
		}
	}
	var island, plain, mountain, mine Canon
	var haveIsland, havePlain, haveMountain, haveMine bool
	var pm, boom, fb Canon
	var havePM, haveBoom, haveFB bool
	for _, o := range c.D.Options {
		switch {
		case o.Kind() == ActPlayLand:
			switch n := optionFaceName(c, o); n {
			case "Island":
				island, haveIsland = o, true
			case "Plains":
				plain, havePlain = o, true
			case "Mountain":
				mountain, haveMountain = o, true
			case "Gemstone Mine":
				mine, haveMine = o, true
			}
		case o.Kind() == ActCast && optionFaceName(c, o) == "Abstract Paintmage":
			pm, havePM = o, true
			p.sawPM++
		case o.Kind() == ActCast && optionFaceName(c, o) == "Boomerang":
			boom, haveBoom = o, true
			p.sawBoom++
		case o.Kind() == ActCast && optionFaceName(c, o) == "Fireball":
			fb, haveFB = o, true
			p.sawFB++
		}
	}
	switch {
	case havePM && pms == 0 && islands >= 1 && mountains >= 2:
		// the float's maker: {U}{U/R}{R} over the Island and both Mountains
		p.kind = mbcFBCastPaintmage
		return pm, true
	case haveBoom && pms >= 1 && restr >= 1 && basics >= 1:
		// the printed-source eraser: the float's {U} pays the Boomerang's own
		// pip (its plan taps one basic land only), so each cast bounces a
		// basic land to hand untapped until no printed source is left
		p.kind = mbcFBCastBoomerang
		return boom, true
	case haveIsland && islands == 0 && pms == 0:
		// the Paintmage's {U}: one Island prints it, the Mountains do not
		return island, true
	case havePlain && plains < 2:
		// printed generic for the Boomerangs' plans to tap and bounce
		p.tookLand++
		return plain, true
	case haveMountain && mountains < 2:
		// the Paintmage's hybrid and {R} pips
		p.tookLand++
		return mountain, true
	case haveMine && mines < 2:
		// the fallback window's generic: the planner's census does not price
		// the Mine's choice-shaped rider, so the window is the only path to
		// it (mana_rider.go)
		return mine, true
	case haveFB && pms >= 1 && basics == 0 && mines >= 2 && restr >= 2 && p.fired < 1:
		// the cast must run through the fallback window: {2}{R} (X 0, three
		// targets) over the floating {U}{R} with no printed source untapped —
		// the shortened cost is one unit beyond it (the plan fails), the
		// fix's window settles it over a Mine, and the base's twice-shortened
		// settle reads one unit less and funds a payment gorge cannot
		p.kind = mbcFBCastFireball
		p.tgts = 0
		p.fired++
		p.tookFB++
		return fb, true
	}
	return Pack(ActPass, RefNone, RefNone, 0), true
}

func (p *mbcFBFloatPolicy) answer(c *PickCtx) (Canon, bool) {
	switch {
	case p.kind == mbcFBCastBoomerang && c.D.Kind == DTarget:
		// a basic land of ours: the bounce that erases the printed sources
		for _, o := range c.D.Options {
			if o.Kind() != ActTarget {
				continue
			}
			switch n := optionFaceName(c, o); n {
			case "Island", "Mountain", "Plains":
				if gopt, ok := c.G.GorgeOption(o); ok {
					if ob := c.G.E.G.Obj(gopt.Obj); ob != nil && ob.Controller == 0 {
						return o, true
					}
				}
			}
		}
	case p.kind == mbcFBCastFireball && c.D.Kind == DChoose:
		// the X ask: X announced as 0, so the cost the payment plans is {2}{R}
		// and the plain arm's fallback window runs (mana_window.go)
		for _, o := range c.D.Options {
			if o.Kind() == ActChoose && o.Mode() == 0 {
				return o, true
			}
		}
	case p.kind == mbcFBCastFireball && c.D.Kind == DTarget:
		// a target ask (gorge's Decision.AffordableTargets): the ask's hint
		// prices the taxed count over the census's sources only — the Mines'
		// riders are census-excluded (fail closed, mana_rider.go) — so the
		// hint stops at two targets while the cast's real window still pays
		// three ({2}{R} over the float and a Mine). The policy deliberately
		// overruns the hint (the raw policy, policy.go): the cost is payable,
		// so no CR 733.1 reversal follows; both engines pay through their
		// windows.
		if p.tgts < 3 {
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

// mbcFBFloatPool counts seat 0's untapped Plains (printed generic for the
// Boomerangs' plans), its Gemstone Mines and the restricted batches in its
// pool: the float the Fireball cast pays over is restricted, and the Mines are
// the units only the fallback window can take.
func mbcFBFloatPool(g *GorgeSide) (plains int, mines int, restr int32) {
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		o := g.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Plains":
			if !o.Tapped {
				plains++
			}
		case "Gemstone Mine":
			mines++
		}
	}
	for _, b := range g.E.G.Players[0].RestrictedMana {
		restr += b.Amount
	}
	return plains, mines, restr
}

// mbcFBFloatMarks is a gorge log's scenario marks: the record loop's
// diagnostics read them to see how far a game's steering got.
type mbcFBFloatMarks struct {
	pms    int // Paintmages that entered seat 0's battlefield
	booms  int // Boomerangs put on the stack
	fbs    int // Fireballs put on the stack
	tgts   int // Fireball target picks while floats lived
	floats int // restricted units floating when the Fireball's targets were chosen
}

// mbcFBFloatScan reads the finished gorge log for the scenario: a Fireball
// chose targets on a step whose pool held admitted restricted units (the
// mana-restriction ManaAdds of the Paintmage's floats, less their spends), and
// resolved — the fallback window funded it (under the base the payment came up
// one unit short instead).
func mbcFBFloatScan(g *GorgeSide) (mk mbcFBFloatMarks, reached bool) {
	var pm bool
	var batch [8]int32
	var fbStack state.ObjID
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
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil {
				switch o.Face().Name {
				case "Boomerang":
					mk.booms++
				case "Fireball":
					fbStack, fire = e.Obj, false
					mk.fbs++
				}
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
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Fireball" &&
				e.Obj == fbStack && mbcFBFloatFloating(batch) {
				mk.tgts++
				mk.floats = int(mbcFBFloatFloatingN(batch))
				fire = true
			}
		case events.SpellCastReversed:
			if e.Obj == fbStack {
				fbStack, fire = 0, false
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

// TestMBCFireballX0RestrFloatReprosLockstep replays the recorded game: no
// divergence, every scripted action offered — the CR 601.2g fallback window
// funds the three-target Fireball exactly once over the admitted floating
// batch.
func TestMBCFireballX0RestrFloatReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, mbcFBFloatRepro)
}

// TestMBCFireballX0RestrFloatRecord plays the policy over seeds until one
// reaches the float beside a resolved three-target Fireball cast, and writes
// the repro. It runs only with PINS_RECORD=1.
func TestMBCFireballX0RestrFloatRecord(t *testing.T) {
	if os.Getenv("PINS_RECORD") == "" {
		return // a recording tool, not coverage: it returns rather than skips (testdata/poolskips.txt)
	}
	reg := synthReg(t)
	set := printedDelveSet(t, mbcFBFloatDecks)
	for seed := uint64(1); seed <= 600; seed++ {
		r := &Repro{Format: reproFormat, Decks: [2]string{mbcFBFloatRepro + "_0", mbcFBFloatRepro + "_1"},
			Cards: mbcFBFloatDecks, Seed: seed, PassElision: true, Surface: "full",
			Kernel: "custom", Pool: "custom", Why: mbcFBFloatWhy}
		var mk mbcFBFloatMarks
		res, reached := mbcFBFloatPlay(t, reg, set, r, mbcFBFloatPolicyFor(seed), func(g *GorgeSide) {
			mk, _ = mbcFBFloatScan(g)
		})
		t.Logf("seed %d: reached=%v marks=%+v", seed, reached, mk)
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
	t.Fatalf("no seed reached a Paintmage float beside a resolved three-target Fireball cast")
}
