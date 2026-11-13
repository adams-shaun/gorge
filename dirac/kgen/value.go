package kgen

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// value lowers an amount parameter: a literal ("3", "+2", "-1"), an SVar
// name ("X", "-X", "+X") or, for the few params that allow it, a keyword
// the caller handles first ("All", "Any", "Double").
func (lc *lowerCtx) value(raw string) (lValue, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return lValue{}, fmt.Errorf("empty amount")
	}
	neg := false
	switch raw[0] {
	case '+':
		raw = raw[1:]
	case '-':
		neg = true
		raw = raw[1:]
	}
	if n, err := strconv.Atoi(raw); err == nil {
		if neg {
			n = -n
		}
		return cst(n), nil
	}
	if strings.Contains(raw, "$") && !neg {
		// An inline expression (ImmediateTrigger's TriggerAmount$).
		return lc.svarValue(raw, 0)
	}
	if raw == "Chosen" && lc.chosenNumOK && !neg {
		// gorge specCtx's Resolve: a "Chosen" bound is the source's
		// ChosenNumber, ahead of any SVar (lower_choosenumber.go)
		return lValue{Kind: vChosenNumber}, nil
	}
	body, ok := lc.face.SVars[raw]
	if !ok && raw == lc.rollName && lc.rollAb == lc.ab && lc.ab != nil {
		// gorge Num: a RollDice ResultSVar$ publication, read after the
		// card's own SVars (count.go runtimePublished)
		return lValue{Kind: vRollResult, Neg: neg}, nil
	}
	if !ok && !neg && raw == lc.excess {
		// gorge Num: the resolution-scoped Excess SVar a DealDamage node
		// in this chain published (ExcessSVar$, count.go publishSVar).
		// The kernel's ExExcessDamage is the same scratch, so it is exact
		// only within that chain -- hence lc.excess, never the bare name.
		return lValue{Kind: vExcessDamage}, nil
	}
	if !ok && !neg && raw == "X" && lc.ab != nil && lc.xPaidAb == lc.ab {
		// Forge's built-in X: a card whose own cost has {X} may write a bare
		// "X" amount with no SVar (Consume Spirit's NumDmg$ X and LifeAmount$
		// X), and gorge resolves it as the paid X (count.go raw "X").
		return lValue{Kind: vX}, nil
	}
	if !ok {
		return lValue{}, fmt.Errorf("amount %q names no SVar", raw)
	}
	v, err := lc.svarValue(body, 0)
	if err != nil {
		return lValue{}, fmt.Errorf("SVar %s=%q: %w", raw, body, err)
	}
	if neg {
		v.Neg = !v.Neg
	}
	return v, nil
}

// targetedCastSpentValue lowers Targeted$CastTotalManaSpent: the resolving
// spell's target spell's cast spend, snapshotted as the resolution starts,
// before the counter that removes its stack entry (gorge
// targetManaSpentLKI). The kernel holds one snapshot of target slot 0, so the
// spell must have exactly one target slot, a spell on the stack, declared at
// cast (EOE Unravel).
func (lc *lowerCtx) targetedCastSpentValue() (lValue, error) {
	if lc.ab == nil || lc.ab.Kind != akSpell || lc.ab.Latent {
		return lValue{}, refuse("Targeted$CastTotalManaSpent outside a resolving spell")
	}
	if len(lc.chainT) != 1 {
		return lValue{}, refuse("Targeted$CastTotalManaSpent over %d target slots", len(lc.chainT))
	}
	if lc.chainT[0] != 0 {
		return lValue{}, refuse("Targeted$CastTotalManaSpent of target slot %d, not the snapshot's slot 0", lc.chainT[0])
	}
	tg := lc.ab.Targets[lc.chainT[0]]
	if tg.AtRes {
		return lValue{}, refuse("Targeted$CastTotalManaSpent of a target chosen at resolution")
	}
	if !tg.Spell || tg.Zone != zStack || tg.StackAb || !isConstN(tg.Min, 1) || !isConstN(tg.Max, 1) {
		return lValue{}, refuse("Targeted$CastTotalManaSpent of a target slot that is not exactly one spell on the stack")
	}
	return lValue{Kind: vTgtSpent}, nil
}

// triggeredAttackerValue lowers TriggeredAttacker$...: the ref is the single
// attacker the trigger captured (gorge count_ref.go refTargetsTriggeredAttacker).
// CardPower reads its live power (Arahbo, Roar of the World's "X is its
// power"); CardCounters.ALL sums its LIVE counters (count_ref_property.go's
// sumCounters, no LKI substitution for a trigger object), so X sees the
// counter the sibling PutCounter just added. defined() refuses the shapes with
// no single attacker (an AttackersDeclared player event). CardToughness has no
// carrier in the class and is left to the ordinary refusal. Spider-Man Noir.
func (lc *lowerCtx) triggeredAttackerValue(rest string) (lValue, error) {
	ref, err := lc.defined("TriggeredAttacker")
	if err != nil {
		return lValue{}, err
	}
	switch rest {
	case "CardPower":
		return lValue{Kind: vPowerOf, Ref: ref}, nil
	case "CardCounters.ALL":
		return lValue{Kind: vCountersOn, Ref: ref, S: "ALL", N: 1}, nil // N 1: the live read (kernel refCountersAll)
	}
	return lValue{}, fmt.Errorf("TriggeredAttacker$%s", rest)
}

// svarValue lowers an SVar value body.
func (lc *lowerCtx) svarValue(body string, depth int) (lValue, error) {
	if depth > 4 {
		return lValue{}, fmt.Errorf("SVar nesting too deep")
	}
	body = strings.TrimSpace(body)
	if n, err := strconv.Atoi(body); err == nil {
		return cst(n), nil
	}
	head, rest, ok := strings.Cut(body, "$")
	if !ok {
		return lValue{}, fmt.Errorf("not a value expression")
	}
	// Trailing arithmetic: "/Minus.1", "/Plus.10", "/Twice", "/DivideEvenlyDown.2".
	// Remembered$ splits only for the two arithmetic properties (Amount,
	// CardManaCost) and only one op: gorge cuts at the first "/" and reads
	// the whole suffix as one op, so a chain refuses. A Valid spec may itself
	// contain "/" and is never split.
	var mods []string
	if i := strings.Index(rest, "/"); i >= 0 {
		if head == "Remembered" {
			body, mod, split, err := rememberedModSplit(rest)
			if err != nil {
				return lValue{}, err
			}
			if split {
				rest, mods = body, []string{mod}
			}
		} else {
			mods = strings.Split(rest[i+1:], "/")
			rest = rest[:i]
		}
	}
	if lc.numTimesK > 0 && body == "Count$TriggerRememberAmount" {
		return cst(lc.numTimesK), nil // the announced count (lower_numtimes.go)
	}
	if lc.trigXPaid && body == "Count$TriggerRememberAmount" && lc.trigRemX {
		// RememberSVarAmount$ X under a CollectEvidence<X> cost: the
		// parent's announced X (lower_triggerxcost.go)
		if v, ok, err := lc.reflexXPaid(); ok || err != nil {
			return v, err
		}
	}
	var v lValue
	var err error
	// <group>$HasPropertyHasCardsIn... (value_hascards.go) takes the
	// switch's first arm, then the trailing mods (/Times.2) apply as usual
	hv, hok, herr := lc.hasCardsValue(head, rest)
	if hok {
		head = "\x00hascards"
	}
	if head == "ThisTargetedPlayer" && !strings.HasPrefix(rest, "Valid ") {
		// gorge evalPlayerRefProperty binds both heads to one ref; only
		// the Valid form has its own reading below
		head = "TargetedPlayer"
	}
	switch head {
	case "\x00hascards":
		v, err = hv, herr
	case "Number":
		n, e := strconv.Atoi(rest)
		if e != nil {
			return lValue{}, fmt.Errorf("Number$%s", rest)
		}
		v = cst(n)
	case "Count":
		v, err = lc.countValue(rest, depth)
	case "Convoked":
		// Convoked$Amount: the creatures that convoked the source's spell
		// (Knight-Errant of Eos), read by its own enters trigger: the
		// kernel binds the count into the trigger amount as it triggers
		// (Tf2Convoked)
		if rest != "Amount" {
			err = fmt.Errorf("Convoked$%s", rest)
			break
		}
		tr := (*lTrigger)(nil)
		if lc.ab != nil {
			tr = lc.ab.Trigger
		}
		if tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "ETB" || tr.Batch || tr.F == nil || tr.F.Players != 0 || len(tr.F.Clauses) != 1 || tr.F.Clauses[0] != (lClause{Flags: fSelf}) {
			err = fmt.Errorf("Convoked$Amount outside its own enters trigger")
			break
		}
		tr.Convoked = true
		v = lValue{Kind: vTriggerAmount}
	case "CastSA>Count":
		v, err = lc.adamantValue(rest) // lower_evoke.go
	case "TriggerRemembered":
		v, err = lc.triggerRemembered(rest) // lower_trigremembered.go
	case "TriggerCount":
		switch rest {
		case "DamageAmount", "LifeAmount", "Amount":
			v = lValue{Kind: vTriggerAmount}
		default:
			err = fmt.Errorf("TriggerCount$%s", rest)
		}
	case "TriggerObjectsAttackers":
		// "that many" in a "whenever one or more <creatures> you control
		// attack" trigger (Poetic Ingenuity, The Earth King): gorge's
		// refTargetsTriggerObjectsAttackers reads the TriggerAttackers
		// capture, the attackers that passed THIS line's own
		// ValidAttackers$ filter. The kernel binds that count as the
		// trigger amount (fireAttacksGen: attackersMatching over the
		// whole declaration, or nAttackers with no filter). Only the
		// batch shape captures once per declaration; a per-defender line
		// (Tf2DefPlayer) captures once per defender event, so the
		// first-event batch amount would be wrong.
		tr := (*lTrigger)(nil)
		if lc.ab != nil {
			tr = lc.ab.Trigger
		}
		if rest != "Amount" || tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "AttackersDeclared" || tr.DefPlayer != 0 {
			err = fmt.Errorf("TriggerObjectsAttackers$%s outside a batch AttackersDeclared trigger", rest)
			break
		}
		v = lValue{Kind: vTriggerAmount}
	case "TriggeredCapturedPlayers":
		// cards.MeleePumpCount: gorge captures one player per distinct
		// opponent attacked in the declaration (rules/melee.go
		// meleeRemembered). In a two-player game every attack is at the
		// one opponent (gorge counts a planeswalker's or battle's attack
		// as its controller's or protector's, combat.go finishAttackers),
		// so the capture is one.
		if rest != "Amount" || !lc.mlTrigger || !lc.trigBody {
			err = fmt.Errorf("value head TriggeredCapturedPlayers$%s outside a Melee trigger", rest)
			break
		}
		lc.twoSeat("the melee capture counted as one")
		v = cst(1)
	case "PlayerCountOpponents":
		// the living opponents in seat order (gorge opponentGroup): seat-exact
		switch {
		case rest == "Amount":
			v = lValue{Kind: vOppExtreme, N: kop.OppExtAmount}
		case rest == "HighestLifeTotal":
			v = lValue{Kind: vOppExtreme, N: kop.OppExtLife}
		case rest == "LowestLifeTotal":
			v = lValue{Kind: vOppExtreme, N: kop.OppExtLife | kop.OppExtLowest}
		case rest == "HighestCardsInHand":
			v = lValue{Kind: vOppExtreme, N: kop.OppExtHand}
		case rest == "LowestCardsInHand":
			v = lValue{Kind: vOppExtreme, N: kop.OppExtHand | kop.OppExtLowest}
		case rest == "HighestCardsInGraveyard":
			v = lValue{Kind: vOppExtreme, N: kop.OppExtGrave}
		case strings.HasPrefix(rest, "HighestValid "):
			// the filter is read from each opponent's side: "YouCtrl"
			// there is that opponent's (gorge SpecContext(p))
			f, ferr := lc.filter(strings.TrimPrefix(rest, "HighestValid "))
			if ferr != nil {
				return lValue{}, ferr
			}
			// only the controller words are rebound to the opponent: a
			// player clause or a source-relative word (Other, a compare
			// against X) would read the controller's side, so it refuses
			// as opponentView does
			if _, err = opponentView(f); err == nil {
				v = lValue{Kind: vOppExtreme, N: kop.OppExtValid, F: f}
			}
		case strings.HasPrefix(rest, "HighestValid"):
			// the zone-scoped extreme over the opponents (value_plrextreme.go):
			// Nautiloid Ship's X, each opponent read from their own side
			v, err = lc.plExtremeValue(head, rest)
		case rest == "HighestCounters.Poison":
			// gorge playerCountExtreme's Counters.Poison arm: the extreme
			// over the living opponents, in group order. Every other
			// Counters.<kind> stays refused.
			v = lValue{Kind: vHighestOppPoison}
		case strings.HasPrefix(rest, "Condition"):
			v, err = lc.oppCastCondValue(rest) // value_oppcondition.go
		default:
			err = fmt.Errorf("PlayerCountOpponents$%s", rest)
		}
	case "TriggeredSource":
		v, err = lc.triggeredSourceValid(rest) // lower_ninjutsu.go
	case "PlayerCountRegisteredOpponents":
		if rest == "HasPropertyLostLifeThisTurn" {
			v = lValue{Kind: vOppsLostLife}
		} else if hv, ok := oppsHitByLegendaryValue(rest); ok {
			v = hv // value_turntallies.go
		} else if rest == oppsDealtCombatDamageBody {
			v = lValue{Kind: vOppsLostLife, N: 2}
		} else {
			err = fmt.Errorf("PlayerCountRegisteredOpponents$%s", rest)
		}
	case "TriggeredSpellAbility":
		tr := lc.ab.Trigger
		if tr == nil || tr.Event != "SpellCast" || !lc.trigBody || lc.ab.Latent {
			return lValue{}, fmt.Errorf("%s$ outside a SpellCast trigger body", head)
		}
		switch rest {
		case "CardManaCostLKI":
			v = lValue{Kind: vTriggerSpellMVLKI}
		default:
			err = fmt.Errorf("%s$%s", head, rest)
		}
	case "SpellTargeted":
		// Sound the Trumpets: the spell the resolving spell targeted, read
		// where it now is (gorge refTargetsSpellTargeted, then the shared
		// CardManaCostLKI property arm). The kernel's ExMVOf/targetMV is the
		// same read: the live object's printed mana value, plus its
		// announced X only while it is still a spell on the stack (CR
		// 202.3e). Object identity survives the Counter's zone move.
		if rest != "CardManaCostLKI" {
			return lValue{}, refuse("%s$%s", head, rest)
		}
		if lc.ab == nil || lc.ab.Kind != akSpell || lc.ab.Latent {
			return lValue{}, refuse("%s$%s outside a resolving spell", head, rest)
		}
		if len(lc.chainT) != 1 {
			return lValue{}, refuse("%s$%s over %d target slots", head, rest, len(lc.chainT))
		}
		t := lc.lastTarget()
		if t < 0 || t >= len(lc.ab.Targets) {
			return lValue{}, refuse("%s$%s with no target slot", head, rest)
		}
		tg := lc.ab.Targets[t]
		if !tg.Spell || tg.Zone != zStack || !isConstN(tg.Min, 1) || !isConstN(tg.Max, 1) {
			return lValue{}, refuse("%s$%s of a target slot that is not exactly one spell on the stack", head, rest)
		}
		v = lValue{Kind: vMVOf, Ref: lRef{Kind: rTarget, N: t}}
	case "ParentTargeted", "Targeted", "TriggeredCard", "Sacrificed":
		if head == "Targeted" {
			lc.unionProbe("value:Targeted$" + rest)
		}
		ref := lRef{Kind: rTarget, N: lc.lastTarget()}
		switch head {
		case "ParentTargeted":
			ref = lRef{Kind: rTarget, N: lc.parentFirstTarget()}
		case "TriggeredCard":
			ref = lRef{Kind: rTriggered}
		case "Sacrificed":
			ref = lRef{Kind: rSacrificed}
		}
		if ref.Kind == rTarget && ref.N < 0 {
			return lValue{}, fmt.Errorf("%s$ with no target slot", head)
		}
		if spec, ok := strings.CutPrefix(rest, "Valid "); ok && (head == "Targeted" || head == "Sacrificed" || head == "TriggeredCard") {
			v, err = lc.refValidCount(head, spec, ref) // value_compareheads.go
			break
		}
		switch rest {
		case "CardManaCost":
			// TriggeredCard, and Sacrificed: the sacrificed cost
			// object's last known information (gorge c.Sacrificed LKI)
			if ref.Kind == rSacrificed {
				if at := lc.ifPaidSacrificeIndex(); at >= 0 {
					// a printed trigger's own IfPaid Sacrifice cost (Venom,
					// Lethal Protector): the sacrifice sum rides the same
					// resolution's accumulator (kop.ExRemLKIPT N 3), so the
					// kernel reads the printed mana value before the
					// permanent moves (gorge sacrificedInfoFor.ManaValue,
					// CR 608.2g/h)
					lc.ab.Prog[at].Note = noteRememberSacrificed + ":mv"
					lc.remLKN++
					v = lValue{Kind: vRemLKIPT, N: 3}
					break
				}
			}
			if ref.Kind != rTriggered && !(ref.Kind == rSacrificed && lc.sacrificedIsCost()) && !lc.targetMVReadable(ref) {
				err = fmt.Errorf("%s$%s", head, rest)
				break
			}
			v = lValue{Kind: vMVOf, Ref: ref}
		case "CardPower":
			v = lValue{Kind: vPowerOf, Ref: ref}
		case "CardToughness":
			v = lValue{Kind: vToughnessOf, Ref: ref}
		case "CardSumPT":
			// its power plus its toughness (Phthisis, Predator's Rapport;
			// gorge count_ref_property.go's CardSumPT): the two reads, summed
			v = lValue{Kind: vSum, Sub: []lValue{{Kind: vPowerOf, Ref: ref}, {Kind: vToughnessOf, Ref: ref}}}
		case "Converge":
			// the spell's colours spent, snapshotted as the
			// trigger fires (gorge TriggerConverge)
			tr := lc.ab.Trigger
			if head != "TriggeredCard" || tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "SpellCast" {
				err = fmt.Errorf("TriggeredCard$Converge outside a SpellCast trigger")
				break
			}
			tr.CastConverge = true
			v = lValue{Kind: vTriggerAmount}
		case "CastTotalManaSpent":
			if head == "Targeted" {
				v, err = lc.targetedCastSpentValue()
				break
			}
			// opus: the spell's pool mana spent, snapshotted as the
			// trigger fires (gorge TriggerManaSpent)
			tr := lc.ab.Trigger
			if head != "TriggeredCard" || tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "SpellCast" {
				err = fmt.Errorf("TriggeredCard$CastTotalManaSpent outside a SpellCast trigger")
				break
			}
			tr.CastSpent = true
			v = lValue{Kind: vTriggerAmount}
		case "CardNumColors":
			// the triggering spell's colours on the
			// stack (gorge count_ref_property.go CardNumColors,
			// len(h.ObjectColors(o)))
			tr := lc.ab.Trigger
			if head != "TriggeredCard" || tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "SpellCast" {
				err = refuse("TriggeredCard$CardNumColors outside a SpellCast trigger")
				break
			}
			v = lValue{Kind: vNumColorsOf, Ref: ref}
		case "ChromaSource.White", "ChromaSource.Blue", "ChromaSource.Black", "ChromaSource.Red", "ChromaSource.Green":
			// the colour's mana symbols in the triggering spell's printed
			// cost (gorge chromaObjectCount, Forge getTotalChroma)
			tr := lc.ab.Trigger
			if head != "TriggeredCard" || tr == nil || !lc.trigBody || lc.ab.Latent || tr.Event != "SpellCast" {
				err = refuse("TriggeredCard$ChromaSource outside a SpellCast trigger")
				break
			}
			v = lValue{Kind: vChromaOf, Ref: ref, N: map[string]int{"White": 0, "Blue": 1, "Black": 2, "Red": 3, "Green": 4}[strings.TrimPrefix(rest, "ChromaSource.")]}
		default:
			if head == "ParentTargeted" && strings.HasPrefix(rest, "CardCounters.") && !strings.EqualFold(rest, "CardCounters.ALL") {
				// the parent's first target's live counters
				// (gorge count_ref_property.go:265; the LKI substitution at :275
				// is the Targeted ref's only).
				v = lValue{Kind: vCountersOn, Ref: ref, S: strings.TrimPrefix(rest, "CardCounters.")}
				break
			}
			if head == "TriggeredCard" && strings.HasPrefix(rest, "CardCounters.") {
				// the counters the triggering object
				// had on the battlefield, read from the trigger's LKI
				// snapshot (gorge evalRefProperty binds TriggeredCard to
				// c.LKI; CR 603.10a, 608.2h). Only a leaves-the-battlefield
				// trigger's object carries a snapshot the kernel reads.
				tr := lc.ab.Trigger
				if tr == nil || !lc.trigBody || lc.ab.Latent || tr.Batch || tr.Origin != zBattlefield || tr.Event != "Dies" && tr.Event != "Leaves" {
					err = refuse("TriggeredCard$CardCounters outside a leaves-the-battlefield trigger")
					break
				}
				v = lValue{Kind: vCountersOn, Ref: ref, S: strings.TrimPrefix(rest, "CardCounters.")}
				break
			}
			if strings.HasPrefix(rest, "CastTotalManaSpent ") {
				err = fmt.Errorf("TriggeredCard$CastTotalManaSpent with a producer filter %q", strings.TrimPrefix(rest, "CastTotalManaSpent "))
				break
			}
			err = fmt.Errorf("%s$%s", head, rest)
		}
	case "ReplacedCard":
		v, err = lc.replacedCardValue(rest) // value_compareheads.go
	case "ThisTargetedPlayer":
		// gorge count_ref_property.go:727: the TargetedPlayer ref; Valid <spec>
		// counts the targeted player's battlefield with that player as You.
		// Two-player that is the opponent's only for an opponents-only slot.
		spec, ok := strings.CutPrefix(rest, "Valid ")
		t := lc.lastTarget()
		if !ok || t < 0 {
			return lValue{}, refuse("ThisTargetedPlayer$%s", rest)
		}
		if f := lc.ab.Targets[t].F; f == nil || f.Players != plOpp || len(f.Clauses) != 0 {
			return lValue{}, refuse("ThisTargetedPlayer$%s of a slot that is not opponents-only", rest)
		}
		f, ferr := lc.filter(spec)
		if ferr != nil {
			return lValue{}, ferr
		}
		// the entry's one player slot: the targeted player's own battlefield
		// at any seat count; else the one opponent, a two-seat read
		if tv := lc.targetedPlayerView(f); tv != nil {
			v = lValue{Kind: vCount, F: tv}
			break
		}
		lc.twoSeat("the targeted player's battlefield read as the opponent's")
		if f, err = opponentView(f); err == nil {
			v = lValue{Kind: vCount, F: f}
		}
	case "Equipped":
		// gorge count_ref.go:622: the object the source is attached to
		// (unattached: empty, 0); CardPower is its live power.
		if rest != "CardPower" {
			return lValue{}, refuse("Equipped$%s", rest)
		}
		v = lValue{Kind: vPowerOf, Ref: lRef{Kind: rEquipped}}
	case "TargetedPlayer":
		t := lc.lastTarget()
		if t < 0 {
			return lValue{}, fmt.Errorf("TargetedPlayer$ with no target slot")
		}
		ref := lRef{Kind: rTarget, N: t}
		switch rest {
		case "LifeTotal":
			v = lValue{Kind: vLifeOf, Ref: ref}
		case "CardsInLibrary":
			v = lValue{Kind: vLibrarySizeOf, Ref: ref}
		case "CardsInHand":
			v = lValue{Kind: vHandSizeOf, Ref: ref}
		case "Amount":
			// gorge counts the player targets of the resolution (the
			// Hollow Marauder family): over an opponents-only slot two-player
			// that is whether the slot holds the one opponent (seat-exact:
			// the slot holds one player at any seat count). A slot that may
			// hold the controller counts up to two and refuses.
			if f := lc.ab.Targets[t].F; f == nil || f.Players != plOpp || len(f.Clauses) != 0 {
				err = refuse("TargetedPlayer$%s of a slot that is not opponents-only", rest)
				break
			}
			v = lValue{Kind: vTargetPlayers, Ref: ref}
		case "LifeLostThisTurn":
			// Thought-Stalker Warlock: lowered only for an opponents-only
			// player slot, where two-player it is the controller's
			// opponent's figure.
			if f := lc.ab.Targets[t].F; f == nil || f.Players != plOpp || len(f.Clauses) != 0 {
				err = fmt.Errorf("TargetedPlayer$%s of a slot that is not opponents-only", rest)
				break
			}
			lc.twoSeat("the targeted player's life lost read as the opponent's")
			v = lValue{Kind: vOppLifeLostThisTurn}
		default:
			err = fmt.Errorf("TargetedPlayer$%s", rest)
		}
	case "TriggeredAttacker":
		v, err = lc.triggeredAttackerValue(rest)
	case "Spawner>Sacrificed":
		// A reflexive trigger reading the creature its parent's own IfPaid
		// Sacrifice cost took (Bolg of the North's "that creature's power";
		// gorge count_ref.go Sacrificed$ over the trigger_cost capture). The
		// parent's sacrifice sum is captured into the reflexive trigger's
		// amount as it is put on the stack: the sacrificial resolution is
		// over by the time the reflexive body resolves..
		ab := lc.ab
		if ab == nil || !ab.Latent || ab.Name != "ReflexiveTrigger" {
			err = fmt.Errorf("Spawner>Sacrificed$%s outside a reflexive trigger", rest)
			break
		}
		if rest != "CardPower" {
			err = fmt.Errorf("Spawner>Sacrificed$%s", rest)
			break
		}
		if ab.TrigRem != "" && ab.TrigRem != trigRemSacPower {
			err = fmt.Errorf("Spawner>Sacrificed$%s beside another captured amount", rest)
			break
		}
		ab.TrigRem = trigRemSacPower
		v = lValue{Kind: vTriggerAmount}
	case "Spawner>TriggeredCard":
		// A delayed trigger reading the card that spawned it: Nine-Lives
		// Familiar's revival counters on the dead card (LKI).
		if strings.HasPrefix(rest, "CardCounters.") {
			v = lValue{Kind: vCountersOn, Ref: lRef{Kind: rDelayedRemembered}, S: strings.TrimPrefix(rest, "CardCounters.")}
		} else {
			err = fmt.Errorf("Spawner>TriggeredCard$%s", rest)
		}
	case "TriggerObjectsCurrentCastSpells":
		// gorge evalTriggerCurrentCastSpellsOK: the controller's instant and
		// sorcery casts this turn up to the triggering one, counted when the
		// SpellCast trigger fires (the trigger amount)
		tr := lc.ab.Trigger
		if tr == nil || tr.Event != "SpellCast" {
			return lValue{}, fmt.Errorf("%s$ outside a SpellCast trigger", head)
		}
		switch rest {
		case "Valid Sorcery.YouCtrl,Instant.YouCtrl", "Valid Instant.YouCtrl,Sorcery.YouCtrl":
		default:
			return lValue{}, fmt.Errorf("%s$%s", head, rest)
		}
		tr.CastSnap = true
		v = lValue{Kind: vTriggerAmount}
	case "PlayerCountPropertyYou":
		switch rest {
		case "LifeLostThisTurn":
			v = lValue{Kind: vLifeLostThisTurn}
		case "LifeLostLastTurn":
			// gorge Engine.LifeLostLastTurn: the controller's losses in the
			// previous turn's window (Paladin of Atonement's upkeep CheckSVar).
			v = lValue{Kind: vLifeLostLastTurn}
		case "HasPropertyActive":
			v = lValue{Kind: vYourTurns, N: 1} // 1 on the controller's turn
		case "CardsDiscardedThisTurn":
			v = lValue{Kind: vDiscardedThisTurn}
		case "OpponentsAttackedThisTurn":
			// gorge Engine.OpponentsAttackedThisTurn: the distinct players the
			// controller's declared attacks named (a planeswalker or battle
			// defender is not its controller); the active player only.
			v = lValue{Kind: vOppsAttacked}
		case "HasPropertyMaxSpeed":
			v = lValue{Kind: vYourSpeed, N: 1} // 1 at max speed (gorge playerMaxSpeedProperty), else 0
		default:
			v, err = sacrificedThisTurnValue(rest) // value_sacrificed.go; refuses any other property
		}
	case "PlayerCountRemembered":
		v, err = lc.playerCountRemembered(rest, depth) // lower_repeatplayers.go
	case "PlayerCountPlayers", "TargetedController", "TriggeredTarget":
		v, err = lc.kvfHead(head, rest) // value_filterheads.go
	case "SVar":
		// gorge count.go: SVar$<name>[/Op] reads another SVar on the face
		body, ok := lc.face.SVars[rest]
		if !ok {
			return lValue{}, fmt.Errorf("value head SVar$%s names no SVar", rest)
		}
		if v, err = lc.svarValue(body, depth+1); err != nil {
			return lValue{}, fmt.Errorf("SVar$%s=%q: %w", rest, body, err)
		}
	case "Discarded":
		// gorge refTargetsDiscarded (cr-discarded-cost-value-ref): the card
		// the spell's own Discard<1/...> cost discarded, read where it is
		// now (kernel countRefMatches over RefDiscarded: stackEntry.trigObj)
		spec, ok := strings.CutPrefix(rest, "Valid ")
		if !ok || !lc.discardedIsCost() {
			return lValue{}, fmt.Errorf("value head Discarded$%s", rest)
		}
		f, ferr := lc.filter(spec)
		if ferr != nil {
			return lValue{}, ferr
		}
		v = lValue{Kind: vRefCount, Ref: lRef{Kind: rDiscarded}, F: f}
	case "ExiledWith":
		// gorge count_ref.go ExiledWith$Amount: exiledWithSet's size, the
		// cards in every seat's exile zone exiled with the reading source
		// (kernel exwCount). CardManaCost sums the printed mana values over
		// the same set (gorge count_ref_property.go:214-227; kernel
		// ExExiledWithN N 1). A granted ability's source is another card.
		if lc.granted > 0 {
			err = fmt.Errorf("ExiledWith$%s", rest)
			break
		}
		switch rest {
		case "Amount":
			lc.out.ExiledWith = true
			v = lValue{Kind: vExiledWithN}
		case "CardManaCost":
			lc.out.ExiledWith = true
			v = lValue{Kind: vExiledWithN, N: 1}
		case "SharesCardTypes":
			// the most cards of the set with one card type in common,
			// at least 1 (gorge count_ref_property.go; kernel
			// ExExiledWithN N 2)
			lc.out.ExiledWith = true
			v = lValue{Kind: vExiledWithN, N: 2}
		default:
			err = fmt.Errorf("ExiledWith$%s", rest)
		}
	case "RememberedLKI":
		// gorge count_ref_property.go: the sacrifice capture of each
		// remembered object, summed; lowerTail's refuseRemLKIPT admits the
		// read only over one Sacrifice RememberSacrificed$ set
		switch rest {
		case "CardPower":
			v = lValue{Kind: vRemLKIPT, N: 0}
			lc.remLKN++
		case "CardToughness":
			v = lValue{Kind: vRemLKIPT, N: 1}
			lc.remLKN++
		case "CardManaCost":
			v, err = lc.remLKIManaValue() // value_remlkimv.go
		default:
			err = fmt.Errorf("RememberedLKI$%s", rest)
		}
	case "Remembered":
		if rest == "Amount/DivideEvenlyDown.2" {
			v = lValue{Kind: vTokensCreated, Div: 2}
			return v, nil
		}
		if rest == "CardManaCost" {
			// gorge evalRefProperty: the remembered objects' mana values
			// summed (kernel rememberedMV)
			v = lValue{Kind: vMVOf, Ref: lRef{Kind: rRemembered}}
			break
		}
		if rest == "CardPower" {
			// gorge count_ref_property.go:165-177 sums the power over the
			// remembered set; the kernel's ExPowerOf reads one object, so
			// refuseRememberedPower admits only a provably one-card set
			// written by a one-target ChangeZone RememberChanged$ (Golbez,
			// Crystal Collector).
			v = lValue{Kind: vPowerOf, Ref: lRef{Kind: rRemembered}}
			break
		}
		if rest == "Amount" {
			// gorge evalRememberedOK: the resolution's remembered set's
			// size (the fire-time capture excluded; the kernel seeds none).
			// S marks the ctx-only read for refuseCtxRememberedCount.
			v = lValue{Kind: vRememberedCount, S: "ctx"}
			if lc.effCount {
				// an Effect trigger's body (The Eagles Are Coming!): the set's
				// size the Effect froze, which the emblem carries as the
				// trigger amount (lower_effectremember.go)
				v = lValue{Kind: vTriggerAmount}
			}
			break
		}
		if spec, ok := strings.CutPrefix(rest, "Valid "); ok {
			// gorge evalRefProperty Valid: the remembered objects matching
			// the spec where they now are (kernel countRefMatches)
			f, ferr := lc.filter(spec)
			if ferr != nil {
				return lValue{}, ferr
			}
			v = lValue{Kind: vRefCount, Ref: lRef{Kind: rRemembered}, F: f}
			break
		}
		if rest == "CardTypes" {
			// gorge count_ref_property.go:145: the distinct card types over
			// the remembered objects (kernel rememberedTypes; N 1)
			v = lValue{Kind: vGraveTypes, N: 1}
			break
		}
		err = fmt.Errorf("Remembered$%s", rest)
	case "Imprinted":
		if rest == "Amount" {
			// the size of the set the writer filled: DigUntil ImprintFound$
			// accumulates the found cards in the chosen set (Demonlord
			// Belzenlok), a Dig Imprint$ in the imprinted one
			ref := rImprinted
			if lc.digImprintChosen {
				ref = rChosen
			}
			v = lValue{Kind: vRefCount, Ref: lRef{Kind: ref}}
			break
		}
		if spec, ok := strings.CutPrefix(rest, "Valid "); ok {
			// gorge evalRefProperty Valid over the imprinted set where the
			// objects now are (kernel countRefMatches); Tainted Pact's
			// CheckName (Imprinted$Valid Card.sharesNameWith Remembered)
			f, ferr := lc.filter(spec)
			if ferr != nil {
				return lValue{}, ferr
			}
			v = lValue{Kind: vRefCount, Ref: lRef{Kind: rImprinted}, F: f}
			break
		}
		err = fmt.Errorf("Imprinted$%s", rest)
	default:
		err = fmt.Errorf("value head %s$", head)
	}
	if err != nil {
		return lValue{}, err
	}
	for _, m := range mods {
		if v, err = lc.applyModifier(v, m, depth); err != nil {
			return lValue{}, err
		}
	}
	return v, nil
}

// opponentView rewrites a filter read from the opponent's side (a
// PlayerCount... per-opponent count) into the controller's frame: "you"
// and "opponent" swap. A source-relative word (Self, Other, ...) has no
// meaning from the opponent's side and refuses.
func opponentView(f *lFilter) (*lFilter, error) {
	if f.Players != 0 {
		return nil, fmt.Errorf("opponent-side filter with players")
	}
	swap := func(w uint8) uint8 {
		switch w {
		case whoYou:
			return whoOpp
		case whoOpp:
			return whoYou
		}
		return w
	}
	out := &lFilter{}
	for _, c := range f.Clauses {
		if c.Flags != 0 || c.CmpX != "" {
			return nil, fmt.Errorf("opponent-side filter with source-relative words")
		}
		c.Ctrl, c.Owner = swap(c.Ctrl), swap(c.Owner)
		out.Clauses = append(out.Clauses, c)
	}
	return out, nil
}

// applyModifier applies one trailing "/Name.Arg" modifier. an argument
// that is not an integer is an SVar name on the card, lowered recursively
// into a compound value; anything else is a refusal, never 0.
func (lc *lowerCtx) applyModifier(v lValue, m string, depth int) (lValue, error) {
	name, arg, hasArg := strings.Cut(m, ".")
	switch name {
	case "Twice":
		if hasArg || v.Mul != 0 || v.Neg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		return kvModMultiply(v, 2), nil
	case "Thrice", "Times":
		// gorge applyCountOp: Thrice is *3, Times.N is *N (a numeric N; an
		// SVar operand refuses)
		k := 3
		if name == "Times" && hasArg {
			if _, err := strconv.Atoi(arg); err != nil {
				return lc.kvfTimesSVar(v, m, arg, depth) // value_filterheads.go
			}
		}
		if name == "Times" {
			n, err := strconv.Atoi(arg)
			if !hasArg || err != nil || n < 1 || n > 127 {
				return lValue{}, fmt.Errorf("value modifier %q", m)
			}
			k = n
		} else if hasArg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		if v.Mul != 0 || v.Neg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		return kvModMultiply(v, k), nil
	case "HalfDown":
		// gorge: v /= 2 (truncation), the kernel's Div floors: equal for a
		// non-negative operand, so a life total (which may be negative)
		// refuses, except the repeated player's (rEach, PlayerCountRemembered$
		// LifeTotal/HalfDown): the kernel's Div is Go's v /= x.Div,
		// truncating toward zero exactly like gorge's v /= 2
		if hasArg || v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg || v.Kind == vLifeOf && v.Ref.Kind != rEach || v.Kind == vConst {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		v.Div = 2
		return v, nil
	case "HalfUp":
		return kvfHalfUp(v, m, hasArg) // value_filterheads.go
	case "LimitMin", "LimitMax":
		// gorge applyCountOp: LimitMax.N is min(v, N), LimitMin.N is
		// max(v, N). A numeric N only (an SVar operand refuses), lowered to
		// ExCompare: v keeps its own value when it passes the bound, else N.
		n, aerr := strconv.Atoi(arg)
		if !hasArg || aerr != nil {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		op := uint8(opGE)
		if name == "LimitMax" {
			op = opLE
		}
		return lValue{Kind: vCompare, N: n, Op: op, Sub: []lValue{v, {Kind: vBranches, Sub: []lValue{v, cst(n)}}}}, nil
	case "NMinus":
		// gorge applyCountOp: NMinus.N is N - v (operand-first). The
		// kernel applies v*Mul/Div + Add then Neg, so Add = -N and Neg make
		// -(v - N) = N - v, exact with any prior Mul/Div. A numeric operand
		// only: gorge's /NMinus.<SVar> family is unimplemented.
		if !hasArg || arg == "" {
			return lValue{}, fmt.Errorf("value modifier %q without an operand", m)
		}
		n, aerr := strconv.Atoi(arg)
		if aerr != nil {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		if v.Add != 0 || v.Neg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		v.Add, v.Neg = -n, true
		return v, nil
	case "Mod":
		// gorge applyCountOp: v %= N for a non-zero integer N (Go's
		// remainder, the dividend's sign); an SVar operand, 0 or a negative
		// refuses. Applied on top of the value as one op.
		n, aerr := strconv.Atoi(arg)
		if !hasArg || aerr != nil || n < 1 || n > 127 || v.Mul != 0 || v.Div != 0 || v.Add != 0 || v.Neg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		return lValue{Kind: vMod, N: n, Sub: []lValue{v}}, nil
	case "Pow":
		// gorge applyCountOp's Pow.<operand>: Sub[0] raised to the operand.
		// A non-negative integer literal or an SVar naming one (Mathemagics'
		// Number$2/Pow.X, X = Count$xPaid) lowers; a negative literal or an
		// unresolved name refuses (gorge fails closed on both, and the
		// kernel's exprPow answers 0 for a negative exponent). The base keeps
		// its own Mul/Div/Add, applied before the power exactly as gorge's
		// running value is.
		if !hasArg || arg == "" {
			return lValue{}, fmt.Errorf("value modifier %q without an operand", m)
		}
		if n, aerr := strconv.Atoi(arg); aerr == nil {
			if n < 0 {
				return lValue{}, fmt.Errorf("value modifier %q: a negative exponent", m)
			}
			return lValue{Kind: vPow, Sub: []lValue{v, cst(n)}}, nil
		}
		body, ok := lc.face.SVars[arg]
		if !ok {
			return lValue{}, fmt.Errorf("value modifier %q: %s is neither a number nor an SVar", m, arg)
		}
		sv, err := lc.svarValue(body, depth+1)
		if err != nil {
			return lValue{}, fmt.Errorf("value modifier %q: SVar %s=%q: %w", m, arg, body, err)
		}
		return lValue{Kind: vPow, Sub: []lValue{v, sv}}, nil
	case "Minus", "Plus", "DivideEvenlyDown":
	default:
		return lValue{}, fmt.Errorf("value modifier %q", m)
	}
	if !hasArg || arg == "" {
		return lValue{}, fmt.Errorf("value modifier %q without an operand", m)
	}
	n, aerr := strconv.Atoi(arg)
	if aerr != nil {
		body, ok := lc.face.SVars[arg]
		if !ok && name == "Minus" && kvfInlineOperand(arg) {
			// gorge countOperandValue: an operand naming no SVar but
			// carrying a '$' is an inline count expression
			// (PlayerCountOpponents$Amount/Minus.Remembered$Amount)
			body, ok = arg, true
		}
		if !ok {
			return lValue{}, fmt.Errorf("value modifier %q: %s is neither a number nor an SVar", m, arg)
		}
		if name == "DivideEvenlyDown" {
			return lValue{}, fmt.Errorf("value modifier %q: no compound form for an SVar divisor", m)
		}
		sv, err := lc.svarValue(body, depth+1)
		if err != nil {
			return lValue{}, fmt.Errorf("value modifier %q: SVar %s=%q: %w", m, arg, body, err)
		}
		if name == "Minus" {
			sv.Neg = !sv.Neg
		}
		return lValue{Kind: vSum, Sub: []lValue{v, sv}}, nil
	}
	switch name {
	case "Minus":
		if v.Neg {
			// Neg is applied last: (N - v) + n is not -(v - N + n).
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		v.Add -= n
	case "Plus":
		if v.Neg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		v.Add += n
	case "DivideEvenlyDown":
		if n <= 0 || v.Div != 0 || v.Add != 0 || v.Mul != 0 || v.Neg {
			return lValue{}, fmt.Errorf("value modifier %q", m)
		}
		v.Div = n
	}
	return v, nil
}

// castBattlefieldValue lowers Count$LastStateBattlefieldWithFallback <spec>
// the battlefield permanents matching <spec> as the
// spell was cast (CR 601.2a; gorge castBattlefieldEvent freezes the
// battlefield the moment the spell reaches the stack, before targets,
// divisions and costs, and the resolution reads the frozen count). The
// kernel freezes ONE count per spell entry (stackEntry.trigMV), so the card
// keeps one filter (CardDef.CastBF) and a second, different one refuses
func (lc *lowerCtx) castBattlefieldValue(spec string) (lValue, error) {
	if lc.ab == nil || lc.ab.Kind != akSpell {
		return lValue{}, refuse("LastStateBattlefieldWithFallback outside a spell's own program")
	}
	f, err := lc.filter(spec)
	if err != nil {
		return lValue{}, err
	}
	if lc.out.CastBF != nil && !reflect.DeepEqual(lc.out.CastBF, f) {
		return lValue{}, refuse("LastStateBattlefieldWithFallback: a second, different filter")
	}
	lc.out.CastBF = f
	return lValue{Kind: vCastBattlefield}, nil
}

// countValue lowers the Count$ family.
// selfLeavesBody reports whether this is the body of a card's own leaves-the-battlefield
// trigger, where its source has a last-known snapshot.
func (lc *lowerCtx) selfLeavesBody() bool {
	if lc.ab == nil {
		return false
	}
	tr := lc.ab.Trigger
	return tr != nil && lc.trigBody && !lc.ab.Latent && !tr.Batch && tr.Origin == zBattlefield && (tr.Event == "Dies" || tr.Event == "Leaves") &&
		tr.F != nil && tr.F.Players == 0 && len(tr.F.Clauses) == 1 && tr.F.Clauses[0] == (lClause{Flags: fSelf})
}

func (lc *lowerCtx) countValue(rest string, depth int) (lValue, error) {
	if lc.repeatPl > 0 {
		if v, ok, err := eachOtherRevealMV(rest); ok {
			return v, err // lower_repeatplayers.go
		}
	}
	if spec, ok := strings.CutPrefix(rest, "ValidSelf "); ok {
		return lc.validSelfValue(spec) // lower_prepare.go
	}
	if spec, ok := strings.CutPrefix(rest, "CreaturesAttackedThisTurn"); ok && (spec == "" || spec[0] == ' ') {
		return lc.attackedTurnValue(spec) // value_turntallies.go
	}
	if spec, ok := strings.CutPrefix(rest, "LastStateBattlefieldWithFallback "); ok {
		return lc.castBattlefieldValue(spec)
	}
	switch rest {
	case "TotalDamageReceivedThisTurn":
		// the positive damage the source received this turn (gorge
		// Object.DamageReceivedThisTurn; CardDef.DmgAmt)
		lc.out.DmgAmt = true
		return lValue{Kind: vDmgAmount}, nil
	case "FinishedEndOfTurnsThisTurn":
		return finishedEndStepsValue(), nil // lower_addphase.go
	case "ChosenNumber":
		// Count$ChosenNumber: the resolving source's Object.ChosenNumber
		// (gorge specCtx's Resolve), set by api:ChooseNumber (the as-enters
		// entry ask or the mid-resolution ask). ExChosenNumber reads the
		// permanent's chosen byte, the resolving spell's card object, or the
		// resolution's own recorded answer.
		return lValue{Kind: vChosenNumber}, nil
	case "YourCountersEnergy":
		// Count$YourCountersEnergy: the controller's energy counters
		// (CR 122.1), the Max$ bound of the "pay one or more {E}" family.
		return lValue{Kind: vEnergyOf, Ref: lRef{Kind: rYou}}, nil
	case "OffspringPaid":
		// CR 702.175a: the cast paid the offspring cost, read off the permanent
		// the spell became (the kernel's kicked mark), 0 for a token copy. The
		// trigger reads it as it fires and is not checked again, so its body knows
		// it was paid, and a source that has left still mints.
		if lc.out.Offspring == nil {
			return lValue{}, fmt.Errorf("Count$OffspringPaid without K:Offspring")
		}
		if lc.trigBody {
			return lValue{Kind: vConst, N: 1}, nil
		}
		f, err := lc.filter("Card.Self+kicked")
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vCount, F: f}, nil
	case "xPaid":
		if v, ok, err := lc.reflexXPaid(); ok || err != nil {
			return v, err // lower_triggerxcost.go: a "when you do" body under a paid {X}
		}
		if lc.ab != nil && lc.ab.Kind == akTriggered && !lc.ab.Cost.Mana.X && !lc.trigXPaid {
			// a trigger's X without an {X} of its own is the X of the
			// spell its trigger object was (CR 107.3m, gorge
			// TriggerContext.TriggerPaidX): bound as it triggers, only
			// for the shapes the kernel captures (lower_xbound.go)
			if err := lc.xBound(); err != nil {
				return lValue{}, err
			}
		}
		return lValue{Kind: vX}, nil
	case "CastTotalManaSpent":
		// gorge Object.ManaSpent (FlagManaSpent): read only by the clone
		// ask's bound (Mockingbird), where the spell is about to enter
		// and by a spell's own resolution (Memory Deluge's Dig X): the
		// resolving spell entry's recorded spend
		if lc.ab != nil && lc.ab.Kind == akLatent && lc.cloneRepl {
			return lValue{Kind: vCastManaSpent}, nil
		}
		if lc.replSelf && !lc.cloneRepl { // a self ETB replacement reads the entering spell (gorge count_body_paid.go:131)
			lc.out.SpentSelf = true
			return lValue{Kind: vCastManaSpent}, nil
		}
		// the resolving spell's own chain (the spell entry
		// carries the spend), or a self-ETB trigger body (bound as it
		// triggers, Tf2SpentBound); every other context refuses
		if lc.ab != nil && lc.ab.Kind == akSpell && !lc.ab.Latent {
			lc.out.SpentSelf = true
			return lValue{Kind: vCastManaSpent}, nil
		}
		if err := lc.spentBound(); err != nil {
			return lValue{}, err
		}
		lc.out.SpentSelf = true
		return lValue{Kind: vCastManaSpent}, nil
	case "Converge":
		return lc.convergeValue() // value_converge.go
	case "AttackersDeclared":
		return lValue{Kind: vAttackedThisTurn}, nil
	case "CrewSize":
		// gorge Count$CrewSize (effects/
		// count_body_simple.go evalCountBodyCostCrewSize) counts the
		// battlefield creatures whose CrewedThisTurn pairing names the
		// source this turn (pairedWithSourceThisTurn). Reuse the ordinary
		// Count$Valid lowering with the F2CrewedSrc filter; no new Ex.
		f, err := lc.filter("Creature.CrewedThisTurn")
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vCount, F: f}, nil
	case "YourLifeTotal":
		return lValue{Kind: vLifeOf, Ref: lRef{Kind: rYou}}, nil
	case "CardsInYourHand":
		return lValue{Kind: vHandSizeOf, Ref: lRef{Kind: rYou}}, nil
	case "YourStartingLife":
		return lValue{Kind: vStartingLife}, nil
	case "CardPower":
		return lValue{Kind: vPowerOf, Ref: lRef{Kind: rSelf}}, nil
	case "CardToughness":
		return lValue{Kind: vToughnessOf, Ref: lRef{Kind: rSelf}}, nil
	case "TotalDamageDoneByThisTurn":
		return lc.totalDamageValue()
	case "LifeYouGainedThisTurn":
		return lValue{Kind: vLifeGainedThisTurn}, nil
	case "LifeYouLostThisTurn":
		// the sibling of LifeYouGainedThisTurn (gorge value_heads.go:
		// both heads); Rowan, Scion of War's chosen X
		return lValue{Kind: vLifeLostThisTurn}, nil
	case "YourTurns":
		return lValue{Kind: vYourTurns}, nil // gorge TurnsTaken
	case "YourExtraTurns":
		return lValue{Kind: vYourExtraTurns}, nil // gorge ExtraTurnsBegun
	case "LifeOppsLostThisTurn":
		return lValue{Kind: vOppLifeLostThisTurn}, nil
	case "RememberedSize":
		return lValue{Kind: vRememberedCount}, nil
	case "RememberedNumber":
		// gorge count_body_paid.go evalCountBodyPaidRememberedNumber: the
		// resolution's capture-excluded remembered count, the list
		// Remembered$Amount reads (its ExchangeLife / FlipCoin bindings are ops
		// kgen never pairs with it; RememberCounteredCMC's is lowered below).
		// S marks the ctx-only read for refuseCtxRememberedCount.
		if lc.counterCMCWriter() { // lower_counterspell.go
			// a Counter RememberCounteredCMC$ bound the countered spell's mana
			// value: the number lands in resCtx.remLK (refuseRemLKIPT pairs it)
			lc.remLKN++
			return lValue{Kind: vRemLKIPT, N: 2}, nil
		}
		return lValue{Kind: vRememberedCount, S: "ctx"}, nil
	case "ThisTurnEntered_Graveyard_from_Battlefield_Creature":
		// The one exact ThisTurnEntered spelling: creatures put into a
		// graveyard from the battlefield this turn. genState.died is a
		// per-ADD tally (it increments on every creature's leave, CR
		// 603.10a's WasCreature read at the move), so a card that died
		// twice counts twice and a card that has since left the graveyard
		// still counts -- gorge's state.Entered semantics.
		return lValue{Kind: vCreaturesDied}, nil
	case "NonCombatDamageThisTurn Card.Red+YouCtrl Any":
		return lValue{Kind: vRedNCDmg}, nil // gorge evalDamageHistory
	case "ValidHand Card.YouOwn":
		// gorge's zone count over every hand: the cards you own there are
		// exactly your hand. N 1: the cards, not the zone length -- a token
		// bounced into the hand waits there (CR 704.5d) and is no card (CR
		// 111.6), as gorge's zoneCountFold.visit reads.
		return lValue{Kind: vHandSizeOf, N: 1, Ref: lRef{Kind: rYou}}, nil
	case "ValidHand Card":
		// gorge's zone count over every hand: both players' hand sizes
		return lValue{Kind: vSum, Sub: []lValue{{Kind: vHandSizeOf, N: 1, Ref: lRef{Kind: rYou}}, {Kind: vHandSizeOf, N: 1, Ref: lRef{Kind: rEachOpp}}}}, nil
	case "ValidHand Card.ActivePlayerCtrl":
		// gorge's zone count over every hand, each card controlled by its
		// owner: the active player's hand size, yours on your turn and the
		// opponent's otherwise
		hand := func(r refKind) lValue { return lValue{Kind: vHandSizeOf, N: 1, Ref: lRef{Kind: r}} }
		return lValue{Kind: vCompare, N: 1, Op: opGE, Sub: []lValue{{Kind: vYourTurns, N: 1}, {Kind: vBranches, Sub: []lValue{hand(rYou), hand(rEachOpp)}}}}, nil
	case "ValidHand Land.YouOwn":
		// gorge's zone count over every hand (Land Grant): a
		// general spec counts each hand's matching cards
		f, err := lc.filter("Land.YouOwn")
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vCountHand, F: f}, nil
	case "ValidLibrary Card.YouOwn":
		return lValue{Kind: vLibrarySizeOf, Ref: lRef{Kind: rYou}}, nil
	case "YouDescendedThisTurn":
		// gorge count_body.go Count$YouDescendedThisTurn is descendedThisTurn
		// (player_filter.go): the permanent CARDS (no token, no copy) that
		// entered the controller's graveyard this turn from any zone, the
		// same count the You.descended trigger gate lowers (CR 700.11)
		return lc.countValue("ThisTurnEntered_Graveyard_Permanent.YouOwn+nonToken", depth)
	case "ResolvedThisTurn":
		if lc.ab == nil || lc.ab.Kind != akTriggered && lc.ab.Kind != akActivated || lc.ab.Latent {
			return lValue{}, fmt.Errorf("Count$ResolvedThisTurn outside a printed triggered or activated ability")
		}
		lc.ab.ResolveCount = true
		return lValue{Kind: vResolvedThisTurn}, nil
	}
	if spec, ok := strings.CutPrefix(rest, "ValidHand "); ok && !strings.ContainsAny(spec, "$,") && !strings.Contains(spec, "ActivePlayer") {
		// gorge's zone count over every hand (Land Grant's arm above): each
		// hand's matching cards. A hand card's controller is its owner (a
		// card outside the battlefield is controlled by its owner), so a
		// YouCtrl/YouOwn clause reads the same seat
		// (Ghalta's HandX). ActivePlayerCtrl reads the active seat, not the
		// owner the kernel's hand walk carries: refused, no lockstep pool
		// reads it
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vCountHand, F: f}, nil
	}
	if spec, ok := strings.CutPrefix(rest, "ThisTurnEntered_"); ok {
		// gorge evalThisTurnEntered counts a per-ADD list (state.Entered):
		// a card that entered a zone twice counts twice, and a card that
		// has since moved on is still listed, its validity read against
		// the object wherever it now lives (CR 400.7 makes the moved card
		// a new object; CR 603.10a supplies the departure-time creature
		// status). The kernel's genState.entered ring carries the same
		// entries.
		prop := uint8(kop.PropNone)
		if head, tail, found := strings.Cut(spec, "$"); found {
			if p, ok := thisTurnEnteredProps[strings.TrimSpace(tail)]; ok {
				spec, prop = head, p
			}
		}
		dest, origin, hasOrigin, valid, ok := parseThisTurnEnteredSpec(spec)
		if !ok {
			return lValue{}, refuse("other:count ThisTurnEntered %q", spec)
		}
		if dest == kop.ZoneCommand {
			// no kernel zone for Command (a card there is in exile with a
			// marker); the corpus has no such head
			return lValue{}, refuse("other:count ThisTurnEntered dest Command")
		}
		if dest == kop.ZoneStack {
			// the kernel does not record stack entries: a stack-entry match
			// has no stable object once the spell resolves; refuse until a
			// corpus head needs one
			return lValue{}, refuse("other:count ThisTurnEntered dest Stack")
		}

		if dest == kop.ZoneBattlefield && hasOrigin && origin == kop.ZoneBattlefield {
			// unreachable in the kernel: a permanent never moves
			// battlefield-to-battlefield (a blink goes through exile)
			return lValue{}, refuse("other:count ThisTurnEntered Battlefield_from_Battlefield")
		}
		v := lValue{Kind: vThisTurnEntered, Zone: uint8(dest), From: -1, Prop: prop}
		if hasOrigin {
			v.From = int8(origin)
		}
		var err error
		if v.F, err = lc.filter(valid); err != nil {
			return lValue{}, err
		}
		if cr, other, split := splitThisTurnEnteredCreature(valid); split {
			if v.F2, err = lc.filter(cr); err != nil {
				return lValue{}, err
			}
			if other != "" {
				if v.F3, err = lc.filter(other); err != nil {
					return lValue{}, err
				}
			}
		}
		if dest == kop.ZoneBattlefield {
			v.F4 = v.F
		}
		return v, nil
	}
	if spec, ok := strings.CutPrefix(rest, "LeftGraveyardThisTurn"); ok && (spec == "" || spec[0] == ' ') {
		// gorge evalCov3HeadLeftZoneThisTurn: distinct cards that left a
		// graveyard this turn, the spec matched where each now lives; an
		// empty spec is Card. Only the two exact spellings are read.
		switch spec = strings.TrimSpace(spec); spec {
		case "", "Card":
			spec = "Card"
		case "Card.YouOwn":
		default:
			return lValue{}, refuse("Count$LeftGraveyardThisTurn spec %q", spec)
		}
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vLeftZone, Zone: uint8(kop.ZoneGraveyard), F: f}, nil
	}
	if spec, ok := strings.CutPrefix(rest, "LeftBattlefieldThisTurn"); ok && (spec == "" || spec[0] == ' ') {
		// gorge evalCov3HeadLeftZoneThisTurn with from = Battlefield: the
		// distinct objects that left the battlefield this turn, the spec
		// matched where each now lives. Only the corpus
		// spellings are read; the whitelist also excludes any `$` or
		// cast-provenance word.
		switch spec = strings.TrimSpace(spec); spec {
		case "", "Card":
			spec = "Card"
		case "Creature.YouCtrl", "Creature":
		default:
			return lValue{}, refuse("Count$LeftBattlefieldThisTurn spec %q", spec)
		}
		f, err := lc.filter(spec)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: vLeftZone, Zone: uint8(kop.ZoneBattlefield), F: f}, nil
	}
	if arg, ok := strings.CutPrefix(rest, "CountersAddedThisTurn "); ok {
		return lc.ctrAddedValue(arg) // value_countersadded.go
	}
	if spec, ok := strings.CutPrefix(rest, "DifferentCounterKinds_"); ok {
		return lc.counterKindsValue(spec) // value_counterkinds.go
	}
	if spec, ok := strings.CutPrefix(rest, "ThisTurnCast_"); ok {
		return lc.thisTurnCastValue(spec) // value_thisturncast.go
	}
	if spec, ok := strings.CutPrefix(rest, "ThisTurnActivated_"); ok {
		return lc.thisTurnActivatedValue(spec) // value_thisturnactivated.go
	}
	if strings.HasPrefix(rest, "Compare ") {
		return lc.compareValue(strings.TrimPrefix(rest, "Compare "), depth)
	}
	if rest == "Domain" {
		return lc.domainValue()
	}
	if strings.HasPrefix(rest, "Delirium.") {
		return lc.deliriumValue(strings.TrimPrefix(rest, "Delirium."), depth)
	}
	if v, ok, err := lc.countHeads8(rest, depth); ok {
		return v, err // value_countheads.go
	}
	if rest == "YouDrewThisTurn" {
		return lValue{Kind: vDrawnThisTurn}, nil // gorge CardsDrawnThisTurn
	}
	if rest == "YouFaceDilemmaThisTurn" {
		return lValue{Kind: vDilemmasFaced}, nil // gorge Player.DilemmasFacedThisTurn
	}
	if rest == "YouScryThisTurn" {
		return lValue{Kind: vScriedThisTurn, N: 0}, nil // gorge ScriedThisTurn
	}
	if rest == "YouSurveilThisTurn" {
		return lValue{Kind: vScriedThisTurn, N: 1}, nil // gorge SurveilledThisTurn
	}
	if rest == "YourSpeed" {
		return lValue{Kind: vYourSpeed}, nil // gorge evalCov3HeadYourSpeed: the controller's speed
	}
	if rest == "Party" {
		return lValue{Kind: vParty}, nil // gorge partySize
	}
	if strings.HasPrefix(rest, "Devotion.") {
		return devotionValue(strings.TrimPrefix(rest, "Devotion."))
	}
	if strings.HasPrefix(rest, "CardCounters.") {
		ref := lRef{Kind: rSelf}
		if lc.selfLeavesBody() {
			ref.Kind = rTriggered // the counters it left with (CR 608.2h): Selfless Police Captain's
		}
		return lValue{Kind: vCountersOn, Ref: ref, S: strings.TrimPrefix(rest, "CardCounters.")}, nil
	}
	if arms, ok := strings.CutPrefix(rest, "Metalcraft."); ok {
		return lc.metalcraftValue(arms, depth) // value_filterheads.go
	}
	if arms, ok := strings.CutPrefix(rest, "wasCastFromExile."); ok {
		return lc.castFromExileValue(arms) // lower_castfromexile.go
	}
	if arms, ok := strings.CutPrefix(rest, "Threshold."); ok {
		return lc.thresholdValue(arms, depth) // value_threshold.go
	}
	if arms, ok := strings.CutPrefix(rest, "Hellbent."); ok {
		// gorge cov3BranchHolds: <yes> with no cards in your hand, else <no>
		parts := strings.Split(arms, ".")
		if len(parts) != 2 {
			return lValue{}, refuse("Count$Hellbent.%s", arms)
		}
		var br [2]lValue
		for i, t := range parts {
			var err error
			if br[i], err = lc.headArm("Hellbent", t, depth); err != nil {
				return lValue{}, err
			}
		}
		return lValue{Kind: vCompare, N: 0, Op: opLE, Sub: []lValue{{Kind: vHandSizeOf, Ref: lRef{Kind: rYou}}, {Kind: vBranches, Sub: br[:]}}}, nil
	}
	if rest == "TimesKicked" {
		// the count of kicker payments. A plain Kicker is paid once or not
		// at all, so it reads as the kicked bit; a Multikicker reads the
		// repeated payment count (gorge count_body_paid.go's TimesKicked).
		switch {
		case lc.out.Multikicker != nil && lc.out.Kicker != nil:
			return lValue{}, refuse("Count$TimesKicked with both Kicker and Multikicker")
		case lc.out.Multikicker != nil:
			return lValue{Kind: vTimesKicked}, nil
		case lc.out.Kicker != nil:
			return lValue{Kind: vKicked, Sub: []lValue{cst(1), cst(0)}}, nil
		case faceHasKeyword(lc.face, "Multikicker"):
			// the keyword itself refused (an {X} base cost, a non-mana
			// payment): name the refused Multikicker, not a missing Kicker
			return lValue{}, refuse("Count$TimesKicked with a refused Multikicker")
		}
		return lValue{}, refuse("Count$TimesKicked without a Kicker keyword")
	}
	for _, cv := range []struct {
		pre     string
		kind    valKind
		bargain bool
	}{{"Kicked.", vKicked, false}, {"Teamwork.", vKicked, false}, {"Morbid.", vMorbid, false}, {"PromisedGift.", vPromisedGift, false},
		{"CommittedCrimeThisTurn.", vCommittedCrime, false}, {"Void.", vVoid, false}, {"IfCastInOwnMainPhase.", vIfCastOwnMain, false},
		{"Monarch.", vMonarch, false},
		{"wasCastFromGraveyard.", vCastFromGrave, false},
		{"Bargained.", vKicked, true}, {"Bargain.", vKicked, true}, {"OptionalGenericCostPaid.", vKicked, false}} {
		if !strings.HasPrefix(rest, cv.pre) {
			continue
		}
		if cv.kind == vCastFromGrave && (lc.ab == nil || lc.ab.Kind != akSpell) {
			// gorge reads the resolving object's persisting CastFlags; the
			// kernel reads the resolving stack entry, so only a spell
			// ability has the same object.
			return lValue{}, refuse("Count$%s outside a spell ability", strings.TrimSuffix(cv.pre, "."))
		}
		if cv.kind == vIfCastOwnMain && (lc.ab == nil || lc.ab.Kind != akSpell) {
			// the kernel reads the spell entry under resolution or announcement
			return lValue{}, refuse("Count$IfCastInOwnMainPhase outside a spell ability")
		}
		if cv.pre == "Teamwork." && lc.out.Teamwork == 0 {
			return lValue{}, refuse("Count$%s without Teamwork", rest)
		}
		if cv.bargain && !lc.out.Bargain {
			return lValue{}, refuse("Count$%s without Bargain", rest)
		}
		if cv.pre == "OptionalGenericCostPaid." {
			if err := lc.optCostRead(rest); err != nil {
				return lValue{}, err // lower_optcost.go
			}
		}
		parts := strings.Split(strings.TrimPrefix(rest, cv.pre), ".")
		if len(parts) != 2 {
			if cv.kind == vCommittedCrime || cv.kind == vVoid || cv.kind == vIfCastOwnMain || cv.kind == vMonarch {
				return lValue{}, refuse("Count$%s arms %q", strings.TrimSuffix(cv.pre, "."), rest)
			}
			return lValue{}, fmt.Errorf("Count$%s", rest)
		}
		if cv.kind == vCommittedCrime || cv.kind == vVoid || cv.kind == vIfCastOwnMain || cv.kind == vMonarch {
			// gorge cov3BranchHoldsCommittedCrimeThisTurn / ...HoldsVoid /
			// evalCountBodyDottedMorbidOrMonarch's Monarch arm: two
			// arms, read as integers here (an SVar arm refuses)
			for _, p := range parts {
				if _, err := strconv.Atoi(p); err != nil {
					return lValue{}, refuse("Count$%s arms %q", strings.TrimSuffix(cv.pre, "."), rest)
				}
			}
		}
		if cv.kind == vPromisedGift && !lc.out.Gift {
			return lValue{}, fmt.Errorf("Count$%s without a gift", rest)
		}
		v := lValue{Kind: cv.kind}
		for _, p := range parts {
			if n, err := strconv.Atoi(p); err == nil {
				v.Sub = append(v.Sub, cst(n))
				continue
			}
			body, ok := lc.face.SVars[p]
			if !ok {
				return lValue{}, fmt.Errorf("Count$%s: %s names no SVar", rest, p)
			}
			sv, err := lc.svarValue(body, depth+1)
			if err != nil {
				return lValue{}, err
			}
			v.Sub = append(v.Sub, sv)
		}
		return v, nil
	}
	if rest == "ValidStack Card.Self+sneaked" {
		// The Last Ronin's Technique: the resolving
		// spell's own sneak provenance, 1 or 0. The stack entry's trigPlSneak
		// mark is the stack-side read; a permanent's is the arena's mechSneaked.
		return lValue{Kind: vSneaked, Sub: []lValue{cst(1), cst(0)}}, nil
	}
	if strings.HasPrefix(rest, "ValidStack ") {
		return lValue{}, refuse("Count$%s: only ValidStack Card.Self+sneaked is read", rest)
	}
	for _, zc := range []struct {
		pre  string
		kind []valKind
	}{{"ValidExile ", []valKind{vCountExile}}, {"ValidGraveyard,Exile ", []valKind{vCountGraveyard, vCountExile}},
		{"ValidBattlefield,Graveyard ", []valKind{vCount, vCountGraveyard}},
		// gorge multiCountZones: the named zones' union, in any order
		{"ValidGraveyard,Battlefield ", []valKind{vCountGraveyard, vCount}}} {
		if !strings.HasPrefix(rest, zc.pre) {
			continue
		}
		expr := strings.TrimPrefix(rest, zc.pre)
		if e, agg, ok := strings.Cut(expr, "$"); ok && zc.pre == "ValidExile " && agg == "CardTypes" {
			// the distinct card types among the matching exiled cards (CR
			// 205.2a; gorge's $CardTypes aggregate over the exile zones;
			// Keen-Eyed Curator): kernel ExCountExile N=1.
			// Any other exile aggregate stays refused
			f, err := lc.filter(e)
			if err != nil {
				return lValue{}, err
			}
			return lValue{Kind: vExileTypes, F: f}, nil
		}
		if strings.Contains(expr, "$") {
			return lValue{}, fmt.Errorf("Count$%s: aggregate", rest)
		}
		f, err := lc.filter(expr)
		if err != nil {
			return lValue{}, err
		}
		if len(zc.kind) == 1 {
			return lValue{Kind: zc.kind[0], F: f}, nil
		}
		return lValue{Kind: vSum, Sub: []lValue{{Kind: zc.kind[0], F: f}, {Kind: zc.kind[1], F: f}}}, nil
	}
	if strings.HasPrefix(rest, "Valid ") || strings.HasPrefix(rest, "ValidGraveyard ") {
		kind := vCount
		expr := strings.TrimPrefix(rest, "Valid ")
		if strings.HasPrefix(rest, "ValidGraveyard ") {
			kind = vCountGraveyard
			expr = strings.TrimPrefix(rest, "ValidGraveyard ")
		}
		if e, agg, ok := strings.Cut(expr, "$"); ok {
			expr = e
			// every aggregate below reads the battlefield --
			// ExSumPower/ExDistinctCMC walk s.occ, and validAggs is the
			// ExValidAgg fold over battlefield permanents. A graveyard head
			// must not borrow one of them: it takes ExGraveAgg or
			// refuses, never re-targets the zone.
			if kind == vCountGraveyard {
				// the mana-value folds have a graveyard twin
				// (ExGraveAgg); every other graveyard aggregate refuses
				a, ok := graveAggs[agg]
				if !ok {
					return lValue{}, fmt.Errorf("Count$ValidGraveyard aggregate %q", agg)
				}
				f, err := lc.filter(e)
				if err != nil {
					return lValue{}, err
				}
				return lValue{Kind: vGraveAgg, N: a, F: f}, nil
			}
			switch agg {
			case "CardPower":
				kind = vSumPower
			case "DifferentCardManaCost":
				kind = vDistinctCMC
			case "PlaneswalkerType":
				// CR 205.3j: the distinct subtypes printed on the matching
				// planeswalkers (gorge zoneFold's PlaneswalkerType): kernel
				// ExCount N=1
				kind = vPWTypes
			default:
				if ck, ok := strings.CutPrefix(agg, "CardCounters."); ok && ck != "ALL" {
					// gorge zoneFold: CardCounters.<KIND> sums o.Counter(kind).
					// ALL (every kind) has no kernel read (refCounters takes
					// one kind) and keeps refusing below.
					if _, kerr := counterKind(ck); kerr != nil {
						return lValue{}, fmt.Errorf("Count$Valid aggregate %q", agg)
					}
					f, err := lc.filter(e)
					if err != nil {
						return lValue{}, err
					}
					return lValue{Kind: vValidAgg, N: kop.AggSumCounters, F: f, S: ck}, nil
				}
				if a, ok := validAggs[agg]; ok {
					f, err := lc.filter(e)
					if err != nil {
						return lValue{}, err
					}
					return lValue{Kind: vValidAgg, N: a, F: f}, nil
				}
				return lValue{}, fmt.Errorf("Count$Valid aggregate %q", agg)
			}
		}
		f, err := lc.filter(expr)
		if err != nil {
			return lValue{}, err
		}
		return lValue{Kind: kind, F: f}, nil
	}
	return lValue{}, fmt.Errorf("Count$%s", rest)
}

// cmpSpec parses a compare spec such as "GE3", "EQ1", "GTY" (Y an SVar).
func (lc *lowerCtx) cmpSpec(s string) (uint8, lValue, error) {
	if len(s) < 3 {
		return 0, lValue{}, fmt.Errorf("compare %q", s)
	}
	op, ok := opNamesCmp[s[:2]]
	if !ok {
		return 0, lValue{}, fmt.Errorf("compare %q", s)
	}
	v, err := lc.value(s[2:])
	return op, v, err
}

// domainValue lowers Count$Domain (gorge domainCount: for each basic land
// type, 1 if Count$Valid <Type>.YouCtrl is positive) as the sum of five
// at-least-one battlefield counts.
func (lc *lowerCtx) domainValue() (lValue, error) {
	var v lValue
	for i, t := range []string{"Plains", "Island", "Swamp", "Mountain", "Forest"} {
		f, err := lc.filter(t + ".YouCtrl")
		if err != nil {
			return lValue{}, err
		}
		one := lValue{Kind: vCountAtLeast, F: f, N: 1, Sub: []lValue{cst(1), cst(0)}}
		if i == 0 {
			v = one
			continue
		}
		v = lValue{Kind: vSum, Sub: []lValue{v, one}}
	}
	return v, nil
}

// thisTurnEnteredZoneWords mirrors gorge's zoneWords (the zone names a
// Count$ThisTurnEntered_<Dest> or from_<Origin> may name).
var thisTurnEnteredZoneWords = map[string]kop.Zone{
	"Battlefield": kop.ZoneBattlefield,
	"Graveyard":   kop.ZoneGraveyard,
	"Hand":        kop.ZoneHand,
	"Library":     kop.ZoneLibrary,
	"Exile":       kop.ZoneExile,
	"Stack":       kop.ZoneStack,
	"Command":     kop.ZoneCommand,
}

// thisTurnEnteredProps are gorge's modeledProperty set, the only $<Property>
// tails split off the <Valid> filter.
var thisTurnEnteredProps = map[string]uint8{
	"CardPower":     kop.PropCardPower,
	"CardToughness": kop.PropCardToughness,
	"CardManaCost":  kop.PropCardManaCost,
}

// parseThisTurnEnteredSpec splits ThisTurnEntered_<Dest>[_from_<Origin>]_<Valid>
// exactly as gorge's parseThisTurnEnteredSpec: at most five underscore parts,
// a known destination and (when present) origin, and a non-empty valid tail
// (rejoined when it spans several tokens). An unknown word or an empty valid
// is not a spec at all.
func parseThisTurnEnteredSpec(rest string) (dest kop.Zone, origin kop.Zone, hasOrigin bool, valid string, ok bool) {
	parts := strings.Split(strings.TrimSpace(rest), "_")
	if len(parts) < 2 || len(parts) > 5 {
		return 0, 0, false, "", false
	}
	d, known := thisTurnEnteredZoneWords[parts[0]]
	if !known {
		return 0, 0, false, "", false
	}
	if len(parts) >= 3 && parts[1] == "from" {
		if len(parts) < 4 {
			return 0, 0, false, "", false
		}
		o, known := thisTurnEnteredZoneWords[parts[2]]
		if !known {
			return 0, 0, false, "", false
		}
		v := strings.Join(parts[3:], "_")
		if strings.TrimSpace(v) == "" {
			return 0, 0, false, "", false
		}
		return d, o, true, v, true
	}
	v := strings.Join(parts[1:], "_")
	if strings.TrimSpace(v) == "" {
		return 0, 0, false, "", false
	}
	return d, 0, false, v, true
}

// splitThisTurnEnteredCreature mirrors gorge's
// splitCreatureBaseAlternatives: each alternative whose base word is exactly
// Creature moves to the creature group, its base rewritten to Card (the
// departure-time WasCreature bit answers the creature test); every other
// alternative stays verbatim.
func splitThisTurnEnteredCreature(spec string) (creature, other string, ok bool) {
	var cr, ot []string
	for _, alt := range strings.Split(spec, ",") {
		alt = strings.TrimSpace(alt)
		if alt == "" {
			continue
		}
		base, rest, dotted := strings.Cut(alt, ".")
		if base != "Creature" {
			ot = append(ot, alt)
			continue
		}
		ok = true
		if dotted {
			cr = append(cr, "Card."+rest)
		} else {
			cr = append(cr, "Card")
		}
	}
	if !ok {
		return "", "", false
	}
	return strings.Join(cr, ","), strings.Join(ot, ","), true
}
