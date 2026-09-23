// CantAttackUnless (the attack-prop static, CR 508.1g attack costs).
//
// `S:Mode$ CantAttackUnless | ValidCard$ ... | Target$ ... | Cost$ N` is the
// "creatures can't attack you unless their controller pays {N} for each
// creature they control that's attacking you" family (Ghostly Prison,
// Windborn Muse, Propaganda, Baird, ...). Before this file the mode was a
// bare Note: nothing read it, so the cards did not even stop attacks.
//
// The reading, one model for every carrier: a static whose ValidCard$ admits
// the attacking creature and whose Target$ admits the defender (absent = every
// defender, the same scoping attackBlocked gives CantAttack's Target$) charges
// the attacking creature's controller a mana price PER ATTACKING CREATURE the
// static matches. Several statics on one defender sum. The charge is paid
// during the declare-attackers step (CR 508.1: costs paid as attackers are
// declared), from the floating pool first and then by tapping mana sources
// through a payment window (rules/attack_cost.go's startAttackPay, the
// CR 601.2g payment-window discipline rules/ward.go already runs for ward).
//
// A static this build cannot price is SKIPPED, never enforced blanket -- the
// permissive direction the shipped-restriction convention (combatrestriction1)
// fixed for CantAttack/CantSacrifice: an unwhitelisted parameter (Nils'
// RememberingAttacker$, a per-creature variable price) or an unresolvable
// cost body (the Sac<...>/Return<...>/tapXType<...> non-mana costs, the
// Phyrexian {W/P}) leaves the creature free to attack, and the shape is
// ledgered in AGENTS.md's approximations table rather than silently
// over-blocking.
package rules

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// chooseAttackPay is the declare-attackers attack-cost payment window
// (attackPayAnswer). chooseEnlist is 27; this is the next free value.
const chooseAttackPay chooseFor = chooseEnlist + 1

// chooseUnleash occupies chooseAttackPay+1; keep this transient window above
// the fixed untap selector (40).
const chooseBlockPay chooseFor = 41

// cantAttackUnlessParamsReadable is the parameter whitelist a face
// CantAttackUnless static must pass before this build enforces it. The gate
// parameters (IsPresent$/IsPresent2$/CheckSVar$/SVarCompare$/Condition$) are
// evaluated by the shared continuousGateHolds grammar; every other parameter
// fails the whitelist and the static is skipped permissively. The corpus's
// carriers all pass except Dain's Condition$ EnduringStory (whose gate
// continuousConditionHolds fails closed, Storied being unimplemented).
//
// RememberingAttacker$ True is readable: attackUnlessPrice binds the
// attacking creature into the pricing context as Remembered, which is what
// the Remembered$CardCounters.ALL SVar body (Nils, Discipline Enforcer)
// resolves against. It is meaningful only for the attack direction, so
// blockPairCharge deliberately prices with a zero attacker.
func cantAttackUnlessParamsReadable(params map[string]string) bool {
	for k := range params {
		switch k {
		case "Mode", "ValidCard", "Target", "Cost", "Description", "Secondary", "Attacker",
			"IsPresent", "IsPresent2", "CheckSVar", "SVarCompare", "Condition",
			"RememberingAttacker":
		default:
			return false
		}
	}
	return true
}

// attackUnlessPrice prices one static's Cost$: a literal integer directly;
// an inline Count$ expression or an SVar name on the static's own face
// through the shared effects.EvalCountOK grammar bound to the static's
// source (so Sphere of Safety's Count$Valid Enchantment.YouCtrl and Cowed by
// Wisdom's Count$ValidHand Card.YouOwn both read "you" as the static's
// controller, and a Card.Self shape such as Myr Prototype's
// Count$CardCounters.P1P1 reads the static's own source). A cost this
// resolver cannot price -- a non-mana cost token (Sac<...>, Return<...>,
// tapXType<...>), a Phyrexian symbol, an unresolvable SVar name -- returns
// ok=false and the caller skips the static.
// The price is re-derived per ATTACKER so a RememberingAttacker$ static can
// read the creature the charge is for. attacker is the attacking creature
// whose pair is being priced, or 0 for the block direction (a CantBlockUnless
// static has no RememberingAttacker$ carrier in the corpus).
func (e *Engine) attackUnlessPrice(sv staticView, attacker state.ObjID) (int32, bool) {
	raw := strings.TrimSpace(sv.Params["Cost"])
	if raw == "" {
		return 0, false
	}
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n), true
	}
	ctx := &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: sv.SVars}
	// RememberingAttacker$ True makes the attacking creature the resolution's
	// Remembered referent (Forge's CostRememberingAttacker convention), so an
	// SVar body such as Nils's `Remembered$CardCounters.ALL` prices the charge
	// off THAT creature's counters rather than the static's own source.
	if attacker != 0 && strings.EqualFold(strings.TrimSpace(sv.Params["RememberingAttacker"]), "True") {
		ctx.Remembered = []state.Target{{Obj: attacker}}
	}
	if strings.HasPrefix(raw, "Count$") {
		return effects.EvalCountOK(e, ctx, raw)
	}
	if body, ok := sv.SVars[raw]; ok {
		return effects.EvalCountOK(e, ctx, body)
	}
	return 0, false
}

// attackPairCharge prices the (attacker, defender) pair: the total mana the
// attacker's controller must pay per creature attacking defender, the sum of
// every live face CantAttackUnless static that admits the pair and prices.
// It is a pure read (no event, no state write) and deterministic (the
// activeStatics walk is the one deterministic scan every static consumer
// shares), so the offer list, the requirement solver, the validator and the
// payer all re-derive the same number.
func (e *Engine) attackPairCharge(id state.ObjID, defender state.PlayerID) int32 {
	total := int32(0)
	for _, sv := range e.activeStatics("CantAttackUnless") {
		if !cantAttackUnlessParamsReadable(sv.Params) {
			continue
		}
		if !e.continuousGateHolds(sv) {
			continue
		}
		spec := sv.Params["ValidCard"]
		if spec == "" {
			// Every corpus carrier states ValidCard$; a static without one
			// has no readable shape here. Skip, never blanket.
			continue
		}
		if !e.matchesSpec(spec, id, e.specCtxSVars(sv.Source, sv.Controller, sv.SVars)) {
			continue
		}
		if !restrictionPlayerTargetMatches(e.G, sv.Params["Target"], defender, sv.Controller, nil) {
			continue
		}
		n, ok := e.attackUnlessPrice(sv, id)
		if !ok || n <= 0 {
			continue
		}
		total += n
	}
	return total
}

// blockPairCharge is the per (blocker, attacker) charge from block-prop
// statics. The two specs are deliberately evaluated against their respective
// combat objects; an absent ValidCard$ is an UNCONDITIONAL blocker match
// (Awesome Presence scopes only by Attacker$ and still prices every blocker
// against its enchanted attacker), and an absent Attacker$ is an
// unconditional attacker scope.
func (e *Engine) blockPairCharge(blocker, attacker state.ObjID) int32 {
	total := int32(0)
	for _, sv := range e.activeStatics("CantBlockUnless") {
		if !cantAttackUnlessParamsReadable(sv.Params) || !e.continuousGateHolds(sv) {
			continue
		}
		if spec := sv.Params["ValidCard"]; spec != "" && !e.matchesSpec(spec, blocker, e.specCtxSVars(sv.Source, sv.Controller, sv.SVars)) {
			continue
		}
		if spec := sv.Params["Attacker"]; spec != "" && !e.matchesSpec(spec, attacker, e.specCtxSVars(sv.Source, sv.Controller, sv.SVars)) {
			continue
		}
		// No attacker binding in the block direction: a CantBlockUnless
		// static has no RememberingAttacker$ carrier, so 0 is passed.
		n, ok := e.attackUnlessPrice(sv, 0)
		if ok && n > 0 {
			total += n
		}
	}
	return total
}

// blockManaBudget is the defending player's available payment budget. Unlike
// attackers, that player has had priority since attackers were declared, so
// floating mana is part of the normal declaration path.
func (e *Engine) blockManaBudget(p state.PlayerID) int32 { return e.attackBudget(p) }

type blockPayWindow struct {
	chosen []decision.Option
	player state.PlayerID
	charge int32
}

func (e *Engine) startBlockPay(chosen []decision.Option, player state.PlayerID, charge int32) bool {
	if e.blockManaBudget(player) < charge {
		return false
	}
	e.blockPay = &blockPayWindow{chosen: chosen, player: player, charge: charge}
	e.askNextBlockPay()
	return true
}

func (e *Engine) askNextBlockPay() bool {
	st := e.blockPay
	if st == nil {
		return false
	}
	remaining := st.charge - e.G.Players[st.player].Pool.Total()
	if remaining <= 0 {
		return false
	}
	sources := e.attackManaSources(st.player)
	if len(sources) == 0 {
		return false
	}
	d := &decision.Decision{Player: st.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: fmt.Sprintf("Pay %d to block -- tap a mana source", st.charge)}
	for _, s := range sources {
		name := "a permanent"
		if o := e.G.Obj(s.id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "block_mana", Obj: s.id,
			Label: fmt.Sprintf("Tap %s for mana", name)})
	}
	e.choosing = chooseBlockPay
	e.ask(d)
	return true
}

func (e *Engine) blockPayAnswer(d *decision.Decision, in decision.Intent) {
	st := e.blockPay
	e.choosing = chooseNone
	if st == nil {
		return
	}
	chosen := d.Chosen(in)
	if len(chosen) == 1 {
		for _, s := range e.attackManaSources(st.player) {
			if s.id == chosen[0].Obj {
				e.resolveManaAbilityRefOriginal(st.player, s.id, s.ma, s.original, s.gained, false, false, false)
				break
			}
		}
	}
	if st.charge-e.G.Players[st.player].Pool.Total() <= 0 {
		e.payMana(st.player, Cost{Generic: st.charge})
		chosenPairs := make([][2]state.ObjID, 0, len(st.chosen))
		for _, opt := range st.chosen {
			chosenPairs = append(chosenPairs, [2]state.ObjID{opt.Attacker, opt.Obj})
		}
		e.blockPay = nil
		e.emit(events.Event{Kind: events.DeclareBlockers, Player: st.player, Pairs: chosenPairs})
		e.blockerRound.cursor++
		return
	}
	if !e.askNextBlockPay() {
		e.blockPay = nil
	}
}

// attackManaSource is one mana source the declare-attackers payment window
// may tap, with the units its single free mana ability produces.
type attackManaSource struct {
	id state.ObjID
	// ma is the EXACT ability the window resolves, with a choice-shaped
	// Produced$ already pinned to one concrete colour so the activation poses
	// no colour sub-ask and adds exactly units.
	ma *cards.SA
	// original is the source pile's compiled ability. ma can be an immutable
	// Produced$ rewrite, but activation-limit markers must retain original's
	// pile identity.
	original *cards.SA
	// gained is the ability's gained-ability identity, captured from original
	// before a Produced$ rewrite.
	gained gainedManaRef
	units  int32
}

// attackManaSources walks the payer's battlefield in zone order and returns
// every untapped permanent whose window-usable mana abilities contain a
// free-cost ability whose production this build can price: a plain Produced$
// symbol list ("G", "R G", "RR") adds exactly those symbols; a
// choice-shaped production (the blank/"Any"/"Combo Any" shapes and the
// "Combo <colours>"/"Chosen" families) yields ONE unit of mana per
// activation, of a colour chosen when it is tapped (CR 106.1b) -- and any
// colour pays a generic attack tax, so it counts one unit. A fail-closed
// shape the parser claims no colour for (ColorIdentity, a Special word) is
// excluded. A known literal Amount$ scales the units. The SAME membership is
// the affordability bound's input (attackBudget), so a pair is offered only
// when the window can actually reach the charge -- the wedge guard: every
// tap adds its counted units, so the window can never strand.
//
// One source per PRICEABLE ALTERNATIVE: a permanent with several free mana
// abilities (a Volcanic Island's intrinsic {U} and {R}) offers one tap option
// per ability. Each option resolves the exact ability (no nested colour
// sub-ask), so the membership is no longer narrowed to a single-ability
// permanent. An ability the shared walk cannot price deterministically
// (Indeterminate Amount$, a RestrictValid$-governed batch) is still excluded:
// its produced batch cannot pay an attack cost, so counting its units would
// overstate the payer's reach. A permanent whose only remaining abilities are
// so excluded contributes nothing.
func (e *Engine) attackManaSources(p state.PlayerID) []attackManaSource {
	var out []attackManaSource
	// windowManaUnits is the ONE membership the offer gate (attackBudget) and
	// this tap list share, so the attack window can never be offered a charge
	// its sources cannot reach (see the doc comment on windowManaUnits). The
	// window taps one concrete ability with no sub-ask, and windowManaUnits
	// already exposes one alt per priceable ability, so every alt becomes an
	// option -- a multi-colour dual's two intrinsics are both payable now.
	for _, u := range e.windowManaUnits(p) {
		for _, a := range u.alts {
			units := int32(0)
			for _, n := range a.counts {
				units += n * a.amt
			}
			if units <= 0 {
				continue
			}
			out = append(out, attackManaSource{id: u.id, ma: a.ma, original: a.ma, gained: e.gainedManaRefFor(p, u.id, a.ma), units: units})
		}
	}
	// The choice-shaped productions the shared membership deliberately
	// excludes (windowManaUnits skips every "Any"/"Combo"/"Chosen" shape: the
	// unless-cost window cannot tap one without a mid-resolution colour
	// sub-ask). A generic attack tax is payable by ANY colour, so the attack
	// window admits them through its own walk and pins the choice (ticket
	// cli-20260922T225137Z-c670f42d) -- every source counted here is also
	// tappable, so the wedge guard still holds.
	out = append(out, e.attackChoiceManaSources(p)...)
	return out
}

// attackChoiceManaSources is the attack window's choice-shaped membership: it
// walks the payer's battlefield in zone order and returns, for every untapped
// permanent, one option per free-cost window-usable mana ability whose
// production is choice-shaped (the blank-excluded "Any" / "Combo Any" /
// "Combo <colours>" / "Chosen" families). A permanent with several free
// abilities (a plain symbol ability plus a choice-shaped one, or two
// colour-fixing abilities) therefore yields one option per choice-shaped
// ability. A plain Produced$ symbol list is the shared walk's domain and never
// reaches here; a fail-closed shape the parser claims no colour for
// (ColorIdentity, a Special word) has an empty slot set and is excluded. A
// known literal Amount$ scales the units.
func (e *Engine) attackChoiceManaSources(p state.PlayerID) []attackManaSource {
	var out []attackManaSource
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		o := e.G.Obj(id)
		if o == nil || o.Tapped || o.Face() == nil {
			continue
		}
		for _, ma := range e.availableManaAbilitiesForWindow(p, id, false) {
			if strings.TrimSpace(ma.Params["RestrictValid"]) != "" {
				continue
			}
			if !manaFreeCost(e.parseCost(ma.Params["Cost"])) {
				continue
			}
			amt := availableAmount(ma)
			if amt <= 0 {
				continue
			}
			produced := strings.TrimSpace(ma.Params["Produced"])
			if produced == "" {
				// The shared walk's domain: the executor's deterministic one-
				// colourless default, already counted there. Admitting it here
				// too would double-count the source.
				continue
			}
			// Chosen is an as-enters read. Substitute it before parsing so a
			// Combo R Chosen land recorded as G has only R/G slots, never the
			// raw parser's source-agnostic WUBRG superset. Without a valid record
			// it cannot produce a colour and stays out of this payment window.
			if producedNeedsChosen(produced) {
				chosen := e.chosenProducedColour(id)
				if chosen == "" {
					continue
				}
				produced = substituteChosenProduced(produced, chosen)
			}
			counts, choice := cards.ProducedCounts(produced)
			units := int32(0)
			if choice {
				// A choice-shaped production produces exactly ONE unit of mana
				// per activation, of a colour chosen when the source is tapped
				// ("Any" / "Combo Any" / "Combo B R" / "Chosen", CR 106.1b). One
				// unit of any colour pays a generic attack tax, so the source
				// counts one payable unit no matter how many colour slots its
				// alternatives name.
				total := int32(0)
				for _, n := range counts {
					total += n
				}
				if total > 0 {
					units = amt
				}
			}
			if units <= 0 {
				continue
			}
			// Pin the choice to ONE concrete colour before recording the
			// ability. resolveManaAbility resolves the source's resolution
			// inline, and a choice-shaped Produced$ would pose a
			// mid-resolution colour ask there; attackPayAnswer then re-reads
			// the pool and re-poses the next tap ask, and e.ask (with e.resume
			// nil for a window, not a suspended resolution) would silently
			// displace that colour ask -- tapping the source for nothing. The
			// tax is generic, so the colour cannot matter: take the
			// deterministic first producible colour (R-9), the same rewrite
			// the activation path performs, so the tap adds exactly one unit
			// and poses no sub-ask. The gained identity is captured before the
			// rewrite, which changes the SA pointer.
			gained := e.gainedManaRefFor(p, id, ma)
			out = append(out, attackManaSource{id: id, ma: withProduced(ma, ma, oneColourProduced(counts)), original: ma, gained: gained, units: units})
		}
	}
	return out
}

// oneColourProduced turns a choice-shaped production's slot vector into the
// single plain Produced$ symbol the attack payment window resolves without a
// sub-ask. It takes the first producible slot in WUBRG order (deterministic,
// R-9) and colourless only when no colour is on offer, so a blank ability's
// one colourless unit round-trips to "C". The caller has already established
// at least one non-zero slot.
// producedNeedsChosen reports whether a Produced$ grammar token needs the
// source's recorded as-enters colour before it can be priced.
func producedNeedsChosen(produced string) bool {
	for _, tok := range strings.Fields(produced) {
		switch strings.Trim(tok, "{}") {
		case "Chosen", "ChosenColor", "ComboChosen":
			return true
		}
	}
	return false
}

func oneColourProduced(counts [6]int32) string {
	for i := range counts {
		if counts[i] > 0 {
			return string(cards.ManaSymbol(i))
		}
	}
	return "C"
}

// attackBudget is the declare-attackers affordability bound: the floating
// pool plus the units attackManaSources can produce. At the declare-attackers
// step the pool is empty (CR 500.4 empties it at every step boundary and the
// declaring player has held no priority since), so the bound is the tappable
// production; the pool term keeps the helper honest if a future path ever
// reaches a declaration with mana floating.
//
// A permanent contributes ONE tap, so its alternatives are NOT additive: a
// dual land that can tap for {U} or {R} contributes one unit, not two. Each
// permanent's contribution is therefore the MAXIMUM units over its
// alternatives (plain abilities from windowManaUnits and choice-shaped ones
// from attackChoiceManaSources, both keyed by the same ObjID), and only then
// are the per-permanent bounds summed. Summing the alternatives instead would
// let the offer gate admit a charge the window cannot reach (a single
// two-ability land would look like two sources), which is the stranding
// defect the wedge guard exists to prevent.
func (e *Engine) attackBudget(p state.PlayerID) int32 {
	total := e.G.Players[p].Pool.Total()
	best := make(map[state.ObjID]int32)
	for _, s := range e.attackManaSources(p) {
		if s.units > best[s.id] {
			best[s.id] = s.units
		}
	}
	// A map range is order-independent here (integer addition is
	// commutative) and reaches no event, option or view: only total matters.
	for _, n := range best {
		total += n
	}
	return total
}

// attackOffer is one (attacker, defender) pair askAttackers offers, with the
// mana price attacking that defender charges per creature (0 = free).
type attackOffer struct {
	id    state.ObjID
	def   state.PlayerID
	price int32
}

// attackOffers builds the offer list askAttackers, attackDutyDischargeable
// and validateAttackers share -- the one source of truth for which pairs exist
// this combat. The enumeration and the ORDER are exactly askAttackers':
// defender-major -- for each defender in AliveFrom(0) minus the active
// player, for each canAttack-filtered battlefield creature in zone order,
// through the goad/CantAttack filters. On top, the affordability
// bound: a chargeable pair is admitted when its INDIVIDUAL price fits the
// payer's attackBudget, so the list never offers an option the payer cannot
// afford at all. The declaration's TOTAL is enforced at submit
// (validateAttackers sums the chosen prices and rejects a total over the
// budget) rather than by a running serialization here: a list-order greedy
// bound denied legal, payable declarations -- two attackers at a {1} and a
// {2} prop with a budget of 3 could not both be declared when they were
// enumerated in the wrong order, and validateAttackers then rejected the
// declaration outright. Admitting each individually-affordable pair and
// pricing the whole declaration on submission lets the client assemble any
// declaration the payer can actually pay; the solver's requirement read
// (attackDutyDischargeable) then asks only that a required creature has an
// affordable pair that DISCHARGES one of its duties, which is the CR 508.1d
// "if able" reading.
//
// CR 508.1d's "satisfy as many requirements as possible": once the legal,
// affordable pairs are known, every pair of a creature that satisfies FEWER
// named requirements than the creature's best available defender is dropped.
// That is what keeps a named MustAttack$ duty and a goad from cancelling each
// other out into "the creature attacks nobody": the earlier single-defender
// filter removed the non-named pairs while goadMayAttack removed the named
// one, so the offer list emptied and the requirement solver saw an unrequired
// creature (the t2 review's defect). A creature with no requirement set keeps
// every legal pair, so an ordinary declaration is byte-identical. When two
// named duties name different defenders neither dominates (both satisfy one),
// so both defenders stay offered and the controller picks; when they name the
// same defender that defender is uniquely maximal.
func (e *Engine) attackOffers() []attackOffer {
	p := e.G.Active
	var out []attackOffer
	budget := e.attackBudget(p)
	var defenders []state.PlayerID
	for _, q := range e.G.AliveFrom(0) {
		if q != p {
			defenders = append(defenders, q)
		}
	}
	// One requirement set per creature, computed once from the board (never
	// per pair), keyed by ObjID and read by lookup only -- no map iteration
	// reaches the offer list order.
	reqs := make(map[state.ObjID]attackRequirementSet)
	for _, id := range e.G.Zone(state.ZBattlefield, p) {
		if e.canAttack(id) {
			reqs[id] = e.attackRequirements(id)
		}
	}
	for _, d := range defenders {
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if !e.canAttackPair(id, d) {
				continue
			}
			if !e.goadMayAttack(id, d) {
				continue
			}
			if e.attackBlocked(id, d) {
				continue
			}
			price := e.attackPairCharge(id, d)
			if price > 0 && budget < price {
				continue
			}
			out = append(out, attackOffer{id: id, def: d, price: price})
		}
	}
	// Best named satisfaction per creature over the pairs that survived.
	best := make(map[state.ObjID]int)
	for _, of := range out {
		if n := reqs[of.id].satisfiedBy(of.def); n > best[of.id] {
			best[of.id] = n
		}
	}
	keep := out[:0]
	for _, of := range out {
		rs := reqs[of.id]
		if rs.any() && rs.satisfiedBy(of.def) < best[of.id] {
			continue
		}
		keep = append(keep, of)
	}
	return keep
}

// attackCharge prices a whole declaration: the sum over its chosen pairs.
func (e *Engine) attackCharge(chosen []decision.Option) int32 {
	total := int32(0)
	for _, opt := range chosen {
		total += e.attackPairCharge(opt.Obj, opt.Player)
	}
	return total
}

// attackPayWindow is the declare-attackers attack-cost payment window's
// resumable state (the enlistAsk/exertAsk precedent, plain value... the
// chosen options slice is shared with the enlist state, never mutated): the
// answered KAttackers declaration, the paying player, and the outstanding
// charge. Clone copies the pointer.
type attackPayWindow struct {
	chosen []decision.Option
	player state.PlayerID
	charge int32
}

// startAttackPay opens the payment window for a declaration whose charge the
// floating pool cannot cover. The coverage guard is the budget the offer
// list already checked (attackOffers admits each individually-affordable
// pair) re-checked here for the defensive paths: the window can only be
// opened when pool + tappable units >= charge, and every offered source adds
// its counted units, so the window always terminates with the charge paid --
// never stranded. Returns false (the caller emits the one loud Note and
// completes the declaration) when even the budget cannot cover the charge --
// unreachable through a submitted intent, whose total Decision.MaxSum
// already bounded, kept for a hand-built declaration.
func (e *Engine) startAttackPay(chosen []decision.Option, player state.PlayerID, charge int32) bool {
	if e.attackBudget(player) < charge {
		return false
	}
	e.attackPay = &attackPayWindow{chosen: chosen, player: player, charge: charge}
	e.askNextAttackPay()
	return true
}

// askNextAttackPay poses one tap ask over the remaining attackManaSources
// (in zone order) and reports whether it did. The pool is re-read every
// round: each tap grew it, and the window completes the moment it covers the
// charge -- exact coverage, never over-tapping.
func (e *Engine) askNextAttackPay() bool {
	st := e.attackPay
	if st == nil {
		return false
	}
	remaining := st.charge - e.G.Players[st.player].Pool.Total()
	if remaining <= 0 {
		return false
	}
	sources := e.attackManaSources(st.player)
	if len(sources) == 0 {
		// Unreachable while the coverage invariant holds (the remaining
		// sources' units are always >= the remaining charge). Loud and
		// non-wedging if a future shape ever breaks it.
		e.emit(events.Event{Kind: events.Note, Player: st.player,
			Text: fmt.Sprintf("could not pay the {%d} attack cost", st.charge)})
		return false
	}
	d := &decision.Decision{Player: st.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: fmt.Sprintf("Pay {%d} to attack -- tap a mana source", st.charge)}
	for _, s := range sources {
		name := "a permanent"
		if o := e.G.Obj(s.id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "attack_mana", Obj: s.id,
			Label: fmt.Sprintf("Tap %s for mana", name)})
	}
	e.choosing = chooseAttackPay
	e.ask(d)
	return true
}

// attackPayAnswer applies one answered tap ask: resolve the source's single
// free mana ability (the exact ma the membership walk recorded, so the
// activation poses no sub-ask), then complete the moment the pool covers the
// charge -- payMana spends it, the window clears, and the declaration
// resumes where handleAttackers parked it (the enlist election, or
// finishAttackers when no attacker enlists).
func (e *Engine) attackPayAnswer(d *decision.Decision, in decision.Intent) {
	st := e.attackPay
	e.choosing = chooseNone
	if st == nil {
		return
	}
	chosen := d.Chosen(in)
	if len(chosen) == 1 && chosen[0].Obj != 0 {
		for _, s := range e.attackManaSources(st.player) {
			if s.id != chosen[0].Obj {
				continue
			}
			e.resolveManaAbilityRefOriginal(st.player, s.id, s.ma, s.original, s.gained, false, false, false)
			break
		}
	}
	if st.charge-e.G.Players[st.player].Pool.Total() <= 0 {
		e.payMana(st.player, Cost{Generic: st.charge})
		e.attackPay = nil
		if !e.startEnlistAsks(st.chosen, st.player) {
			// No attacker enlists: startEnlistAsks reset the election and
			// left the declaration to finish here, the same fallthrough
			// handleAttackers takes when it seeds the election inline.
			e.finishAttackers(st.chosen, st.player)
		}
		return
	}
	if !e.askNextAttackPay() {
		e.attackPay = nil
		// Defensive (the coverage invariant makes this unreachable): the
		// window found no source left to tap, so the charge cannot be paid.
		// Emit ONE loud Note and ABORT -- the empty no-attack declaration --
		// rather than finishing the declaration unpaid (which would commit an
		// attack whose CR 508.1 cost was never paid).
		e.emit(events.Event{Kind: events.Note, Player: st.player,
			Text: fmt.Sprintf("could not pay the {%d} attack cost", st.charge)})
		e.emit(events.Event{Kind: events.DeclareAttackers, Player: e.G.NextAlive(e.G.Active)})
	}
}
