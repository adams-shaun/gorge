package rules

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
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
	a := decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode}
	sr := e.newScriptSearch(p, a, o.AltCostIndex, potentialScriptNodes)
	if sr == nil {
		return nil, false
	}
	sr.gaps, sr.gapsOnly, sr.funding, sr.printedOnly = gaps, true, funding, o.Kind == "ability"
	sr.passes = true
	script, found := sr.run(e)
	return script, found && len(script) > 0
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
	// printedOnly: the play is a printed ability, and a granted one sharing its
	// Ability is another option (only a printed ability reaches the planner).
	printedOnly bool
	// nodePlans asks the planner at every payer priority node; unproven
	// records a node verdict that was neither a witness nor a proof.
	nodePlans bool
	unproven  bool
	budget    int
	turn      int32
	step      state.Step
	stack     int // the stack height at the decision
	nodes     int
	limited   bool // the budget ran out
	approx    bool // a branch assumed an answer it did not enumerate
	seen      map[string]bool
	path      []ScriptStep
	// pool is the live engine whose hypothetical-clone pool backs the
	// search's nested witness checks (hypclone.go); set by run.
	pool *Engine
}

func (e *Engine) newScriptSearch(p state.PlayerID, a decision.PotentialAction, altCost, budget int) *scriptSearch {
	d := e.Pending()
	if d == nil || d.Player != p || d.Kind != decision.KPriority {
		return nil
	}
	sr := &scriptSearch{p: p, play: a, altCost: altCost, budget: budget, turn: e.G.Turn, step: e.G.Step, stack: len(e.G.Stack), seen: map[string]bool{}}
	if altCost <= 0 {
		sr.need = scriptNeedColours(e, e.G.Obj(a.Obj), p, a)
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
// search tries first, never which answers exist. A cast of a mode or an
// alternative cost (MayPlay*, flashback) whose pips the printed base does not
// carry leaves the set empty, and so the engine order.
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
	if o == nil || o.Face() == nil {
		return need
	}
	switch a.Kind {
	case "cast":
		if a.Mode == "" {
			add(r.castOfferBase(p, a.Obj))
		}
	case "ability":
		if pa, ok := o.PileAbilityAt(a.Ability); ok && pa.SA != nil {
			add(r.abilityCompiledCost(pa.SA).Cost)
		}
	}
	return need
}

// run searches from a clone of e; found reports a path (possibly empty,
// when the play is already payable at the root).
func (sr *scriptSearch) run(e *Engine) ([]ScriptStep, bool) {
	sr.pool = e
	root := e.hypClone(e)
	found := sr.visit(root, 0, false)
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
	if sr.play.Kind != "ability" || id != sr.play.Obj {
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
	if sr.play.Kind != "ability" {
		return false
	}
	o := c.G.Obj(sr.play.Obj)
	return o != nil && o.Tapped && sr.ownTapSource(c, sr.play.Obj)
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
	if sr.play.Kind == "ability" {
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

func (sr *scriptSearch) offered(o *decision.Option) bool {
	a := sr.play
	if sr.printedOnly && (o.SVar != "" || o.GainedSource != 0) {
		return false
	}
	return o.Kind == a.Kind && o.Obj == a.Obj && o.Ability == a.Ability && o.Mode == a.Mode && (sr.altCost < 0 || o.AltCostIndex == sr.altCost)
}

// priced reports whether, at c's payer priority, the planner pays the play
// from the floating pool (a witness that also reaches it); a verdict that
// is neither a witness nor a proof marks the search unproven.
func (sr *scriptSearch) priced(c *Engine) bool {
	defer pay.PaymentPlanQueryEnd(asPayer(c), pay.PaymentPlanQueryBegin(asPayer(c)))
	o := decision.Option{Kind: sr.play.Kind, Obj: sr.play.Obj, Ability: sr.play.Ability, Mode: sr.play.Mode, AltCostIndex: max(sr.altCost, 0)}
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
	if paymentPlanConsumes(*got.Plan) && !c.witnessReachesFrom(sr.pool, sr.p, o, *got.Plan) {
		if sr.nodePlans {
			sr.unproven = true
		}
		return false
	}
	return true
}

// try submits in to a clone of c and continues the search there. gapUsed
// records that the path activated an uncovered source (or let the stack
// resolve): only then can the planner's verdict differ from the root's.
func (sr *scriptSearch) try(c *Engine, d *decision.Decision, in decision.Intent, funded int, gapUsed bool) bool {
	child := sr.pool.hypClone(c)
	found := false
	if child.Submit(in) == nil {
		sr.path = append(sr.path, scriptStepOf(d, in))
		if found = sr.visit(child, funded, gapUsed); !found {
			sr.path = sr.path[:len(sr.path)-1]
		}
	}
	// The path holds plain values (scriptStepOf), so the clone is dead
	// either way.
	sr.pool.hypRelease(child)
	return found
}

func (sr *scriptSearch) visit(c *Engine, funded int, gapUsed bool) bool {
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
	if d.Kind != decision.KPriority {
		// An ask the activation poses: branch over its answers.
		answers := scriptAnswers(d)
		answers = scriptOrderAnswers(d, answers, sr.need)
		if len(answers) == 0 {
			sr.approx = true
		}
		for _, in := range answers {
			if sr.tapsOwnSource(c, d, in) {
				continue // the ask taps the play's own {T} cost source (CR 602.2b)
			}
			if sr.try(c, d, in, funded, gapUsed) {
				return true
			}
		}
		return false
	}
	for i := range d.Options {
		if sr.offered(&d.Options[i]) {
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
	if (gapUsed || sr.nodePlans) && !sr.waitsForStack(c) && sr.priced(c) {
		return true
	}
	// The uncovered sources first; then the others (under gapsOnly, while
	// funding lasts: a covered source tapped to pay a filter's fee).
	for _, gap := range []bool{true, false} {
		if !gap && sr.gapsOnly && funded >= sr.funding {
			break
		}
		for i := range d.Options {
			opt := &d.Options[i]
			if opt.Kind != "activate" || scriptGap(gaps, opt.Obj) != gap {
				continue
			}
			if sr.ownTapSource(c, opt.Obj) {
				continue // its mana would tap the play's own {T} cost source (CR 602.2b)
			}
			next := funded
			if !gap {
				next++
			}
			if sr.try(c, d, decision.Intent{Seq: d.Seq, Player: p, Choices: []int{opt.Index}}, next, gapUsed || gap) {
				return true
			}
		}
	}
	if sr.passes && len(c.G.Stack) > sr.stack {
		// A trigger an activation caused: let it resolve, the pool floats.
		// (An object already on the stack at the decision is not the
		// search's to resolve: the play is then a later decision's.)
		for i := range d.Options {
			if d.Options[i].Kind == "pass" {
				return sr.try(c, d, decision.Intent{Seq: d.Seq, Player: p, Choices: []int{d.Options[i].Index}}, funded, true)
			}
		}
	}
	return false
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
// Min == Max == n object election every n-combination of its options, or for
// a Min == Max == n unit-major colour allocation every per-unit combination
// (each at most 64).
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
		out = scriptSubsets(d, mk)
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
func scriptSubsets(d *decision.Decision, mk func([]int) decision.Intent) []decision.Intent {
	n, k := d.Min, len(d.Options)
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

// scriptOrderAnswers stably reorders an ask's valid answers so the ones that
// name a colour the searched play's cost asks for come first, engine order
// kept inside each half. The answer SET is unchanged, but the search is
// budget-capped (potentialScriptNodes), not complete: order decides which
// plans it reaches, and so which plan it finds. A W U B R G colour ask
// otherwise spends its nodes proving every colour the cost does not name dead
// before the first one it does.
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
	return e.witnessReachesMode(pool, p, o, plan, false)
}

// witnessReachesMode is witnessReachesFrom, counting a game the witness's
// own triggers end in p's favour as a reach when winOK.
func (e *Engine) witnessReachesMode(pool *Engine, p state.PlayerID, o decision.Option, plan decision.PaymentPlan, winOK bool) bool {
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
			if d.Kind != decision.KPriority && waits > 0 {
				break // another player's answer to a waiting trigger: searched below
			}
			// Another player's priority while a trigger resolves: pass.
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
		if winOK && c.G.Over && next == len(steps) && !c.G.Players[p].Lost {
			// A trigger the paying sacrifice set off ended the game in p's
			// favour: it waits for the cast to finish (CR 601.2i, 603.3b),
			// so the play was payable.
			return true
		}
		// A trigger the paying sacrifice set off waits for the cast to
		// finish (CR 601.2g, 603.3b): its controller's answer to it, when the
		// walk lets it resolve first, is any answer (Zoetic Glyph's discover
		// after its Treasure paid).
		other := d != nil && d.Player != p && d.Kind != decision.KPriority && waits > 0
		if d == nil || d.Player != p && !other || c.G.Over || c.G.Turn != turn || c.G.Step != step {
			return false
		}
		if d.Kind != decision.KPriority {
			for _, in := range scriptAnswers(d) {
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
			for i := range d.Options {
				opt := &d.Options[i]
				if opt.Kind == o.Kind && opt.Obj == o.Obj && opt.Ability == o.Ability && opt.Mode == o.Mode && opt.AltCostIndex == o.AltCostIndex {
					return true
				}
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
			if opt.Kind != "activate" || opt.Obj != act.Source {
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
