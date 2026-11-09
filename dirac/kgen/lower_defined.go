package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// the grammar sweep over other:Defined$ "_".

// sacrificedGroup admits ConditionDefined$ Sacrificed + ConditionPresent$ f
// (gorge conditionMet, defined == "Sacrificed"): the group is the cost's one
// sacrificed object, matched as the card it is now (a graveyard card; a
// ceased token is skipped). The kernel reads it from the entry's LKI record,
// which only a sacrifice paid as the ability's own cost leaves
// (sacrificedIsCost). A filter that reads state rather than printed
// characteristics (controller, counters, power, tapped, modified, ...) is
// refused: the object is no longer on the battlefield to read them from.
func (lc *lowerCtx) sacrificedGroup(f *lFilter) error {
	if !lc.sacrificedIsCost() {
		return refuse("ConditionDefined$ Sacrificed without exactly one sacrifice paid as the ability's own cost")
	}
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("ConditionDefined$ Sacrificed with a player or empty filter")
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoAny || c.Owner != whoAny || c.CmpStat != cmpNone || c.BasePermanent ||
			c.Flags&^(fLegendary|fBasic) != 0 {
			return refuse("ConditionDefined$ Sacrificed with a filter that reads state, not printed characteristics")
		}
	}
	return nil
}

// playerSetPred is a Player.<q> / Opponent.<q> qualifier kgen reads exactly:
// the kernel predicate code (kop.PlayerPred*, | PlayerPredNeg) and, for
// controlsCreature, its creature filter.
type playerSetPred struct {
	base string // "Player" or "Opponent"
	code int
	f    *lFilter
	seat bool // seat order (seat 0 first), not controller first
}

// playerSetAPIs are the ops whose Defined$ / TokenOwner$ player set the
// kernel restricts (Draw, LoseLife and Token through OpRepeatEachPlayer's
// predicate, DealDamage through the each-player damage sequence).
var playerSetAPIs = map[string]string{"Draw": "Defined", "LoseLife": "Defined", "Token": "TokenOwner", "DealDamage": "Defined"}

// playerSetSpec reads a qualified player set (gorge definedSpec,
// context.go: the AliveFrom(controller) candidates, the controller skipped
// for base Opponent, each kept when MatchesPlayerSpecWithSVars admits it;
// the set is built once, before the effect acts). matched is false for a
// value that is not a qualified Player./Opponent. spelling (the plain
// values defined() reads stay on its path).
func (lc *lowerCtx) playerSetSpec(api, v string) (pred playerSetPred, matched bool, err error) {
	v = strings.TrimSpace(v)
	if api == "Token" && v == "Player" {
		// CreateToken has one owner, so the set takes the qualifier's player
		// loop; gorge's effToken lists these owners with AliveFrom(0).
		return playerSetPred{base: v, seat: true}, true, nil
	}
	base, q, ok := strings.Cut(v, ".")
	if !ok || (base != "Player" && base != "Opponent") {
		return pred, false, nil
	}
	switch v {
	case "Player.Opponent", "Player.Other", "Player.IsRemembered":
		return pred, false, nil
	}
	pred.base = base
	neg := 0
	switch {
	case strings.HasPrefix(q, "controlsCreature."):
		rem := strings.TrimPrefix(q, "controlsCreature.")
		if strings.ContainsAny(rem, "_+,!") {
			return pred, true, refuse("Defined$ %q: controlsCreature with a count suffix or conjunction", v)
		}
		f, err := lc.filter("Creature." + rem)
		if err != nil {
			return pred, true, err
		}
		for _, c := range f.Clauses {
			if c.Types != tCreature || c.TypesAll != 0 || c.Ctrl != whoAny || c.Owner != whoAny || c.Flags != 0 ||
				(c.CmpStat != cmpNone && c.CmpStat != cmpMultiColor) || c.BasePermanent {
				return pred, true, refuse("Defined$ %q: controlsCreature filter reads more than printed colours and subtypes", v)
			}
		}
		if f.Players != 0 || len(f.Clauses) != 1 {
			return pred, true, refuse("Defined$ %q: controlsCreature filter", v)
		}
		pred.code, pred.f = kop.PlayerPredControlsCreature, f
	case q == "MaxSpeed", q == "!MaxSpeed":
		pred.code = kop.PlayerPredMaxSpeed
		if q[0] == '!' {
			neg = kop.PlayerPredNeg
		}
	case q == "IsCorrupted":
		pred.code = kop.PlayerPredCorrupted
	case q == "controlsEnchantment,controlsArtifact":
		// TokenOwner$ only (gorge tokenOwnerPlayers, token.go): the alive
		// seats from seat 0 that control an Enchantment or an Artifact (any
		// permanent of either type). definedSpec, which every other API reads,
		// has no such qualifier: no seat would match. gorge's tokenOwnerPlayers
		// always walks from seat 0, so a base Opponent spelling (which would
		// exclude the controller) is not that set: refuse it rather than let
		// the seat-order branch override RefEachOpp.
		if base != "Player" {
			return pred, true, refuse("Defined$ %q: a TokenOwner$-only seat spelling on %s", v, base)
		}
		pred.code, pred.seat = kop.PlayerPredArtOrEnch, true
	default:
		return pred, true, refuse("Defined$ %q: player qualifier", v)
	}
	pred.code |= neg
	if pred.seat && api != "Token" {
		return pred, true, refuse("Defined$ %q: a TokenOwner$-only spelling on %s", v, api)
	}
	return pred, true, nil
}

// wrapPlayerSet rewrites the instructions a Draw / LoseLife / Token node
// emitted over Player (or Opponent) into an OpRepeatEachPlayer body that
// runs for the seats the predicate keeps, or tags the each-player damage
// instruction with it.
func (lc *lowerCtx) wrapPlayerSet(api string, at int, pred playerSetPred) error {
	kind := rEachPlayer
	if pred.base == "Opponent" {
		kind = rEachOpp
	}
	prog := lc.ab.Prog[at:]
	if len(prog) == 0 {
		return refuse("%s over a qualified player set emitted nothing", api)
	}
	for i := range prog {
		if prog[i].Obj.Kind != kind || prog[i].Op != api && !(api == "Token" && prog[i].Op == "CreateToken") {
			return refuse("%s over a qualified player set: unexpected %s", api, prog[i].Op)
		}
	}
	if api == "DealDamage" {
		if len(prog) != 1 || kind != rEachPlayer {
			return refuse("DealDamage over a qualified opponent set")
		}
		prog[0].B, prog[0].F = cst(pred.code), pred.f
		return nil
	}
	for i := range prog {
		prog[i].Obj = lRef{Kind: rEach}
	}
	c := 1
	// The seat-order flag makes the kernel walk seats 0,1; that is gorge's
	// tokenOwnerPlayers order and only applies to the all-players set. An
	// opponent set (RefEachOpp) must never take it, or the controller would
	// be included.
	if pred.seat && kind == rEachPlayer {
		c |= kop.RepeatEachSeatOrder
	}
	lc.ab.Prog = append(lc.ab.Prog, lInstr{})
	copy(lc.ab.Prog[at+1:], lc.ab.Prog[at:])
	lc.ab.Prog[at] = lInstr{Op: "RepeatEach", Obj: lRef{Kind: kind}, C: c, Skip: len(lc.ab.Prog) - at - 1, B: cst(pred.code), F: pred.f}
	return nil
}

// definedValid lowers Defined$ Valid <filter> as the subject of Pump,
// PutCounter and MultiplyCounter (gorge definedSpec -> battlefieldValidTargets:
// every battlefield permanent matching the filter, the set read once at
// resolution, seat 0 first, each battlefield in entry order; the effect then
// loops over it: effPump resolves NumAtt$ / NumDef$ per object, effPutCounter
// and effMultiplyCounter act per object). done is false for any other API or
// subject, which stay on their own paths.
func (lc *lowerCtx) definedValid(sa *cards.SA, ps *params) (done bool, err error) {
	d, ok := ps.m["Defined"]
	d = strings.TrimSpace(d)
	if !ok || !strings.HasPrefix(d, "Valid") {
		return false, nil
	}
	if strings.Contains(d, " & ") {
		return true, refuse("Defined$ %q: a compound selector", d)
	}
	if !strings.HasPrefix(d, "Valid ") {
		return true, refuse("Defined$ %q: a zone-qualified Valid selector", d)
	}
	ps.get("Defined")
	f, err := lc.filter(strings.TrimSpace(strings.TrimPrefix(d, "Valid ")))
	if err != nil {
		return true, err
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return true, refuse("Defined$ %q: a player alternative", d)
	}
	switch sa.API {
	case "Pump":
		for _, c := range f.Clauses {
			if c.Types != tCreature {
				return true, refuse("Pump over Defined$ %q: not a creature-only filter", d)
			}
		}
		att, def := cst(0), cst(0)
		bits := 0
		a, hasA := ps.get("NumAtt")
		b, hasB := ps.get("NumDef")
		if hasA && a == "Double" {
			bits |= kop.PumpEachPower
		} else if hasA {
			if att, err = lc.value(a); err != nil {
				return true, err
			}
		}
		if hasB && b == "Double" {
			bits |= kop.PumpEachTough
		} else if hasB {
			if def, err = lc.value(b); err != nil {
				return true, err
			}
		}
		if bits != 0 && (hasA && a != "Double" || hasB && b != "Double") {
			return true, refuse("Pump over Defined$ %q: Double beside a fixed amount", d)
		}
		in := lInstr{Op: "PumpAllEach", F: f, A: att, B: def, C: bits}
		if kwc, ok := ps.m["KWChoice"]; ok && !hasA && !hasB {
			// a keyword-only mass grant; with a P/T change KWChoice$ stays
			// unread and the ability refuses
			ps.get("KWChoice")
			return true, lc.pumpAllEachChoice(ps, in, kwc)
		}
		if kw, ok := ps.get("KW"); ok {
			ks, err := pumpKeywords(kw)
			if err != nil {
				return true, err
			}
			in.CS = strings.Join(ks, "&")
		}
		lc.emit(in)
	case "PutCounter":
		ct := ps.str("CounterType")
		if _, err := counterKind(ct); err != nil || strings.ContainsAny(ct, ",") || ct == "" {
			return true, refuse("PutCounter over Defined$ %q: CounterType$ %q", d, ct)
		}
		n, err := lc.amount(ps, "CounterNum", 1)
		if err != nil {
			return true, err
		}
		if n.Kind != vConst || n.N < 1 || n.Mul != 0 || n.Add != 0 || n.Neg {
			return true, refuse("PutCounter over Defined$ %q: a CounterNum$ that is not a constant of at least one", d)
		}
		lc.emit(lInstr{Op: "PutCounterAll", F: f, A: n, B: cst(1), CS: ct})
	case "MultiplyCounter":
		in := lInstr{Op: "DoubleCountersAll", F: f, CS: ps.str("CounterType")}
		if in.CS == "" {
			in.CS = "All"
		}
		lc.emit(in)
	case "Animate":
		// (Sita Varma) gorge effAnimate reads the grant
		// once (parseAnimateGrant) and registers it on each object of the
		// set; Optional$ True asks only when the grant changes something
		// and the set holds a live object (animateWouldChange)
		_, hasP := ps.m["Power"]
		_, hasT := ps.m["Toughness"]
		if !hasP || !hasT {
			return true, refuse("Animate over Defined$ %q: Power$ and Toughness$ not both set", d)
		}
		ps.consume("IsCurse")
		if v, ok := ps.get("Optional"); ok {
			if strings.TrimSpace(v) != "True" {
				return true, refuse("Animate over Defined$ %q: Optional$ %q", d, v)
			}
			lc.emit(lInstr{Op: "MayAsk", C: kop.MayGuardAnimateAll, Skip: 1, Note: "you may"})
		}
		o, dur, chosen, err := lc.animateOverride(ps, lRef{})
		if err != nil {
			return true, err
		}
		if chosen {
			// an OverrideAll applies one record; the chosen-colour form is
			// five records read per object
			return true, refuse("Animate over Defined$ %q: Colors$ ChosenColor", d)
		}
		in := lInstr{Op: "OverrideAll", F: f, CS: dur, Note: overrideString(o)}
		lc.out.overrides = append(lc.out.overrides, o)
		in.C = len(lc.out.overrides) - 1
		lc.emit(in)
	case "CopyPermanent":
		// (Multiversal Incursion) gorge's Defined arm
		// sizes every source's copies through the CreateToken replacements
		// first, then mints source by source in set order (ExtCopyAllEach)
		exc, hasExc, err := lc.copyExceptions(ps)
		if err != nil {
			return true, err
		}
		in := lInstr{Op: "CopyAllEach", F: f}
		if hasExc {
			lc.out.overrides = append(lc.out.overrides, exc)
			in.C = len(lc.out.overrides) // index+1
			in.Note = "except " + overrideString(exc)
		}
		lc.emit(in)
	default:
		return false, nil
	}
	return true, nil
}
