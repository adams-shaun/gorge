package rules

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	costvocab "github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// ScriptPick names one chosen option of a ScriptStep by identity, so a
// replay can find it again in the live decision (whose option indices are
// the clone's) and refuse a decision that is not the one searched.
type ScriptPick struct {
	Kind       string
	Obj        state.ObjID
	Label      string
	ManaSymbol string
	// Ability is the option's ability index: a source with several abilities
	// of one label (Jasmine Dragon Tea Shop's restricted "any color" beside the
	// one Great Divide Guide grants it) is told apart by it alone, and the
	// replay must take the ability the search took.
	Ability int
}

// Matches reports whether o is the picked option.
func (k ScriptPick) Matches(o *decision.Option) bool {
	return o.Kind == k.Kind && o.Obj == k.Obj && o.Label == k.Label && o.ManaSymbol == k.ManaSymbol && o.Ability == k.Ability
}

// ScriptStep is one of the payer's answers along a PotentialPlan.Script: at
// a priority decision the "activate" option of an uncovered source, at an
// activation's ask (the mana wheel, the colour, Saruli Caretaker's creature
// to tap) the options chosen, in order. An ask answered with nothing has no
// picks.
type ScriptStep struct {
	Kind  decision.Kind
	Picks []ScriptPick
}

// potentialScriptPlays bounds the prefix searches one PotentialPaymentPlans
// query runs, and potentialScriptNodes the clones one search makes: a play
// beyond either stays unpriced (never a proof). Both are stated limits, not
// proofs of completeness: a query with more than sixteen plays the printed
// sources cannot pay leaves the later ones census_incomplete, and so does a
// search that needs more than 160 clones. (Four plays left a legal play
// unoffered on boards with a gap source; sixteen was never reached in 1500
// Standard games, at about 2% more clones than four.)
const (
	potentialScriptPlays = 16
	potentialScriptNodes = 160
)

// potentialCastKind is the potential play kind a plain cast's prefix search
// prices (the covered-source spendability prune is cast-only).
const potentialCastKind = "cast"

// potentialPrefixScript searches, on clones of the engine, for a sequence
// of activations of the census's uncovered sources (gaps: the mana
// abilities the planner census cannot price, e.g. Saruli Caretaker's
// "{T}, tap an untapped creature you control: Add one mana of any color",
// Wall of Roots' "Put a -0/-1 counter: Add {G}") after which the planner
// itself prices potential play o of p from the floating pool -- or the
// play is simply offered. Every answer an activation poses (the colour, the
// creature to tap) is branched on, in engine order, and each distinct
// payer state is visited once. The script is the payer's answers along the
// found path; ok is false when none was found within potentialScriptNodes.
// funding is how many other (covered) sources the prefix may activate too:
// one per missed ability with a mana fee, to pay it (a filter such as Heap
// Gate's "{1}, {T}: Add one mana of any color" needs mana floating first).
//
// The search is a pure read of e: it only submits intents to clones taken
// at this intent boundary.
func (e *Engine) potentialPrefixScript(p state.PlayerID, o decision.Option, gaps []state.ObjID, funding int) ([]ScriptStep, bool) {
	if len(gaps) == 0 {
		return nil, false
	}
	gaps = e.scriptSpendableGaps(p, o, gaps)
	if len(gaps) == 0 {
		return nil, false
	}
	a := decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}
	desc, descOK := scriptPlayDesc(o, func() Cost { return e.castOfferBase(p, o.Obj) })
	build := func(phased bool) *scriptSearch {
		sr := e.newScriptSearch(p, a, o.AltCostIndex, potentialScriptNodes)
		if sr == nil {
			return nil
		}
		sr.gaps, sr.gapsOnly, sr.funding, sr.printedOnly = gaps, true, funding, o.Kind == optAbility
		sr.twinCut = potentialTwinCut(o)
		sr.feeGaps = map[state.ObjID]bool{}
		for _, id := range gaps {
			for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
				cc := e.manaAbilityCC(p, id, ma)
				mp := effects.ManaOf(ma)
				units := availableAmount(ma)
				if !mp.CountsAny {
					// Counts is the colour superset for an "Any"/"Combo"
					// production (five colours for one unit), so only a
					// fixed multi-colour string counts its units directly.
					total := int32(0)
					for _, n := range mp.Counts {
						total += n
					}
					units *= total
				}
				if cc != nil && units >= 2 && (cc.Cost.CMC() > 0 || cc.Cost.X > 0) {
					sr.feeGaps[id] = true
					break
				}
			}
		}
		// A may-play cast (Mode "mayplay": cast from exile by a granted
		// permission, at the printed cost) takes the same order as a hand
		// cast: its plain walk taps Baxter Building's free {C} ability first
		// and spends the budget before the {4} filter is funded.
		if phased && (o.Kind == optCast && (o.Mode == "" || o.Mode == "mayplay") || o.Kind == optAbility) && o.AltCostIndex == 0 &&
			len(sr.feeGaps) != 0 && !sr.phasedScope(e) {
			sr.feeOrder = true
		}
		for _, id := range gaps {
			for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
				if repeatableFeeOnlyMana(e, p, id, ma) {
					sr.feeOnly = true
				}
			}
		}
		sr.actDesc, sr.actDescOK = desc, descOK
		sr.instance, sr.instanced = o.Cost, o.Kind == "cast"
		sr.passes = true
		sr.phased = phased
		return sr
	}
	// The plain order first: a script it finds is the script dirac's
	// node-for-node mirror (ctrScript and friends) prices the play by, so it
	// must stay the search's answer wherever the plain search finds one --
	// an offer both engines priced yesterday, replayed by a recorded game,
	// may not change the prefix it is funded by. Only where the plain
	// search found none does the phased order run: it spends the plain
	// search's budget on the sources' free mana first and the fee-carrying
	// gaps last (their fee is paid from the floating pool, CR 601.2b), so a
	// plain search that budget-limited on dead fee branches finds the
	// funding line it walked past. The fallback only takes a script when the
	// play's own cost sacrifices or returns a permanent, when a census gap
	// is a hybrid-fee mana ability, or when the plain run exhausted its node
	// budget walking a repeatable fee-only filter's branches: the ordering it
	// fixes is the one a cast that sacrifices its own funding source needs,
	// and for any other play the node-for-node mirror does not model the
	// phased order, so a recorded game's other offers keep the pre-existing
	// scope.
	plain := build(false)
	if plain == nil {
		return nil, false
	}
	plain.feeStall = true // the plain run's doomed-branch prune (feeOnlyAskPrune)
	script, found := plain.run(e)
	if found && len(script) > 0 {
		return script, true
	}
	sr := build(true)
	if sr == nil {
		return nil, false
	}
	sr.plainLimited = plain.limited && plain.feeOnly
	script, found = sr.run(e)
	if found && len(script) > 0 {
		return script, true
	}
	// The subsume run, only where the plain search ran out of budget (a
	// completed plain search saw the whole space, whose paying states the
	// subsume run's kept answers also reach) and the play's cost is
	// coloured pips alone with no fee-carrying gap to fund (a generic or X
	// pip, or a fee, is payable with any colour, so a covered colour is not
	// dominated while one is outstanding; census.fees counts the latter).
	if plain.limited && funding == 0 {
		ss := build(false)
		if ss != nil {
			ss.subsume = true
			script, found = ss.run(e)
		}
	}
	return script, found && len(script) > 0
}

// scriptSpendableGaps drops from gaps each source whose every mana ability
// makes mana the play o cannot spend (CR 106.6): Mm'menon's granted
// "{T}: Add {U}. Spend this mana only to cast a spell from anywhere other
// than your hand" on an artifact, for a cast from the hand. Activating such a
// source can only float mana the cast refuses, so a prefix through it pays
// nothing the prefix without it does not; the search would spend its node
// budget on those dead branches (and on the triggers their taps set off)
// before reaching the source that pays. An activation is filtered the same
// way (Lilysplash Mentor's {1}{G}{U} beside Frogs whose granted Clement mana
// spends only on creature spells, with Hidden Grotto's filter the source that
// pays). Only a plain cast of the printed cost (from the hand, or under a
// may-play permission that leaves the cost alone: Summoning Materia's top of
// the library) or an activation's printed cost is filtered; any other play
// keeps every gap.
func (e *Engine) scriptSpendableGaps(p state.PlayerID, o decision.Option, gaps []state.ObjID) []state.ObjID {
	if o.AltCostIndex != 0 || o.Mode != "" && castModeCodes.Code(o.Mode) != castModeMayplay {
		return gaps
	}
	desc, ok := scriptPlayDesc(o, func() Cost { return e.castOfferBase(p, o.Obj) })
	if !ok {
		return gaps
	}
	var out []state.ObjID
	for i, id := range gaps {
		if e.gapSpendableFor(p, id, desc) {
			if out != nil {
				out = append(out, id)
			}
			continue
		}
		if out == nil {
			out = append(make([]state.ObjID, 0, len(gaps)), gaps[:i]...)
		}
	}
	if out == nil {
		return gaps
	}
	return out
}

// scriptPlayDesc is the searched play's payment descriptor: scriptSpendableGaps'
// gap admission and the fee-stall prune's pool read (pay.AvailableFor) share
// it. false for a play that is neither a plain cast nor a printed ability.
func scriptPlayDesc(o decision.Option, castBase func() Cost) (paymentDescriptor, bool) {
	switch o.Kind {
	case "cast":
		cost := castBase()
		return paymentDescriptor{ID: o.Obj, Class: pay.PurposeSpell, Cost: &cost}, true
	case optAbility:
		return paymentDescriptor{ID: o.Obj, Class: pay.PurposeActivated}, true
	}
	return paymentDescriptor{}, false
}

// gapSpendableFor reports whether one of id's mana abilities the census reads
// (its own membership walk, which ignores the live pool so a filter's fee may
// be funded later) makes mana desc's payment can spend.
func (e *Engine) gapSpendableFor(p state.PlayerID, id state.ObjID, desc paymentDescriptor) bool {
	for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
		if valid := effects.ManaOf(ma).RestrictValid; valid == "" || pay.RestrictValidMatches(asPayer(e), p, desc, valid, id) {
			return true
		}
	}
	return false
}

// scriptSearch is the depth-first search behind potentialPrefixScript and
// PotentialPlayScript: from p's priority decision, activate mana sources on
// clones, branch on every answer their asks admit, and stop at the first
// payer priority where the play is offered or (priced) the planner pays it
// from the floating pool. Each distinct payer state is visited once.
type scriptSearch struct {
	p        state.PlayerID
	play     decision.PotentialAction
	altCost  int
	gaps     []state.ObjID // sources the census misses, tried first
	gapsOnly bool          // activate only gaps (plus funding others)
	funding  int           // non-gap activations allowed when gapsOnly
	// need holds the coloured pips of the play's base cost (scriptNeedColours):
	// the search tries an ask's answers naming one of them first. A colour ask
	// whose options run W U B R G otherwise spends the node budget proving the
	// colours a cost does not name dead before the first one it does -- a
	// 4GG cast behind Great Divide Guide's granted any-colour abilities went
	// budget-limited at 160 clones exploring B before G at the deepest asks.
	need   [state.MC + 1]bool
	passes bool // also pass priority while the stack is non-empty
	// phased runs the visit activation loop in its phased order with the
	// real-payable gates (potentialPrefixScript's fallback run). The plain
	// run keeps the order and node walk dirac's mirror prices plays by.
	phased bool
	// feeOrder is the phased run's generic mana-fee order: the covered
	// funding first, then the fee-carrying gaps, then the fee-free gaps. A
	// fee-carrying gap's fee is a mana payment the engine settles from the
	// floating pool; settling it before an any-colour free gap has floated
	// the play's own pip strands that pip (the generic deduction spends
	// WUBRG order: Shadowblood Ridge's {1} takes the Treasure's {U} before a
	// Cinder Barrens {B} beside it), while funding first leaves the fee a
	// colour the play can spare. The order is a fallback only, like phased:
	// the plain run keeps the node walk dirac's mirror prices plays by.
	feeOrder bool
	// feeGaps marks the census gaps whose mana ability costs mana and makes
	// more than one unit. A one-unit fee source is a filter: its net
	// production after the fee is zero, so the fee-only paths price it, and
	// funding it first only changes the node walk. feeOrder walks the
	// multi-unit fee gaps between the funding and the free gaps.
	feeGaps map[state.ObjID]bool
	// plainLimited is set on the phased run when the plain run exhausted its
	// node budget on a board with a repeatable fee-only filter (Signpost
	// Scarecrow's "{2}: Add one mana of any color"), whose fee branches the
	// plain order spends its budget on before the fee is funded. A phased
	// script is taken outside phasedScope only then; a plain run that
	// completed, or limited without such a filter (Barrels of Blasting
	// Jelly's once-a-turn {1} filter), keeps the scope below, where the
	// phased order is only modelled for a cost that sacrifices or returns a
	// permanent, or for a hybrid-fee gap.
	plainLimited bool
	// subsume is potentialPrefixScript's third run: with the plain and phased
	// orders both unable to price the play, it drops a mana activation's
	// colour-ask answers the floating pool has already covered (a kept answer
	// dominates them), so the search walks the funding line before its budget
	// runs out (subsumeAnswers). The run is strictly narrower than the plain
	// one -- every dropped answer's paying continuations are reachable along
	// a kept one -- so it can only find a script the plain search walked
	// past, never refuse one it found.
	subsume bool
	// feeOnly: the census's gaps include a fee-only repeatable source (Energy
	// Refractor's "{2}: Add one mana of any color"). The activation-site and
	// one-shot cuts below are scoped to it: they exist to float that source's
	// fee before it is activated, and applying them to a board with no such
	// source only moves the node walk other searches share a budget with (the
	// granted-mana prefix's capSpent). The landed ask prune (feeStall) stays
	// on every plain run; the twin cut (scriptTwin) runs on this fee-only
	// path and for the activated plays potentialTwinCut classifies as printed
	// or gained.
	feeOnly bool
	// twinCut: run the twin cut even outside the fee-only path. The prefix
	// search sets it from the play's full option (potentialTwinCut: printed or
	// gained only -- an SVar grant is Kind "ability" too, but dirac encodes it
	// 0x80|ability like the max-speed and exile grants it cannot tell it apart
	// from). The public PotentialPlayScript cannot see the grant anchors (its
	// action's wire form has none) and keeps its Kind "ability" cut.
	twinCut bool
	// feeStall arms the plain run's doomed-branch prune (feeOnlyAskPrune): a
	// fee-only repeatable gap's colour ask drops the answers whose colour
	// cannot bring the play closer to paid (CR 605.3a: a line that skips the
	// net-<=0 repeatable activation holds at least as much of every colour as
	// one that uses it). Without it the search spends its whole node budget
	// on a fee-only source's own activations (Energy Refractor's "{2}: Add
	// one mana of any color" branches five ways per activation, each a
	// distinct payer state) and starves before the funding line that pays the
	// play without the source is reached. Only the plain run arms it: it is
	// the run dirac's node-for-node mirror (mana_filter.go filtColours)
	// prices plays by, and the fallbacks below only run where the plain run
	// already found nothing.
	feeStall bool
	// actPool is the pool at the node the last gap activation was submitted
	// from, and actSrc its source: the activation's colour ask is read after
	// its fee left the pool, and the fee's payment destroys the shapes it
	// spent ({W}{R} paying {2} leaves no trace of either), so the stall test
	// reads the opening pool (the kernel's mirror reads the node's pool,
	// mana_filterrepeat.go filtFeeColStalls). The ask of the activation's own
	// resolution follows it directly, so the stash is still the ask's.
	// The stash reads the pool this payment may spend (pay.AvailableFor: the
	// restricted batches actDesc admits back in), the view the kernel's mirror
	// adds its admitted restricted units beside (filtFeeColStalls: s.pl[p].pool
	// plus rbUnits of restrPoolAdmits), so both short-counts read the same
	// mana. actDesc is the searched play's payment descriptor
	// (scriptPlayDesc); actDescOK false (a play that is neither a plain cast
	// nor a printed ability, which the kernel never searches a filter for)
	// makes the prune fail open.
	actPool   state.Mana
	actSrc    state.ObjID
	actDesc   paymentDescriptor
	actDescOK bool
	// costCol holds the searched play's base cost's coloured pips per colour
	// when that cost is coloured pips alone (scriptNeedCounts), the bound the
	// subsume run's colour-ask filter prunes against. costOK is false for a
	// cost with a generic, snow or X pip or a hybrid face, whose pips any
	// colour may help pay.
	costCol [state.MC + 1]int32
	costOK  bool
	// pipCol is the searched play's base cost's fixed pips per colour, pipOK
	// whether they were read (scriptPipCounts): the fee-stall prune's bound
	// (feeOnlyAskPrune). Unlike costOK it is not about the pips alone -- a
	// generic pip is fine, payable with any colour -- but about FIXED pips:
	// pipOK is false when the cost has a part whose payment any colour may
	// help beyond a generic one (an X, snow, forage or hybrid-family pip) or
	// whose base the read does not name (scriptPlayBaseCost: a mode or
	// alternative cost), and the prune then fails open.
	pipCol [state.MC + 1]int32
	pipOK  bool
	// printedOnly: the play is a printed ability, and a granted one sharing its
	// Ability is another option (only a printed ability reaches the planner).
	printedOnly bool
	// instance, when instanced, is the Option.Cost of the cast instance the
	// play denotes: a printed flashback ("") or a further granted one (CR
	// 702.34a, 113.2c). Each instance is its own permission with its own
	// cost, so the cheaper granted instance appearing is not the printed
	// play being offered.
	instance  string
	instanced bool
	// nodePlans asks the planner at every payer priority node; unproven
	// records a node verdict that was neither a witness nor a proof.
	nodePlans bool
	unproven  bool
	budget    int
	turn      int32
	step      state.Step
	stack     int        // the stack height at the decision
	ownZone   state.Zone // the zone the searched ability's source is activated from
	nodes     int
	limited   bool // the budget ran out
	approx    bool // a branch assumed an answer it did not enumerate
	seen      map[string]bool
	path      []ScriptStep
}

func (e *Engine) newScriptSearch(p state.PlayerID, a decision.PotentialAction, altCost, budget int) *scriptSearch {
	d := e.Pending()
	if d == nil || d.Player != p || d.Kind != decision.KPriority {
		return nil
	}
	sr := &scriptSearch{p: p, play: a, altCost: altCost, budget: budget, turn: e.G.Turn, step: e.G.Step, stack: len(e.G.Stack), seen: map[string]bool{}}
	if o := e.G.Obj(a.Obj); o != nil {
		sr.ownZone = o.Zone
	}
	if altCost <= 0 {
		sr.need = scriptNeedColours(e, e.G.Obj(a.Obj), p, a)
		sr.costCol, sr.costOK = scriptNeedCounts(e, e.G.Obj(a.Obj), p, a)
		sr.pipCol, sr.pipOK = scriptPipCounts(e, e.G.Obj(a.Obj), p, a)
	}
	return sr
}

// scriptNeedReader is the read-only side of *Engine scriptNeedColours needs;
// a narrow interface keeps the helper off Engine's method set.
type scriptNeedReader interface {
	castOfferBase(state.PlayerID, state.ObjID) costvocab.Cost
	abilityCompiledCost(*cards.SA) *compiledCost
}

// scriptNeedColours reports the coloured pips the searched play's base cost
// announces (CR 601.2b: the caster announces each pip's colour before
// targets): a cast's printed mana cost, an ability's compiled cost. A hybrid,
// twobrid or hybrid-Phyrexian pip needs each of its faces; a Phyrexian pip
// needs its colour (its life face is not mana). The set is only the search's
// branch-order preference -- it changes which equal answers a colour ask's
// search tries first, never which answers exist. A cast of a card mode or an
// alternative cost (flashback, kicker) whose pips the printed base does not
// carry leaves the set empty, and so the engine order; a may-play cast reads
// the printed base (scriptPlayBaseCost).
func scriptNeedColours(r scriptNeedReader, o *state.Object, p state.PlayerID, a decision.PotentialAction) [state.MC + 1]bool {
	var need [state.MC + 1]bool
	add := func(c costvocab.Cost) {
		for i, n := range c.Colored {
			if n > 0 && i <= state.MC {
				need[i] = true
			}
		}
		for _, h := range c.Hybrid {
			if i := state.ManaIndex(h.A); i <= state.MC {
				need[i] = true
			}
			if i := state.ManaIndex(h.B); i <= state.MC {
				need[i] = true
			}
		}
		for _, ph := range c.Phyrexian {
			if i := state.ManaIndex(ph); i <= state.MC {
				need[i] = true
			}
		}
		for _, tw := range c.Twobrid {
			if i := state.ManaIndex(tw.Col); i <= state.MC {
				need[i] = true
			}
		}
		for _, hp := range c.HybridPhyrexian {
			if i := state.ManaIndex(hp.A); i <= state.MC {
				need[i] = true
			}
			if i := state.ManaIndex(hp.B); i <= state.MC {
				need[i] = true
			}
		}
	}
	if c, ok := scriptPlayBaseCost(r, o, p, a); ok {
		add(c)
	}
	return need
}

// scriptPlayBaseCost is the cost the searched play's base face announces (CR
// 601.2b: the caster announces each pip's colour before targets): a plain
// cast's printed mana cost, an ability's compiled cost. A may-play cast (a
// permission that only opens the zone: Summoning Materia's top of the
// library) reads the printed base too -- scriptSpendableGaps' same
// admission, which prices its payment descriptor with castOfferBase
// (scriptPlayDesc), and dirac's mirror prices the play by the printed pips
// beside it (mana_filter.go gapFilteredPlay admits castMayPlay). ok is false
// when the play's cost is not the printed base one -- a cast of a card mode
// or an alternative cost (flashback, kicker, bargain), an unresolvable
// ability, or a play that is neither kind -- so both need-readers
// (scriptNeedColours, the search's branch-order preference, and
// scriptNeedCounts, the subsume run's prune bound) leave the cost unread
// rather than guess one.
func scriptPlayBaseCost(r scriptNeedReader, o *state.Object, p state.PlayerID, a decision.PotentialAction) (costvocab.Cost, bool) {
	if o == nil || o.Face() == nil {
		return costvocab.Cost{}, false
	}
	switch a.Kind {
	case "cast":
		if a.Mode != "" && castModeCodes.Code(a.Mode) != castModeMayplay {
			return costvocab.Cost{}, false
		}
		return r.castOfferBase(p, a.Obj), true
	case optAbility:
		pa, ok := o.PileAbilityAt(a.Ability)
		if !ok || pa.SA == nil {
			return costvocab.Cost{}, false
		}
		return r.abilityCompiledCost(pa.SA).Cost, true
	default:
		return costvocab.Cost{}, false
	}
}

// scriptNeedCounts is scriptNeedColours as counts: the searched play's base
// cost's coloured pips per colour, ok false when the cost has a part whose
// payment any colour may help (a generic, snow or X pip, a hybrid, Phyrexian,
// twobrid or hybrid-Phyrexian face) or that the base cost does not announce
// (a non-"mayplay" mode or an alternative cost; scriptPlayBaseCost): the
// subsume run's colour-ask filter prunes a colour only when floating more of
// it can pay nothing.
func scriptNeedCounts(r scriptNeedReader, o *state.Object, p state.PlayerID, a decision.PotentialAction) ([state.MC + 1]int32, bool) {
	var counts [state.MC + 1]int32
	c, ok := scriptPlayBaseCost(r, o, p, a)
	if !ok {
		return counts, false
	}
	if c.Generic != 0 || c.Snow != 0 || c.X != 0 || c.Forage ||
		len(c.Hybrid) != 0 || len(c.Phyrexian) != 0 || len(c.Twobrid) != 0 || len(c.HybridPhyrexian) != 0 {
		return counts, false
	}
	any := false
	for i, n := range c.Colored {
		if n > 0 && i <= state.MC {
			counts[i] = n
			any = true
		}
	}
	return counts, any
}

// scriptPipCounts is scriptNeedCounts as the fee-stall prune's bound
// (feeOnlyAskPrune; dirac's filtUsefulCost on the same play): the searched
// play's base cost's fixed pips per colour, ok false when the cost has a
// part whose payment any colour may help beyond a generic one (an X, snow,
// forage or hybrid-family pip -- a generic pip is fine, payable with any
// colour) or that the base cost does not announce (scriptPlayBaseCost: a
// non-"mayplay" mode or an alternative cost). The set may be empty (an
// all-generic cost), and
// the prune then drops every colour ask answer of a fee-only repeatable gap:
// a conversion that never raises the spendable total cannot help pay it.
func scriptPipCounts(r scriptNeedReader, o *state.Object, p state.PlayerID, a decision.PotentialAction) ([state.MC + 1]int32, bool) {
	var counts [state.MC + 1]int32
	c, ok := scriptPlayBaseCost(r, o, p, a)
	if !ok {
		return counts, false
	}
	if c.X != 0 || c.Snow != 0 || c.Forage ||
		len(c.Hybrid) != 0 || len(c.Phyrexian) != 0 || len(c.Twobrid) != 0 || len(c.HybridPhyrexian) != 0 {
		return counts, false
	}
	for i, n := range c.Colored {
		if n > 0 && i <= state.MC {
			counts[i] = n
		}
	}
	return counts, true
}

// run searches from a clone of e; found reports a path (possibly empty,
// when the play is already payable at the root). e is threaded through the
// search as pool, the live engine whose hypothetical-clone pool backs the
// search's nested witness checks (hypclone.go): the struct holds no engine,
// so its methods stay off the engine's surface.
func (sr *scriptSearch) run(e *Engine) ([]ScriptStep, bool) {
	// The search's clones pose priority asks whose offer walks must price
	// only the plain planner witness (scriptSearching's gate, autoPayCensus):
	// the census arm's own prefix search inside a search is the recursion
	// the node budgets cannot bound, and its offers are not the prefix this
	// search is out to find.
	prev := e.scriptSearching
	e.scriptSearching = true
	defer func() { e.scriptSearching = prev }()
	root := e.hypClone(e)
	found := sr.visit(e, root, 0, false)
	e.hypRelease(root)
	if !found {
		return nil, false
	}
	return append([]ScriptStep(nil), sr.path...), true
}

// ownTapSource reports whether id is the source of the searched activated
// ability and that ability's cost taps it: activating id for mana first
// leaves the play's own {T} unpayable, so no continuation of that prefix
// can pay the play (Katilda, Dawnhart Prime's granted mana ability before
// her own "{4}{G}{W}, {T}" counters ability). The node pricing would still
// call it payable (planAbilityPayment withholds the source from the census
// but does not read its tapped state), ending the search in a dead-end
// script that hides every other prefix.
func (sr *scriptSearch) ownTapSource(c *Engine, id state.ObjID) bool {
	if sr.play.Kind != optAbility || id != sr.play.Obj {
		return false
	}
	o := c.G.Obj(id)
	if o == nil || o.Face() == nil {
		return false
	}
	pa, ok := o.PileAbilityAt(sr.play.Ability)
	return ok && pa.SA != nil && c.abilityCompiledCost(pa.SA).Cost.Tap
}

// tappedOwnSource reports whether the path has tapped the source of the
// searched ability whose cost taps it. An ask's answer that does so is skipped
// before it is tried (tapsOwnSource); a tap cost with exactly as many
// candidates as it needs asks nothing and taps them all, Guy in the Chair
// among them when Supportive Parents' second activation has only him and one
// other creature left, and the node pricing would still call the play payable.
func (sr *scriptSearch) tappedOwnSource(c *Engine) bool {
	if sr.play.Kind != optAbility {
		return false
	}
	o := c.G.Obj(sr.play.Obj)
	return o != nil && o.Tapped && sr.ownTapSource(c, sr.play.Obj)
}

// leftOwnZone reports whether the path has moved the source of the searched
// ability out of the zone it was activated from: a granted sacrifice-only
// mana ability (Lotus Ring's) activated on the Equipment that is itself a
// creature token (a Lost Jitte copy the Ring equips) sacrifices the very
// permanent whose Equip is the play, so no continuation offers it. The
// search tries the next source (another creature the Ring equips) instead of
// ending in a dead-end script that hides every other prefix.
func (sr *scriptSearch) leftOwnZone(c *Engine) bool {
	if sr.play.Kind != optAbility {
		return false
	}
	o := c.G.Obj(sr.play.Obj)
	return o == nil || o.Zone != sr.ownZone
}

// tapsOwnSource reports whether answer in to the activation's ask taps the
// source of the searched ability as the cost of the mana ability just
// activated (Gene Pollinator's "tap an untapped permanent you control" chosen
// as Nutrient Block): the play's own {T} is then unpayable, and the node
// pricing, which does not read the tapped state, would still call it payable.
func (sr *scriptSearch) tapsOwnSource(c *Engine, d *decision.Decision, in decision.Intent) bool {
	for _, ch := range in.Choices {
		for i := range d.Options {
			if o := &d.Options[i]; o.Index == ch && o.Kind == "tapcost" && sr.ownTapSource(c, o.Obj) {
				return true
			}
		}
	}
	return false
}

// waitsForStack reports whether the searched play is sorcery-speed and the
// path has left an object of its own on c's stack (Hemosymbic Mite's tap
// trigger, from the tapped permanent Gene Pollinator's cost chose): the
// planner prices the play, but it is not offered until that object resolves
// (CR 307.1, 602.5d), so the line continues through a pass.
func (sr *scriptSearch) waitsForStack(c *Engine) bool {
	if len(c.G.Stack) <= sr.stack {
		return false
	}
	o := c.G.Obj(sr.play.Obj)
	if o == nil || o.Face() == nil {
		return false
	}
	if sr.play.Kind == optAbility {
		pa, ok := o.PileAbilityAt(sr.play.Ability)
		if !ok || pa.SA == nil {
			return false
		}
		sa := pa.SA
		return sa.ParamStr(cards.PKSorcerySpeed) == "True"
	}
	if sr.play.Kind == "cast" && sr.play.Mode == "" {
		return !c.spellTimingOK(sr.p, sr.play.Obj, o.Face(), false)
	}
	return false
}

// targetsGone reports whether the path has left the searched play with no
// legal target: a sacrificed mana source that was the only creature an
// activation could target (Collector's Cage after its controller sacrifices
// the creature carrying a granted sacrifice-for-mana ability). The pool
// prices the play, but it cannot be activated or cast without a target (CR
// 601.2c, 602.2b), so the line continues (a dies trigger may make a new
// target) instead of ending in a script the play cannot follow.
func (sr *scriptSearch) targetsGone(c *Engine) bool {
	o := c.G.Obj(sr.play.Obj)
	if o == nil || o.Face() == nil {
		return false
	}
	switch {
	case sr.play.Kind == optAbility:
		pa, ok := o.PileAbilityAt(sr.play.Ability)
		return ok && pa.SA != nil && !c.abilityTargetsAvailable(sr.p, sr.play.Obj, pa.SA)
	case sr.play.Kind == "cast" && sr.play.Mode == "":
		return !c.castTargetsAvailable(sr.p, sr.play.Obj, o.Face().SpellAbility())
	}
	return false
}

// presentGone reports whether the path has broken the searched activated
// ability's IsPresent$ gate ("Activate only if you control a Sliver", Sliver
// Hive): a prefix that sacrifices the only Sliver for Phyrexian Tower's mana
// leaves the pool pricing the play, but the offer gate no longer lists it
// (CR 602.5b, abilityPresentHolds). The line continues instead of ending in
// a script the play cannot follow, so the branch that sacrifices another
// creature is searched.
func (sr *scriptSearch) presentGone(c *Engine) bool {
	if sr.play.Kind != optAbility {
		return false
	}
	o := c.G.Obj(sr.play.Obj)
	if o == nil || o.Face() == nil {
		return false
	}
	pa, ok := o.PileAbilityAt(sr.play.Ability)
	return ok && pa.SA != nil && !c.abilityPresentHolds(sr.p, sr.play.Obj, pa.SA)
}

// unattachedSource reports whether the path has detached the source of the
// searched Reconfigure unattach ability (the prefix sacrificed its bearer):
// the offer gate withholds an Unattach$ ability while its source is
// unattached (CR 702.151a: "unattach from a creature" has no legal action
// there), but the node pricing below reads only the mana, so the search
// would end in a dead-end script and hide every other prefix (here one that
// sacrifices some other creature for Ashnod's Altar's mana).
func (sr *scriptSearch) unattachedSource(c *Engine) bool {
	if sr.play.Kind != optAbility {
		return false
	}
	o := c.G.Obj(sr.play.Obj)
	if o == nil || o.Face() == nil {
		return false
	}
	pa, ok := o.PileAbilityAt(sr.play.Ability)
	if !ok || pa.SA == nil {
		return false
	}
	return pa.SA.CompiledAPI() == cards.APIAttach && effects.AttachOf(pa.SA).Unattach && o.AttachedTo == 0
}

func (sr *scriptSearch) offered(o *decision.Option) bool {
	if o.CensusOffer {
		// The census-gap arm's script-backed offer is not the plain listing
		// the search's probe prices (the kernel's own listing probe excludes
		// the arm's listings, actScriptPays' availNoActScript): at the
		// search's root it is the very offer the probe's caller is out to
		// prove, and taking it for the plain walk's would answer "already
		// offered" before any prefix paid for the play.
		return false
	}
	a := sr.play
	if sr.printedOnly && (o.SVar != "" || o.GainedSource != 0) {
		return false
	}
	if sr.instanced && o.Cost != sr.instance {
		return false
	}
	return o.Kind == a.Kind && o.Obj == a.Obj && o.Ability == a.Ability && o.Mode == a.Mode && (sr.altCost < 0 || o.AltCostIndex == sr.altCost)
}

// childOffers reports whether the play is already offered at child's
// priority decision cd (visit's scan): the visited-state key is a colour-only
// state, and a priced state can key equal to an earlier unpriced sibling (a
// restricted unit beside a filter's converted one), so the hoisted dedup must
// not skip the offered node. The phased run's scope is visit's: an offer the
// fallback does not take is not a reason to keep the node.
func (sr *scriptSearch) childOffers(d *decision.Decision, c *Engine) bool {
	for i := range d.Options {
		if !sr.offered(&d.Options[i]) {
			continue
		}
		if sr.phasedOutOfScope(c) {
			continue
		}
		return true
	}
	return false
}

// priced reports whether, at c's payer priority, the planner pays the play
// from the floating pool (a witness that also reaches it); a verdict that
// is neither a witness nor a proof marks the search unproven.
func (sr *scriptSearch) priced(pool *Engine, c *Engine) bool {
	defer pay.PaymentPlanQueryEnd(asPayer(c), pay.PaymentPlanQueryBegin(asPayer(c)))
	o := sr.option()
	got, _ := c.potentialPlayVerdict(sr.p, o)
	// Only a search that leans on these verdicts for its proof (nodePlans)
	// is weakened by one that proves nothing; the full search's proof is
	// its own exhaustion.
	if got.Reason != "" || got.Plan == nil {
		if got.Reason != "insufficient" && sr.nodePlans {
			sr.unproven = true
		}
		return false
	}
	if sr.phasedOutOfScope(c) {
		return false // the fallback's scope: a consuming cost, a hybrid-fee gap, or a plain fee-only budget exhaustion
	}
	if paymentPlanConsumes(*got.Plan) && !c.witnessReachesMode(pool, sr.p, o, *got.Plan, true, true) {
		// winOK: a payment whose own sacrifice trigger ends the game in the
		// payer's favour still reaches the play (CR 104.3, 603.3b), exactly
		// as the plan offer counts it (payment_plan.go's witnessReachesMode
		// call). Without it the replay rejects the winning line and the
		// prefix search drops a legal cast (dr-mbc-bbba, Briarblade Adept at
		// decision 449: the Havoc Jester trigger from a Treasure sacrifice is
		// lethal and every branch then fails).
		if sr.nodePlans {
			sr.unproven = true
		}
		return false
	}
	return true
}

// option is sr's play as the planner's option.
func (sr *scriptSearch) option() decision.Option {
	return decision.Option{Kind: sr.play.Kind, Obj: sr.play.Obj, Ability: sr.play.Ability, Mode: sr.play.Mode, AltCostIndex: max(sr.altCost, 0)}
}

// phasedOutOfScope reports whether the phased fallback must leave this play
// to the plain order: only a play phasedScope prices is in scope, unless the
// plain run exhausted the node budget on a repeatable fee-only filter
// (plainLimited), because the phased order is a different node walk the
// kernel's plain-search mirror does not price elsewhere. A consuming cost or
// a hybrid-fee gap is always in scope (the prefix may have activated the
// permanent the payment must sacrifice, and a hybrid fee is floated first).
func (sr *scriptSearch) phasedOutOfScope(c *Engine) bool {
	return sr.phased && !sr.phasedScope(c) && !sr.plainLimited
}

// phasedScope reports whether the phased order may price sr's play: one whose
// own cost sacrifices or returns a permanent (CR 601.2b: the activation comes
// before the sacrifice), or one whose census gaps include a hybrid-fee mana
// ability (Fetid Heath's "{W/B}, {T}: Add {W}{W}, {W}{B} or {B}{B}"). A hybrid
// fee is paid only from mana already floating (CR 601.2g: the filter's
// activation spends the fee as it resolves), so the phased order's
// covered-funding-then-fee-carrying-gap walk is the one that floats the fee
// first. For any other play the plain order is the script dirac's
// node-for-node mirror prices, so the fallback must leave its offer as the
// plain search found it.
func (sr *scriptSearch) phasedScope(c *Engine) bool {
	if sr.feeOrder {
		return true
	}
	o := sr.option()
	obj := c.G.Obj(o.Obj)
	if o.Kind != "cast" || obj == nil || obj.Face() == nil {
		return false
	}
	base, ok := c.potentialModeBaseCost(sr.p, o.Obj, obj.Face(), o)
	if ok && (len(base.Sac) != 0 || len(base.Return) != 0) {
		return true
	}
	for _, id := range sr.gaps {
		g := c.G.Obj(id)
		if g == nil || g.Face() == nil {
			continue
		}
		for _, ma := range g.Face().Abilities {
			if ma.API == "Mana" && len(c.manaAbilityCC(sr.p, id, ma).Cost.Hybrid) != 0 {
				return true
			}
		}
	}
	return false
}

// try submits in to a clone of c and continues the search there. gapUsed
// records that the path activated an uncovered source (or let the stack
// resolve): only then can the planner's verdict differ from the root's.
func (sr *scriptSearch) try(pool *Engine, c *Engine, d *decision.Decision, in decision.Intent, funded int, gapUsed bool) bool {
	child := pool.hypClone(c)
	found := false
	if child.Submit(in) == nil {
		// The visited-state check is hoisted here (cut G): a duplicate
		// payer state is not a node of the search. visit's own check stays
		// (the root and any child whose key it reaches first still mark it),
		// and the seen set only ever holds states an earlier offered scan
		// did not find the play in, so skipping before that scan cannot lose
		// an offer. The budget is potentialScriptNodes *distinct* states.
		if cd := child.Pending(); sr.feeOnly && cd != nil && cd.Kind == decision.KPriority && cd.Player == sr.p && !sr.childOffers(cd, child) && sr.seen[scriptStateKey(child, sr.p)] {
			pool.hypRelease(child)
			return false
		}
		sr.path = append(sr.path, scriptStepOf(d, in))
		if found = sr.visit(pool, child, funded, gapUsed); !found {
			sr.path = sr.path[:len(sr.path)-1]
		}
	}
	// The path holds plain values (scriptStepOf), so the clone is dead
	// either way.
	pool.hypRelease(child)
	return found
}

func (sr *scriptSearch) visit(pool *Engine, c *Engine, funded int, gapUsed bool) bool {
	if sr.nodes >= sr.budget {
		sr.limited = true
		return false
	}
	sr.nodes++
	p := sr.p
	d := c.Pending()
	// Another player's decision (their priority while a trigger resolves,
	// or the order of their simultaneous triggers the line set off, CR
	// 603.3b): the searched line assumes they pass, or order the triggers
	// as offered. Any other ask of another player dead-ends the line.
	for guard := 0; d != nil && d.Player != p && guard < 8; guard++ {
		sr.approx = true
		if d.Kind == decision.KTriggerOrder {
			answers := scriptAnswers(d)
			if len(answers) == 0 || c.Submit(answers[0]) != nil {
				return false
			}
			d = c.Pending()
			continue
		}
		pass := -1
		for _, opt := range d.Options {
			if opt.Kind == "pass" {
				pass = opt.Index
			}
		}
		if d.Kind != decision.KPriority || pass < 0 || c.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}) != nil {
			return false
		}
		d = c.Pending()
	}
	if d == nil || d.Player != p || c.G.Over || c.G.Turn != sr.turn || c.G.Step != sr.step {
		return false
	}
	if sr.tappedOwnSource(c) {
		return false // a forced tap (no ask) of the play's own {T} source: no continuation pays it (CR 602.2b)
	}
	if sr.leftOwnZone(c) {
		return false // the prefix sacrificed the play's own source: the play is gone, whichever way the rest pays (CR 602.2b)
	}
	if d.Kind != decision.KPriority {
		// An ask the activation poses: branch over its answers.
		answers := scriptAnswers(d)
		if scriptManaAskHoists(asPayer(c), c.manaActivation, p, d) {
			answers = scriptOrderManaAbilityAnswers(c, d, answers, c.manaActivation, sr.need)
			answers = scriptOrderAnswers(d, answers, sr.need)
		}
		// A mana ability that yields nothing is no branch (an idle storage land's
		// tap stays one: gorge's search takes it and the kernel's mirrors that,
		// mbc_idle_storage_tap).
		answers = scriptDropEmptyManaAnswers(c.manaActivation, p, d, answers, func(ab *cards.SA) bool {
			if _, storage := announcedSourceSubCounter(c.abilityCompiledCost(ab).Cost); storage {
				return false
			}
			o := c.G.Obj(d.Source)
			return o != nil && o.Face() != nil && c.manaAbilityYieldsNothing(p, d.Source, o.Face(), ab)
		})
		if sr.subsume {
			answers = sr.subsumeAnswers(c, d, answers)
		}
		if sr.feeStall {
			answers = sr.feeOnlyAskPrune(c, d, answers)
			if sr.feeOnly {
				answers = sr.oneShotAskPrune(c, d, answers)
			}
		}
		if len(answers) == 0 {
			sr.approx = true
		}
		for _, in := range answers {
			if sr.tapsOwnSource(c, d, in) {
				continue // the ask taps the play's own {T} cost source (CR 602.2b)
			}
			if sr.try(pool, c, d, in, funded, gapUsed) {
				return true
			}
		}
		return false
	}
	for i := range d.Options {
		if sr.offered(&d.Options[i]) {
			if sr.phasedOutOfScope(c) {
				continue // the fallback's scope: a consuming cost, a hybrid-fee gap, or a plain fee-only budget exhaustion
			}
			return true
		}
	}
	key := scriptStateKey(c, p)
	if sr.seen[key] {
		return false
	}
	sr.seen[key] = true
	gaps := sr.gaps
	if sr.nodePlans {
		// The census misses at THIS node (a creature Saruli Caretaker just
		// tapped still has its sacrifice ability, outside a census that
		// lists untapped sources only): those are branched on here, so the
		// planner's "insufficient" below covers every other continuation.
		gaps = c.scriptNodeGaps(p)
	}
	if (gapUsed || sr.nodePlans) && !sr.waitsForStack(c) && !sr.targetsGone(c) && !sr.unattachedSource(c) && !sr.presentGone(c) && sr.priced(pool, c) {
		return true
	}
	if sr.activations(pool, c, d, gaps, p, funded, gapUsed) {
		return true
	}
	if sr.passes && len(c.G.Stack) > sr.stack {
		// A trigger an activation caused: let it resolve, the pool floats.
		// (An object already on the stack at the decision is not the
		// search's to resolve: the play is then a later decision's.)
		for i := range d.Options {
			if d.Options[i].Kind == "pass" {
				return sr.try(pool, c, d, decision.Intent{Seq: d.Seq, Player: p, Choices: []int{d.Options[i].Index}}, funded, true)
			}
		}
	}
	return false
}

// potentialTwinCut reports whether the twin cut (scriptTwin) runs for the
// potential play o on the plain walk: a printed activation (no SVar grant and
// no gained anchor) or a gained one (GainedSource, the 0xC0 encoding). It is
// the gorge half of dirac's isAbilityPlay: an SVar grant is also Kind
// "ability" here, but dirac gives it the same 0x80|ability encoding as the
// max-speed and exile grants it cannot tell it apart from, so those keep the
// older walk on both sides.
func potentialTwinCut(o decision.Option) bool {
	if o.Kind != optAbility {
		return false
	}
	return o.GainedSource != 0 || (o.SVar == "" && o.GrantSource == 0)
}

// scriptTwin reports whether a and b are indistinguishable mana sources for
// the search's purposes: the same printed face, both untapped, no counters
// and nothing attached to either, and the same activate options on the
// current decision (a granted ability a continuous effect gives one and not
// the other is not a twin). Two such sources' activation subtrees are
// isomorphic -- the same mana for the same cost -- so the branch through the
// later one reaches no payer state the earlier one's does not, and the
// search skips it (the kernel's filtSearch.twinTried on the same pair). The
// skip runs on the fee-only path and for a printed or gained activated play
// (potentialTwinCut; an SVar grant is Kind "ability" too, but dirac encodes
// it like a max-speed grant and walks it, so the cut must not run): a
// fee-carrying gap beside a row of identical basics otherwise spends the
// budget ordering the basics and never prices the ability (tail-26 pair 26).
// A cast outside the fee-only path also keeps the older walk, where the cast
// tables price its node count.
func scriptTwin(g *state.Game, d *decision.Decision, a, b state.ObjID) bool {
	if a == b || a == 0 || b == 0 {
		return false
	}
	oa, ob := g.Obj(a), g.Obj(b)
	if oa == nil || ob == nil || oa.Face() == nil || ob.Face() == nil {
		return false
	}
	if oa.Face().Name != ob.Face().Name || oa.Tapped || ob.Tapped || oa.AttachedTo != 0 || ob.AttachedTo != 0 {
		return false
	}
	// Battlefield sources alone: the kernel's twin cut reads battlefield slots
	// and its hand pass (handFund) walks every guide. Two hand sources are not
	// interchangeable for the search either -- one may be the searched play's
	// own card, whose exile ends the play -- so the second guide is a line of
	// its own (the visited set still drops an equal state it does not price).
	if oa.Zone != state.ZBattlefield || ob.Zone != state.ZBattlefield {
		return false
	}
	if oa.Counters != nil || ob.Counters != nil {
		return false
	}
	// Every seat's battlefield: the attachment may belong to the opponent
	// (a Sea's Claim on the payer's land), and a twin pair one attachment
	// touches is two subtrees.
	for seat := range g.Players {
		for _, id := range g.Zone(state.ZBattlefield, state.PlayerID(seat)) {
			if o := g.Obj(id); o != nil && (o.AttachedTo == a || o.AttachedTo == b) {
				return false
			}
		}
	}
	return scriptOptionKey(d, a) == scriptOptionKey(d, b)
}

// scriptManaAbilities is the narrow slice of the engine the twin cut's
// soundness needs: one source's priority mana-ability member set
// (mana_activation.go's activateManaFor resolves from it, the ask the
// search's clone walks), built with the same nil statics a priority
// activation reads.
type scriptManaAbilities interface {
	appendAvailableManaAbilities(buf []*cards.SA, statics *actionStaticSource, p state.PlayerID, id state.ObjID) []*cards.SA
}

// scriptManaTwins reports whether two sources scriptTwin already called
// twins also resolve from the same priority mana-ability member set,
// multiset equality under pay.SameManaAbility. The ask offers one activate
// option a source, so equal options cannot see an ability a continuous
// effect granted one and not the other (CR 305.6: a basic land type's mana
// abilities come with the type -- a Plains this turn animated into an
// Island taps for {U} beside an unanimated Plains), and collapsing the pair
// walks only the earlier source's subtree: the later one's colour ask, the
// one the prefix's fee funding needs, is never branched.
func scriptManaTwins(r scriptManaAbilities, p state.PlayerID, a, b state.ObjID) bool {
	as := r.appendAvailableManaAbilities(nil, nil, p, a)
	bs := r.appendAvailableManaAbilities(nil, nil, p, b)
	if len(as) != len(bs) {
		return false
	}
	for _, x := range as {
		found := false
		for _, y := range bs {
			if pay.SameManaAbility(x, y) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// scriptActivate reports whether o is an "activate" option: a mana ability the
// payer can activate at priority, the search's one branching move.
func scriptActivate(o *decision.Option) bool { return o.Kind == "activate" }

// scriptOptionKey is a source's activate-option signature at d: the labels,
// symbols and ability indices the decision offers for it, in engine order.
func scriptOptionKey(d *decision.Decision, id state.ObjID) string {
	var b strings.Builder
	for i := range d.Options {
		o := &d.Options[i]
		if !scriptActivate(o) || o.Obj != id {
			continue
		}
		fmt.Fprintf(&b, "%s|%s|%d;", o.Label, o.ManaSymbol, o.Ability)
	}
	return b.String()
}

// activations is the visit activation loop: sr's children at c, one try an
// option. The plain order is the one dirac's mirror prices plays by: the
// uncovered sources first, then the others (under gapsOnly, while funding
// lasts: a covered source tapped to pay a filter's fee). The phased order
// (potentialPrefixScript's fallback run) branches the uncovered sources
// whose mana abilities cost no mana first (their mana is the play's for one
// step, so the search need not wait), then the covered ones, then the
// uncovered ones whose abilities themselves cost mana: their fee is paid
// from the floating pool (CR 601.2b), so the funding floats come before
// them or their activation drains the pool the prefix still needs (a
// repeatable "{2}: Add one mana of any color" beside three untapped Swamps
// never lets the pool reach the {2} its last activation should spend on the
// play's pip). The phased run also gates every candidate on realPayable,
// which the plain run may not: the gate's prunings change the node walk,
// and with it the script the mirror sees.
func (sr *scriptSearch) activations(pool *Engine, c *Engine, d *decision.Decision, gaps []state.ObjID, p state.PlayerID, funded int, gapUsed bool) bool {
	// The phases, in walk order: a plain run is the two original passes,
	// the uncovered sources then the others; a phased run branches the
	// uncovered sources whose mana abilities cost no mana first (their mana
	// is the play's for one step, so the search need not wait), then the
	// covered ones (the funding), then the uncovered ones whose abilities
	// themselves cost mana: their fee is paid from the floating pool
	// (CR 601.2b), so the funding floats come before them or their
	// activation drains the pool the prefix still needs (a repeatable
	// "{2}: Add one mana of any color" beside three untapped Swamps never
	// lets the pool reach the {2} its last activation should spend on the
	// play's pip). free marks the phases that want a fee-free ability,
	// gate the ones that drop a source the payer cannot legally activate
	// at the pool so far (the phased run only: the gate's prunings change
	// the node walk, and with it the script dirac's mirror prices plays by).
	n := 2
	if sr.phased {
		n = 3
	}
	if sr.feeOrder {
		n = 3
	}
	for i := 0; i < n; i++ {
		ph := sr.phaseAt(i)
		if !ph.gap && sr.gapsOnly && funded >= sr.funding {
			if sr.feeOrder {
				continue // the funding floated: the fee gaps that follow pay their fees from it
			}
			break // under gapsOnly, while funding lasts: a covered source tapped to pay a filter's fee
		}
		var tried []state.ObjID
		for j := range d.Options {
			opt := &d.Options[j]
			if !scriptActivate(opt) {
				continue
			}
			gap := scriptGap(gaps, opt.Obj)
			if gap != ph.gap {
				continue
			}
			if sr.feeOrder && gap {
				if ph.feeOnly != sr.feeGaps[opt.Obj] {
					continue
				}
			}
			if ph.gate || ph.free {
				real, free := sr.payable(c, opt.Obj)
				if ph.gate && !real {
					continue // the payer cannot legally activate it at the pool so far (CR 601.2b)
				}
				if ph.free && !free {
					continue
				}
			}
			if sr.ownTapSource(c, opt.Obj) {
				continue // its mana would tap the play's own {T} cost source (CR 602.2b)
			}
			twin := false
			if sr.feeOnly || sr.twinCut {
				for _, t := range tried {
					// scriptManaTwins after scriptTwin: the ask offers one mana option a
					// source, so equal options cannot see a second ability a continuous
					// effect granted one and not the other (an animated dual-type land's
					// second tap), whose activation ask the earlier twin's subtree never
					// poses.
					if scriptTwin(c.G, d, t, opt.Obj) && scriptManaTwins(c, p, t, opt.Obj) {
						twin = true
						break
					}
				}
			}
			if twin {
				continue // an indistinguishable earlier source already walked this subtree
			}
			if !gap && !sr.coveredSpendable(c, p, opt.Obj) {
				continue // a covered source whose mana the play cannot spend is a dead funding branch
			}
			if gap && sr.feeStall && sr.feeOnly {
				if sr.actDescOK {
					sr.actPool = pay.AvailableFor(asPayer(c), p, sr.actDesc).Pool
				} else {
					sr.actPool = c.G.Players[p].Pool
				}
				if sr.feeOnlyGapDoomed(c, opt.Obj) {
					continue // only fee-only repeatable activations, none of whose colours can help
				}
			}
			if !gap && !c.windowSourceYields(p, opt.Obj) {
				// A covered source whose every mana ability provably adds
				// nothing (Bag End Banquet with no Food) cannot fund a fee:
				// tapping it only changes the tapped bit, and each such step
				// spends a node, and every node under it, of the budget
				// (the kernel's funding pass never taps it).
				continue
			}
			tried = append(tried, opt.Obj)
			next := funded
			if !gap {
				next++
			} else {
				// The gap activation's opening pool was stashed above for the
				// fee-stall run; its source is what the ask's prune reads.
				sr.actSrc = opt.Obj
			}
			if sr.try(pool, c, d, decision.Intent{Seq: d.Seq, Player: p, Choices: []int{opt.Index}}, next, gapUsed || gap) {
				return true
			}
		}
	}
	return false
}

// coveredSpendable reports whether a covered source's mana can help this
// search: an unrestricted ability always can, a restricted one only when
// its RestrictValid$ admits the searched play (the same test
// scriptSpendableGaps applies to the census's gaps) or a census gap's
// activation fee. A covered source whose every ability makes mana the play
// can spend nowhere is a dead funding branch (CR 106.6): tapping it changes
// only the tapped bit and spends one node of the bounded search. The proof
// run keeps it too: its gaps are per-node (visit's scriptNodeGaps), so a
// source no census gap's fee can use may still fund a deeper node's gap
// fee, and its completed verdict is returned as an insufficient proof --
// the walk it proves over is pruned nowhere. The test otherwise runs for
// plain casts only -- an activation's search keeps today's node walk -- and
// a creature is never pruned, since it may still be tapped as a cost
// (tapXType<Creature>, convoke, crew). A tap that fires another permanent's
// mana trigger (floating mana the search reaches only through that later
// node) is not accounted for here: no corpus card demonstrates it, so the
// next divergence of this class starts there.
func (sr *scriptSearch) coveredSpendable(c *Engine, p state.PlayerID, id state.ObjID) bool {
	if sr.nodePlans {
		return true // the proof run's walk is pruned nowhere
	}
	if sr.play.Kind != potentialCastKind {
		return true
	}
	if o := c.G.Obj(id); o != nil && o.Face() != nil && o.Face().IsCreature() {
		// A creature can still be tapped as a cost (Holdout Settlement's
		// tapXType<Creature>, convoke, crew): its restricted mana being
		// unspendable does not make the source a dead branch.
		return true
	}
	for _, ma := range c.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
		if effects.ManaOf(ma).RestrictValid == "" {
			return true
		}
	}
	o := decision.Option{Kind: sr.play.Kind, Obj: sr.play.Obj, Ability: sr.play.Ability, Mode: sr.play.Mode, AltCostIndex: max(sr.altCost, 0)}
	if len(c.scriptSpendableGaps(p, o, []state.ObjID{id})) > 0 {
		return true
	}
	// ... but a restricted covered source can still pay a census gap's
	// activation fee even when the play itself cannot spend it (Vhal's
	// "{C} ... can't be spent to cast a spell from hand" pays School of the
	// Unseen's {2}, whose mana the cast spends, CR 106.6). The fee is an
	// activation of the gap, not the play, so test each gap source.
	for _, gid := range sr.gaps {
		if gid == id {
			continue
		}
		if c.gapSpendableFor(p, id, paymentDescriptor{Class: pay.PurposeActivated, ID: gid}) {
			return true
		}
	}
	return false
}

// scriptPhase is one pass of the visit activation loop over d's options:
// the uncovered sources (gap) or the others, a fee-free ability only
// (free), gated on a legally activatable one (gate).
type scriptPhase struct {
	gap, free, gate bool
	// feeOnly is set on a feeOrder pass: the pass walks the fee-carrying
	// gaps when true and the fee-free ones when false.
	feeOnly bool
}

// scriptPhaseAt is pass i of the visit activation loop: the plain run's
// two passes, the uncovered sources then the others; the phased run's
// three, the fee-free gaps, the covered funding, then the fee-carrying
// gaps.
func (sr *scriptSearch) phaseAt(i int) scriptPhase {
	if sr.feeOrder {
		switch i {
		case 0:
			return scriptPhase{gate: true}
		case 1:
			return scriptPhase{gap: true, gate: true, feeOnly: true}
		}
		return scriptPhase{gap: true, gate: true}
	}
	return scriptPhaseAt(sr.phased, i)
}

func scriptPhaseAt(phased bool, i int) scriptPhase {
	if !phased {
		if i == 0 {
			return scriptPhase{gap: true}
		}
		return scriptPhase{}
	}
	switch i {
	case 0:
		return scriptPhase{gap: true, free: true, gate: true}
	case 1:
		return scriptPhase{gate: true}
	}
	return scriptPhase{gap: true, gate: true}
}

// payable reports whether the payer can legally activate a mana ability of
// id at c's floating pool (manaAbilityPayablePool's real-pool gate, hyp
// nil), and whether one of those abilities costs no mana: a bare {T}, a
// sacrifice. The search's steps are plays the payer must be able to make:
// the potential bound that admits a paid ability at an empty pool elsewhere
// in the planner is not a play, and walking one spends the node budget on
// an activation whose mana fee drains the pool the prefix still needs (a
// repeatable "{2}: Add one mana of any color" gap beside three untapped
// Swamps never lets the pool reach the {2} its last activation should spend
// on the play's pip). A fee-free source's mana is the play's for one step,
// so the search need not fund it and tries it before the funding floats.
func (sr *scriptSearch) payable(c *Engine, id state.ObjID) (real, free bool) {
	for _, ma := range c.appendAvailableManaAbilitiesGate(nil, nil, sr.p, id, true) {
		if !c.manaAbilityPayablePool(sr.p, id, ma, nil) {
			continue
		}
		real = true
		cc := c.manaAbilityCC(sr.p, id, ma)
		noFee := cc.Cost.Generic == 0 && cc.Cost.X == 0 && cc.Cost.Snow == 0 &&
			len(cc.Cost.Hybrid) == 0 && len(cc.Cost.Phyrexian) == 0 &&
			len(cc.Cost.Twobrid) == 0 && len(cc.Cost.HybridPhyrexian) == 0
		for _, n := range cc.Cost.Colored {
			noFee = noFee && n == 0
		}
		if noFee {
			return true, true
		}
	}
	return real, false
}

// scriptNodeGaps is the census's missed sources for p at e (a search
// clone: it is left in the potential-pool mode).
func (e *Engine) scriptNodeGaps(p state.PlayerID) []state.ObjID {
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	defer pay.PaymentPlanQueryEnd(asPayer(e), pay.PaymentPlanQueryBegin(asPayer(e)))
	hyp := e.PotentialMana(p)
	return e.paymentPlanCensusOf(p, &hyp).gaps
}

func scriptGap(gaps []state.ObjID, id state.ObjID) bool {
	for _, g := range gaps {
		if g == id {
			return true
		}
	}
	return false
}

// scriptStepOf records answer in of d as a ScriptStep.
func scriptStepOf(d *decision.Decision, in decision.Intent) ScriptStep {
	st := ScriptStep{Kind: d.Kind}
	for _, c := range in.Choices {
		for i := range d.Options {
			if o := &d.Options[i]; o.Index == c {
				st.Picks = append(st.Picks, ScriptPick{Kind: o.Kind, Obj: o.Obj, Label: o.Label, ManaSymbol: o.ManaSymbol, Ability: o.Ability})
				break
			}
		}
	}
	return st
}

// scriptAnswers enumerates the answers to an activation's ask that
// validate: every single option (and the empty answer when Min is 0), for a
// Min == Max == n object election every n-combination of its options, for a
// Min == Max == n unit-major colour allocation every per-unit combination,
// and for a range ask (Min < Max, e.g. "return up to two target ... cards")
// the combinations of every size from Min to Max (each at most 64).
func scriptAnswers(d *decision.Decision) []decision.Intent {
	var out []decision.Intent
	mk := func(ch []int) decision.Intent { return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: ch} }
	switch {
	case d.Max <= 1:
		if d.Min == 0 {
			out = append(out, mk(nil))
		}
		for _, o := range d.Options {
			out = append(out, mk([]int{o.Index}))
		}
	case d.Min == d.Max && !scriptManaUnits(d):
		out = scriptSubsets(d, d.Min, mk)
	case d.Min == d.Max && len(d.Options)%d.Min == 0:
		n, per := d.Min, len(d.Options)/d.Min
		idx := make([]int, n)
		for len(out) < 64 {
			ch := make([]int, n)
			for u := 0; u < n; u++ {
				ch[u] = d.Options[u*per+idx[u]].Index
			}
			out = append(out, mk(ch))
			u := n - 1
			for ; u >= 0; u-- {
				if idx[u]++; idx[u] < per {
					break
				}
				idx[u] = 0
			}
			if u < 0 {
				break
			}
		}
	case d.Min < d.Max && !scriptManaUnits(d):
		// A range ask ("up to two target ... cards", CR 601.2c): the
		// combinations of every size the ask admits, each capped at 64. A
		// unit-major colour allocation is not a subset shape -- its sizes
		// mix units -- so it keeps today's (empty) answer set.
		for n := d.Min; n <= d.Max; n++ {
			out = append(out, scriptSubsets(d, n, mk)...)
		}
	}
	valid := out[:0]
	for _, in := range out {
		if d.Validate(in) == nil {
			valid = append(valid, in)
		}
	}
	return valid
}

// scriptManaUnits reports whether d is a unit-major colour allocation: every
// option names a mana symbol, one unit's colours laid out per unit. An object
// election (a tapXType cost, say) has tapcost options with no symbol, so the
// two shapes are told apart without counting options: a multi-object cost's
// candidate count need not be a multiple of the number to choose.
func scriptManaUnits(d *decision.Decision) bool {
	if len(d.Options) == 0 {
		return false
	}
	for i := range d.Options {
		if d.Options[i].ManaSymbol == "" {
			return false
		}
	}
	return true
}

// scriptSubsets enumerates the n-combinations of d's option positions in
// lexicographic order, mapping each position to that option's Index, capped
// at 64 answers (the colour allocation's cap). It returns nil when n > the
// option count.
func scriptSubsets(d *decision.Decision, n int, mk func([]int) decision.Intent) []decision.Intent {
	k := len(d.Options)
	if n > k {
		return nil
	}
	var out []decision.Intent
	idx := make([]int, n)
	for i := range idx {
		idx[i] = i
	}
	for len(out) < 64 {
		ch := make([]int, n)
		for i, j := range idx {
			ch[i] = d.Options[j].Index
		}
		out = append(out, mk(ch))
		i := n - 1
		for ; i >= 0; i-- {
			if idx[i] < k-n+i {
				idx[i]++
				break
			}
		}
		if i < 0 {
			break
		}
		for j := i + 1; j < n; j++ {
			idx[j] = idx[j-1] + 1
		}
	}
	return out
}

// scriptStepNeed is the colours the pending ask's activation produces: the
// witness replay (witnessReachesMode) answers the ask the step's plan
// expected first, scriptOrderAnswers keeping engine order inside each half.
// The ask follows steps[next-1]: the replay submitted it to reach this ask,
// and its Produces are the exact mana the plan counts on it floating. The
// answer set is unchanged; the replay is budget-capped, not complete, and an
// any-colour activation (a Treasure's sacrifice) otherwise spends the budget
// proving every colour the plan does not use dead -- between each activation,
// with the sacrifice's own triggers asking between them (Havoc Jester) --
// before the first one it does.
func scriptStepNeed(steps []decision.PaymentActivation, next int) [state.MC + 1]bool {
	var need [state.MC + 1]bool
	if next <= 0 || next > len(steps) {
		return need
	}
	for c, n := range steps[next-1].Produces {
		if n > 0 && c <= state.MC {
			need[c] = true
		}
	}
	return need
}

// manaOrderEngine is the narrow engine slice scriptOrderManaAbilityAnswers
// reads: the configured production of one mana ability (the same facts the
// payment planner's potential fold uses). A free function over it keeps the
// reorder off the Engine's method surface.
type manaOrderEngine interface {
	addPotentialManaOf(m *state.Mana, ma *cards.SA)
	manaAbilityCC(p state.PlayerID, id state.ObjID, ma *cards.SA) *compiledCost
}

// scriptOrderManaAbilityAnswers is scriptOrderAnswers for the ask a source's
// several mana abilities pose ("choose a mana ability"). The ask's options
// name no mana symbol -- the colour is a second ask -- so scriptOrderAnswers
// alone leaves a granted fixed-colour ability behind the source's printed
// ones, and a search that needs that colour walks the printed subtree first
// (a transformed Ulvenwald Abomination's {C}{C} before the granted {G}).
// Only an answer that picks a granted fixed-colour ability making a needed
// colour is hoisted, in engine order; printed folds (mana_multicolour.go),
// any-colour productions (selColours' second pass) and sacrifice/life
// abilities (needPass) keep the engine's order, which the kernel mirrors.
func scriptOrderManaAbilityAnswers(e manaOrderEngine, d *decision.Decision, answers []decision.Intent, ma *manaActivation, need [state.MC + 1]bool) []decision.Intent {
	if e == nil || ma == nil || ma.source != d.Source || len(ma.abilities) == 0 {
		return answers
	}
	needAny := false
	for _, want := range need {
		if want {
			needAny = true
			break
		}
	}
	if !needAny {
		return answers
	}
	granted := false
	for i := range d.Options {
		if o := &d.Options[i]; o.Kind == "mana" && o.Granted {
			granted = true
			break
		}
	}
	if !granted {
		// Two printed abilities keep the engine's ask order: the kernel's
		// multi-ability fold (mana_multicolour.go) mirrors that order, and a
		// needed-colour hoist there floats a colour the kernel does not
		// (taps_deferred_multi_ability_ask_order). The reorder is for a
		// granted fixed-colour ability standing beside a printed one.
		return answers
	}
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind != "mana" || o.Ability < 0 || o.Ability >= len(ma.abilities) || ma.abilities[o.Ability] == nil {
			continue
		}
		// A sacrifice or life cost is a consequence the kernel's search
		// keeps in the ask's engine order (dirac's needPass): the source
		// is spent, or the payer's life moves, so a needed-colour reorder
		// would take a resource the kernel does not spend.
		if cc := e.manaAbilityCC(ma.player, ma.source, ma.abilities[o.Ability]); cc != nil && (len(cc.Cost.Sac) > 0 || cc.Cost.Life > 0) {
			return answers
		}
	}
	needed := func(in decision.Intent) bool {
		for _, ch := range in.Choices {
			for i := range d.Options {
				o := &d.Options[i]
				if o.Index != ch || o.Kind != "mana" || o.ManaSymbol != "" || !o.Granted ||
					o.Ability < 0 || o.Ability >= len(ma.abilities) || ma.abilities[o.Ability] == nil {
					continue
				}
				mp := effects.ManaOf(ma.abilities[o.Ability])
				if producedOpen(mp.Produced) {
					continue // an any-colour production names no colour: the kernel's ask keeps it for the second pass (dirac's selColours)
				}
				var made state.Mana
				e.addPotentialManaOf(&made, ma.abilities[o.Ability])
				for col, want := range need {
					if want && made[col] > 0 {
						return true
					}
				}
			}
		}
		return false
	}
	var first, rest []decision.Intent
	for _, in := range answers {
		if needed(in) {
			first = append(first, in)
		} else {
			rest = append(rest, in)
		}
	}
	return append(first, rest...)
}

// scriptOrderAnswers stably reorders an ask's valid answers so the ones that
// name a colour the searched play's cost asks for come first, engine order
// kept inside each half. The answer SET is unchanged, but the search is
// budget-capped (potentialScriptNodes), not complete: order decides which
// plans it reaches, and so which plan it finds. A W U B R G colour ask
// otherwise spends its nodes proving every colour the cost does not name dead
// before the first one it does.
//
// The reorder never applies to a plain mana-ability ask whose options cost
// the payer life (scriptManaAskHoists false): there the kernel walks the
// ask's own option order, the source's ability order -- the free printed
// ability before a pay-life one (Haunted Screen's {W}/{B} before its
// {G}/{U}/{R}), the pain-free {C} of a painland before its painful colours
// (Scabland) -- so the script must branch in that same order or the prefix
// it writes takes a life the kernel does not spend.
func scriptOrderAnswers(d *decision.Decision, answers []decision.Intent, need [state.MC + 1]bool) []decision.Intent {
	any := false
	for _, b := range need {
		if b {
			any = true
			break
		}
	}
	if !any {
		return answers
	}
	symNeeded := func(o *decision.Option) bool {
		if len(o.ManaSymbol) != 1 {
			return false
		}
		if i := state.ManaIndex(o.ManaSymbol[0]); i <= state.MC {
			return need[i]
		}
		return false
	}
	var first, rest []decision.Intent
	for _, in := range answers {
		wanted := false
		for _, ch := range in.Choices {
			for i := range d.Options {
				if o := &d.Options[i]; o.Index == ch && symNeeded(o) {
					wanted = true
					break
				}
			}
			if wanted {
				break
			}
		}
		if wanted {
			first = append(first, in)
		} else {
			rest = append(rest, in)
		}
	}
	return append(first, rest...)
}

// subsumeAnswers narrows, for the subsume run (potentialPrefixScript's third
// search), the answers of a mana activation's ask, so the search reaches a
// funding line its budget would otherwise spend itself dead on. The colour
// ask is filtered:
//
//   - a colour ask (every option names a mana symbol) keeps only the colours
//     the floating pool still lacks of the play's coloured-pip-only cost
//     (sr.costCol) and drops the rest: a colour the pool already covers pays
//     nothing more, and every continuation that floats it again reaches the
//     same payer states one that floats a lacking colour reaches -- a pool
//     that covered the pip before covers it after, so any line through the
//     covered answer is dominated by the same line through a lacking one.
//     The run is gated on a play whose cost is coloured pips alone
//     (scriptNeedCounts): a generic or X pip is payable with any colour, so
//     a covered colour is not dominated while one is outstanding.
//
// A colour ask of a source without the play's cost colours (sr.costCol all
// zero at the ask's colours) keeps nothing: with no generic pip outstanding
// its every answer floats mana no pip needs. An ask the filter cannot read
// (an option whose symbol the code cannot resolve) is kept whole, as are
// every non-mana ask and the single-answer asks.
func (sr *scriptSearch) subsumeAnswers(c *Engine, d *decision.Decision, answers []decision.Intent) []decision.Intent {
	if len(answers) <= 1 || !scriptManaUnits(d) || !sr.costOK {
		return answers
	}
	pool := c.G.Players[sr.p].Pool
	kept := 0
	for _, in := range answers {
		wanted := false
		for _, ch := range in.Choices {
			for i := range d.Options {
				o := &d.Options[i]
				if o.Index == ch && len(o.ManaSymbol) == 1 {
					if j := state.ManaIndex(o.ManaSymbol[0]); j <= state.MC && sr.costCol[j] > pool[j] {
						wanted = true
					}
					break
				}
			}
		}
		if wanted {
			answers[kept] = in
			kept++
		}
	}
	if kept == 0 {
		return nil
	}
	return answers[:kept]
}

// scriptPipShort counts the fixed pips pipCol the pool still lacks.
func scriptPipShort(pipCol *[state.MC + 1]int32, pool state.Mana) (n int32) {
	for i := range pool {
		if pipCol[i] > pool[i] {
			n += pipCol[i] - pool[i]
		}
	}
	return n
}

// scriptPayFee pays fee from pool in pay/solve.go's order: the coloured pips
// first, then the generic part from {C}, W, U, B, R, G. false reports a fee
// the pool cannot pay. It mutates pool.
func scriptPayFee(pool *state.Mana, fee state.Mana, generic int32) bool {
	for i := range pool {
		if fee[i] > pool[i] {
			return false
		}
		pool[i] -= fee[i]
	}
	for g := generic; g > 0; g-- {
		spent := false
		for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
			if pool[i] > 0 {
				pool[i]--
				spent = true
				break
			}
		}
		if !spent {
			return false
		}
	}
	return true
}

// feeOnlyGapDoomed reports whether activating gap source id can only walk a
// doomed branch: every mana ability it could activate is a fee-only
// repeatable one (repeatableFeeOnlyMana), and no colour any of them can make
// brings the play's fixed pips closer to paid once the ability's fee is paid
// from the node's opening pool (feeOnlyAskPrune's short test, applied at the
// activation site so the activation and its ask are never visited). CR
// 605.3a: a line that skips such an activation holds at least as much of
// every colour as one that spends a fee it cannot get back; and a fee the
// pool cannot pay is not an activation the payer can make, so the branch is
// not a real line either. Every shape the test cannot read fails open (the
// branch is explored), as feeOnlyAskPrune does.
func (sr *scriptSearch) feeOnlyGapDoomed(c *Engine, id state.ObjID) bool {
	abs := c.appendAvailableManaAbilitiesGate(nil, nil, sr.p, id, true)
	if len(abs) == 0 || !sr.pipOK || !sr.actDescOK {
		return false
	}
	pool := pay.AvailableFor(asPayer(c), sr.p, sr.actDesc).Pool
	before := scriptPipShort(&sr.pipCol, pool)
	for _, ma := range abs {
		if !repeatableFeeOnlyMana(c, sr.p, id, ma) {
			return false // another ability poses a choice the prune cannot read
		}
		// A fixed-colour production ("{R}: Add {B}") is the class too: the
		// kernel's activationStalls prunes it by the same per-colour test, so
		// no open-production gate here (the made loop below reads its colour).
		mp := effects.ManaOf(ma)
		amount, indeterminate := potentialAmount(mp)
		if indeterminate || amount != 1 {
			return false
		}
		cc := c.manaAbilityCC(sr.p, id, ma).Cost
		if amount > cc.Generic+cc.Colored.Total() {
			return false // the ability makes more than its fee spends: the activation can pay
		}
		paid := pool
		if !scriptPayFee(&paid, cc.Colored, cc.Generic) {
			return false // an unpayable fee fails open (the live gate withholds the activation)
		}
		var made state.Mana
		c.addPotentialManaOf(&made, ma)
		for j, n := range made {
			if n <= 0 {
				continue
			}
			after := paid
			after[j] += amount
			if scriptPipShort(&sr.pipCol, after) < before {
				return false // a colour fills a pip the play lacks
			}
		}
	}
	return true
}

// oneShotAskPrune is the plain run's colour-ask prune for a gap whose
// activation is not the fee-only repeatable class feeOnlyAskPrune reads (a
// one-shot flexible source: Vivid Meadow's "{T}, Remove a charge counter: Add
// one mana of any color"). While a fixed pip of the play is still owed
// (before > 0), an answer whose colour fills no such pip
// floats mana a covered colour also pays the generic part with, so the line
// through a filling answer dominates it. An ask with no pip owed, or one
// where no answer fills a pip, is kept whole: the prune only narrows a space
// it can read, never empties an ask it cannot.
func (sr *scriptSearch) oneShotAskPrune(c *Engine, d *decision.Decision, answers []decision.Intent) []decision.Intent {
	if !sr.pipOK || len(answers) <= 1 || !scriptManaUnits(d) {
		return answers
	}
	mca := c.manaColorActivation
	if mca == nil || mca.player != sr.p || mca.source != d.Source || mca.cast || mca.allocation ||
		mca.trigger != nil || mca.ability == nil || !scriptGap(sr.gaps, d.Source) {
		return answers
	}
	if repeatableFeeOnlyMana(c, sr.p, d.Source, mca.ability) {
		return answers // feeOnlyAskPrune's class
	}
	var made state.Mana
	c.addPotentialManaOf(&made, mca.ability)
	pool := c.G.Players[sr.p].Pool
	if sr.actSrc == d.Source && sr.actDescOK {
		pool = sr.actPool // the activation's opening pool (a counter or tap cost spends no pool mana)
	}
	before := scriptPipShort(&sr.pipCol, pool)
	if before == 0 {
		return answers
	}
	kept := 0
	for _, in := range answers {
		wanted := false
		for _, ch := range in.Choices {
			for i := range d.Options {
				o := &d.Options[i]
				if o.Index == ch && len(o.ManaSymbol) == 1 {
					if j := state.ManaIndex(o.ManaSymbol[0]); j <= state.MC && made[j] > 0 {
						after := pool
						after[j]++
						if scriptPipShort(&sr.pipCol, after) < before {
							wanted = true
						}
					}
					break
				}
			}
			if wanted {
				break
			}
		}
		if wanted {
			answers[kept] = in
			kept++
		}
	}
	if kept == 0 {
		return answers // no answer fills a pip: fail open rather than empty the ask
	}
	return answers[:kept]
}

// feeOnlyAskPrune is the plain run's doomed-branch prune (sr.feeStall): the
// colour ask of a fee-only repeatable gap's activation keeps only the
// answers whose colour can bring the play closer to paid, and drops the rest
// (dirac's filtFeeColStalls, mana_filterrepeat.go, is the node-for-node
// mirror; feeOnlyConversionStalls, payment_window_feeonly.go, the same rule
// in the payment window). The ability makes no more mana than its fee
// spends, so activating it only converts pool mana into a colour of its own
// (CR 605.3a: it is repeatable -- no {T}, no activation limit -- so a line
// that skips the activation holds at least as much of every colour as one
// that spends a fee it cannot get back): the conversion helps exactly when
// the made colour fills a fixed pip the play's cost still lacks
// (sr.pipCol). Without the prune the search spends its whole node budget on
// the repeat activations (Energy Refractor's "{2}: Add one mana of any
// color" branches five ways per activation, each a distinct payer state) and
// starves before the funding line that pays the play is reached.
//
// The ask is read only where the whole answer set is a colour choice of ONE
// fee-only repeatable ability of a gap source (the plain walk of the
// census's gaps, scriptGap): any other shape -- another player's ask, a
// multi-ability ask, an option whose symbol the code cannot resolve -- fails
// open. The pool the test reads is the activation's opening pool: the ask's
// pool has already paid the fee, and the fee's units are added back, so the
// short-count before and after is taken where the kernel's mirror takes it.
func (sr *scriptSearch) feeOnlyAskPrune(c *Engine, d *decision.Decision, answers []decision.Intent) []decision.Intent {
	if !sr.pipOK || len(answers) <= 1 || !scriptManaUnits(d) {
		return answers
	}
	// mca is the paid mana ability whose controller picks the colour: the ask
	// is its colour ask. A triggered mana ability's choice, an allocation ask
	// (a Produced$ Combo) and an ask inside a spell's own payment (cast) are
	// not a gap activation's: fail open. mca.source is the activated source;
	// the plain run's gaps are the sources whose activations the search
	// prices (the kernel's mirror runs in the gaps pass alone, mana_filter.go
	// filtColours).
	mca := c.manaColorActivation
	if mca == nil || mca.player != sr.p || mca.source != d.Source || mca.cast || mca.allocation ||
		mca.trigger != nil || mca.ability == nil || !scriptGap(sr.gaps, d.Source) {
		return answers
	}
	sa := mca.ability
	if !repeatableFeeOnlyMana(c, sr.p, d.Source, sa) {
		return answers
	}
	// The kernel's fee-only class (kop.FeeOnlyManaAb): one unit of a chosen
	// colour (ManaN at most one, no each-colour production). A fixed or
	// each-colour production poses no choice ask here, and a variable or
	// multi-unit amount is not the class: fail open.
	mp := effects.ManaOf(sa)
	if !producedOpen(mp.Produced) {
		return answers
	}
	amount, indeterminate := potentialAmount(mp)
	if indeterminate || amount != 1 {
		return answers
	}
	fee := c.manaAbilityCC(sr.p, d.Source, sa).Cost
	if amount > fee.Generic+fee.Colored.Total() {
		return answers // the ability makes more than its fee spends: the activation can pay
	}
	var made state.Mana
	c.addPotentialManaOf(&made, sa)
	// The activation's opening pool: the ask's pool has already paid the fee
	// (CR 605.1a), whose payment destroys the shapes it spent, so the stash
	// taken when the activation was submitted (sr.actPool) is the read.
	if sr.actSrc != d.Source || !sr.actDescOK {
		return answers
	}
	pool := sr.actPool
	var before int32
	for i := range pool {
		if sr.pipCol[i] > pool[i] {
			before += sr.pipCol[i] - pool[i]
		}
	}
	// The fee's coloured pips are paid first, then its generic part from the
	// pool in the order pay/solve.go spends it: {C}, then W, U, B, R, G. That
	// order ignores the pips still owed, so it can spend the very colour the
	// activation is meant to supply. The pool could not pay it: fail open
	// (the live gate withholds such an activation, so this is unreachable).
	for i := range pool {
		if fee.Colored[i] > pool[i] {
			return answers
		}
		pool[i] -= fee.Colored[i]
	}
	for g := fee.Generic; g > 0; g-- {
		spent := false
		for _, i := range [...]int{state.MC, state.MW, state.MU, state.MB, state.MR, state.MG} {
			if pool[i] > 0 {
				pool[i]--
				spent = true
				break
			}
		}
		if !spent {
			return answers
		}
	}
	kept := 0
	for _, in := range answers {
		wanted := false
		for _, ch := range in.Choices {
			for i := range d.Options {
				o := &d.Options[i]
				if o.Index == ch && len(o.ManaSymbol) == 1 {
					if j := state.ManaIndex(o.ManaSymbol[0]); j <= state.MC && made[j] > 0 {
						after := pool
						after[j] += amount
						var short int32
						for k := range after {
							if sr.pipCol[k] > after[k] {
								short += sr.pipCol[k] - after[k]
							}
						}
						// The second cut: a colour a free bare-tap source can
						// make is dominated even when it fills a pip. Read it only
						// where the colour actually improves the short count: the
						// first cut already drops the rest, and the scan is a read
						// of the live board that must not run for a branch this
						// ask does not keep.
						if short < before && !sr.feeOnlyFreeTapColour(c, d.Source, int(j)) {
							wanted = true
						}
					}
					break
				}
			}
		}
		if wanted {
			answers[kept] = in
			kept++
		}
	}
	if kept == 0 {
		// No colour the activation can make fills a pip the play lacks: the
		// activation is skipped (the kernel's order loop `continue`s every
		// colour and walks no step; CR 605.3a). The nil answer set sets
		// sr.approx (visit), which only the verdict path reads
		// (potentialPlayVerdict's search_limit vs insufficient) and that path
		// never arms feeStall -- only potentialPrefixScript's plain run does,
		// and its callers read (script, found) alone -- so the flag cannot
		// change an offer either engine makes and the kernel needs no
		// counterpart.
		return nil
	}
	return answers[:kept]
}

// feeOnlyFreeTapColour is the fee-only ask prune's second cut: p controls an
// untapped source other than skip whose activation costs no mana -- a bare
// tap, the class the kernel's plain source census prices as a free unit
// (dirac's filtFreeTapColour, mana_filterrepeat.go) -- and whose production
// can make colour. A source whose activation also spends a counter, a
// permanent or life is not this class: the swap below would lose that other
// use, so the cut fails open there.
//
// The cut is the swapped-line domination. At the ask the fee-only repeatable
// source S (one mana of any colour, fee F no larger than its production, no
// {T}) offers colour c, and c fills a fixed pip the play still lacks. A
// first-only free source X that can make c is better than S for that pip:
// activate X for c instead of S. That line has c filled and F still floating,
// and S is still untapped (S has no {T}), so every continuation of the
// dominated branch is reachable along it. If the continuation would use X
// again, S makes the same colour for F, paid from the F the swap saved; the
// endpoint after both activations is the same (X spent, one S activation
// paid, the pip filled). X is first-only, so at most one substitution is
// needed. A repeatable free source is the same argument with X never spent,
// so its later uses stay available and the saved F is pure surplus. The
// fee's own coloured pip is saved too when the swap replaces the activation,
// and the any-colour S can make every colour X can. Nothing in the argument
// needs X to sit before or after S in battlefield order: the two lines are
// parallel branches of the same ask, and each one's continuations are
// reachable from the other after both activations.
func (sr *scriptSearch) feeOnlyFreeTapColour(c *Engine, skip state.ObjID, colour int) bool {
	for _, id := range c.G.Zone(state.ZBattlefield, sr.p) {
		if id == skip || sr.ownTapSource(c, id) {
			// The play's own {T} source cannot stand in for the fee-only
			// source: its tap is the play's cost (CR 602.2b, 601.2h), so
			// activating it for mana first leaves that cost unpayable.
			continue
		}
		o := c.G.Obj(id)
		if o == nil || o.PhasedOut || o.Tapped || o.Face() == nil {
			continue
		}
		for _, ma := range c.appendAvailableManaAbilitiesGate(nil, nil, sr.p, id, true) {
			cc := c.manaAbilityCC(sr.p, id, ma)
			if !pay.PaymentPlanTapOnlyCost(cc.Cost) {
				continue
			}
			var made state.Mana
			if !addPotentialChosenColour(c, &made, id, ma) {
				c.addPotentialManaOf(&made, ma)
			}
			if made[colour] > 0 {
				return true
			}
		}
	}
	return false
}

// witnessSelfTargetsLast stably moves, in a target ask the witness replay
// answers, the answers naming p itself behind the rest. The replay is node
// budget-capped, and a trigger the witness's sacrifices stacked (Havoc
// Jester's damage to any target) that p aims at their own last life ends the
// game against p only when it resolves, after a pass chain per answer: with
// "p0" listed first, every combination of such answers was proved dead before
// the first that is not, and the budget ran out before the play was reached.
// A play the answers p can give reach is one gorge offers (CR 601.2c).
func witnessSelfTargetsLast(d *decision.Decision, p state.PlayerID, answers []decision.Intent) []decision.Intent {
	if d.Kind != decision.KTarget {
		return answers
	}
	self := func(in decision.Intent) bool {
		for _, ch := range in.Choices {
			for i := range d.Options {
				// (a player option names no object)
				if o := &d.Options[i]; o.Index == ch && o.Obj == 0 && o.Player == p {
					return true
				}
			}
		}
		return false
	}
	var first, rest []decision.Intent
	for _, in := range answers {
		if self(in) {
			rest = append(rest, in)
		} else {
			first = append(first, in)
		}
	}
	return append(first, rest...)
}

// scriptManaAskHoists reports whether the needed-colour reorder keeps its
// place at the mana-ability ask ma is posing (the engine nils it the moment
// it is answered): the source's own choice among its mana abilities.
//
// The ask's own order is kept only where one of its options costs the payer
// something as the ability resolves -- a painland's damaging colours
// (CR 605.3b: the damage is dealt with the mana) or Haunted Screen's pay-life
// colours (CR 118.3, 119.4) -- and no granted ability stands beside the
// printed ones. Such a source is a last-resort unit (pay.PaymentPlanAbilityTier
// discloses the Damage or Life consequence), and the kernel's ordinary mana
// walk takes it in printed order, the free bare-{T} ability before the painful
// one, so a needed-colour reorder would write a prefix that takes a life the
// kernel does not spend (script_pain_colour_first).
//
// Every other plain ask keeps the reorder. A rider that touches no payer
// life -- Grove of the Burnwillows' {R}/{G} that gives each opponent a life,
// planner-deferred as "source:rider" beside a priced {C} -- is a prefix gap
// the kernel's taps-deferred route (dirac/mana_tapsdefer.go scriptTapsDefer)
// fills with the needed colour itself; branching on {C} first floats a mana
// the kernel never taps (mana_rider_grove_gives_each_opponent_a_life). A
// source holding a granted ability beside its printed ones is paid by the
// kernel's granted-mana prefix search, which mirrors this ask and this
// search's reorder node for node (dirac/mana_grantpain.go painAskOrder) --
// there the reorder stays even for a pain source.
func scriptManaAskHoists(pe pay.Engine, ma *manaActivation, p state.PlayerID, d *decision.Decision) bool {
	if ma == nil || ma.player != p || ma.source != d.Source || len(d.Options) == 0 {
		return true // not a mana-ability ask: the reorder applies
	}
	grants := false
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind != "mana" {
			return true // a granted ability's stage-2 or another ask shape: the reorder applies
		}
		grants = grants || o.Granted
	}
	if grants {
		return true
	}
	for i := range d.Options {
		if o := &d.Options[i]; o.Ability >= 0 && o.Ability < len(ma.abilities) && scriptManaAbilityCostsPayer(pe, p, d.Source, ma.abilities[o.Ability]) {
			return false // a pain or pay-life option: the ask's own order
		}
	}
	return true
}

// scriptManaAbilityCostsPayer reports whether mana ability ab of source costs
// its payer p life as it resolves: a last-resort unit whose disclosed
// consequence is damage to the payer or a life payment.
func scriptManaAbilityCostsPayer(pe pay.Engine, p state.PlayerID, source state.ObjID, ab *cards.SA) bool {
	if ab == nil {
		return false
	}
	tier, c, _ := pay.PaymentPlanAbilityTier(pe, p, source, ab)
	return tier == pay.TierLastResort && (c.Damage != 0 || c.Life != 0)
}

// scriptStateKey identifies a payer state for the search's dedup: every
// pool unit, each own permanent's tapped bit and counters, and the zone
// sizes a sacrifice or a return changes.
func scriptStateKey(c *Engine, p state.PlayerID) string {
	var b strings.Builder
	pl := c.G.Players[p]
	fmt.Fprintf(&b, "%v|%v|%v|", pl.Pool, pl.Snow, pl.ManaUnits())
	for _, id := range c.G.Zone(state.ZBattlefield, p) {
		o := c.G.Obj(id)
		fmt.Fprintf(&b, "%d:%t:%v;", id, o.Tapped, o.Counters)
	}
	fmt.Fprintf(&b, "|%d|%d|%d", len(c.G.Zone(state.ZGraveyard, p)), len(c.G.Zone(state.ZHand, p)), len(c.G.Stack))
	return b.String()
}

// potentialWitnessReaches executes witness plan for potential play o of p on
// a clone, exactly as a manual-surface seat lowers it (plain steps first,
// then the last-resort ones; each step's asks answered with whatever makes
// the step's own production), and reports whether the play is then offered
// -- after letting a trigger the activations stacked resolve. It is the
// check for a witness that sacrifices or returns a permanent: that
// permanent may be the play's only target, or feed a trigger, which the
// planner's mana accounting cannot see.
func (e *Engine) potentialWitnessReaches(p state.PlayerID, o decision.Option, plan decision.PaymentPlan) bool {
	return e.witnessReachesFrom(e, p, o, plan)
}

// witnessReachesFrom is potentialWitnessReaches drawing its clones' storage
// from pool's hypothetical-clone pool (pool is e, or the live engine whose
// own hypothetical search made e).
func (e *Engine) witnessReachesFrom(pool *Engine, p state.PlayerID, o decision.Option, plan decision.PaymentPlan) bool {
	return e.witnessReachesMode(pool, p, o, plan, false, false)
}

// witnessReachesMode is witnessReachesFrom, counting a game the witness's
// own triggers end in p's favour as a reach when winOK. priceOK arms
// witnessCostRepriced, the consuming-cost acceptance the script search's
// priced() wants: a consuming payment may consume a permanent one of the
// play's own cost statics counts, which the replay's end re-ask reprices
// (CR 601.2f), and the search's acceptance is "the plan is the payment",
// not "the end menu re-offers the play". Callers that infer an offer from
// the reach (targetPaidNeedsPlannedPayment's planned-payment conclusion)
// keep it disarmed: their inference reads the end menu, which the repricing
// breaks exactly like any other refusal.
func (e *Engine) witnessReachesMode(pool *Engine, p state.PlayerID, o decision.Option, plan decision.PaymentPlan, winOK, priceOK bool) bool {
	// The reach's clones pose priority asks whose offer walks must price only
	// the plain planner witness (scriptSearching's gate, autoPayCensus): a
	// census arm inside the reach would offer the play on a clone before the
	// witness's steps paid for it, and the reach's end-menu read would take
	// that for a reach (sr.priced's winOK/priceOK acceptance).
	prev := e.scriptSearching
	e.scriptSearching = true
	defer func() { e.scriptSearching = prev }()
	if e.Pending() == nil {
		return false
	}
	var steps []decision.PaymentActivation
	for _, last := range []bool{false, true} {
		for _, a := range plan.Activations {
			if (a.Consequence != nil) == last {
				steps = append(steps, a)
			}
		}
	}
	turn, step, rootStack := e.G.Turn, e.G.Step, len(e.G.Stack)
	nodes := 0
	var visit func(c *Engine, next int, expect state.Mana, waits int) bool
	visit = func(c *Engine, next int, expect state.Mana, waits int) bool {
		if nodes++; nodes > 4*len(steps)+64 {
			return false
		}
		d := c.Pending()
		for guard := 0; d != nil && d.Player != p && guard < 8; guard++ {
			if d.Kind != decision.KPriority {
				// Another player's own ask the payment's activations
				// set off (the order of their simultaneous triggers a
				// sacrificed source triggered, CR 603.3b): searched
				// below, like the acting player's own ask.
				break
			}
			// Another player's priority while a trigger resolves: pass.
			pass := -1
			for _, opt := range d.Options {
				if opt.Kind == "pass" {
					pass = opt.Index
				}
			}
			if pass < 0 || c.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{pass}}) != nil {
				return false
			}
			d = c.Pending()
		}
		if winOK && c.G.Over && next == len(steps) && !c.G.Players[p].Lost {
			// A trigger the paying sacrifice set off ended the game in p's
			// favour: it waits for the cast to finish (CR 601.2i, 603.3b),
			// so the play was payable.
			return true
		}
		// A trigger the paying sacrifice set off waits for the cast to
		// finish (CR 601.2g, 603.3b): its controller's answer to it, when
		// the walk lets it resolve first, is any answer (Zoetic Glyph's
		// discover after its Treasure paid) -- and so is the order of
		// that player's own simultaneous triggers a sacrificed source
		// triggered (Dreadhound over the Eldrazi Spawn whose activation
		// pays), which reaches the witness before p can finish paying.
		other := d != nil && d.Player != p && d.Kind != decision.KPriority
		if d == nil || d.Player != p && !other || c.G.Over || c.G.Turn != turn || c.G.Step != step {
			return false
		}
		if d.Kind != decision.KPriority {
			// the replay answers the step's own colours first (scriptStepNeed)
			for _, in := range witnessSelfTargetsLast(d, p, scriptOrderAnswers(d, scriptAnswers(d), scriptStepNeed(steps, next))) {
				child := pool.hypClone(c)
				ok := child.Submit(in) == nil && visit(child, next, expect, waits)
				pool.hypRelease(child)
				if ok {
					return true
				}
			}
			return false
		}
		if c.G.Players[p].Pool != expect {
			return false
		}
		if next == len(steps) {
			if witnessMenuOffers(d, o) {
				return true
			}
			if witnessAnnouncesBeforePaying(e, c, e.G, p, o, plan) {
				return true
			}
			if priceOK && witnessCostRepriced(c, e, c.G, p, o, plan) {
				return true
			}
			if len(c.G.Stack) <= rootStack || waits >= 4 {
				return false
			}
			for _, opt := range d.Options {
				if opt.Kind == "pass" {
					child := pool.hypClone(c)
					ok := child.Submit(decision.Intent{Seq: d.Seq, Player: p, Choices: []int{opt.Index}}) == nil &&
						visit(child, next, expect, waits+1)
					pool.hypRelease(child)
					return ok
				}
			}
			return false
		}
		act := steps[next]
		for _, opt := range d.Options {
			if !scriptActivate(&opt) || opt.Obj != act.Source {
				continue
			}
			want := expect
			for i, n := range act.Produces {
				want[i] += int32(n)
			}
			child := pool.hypClone(c)
			ok := child.Submit(decision.Intent{Seq: d.Seq, Player: p, Choices: []int{opt.Index}}) == nil && visit(child, next+1, want, waits)
			pool.hypRelease(child)
			if ok {
				return true
			}
		}
		return false
	}
	root := pool.hypClone(e)
	ok := visit(root, 0, e.G.Players[p].Pool, 0)
	pool.hypRelease(root)
	return ok
}

// costTotal is a PaymentCost's mana value in units (CR 202.3): the generic
// part plus every coloured unit.
func costTotal(c decision.PaymentCost) int {
	n := int(c.Generic)
	for _, u := range c.Mana {
		n += int(u)
	}
	return n
}

// witnessCostRepriced is the replay's consuming-cost arm, beside
// witnessAnnouncesBeforePaying's target one: some step of plan sacrifices (or
// returns) a permanent one of the play's own cost statics counts -- a
// Treasure token's mana paying an affinity discount away -- so the menu the
// re-ask poses at the payment's end recomputes a HIGHER price than the
// payment locks. CR 601.2f orders it the other way: the total cost is
// determined at the announcement -- the state the replay started from
// (start), before the plan's steps -- and is paid as determined, so the end
// menu's recomputed price cannot refuse an announcement the plan already
// priced and validates. The replay has proved the plan's steps run and float
// the payment's mana (the pool at the end is the floated one, the play not
// yet announced); accept when every refusal reason but the price is ruled
// out: the recomputed price (CastPaymentCost at the end) is strictly above
// the locked one, the play's targets all survive the payment (the target arm
// above owns a lost one), the recomputed price is payable in isolation (an
// unbounded pool pays it, so the end refusal is the floated mana, not a
// gate) and the locked price is payable from that floated pool (the pure
// pool-only question, pay.PoolCanPay -- the paying primitive would spend the
// node's pool). Plain casts only: a Mode prices its own charge, and an
// alternate-cost index (AltCostIndex) prices the option's own scope, neither
// of which the printed-price comparison reads.
func witnessCostRepriced(end, start witnessCostReader, g *state.Game, p state.PlayerID, o decision.Option, plan decision.PaymentPlan) bool {
	// Plain casts only: a Mode prices its own charge, and an alternate-cost
	// cast's plan.Cost prices the option's own scope, not the printed one
	// CastPaymentCost recomputes.
	if o.Kind != optCast || o.Mode != "" || o.AltCostIndex != 0 {
		return false
	}
	// The announcement priced the plan: the price the announcement state
	// (start, before the plan's steps) recomputes is the plan's own total.
	startCost, ok := start.CastPaymentCost(p, o.Obj)
	if !ok || costTotal(startCost) != costTotal(plan.Cost) {
		return false
	}
	obj := g.Obj(o.Obj)
	if obj == nil || obj.Face() == nil {
		return false
	}
	endCost, ok := end.CastPaymentCost(p, o.Obj)
	if !ok || costTotal(endCost) <= costTotal(plan.Cost) {
		return false
	}
	if sa := obj.Face().SpellAbility(); sa != nil && !end.castTargetsAvailable(p, o.Obj, sa) {
		return false
	}
	var locked Cost
	locked.Generic = int32(plan.Cost.Generic)
	for i := range locked.Colored {
		locked.Colored[i] = int32(plan.Cost.Mana[i])
	}
	// Pool-only feasibility, and pure: the walked node must stay unmutated,
	// so the paying primitive (pay.PayMana) cannot run here -- a spend it
	// emits strands the node's own pass-walk fallback, whose clone must
	// still hold the floated pool.
	if !pay.PoolCanPay(locked, g.Players[p].Pool) {
		return false
	}
	base := pay.WithSpellAbilityExtras(obj.Face(), end.castOfferBase(p, o.Obj))
	hyp := state.Mana{1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28}
	return end.offerCastableUsing(end.collectCostStatics(), p, o.Obj, &base, spellScope(""), false, &hyp)
}

// PotentialPlayScript is the exact fallback for a potential play the
// planner leaves unpriced (PotentialPlan.Reason "unsupported" or
// "search_limit"). It searches, on clones of the engine, for a sequence of
// p's answers after which the play is offered, or priced by the planner
// from the floating pool, at one of p's priority decisions:
//
//   - when the planner prices the play's shape but its census misses some
//     of p's mana abilities (Saruli Caretaker's "{T}, tap an untapped
//     creature", Wall of Roots' counter, a filter such as Heap Gate's
//     "{1}, {T}: any colour"), the search activates only those abilities
//     (any subset, any order, every answer their asks admit, plus as many
//     other sources as their fees need) and asks the planner at every
//     node: the census sources are the planner's to price exactly. When
//     every node's verdict is a proof, the play cannot be paid;
//   - otherwise (a shape the planner does not price) it activates every
//     mana ability p has, in any order, and succeeds only when the play is
//     offered; the whole space searched is a proof.
//
// Either search also passes priority while a trigger an activation caused
// is on the stack. Each distinct payer state is visited once; at most
// budget clones are made. The answer is the script (reason ""; after it
// the seat pays the play as any priced play), or "insufficient" (a proof:
// the whole space was searched and no line assumed another player's
// answer), or "search_limit". It is a pure read of e: only clones taken at
// this intent boundary are submitted to.
func (e *Engine) PotentialPlayScript(p state.PlayerID, a decision.PotentialAction, budget int) ([]ScriptStep, string) {
	// A potential action names no alternative-cost index: any cast option
	// of the play's kind, object and mode reaches it (the seat keys plays
	// the same way).
	sr := e.newScriptSearch(p, a, -1, budget)
	if sr == nil {
		return nil, "unsupported"
	}
	sr.passes = true
	// A potential action carries no grant anchor (its wire form has no SVar or
	// GainedSource), so this public fallback cannot narrow the twin cut the
	// way the prefix search does. The lockstep path is the prefix search
	// (potentialPrefixScript), whose full option tells an SVar grant from a
	// printed activation; here a Kind "ability" action keeps the walk it had.
	sr.twinCut = a.Kind == optAbility
	var census paymentPlanCensus
	var plain PaymentPlanOutcome
	func() {
		e.beginDerivedMemo()
		defer e.endDerivedMemo()
		defer pay.PaymentPlanQueryEnd(asPayer(e), pay.PaymentPlanQueryBegin(asPayer(e)))
		hyp := e.PotentialMana(p)
		census = e.paymentPlanCensusOf(p, &hyp)
		plain, _ = e.potentialPlayVerdict(p, decision.Option{Kind: a.Kind, Obj: a.Obj, Ability: a.Ability, Mode: a.Mode})
	}()
	sr.gaps = census.gaps
	if plain.Reason == "insufficient" && len(census.gaps) > 0 && census.relaxable {
		sr.gapsOnly, sr.nodePlans, sr.funding = true, true, census.fees()
		if script, found := sr.run(e); found {
			return script, ""
		}
		if !sr.limited && !sr.approx && !sr.unproven {
			return nil, "insufficient"
		}
		// Some node's verdict was neither a witness nor a proof: fall
		// back to the full search with the budget left.
		full := e.newScriptSearch(p, a, -1, budget-sr.nodes)
		full.passes, full.gaps = true, census.gaps
		full.twinCut = a.Kind == optAbility
		sr = full
	}
	if script, found := sr.run(e); found {
		return script, ""
	}
	if sr.limited || sr.approx || sr.unproven {
		return nil, "search_limit"
	}
	return nil, "insufficient"
}
