package rules

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

// potentialUnbounded is the per-unit amount one indeterminate mana source
// contributes to the hypothetical potential pool. A source whose Amount$ is
// an X, a Y or a Count$ expression (an Urza land, Gaea's Cradle, Priest of
// Titania) could produce any amount this turn; the stop decision the
// projection serves must never lose an action to a source the engine cannot
// statically price, so such a source is priced unbounded rather than at zero.
// It is math.MaxInt32, the largest representable mana cost. Potential-pool
// additions saturate at that sentinel and Mana.Total does too, so several
// open sources cannot wrap the hypothetical pool below a payable cost.
const potentialUnbounded int32 = math.MaxInt32

// PotentialMana is the hypothetical pool the potential-action walk prices
// against: the seat's floating pool PLUS what every untapped mana source it
// controls could produce. It is deliberately an OVER-bound, the direction the
// auto-pass doctrine names safe (a wrongly withheld pass costs one idle stop;
// a wrongly eaten window loses the player's action):
//
//   - a source with a single fixed-colour production (a Plains) adds its
//     literal Amount (default 1);
//   - a source with several fixed productions (a Volcanic Island's two
//     abilities, a "Produced$ GW" line) adds every face -- the seat can only
//     tap for one at a time, but the bound must not lose a colour;
//   - a source with an alternative or variable production (Produced$ Any /
//     Combo Any / Chosen / an unknown token) or an indeterminate Amount$ (an
//     Urza land, Gaea's Cradle, Elvish Archdruid) contributes
//     potentialUnbounded to EVERY unit -- the one shape a fixed vector cannot
//     represent honestly, and the shape the old client-side bound priced at
//     zero (the Tron + Karn defect);
//   - a mana ability WITH a paid activation cost (Wasteland's
//     "T, Sac<1/CARDNAME>", a "{1}, {T}: add {C}{C}" source) still counts, and
//     it counts even though the real floating pool does not already cover its
//     cost: the seat can tap a free source first, spend that mana on the paid
//     activation, and then tap the paid source. PotentialMana therefore runs a
//     FIXPOINT -- a source's paid mana ability is admitted the moment the
//     accumulated pool can cover its activation cost, and its own production
//     then feeds the next source -- so the pool grows the way a player would
//     actually sequence the activations (the Jitte-round finding: a Plains
//     plus an untapped "{1}, {T}: add {C}{C}" source must cover a {2} spell,
//     because the seat taps the Plains for {W}, pays the {1} to activate the
//     source, and casts from the {C}{C} it produced).
//
// Restriction-gated abilities (CantBeActivated) contribute nothing, and a
// source's own production never pays for that source's own paid activation
// (a source is admitted only when the pool built from OTHER sources already
// covers its cost, and each source contributes at most once -- the same
// single-tap-per-source semantics the engine's own tap-for-mana offer uses).
// The aggregate is a pure read: no event is emitted and no state field is
// written; the pool lives only in this return value.
//
// Determinism: the walk is over zone order, never a map range (the added set
// is only indexed, never ranged), so the aggregate is byte-stable run to run.
//
// Cost: the fixpoint asks EVERY battlefield object for its mana abilities on
// every pass, and that walk reads the board's Continuous statics (its
// AddAbility$ grant scan). Being a pure read, the whole fixpoint runs in one
// Derived memo scope (engine/rules/derivedmemo.go), so activeStatics and the other
// board-only walk caches (engine/rules/walkcache.go) scan the battlefield once per
// call instead of once per object -- outside a scope each object's walk
// rescanned the whole board, O(permanents^2) per call, which is what ran a
// Krenko token board (8k-16k goblins, cardfuzz seed 6181111140895991800)
// past the fuzzer's 90s hang budget. A caller already inside a walk shares
// that walk's generation.
func (e *Engine) PotentialMana(p state.PlayerID) state.Mana {
	return e.potentialManaFor(p, nil)
}

// potentialManaFor is PotentialMana with an optional payment: when for is
// non-nil, a mana ability whose RestrictValid$ cannot spend on that payment
// (pay.RestrictValidMatches against the producing source) contributes
// nothing, and the floating pool it starts from holds only the restricted
// batches that payment may spend (pay.AvailableFor). Secluded Courtyard's
// "one mana of any color, spend only to cast a creature spell of the chosen
// type or activate an ability of a creature of the chosen type" cannot fund Frenzied Goblin's triggered "you may pay
// {R}", so the resolution-time pay ask (resPayNeedsMana) must not count it
// (CR 106.6: the restriction limits what that mana is spent on).
func (e *Engine) potentialManaFor(p state.PlayerID, payment *paymentDescriptor) state.Mana {
	e.beginDerivedMemo()
	defer e.endDerivedMemo()
	// The decision's recorded priority walk, when its caller armed it
	// (potentialWalkOf), serves each object's membership list
	// (walk_block_reuse.go); a nested call never sees it.
	rec := e.potentialManaRec
	e.potentialManaRec = nil
	out := e.G.Players[p].Pool
	if payment != nil {
		// The floating pool counts only the mana this payment may spend: a
		// restricted batch already in the pool is as unspendable here as the
		// restricted ability that would make it (CR 106.6). Chandra, Chill of
		// Compliance's noncreature-spell {U} floating beside one untapped
		// Swamp does not pay Lich's Relic's triggered {2}.
		out = pay.AvailableFor(asPayer(e), p, *payment).Pool
	}
	// A source's abilities are admitted independently: one whose paid
	// activation the pool could not cover yet (Heap Gate's "{1}, {T}: Add
	// one mana of any color" before any other source floated mana) is
	// admitted on a later pass, even when the same source's free ability
	// was already counted -- otherwise the bound depended on zone order and
	// could miss a colour only the paid ability makes (a sound upper bound
	// may count both of a source's abilities; it already did when both were
	// payable on the same pass).
	//
	// Each object's membership list (below) reads the board only, never the
	// accumulated pool, and the fixpoint changes no state, so it is computed
	// once per object on the first pass and reused by every later pass; only
	// the pool-priced payability filter reruns. Indexed by zone position: the
	// zone does not move while the fixpoint runs. The lists live in one flat
	// scratch array kept in e's hypothetical pool (hypclone.go: so a recycled
	// clone inherits it), taken for the call so a nested call builds its
	// own; each object owns a span of it, and admitted parallels it.
	zone := e.G.Zone(state.ZBattlefield, p)
	pl := e.hypPool()
	sc := pl.pm
	pl.pm = potentialManaScratch{}
	flat, admitted := sc.members[:0], sc.admitted[:0]
	spans := slices.Grow(sc.spans[:0], len(zone))[:len(zone)]
	clear(spans)
	own := slices.Grow(sc.own[:0], len(zone))[:len(zone)]
	clear(own) // each source's counted production
	// An object the offer walk's mana section provably skips (manaWalkEmpty:
	// no mana ability reaches p, or a tapped source whose every mana ability
	// costs {T}, which manaAbilityPayablePool never admits) contributes
	// nothing on any pass, so its membership walk is skipped too. The board
	// facts are read once, outside every face probe.
	lw := legalWalk{e: e, p: p, actionStatics: actionStaticSource{e: e}}
	if rec != nil && rec.p == p {
		e.recordedBoardFacts(rec, &lw.actionStatics, p)
	}
	board := lw.boardFacts()
	restricted := false
	tapExtra := e.tapExtraTriggerOnBoard()
	var sparePass state.Mana // the fee-only mana the last pass found (potentialSpareGrew)
	for {
		progressed := false
		for zi, id := range zone {
			o := e.G.Obj(id)
			if o == nil || o.Face() == nil {
				continue
			}
			sp := &spans[zi]
			// Use the payment-window membership walk, but defer its live-pool
			// payability gate: this fixpoint prices activation costs against the
			// accumulated hypothetical pool below. That shared walk includes
			// granted CR 305.6 intrinsics and all current eligibility gates.
			if !sp.walked {
				sp.walked = true
				sp.start = int32(len(flat))
				if mem, ok := rec.potentialMembers(p, zi); ok {
					if walkCacheVerify {
						var fresh []*cards.SA
						if !lw.manaWalkEmpty(board, o, id, o.Face()) {
							fresh = e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true)
						}
						if !slices.EqualFunc(fresh, mem, pay.SameManaAbility) {
							panic(fmt.Sprintf("rules: PotentialMana membership for %d served from the priority walk differs", id))
						}
					}
					flat = append(flat, mem...)
					e.walkMembersServed++
				} else if lw.manaWalkEmpty(board, o, id, o.Face()) {
					if potentialMembersVerify {
						e.verifyPotentialSkip(p, o, id)
					}
				} else {
					flat = e.appendAvailableManaAbilitiesGate(flat, nil, p, id, true)
				}
				sp.end = int32(len(flat))
				for range sp.end - sp.start {
					admitted = append(admitted, false)
				}
			} else if potentialMembersVerify && sp.end > sp.start {
				if fresh := e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true); !slices.EqualFunc(fresh, flat[sp.start:sp.end], pay.SameManaAbility) {
					panic(fmt.Sprintf("rules: PotentialMana membership for %d moved inside the fixpoint", id))
				}
			}
			if sp.end == sp.start {
				continue
			}
			// Price every not-yet-admitted member against the pool built
			// from OTHER sources (a source's own production never funds its
			// own paid activation: one tap cannot do both), then add the
			// admitted ones together. others is a copy taken before any of
			// this pass's admissions, so adding each admitted member at once
			// is the same sum as adding them after the pricing loop.
			others := out
			for c := range others {
				others[c] -= own[zi][c]
			}
			before := out
			admittedAny := false
			for k := sp.start; k < sp.end; k++ {
				if ma := flat[k]; !admitted[k] && potentialRestrictionAdmits(e, p, id, ma, payment) && !potentialIdentityEmpty(e, p, ma) &&
					(e.manaAbilityPayablePool(p, id, ma, &others) ||
						payment != nil && e.potentialFeeSpareCovers(p, payment, id, ma, zone, spans, flat, &others)) {
					admitted[k] = true
					admittedAny = true
					if effects.ManaOf(ma).RestrictValid != "" {
						restricted = true
					}
					// A payment's bound reads a Chosen source's colour.
					if payment == nil || !addPotentialChosenColour(e, &out, id, ma) && !addPotentialIdentityColours(e, &out, p, ma) {
						if !addPotentialReflected(e, &out, p, id, ma) {
							e.addPotentialManaOf(&out, ma)
							// A Condition-split ability adds its sub's production instead
							// when the gate holds (mana_condsplit.go): count both.
							if sub := condSplitLiveSub(e, p, id, ma); sub != nil {
								e.addPotentialManaOf(&out, sub)
							}
						}
					}
					if tapExtra {
						e.addPotentialTapExtra(&out, id, ma)
					}
				}
			}
			if !admittedAny {
				if payment != nil && e.potentialSpareGrew(p, payment, zone, spans, flat, &sparePass) {
					progressed = true // a refused source later in the zone now funds a fee
				}
				continue
			}
			progressed = true
			for c := range out {
				own[zi][c] += out[c] - before[c]
			}
		}
		if !progressed {
			break
		}
	}
	clear(flat)
	e.hypPool().pm = potentialManaScratch{members: flat[:0], admitted: admitted[:0], spans: spans[:0], own: own[:0]}
	if payment == nil {
		e.potentialRestricted = restricted
	}
	return out
}

// potentialFeeSpare is the mana of p's untapped sources whose RestrictValid$
// the payment refuses but an activation of source act admits: it cannot pay
// the spell (CR 106.6) yet pays a mana ability's own fee (Hydraulic Helper's
// "can't be spent to cast a nonartifact spell" {U} pays Surveillance Room's
// {1} when the Room's mana pays the spell). Members of the zone walked so far
// are read from the fixpoint's scratch; act's own production never funds its
// own fee.
func (e *Engine) potentialFeeSpare(p state.PlayerID, payment *paymentDescriptor, act state.ObjID, zone []state.ObjID, spans []potentialManaSpan, flat []*cards.SA, others *state.Mana) state.Mana {
	var spare state.Mana
	for zi, zid := range zone {
		if zid == act || !spans[zi].walked {
			continue
		}
		for k := spans[zi].start; k < spans[zi].end; k++ {
			ma := flat[k]
			rv := effects.ManaOf(ma).RestrictValid
			if rv == "" || pay.RestrictValidMatches(asPayer(e), p, *payment, rv, zid) {
				continue // unrestricted or admitted by the spell: the fixpoint's own sum
			}
			d := paymentDescriptor{Class: pay.PurposeActivated, ID: act}
			if pay.RestrictValidMatches(asPayer(e), p, d, rv, zid) && e.manaAbilityPayablePool(p, zid, ma, others) {
				e.addPotentialManaOf(&spare, ma)
			}
		}
	}
	return spare
}

// potentialFeeSpareCovers reports whether ma's paid activation is payable
// from the pool of the other sources plus the fee-only mana.
func (e *Engine) potentialFeeSpareCovers(p state.PlayerID, payment *paymentDescriptor, id state.ObjID, ma *cards.SA, zone []state.ObjID, spans []potentialManaSpan, flat []*cards.SA, others *state.Mana) bool {
	spare := e.potentialFeeSpare(p, payment, id, zone, spans, flat, others)
	if spare == (state.Mana{}) {
		return false
	}
	withSpare := *others
	for c := range withSpare {
		withSpare[c] = saturatingPotentialMana(withSpare[c], spare[c])
	}
	return e.manaAbilityPayablePool(p, id, ma, &withSpare)
}

// potentialSpareGrew reports whether the fee-only mana grew since the last
// pass (last holds the previous total), which a source earlier in the zone may
// now spend.
func (e *Engine) potentialSpareGrew(p state.PlayerID, payment *paymentDescriptor, zone []state.ObjID, spans []potentialManaSpan, flat []*cards.SA, last *state.Mana) bool {
	var none state.Mana
	now := e.potentialFeeSpare(p, payment, 0, zone, spans, flat, &none)
	grew := now != *last
	*last = now
	return grew
}

// tapExtraTriggerOnBoard reports whether any object prints a static
// TapsForMana trigger (CR 605.1b), the only carriers whose extra mana a
// source's tap adds to the bound. The carrier list is memoised and the
// common board's is empty, so the scan is the list walk itself.
func (e *Engine) tapExtraTriggerOnBoard() bool {
	for _, oid := range pay.PaymentPlanInterferenceCarriers(asPayer(e)) {
		o := e.G.Obj(oid)
		if o == nil || o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		for _, t := range o.Face().Triggers {
			if t.Mode == "TapsForMana" {
				return true
			}
		}
	}
	return false
}

// addPotentialTapExtra folds into the bound the additional mana the folded
// static TapsForMana triggers add when id taps for ma (CR 106.12a, 605.4a):
// Groundchuck & Dirtbag's {G} beside a land's own mana. A tap fires the
// trigger once whichever colour it takes, so the bound takes the largest
// extra per colour over the colours the ability can produce, and an ability
// that does not tap adds none.
func (e *Engine) addPotentialTapExtra(m *state.Mana, id state.ObjID, ma *cards.SA) {
	most := e.potentialTapExtra(id, ma)
	for i, n := range most {
		if n > 0 {
			m[i] = saturatingPotentialMana(m[i], n)
		}
	}
}

// potentialTapExtra is the largest extra per colour the static TapsForMana
// triggers add when id taps for ma, over the colours ma can produce; zero for
// an ability that does not tap.
func (e *Engine) potentialTapExtra(id state.ObjID, ma *cards.SA) state.Mana {
	var most state.Mana
	if !e.abilityCompiledCost(ma).Cost.Tap {
		return most
	}
	for _, produced := range pay.PaymentPlanProductions(asPayer(e), id, ma) {
		x := e.paymentPlanTapExtraMana(id, produced)
		for i := range most {
			most[i] = max(most[i], x[i])
		}
	}
	return most
}

// potentialPoolFor is the hypothetical bound a payment may be priced against:
// hyp itself while no restricted mana ability fed the bound, else the bound
// rebuilt admitting only the restricted mana whose RestrictValid$ this
// payment satisfies (CR 106.6), so an Orb of Dragonkind's mana funds a
// Dragon's ability as a potential action and never an Equip.
func (e *Engine) potentialPoolFor(p state.PlayerID, d paymentDescriptor, hyp state.Mana) state.Mana {
	if d.OnlyTag == "" && (!e.potentialRestricted || e.potentialWitness) {
		// A witness walk's pool is what the plan floats, built by the payment
		// planner for this very payment: it already spent only mana the
		// restriction admits. Rebuilding it from the board's over-bound (which
		// counts a Combo source in both its colours) would price the offer
		// against mana the plan never makes.
		return hyp
	}
	return e.potentialManaFor(p, &d)
}

// potentialRestrictionAdmits reports whether mana ability ma of source id
// may count toward payment (potentialManaFor): always without a payment or
// without a RestrictValid$, else when the restriction admits it.
// potentialIdentityEmpty reports a "mana of any color in your commander's
// color identity" ability whose payer has no such colour (no commander, or a
// colourless one): it makes nothing (CR 903.4f), so no payment counts it, as
// the activation itself adds none (effMana) and windowSourceYields skips it.
func potentialIdentityEmpty(e *Engine, p state.PlayerID, ma *cards.SA) bool {
	return pay.IsColourIdentityProduced(effects.ManaOf(ma).Produced) && len(pay.CommanderIdentityColours(asPayer(e), p)) == 0
}

func potentialRestrictionAdmits(e *Engine, p state.PlayerID, id state.ObjID, ma *cards.SA, payment *paymentDescriptor) bool {
	if payment == nil {
		return true
	}
	if payment.OnlyTag != "" {
		// An AlternativeCost ManaRestriction$: only a source whose production
		// carries the tag can fund the payment (the producer tag the
		// activation would stamp on the mana).
		if tag, _ := effects.ManaProducerTag(e, id); !pay.OnlyTagAdmitsProducer(payment.OnlyTag, tag) {
			return false
		}
	}
	rv := effects.ManaOf(ma).RestrictValid
	return rv == "" || pay.RestrictValidMatches(asPayer(e), p, *payment, rv, id)
}

// addPotentialChosenColour adds a Produced$ Chosen ability's exact
// production -- its source's as-enters colour (Heraldic Banner's "Add one
// mana of the chosen color"; a gate land's "Combo U Chosen" adds the listed
// colours beside it) -- and reports whether it did. The plain
// PotentialMana over-bound prices Chosen unbounded in every colour; a
// payment's bound (potentialManaFor) must not, or a triggered "you may pay
// {R}" is offered to a seat whose only untapped source makes {W} (CR
// 106.1a). False (the caller's ordinary fold) for any other production, an
// indeterminate amount, or a source with no chosen colour.
func addPotentialChosenColour(e *Engine, m *state.Mana, id state.ObjID, ma *cards.SA) bool {
	mp := effects.ManaOf(ma)
	// the colours a "Combo U Chosen" gate land lists beside the chosen one
	var fixed []string
	if mp.Produced != "Chosen" && mp.Produced != "ChosenColor" { // Produced is already trimmed
		fixed = strings.Fields(mp.Produced)
		if len(fixed) < 2 || fixed[0] != "Combo" || fixed[len(fixed)-1] != "Chosen" {
			return false
		}
		fixed = fixed[1 : len(fixed)-1]
		for _, f := range fixed {
			if len(f) != 1 || state.ManaIndex(f[0]) == state.MC {
				return false
			}
		}
	}
	o := e.G.Obj(id)
	if o == nil || len(o.ChosenColor) != 1 {
		return false
	}
	i := state.ManaIndex(o.ChosenColor[0])
	if i == state.MC { // not a WUBRG letter
		return false
	}
	amt, indeterminate := potentialAmount(mp)
	if indeterminate {
		return false
	}
	m[i] = saturatingPotentialMana(m[i], amt)
	// a Combo gate land makes one mana of any listed colour or the chosen one:
	// each colour is bounded as a dual's face is (the kernel's exact mask)
	for _, f := range fixed {
		if j := state.ManaIndex(f[0]); j != i {
			m[j] = saturatingPotentialMana(m[j], amt)
		}
	}
	return true
}

// addPotentialIdentityColours adds a commander-identity ability's exact
// production -- one mana of each colour in its payer's commanders' colour
// identity (Arcane Signet, Command Tower: CR 903.4) -- and reports whether it
// did. The plain PotentialMana over-bound prices the ColorIdentity word
// unbounded in every colour; a payment's bound must not, or a triggered "you
// may pay {5}{R}{R}" is offered to a seat whose only untapped source is one
// Arcane Signet (CR 106.1a, 118.3). False (the caller's ordinary fold) for any
// other production or an indeterminate amount; an empty identity never gets
// here (potentialIdentityEmpty).
func addPotentialIdentityColours(e *Engine, m *state.Mana, p state.PlayerID, ma *cards.SA) bool {
	mp := effects.ManaOf(ma)
	if !pay.IsColourIdentityProduced(mp.Produced) {
		return false
	}
	amt, indeterminate := potentialAmount(mp)
	if indeterminate {
		return false
	}
	for _, c := range pay.CommanderIdentityColours(asPayer(e), p) {
		if i := state.ManaIndex(c[0]); i != state.MC {
			m[i] = saturatingPotentialMana(m[i], amt)
		}
	}
	return true
}

// potentialManaScratch is PotentialMana's reusable fixpoint storage (see
// there), kept in the engine's hypSparePool. Clone copies none.
type potentialManaScratch struct {
	members  []*cards.SA
	admitted []bool
	spans    []potentialManaSpan
	own      []state.Mana
}

// potentialManaSpan is one zone position's membership span in the flat
// member array, and whether it was walked yet.
type potentialManaSpan struct {
	start, end int32
	walked     bool
}

// verifyPotentialSkip panics when an object PotentialMana skipped
// (manaWalkEmpty) could have contributed: some member of its membership
// walk is payable against an unbounded pool.
func (e *Engine) verifyPotentialSkip(p state.PlayerID, o *state.Object, id state.ObjID) {
	huge := state.Mana{1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28, 1 << 28}
	for _, ma := range e.appendAvailableManaAbilitiesGate(nil, nil, p, id, true) {
		if e.manaAbilityPayablePool(p, id, ma, &huge) {
			panic(fmt.Sprintf("rules: PotentialMana skipped obj %d (tapped %v) but one of its mana abilities is payable", id, o.Tapped))
		}
	}
}

// potentialMembersVerify makes PotentialMana recompute every reused
// membership list and panic on a difference. Set by the rules test binary
// (derivedmemo_verify_test.go), or at link time with derivedMemoVerifyFlag.
var potentialMembersVerify = derivedMemoVerifyFlag != ""

// potentialProducedStrip strips the braces and spaces from a Produced$
// value. A strings.Replacer is safe for concurrent use, so one serves every
// call instead of building a replacer per folded ability.
// "Combo" is stripped too: a "Combo W U" source (Tranquil Cove) makes one of
// the listed colours, so it bounds each of them like a dual's two abilities;
// pricing it unbounded in every colour offered a "pay {R}" no red source
// could fund (CR 118.3). "Combo Any" stays open through producedOpenSet, and
// any other Combo word (ColorIdentity, Chosen) keeps a non-colour rune.
var potentialProducedStrip = strings.NewReplacer("{", "", "}", "", " ", "", "Combo", "")

// addPotentialMana folds one mana ability into the potential accumulator. The
// amount and production parsing mirrors AvailableMana's addAvailable (the
// executor's own effMana resolutions: blank Produced$ is one colourless) with
// one divergence: anything the executor cannot statically price -- an
// alternative production or an indeterminate amount -- is UNBOUNDED here
// rather than zero, because this bound must never lose an action.
func addPotentialMana(m *state.Mana, ma *cards.SA) {
	mp := effects.ManaOf(ma)
	amt, indeterminate := potentialAmount(mp)
	raw := mp.Produced
	// Blank Produced$ is the executor's own colourless default (effMana), NOT
	// an alternative production: normalize it to "C" BEFORE the open test so a
	// colourless source contributes one colourless rather than pricing the
	// whole pool unbounded (the blank-`producedOpen("")` bug: 99 of every
	// colour turned an unpayable {W} spell into a potential cast).
	if raw == "" {
		raw = "C"
	}
	if indeterminate || producedOpen(raw) {
		for i := range m {
			m[i] = saturatingPotentialMana(m[i], potentialUnbounded)
		}
		return
	}
	s := potentialProducedStrip.Replace(raw)
	for _, r := range s {
		i := state.ManaIndex(byte(r))
		m[i] = saturatingPotentialMana(m[i], amt)
	}
}

// potentialManaAdd is addPotentialMana's effect as data: unbounded, or a
// per-slot total (each production rune's amount summed per slot; the
// amounts are non-negative, so one saturating add of the total is the
// rune-by-rune saturating adds).
type potentialManaAdd struct {
	unbounded bool
	add       [len(state.Mana{})]int64
}

func computePotentialManaAdd(mp *effects.ManaParams) potentialManaAdd {
	var f potentialManaAdd
	amt, indeterminate := potentialAmount(mp)
	raw := mp.Produced
	if raw == "" {
		raw = "C"
	}
	if indeterminate || producedOpen(raw) {
		f.unbounded = true
		return f
	}
	for _, r := range potentialProducedStrip.Replace(raw) {
		f.add[state.ManaIndex(byte(r))] += int64(amt)
	}
	return f
}

// addPotentialManaOf is addPotentialMana through ab's configured facts (the
// same production, read once per configured text; verify mode compares the
// two folds).
func (e *Engine) addPotentialManaOf(m *state.Mana, ma *cards.SA) {
	mf := e.manaFactsOf(ma)
	if mf == nil {
		addPotentialMana(m, ma)
		return
	}
	var want state.Mana
	if manaSAFactsVerify {
		want = *m
		addPotentialMana(&want, ma)
	}
	if mf.potential.unbounded {
		for i := range m {
			m[i] = saturatingPotentialMana(m[i], potentialUnbounded)
		}
	} else {
		for i, n := range mf.potential.add {
			if n > 0 {
				if sum := int64(m[i]) + n; sum >= math.MaxInt32 {
					m[i] = math.MaxInt32
				} else {
					m[i] = int32(sum)
				}
			}
		}
	}
	if manaSAFactsVerify && want != *m {
		panic(fmt.Sprintf("rules: configured potential production of %q folds %v, the text folds %v", ma.Line, *m, want))
	}
}

// saturatingPotentialMana adds a known non-negative production without
// letting the deliberately-unbounded potential sentinel wrap int32. This is
// projection-only; real mana still enters state through events.ManaAdd.
func saturatingPotentialMana(have, add int32) int32 {
	sum := int64(have) + int64(add)
	if sum >= math.MaxInt32 {
		return math.MaxInt32
	}
	return int32(sum)
}

// producedOpen reports whether a Produced$ value names an ALTERNATIVE or
// unknown production the executor resolves at activation time rather than a
// fixed mana set: "Any"/"Combo Any" (any colour), "Chosen", or any token
// outside the WUBRGC faces. A fixed multi-face value ("GW") is not open --
// each face is folded additively as an over-bound. The blank case is kept
// only defensively: addPotentialMana normalizes a blank Produced$ to "C"
// before calling this, so a colourless source is never mistaken for an open
// production (which would grant 99 of every colour to a
// blank-Produced$ source).
func producedOpen(raw string) bool {
	if producedOpenSet.Has(raw) {
		return true
	}
	s := potentialProducedStrip.Replace(raw)
	for _, r := range s {
		switch r {
		case 'W', 'U', 'B', 'R', 'G', 'C':
		default:
			return true
		}
	}
	return false
}

// potentialAmount is an ability's Amount$ as (literal, indeterminate): a
// blank Amount is the executor's own default of 1; a literal integer is used
// directly with the negative clamp the executor applies; anything else (an X,
// a Y, a Count$ expression, a Sacrificed$ reference) is indeterminate and
// prices unbounded upstream rather than at zero.
func potentialAmount(mp *effects.ManaParams) (int32, bool) {
	if mp.AmountTrim == "" {
		return 1, false
	}
	if v := mp.AmountLit; mp.AmountIsLit {
		if v < 0 {
			return 0, false
		}
		return int32(v), false
	}
	return 0, true
}

// PotentialActions is the view's per-seat projection of every play the seat
// could still make after floating every mana its untapped sources could
// produce: the engine's own legal-offer walk (legalActionsPriced, the exact
// code that builds a priority decision's options) priced against
// PotentialMana. It carries every real play the walk can offer
// (potentialPlayKind) and never the mana tap, pass or concede, which every
// priority window offers and which are never a play. The walk's own gates
// (timing, restrictions, targets, non-mana costs, live RaiseCost/ReduceCost,
// X at 0) are the engine's, so the projection cannot disagree with the engine
// the way a client-side re-derivation does.
//
// This is a pure read and never touches an event; callers project it ONLY for
// the viewer's own seat (engine/view/view.go gates it on p.ID == viewer), because
// the walk reads that seat's hand, command zone and graveyard -- projecting
// another seat's would leak their hidden zones (CR 400.2).
//
// Determinism: the walk is zone-order, never a map range, so the list is
// byte-stable run to run.
func (e *Engine) PotentialActions(p state.PlayerID) []decision.PotentialAction {
	if e.G.Over {
		return nil
	}
	_, opts := e.potentialWalkOf(p, true)
	// Payability is read from the decision builder's memo only. PotentialActions
	// also serves auto-pass and search readers; it must not run the planner or
	// open extra legal-action walks. Without a built decision, payability stays
	// unknown and the affordance remains visible.
	var unpayable map[state.ObjID]struct{}
	var unpayablePlays []decision.PotentialAction
	if e.pending != nil && e.pending.Kind == decision.KPriority && e.pending.Acting() == p {
		unpayablePlays = e.pending.PotentialUnpayablePlays
		unpayable = make(map[state.ObjID]struct{}, len(e.pending.PotentialUnpayableCasts))
		for _, id := range e.pending.PotentialUnpayableCasts {
			unpayable[id] = struct{}{}
		}
	}
	var out []decision.PotentialAction
	for _, o := range opts {
		if !potentialPlayKind(o.Kind) {
			continue
		}
		a := decision.PotentialAction{
			Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode, Label: o.Label,
		}
		if o.Kind == optCast && o.Mode == "" && o.AltCostIndex == 0 {
			if _, proven := unpayable[o.Obj]; proven {
				no := false
				a.Payable = &no
			}
		}
		if slices.ContainsFunc(unpayablePlays, func(u decision.PotentialAction) bool {
			return u.Kind == a.Kind && u.Obj == a.Obj && u.Ability == a.Ability && u.Mode == a.Mode
		}) {
			no := false
			a.Payable = &no
		}
		out = append(out, a)
	}
	// opts belongs to the decision's potential walk cache (or is the
	// decision's own Options): never released here.
	return out
}

// potentialPlayKind is the projection's kind filter: every real play kind
// legalActionsPriced emits (measured from engine/rules/legal.go and pinned by
// TestPotentialActionsProjectsEveryPlayKind, so a new kind in the walk fails
// until it is classified here or as never-a-play):
//
//   - "cast": hand, command zone, graveyard (flashback, escape, ...) and exile
//     casts, plus the special actions that ride the cast kind (foretell,
//     suspend), each with its Mode;
//   - "ability": printed and keyword-granted activated abilities (Equip among
//     them), and "granted": a max-speed static's AddAbility$ (engine/rules/speed.go);
//   - "unlock" (a Room's locked door, CR 309.5), "turn_face_up" (the morph
//     family, CR 708.6) and "specialize" (Mode = the face index): the
//     mana-costed special actions, projected on an empty pool once the
//     hypothetical pool pays them, so the web's auto-pay mode keeps the
//     manual taps they need;
//   - "play_land" and "station": never mana-costed, so never float-gated,
//     projected for completeness.
//
// The excluded kinds are "activate" (the mana tap), "pass" and "concede".
func potentialPlayKind(kind string) bool {
	return potentialPlayKindSet.Has(kind)
}

var producedOpenSet = state.NewNameSet(
	"",
	"Any",
	"Combo Any",
	"Chosen",
)

var potentialPlayKindSet = state.NewNameSet(
	"cast",
	"ability",
	"play_land",
	"granted",
	"unlock",
	"turn_face_up",
	"specialize",
	"station",
)
