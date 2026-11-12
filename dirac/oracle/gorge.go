package oracle

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/spellbench/payexec"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// Config is a lockstep run's knobs that both sides must agree on.
type Config struct {
	// PassElision mirrors the kernel's elision (dirac.Config.NoElide off):
	// a canonical decision -- or multi-pick step -- with exactly one option
	// is answered in place, never posed (doc.go, "Elision"). For priority
	// that is the Pass-only window.
	PassElision bool
	// GorgeAutopaySurface drops the plain casts gorge reaches only by
	// lowering or scripting a potential play -- the spells its planner
	// never prices ({X}, an either-or additional cost, a target-conditional
	// reduction, one payable only with a granted mana ability) -- from the canonical priority list, and sets the kernel's
	// dirac.Config.GorgeAutopaySurface, which offers them only when the
	// floating pool pays: both sides then list the same casts.
	GorgeAutopaySurface bool
	// RecordChance sets the kernel's dirac.Config.RecordChance, so a
	// RealKernel/SetSide kernel reports its random outcomes through
	// ChanceTranscript (ChanceSource). Recording changes no game.
	RecordChance bool
	// ArrangeRestOrder sets the kernel's dirac.Config.ArrangeRestOrder and
	// mirrors it on the GorgeSide: a scry/surveil's pile B is ordered as a
	// second canonical phase instead of keeping the offered order. Both
	// sides must agree on it: the kernel's phase is part of the canonical
	// decision stream, so a side that left it off would pose a different
	// decision list. Off by default (gorge's legacy answer with no Rest).
	ArrangeRestOrder bool
}

// Stats counts what the gorge side did beneath the canonical surface.
type Stats struct {
	Decisions int // canonical decisions posed
	Commits   int // gorge intents submitted for canonical actions
	Elided    int // priority passes answered by PassElision
	PlanCasts int // casts paid by a PaymentAction plan (Intent.Payment)
	Lowered   int // plays paid by lowering a PotentialPaymentPlans witness
	Direct    int // plays submitted as offered (free, or the pool pays)
	Internal  int // gorge decisions answered beneath the surface (payment windows, colour asks, lowering steps)
	Fallbacks int // planned casts that fell back to the manual window
	Yields    int // foreign decisions a lowering handed back (answered minimally)
	Aborts    int // lowerings and scripts that aborted mid-way
	Scripted  int // plays paid after the planner's scripted prefix
	Refused   int // commits gorge refused (then repaired with Clamp)
	Overflow  int // decisions with more than MaxOptions canonical options
	// Unknown counts options with no canonical form, by "<decision>/<option kind>".
	Unknown map[string]int
	// Occurrence counts decisions where an action needed an occurrence index
	// to stay unique (doc.go, "Not 1:1"), by "<decision>/<option kind>".
	Occurrence map[string]int
	// Duplicate counts options dropped because their canonical action
	// collided even after occurrence indexing.
	Duplicate map[string]int
	// Unpriced counts potential plays gorge's planner could not price
	// (Reason unsupported/search_limit/ambiguous), by reason: legal plays
	// the canonical vocabulary leaves out.
	Unpriced map[string]int
	// UnpricedPlays is Unpriced by planner detail and play label.
	UnpricedPlays map[string]int
}

func (s *Stats) note(m *map[string]int, k string) {
	if *m == nil {
		*m = map[string]int{}
	}
	(*m)[k]++
}

// Add folds o into s.
func (s *Stats) Add(o Stats) {
	s.Decisions += o.Decisions
	s.Commits += o.Commits
	s.Elided += o.Elided
	s.PlanCasts += o.PlanCasts
	s.Lowered += o.Lowered
	s.Direct += o.Direct
	s.Internal += o.Internal
	s.Fallbacks += o.Fallbacks
	s.Yields += o.Yields
	s.Aborts += o.Aborts
	s.Scripted += o.Scripted
	s.Refused += o.Refused
	s.Overflow += o.Overflow
	for _, p := range []struct {
		dst *map[string]int
		src map[string]int
	}{
		{&s.Unknown, o.Unknown}, {&s.Occurrence, o.Occurrence}, {&s.Duplicate, o.Duplicate}, {&s.Unpriced, o.Unpriced},
		{&s.UnpricedPlays, o.UnpricedPlays},
	} {
		for k, v := range p.src {
			if *p.dst == nil {
				*p.dst = map[string]int{}
			}
			(*p.dst)[k] += v
		}
	}
}

// GorgeSide drives a gorge engine through the canonical surface: Step poses
// canonical decisions, Apply executes canonical actions. It is the oracle
// side of a lockstep run, and (GorgeKernel) a Kernel itself.
type GorgeSide struct {
	// CharErr is the last projection's mapping error (a gorge keyword
	// with no kernel bit, projchars.go); "" when none.
	CharErr string

	E     *rules.Engine
	M     *Matchup
	Cfg   Config
	Stats Stats
	// OnInternal, when set, observes each ask the side answers itself (a
	// payment window or colour ask) with the option index it chose; a
	// test-only observation (vs_test.go), like the lockstep Check hook.
	OnInternal func(d *decision.Decision, idx int)

	id        identity
	fr        *frame
	hyp       bool // a hypothetical engine (forced chance outcomes): submit through SubmitHypothetical
	committed bool // the last Apply submitted to gorge
	dirty     bool // gorge was answered since the last Project
	err       error
	// castCost caches the resolved hand-only PaymentCost of plain cast
	// options seen at priority, so a convoke announcement (which carries
	// no cost) can be projected against the same cost the offer used.
	castCost map[state.ObjID]decision.PaymentCost
	// castX marks the cached plain casts whose face mana cost has an {X}
	// (gorge's ConvokeOffer.HasX): PaymentCost drops X, and the convoke
	// announcement is posed before X is announced.
	castX map[state.ObjID]bool
	// manaUses are the mana abilities the engine resolved since
	// ArmCoverage (coverage.go); nil until armed.
	manaUses  []manaUse
	manaArmed bool
	// payPips are the hybrid/Phyrexian pip faces the payer announced for the
	// cast/activation in flight, by colour (W U B R G C). A payment window a
	// plan fell back to carries no ManaPayment, so the owed colour of an
	// any-colour mana ability's colour ask is this demand less the pool.
	payPips [6]uint8
}

// frame is the canonical view of one posed gorge decision.
type frame struct {
	seq   uint64
	d     *decision.Decision // a clone: the engine's may be arena storage
	dk    DKind
	multi bool
	acts  []Canon // per d.Options index (multi/single kinds)
	valid []bool
	prio  []prioEntry // priority decisions
	// chosen is the multi-pick answer so far, in pick order.
	chosen []int
	// restable: this ask's pile B may carry an order (Config.ArrangeRestOrder
	// and gorge's Decision.Restable). resting: the top phase is closed and
	// the rest cards are being ordered; rest holds their native option
	// indices in pick order, and restN is how many there are.
	restable bool
	resting  bool
	restN    int
	rest     []int
	cur      CDecision
	// curAt maps cur.Options[i] to a d.Options index, a prio index (priority)
	// or -1 (Done).
	curAt []int
	// convoke is the outstanding cost of a convoke announcement.
	convoke *convokeFrame
	// unitDedupe: a Combo mana allocation (manaChoiceResolving), whose
	// options repeat the colours once per unit; each colour is offered once.
	unitDedupe bool
}

// manaChoiceResolving reports whether d is the colour choice of a resolving
// Mana effect (a loyalty ability, Chandra, Hope's Beacon's [+2]; a spell or a
// triggered/sub ability, Realm-Scorcher Hellkite; CR 106.3, 605.1a): not the
// colour ask of a mana ability activated while paying (Kind "AB", no
// Planeswalker param), which the harness answers itself. An AB$ Mana that is
// no mana ability (its chain draws: Chromatic Sphere, the eggs) resolves on
// the stack, so its colour ask is a resolving one.
func manaChoiceResolving(d *decision.Decision) bool {
	return d.Kind == decision.KChoose && d.ResumeKind == "mana_color" && d.ResumeSA != nil &&
		(d.ResumeSA.Kind != "AB" || strings.TrimSpace(d.ResumeSA.Params["Planeswalker"]) == "True" ||
			!cards.IsManaAbilitySA(d.ResumeSA))
}

// convokeFrame is the absorption budget of a convoke announcement: one
// generic slot plus one slot per coloured pip (gorge manaToPay).
type convokeFrame struct {
	gen  uint32
	pips [6]uint32
	// open: the spell's {X} is not announced yet (the cast's face cost has
	// an X), so a generic pick against {0} is tentatively legal
	// (gorge convokeAbsorbs, xOpen) and pays nothing until X is folded.
	open bool
	// gate, when set, replaces the budget: a cast with no cached hand price
	// (a may-play, alternative-cost or kicked route) is filtered by gorge's
	// own Submit gate over its pending cast (rules.CastContributionsAdmitted),
	// so the walk never builds an over-selection gorge refuses.
	gate func(choices []int) bool
}

func (c *convokeFrame) admits(d *decision.Decision, chosen []int, i int) bool {
	if c.gate != nil {
		return c.gate(append(slices.Clip(chosen), d.Options[i].Index))
	}
	gen, pips := c.gen, c.pips
	apply := func(idx int) bool {
		o := optByIndex(d, idx)
		if !strings.HasPrefix(o.Kind, "convoke_") && o.Kind != "improvise_generic" {
			return true
		}
		if o.Kind == "convoke_generic" || o.Kind == "improvise_generic" {
			if gen == 0 {
				return c.open
			}
			gen--
			return true
		}
		col := strings.Index(manaSyms, strings.TrimPrefix(o.Kind, "convoke_"))
		if col < 0 || col >= len(pips) || pips[col] == 0 {
			return false
		}
		pips[col]--
		return true
	}
	for _, idx := range chosen {
		if !apply(idx) {
			return false
		}
	}
	return apply(i)
}

type prioKind uint8

const (
	prOption prioKind = iota // submit d.Options[opt] as offered
	prPlan                   // Intent.Payment with pay.Plans[0]
	prLower                  // lower plan, then pick the play's option
	prScript                 // play the planner's scripted prefix, then pay the play as priced then
)

type prioEntry struct {
	a       Canon
	kind    prioKind
	opt     int
	offered bool // a prPlan that replaced an offered option: opt is that option (v2bridge.go)
	pay     decision.PaymentAction
	play    payexec.Play
	plan    decision.PaymentPlan
	script  []rules.ScriptStep
}

// NewGorgeSide starts gorge on m with seed, exactly as gorge self-play does
// (Matchup.Config, rules.New, Advance). chance, when non-empty, forces the
// engine's random outcomes (rules.NewHypothetical; Replay's forced outcomes).
func NewGorgeSide(m *Matchup, seed uint64, cfg Config, chance []rules.ChanceDraw) (*GorgeSide, error) {
	g := &GorgeSide{M: m, Cfg: cfg}
	g.id.setLayout(m)
	rc := m.Config(seed)
	if len(chance) > 0 {
		e, err := rules.NewHypothetical(rc, chance)
		if err != nil {
			return nil, err
		}
		g.E, g.hyp = e, true
		if err := e.AdvanceHypothetical(); err != nil {
			return nil, err
		}
	} else {
		g.E = rules.New(rc)
		g.E.Advance()
	}
	g.id.sync(g.E.G)
	return g, nil
}

// Err is the first internal failure (a refused lowering, an engine panic
// recovered by the driver); the side then reports DNone.
func (g *GorgeSide) Err() error { return g.err }

// Committed reports whether the last Apply submitted an answer to gorge (as
// opposed to adding a pick to a multi-pick decision).
func (g *GorgeSide) Committed() bool { return g.committed }

// Dirty reports whether gorge has been answered since the last Project: the
// state moved, so the two sides' projections are due for comparison.
func (g *GorgeSide) Dirty() bool { return g.dirty }

// Turn implements Side.
func (g *GorgeSide) Turn() int { return int(g.E.G.Turn) }

// Pending is gorge's pending decision behind the current canonical one (nil
// when none); valid until the next Apply.
func (g *GorgeSide) Pending() *decision.Decision {
	if g.fr == nil {
		return nil
	}
	return g.fr.d
}

// Terminal implements Side.
func (g *GorgeSide) Terminal() (bool, int8) {
	if !g.E.G.Over {
		return false, -1
	}
	if g.E.G.Draw {
		return true, -1
	}
	return true, int8(g.E.G.Winner)
}

// Decided is gorge's game result once a player has lost, before G.Over
// too: gorge can still pose a decision after the loss (a legend-rule ask
// parked as the loser's objects leave, pair 1366; CR 104.2a, k5-queue).
func (g *GorgeSide) Decided() (bool, int8) {
	if g.E.G.Over {
		return g.Terminal()
	}
	lost, winner := 0, int8(-1)
	for i := range g.E.G.Players {
		if g.E.G.Players[i].Lost {
			lost++
		} else {
			winner = int8(i)
		}
	}
	switch {
	case lost == 0:
		return false, -1
	case lost == len(g.E.G.Players):
		return true, -1
	}
	return true, winner
}

// Project implements Side.
func (g *GorgeSide) Project(p *Projection) {
	g.dirty = false
	g.CharErr = project(g.E, &g.id, g.M, p)
}

// InstRef is the canonical reference of a gorge object (reports, policies).
func (g *GorgeSide) InstRef(id state.ObjID) Ref { return g.id.ref(g.E.G, id) }

func (g *GorgeSide) submit(in decision.Intent) error {
	g.dirty = true
	if g.hyp {
		return g.E.SubmitHypothetical(in)
	}
	return g.E.Submit(in)
}

func (g *GorgeSide) fail(err error) {
	if g.err == nil {
		g.err = err
	}
}

// Step implements Side: it answers every gorge decision beneath the
// canonical surface (payment windows, elided passes) and poses the next
// canonical one, or DNone at the end of the game.
func (g *GorgeSide) Step() CDecision {
	for guard := 0; ; guard++ {
		if g.err != nil || g.E.G.Over || guard > 10000 {
			if guard > 10000 {
				g.fail(fmt.Errorf("oracle: 10000 internal answers without a canonical decision"))
			}
			return CDecision{Kind: DNone}
		}
		d := g.E.Pending()
		if d == nil {
			g.fail(fmt.Errorf("oracle: no pending decision in a live game"))
			return CDecision{Kind: DNone}
		}
		fr := g.fr
		if fr == nil || fr.seq != d.Seq {
			g.fr = nil
			if g.internal(d) {
				continue
			}
			fr = g.build(d)
			if len(fr.cur.Options) == 0 {
				g.fail(fmt.Errorf("oracle: uncanonicalisable %s decision (%d gorge options, none with a canonical form)", d.Kind, len(d.Options)))
				return CDecision{Kind: DNone}
			}
			g.fr = fr
		}
		if g.Cfg.PassElision && len(fr.cur.Options) == 1 {
			// The kernel's single-option elision: a decision (or pick) with
			// one option is answered in place, never posed.
			g.Stats.Elided++
			g.Apply(fr.cur.Options[0])
			if g.err != nil {
				return CDecision{Kind: DNone}
			}
			continue
		}
		g.Stats.Decisions++
		return fr.cur
	}
}

// internal answers d when it is not a canonical decision: a payment window
// (the manual CR 601.2g window a planned cast fell back to) or a colour ask
// of a mana ability activated while paying.
func (g *GorgeSide) internal(d *decision.Decision) bool {
	if d.Kind != decision.KChoose || len(d.Options) == 0 {
		return false
	}
	if g.selfDiscardCost(d) {
		g.Stats.Internal++
		if err := g.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			g.fail(fmt.Errorf("oracle: self-discard cost answer refused: %w", err))
		}
		return true
	}
	if forcedPlayerChoice(d) {
		g.Stats.Internal++
		if err := g.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			g.fail(fmt.Errorf("oracle: forced player choice refused: %w", err))
		}
		return true
	}
	if manaChoiceResolving(d) {
		return false
	}
	window := d.ManaPayment != nil || d.PaymentFallback != nil
	allMana := true
	for _, o := range d.Options {
		switch o.Kind {
		case "activate", decision.OptAutoFill, decision.OptUndoTap, decision.OptCancelCast:
			window = true
		case "done":
		default:
			if o.Kind != "mana" {
				allMana = false
			}
		}
	}
	if !window && !allMana {
		return false
	}
	g.Stats.Internal++
	if d.PaymentFallback != nil {
		g.Stats.Fallbacks++
	}
	picks := payexec.WindowChoices(d, g.windowOwed(d))
	if g.OnInternal != nil {
		g.OnInternal(d, picks[0])
	}
	if err := g.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}); err != nil {
		g.fail(fmt.Errorf("oracle: payment window answer refused: %w", err))
	}
	return true
}

// windowOwed is the mana demand payWindowAnswer matches against. A payment
// window carries it in ManaPayment; a standalone colour ask of an any-colour
// mana ability in a fallback window does not, so derive it from the pips the
// payer announced for the cast/activation in flight, less the floating pool.
func (g *GorgeSide) windowOwed(d *decision.Decision) decision.ManaAmount {
	var owed decision.ManaAmount
	if d.ManaPayment != nil {
		owed = d.ManaPayment.Owed.Mana
		return owed
	}
	if len(d.Options) == 0 || !hasManaOption(d) {
		return owed
	}
	pool := g.E.G.Players[d.Player].Pool
	for c := 0; c < len(owed); c++ {
		if n := int32(g.payPips[c]) - pool[c]; n > 0 {
			owed[c] = uint32(n)
		}
	}
	return owed
}

// hasManaOption reports whether d poses a colour ask (its options name mana
// colours). A manual payment window lists sources as "activate" instead.
func hasManaOption(d *decision.Decision) bool {
	for _, o := range d.Options {
		if o.Kind == "mana" {
			return true
		}
	}
	return false
}

// castHasX reports whether o's face mana cost has an {X} (the plain cast's
// HasX in gorge's convoke offer).
func castHasX(o *state.Object) bool {
	return o != nil && o.Face() != nil && pay.FaceCostOf(o.Face()).X > 0
}

// convokeFrameFor is the absorption frame of a convoke announcement d, or
// nil when d is not one. An uncached cast cost is gated by gorge itself.
func (g *GorgeSide) convokeFrameFor(d *decision.Decision) *convokeFrame {
	if d.Kind != decision.KChoose || len(d.Options) == 0 || !(strings.HasPrefix(d.Options[0].Kind, "convoke_") || d.Options[0].Kind == "improvise_generic") {
		return nil
	}
	pc, ok := g.castCost[d.Source]
	if !ok {
		e, gd := g.E, d.Clone()
		return &convokeFrame{gate: func(choices []int) bool { return rules.CastContributionsAdmitted(e, gd, choices) }}
	}
	// open is the cast's unannounced {X} itself: Max == #options also
	// holds for an X-less menu no larger than its slot count, where gorge
	// refuses a generic pick once the generic is used (convokeAbsorbs).
	return &convokeFrame{gen: pc.Generic, pips: pc.Mana, open: g.castX[d.Source]}
}

// build makes the canonical frame of gorge decision d.
func (g *GorgeSide) build(d0 *decision.Decision) *frame {
	g.id.sync(g.E.G)
	d := d0.Clone()
	fr := &frame{seq: d.Seq, d: d, dk: decisionKinds[d.Kind]}
	fr.convoke = g.convokeFrameFor(d)
	if d.Kind == decision.KPriority {
		g.priorityVocabulary(fr)
		fr.reposeSingle()
		return fr
	}
	fr.acts = make([]Canon, len(d.Options))
	fr.valid = make([]bool, len(d.Options))
	for i := range d.Options {
		if strings.HasSuffix(d.Options[i].Kind, "_manual") {
			// The manual-sources route of a resolution-time payment
			// (rules.resPayOptions): autopay hides it as it hides mana taps.
			continue
		}
		a, ok := g.optionAction(d, &d.Options[i])
		if !ok {
			g.Stats.note(&g.Stats.Unknown, string(d.Kind)+"/"+d.Options[i].Kind)
			continue
		}
		fr.acts[i], fr.valid[i] = a, true
	}
	occ, dup := disambiguate(fr.acts, fr.valid)
	if occ || dup {
		key := string(d.Kind)
		if len(d.Options) > 0 {
			key += "/" + d.Options[0].Kind
		}
		if occ {
			g.Stats.note(&g.Stats.Occurrence, key)
		}
		if dup {
			g.Stats.note(&g.Stats.Duplicate, key)
		}
	}
	fr.multi = !(d.Min == 1 && d.Max == 1) || d.AllowNone // an AllowNone ask has the empty answer, so it is closed by Done (Teamwork's tap pick)
	// A scry/surveil's pile B is ordered by a second canonical phase when
	// the kernel's Config.ArrangeRestOrder is on: gorge's Decision.Restable
	// marks the asks whose pile B can be non-empty and ordered.
	fr.restable = g.Cfg.ArrangeRestOrder && d.Kind == decision.KArrange && d.Restable
	if manaChoiceResolving(d) && d.Max > 1 {
		// a Combo's allocation: gorge offers every colour once per unit, the
		// kernel one pick per unit over the colours. Every option stays valid
		// (disambiguate would drop the later units' copies as repeats) and
		// repose offers each colour once, at its first unchosen unit.
		for i := range d.Options {
			a, ok := g.optionAction(d, &d.Options[i])
			fr.acts[i], fr.valid[i] = a, ok
		}
		fr.unitDedupe = true
	}
	g.repose(fr)
	return fr
}

// priorityVocabulary is the autopay priority vocabulary (doc.go): pass,
// every land play, every cast gorge's planner pays (PaymentActions), every
// potential play PotentialPaymentPlans prices, and every cast/ability/special
// option offered as is. Mana abilities and concede are never actions.
func (g *GorgeSide) priorityVocabulary(fr *frame) {
	d := fr.d
	e := g.E
	// The cast cost a convoke frame filters by is the CURRENT decision's
	// plain hand offers. Dropping last decision's entries keeps a card cast
	// from hand earlier and later cast through a permission (or a
	// flashback) from filtering its announcement by a stale hand price.
	g.castCost, g.castX = nil, nil
	ref := func(id state.ObjID) Ref { return g.id.ref(e.G, id) }
	add := func(pe prioEntry) {
		for _, x := range fr.prio {
			if x.a == pe.a {
				return
			}
		}
		fr.prio = append(fr.prio, pe)
	}
	note := func(k string) { g.Stats.note(&g.Stats.Unknown, "priority/"+k) }
	// Offered options first: an offered play needs no lowering.
	for i := range d.Options {
		o := &d.Options[i]
		switch o.Kind {
		case "pass":
			add(prioEntry{a: Pack(ActPass, RefNone, RefNone, 0), opt: o.Index})
		case "play_land":
			add(prioEntry{a: Pack(ActPlayLand, ref(o.Obj), RefNone, landFaceCode(o.Mode)), opt: o.Index})
		case "cast":
			// A may-play cast (an untyped permission's plain cast of a card in
			// exile or the graveyard) prices beside the plain hand offers: its
			// convoke announcement must filter by the same cost beginCast
			// charges, or the oracle picks contributions the announcement
			// cannot absorb and gorge's Submit refuses (CR 702.51b: "up to two
			// green creatures and up to three other creatures"). May-play
			// casts at an alternative (AltCostIndex > 0) keep the older
			// unfiltered behaviour.
			if o.Mode == "mayplay" && o.AltCostIndex == 0 {
				if pc, ok := e.MayPlayCastPaymentCost(d.Player, o.Obj, o.MayPlayPerm); ok {
					if g.castCost == nil {
						g.castCost = map[state.ObjID]decision.PaymentCost{}
						g.castX = map[state.ObjID]bool{}
					}
					g.castCost[o.Obj] = pc
					g.castX[o.Obj] = castHasX(e.G.Obj(o.Obj))
				}
			}
			if o.Mode == "" && o.AltCostIndex == 0 {
				if pc, ok := e.CastPaymentCost(d.Player, o.Obj); ok {
					if g.castCost == nil {
						g.castCost = map[state.ObjID]decision.PaymentCost{}
						g.castX = map[state.ObjID]bool{}
					}
					g.castCost[o.Obj] = pc
					g.castX[o.Obj] = castHasX(e.G.Obj(o.Obj))
				}
			}
			if fc, ok := faceCastCode(o.Mode); ok && o.AltCostIndex == 0 {
				// a face cast (faces.go): offered only when the pool pays it,
				// since gorge's planner prices none (EnsurePaymentActions
				// plans no Mode, PotentialPaymentPlans calls it "shape")
				add(prioEntry{a: Pack(ActCastFace, ref(o.Obj), RefNone, fc), opt: o.Index})
				continue
			}
			if o.Mode == "casualty" {
				// CR 702.153a: the casualty cast is its own action kind, not a
				// cast-mode code (the 16-entry table is full; kw_casualty.go).
				if o.AltCostIndex > 15 {
					note("cast/casualty")
					continue
				}
				add(prioEntry{a: Pack(ActCastCasualty, ref(o.Obj), RefNone, uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "airbend_cast" && o.AltCostIndex == 0 {
				// CR 701.65a: an airbent card cast from exile is its own action
				// kind (act_airbend.go)
				add(prioEntry{a: Pack(ActCastAirbend, ref(o.Obj), RefNone, 0), opt: o.Index})
				continue
			}
			if o.Mode == "impended" && o.AltCostIndex <= 15 {
				// CR 702.176a: the impending cast is its own action kind (the
				// 16-entry cast-mode table is full; kw_impending.go)
				add(prioEntry{a: Pack(ActCastImpended, ref(o.Obj), RefNone, uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "harmonize" {
				// CR 702.180a: the 16-entry cast-mode table is full; the
				// kernel's mode-2 cast of a Harmonize card canonicalises to
				// castModeOther, like "bargained" (kw_harmonize.go)
				if o.AltCostIndex > 15 {
					note("cast/harmonize")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "beam_me_up" {
				// the kernel's mode-2 cast of a beam me up card canonicalises to
				// castModeOther, like "harmonize" (kw_beammeup.go)
				alt := beamAlt(o.Mode, o.AltCostIndex, o.Cost)
				if alt > 15 {
					note("cast/beam_me_up")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(alt)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "retrace" {
				// CR 702.81a: the kernel's retrace cast (the printed cost
				// plus the discard-a-land additional cost) canonicalises to
				// castModeOther, like "harmonize" (kw_retrace.go)
				if o.AltCostIndex > 15 {
					note("cast/retrace")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "mayhem" {
				// CR 702.187a: the kernel's mode-2 cast of a Mayhem card
				// canonicalises to castModeOther, like "harmonize"
				// (kw_mayhem.go)
				if o.AltCostIndex > 15 {
					note("cast/mayhem")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "mayflash" {
				// CR 702.8: the kernel's AkMayFlash canonicalises to
				// castModeOther, like "web-slinging" (kw_mayflash.go)
				if o.AltCostIndex > 15 {
					note("cast/mayflash")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "web-slinging" {
				// CR 702.188a: the kernel's AkWebSling canonicalises to
				// castModeOther, like "harmonize" (kw_websling.go)
				if o.AltCostIndex > 15 {
					note("cast/web-slinging")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "optionalcost" {
				// CR 601.2b: an OptionalCost static's view, AltCostIndex
				// i+1; the kernel's kicked cast of an OptCost card packs
				// castModeOther|1<<4 (cost_optional.go)
				if o.AltCostIndex > 15 {
					note("cast/optionalcost")
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			if o.Mode == "conspired" && o.AltCostIndex == 0 {
				// the conspired cast keeps its own canonical form (a granted
				// conspire pairs with a bargain/teamwork cast of the same card)
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|15<<4), opt: o.Index})
				continue
			}
			if o.Mode == "bargained" || o.Mode == "teamworked" {
				// CR 702.166: the 16-entry cast-mode table is full, so the
				// "bargained" mode canonicalises to castModeOther, exactly as
				// realkernel's AkCastCasualty mapping does
				// (kw_bargain.go); so does CR 702.194's "teamworked"
				// (kw_teamwork.go)
				if o.AltCostIndex > 15 {
					note("cast/" + o.Mode)
					continue
				}
				add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, castModeOther|uint8(o.AltCostIndex)<<4), opt: o.Index})
				continue
			}
			mc, ok := castModeCode(o.Mode)
			if !ok || o.AltCostIndex > 15 {
				note("cast/" + o.Mode)
				continue
			}
			add(prioEntry{a: Pack(ActCast, ref(o.Obj), RefNone, mc|uint8(o.AltCostIndex)<<4), opt: o.Index})
		case "granted":
			// CR 702.179e: a max-speed static's AddAbility$ the controller
			// can activate at speed 4 (rules.maxSpeedAbilities); the kernel's
			// gated ability, canonical code maxSpeedBase+n
			if n, ok := g.maxSpeedOrdinal(o.Obj, o.SVar); ok {
				add(prioEntry{a: Pack(ActActivate, ref(o.Obj), RefNone, maxSpeedBase+uint8(n)), opt: o.Index})
				continue
			}
			note("granted/" + o.SVar)
		case "ability":
			if o.SVar != "" && o.GrantSource != 0 && o.Keyword == "" && o.GainedSource == 0 && o.AltCostIndex == 0 {
				// An ability another permanent grants (an Equipment's: Fishing
				// Pole's), offered when the floating pool pays it: the kernel's
				// latent-ability activation (ability code 255). Its grantor tells
				// two granted abilities on one permanent apart; a second copy of a
				// grantor gives the same ability, which stays the first's option.
				grantor := o.GrantSource
				for j := range d.Options[:i] {
					if x := &d.Options[j]; x.Kind == "ability" && x.Obj == o.Obj && x.SVar == o.SVar && x.GrantSource != 0 {
						grantor = x.GrantSource
						break
					}
				}
				add(prioEntry{a: Pack(ActActivate, ref(o.Obj), ref(grantor), 255), opt: o.Index})
				continue
			}
			if o.Keyword != "" && o.SVar == "" && o.GrantSource == 0 && o.GainedSource == 0 && o.AltCostIndex == 0 && g.selfKeywordGrant(o.Obj, o.Keyword) {
				// A line an effect on the permanent itself grants it while
				// the permanent keeps its abilities (Domesticated
				// Watercourse's Animate: "becomes an Equipment with equip
				// {2}"): the latent-ability activation, the object as its
				// own grantor. A static Equip grant has another source and
				// keeps the anchored note below.
				add(prioEntry{a: Pack(ActActivate, ref(o.Obj), ref(o.Obj), 255), opt: o.Index})
				continue
			}
			if o.SVar == "" && o.Keyword == "" && o.GrantSource == 0 && o.GainedSource == 0 && o.AltCostIndex == 0 && g.copyGrantedAbility(o.Obj, o.Ability) {
				// An as-enters copy's AddAbilities$ (Mercurial Pretender, and a
				// copy of it): a copiable value gorge lists among the face's own
				// abilities, the kernel's latent-ability activation with the
				// object as its own grantor.
				add(prioEntry{a: Pack(ActActivate, ref(o.Obj), ref(o.Obj), 255), opt: o.Index})
				continue
			}
			if o.SVar != "" || o.Keyword != "" || o.GainedSource != 0 || o.AltCostIndex != 0 || o.Ability > 0x7F {
				note("ability/anchored")
				continue
			}
			add(prioEntry{a: Pack(ActActivate, ref(o.Obj), RefNone, uint8(o.Ability)), opt: o.Index})
		case "activate", "concede":
		default:
			if c, ok := specialCode(o.Kind); ok {
				sub := turnUpModeCode(o.Mode)
				if o.Kind == "unlock" {
					sub = unlockModeCode(o.Mode)
				}
				add(prioEntry{a: Pack(ActSpecial, ref(o.Obj), RefNone, c|sub<<4), opt: o.Index})
			} else {
				note(o.Kind)
			}
		}
	}
	// Planner-paid casts.
	pays := e.EnsurePaymentActions()
	for _, pa := range pays {
		if len(pa.Plans) == 0 {
			continue
		}
		var a Canon
		if pa.Cast.CastMode() == "casualty" {
			a = Pack(ActCastCasualty, ref(pa.Cast.Object), RefNone, 0)
		} else if pa.Cast.CastMode() == "impended" {
			a = Pack(ActCastImpended, ref(pa.Cast.Object), RefNone, 0)
		} else if pa.Cast.CastMode() == "conspired" {
			a = Pack(ActCast, ref(pa.Cast.Object), RefNone, castModeOther|15<<4)
		} else if pa.Cast.CastMode() == "bargained" || pa.Cast.CastMode() == "teamworked" || pa.Cast.CastMode() == "harmonize" || pa.Cast.CastMode() == "mayhem" || pa.Cast.CastMode() == "optionalcost" || pa.Cast.CastMode() == "web-slinging" {
			// CR 702.166: the full cast-mode table's castModeOther code, the
			// same canonical form the offered option uses
			a = Pack(ActCast, ref(pa.Cast.Object), RefNone, castModeOther)
		} else {
			mc, ok := castModeCode(pa.Cast.CastMode())
			if !ok {
				note("payment/" + pa.Cast.Origin)
				continue
			}
			a = Pack(ActCast, ref(pa.Cast.Object), RefNone, mc)
		}
		replaced := false
		for k := range fr.prio {
			if fr.prio[k].a == a {
				// The pool already pays the offered cast; gorge's planned
				// route is still the autopay route (it spends the pool first).
				fr.prio[k] = prioEntry{a: a, kind: prPlan, pay: decision.ClonePaymentAction(pa), opt: fr.prio[k].opt, offered: fr.prio[k].kind == prOption}
				replaced = true
			}
		}
		if !replaced {
			add(prioEntry{a: a, kind: prPlan, pay: decision.ClonePaymentAction(pa)})
		}
	}
	// Potential plays the pool cannot pay yet: lower the planner's witness.
	for _, pp := range e.PotentialPaymentPlans(d.Player) {
		pa := pp.Action
		var a Canon
		switch pa.Kind {
		case "cast":
			// The planner prices the face routes the kernel offers from its
			// mana census: map those through the face table the
			// offered options use, or the witness is dropped and the kernel
			// is compared against a vocabulary gorge does not have. A Room's
			// other door follows the same route (followedFaceRoute includes
			// faceCodeRoomAlt now that the kernel prices it).
			if fc, ok := faceCastCode(pa.Mode); ok && followedFaceRoute(fc) {
				a = Pack(ActCastFace, ref(pa.Obj), RefNone, fc)
			} else if pa.Mode == "impended" {
				a = Pack(ActCastImpended, ref(pa.Obj), RefNone, 0) // kw_impending.go
			} else if mc, ok := castModeCode(pa.Mode); ok {
				a = Pack(ActCast, ref(pa.Obj), RefNone, mc)
			} else {
				continue
			}
		case "ability":
			if pa.Ability > 0x7F {
				continue
			}
			a = Pack(ActActivate, ref(pa.Obj), RefNone, uint8(pa.Ability))
			if g.copyGrantedAbility(pa.Obj, pa.Ability) {
				a = Pack(ActActivate, ref(pa.Obj), ref(pa.Obj), 255) // as the offer above
			}
		default:
			continue
		}
		have := false
		for _, x := range fr.prio {
			if x.a == a {
				have = true
			}
			if pa.Kind == "cast" && pa.Mode == "mayplay" && strings.Contains(pa.Label, "(alternative cost") &&
				x.a.Kind() == ActCast && x.a.Src() == a.Src() && x.a.Mode()&0xf == a.Mode() && x.a.Mode()>>4 != 0 {
				// the witness of a may-play alternative cast the walk already
				// offers: the planner's action carries no AltCostIndex, so it
				// reads as the plain may-play cast beside the offered alternative
				have = true
			}
		}
		if have {
			continue
		}
		if pp.Reason != "" {
			if pp.Reason != "insufficient" {
				g.Stats.note(&g.Stats.Unpriced, pa.Kind+"/"+pp.Reason)
				g.Stats.note(&g.Stats.UnpricedPlays, pp.Reason+" "+pp.Detail+": "+pa.Label)
			}
			continue
		}
		if g.Cfg.GorgeAutopaySurface && pa.Kind == "cast" && a.Kind() == ActCast && a.Mode() == 0 {
			// A plain cast the planner does not price, priced or scripted
			// (Tolarian Terror payable only with Imprisoned in the Moon's
			// granted {C}): the autopay vocabulary (azmcts
			// paymentVocabulary: the payment actions with a plan, then
			// d.Options) has neither.
			continue
		}
		if len(pp.Script) > 0 {
			// A source the planner's census cannot price (restricted mana,
			// Giada's Angel-only {W}): the planner found the activations
			// that make the play priceable; play them, then pay as priced.
			// Offered only when that completes on a clone: after some
			// scripts the planner refuses the floating pool ("unsupported
			// pool") and the play would be stranded half paid.
			pe := prioEntry{a: a, kind: prScript, script: pp.Script}
			if !g.scriptCompletes(pe) {
				g.Stats.note(&g.Stats.Unpriced, pa.Kind+"/script_dead_end")
				g.Stats.note(&g.Stats.UnpricedPlays, "script dead end: "+pa.Label)
				continue
			}
			add(pe)
			continue
		}
		if pp.Plan == nil {
			g.Stats.note(&g.Stats.Unpriced, pa.Kind+"/noplan")
			g.Stats.note(&g.Stats.UnpricedPlays, "no plan: "+pa.Label)
			continue
		}
		pe := prioEntry{a: a, kind: prLower, play: payexec.Play{Kind: pa.Kind, Obj: pa.Obj, Ability: pa.Ability, Mode: pa.Mode},
			plan: decision.ClonePaymentPlan(*pp.Plan)}
		if (pa.Kind == "ability" || a.Kind() == ActCastFace) && !g.lowerCompletes(pe) {
			// the witness funds only the mana half of a Phyrexian
			// activation (pay.PlanManaHalf), and gorge then refuses the
			// play its two-life faces cannot pay (cast_not_offered): the
			// lowering would leave the sources tapped and the play unmade,
			// so the play is not one gorge makes
			g.Stats.note(&g.Stats.Unpriced, pa.Kind+"/lower_dead_end")
			g.Stats.note(&g.Stats.UnpricedPlays, "lower dead end: "+pa.Label)
			continue
		}
		add(pe)
	}
	sort.Slice(fr.prio, func(i, j int) bool { return fr.prio[i].a < fr.prio[j].a })
}

// reposeSingle poses a priority frame: every entry once.
func (fr *frame) reposeSingle() {
	fr.cur = CDecision{Kind: fr.dk, Player: uint8(fr.d.Player)}
	fr.curAt = fr.curAt[:0]
	for i, pe := range fr.prio {
		fr.cur.Options = append(fr.cur.Options, pe.a)
		fr.curAt = append(fr.curAt, i)
	}
}

// repose poses the frame's next pick: every admissible option not yet
// chosen after which the answer can still be completed into a valid one,
// plus Done once the answer so far is valid (doc.go, "Multi-pick"). In a
// KArrange rest phase the remaining cards are offered the same way and no
// Done is appended: the last rest card is implied.
func (g *GorgeSide) repose(fr *frame) {
	d := fr.d
	type ent struct {
		a  Canon
		at int
	}
	var es []ent
	if fr.resting {
		// Pile B's order phase: every card not in pile A and not yet
		// ordered, with no Done. The kernel's last card is implied, so it
		// poses r-1 picks for r cards; so does this side.
		for i := range d.Options {
			idx := d.Options[i].Index
			if !fr.valid[i] || slices.Contains(fr.chosen, idx) || slices.Contains(fr.rest, idx) {
				continue
			}
			es = append(es, ent{fr.acts[i], idx})
		}
		sort.Slice(es, func(i, j int) bool { return es[i].a < es[j].a })
		fr.cur = CDecision{Kind: fr.dk, Player: uint8(d.Player), Options: make([]Canon, 0, len(es))}
		fr.curAt = fr.curAt[:0]
		for _, x := range es {
			fr.cur.Options = append(fr.cur.Options, x.a)
			fr.curAt = append(fr.curAt, x.at)
		}
		if len(es) > MaxOptions {
			g.Stats.Overflow++
		}
		return
	}
	budget := completeBudget
	playSeen := map[state.ObjID]bool{} // playRepeatDup's cards already offered
	for i := range d.Options {
		if !fr.valid[i] {
			continue
		}
		if fr.multi {
			if playRepeatDup(d, playSeen, i) {
				continue
			}
			if !g.admissible(fr, fr.chosen, i) {
				continue
			}
			if fr.unitDedupe {
				dup := false
				for _, x := range es {
					dup = dup || x.a == fr.acts[i]
				}
				if dup {
					continue
				}
			}
			next := append(append([]int(nil), fr.chosen...), d.Options[i].Index)
			if !g.completable(fr, next, &budget) {
				continue
			}
		}
		es = append(es, ent{fr.acts[i], d.Options[i].Index})
		playSeen[d.Options[i].Obj] = true
	}
	if fr.multi && g.validAnswer(fr, fr.chosen) {
		es = append(es, ent{Pack(ActDone, RefNone, RefNone, 0), -1})
	}
	sort.Slice(es, func(i, j int) bool { return es[i].a < es[j].a })
	fr.cur = CDecision{Kind: fr.dk, Player: uint8(d.Player), Options: make([]Canon, 0, len(es))}
	fr.curAt = fr.curAt[:0]
	for _, x := range es {
		fr.cur.Options = append(fr.cur.Options, x.a)
		fr.curAt = append(fr.curAt, x.at)
	}
	if len(es) > MaxOptions {
		g.Stats.Overflow++
	}
}

// completeBudget bounds the completability search of one repose.
const completeBudget = 4096

// validAnswer reports whether chosen is a whole answer gorge accepts, as far
// as the published rules tell: Min, Decision.Validate (group caps, budgets,
// blocker floors and ceilings) and the Required attackers.
func (g *GorgeSide) validAnswer(fr *frame, chosen []int) bool {
	d := fr.d
	return (len(chosen) >= d.Min || d.AllowNone && len(chosen) == 0) && requiredCovered(fr, chosen) && evidenceTotalMet(d, chosen) &&
		d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: chosen}) == nil &&
		blockQuotaMet(d, chosen) && g.searchSharesLandType(d, chosen)
}

// searchSharesLandType reports whether a ShareLandType$ search answer names
// cards that share a land type: gorge's validateSearch (rules/cast_pricing.go)
// refuses one that does not, which Decision.Validate does not check, through
// the one classifier effects.SharedLandTypes (Myriad Landscape).
func (g *GorgeSide) searchSharesLandType(d *decision.Decision, chosen []int) bool {
	if d.Kind != decision.KChoose || d.ResumeKind != "search" || d.ResumeSA == nil || !effects.ChangeZoneOf(d.ResumeSA).ShareLandType {
		return true
	}
	ids := make([]state.ObjID, 0, len(chosen))
	for _, c := range chosen {
		if o := optByIndex(d, c); o.Obj != 0 {
			ids = append(ids, o.Obj)
		}
	}
	return effects.SharedLandTypes(g.E.G, ids)
}

// blockDutyFull reports whether blocker b carries a CR 509.1c requirement
// that every maximal declaration obeys: the quota equals the number of
// required blockers, so a pair of b's that obeys none can join no valid
// answer. It makes the completability search exact there, where its budget
// would otherwise turn optimistic on a wide board (Prized Unicorn).
func blockDutyFull(d *decision.Decision, b state.ObjID) bool {
	req, bReq := 0, false
	var seen []state.ObjID
	for i := range d.Options {
		o := &d.Options[i]
		if !(o.BlockMust || o.Required) || o.BlockMustAll || o.AttackMust || o.MinBlockers > 1 {
			if o.BlockMustAll || o.AttackMust || o.MinBlockers > 1 {
				return false // the other requirement shapes: leave it to the search
			}
			continue
		}
		if o.Obj == b {
			bReq = true
		}
		dup := false
		for _, x := range seen {
			if x == o.Obj {
				dup = true
			}
		}
		if !dup {
			seen = append(seen, o.Obj)
			req++
		}
	}
	return bReq && d.RequiredQuota() >= req
}

// blockQuotaMet reports whether a blockers answer obeys as many CR 509.1c
// requirements as possible: gorge's validateBlockers (rules/combat.go)
// refuses one below decision.RequiredQuota, which Decision.Validate does not
// check (Prized Unicorn's lure).
func blockQuotaMet(d *decision.Decision, chosen []int) bool {
	if d.Kind != decision.KBlockers {
		return true
	}
	return d.RequiredChosen(chosen) >= d.RequiredQuota()
}

// completable reports whether chosen extends (by zero or more admissible
// picks) to a valid answer. The search is exact within budget nodes and
// optimistic beyond it.
func (g *GorgeSide) completable(fr *frame, chosen []int, budget *int) bool {
	return g.completeFrom(fr, chosen, -1, budget)
}

// completeFrom is completable extending only with options of index > last
// (the completion's own picks are a set, searched in increasing order; the
// answer so far may hold any indices).
func (g *GorgeSide) completeFrom(fr *frame, chosen []int, last int, budget *int) bool {
	if g.validAnswer(fr, chosen) {
		return true
	}
	if !blockFloorsMatchable(fr, chosen) || !blockAttackMustReachable(fr, chosen) {
		return false
	}
	if len(chosen) >= fr.d.Max {
		return false
	}
	for i := range fr.d.Options {
		if *budget <= 0 {
			return true
		}
		// Picks are a set: extend in increasing index order only.
		if !fr.valid[i] || fr.d.Options[i].Index <= last && !fr.d.Repeatable || !g.admissible(fr, chosen, i) {
			continue
		}
		*budget--
		if g.completeFrom(fr, append(append([]int(nil), chosen...), fr.d.Options[i].Index), fr.d.Options[i].Index, budget) {
			return true
		}
	}
	return false
}

// blockFloorsMatchable reports whether every attacker a blockers answer has
// begun to block (CR 509.1a: menace, a MinMaxBlocker floor) can still get its
// missing blockers: each missing blocker is a distinct creature not yet
// declared that has a valid pair on that attacker (a bipartite matching, as
// the kernel's blocksCompletable). It is a necessary condition only, checked
// before the budgeted completion search, which on a wide board runs out of
// budget and answers optimistically for a pair that strands a menace
// attacker's second blocker.
func blockFloorsMatchable(fr *frame, chosen []int) bool {
	d := fr.d
	if d.Kind != decision.KBlockers {
		return true
	}
	var used []state.ObjID
	var blocked []state.ObjID // attacker, one entry per chosen pair
	for _, c := range chosen {
		o := optByIndex(d, c)
		used = append(used, o.Obj)
		blocked = append(blocked, o.Attacker)
	}
	var missing []state.ObjID // attacker, one entry per blocker still owed
	var seen []state.ObjID
	for i := range d.Options {
		o := &d.Options[i]
		if o.MinBlockers <= 1 || slices.Contains(seen, o.Attacker) {
			continue
		}
		seen = append(seen, o.Attacker)
		n := 0
		for _, a := range blocked {
			if a == o.Attacker {
				n++
			}
		}
		for ; n > 0 && n < o.MinBlockers; n++ {
			missing = append(missing, o.Attacker)
		}
	}
	if len(missing) == 0 {
		return true
	}
	var free []state.ObjID
	for i := range d.Options {
		o := &d.Options[i]
		if fr.valid[i] && !slices.Contains(used, o.Obj) && !slices.Contains(free, o.Obj) {
			free = append(free, o.Obj)
		}
	}
	canBlock := func(b, a state.ObjID) bool {
		for i := range d.Options {
			if o := &d.Options[i]; fr.valid[i] && o.Obj == b && o.Attacker == a {
				return true
			}
		}
		return false
	}
	match := make([]int, len(free)) // free blocker -> missing entry, -1 none
	for i := range match {
		match[i] = -1
	}
	var augment func(m int, seen []bool) bool
	augment = func(m int, seen []bool) bool {
		for j, b := range free {
			if seen[j] || !canBlock(b, missing[m]) {
				continue
			}
			seen[j] = true
			if match[j] < 0 || augment(match[j], seen) {
				match[j] = m
				return true
			}
		}
		return false
	}
	for m := range missing {
		if !augment(m, make([]bool, len(free))) {
			return false
		}
	}
	return true
}

// requiredCovered reports whether every creature an attackers decision
// marks Required (CR 508.1d) that can still be declared is in the answer so
// far: gorge's validateAttackers refuses a declaration that omits one.
func requiredCovered(fr *frame, chosen []int) bool {
	d := fr.d
	if d.Kind != decision.KAttackers {
		return true
	}
	for i := range d.Options {
		o := &d.Options[i]
		if !o.Required || !fr.valid[i] {
			continue
		}
		covered := false
		for _, c := range chosen {
			if optByIndex(d, c).Obj == o.Obj {
				covered = true
			}
		}
		if !covered && len(chosen) < d.Max {
			return false
		}
	}
	return true
}

// admissible reports whether option i may join the answer so far (the
// published whole-answer constraints: not chosen yet unless Repeatable,
// group caps, budget, set properties, same controller; one declaration per
// attacker/blocker creature).
func (g *GorgeSide) admissible(fr *frame, chosen []int, i int) bool {
	d := fr.d
	o := &d.Options[i]
	if fr.convoke != nil && !dirac.DebugConvokeNoAbsorb && !fr.convoke.admits(d, chosen, i) {
		return false
	}
	for _, c := range chosen {
		if c == o.Index && !d.Repeatable {
			return false
		}
		if o.Group == "" && (d.Kind == decision.KAttackers || d.Kind == decision.KBlockers) && optByIndex(d, c).Obj == o.Obj {
			return false
		}
	}
	if len(chosen) >= d.Max {
		return false
	}
	if d.Kind == decision.KBlockers && !o.BlockMust && !o.Required && blockDutyFull(d, o.Obj) {
		return false
	}
	if d.Kind == decision.KBlockers && o.MaxBlockers > 0 {
		n := 0
		for _, c := range chosen {
			if optByIndex(d, c).Attacker == o.Attacker {
				n++
			}
		}
		if n >= o.MaxBlockers {
			return false
		}
	}
	if o.Group != "" {
		n := 0
		for _, c := range chosen {
			if optByIndex(d, c).Group == o.Group {
				n++
			}
		}
		if n >= d.GroupCapFor(o.Group) {
			return false
		}
	}
	if d.HasBudget() {
		sum := o.Value
		for _, c := range chosen {
			sum += optByIndex(d, c).Value
		}
		if sum > d.MaxSum {
			return false
		}
	}
	if d.SetPropMode != "" && !decision.SetPropAdmits(d.SetPropMode, d.SetPropsOf(chosen), o.SetProps) {
		return false
	}
	if d.TargetsWithSameController && len(chosen) > 0 && optByIndex(d, chosen[0]).Controller != o.Controller {
		return false
	}
	return true
}

func optByIndex(d *decision.Decision, idx int) *decision.Option {
	if idx >= 0 && idx < len(d.Options) && d.Options[idx].Index == idx {
		return &d.Options[idx]
	}
	for i := range d.Options {
		if d.Options[i].Index == idx {
			return &d.Options[i]
		}
	}
	return &decision.Option{Index: idx}
}

// Apply implements Side: a must be one of the last Step's options.
func (g *GorgeSide) Apply(a Canon) {
	g.committed = false
	// Track the pip faces announced for the cast/activation in flight. A new
	// cast, activation or priority action starts a fresh payment; a pip
	// announcement accumulates into it. Target/mode picks sit between the pip
	// asks and the mana window, so they must not reset it. A trigger-cost
	// election is its own payment (no pips): reset so its manual window keeps
	// the no-owed first-colour answer.
	switch a.Kind() {
	case ActActivate, ActCast, ActCastFace, ActCastCasualty, ActCastImpended, ActCastAirbend, ActPass, ActPlayLand, ActSpecial:
		g.payPips = [6]uint8{}
	}
	if a.Kind() >= ActChoose {
		if i := int(a.Kind() - ActChoose); i < len(chooseKinds) {
			switch chooseKinds[i] {
			case "trigger_cost_pay", "trigger_cost_blight", "trigger_cost_counter":
				g.payPips = [6]uint8{}
			case "pay":
				if a.Mode() < 6 {
					g.payPips[a.Mode()]++
				}
			}
		}
	}
	fr := g.fr
	if fr == nil || g.err != nil {
		g.fail(fmt.Errorf("oracle: Apply(%v) with no posed decision", a))
		return
	}
	at := -2
	for i := range fr.cur.Options {
		if fr.cur.Options[i] == a {
			at = fr.curAt[i]
			break
		}
	}
	if at == -2 {
		g.fail(fmt.Errorf("oracle: Apply(%v): not offered", a))
		return
	}
	if fr.dk == DPriority {
		g.answer(fr, fr.prio[at])
		g.fr = nil
		return
	}
	if !fr.multi {
		g.commit(fr, []int{at})
		return
	}
	if fr.resting {
		if at < 0 {
			g.fail(fmt.Errorf("oracle: Apply(Done) in the rest phase of %s", fr.d.Kind))
			return
		}
		fr.rest = append(fr.rest, at)
		if len(fr.rest) < fr.restN-1 {
			g.repose(fr)
			return
		}
		// The last rest card is implied, as in the kernel's finishPeek.
		for i := range fr.d.Options {
			idx := fr.d.Options[i].Index
			if !slices.Contains(fr.chosen, idx) && !slices.Contains(fr.rest, idx) {
				fr.rest = append(fr.rest, idx)
				break
			}
		}
		g.commit(fr, fr.chosen)
		return
	}
	if at == -1 {
		if fr.restable {
			if n := len(fr.d.Options) - len(fr.chosen); n >= 2 {
				// Closing pile A opens pile B's order phase: keep the native
				// decision (and its top picks) for the intent's Rest.
				fr.resting, fr.restN = true, n
				g.repose(fr)
				return
			}
		}
		g.commit(fr, fr.chosen)
		return
	}
	fr.chosen = append(fr.chosen, at)
	// A combat declaration is always closed by Done (the kernel's AkDone,
	// which is where it taps the attackers); every other multi-pick
	// answer commits as soon as Max picks are made.
	combat := fr.d.Kind == decision.KAttackers || fr.d.Kind == decision.KBlockers
	if !combat && len(fr.chosen) >= fr.d.Max && g.validAnswer(fr, fr.chosen) {
		g.commit(fr, fr.chosen)
		return
	}
	g.repose(fr)
	if fr.d.Kind == decision.KChoose && fr.d.ResumeKind == "hand_move" && !fr.restable && len(fr.cur.Options) == 1 && fr.curAt[0] == -1 {
		// Only Done is left: no candidate remains to pick, and the kernel's
		// optional hand pick ("put up to N cards from your hand onto the
		// battlefield") ends there rather than posing a lone Done (CR 107.1c).
		// A target list and a convoke pick keep their Done: the kernel asks it
		// after the last candidate.
		g.commit(fr, fr.chosen)
		return
	}
	if len(fr.cur.Options) == 0 {
		// No pick and no valid Done: the published constraints cannot
		// finish this answer; let gorge's repair have it.
		g.commit(fr, fr.chosen)
	}
}

// commit submits a non-priority answer.
func (g *GorgeSide) commit(fr *frame, choices []int) {
	d := fr.d
	in := decision.Intent{Seq: d.Seq, Player: d.Player, Choices: append([]int(nil), choices...)}
	if fr.resting {
		in.Rest = append([]int(nil), fr.rest...)
	}
	g.fr = nil
	g.committed = true
	g.Stats.Commits++
	if err := g.submit(in); err != nil {
		g.Stats.Refused++
		rep := botpolicy.Clamp(d, in)
		rep.Seq, rep.Player = d.Seq, d.Player
		if err2 := g.submit(rep); err2 != nil {
			g.fail(fmt.Errorf("oracle: %s answer %v refused (%v); repair refused (%v)", d.Kind, in.Choices, err, err2))
		}
	}
}

// answer executes a priority entry.
func (g *GorgeSide) answer(fr *frame, pe prioEntry) {
	d := fr.d
	g.committed = true
	g.Stats.Commits++
	switch pe.kind {
	case prOption:
		if pe.a.Kind() != ActPass {
			g.Stats.Direct++
		}
		if err := g.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pe.opt}}); err != nil {
			g.fail(fmt.Errorf("oracle: %v refused: %w", pe.a, err))
		}
	case prPlan:
		g.Stats.PlanCasts++
		in := decision.Intent{Seq: d.Seq, Player: d.Player,
			Payment: &decision.PaymentSelection{ActionID: pe.pay.ID, Plan: decision.ClonePaymentPlan(pe.pay.Plans[0])}}
		if err := g.submit(in); err != nil {
			g.fail(fmt.Errorf("oracle: planned %v refused: %w", pe.a, err))
		}
	case prLower:
		g.Stats.Lowered++
		g.lower(pe)
	case prScript:
		g.Stats.Scripted++
		g.runScript(pe)
	}
}

// scriptCompletes runs pe on a clone of the engine and reports whether the
// play was then paid (the script reached a priority decision that prices it).
func (g *GorgeSide) scriptCompletes(pe prioEntry) bool {
	c := &GorgeSide{E: g.E.Clone(), M: g.M, Cfg: g.Cfg, hyp: false}
	c.id = identity{inst: append([]Ref(nil), g.id.inst...), obj: append([]state.ObjID(nil), g.id.obj...), scanned: g.id.scanned, lay: g.id.lay, ncard: g.id.ncard, gen: g.id.gen}
	if g.hyp {
		// A hypothetical engine's chance state does not survive Clone's
		// plain submit path; run the probe through SubmitHypothetical too.
		c.hyp = true
	}
	ok := func() (ok bool) {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		c.runScript(pe)
		return c.err == nil && c.Stats.Aborts == 0
	}()
	return ok
}

// lowerCompletes runs pe's lowering on a clone of the engine and reports
// whether it made the play: no abort, no failure, and not a lowering that
// only passed until the game ended.
func (g *GorgeSide) lowerCompletes(pe prioEntry) bool {
	c := &GorgeSide{E: g.E.Clone(), M: g.M, Cfg: g.Cfg, hyp: g.hyp}
	c.id = identity{inst: append([]Ref(nil), g.id.inst...), obj: append([]state.ObjID(nil), g.id.obj...), scanned: g.id.scanned, lay: g.id.lay, ncard: g.id.ncard, gen: g.id.gen}
	return func() (ok bool) {
		defer func() {
			if recover() != nil {
				ok = false
			}
		}()
		return c.lower(pe) && c.err == nil
	}()
}

// passOthers passes priority for the other players while a trigger the
// script stacked resolves, and answers a trigger order of theirs in option
// order, as the search assumed (scriptSearch.visit), and returns the
// decision then pending.
func (g *GorgeSide) passOthers(p state.PlayerID, pe prioEntry) *decision.Decision {
	e := g.E
	d := e.Pending()
	for guard := 0; d != nil && !e.G.Over && d.Player != p && (d.Kind == decision.KPriority || d.Kind == decision.KTriggerOrder) && guard < 8; guard++ {
		if d.Kind == decision.KTriggerOrder {
			order := make([]int, 0, len(d.Options))
			for _, o := range d.Options {
				order = append(order, o.Index)
			}
			g.Stats.Internal++
			if err := g.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: order}); err != nil {
				g.fail(fmt.Errorf("oracle: script of %v: other player's trigger order refused: %w", pe.a, err))
				return nil
			}
			d = e.Pending()
			continue
		}
		pass := -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				pass = o.Index
			}
		}
		if pass < 0 {
			break
		}
		g.Stats.Internal++
		if err := g.submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}); err != nil {
			g.fail(fmt.Errorf("oracle: script of %v: other player's pass refused: %w", pe.a, err))
			return nil
		}
		d = e.Pending()
	}
	return d
}

// runScript answers the scripted prefix's decisions, then pays the play by
// the route the vocabulary at the following priority decision prices it
// (planned cast, lowering or offered option).
func (g *GorgeSide) runScript(pe prioEntry) {
	e := g.E
	p := e.Pending().Player
	for i, st := range pe.script {
		// A trigger the script's activation stacked: the search assumed the
		// other players pass while it resolves (scriptSearch.visit).
		d := g.passOthers(p, pe)
		if g.err != nil {
			return
		}
		if d == nil || e.G.Over || d.Player != p || d.Kind != st.Kind {
			g.Stats.Aborts++
			return
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		for _, pk := range st.Picks {
			found := -1
			for j := range d.Options {
				// a pick names its option by identity, not by slot: a repeat
				// of the same label (a monochrome "Add 2 mana in any
				// combination of colors" answer: W then W) is the next
				// matching option, not the first one again
				if pk.Matches(&d.Options[j]) && !slices.Contains(in.Choices, d.Options[j].Index) {
					found = d.Options[j].Index
					break
				}
			}
			if found < 0 {
				g.fail(fmt.Errorf("oracle: script step %d of %v: pick %q not offered", i, pe.a, pk.Label))
				return
			}
			in.Choices = append(in.Choices, found)
		}
		g.Stats.Internal++
		if err := g.submit(in); err != nil {
			g.fail(fmt.Errorf("oracle: script step %d of %v refused: %w", i, pe.a, err))
			return
		}
	}
	// Answer any colour ask the last activation posed, then re-price.
	for guard := 0; guard < 8; guard++ {
		d := g.passOthers(p, pe)
		if g.err != nil || d == nil || e.G.Over {
			return
		}
		if d.Kind == decision.KPriority {
			if d.Player != p {
				g.Stats.Aborts++
				return
			}
			g.id.sync(e.G)
			fr := &frame{seq: d.Seq, d: d.Clone(), dk: DPriority}
			g.priorityVocabulary(fr)
			for _, x := range fr.prio {
				if x.a == pe.a && x.kind != prScript {
					g.Stats.Commits--
					g.answer(fr, x)
					return
				}
			}
			g.Stats.Aborts++
			return
		}
		if !g.internal(d) {
			g.Stats.Aborts++
			return
		}
	}
}

// lower pays a potential play by its witness (payexec): activate each plan
// source at priority, answer the colour asks, then pick the play's option.
var debugLower = os.Getenv("KFEAT_DEBUG") != ""

func (g *GorgeSide) lower(pe prioEntry) (done bool) {
	e := g.E
	d := e.Pending()
	x := payexec.StartPlay(d.Player, pe.play, pe.plan, payexec.PoolOf(e, d.Player))
	for x.Status() == payexec.InProgress {
		if e.G.Over {
			// The activations that fund the play can win the game before it
			// is cast: a Treasure sacrificed beside The Sackville-Bagginses
			// drains the last life, and its trigger resolves ahead of the
			// cast the kernel makes from inside its payment window (CR
			// 601.2g, 603.3b). With every activation made and the lowering
			// seat the winner the line is complete: nothing a later cast
			// could change remains.
			return x.Remaining() == 0 && !e.G.Draw && e.G.Winner == x.Player
		}
		d := e.Pending()
		if d == nil {
			g.fail(fmt.Errorf("oracle: lowering %v: no pending decision", pe.a))
			return false
		}
		var in decision.Intent
		if d.Player != x.Player && d.Kind == decision.KPriority && x.Waits > 0 {
			in = decision.Intent{Seq: d.Seq, Player: d.Player}
			for _, o := range d.Options {
				if o.Kind == "pass" {
					in.Choices = []int{o.Index}
				}
			}
		} else {
			var st payexec.Status
			in, st = x.StepOn(d, payexec.SurfaceOf(e, d.Player))
			switch st {
			case payexec.Aborted:
				g.Stats.Aborts++
				if debugLower {
					fmt.Printf("  lower %v aborted: %s %s at %v %q\n", pe.a, x.Reason, x.Detail, d.Kind, d.Prompt)
				}
				return false
			case payexec.Yield:
				g.Stats.Yields++
				in = botpolicy.Clamp(d, decision.Intent{Seq: d.Seq, Player: d.Player})
				in.Seq, in.Player = d.Seq, d.Player
			}
		}
		g.Stats.Internal++
		if debugLower {
			fmt.Printf("  lower %v: %v p%d %d options, intent %v %v\n", pe.a, d.Kind, d.Player, len(d.Options), in.Choices, in.Payment != nil)
			for _, o := range d.Options {
				fmt.Printf("    opt %s %q\n", o.Kind, o.Label)
			}
		}
		if err := g.submit(in); err != nil {
			g.fail(fmt.Errorf("oracle: lowering %v refused: %w", pe.a, err))
			return false
		}
	}
	return x.Status() == payexec.Done
}

// GorgeKernel is the self-check's kernel: a second gorge engine, an
// independent GorgeSide. It validates canonicalisation, projection, intent
// mapping and RNG draw accounting before the real kernel exists.
func GorgeKernel(cfg Config) SideFactory {
	return func(m *Matchup, seed uint64) (Side, error) {
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err == nil {
			// Unlike the oracle side, this one recycles decision storage
			// (rules' decision arena): a frame that kept the engine's
			// decision instead of its clone would read reused storage.
			g.E.SetDecisionArena(true)
		}
		return g, err
	}
}

var _ Side = (*GorgeSide)(nil)

// forcedPlayerChoice reports gorge's ChoosePlayer ask with one player to name.
// With two players that is "choose an opponent", which kgen lowers to the
// opponent without an ask, so the kernel has no decision to answer here.
func forcedPlayerChoice(d *decision.Decision) bool {
	return len(d.Options) == 1 && d.Options[0].Kind == "player" && d.ResumeSA != nil && d.ResumeSA.API == "ChoosePlayer"
}

// selfDiscardCost reports gorge's singleton Discard<1/CARDNAME> cost ask
// (rules discardAsk, cast_subcounter.go): a cycling, typecycling or channel
// activation poses a KChoose whose only option is the source card itself.
// It is no choice (CR 602.2b/601.2h; Forge CostDiscard.payCostFromSource and
// XMage DiscardSourceCost pay it without a prompt, and gorge's own sacAsk,
// exAsk and returnAsk skip the same singleton), and the kernel pays it in
// place (discardSelfCost), so it is answered beneath the canonical surface
// like a payment window.
func (g *GorgeSide) selfDiscardCost(d *decision.Decision) bool {
	if len(d.Options) != 1 || d.Min != 1 || d.Max != 1 || d.Source == 0 {
		return false
	}
	o := &d.Options[0]
	if o.Kind != "discard" || o.Obj != d.Source || !strings.HasPrefix(d.Prompt, "Discard a card to pay the cost of ") {
		return false
	}
	ob := g.E.G.Obj(d.Source)
	return ob != nil && ob.Zone == state.ZHand
}

// maxSpeedOrdinal is the position of the AddAbility$ SVar sv among the
// Max-speed statics of object id's face (the order the kernel's gated
// activated abilities follow; dirac/oracle/realkernel.go maxSpeedCode).
func (g *GorgeSide) maxSpeedOrdinal(id state.ObjID, sv string) (int, bool) {
	o := g.E.G.Obj(id)
	if o == nil || o.Face() == nil {
		return 0, false
	}
	n := 0
	for _, st := range o.Face().Statics {
		if st.Mode != "Continuous" || st.Params["Condition"] != "MaxSpeed" || st.Params["AddAbility"] == "" {
			continue
		}
		if st.Params["AddAbility"] == sv {
			return n, true
		}
		n++
	}
	return 0, false
}

// copyGrantedAbility reports whether ability i of object id's face is one a
// copy effect's AddAbilities$ added (CR 707.9b): the fold marks it
// (cards.SA.IsCopyGranted), which no printed A: line is, whatever its text.
func (g *GorgeSide) copyGrantedAbility(id state.ObjID, i int) bool {
	return copyGrantedAbilityOf(g.E.G.Obj(id), i)
}

// copyGrantedAbilityOf is copyGrantedAbility over the object itself.
func copyGrantedAbilityOf(o *state.Object, i int) bool {
	if o == nil || o.Face() == nil || i < 0 || i >= len(o.Face().Abilities) {
		return false
	}
	return o.Face().Abilities[i].IsCopyGranted()
}

// selfKeywordGrant reports whether a continuous effect on object id itself
// grants it the derived keyword line line and leaves its abilities alone
// (Domesticated Watercourse's Animate registers Source: id, Affects:
// Card.Self, Layer: LAbilities, AddKeywords: "Equip:2"). A printed static is
// not in RegisteredContinuous, and a static another permanent grants carries
// that source, so neither matches.
func (g *GorgeSide) selfKeywordGrant(id state.ObjID, line string) bool {
	line = strings.TrimSpace(line)
	for _, ce := range g.E.RegisteredContinuous() {
		if ce.Source != id || ce.Layer != state.LAbilities || ce.RemoveAbilities || ce.GrantedStatic || !strings.EqualFold(strings.TrimSpace(ce.Affects), "Card.Self") {
			continue
		}
		for _, kw := range ce.AddKeywords {
			if strings.EqualFold(strings.TrimSpace(kw), line) {
				return true
			}
		}
	}
	return false
}
