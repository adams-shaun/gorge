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
		if !restrictionPlayerTargetMatches(e.G, sv.Params["Target"], defender, sv.Controller, sv.Source, nil) {
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

// blockTapReq is one tapXType component of a block charge: tap n untapped
// permanents matching spec, with the spec resolved relative to the static's
// source (costCandidates' matchesSpecFrom binding).
type blockTapReq struct {
	n      int32
	spec   string
	source state.ObjID
}

// blockCharge is the composite per-(blocker, attacker) block charge: the
// mana (the flat/Count$/SVar prices, the only component the original
// mana-only model had), the life a PayLife<N> component charges (CR 118.3),
// and the tapXType<N/Spec> tap obligations. The components SUM over every
// matching static, printed or delivered.
type blockCharge struct {
	mana int32
	life int32
	taps []blockTapReq
}

func (c blockCharge) zero() bool { return c.mana == 0 && c.life == 0 && len(c.taps) == 0 }

func (c blockCharge) plus(o blockCharge) blockCharge {
	out := c
	out.mana += o.mana
	out.life += o.life
	out.taps = append(out.taps, o.taps...)
	return out
}

// blockUnlessCharge prices one CantBlockUnless static's Cost$ into a
// composite blockCharge, the block-side sibling of attackUnlessPrice: a
// literal integer directly; an inline Count$ expression or an SVar name on
// the static's own face (with the Effect's frozen ChosenNumber binding when
// the view carries one, so War Cadence's Cost$ XChosen reads
// Count$ChosenNumber); then the cost-token grammar for the non-mana
// components -- PayLife<N> is the life component, tapXType<N/Spec> a tap
// obligation. Anything else -- a dynamic tapXType head (X/Any), a
// Sac</Return</Phyrexian token, an unresolvable SVar -- returns ok=false and
// the caller skips the static (the permissive direction).
func (e *Engine) blockUnlessCharge(sv staticView) (blockCharge, bool) {
	raw := strings.TrimSpace(sv.Params["Cost"])
	if raw == "" {
		return blockCharge{}, false
	}
	ctx := &effects.Ctx{Source: sv.Source, Controller: sv.Controller, SVars: sv.SVars,
		ChosenNumber: sv.ChosenNumber, ChosenNumberBound: sv.chosenNumberBound}
	if n, err := strconv.Atoi(raw); err == nil {
		if n <= 0 {
			return blockCharge{}, false
		}
		return blockCharge{mana: int32(n)}, true
	}
	if strings.HasPrefix(raw, "Count$") {
		n, ok := effects.EvalCountOK(e, ctx, raw)
		if !ok || n <= 0 {
			return blockCharge{}, false
		}
		return blockCharge{mana: n}, true
	}
	if body, ok := sv.SVars[raw]; ok {
		n, ok2 := effects.EvalCountOK(e, ctx, body)
		if !ok2 || n <= 0 {
			return blockCharge{}, false
		}
		return blockCharge{mana: n}, true
	}
	c := e.parseCost(raw)
	// Only the three components the block reader prices may be present: a
	// cost token this build models elsewhere but not here (Sac<, Return<,
	// Blight<, Reveal<, Behold<, Energy, Mill, ...), a coloured/hybrid/
	// Phyrexian pip, a dynamic tap head, an {X} or a {T} leaves the static
	// unpriced rather than degrading any of it to a phantom generic.
	colored := false
	for _, v := range c.Colored {
		if v > 0 {
			colored = true
		}
	}
	if len(c.Unknown) > 0 || c.X > 0 || c.Tap || c.Snow > 0 || c.Forage ||
		colored || len(c.Hybrid) > 0 || len(c.Phyrexian) > 0 ||
		len(c.Twobrid) > 0 || len(c.HybridPhyrexian) > 0 ||
		len(c.Sac) > 0 || len(c.Discard) > 0 || len(c.SubCounter) > 0 ||
		len(c.AddCounter) > 0 || len(c.Exile) > 0 || len(c.Reveal) > 0 ||
		len(c.RevealChosen) > 0 || len(c.Behold) > 0 || len(c.Blight) > 0 ||
		len(c.Draw) > 0 || len(c.Energy) > 0 || len(c.LifeX) > 0 ||
		len(c.Return) > 0 || len(c.PutToLib) > 0 || len(c.DamageYou) > 0 ||
		len(c.MoveToGrave) > 0 || len(c.Mill) > 0 {
		return blockCharge{}, false
	}
	var ch blockCharge
	ch.mana = c.Generic
	ch.life = c.Life
	for _, part := range c.TapPermanent {
		if part.Dyn != "" || part.N <= 0 {
			return blockCharge{}, false
		}
		ch.taps = append(ch.taps, blockTapReq{n: part.N, spec: part.Spec, source: sv.Source})
	}
	if ch.zero() {
		return blockCharge{}, false
	}
	return ch, true
}

// blockStaticMatches evaluates one CantBlockUnless static's two combat
// specs against their respective combat objects: ValidCard$ against the
// BLOCKER, Attacker$ against the ATTACKER; an absent spec is unconditional
// on that half (Awesome Presence scopes only by Attacker$ and still prices
// every blocker against its enchanted attacker; War Cadence scopes neither
// and prices every blocker in the game).
func (e *Engine) blockStaticMatches(sv staticView, sc effects.SpecContext, blocker, attacker state.ObjID) bool {
	if spec := sv.Params["ValidCard"]; spec != "" && !e.matchesSpec(spec, blocker, sc) {
		return false
	}
	if spec := sv.Params["Attacker"]; spec != "" && !e.matchesSpec(spec, attacker, sc) {
		return false
	}
	return true
}

// blockPairCharge is the per (blocker, attacker) composite charge from
// block-prop statics, the sum of every live face static that admits the
// pair and prices PLUS every delivered one: an Effect's
// StaticAbilities$ CantBlockUnless registration (War Cadence) and an
// Animate's staticAbilities$ grant (Whipgrass Entangler) register into the
// continuous-effect registry with the granting ability's SVar table, and
// the registry walk below consults them beside the printed statics with the
// same absent-spec-is-unconditional read. It is a pure read (no event, no
// state write) and deterministic (both walks are the fixed scans every
// static consumer shares), so the offer list, the validator, the bot guard
// and the payer all re-derive the same charge.
func (e *Engine) blockPairCharge(blocker, attacker state.ObjID) blockCharge {
	total := blockCharge{}
	for _, sv := range e.activeStatics("CantBlockUnless") {
		if !cantAttackUnlessParamsReadable(sv.Params) || !e.continuousGateHolds(sv) {
			continue
		}
		if !e.blockStaticMatches(sv, e.specCtxSVars(sv.Source, sv.Controller, sv.SVars), blocker, attacker) {
			continue
		}
		if ch, ok := e.blockUnlessCharge(sv); ok {
			total = total.plus(ch)
		}
	}
	for _, ce := range e.active() {
		if ce.Restriction != "CantBlockUnless" {
			continue
		}
		sv := staticView{Source: ce.Source, Controller: ce.Controller,
			Params: ce.RestrictParams, SVars: ce.RestrictSVars,
			ChosenNumber: ce.ChosenNumber, chosenNumberBound: true}
		if !cantAttackUnlessParamsReadable(sv.Params) || !e.continuousGateHolds(sv) {
			continue
		}
		sc := e.specCtx(ce.Source, ce.Controller)
		for _, r := range ce.Remembered {
			sc.Remembered = append(sc.Remembered, state.Target{Obj: r})
		}
		if !e.blockStaticMatches(sv, sc, blocker, attacker) {
			continue
		}
		if ch, ok := e.blockUnlessCharge(sv); ok {
			total = total.plus(ch)
		}
	}
	return total
}

// blockChargeOf sums the composite charges of a whole chosen declaration --
// the one charge derivation validateBlockers and handleBlockers share.
func (e *Engine) blockChargeOf(chosen []decision.Option) blockCharge {
	total := blockCharge{}
	for _, opt := range chosen {
		total = total.plus(e.blockPairCharge(opt.Obj, opt.Attacker))
	}
	return total
}

// blockTapCandidates lists the payer's untapped permanents that satisfy one
// tap obligation's spec, minus the given exclusions -- the declaration's own
// committed blockers, which are "declared as a blocking creature this
// combat" the moment the declaration lands even though the DeclareBlockers
// event has not been applied yet (Hollow Warrior's Creature.!blocking).
func (e *Engine) blockTapCandidates(p state.PlayerID, t blockTapReq, excluded map[state.ObjID]bool) []state.ObjID {
	cands := e.costCandidates(p, t.source, state.ZBattlefield, t.spec, false, true)
	out := cands[:0]
	for _, id := range cands {
		if excluded[id] {
			continue
		}
		out = append(out, id)
	}
	return out
}

// blockTapPlan reserves the exact permanents a charge's tap obligations
// take: per requirement in charge order, the first unreserved candidates in
// zone order (deterministic; R-9 -- the build never asks which permanents
// to tap, the same no-ask reading every deterministic cost payment takes).
// A requirement with fewer eligible candidates than n reserves what exists;
// callers that need the full charge check blockChargeAffordable first.
func (e *Engine) blockTapPlan(p state.PlayerID, c blockCharge, excluded map[state.ObjID]bool) []state.ObjID {
	plan := make([]state.ObjID, 0, len(c.taps))
	for _, t := range c.taps {
		taken := int32(0)
		for _, id := range e.blockTapCandidates(p, t, excluded) {
			if taken >= t.n {
				break
			}
			reserved := false
			for _, pid := range plan {
				if pid == id {
					reserved = true
					break
				}
			}
			if reserved {
				continue
			}
			plan = append(plan, id)
			taken++
		}
	}
	return plan
}

// blockChargeAffordable reports whether the payer can pay every component
// of the charge: mana within the block budget (blockManaBudget), life
// within the payer's total (CR 118.3 / 119.4), and enough untapped matching
// permanents for every tap obligation. excluded holds any permanent the
// declaration already commits. This is the ONE affordability read: the
// offer gate (askBlockers), the whole-declaration validator (validateBlockers)
// and the payment all re-derive from it, so they cannot disagree about what
// is payable.
func (e *Engine) blockChargeAffordable(p state.PlayerID, c blockCharge, excluded map[state.ObjID]bool) bool {
	if e.blockManaBudget(p) < c.mana {
		return false
	}
	if e.G.Players[p].Life < c.life {
		return false
	}
	need := int32(0)
	for _, t := range c.taps {
		need += t.n
	}
	return int32(len(e.blockTapPlan(p, c, excluded))) >= need
}

// payBlockExtras settles a completed block charge's non-mana components:
// one LifeChange per life component (the same event shape payMana's Life
// part emits, so life-loss replacements see it) and one Tap event per
// planned permanent (the same event the cast tap costs emit). The order is
// fixed -- life, then taps -- and both fire only after the charge's mana is
// covered, so a defensive window strand can never leave a half-paid
// composite behind.
func (e *Engine) payBlockExtras(p state.PlayerID, c blockCharge, taps []state.ObjID) {
	if c.life > 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: p, Amount: -c.life})
	}
	for _, id := range taps {
		e.emit(events.Event{Kind: events.Tap, Obj: id, Text: "tapped as a block cost"})
	}
}

// chosenBlockers is the set of permanents a declaration commits to blocking;
// they are excluded from the tap-cost candidate pool.
func chosenBlockers(chosen []decision.Option) map[state.ObjID]bool {
	m := make(map[state.ObjID]bool, len(chosen))
	for _, o := range chosen {
		m[o.Obj] = true
	}
	return m
}

// blockManaBudget is the defending player's available payment budget. Unlike
// attackers, that player has had priority since attackers were declared, so
// floating mana is part of the normal declaration path.
func (e *Engine) blockManaBudget(p state.PlayerID) int32 { return e.attackBudget(p) }

type blockPayWindow struct {
	chosen []decision.Option
	player state.PlayerID
	charge blockCharge
	// taps is the tap plan frozen when the window opened, against the board
	// as it stood -- the mana window's own taps can remove a candidate
	// (tapping a creature land), so the plan is computed once and paid
	// verbatim at completion rather than re-derived.
	taps []state.ObjID
	// sources is the tap list the CURRENT ask posed, in option order: the
	// answer resolves its exact entry by the chosen option's Index (see
	// paySourceForAnswer), the same alternative-identity rule the attack
	// window follows.
	sources []attackManaSource
}

func (e *Engine) startBlockPay(chosen []decision.Option, player state.PlayerID, charge blockCharge) bool {
	if e.blockManaBudget(player) < charge.mana {
		return false
	}
	e.blockPay = &blockPayWindow{chosen: chosen, player: player, charge: charge,
		taps: e.blockTapPlan(player, charge, chosenBlockers(chosen))}
	e.askNextBlockPay()
	return true
}

func (e *Engine) askNextBlockPay() bool {
	st := e.blockPay
	if st == nil {
		return false
	}
	remaining := st.charge.mana - e.G.Players[st.player].Pool.Total()
	if remaining <= 0 {
		return false
	}
	sources := e.attackManaSources(st.player)
	if len(sources) == 0 {
		return false
	}
	st.sources = sources
	d := &decision.Decision{Player: st.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: fmt.Sprintf("Pay %d to block -- tap a mana source", st.charge.mana)}
	for _, s := range sources {
		name := "a permanent"
		if o := e.G.Obj(s.id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "block_mana", Obj: s.id,
			Label: fmt.Sprintf("Tap %s for %s", name, s.prod)})
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
		if s, ok := paySourceForAnswer(st.sources, chosen[0]); ok {
			e.resolveManaAbilityRefOriginal(st.player, s.id, s.ma, s.original, s.gained, false, false, false)
		}
	}
	if st.charge.mana-e.G.Players[st.player].Pool.Total() <= 0 {
		e.payMana(st.player, Cost{Generic: st.charge.mana})
		e.payBlockExtras(st.player, st.charge, st.taps)
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
	// prod is the exact production one activation yields, rendered as braced
	// symbols ("{R}", "{C}{C}"): the tap option's label, so the
	// alternatives of one multi-ability permanent are distinguishable on the
	// wire and the payer taps the ability it meant to tap.
	prod string
}

// manaUnitsLabel renders a production as braced mana symbols, e.g. "{R}{R}":
// counts[i] slots of symbol i, each scaled by amt. It is the tap option's
// production half of the label.
func manaUnitsLabel(counts [6]int32, amt int32) string {
	var b strings.Builder
	for i := range counts {
		for n := int32(0); n < counts[i]*amt; n++ {
			b.WriteString("{")
			b.WriteString(string(cards.ManaSymbol(i)))
			b.WriteString("}")
		}
	}
	return b.String()
}

// paySourceForAnswer resolves the EXACT source a tap answer selected. The
// option's Index is its position in the list the window posed and the window
// stores that list (attackPayWindow.sources / blockPayWindow.sources), so a
// multi-ability permanent's second alternative resolves ITS ability, not the
// first alternative that shares its Obj -- the round-1 defect: duplicate-Obj
// options resolved the first ability while the budget counted the maximum.
// The Obj cross-check keeps a malformed answer from activating another
// object's ability; on a mismatch or an out-of-range Index the legacy
// first-match-by-Obj scan applies, so an old-style answer still pays.
func paySourceForAnswer(sources []attackManaSource, opt decision.Option) (attackManaSource, bool) {
	if opt.Index >= 0 && opt.Index < len(sources) && sources[opt.Index].id == opt.Obj {
		return sources[opt.Index], true
	}
	for _, s := range sources {
		if s.id == opt.Obj {
			return s, true
		}
	}
	return attackManaSource{}, false
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
			out = append(out, attackManaSource{id: u.id, ma: a.ma, original: a.ma, gained: e.gainedManaRefFor(p, u.id, a.ma), units: units, prod: manaUnitsLabel(a.counts, a.amt)})
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
			rewritten := withProduced(ma, ma, oneColourProduced(counts))
			pc, _ := cards.ProducedCounts(oneColourProduced(counts))
			out = append(out, attackManaSource{id: id, ma: rewritten, original: ma, gained: gained, units: units, prod: manaUnitsLabel(pc, amt)})
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
	for tok := range strings.FieldsSeq(produced) {
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
	// battle is the non-player permanent being attacked -- a CR 310.7 battle
	// or a planeswalker (CR 508.1) -- or 0 for a player attack. def is the
	// permanent's seat for a permanent attack (a battle's protector, or the
	// planeswalker's controller; it is what blocks and what every
	// player-scoped restriction reads), so the two fields together are the
	// whole defender. Despite the historical field name, the value covers both
	// permanent kinds; the recipient's face decides the damage conversion
	// (defense counters vs loyalty) in events.Apply's Damage fold.
	battle state.ObjID
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
// player-attack duties (named or goad) than the creature's best available
// defender is dropped.
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
	// CR 310.7: a battle a player protects is a legal defender for that
	// player's opponents, offered in addition to (not instead of) the player
	// themselves. battleDefenders names each such battle ONCE per (protector,
	// battle) pair, in a deterministic order: the defender enumeration above
	// (ascending seat), then each protected player's battle in that player's
	// battlefield zone order. Only the protector's opponents may attack it, so
	// the active player is excluded here exactly as it is above.
	battles := e.battleDefenders(p)
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
		var walkerTargets []state.ObjID
		for _, wid := range e.G.Zone(state.ZBattlefield, d) {
			o := e.G.Obj(wid)
			if o != nil && !o.FaceDown && o.Face() != nil && o.Face().IsPlaneswalker() {
				walkerTargets = append(walkerTargets, wid)
			}
		}
		for _, id := range e.G.Zone(state.ZBattlefield, p) {
			if !e.canAttackPair(id, d) || !e.goadMayAttack(id, d) || e.attackBlocked(id, d) {
				continue
			}
			price := e.attackPairCharge(id, d)
			if price > 0 && budget < price {
				continue
			}
			out = append(out, attackOffer{id: id, def: d, price: price})
			// CR 508.1: a planeswalker on d's battlefield is a legal defender
			// for d's opponents, offered in addition to the player themselves
			// (defender-major: the player's own pair block, then its
			// planeswalkers). The walk happens in the planeswalker's
			// controller's zone order, and d IS that controller here.
			for _, wid := range walkerTargets {
				out = append(out, attackOffer{id: id, def: d, battle: wid, price: price})
			}
		}
		// This protector's battles, immediately after the protector's own
		// pair block (defender-major: one defender slot at a time).
		for _, b := range battles {
			if b.protector != d {
				continue
			}
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
				out = append(out, attackOffer{id: id, def: d, price: price, battle: b.id})
			}
		}
	}
	// Best player-attack duty satisfaction per creature over surviving pairs.
	best := make(map[state.ObjID]int)
	for _, of := range out {
		if n := reqs[of.id].satisfiedByOffer(of); n > best[of.id] {
			best[of.id] = n
		}
	}
	keep := out[:0]
	for _, of := range out {
		rs := reqs[of.id]
		if rs.any() && rs.satisfiedByOffer(of) < best[of.id] {
			continue
		}
		keep = append(keep, of)
	}
	return keep
}

// battleTarget is one attackable battle and the player who protects it.
type battleTarget struct {
	id        state.ObjID
	protector state.PlayerID
}

// battleDefenders lists every battle the active player p may attack under
// CR 310.7: the battle is on the battlefield, is a battle (its printed face,
// and not face down -- a face-down permanent is a vanilla 2/2 creature, CR
// 708.5), has a recorded protector whose seat is still in the game, and that
// protector is not p itself. It is the ONE eligibility home: attackOffers
// enumerates from it, and canAttackBattle below reads it through this list,
// so the offer list and the legality guard can never disagree about which
// battles are attackable. Order is deterministic: protector ascending, then
// the protector's battlefield zone order (the battle's own controller may
// differ from its protector, but a battle is only ever offered under the
// protector's slot, and every controller's battlefield is walked in the
// ascending seat order attacks already use).
func (e *Engine) battleDefenders(p state.PlayerID) []battleTarget {
	var out []battleTarget
	for _, ctrl := range e.G.AliveFrom(0) {
		for _, id := range e.G.Zone(state.ZBattlefield, ctrl) {
			o := e.G.Obj(id)
			if o == nil || o.FaceDown || o.Face() == nil || !o.Face().IsBattle() {
				continue
			}
			if !o.ProtectorValid || o.Protector == p ||
				int(o.Protector) >= len(e.G.Players) || e.G.Players[o.Protector].Lost {
				continue
			}
			out = append(out, battleTarget{id: id, protector: o.Protector})
		}
	}
	return out
}

// canAttackBattle reports whether p may declare an attack at a non-player
// permanent id -- a CR 310.7 battle or a planeswalker (CR 508.1) -- reading
// the same eligibility attacks enumerates: a battle comes from battleDefenders
// (a battle the active player protects, a protectorless battle, or a battle
// that has left the battlefield is not attackable), and a planeswalker is one
// on p's opponent's battlefield, face up (a face-down permanent is a vanilla
// 2/2 creature, CR 708.5).
func (e *Engine) canAttackBattle(id state.ObjID, p state.PlayerID) bool {
	for _, b := range e.battleDefenders(p) {
		if b.id == id {
			return true
		}
	}
	o := e.G.Obj(id)
	if o != nil && !o.FaceDown && o.Zone == state.ZBattlefield && o.Face() != nil && o.Face().IsPlaneswalker() {
		return o.Controller != p
	}
	return false
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
	// sources is the tap list the CURRENT ask posed, in option order: the
	// answer resolves its exact entry by the chosen option's Index (see
	// paySourceForAnswer), so a permanent with several free abilities pays
	// with the alternative the payer selected.
	sources []attackManaSource
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
	st.sources = sources
	d := &decision.Decision{Player: st.player, Kind: decision.KChoose, Min: 1, Max: 1,
		Prompt: fmt.Sprintf("Pay {%d} to attack -- tap a mana source", st.charge)}
	for _, s := range sources {
		name := "a permanent"
		if o := e.G.Obj(s.id); o != nil && o.Face() != nil {
			name = o.Face().Name
		}
		d.Options = append(d.Options, decision.Option{Index: len(d.Options),
			Kind: "attack_mana", Obj: s.id,
			Label: fmt.Sprintf("Tap %s for %s", name, s.prod)})
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
		if s, ok := paySourceForAnswer(st.sources, chosen[0]); ok {
			e.resolveManaAbilityRefOriginal(st.player, s.id, s.ma, s.original, s.gained, false, false, false)
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
