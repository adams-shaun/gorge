package effects

import (
	"math"
	"math/bits"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// Num resolves a numeric parameter. A literal is used directly (a leading
// sign included); anything else is treated as an SVar name whose body is a
// Count$ expression, after stripping a leading sign that carries Forge's
// stat-direction convention rather than naming the reference. An expression
// this build does not model evaluates to zero rather than to the default, so
// the failure mode is "the card did nothing" rather than "the card did
// something arbitrary".
func Num(h Host, c *Ctx, sa *cards.SA, key string, def int32) int32 {
	if n, ok := NumResolved(h, c, sa, key, def); ok {
		return n
	}
	if _, present := sa.Params[key]; present {
		return 0 // present but unresolvable degrades to zero, not to def
	}
	return def
}

// NumResolved is Num plus a resolvability verdict: it answers whether the
// parameter RESOLVED under the same grammar Num reads -- a signed literal, an
// SVar name present in the context's table, a recognised inline expression
// prefix (Count$/Sacrificed$/Remembered$/TriggerCount$/ReplaceCount$), or the
// bare X. Num itself degrades an unresolvable value to zero ("the card did
// nothing"); NumResolved exists for a caller that must not confuse that
// degrade-to-zero with a LEGITIMATE zero -- rules' replacement matcher gates
// a DB$ ReplaceDamage prevention body on its Amount$ and fails closed on a
// value this build cannot price, so an unmodelled ShieldAmount frame cannot
// silently erase the damage it was supposed to partially prevent.
func NumResolved(h Host, c *Ctx, sa *cards.SA, key string, def int32) (int32, bool) {
	if c == nil {
		c = &Ctx{}
	}
	raw, ok := sa.Params[key]
	if !ok {
		return def, false
	}
	raw = strings.TrimSpace(raw)
	if n, err := strconv.Atoi(raw); err == nil {
		return int32(n), true // a signed literal ("+2"/"-2") lands here: Atoi eats the sign
	}
	// Forge writes a stat direction as a sign on the value ("NumAtt$ +X" --
	// Goblin Piledriver), so a signed non-literal is not a reference NAMED
	// with the sign but an ordinary reference carrying a direction. Strip a
	// leading sign here and resolve the bare body through the fallbacks
	// below, applying the sign to whatever they return. A lone sign with
	// nothing after it is not a value and keeps the degrade-to-zero path.
	sign := int32(1)
	if len(raw) > 1 && (raw[0] == '+' || raw[0] == '-') {
		if raw[0] == '-' {
			sign = -1
		}
		raw = raw[1:]
	}
	if c.SVars != nil {
		if body, ok := c.SVars[raw]; ok {
			return sign * EvalCount(h, c, body), true
		}
	}
	// A DB$ RollDice publication of this same resolution (effects/dice.go):
	// a sub's numeric parameter naming the roll -- Boomflinger's NumDmg$
	// Result, Grave Endeavor's LifeAmount$ Y, Neverwinter Hydra's
	// CounterNum$ Result -- reads the published value through the bare name,
	// exactly as the SVar$ indirection resolves the same name in an SVar
	// body. Checked after the card's own SVar table so a real SVar of the
	// same name keeps winning.
	if v, ok := rollPublished(c, raw); ok {
		return sign * v, true
	}
	// An inline Count$ expression (Storm's own Amount$ Count$ThisTurnCast/
	// Minus1, Task 17) is a body in its own right, not an SVar name -- a
	// param value of "Count$..." evaluates directly rather than being
	// mistaken for an SVar lookup (which would fail and degrade the count to
	// zero, silencing the whole SpellCopy/amount the expression was meant to
	// size). The SVar-indirection form above stays authoritative for names.
	if strings.HasPrefix(raw, "Count$") {
		return sign * EvalCount(h, c, raw), true
	}
	if strings.HasPrefix(raw, "Sacrificed$") {
		return sign * EvalCount(h, c, raw), true
	}
	if strings.HasPrefix(raw, "Remembered$") {
		// The doc above already listed this prefix; the read makes it real --
		// the corpus writes Remembered$Amount as a DIRECT parameter value on
		// the ImmediateTrigger family (TriggerAmount$ Remembered$Amount,
		// Forum Filibuster / Dain Ironfoot / Ratonhnhaké:ton; the /Op suffix
		// rides the same body, Diregraf Horde's /DivideEvenlyDown.2), not
		// behind an SVar name.
		return sign * EvalCount(h, c, raw), true
	}
	if strings.HasPrefix(raw, "TriggerCount$") || strings.HasPrefix(raw, "ReplaceCount$") {
		return sign * EvalCount(h, c, raw), true
	}
	// A <Ref>>Count$... indirection (Unbound Flourishing's Value$
	// TriggeredSpellAbility>Count$xPaid/Twice) is a count expression in its
	// own right, not an SVar name -- the SVar lookup above would otherwise
	// miss it and degrade the whole parameter to zero. The verdict rides
	// through from evalCountExprOK, so an unknown ref (the CastSA
	// adamant-gate family's shape) stays NOT evaluated rather than a silent
	// zero.
	if _, rest, found := strings.Cut(raw, ">"); found && strings.HasPrefix(strings.TrimSpace(rest), "Count$") {
		n, ok := evalCountExprOK(h, c, raw, 0)
		return sign * n, ok
	}
	if raw == "X" {
		return sign * c.X, true
	}
	return 0, false
}

// EvalCount evaluates a "Count$..." expression. The grammar in the corpus is a
// head, an optional space-separated argument, and an optional "/Op" suffix.
func EvalCount(h Host, c *Ctx, expr string) int32 {
	n, _ := evalCountExprOK(h, c, expr, 0)
	return n
}

// EvalCountOK is EvalCount plus a resolvability verdict: ok is false exactly
// when the expression's head matched NOTHING the evaluator models (the
// dispatch's fallthrough), so a caller can tell a legitimate zero (a modelled
// head that counted zero things) from "this body was never understood". The
// SVar-condition gates (effects.CheckSVarHolds) are the reason it exists: a
// gate over an unmodelled count head must fail OPEN (run anyway, the
// documented conditionMet convention) rather than enforce a meaningless zero,
// and only the dispatch itself knows which heads are modelled -- deriving the
// verdict here, at the dispatch, keeps it rot-proof: a head added to
// evalCountBody's switch automatically becomes evaluated, one deleted
// automatically stops being so.
func EvalCountOK(h Host, c *Ctx, expr string) (int32, bool) {
	return evalCountExprOK(h, c, expr, 0)
}

// maxCountDepth bounds the SVar recursion the Compare head introduces: a
// compared value or a branch may name another SVar, whose body may itself be
// a Count$Compare naming further SVars. The corpus chains are two deep
// (Nissa's Pilgrimage: X -> Y; The Biblioplex: X -> Y -> Z), so 8 is generous
// headroom against a self-referential or accidental-cycle SVar table, which
// would otherwise be the only unbounded recursion in this evaluator.
const maxCountDepth = 8

// evalCountExpr is EvalCount's body plus a recursion depth for the Compare
// head's SVar-name resolution; the public entry point always starts at 0.
func evalCountExpr(h Host, c *Ctx, expr string, depth int) int32 {
	n, _ := evalCountExprOK(h, c, expr, depth)
	return n
}

// evalCountExprOK is EvalCountOK's body plus the recursion depth; see the
// EvalCountOK doc for the verdict's meaning.
func evalCountExprOK(h Host, c *Ctx, expr string, depth int) (int32, bool) {
	if h == nil || c == nil {
		return 0, false
	}
	if depth > maxCountDepth {
		return 0, false
	}
	expr = strings.TrimSpace(expr)
	// A Remembered$... expression answers a question about the objects this
	// resolving spell/ability has remembered so far (Ctx.Remembered). It is
	// cut BEFORE evalRefProperty so its "Amount" head keeps answering
	// len(Ctx.Remembered) -- the resolution's own remembered set -- rather
	// than refTargets' Remembered read (rememberedWithSource), which unions
	// the source's persistent list and drops the ctx's own-source entry.
	// The one head this build models directly is Amount -- the number of
	// remembered objects, which is Swift Silence's "Draw a card for each
	// spell countered this way" (SVar:X:Remembered$Amount after effCounter's
	// RememberCountered$ True appended every countered spell); every other
	// property delegates to the shared <Ref>$<Property> family inside
	// evalRememberedOK. The /Op suffix is applied the same way Count$ applies
	// it. An unmodelled head degrades to zero.
	if body, ok := strings.CutPrefix(expr, "Remembered$"); ok {
		return evalRememberedOK(h, c, strings.TrimSpace(body))
	}
	// A <Ref>$<Property> body answers a numeric question about the objects a
	// target reference names: Targeted$CardPower (Vein Drinker's "deals
	// damage equal to its power", Kiku's Shadow), ParentTargeted$CardPower,
	// TriggeredCard$CardPower, and their Toughness/ManaCost/CardCounters/
	// Valid siblings -- heads that used to evaluate to zero and made exactly
	// the damage amounts they sized collapse. A ref or property outside the
	// modelled family returns false and falls through to the heads below
	// (evalRemembered still owns Remembered$Amount), so every shape that was
	// zero before stays zero.
	if n, ok := evalRefProperty(h, c, expr); ok {
		return n, true
	}
	// A <Ref>">Count$..."[/Op] indirection (Unbound Flourishing's Value$
	// TriggeredSpellAbility>Count$xPaid/Twice): the ref names the objects and
	// the right side is a Count$ expression evaluated against the FIRST
	// resolved object, bound as that object's own resolution context (Source,
	// Controller, X seeded from the object, SVars from its face) -- so
	// Count$xPaid answers the {X} the CAST paid, not the triggering
	// permanent's own. The ref switch is the same one evalRefProperty
	// dispatches through (refTargets), so the two cannot disagree; an unknown
	// ref fails closed to not-evaluated, exactly as evalRefProperty's default
	// does. The /Op suffix rides the ordinary Count$ read of the right side.
	if ref, right, found := strings.Cut(expr, ">"); found {
		if strings.HasPrefix(strings.TrimSpace(right), "Count$") {
			ts, ok := refTargets(h, c, strings.TrimSpace(ref))
			if !ok {
				return 0, false
			}
			g := h.Game()
			for _, t := range ts {
				if t.IsPlayer {
					continue
				}
				o := g.Obj(t.Obj)
				if o == nil {
					continue
				}
				sub := &Ctx{Source: o.ID, Controller: o.Controller, X: o.X}
				if f := o.Face(); f != nil {
					sub.SVars = f.SVars
				}
				return evalCountExprOK(h, sub, strings.TrimSpace(right), depth+1)
			}
			return 0, false
		}
	}
	// A Sacrificed$... expression answers "the sacrificed object's" head (CR
	// 608.2g last-known-information): power, toughness, mana value, or the
	// number of objects sacrificed. It reads the LKI snapshot captured at the
	// instant of the sacrifice (Ctx.Sacrificed), never the live object -- a
	// graveyard object has no layer-derived P/T and Move has reset its
	// counters. The /Op suffix (e.g. Sacrificed$Amount/Plus.1) is applied the
	// same way Count$ applies it.
	if body, ok := strings.CutPrefix(expr, "Sacrificed$"); ok {
		return evalSacrificedOK(c, strings.TrimSpace(body))
	}
	// A Remembered$... expression block was hoisted above evalRefProperty;
	// see its comment there for the ordering contract.
	// A TriggerCount$... expression answers a question about the event that
	// fired the trigger currently resolving -- "how much damage did that event
	// deal" (TriggerCount$DamageAmount), "how much life did it gain/lose"
	// (TriggerCount$LifeAmount), or the generic event magnitude
	// (TriggerCount$Amount). The answer comes from the triggering event's own
	// amount, captured by rules into Ctx.TriggerAmount when the trigger fired
	// and carried to resolution through the per-stack-instance
	// triggerContexts map -- never from the live board, and never re-inferred
	// at resolution. A head this build does not model (Result, ScryNum,
	// ScryBottom) degrades to zero, exactly as it did before TriggerCount$ was
	// recognised at all.
	if body, ok := strings.CutPrefix(expr, "TriggerCount$"); ok {
		return evalTriggerCountOK(c, strings.TrimSpace(body))
	}
	// A SVar$<name>[/Op] indirection resolves another SVar on the same face
	// and applies the suffix (Herald of War-adjacent shapes:
	// SVar:Z:SVar$Y/Times.2 chains two reductions' amounts). It also resolves
	// the RollDice publications (effects/dice.go's ResultSVar$ names -> the
	// die result/total/difference, plus the chosen/other and
	// MaxRolls/EvenResults counts): a roll's value lives in the resolution's
	// publication record, not in a static SVar table, so it is consulted
	// only when the name is not an SVar of this face. An unknown name
	// degrades to zero -- the conservative no-op every unmodelled head
	// applies. The /Op suffix is applied exactly as applyCountOp does.
	if rest, ok := strings.CutPrefix(expr, "SVar$"); ok {
		name, op, hasOp := strings.Cut(rest, "/")
		n, ok3 := int32(0), false
		if body, ok2 := c.SVars[strings.TrimSpace(name)]; ok2 {
			n, ok3 = evalCountExprOK(h, c, body, depth+1)
		} else if v, ok2 := rollPublished(c, strings.TrimSpace(name)); ok2 {
			n, ok3 = v, true
		}
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n, ok3
	}
	// ReplaceCount$ reads the event currently being replaced. Damage
	// replacement bodies use both the bare DamageAmount form (Vigor, Purity,
	// Hostility) and arithmetic suffixes (Fiery Emancipation, Angel of
	// Suffering). Rules carries the amount in Ctx so every supported body API,
	// not only ReplaceEffect itself, sees the same in-flight value.
	if body, ok := strings.CutPrefix(expr, "ReplaceCount$"); ok {
		field, op, hasOp := strings.Cut(strings.TrimSpace(body), "/")
		// "Number" is Forge's DrawCards-replacement spelling of the same
		// in-flight amount (Quantum Riddler's NumCards$
		// ReplaceCount$Number/Plus.1 body; 8 corpus files carry the field).
		if field != "DamageAmount" && field != "Amount" && field != "Number" {
			return 0, false
		}
		n := c.ReplacementAmount
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n, true
	}
	body, ok := strings.CutPrefix(expr, "Count$")
	if !ok {
		if n, err := strconv.Atoi(expr); err == nil {
			return int32(n), true
		}
		// Forge's PlayerCount SVar bodies omit the Count$ prefix
		// (SVar:OpponentSmallest:PlayerCountOpponents$LowestLifeTotal --
		// Vampire Lacerator's upkeep gate): run the head dispatch on the raw
		// body before giving up. An unrecognised bare word still falls
		// through, not evaluated.
		if n, ok2 := evalCountBody(h, c, strings.TrimSpace(expr), depth); ok2 {
			return n, true
		}
		// A bare SVar-name body (Spark Fiend's StoreSVar Expression$ Result)
		// resolves a DB$ RollDice publication of this same resolution -- the
		// same name the SVar$ indirection resolves above, in the one shape a
		// corpus body carries a bare runtime name. Anything else is
		// unrecognised: zero, and NOT evaluated.
		if v, ok := rollPublished(c, strings.TrimSpace(expr)); ok {
			return v, true
		}
		return 0, false
	}
	body, op, hasOp := strings.Cut(body, "/")
	n, ok2 := evalCountBody(h, c, strings.TrimSpace(body), depth)
	if hasOp {
		if clamped, isLimit := countColorsLimitMax(strings.TrimSpace(body), op, n); isLimit {
			n = clamped
		} else {
			n = applyCountOp(n, op)
		}
	}
	return n, ok2
}

// countColorsLimitMax answers whether op is a LimitMax.<n> clamp on a
// Count$Valid/ValidZone body whose property is Colors -- Colors's one corpus
// op suffix (happily_ever_after's Permanent.YouCtrl$Colors/LimitMax.5). It is
// called from evalCountExprOK's generic /Op site, which cuts the suffix off
// the whole body BEFORE the head dispatch, so the countZone branch never sees
// it; keeping the clamp here scopes the new op to Colors bodies only (the
// summed properties carry no op in the corpus and keep the plain
// applyCountOp read, where an unknown op name is ignored and the base value
// stands).
func countColorsLimitMax(body, op string, n int32) (int32, bool) {
	head, arg, _ := strings.Cut(body, " ")
	if _, ok := countZone(head); !ok {
		return n, false
	}
	_, prop, hasProp := strings.Cut(strings.TrimSpace(arg), "$")
	if !hasProp || strings.TrimSpace(prop) != "Colors" {
		return n, false
	}
	lim, ok := strings.CutPrefix(op, "LimitMax.")
	if !ok {
		return n, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(lim))
	if err != nil || v < 0 {
		return n, false
	}
	if n > int32(v) {
		return int32(v), true
	}
	return n, true
}

// evalTriggerCount resolves a "TriggerCount$<Head>[/Op]" body against the
// triggering event's magnitude (Ctx.TriggerAmount). The heads this build
// models -- DamageAmount (damage the event dealt), LifeAmount (life it
// gained/lost) and Amount (the generic event magnitude) -- all answer the
// same number, because each is the single amount the causing event carried;
// the distinction between them is only in which trigger mode populates it
// (and, for the corpus, that the LifeGained trigger mode is not yet
// registered, so a LifeAmount head is unreachable today). The /Op suffix is
// applied exactly as applyCountOp does for Count$ and Sacrificed$. An
// unmodelled head (Result, ScryNum, ScryBottom) degrades to zero.
func evalTriggerCountOK(c *Ctx, body string) (int32, bool) {
	body, op, hasOp := strings.Cut(body, "/")
	var n int32
	switch strings.TrimSpace(body) {
	case "DamageAmount", "LifeAmount", "Amount":
		n = c.TriggerAmount
	default:
		// Result (die-roll/dice), ScryNum and ScryBottom (scry events) are
		// heads whose triggering events this build does not raise, so they
		// stay zero -- the same conservative no-op as before the prefix was
		// recognised. NOT evaluated: a gate over one of these fails open.
		return 0, false
	}
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n, true
}

// evalSacrificed resolves a "Sacrificed$<Property>[/Op]" body against the
// LKI snapshots this resolving spell/ability captured when it sacrificed
// each object (Ctx.Sacrificed). Property is the corpus head after the
// Sacrificed$ prefix: CardPower, CardToughness, CardManaCost, or Amount (the
// count of objects sacrificed). For a numeric property over more than one
// sacrificed object the values are summed -- corpus Sac costs are almost
// always exactly one candidate, so summing agrees with the single-value
// answer and is the least surprising reading of "the sacrificed creature's
// power" when a shape somehow names several. An empty Sacrificed list
// (nothing captured) degrades to zero rather than panicking, preserving the
// "the card did nothing" totality convention of every other head here. The
// /Op suffix (Plus/Minus/Times./Twice/HalfDown/HalfUp/Negative) is applied
// after the base value, exactly as applyCountOp does for Count$.
func evalSacrificedOK(c *Ctx, body string) (int32, bool) {
	body, op, hasOp := strings.Cut(body, "/")
	var n int32
	switch strings.TrimSpace(body) {
	case "CardPower":
		n = sacrificedNumeric(c, func(s state.SacrificedInfo) int32 { return s.Power })
	case "CardToughness":
		n = sacrificedNumeric(c, func(s state.SacrificedInfo) int32 { return s.Toughness })
	case "CardManaCost":
		n = sacrificedNumeric(c, func(s state.SacrificedInfo) int32 { return s.ManaValue })
	case "Amount":
		n = int32(len(c.Sacrificed))
	default:
		// An out-of-scope head (Valid, CardTypes, ChromaSource, CardNumColors,
		// CardCounters) degrades to zero, exactly as before the fix -- the
		// conservative same-as-before no-op the brief scopes out. NOT
		// evaluated: a gate over one of these fails open.
		return 0, false
	}
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n, true
}

// sacrificedNumeric folds a numeric property across every object this
// spell/ability sacrificed, summing (see evalSacrificed's doc for why sum and
// not first).
func sacrificedNumeric(c *Ctx, f func(state.SacrificedInfo) int32) int32 {
	var n int32
	for _, s := range c.Sacrificed {
		n += f(s)
	}
	return n
}

func evalRememberedOK(h Host, c *Ctx, body string) (int32, bool) {
	body, op, hasOp := strings.Cut(body, "/")
	switch strings.TrimSpace(body) {
	case "Amount":
		n := int32(len(c.Remembered))
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n, true
	}
	// A Remembered$CardPower / CardToughness / CardManaCost / CardCounters.
	// / Valid body is the shared <Ref>$<Property> family (evalRefProperty);
	// evalCountExpr routes it here first only because the Remembered$
	// prefix cut wins. An unmodelled property still degrades to zero, the
	// same conservative no-op evalSacrificed's default takes, and the
	// property verdict rides through: an unmodelled one is NOT evaluated.
	if n, ok := evalRefProperty(h, c, "Remembered$"+body); ok {
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n, true
	}
	return 0, false
}

// refTargets resolves one ref name of the <Ref>$<Property> family into the
// targets it names. Shared by evalRefProperty and the <Ref>>Count$...>
// indirection branch in evalCountExprOK, so the two cannot disagree about
// which refs exist. An unknown ref returns false -- the caller fails closed,
// exactly as evalRefProperty's default always did.
func refTargets(h Host, c *Ctx, ref string) ([]state.Target, bool) {
	switch ref {
	case "Targeted", "ParentTarget", "ParentTargeted", "ThisTargetedCard", "AllTargeted":
		// AllTargeted (task alltargeted1) is Forge's UNION of every targeting
		// SA's targets down the root ability's sub-ability chain; the only
		// binding this engine carries is the resolving SA's own chosen
		// targets, so the faithful-as-available reading is Ctx.Targets -- the
		// same list "Targeted" names. A sub-targeting chain (Wayta, Trainer
		// Prodigy's fight) therefore still reads 0 here; recorded in
		// AGENTS.md's Known approximations.
		return c.Targets, true
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy",
		"TriggeredAttacker", "TriggeredAttackerLKICopy",
		"TriggeredBlocker", "TriggeredBlockerLKICopy",
		"TriggeredTargetLKICopy", "DelayTriggerRemembered",
		"DelayTriggerRememberedLKI", "RememberedLKI":
		return c.Remembered, true
	case "TriggeredSpellAbility":
		// The activation arm (abcopy1): the fire-time TriggerAbility role is
		// the exact referent (Remembered names the source permanent); the
		// spell-cast arm and hand-built contexts keep the Remembered entry.
		if c.TriggerAbility != 0 {
			return []state.Target{{Obj: c.TriggerAbility}}, true
		}
		return c.Remembered, true
	case "Remembered":
		// Forge's plain Remembered$ form reads the executing ability's shared
		// host-card remembered list: the ctx walk's set UNIONED with the
		// source's persistent event-backed list (rememberedWithSource).
		return rememberedWithSource(h, c), true
	case "ExiledWith":
		// The same defined-targets resolver case effects/context.go's
		// knownDefinedTargets carries, so a count body over the ref (the
		// CheckSVar gate SVar:X:ExiledWith$Amount of Colfenor's Urn,
		// Veteran Survivor and River Song's Diary) evaluates against the
		// same set the body's own Defined$ ExiledWith resolves. Without it
		// the ref fails closed and the gate is never evaluated, so those
		// triggers stay silent even when the exile count meets the gate.
		var out []state.Target
		g := h.Game()
		for _, id := range g.Zone(state.ZExile, c.Controller) {
			if o := g.Obj(id); o != nil && o.ExiledWith == c.Source {
				out = append(out, state.Target{Obj: id})
			}
		}
		return out, true
	default:
		return nil, false
	}
}

// evalRefProperty resolves one "<Ref>$<Property>[...][/Op]" count body over
// the objects a target reference names. Refs: Targeted/ParentTarget/
// ThisTargetedCard/AllTargeted name the resolving ability's chosen targets
// (AllTargeted is Forge's whole-chain union; see refTargets for the
// available-binding narrowing);
// TriggeredCard (and its LKI spellings) and TriggeredAttacker name the
// objects the firing trigger remembered; Remembered is the plain form. A
// property this build does not model (or a body with no $ at all -- every
// other head in this evaluator) returns false, and the caller degrades to
// zero exactly as before this evaluator existed.
//
// The per-object answers mirror evalCountBody's own source-anchored heads:
// CardPower/CardToughness read the face plus marked P1P1 counters
// ( battlefield layer output for a battlefield object; a graveyard object's
// face), CardManaCost the face's converted cost, CardCounters.<KIND> one
// counter kind, Valid the count of referenced objects matching a card spec
// (unknown predicates fail closed inside the matcher, so an unreadable
// filter counts zero, never everything). Several references sum -- Forge's
// Count$ reads the same way -- and the /Op suffix applies through
// applyCountOp like every other head.
func evalRefProperty(h Host, c *Ctx, expr string) (int32, bool) {
	ref, prop, found := strings.Cut(expr, "$")
	if !found || h == nil {
		return 0, false
	}
	prop, op, hasOp := strings.Cut(prop, "/")
	prop = strings.TrimSpace(prop)
	ts, ok := refTargets(h, c, ref)
	if !ok {
		return 0, false
	}
	g := h.Game()
	var n int32
	for _, t := range ts {
		if t.IsPlayer {
			continue
		}
		o := g.Obj(t.Obj)
		lki := c.LKI != nil && c.LKI.ID == t.Obj
		if lki {
			// A zone-change trigger must read the causing object's snapshot,
			// not the same id after Move has cleared its counters and removed
			// battlefield layers. triggerLKI carries this value through the
			// TriggerPush wrapper to both initial and resumed resolution.
			o = c.LKI
		}
		if o == nil {
			continue
		}
		f := o.Face()
		switch {
		case prop == "CardPower":
			if f != nil {
				if lki && c.LKIPTValid {
					n += c.LKIPower
				} else {
					n += refPower(h, o, lki)
				}
			}
		case prop == "CardToughness":
			if f != nil {
				if lki && c.LKIPTValid {
					n += c.LKIToughness
				} else {
					n += refToughness(h, o, lki)
				}
			}
		case prop == "CardManaCost":
			if f != nil {
				n += f.Cmc()
			}
		case strings.HasPrefix(prop, "CardCounters."):
			n += o.Counter(strings.TrimPrefix(prop, "CardCounters."))
		case prop == "Amount":
			// The count of referenced objects themselves (SVar:X:ExiledWith$Amount,
			// the same "how many" the Remembered$Amount head answers for the
			// Remembered ref). Players in the list do not count.
			n++
		case prop == "Valid" || strings.HasPrefix(prop, "Valid "):
			spec := strings.TrimSpace(strings.TrimPrefix(prop, "Valid"))
			if (lki && MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller))) ||
				(!lki && MatchesSpecCtx(g, spec, t.Obj, c.SpecContext(c.Controller))) {
				n++
			}
		case prop == "Converge":
			// CR 107.4f-family converge, the TRIGGER-relative spelling: the
			// distinct-colour spend count of the cast the firing trigger is
			// about (Magmablood Archaic's SVar:Y:TriggeredCard$Converge), not
			// the resolving ability's own cast the plain Count$Converge head
			// at evalCountBody's "Converge" case reads off c.Source. Same
			// provenance discipline as that head and as TriggerPaidX: the
			// value was stamped on the cast spell by payCast's trailing
			// FlagConverged CastInfo BEFORE the deferred SpellCast trigger
			// re-walk fired, so a replay derives the same number; when the
			// read object IS the triggering card the fire-time snapshot
			// TriggerConverge wins over the live field, because a spell
			// countered between trigger push and resolution has had its
			// stack->graveyard move clear ConvergeColours while the colours
			// were spent regardless (CR 601.2h: the payment is not undone).
			// A copy of the spell was never cast and reads 0.
			if c.TriggerCard != 0 && t.Obj == c.TriggerCard {
				n += c.TriggerConverge
			} else {
				n += o.ConvergeColours
			}
		default:
			return 0, false
		}
	}
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n, true
}

// refPower/refToughness use rules' derived characteristics while a referenced
// object is a battlefield permanent. A referred-to object that already left
// keeps the LKI-compatible printed-plus-counters fallback: no live layer
// applies in a graveyard, and asking Host for it would read a different state.
func refPower(h Host, o *state.Object, snapshot bool) int32 {
	if !snapshot && o.Zone == state.ZBattlefield {
		return h.Power(o.ID)
	}
	return int32(o.Face().Power()) + o.Counter("P1P1") - o.Counter("M1M1")
}

func refToughness(h Host, o *state.Object, snapshot bool) int32 {
	if !snapshot && o.Zone == state.ZBattlefield {
		return h.Toughness(o.ID)
	}
	return int32(o.Face().Toughness()) + o.Counter("P1P1") - o.Counter("M1M1")
}

// evalCountBody is the Count$ head dispatch; ok is false only at the
// fallthrough (the head matched nothing), never inside a modelled branch -- a
// modelled head that legitimately counts zero still counts as evaluated.
func evalCountBody(h Host, c *Ctx, body string, depth int) (int32, bool) {
	g := h.Game()
	head, arg, _ := strings.Cut(body, " ")
	arg = strings.TrimSpace(arg)

	switch head {
	case "Compare":
		return evalCompare(h, c, arg, depth), true
	case "xPaid":
		// CR 107.3i: the {X} paid for the resolving spell or ability. On a
		// TRIGGER of a permanent that was cast for {X} the ability object's
		// own X is zero (a trigger was never paid an X), so the paid value
		// is read off the source permanent, which CastInfo carried out of
		// the cast onto the battlefield object (Meathook Massacre II's
		// SVar:X:Count$xPaid driving "each player sacrifices X creatures").
		if c.X != 0 {
			return c.X, true
		}
		if o := g.Obj(c.Source); o != nil {
			return o.X, true
		}
		return 0, true
	case "ReplicatePaid":
		// CR 702.55a: the number of replicate payments the resolving spell's
		// cast made, carried by the pay-time CastInfo's FlagReplicated Amount
		// (rules/cast.go's replicateAsk and payCast). Read off the SOURCE --
		// the cast spell, the same provenance read xPaid makes -- so a replay
		// derives the same count; a copy of the spell was never cast and
		// reads 0.
		if o := g.Obj(c.Source); o != nil {
			return o.ReplicateTimes, true
		}
		return 0, true
	case "TimesKicked":
		// CR 702.43: the number of times the resolving spell's multikicker
		// cost was paid as it was cast, carried by the pay-time CastInfo's
		// FlagMultikicked Amount (rules/cast.go's multikickAsk and payCast).
		// The same provenance read ReplicatePaid makes: read off the SOURCE
		// (the cast spell on the stack; an ETB reader sees the PERMANENT it
		// became -- the stack->battlefield move preserves the field -- so a
		// replay derives the same count). A pending cast's count is seeded
		// into ctx.TimesKicked by targetBoundCtx when the spell's own
		// announcement ask reads a TimesKicked bound BEFORE payment has
		// stamped the object (Comet Storm's TargetMin/Max$ TargetsNum); a
		// COPY of the spell was never kicked and reads 0.
		if c.TimesKicked != 0 {
			return c.TimesKicked, true
		}
		if o := g.Obj(c.Source); o != nil {
			return o.TimesKicked, true
		}
		return 0, true
	case "Converge":
		// CR 107.4f-family converge: the number of DISTINCT colours (WUBRG)
		// of mana actually spent to cast the resolving spell, carried by the
		// pay-time CastInfo's FlagConverged Amount (rules/cast.go's
		// payManaCastSpent capture and payCast's trailing CastInfo). Same
		// provenance read ReplicatePaid makes -- the cast spell, and in the
		// K:etbCounter ETB replacement the same object after the
		// stack->battlefield move preserves it -- so a replay derives the
		// same count; a copy of the spell was never cast and reads 0.
		if o := g.Obj(c.Source); o != nil {
			return o.ConvergeColours, true
		}
		return 0, true
	case "CastTotalManaSpent":
		// CR 601.2h's payment: the TOTAL mana actually spent to cast the
		// resolving spell (the spent delta's pips summed over every slot),
		// carried by the pay-time CastInfo's FlagManaSpent Amount
		// (rules/cast.go's payCast capture -- the converge/replicate/
		// multikick pattern; faceWantsCastSpend is the heads-safety gate).
		// Same provenance read Converge makes -- the cast spell, and in the
		// K:etbCounter ETB replacement the same object after the
		// stack->battlefield move preserves it -- so a replay derives the
		// same number; a copy of the spell was never cast and a cheated-in
		// permanent reads 0. The ref-property readers of OTHER casts
		// (TriggeredCard$CastTotalManaSpent, evalRefProperty) stay on the
		// rv2b exotic-heads ledger -- they read a trigger context, not this
		// field.
		if o := g.Obj(c.Source); o != nil {
			return o.ManaSpent, true
		}
		return 0, true
	case "ChosenNumber":
		// The Effect's SetChosenNumber$ binding (state.ContinuousEffect.ChosenNumber,
		// threaded into Ctx by rules' replCtx for effect-created replacement
		// bodies, task wildgrowth1: torgal_a_fine_hound / communal_brewing /
		// wildgrowth_archaic's "enters with an additional +1/+1 counter for
		// each ..." body). Bound ONCE when the Effect was created, against the
		// trigger's own context, so the body reads the frozen number wherever
		// the entry lands. The VERDICT is the bound flag (Ctx.ChosenNumberBound,
		// set only by rules' seedEffectReplCtx on effect-created matches): an
		// unbound context is UNRESOLVED, so every EvalCountOK consumer keeps
		// its pre-wildgrowth fail direction for the Choose-event population
		// whose ChosenNumber lives on state.Object.ChosenNumber and never
		// reaches here -- CheckSVarHolds fails open, a numeric filter RHS
		// (void's cmcEQX through resolveNumericRHS) never matches -- instead
		// of enforcing a meaningless zero. A bound zero is a real binding and
		// evaluates (torgal with no Dogs/Wolves on the board).
		return c.ChosenNumber, c.ChosenNumberBound
	case "YourLifeTotal":
		if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
			return 0, true
		}
		return g.Players[c.Controller].Life, true
	case "PlayerCountPlayers":
		return int32(g.AliveCount()), true
	case "PlayerCountOpponents":
		return int32(g.AliveCount() - 1), true
	case "ThisTurnCast":
		// Task 17 (Storm): spells cast this turn by anyone, read off the
		// log via h.CastThisTurn() so a replay derives the same count. The
		// classic idiom is Count$ThisTurnCast/Minus1 (storm copies the spell
		// once per spell cast before it, i.e. everyone's casts minus itself).
		return int32(h.CastThisTurn()), true
	case "RememberedNumber":
		// Forge's Count$RememberedNumber is the executing ability's remembered
		// count -- the same list evalRememberedOK's Amount head reads. In this
		// build that is Ctx.Remembered; a caller that needs the list WITHOUT a
		// trigger's event capture (effImmediateTrigger's TriggerAmount$ read)
		// passes a ctx whose Remembered is already the capture-excluded set, so
		// this head needs no special case of its own. Five corpus
		// ImmediateTrigger lines and 38 files elsewhere carry it.
		return int32(len(c.Remembered)), true
	case "RememberedSize":
		// Forge's RememberedSize is the HOST CARD's remembered list -- the
		// persistent list riders (RememberDiscarded$/RememberCountered$/
		// RememberChosen$/RememberControlled$/RememberSacrificed$) add to and
		// Cleanup's ClearRemembered$ clears. In this engine that list is the
		// SOURCE object's event-backed Remembered; the ctx-level list also
		// carries a trigger's captured event object, which is NOT part of
		// Forge's host list (the same exclusion iterationBase applies). A
		// resolution with no source object falls back to the ctx list.
		if o := g.Obj(c.Source); o != nil {
			return int32(len(o.Remembered)), true
		}
		return int32(len(c.Remembered)), true
	case "LifeOppsLostThisTurn":
		// The total life the controller's OPPONENTS have lost this turn
		// (Rakdos, Lord of Riots). Each opponent's loss comes from the Host's
		// log-derived LifeLostThisTurn, so the count is replay-derivable.
		if c.Controller < 0 {
			return 0, true
		}
		var n int32
		for _, p := range g.AliveFrom(0) {
			if p != c.Controller {
				n += h.LifeLostThisTurn(p)
			}
		}
		return n, true
	case "LifeYouGainedThisTurn":
		// The total life the controller GAINED this turn — the CheckSVar$ gate
		// behind the "At the beginning of each end step, if you gained 4 or
		// more life this turn" family (Angelic Accord, Resplendent Angel,
		// Valkyrie Harbinger; 86 raw corpus Count$ lines). Folded from the log
		// through the Host's LifeGainedThisTurn like LifeOppsLostThisTurn, so
		// a replay derives the same count.
		if c.Controller < 0 {
			return 0, true
		}
		return h.LifeGainedThisTurn(c.Controller), true
	case "YourTurns":
		// How many of the game's turns have begun with the controller as the
		// active player, current turn included (Serra Avenger's "your first,
		// second, or third turns of the game"). Log-derived through the Host
		// like LifeOppsLostThisTurn, so a replay derives the same number.
		return h.TurnsTaken(c.Controller), true
	case "CardPower":
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			return refPower(h, o, false), true
		}
		return 0, true
	case "CardToughness":
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			return refToughness(h, o, false), true
		}
		return 0, true
	case "AttackersDeclared":
		// Count$AttackersDeclared: the attackers declared THIS turn — the Raid
		// family's "attacked this turn" read (Bloodsoaked Champion's
		// CheckSVar$ RaidTest activation gate plus 10 ConditionCheckSVar$
		// bodies). Folded from the event log through the Host (rules'
		// Engine.AttackersThisTurn) so a replay derives the identical number,
		// the same discipline CastThisTurn takes.
		return int32(h.AttackersThisTurn()), true
	case "ColorsColorIdentity":
		// Count$ColorsColorIdentity: the number of colours in the resolving
		// controller's commanders' colour identity (War Room's
		// "SVar:X:Count$ColorsColorIdentity" driving "{3}, {T}, Pay life equal
		// to the number of colors in your commanders' color identity: Draw a
		// card", the corpus's only carrier). Read through the Host's
		// CommanderIdentityColourCount like the other log/state-derived heads
		// (LifeLostThisTurn, TurnsTaken), so a replay derives the identical
		// count. An empty identity (no commander, or a colourless one) is a
		// real, resolvable 0 — the gate that withholds the ability outside the
		// Commander format is ActivationGameTypes$, not this count.
		if c.Controller < 0 {
			return 0, true
		}
		return int32(h.CommanderIdentityColourCount(c.Controller)), true
	}

	// PlayerCount<Players|Opponents|RegisteredOpponents>$<Property> — per-
	// group extreme properties. "Players" spans every living player,
	// "Opponents" every living player but the resolving controller, the same
	// groups the bare PlayerCountPlayers/PlayerCountOpponents heads count
	// (RegisteredOpponents is Forge's game-start opponent set, read here as
	// the same living-opponent group). Two property families are resolvable:
	// the life-total extremes (Vampire Lacerator's ConditionCheckSVar$
	// OpponentSmallest: PlayerCountOpponents$LowestLifeTotal, GE11 — "you
	// lose 1 life unless an opponent has 10 or less life") and the
	// count/counted-quantity extremes playerCountExtreme answers below. Any
	// other property is NOT resolvable: (0, false) — the same verdict
	// Count$Valid's UnknownPredicates takes — so a gate over one fails per
	// its caller's documented direction rather than enforcing a fake zero.
	// An empty group also fails unresolvable for a life extreme (lifeExtreme
	// reports no extreme), for the same reason: a threshold compared against
	// an absent extreme is not readable either.
	if rest, ok := strings.CutPrefix(head, "PlayerCountPlayers$"); ok {
		if n, ok2 := lifeExtreme(g, g.AliveFrom(0), rest); ok2 {
			return n, true
		}
		return playerCountExtreme(h, g, c, g.AliveFrom(0), rest, arg)
	}
	if rest, ok := strings.CutPrefix(head, "PlayerCountRegisteredOpponents$"); ok {
		// Forge's REGISTERED opponents — the opponents registered at game
		// start (Bloodchief Ascension's "if an opponent lost 2 or more life
		// this turn" gate). No registered-membership list survives a replay
		// here, so the group reads as the same living-opponent set
		// PlayerCountOpponents$ counts; the property dispatch below is shared.
		if n, ok2 := lifeExtreme(g, opponentGroup(g, c), rest); ok2 {
			return n, true
		}
		return playerCountExtreme(h, g, c, opponentGroup(g, c), rest, arg)
	}
	if rest, ok := strings.CutPrefix(head, "PlayerCountOpponents$"); ok {
		if n, ok2 := lifeExtreme(g, opponentGroup(g, c), rest); ok2 {
			return n, true
		}
		return playerCountExtreme(h, g, c, opponentGroup(g, c), rest, arg)
	}

	// PlayerCountPropertyYou$<Property> — the single resolvable member of
	// Forge's PlayerCountProperty<group>$<Property> family (86 raw corpus
	// files carry the family; the two HasPropertyActive files are Starting
	// Town and Hylda's Crown of Winter). HasPropertyActive reads 1 when the
	// RESOLVING controller is the active player, else 0 — Starting Town's
	// ETB gate reads SVar:Y:PlayerCountPropertyYou$HasPropertyActive and
	// feeds Count$Compare Y GE1.Z.4, so X is YourTurns on your turn and 4
	// off it, tapped only when X > 3. Every OTHER property on this group,
	// and every other group's property (a state qualifier this count path
	// carries no machinery to evaluate), reports (0, false) — the same
	// fail-closed unresolvable verdict the general PlayerCount dispatch
	// above documents, so a gate over one degrades per its caller's
	// documented direction rather than enforcing a fake zero.
	if rest, ok := strings.CutPrefix(head, "PlayerCountPropertyYou$"); ok {
		if strings.TrimSpace(rest) == "HasPropertyActive" {
			if c.Controller == g.Active {
				return 1, true
			}
			return 0, true
		}
		return 0, false
	}

	// ThisTurnCast_<spec> counts the spells cast this turn matching a Forge
	// spec (Count$ThisTurnCast_Card.YouCtrl — the "first/second spell you
	// cast" family): the caster scope is the controller when the spec carries
	// a You* qualifier, everyone otherwise. The spec's bare !CastSaSource
	// qualifier is Forge's "other than the spell being cast" device (every
	// bare-form carrier's oracle says other/another), so the count excludes
	// its own ctx source through the Host's Excluding read; the ARGUMENTED
	// forms (!CastSaSource$CardManaCost, !CastSaSource/Plus.2) stay in place
	// and keep failing closed downstream (no provenance grammar prices them).
	if rest, ok := strings.CutPrefix(head, "ThisTurnCast_"); ok {
		if stripped, selfExcl := stripBareCastSaSource(rest); selfExcl {
			return int32(h.SpellsCastThisTurnMatchingExcluding(c.Controller, stripped, c.Source)), true
		}
		// The ARGUMENTED forms (task castprov2) peel the token and reuse the
		// same Excluding read:
		//
		//   - !CastSaSource/<op> (thunder_salvo's /Plus.2): this form never
		//     reaches this arm — evalCountExprOK's GENERIC /Op peel cuts the
		//     body at the first "/" before the head parse, leaving the bare
		//     !CastSaSource for the bare arm above and handing the op to the
		//     ordinary applyCountOp — which is exactly the oracle's reading
		//     (the exclusion count, then Plus.2). Pinned by
		//     TestThunderSalvoXIsTwoPlusOtherSpellsCast.
		//   - !CastSaSource$<Property> (call_forth_the_tempest's
		//     $CardManaCost): the matching casts' objects, the property
		//     AGGREGATED over them instead of counting 1 each (the zone-count
		//     heads' `$Property` precedent). An unknown property fails closed
		//     to (0, false), the unresolvable verdict.
		if stripped, prop, ok2 := stripCastSaSourceAggregate(rest); ok2 {
			return aggregateCastProperty(h, h.EachSpellCastThisTurnMatching(c.Controller, stripped, c.Source), prop)
		}
		return int32(h.SpellsCastThisTurnMatching(c.Controller, rest)), true
	}

	// StartingPlayer.<yes>.<no> is Forge's two-branch opening designation
	// count. Desert Cenote's StartingPlayer.0.1 feeds LT1, so only the
	// starting player gets its tapped-entry replacement; the other corpus
	// cards use different numeric branches. Parse the grammar rather than a
	// card-specific literal so every branch pair follows the current, replayed
	// designation (including an opening effect that changes it).
	if branches, ok := strings.CutPrefix(head, "StartingPlayer."); ok {
		yes, no := splitDot(branches)
		if g.IsStartingPlayer(c.Controller) {
			return yes, true
		}
		return no, true
	}

	// CardCounters.<KIND> counts a counter kind on the source.
	if kind, ok := strings.CutPrefix(head, "CardCounters."); ok {
		if o := g.Obj(c.Source); o != nil {
			return o.Counter(kind), true
		}
		return 0, true
	}
	// Kicked.<yes>.<no> is <yes> when the source was kicked, else <no>.
	if rest, ok := strings.CutPrefix(head, "Kicked."); ok {
		yes, no := splitDot(rest)
		if o := g.Obj(c.Source); o != nil && o.CastFlags&state.FlagKicked != 0 {
			return yes, true
		}
		return no, true
	}
	// Foretold.<ifTrue>.<ifFalse> is <ifTrue> when the resolving source was
	// cast foretold (CR 702.126a -- the pay-time FlagForetold provenance,
	// the same read Kicked makes), else <ifFalse>. The operands resolve
	// through the same operand machinery evalCompare's branches use (a
	// literal, or an SVar name resolved recursively -- Starnheim Unleashed's
	// Count$Foretold.X.1 reads the announced X through the face's SVar
	// table), with the same depth discipline. A carrier missing a branch is
	// a corpus bug: fail closed (0, false) rather than answer a half body.
	if rest, ok := strings.CutPrefix(head, "Foretold."); ok {
		yes, no, found := strings.Cut(rest, ".")
		if !found || strings.TrimSpace(yes) == "" || strings.TrimSpace(no) == "" {
			return 0, false
		}
		foretold := false
		if o := g.Obj(c.Source); o != nil {
			foretold = o.CastFlags&state.FlagForetold != 0
		}
		if foretold {
			return evalCountOperand(h, c, yes, depth), true
		}
		return evalCountOperand(h, c, no, depth), true
	}
	// NotedNumber is the number a trigger's Execute$ body last noted onto
	// the source card (DB$ Pump | NoteNumber$ <expr> -- Lupine Harbingers'
	// exile trigger noting Count$YourTurns). Read off the card the ETB
	// replacement resolves over (c.Source), the same object events.NotedNumber
	// wrote; a card with no note reads 0.
	if head == "NotedNumber" {
		if o := g.Obj(c.Source); o != nil {
			return o.NotedNumber, true
		}
		return 0, true
	}
	// UrzaLands.<assembled>.<not assembled> is <assembled> when the controller
	// controls at least one of each Urza land subtype on the battlefield
	// (Urza's Mine, Urza's Tower, Urza's Power-Plant), else <not assembled>.
	// The subtypes are matched on the Types line, where Power-Plant is
	// hyphenated -- the card Name's "Urza's Power Plant" is a different
	// string and matching it would be exactly the defect this head fixes.
	if rest, ok := strings.CutPrefix(head, "UrzaLands."); ok {
		assembled, notAssembled := splitDot(rest)
		if controlsAllUrzaLands(g, c.Controller) {
			return assembled, true
		}
		return notAssembled, true
	}

	// Count$ThisTurnEntered_<Dest>[_from_<Origin>]_<Valid> counts the cards
	// ADDED to zone <Dest> this turn (optionally only those that came from
	// <Origin>) matching <Valid> -- Forge's CardUtil.getThisTurnEntered over
	// the per-zone getCardsAddedThisTurn lists. The list is state.Entered,
	// appended once per move by events.Move and cleared at TurnChange, so a
	// replay folds the identical count. A card listed twice (it entered the
	// zone twice) counts twice, exactly like Forge's per-add list; a card
	// that has since moved on is still listed and its validity is evaluated
	// against the object wherever it now lives -- the same live-card read
	// Forge's getValidCards applies to the zone list. Gravelighter's "draw a
	// card if a creature died this turn" is the corpus carrier.
	if rest, ok := strings.CutPrefix(head, "ThisTurnEntered"); ok && strings.HasPrefix(rest, "_") {
		return evalThisTurnEntered(g, c, rest[1:])
	}

	// Count$<Predicate>.<yes>.<no> — Forge's yes/no branch heads: the value
	// is the first number when the predicate holds, the second when it does
	// not (Count$Morbid.1.0 ×33 and Count$Monarch.1.0 ×10 are the corpus's
	// dominant spellings). wasCastFromGraveyard is modelled below — the
	// resolving source's graveyard-origin cast bits (the Increasing cycle's
	// Count$wasCastFromGraveyard.10.5, 11 corpus lines); the remaining
	// exotic predicates — Delirium, Blessing, Void, Adamant_<n>.<colour> —
	// stay unmodelled and degrade to zero. Morbid is
	// CR 702.53's "a creature died this turn": a creature entered a graveyard
	// FROM THE BATTLEFIELD this turn, folded off the same state.Entered list
	// ThisTurnEntered_ reads (a battlefield→graveyard MoveZone is exactly a
	// death, sacrifice included), so a replay derives the identical answer.
	// Monarch is the resolving controller's current designation (the same
	// state g.IsMonarch answers for a CheckDefinedPlayer$ .isMonarch spec).
	if dot := strings.IndexByte(head, '.'); dot > 0 {
		switch head[:dot] {
		case "wasCastFromGraveyard":
			// The resolving source was CAST FROM A GRAVEYARD (CR 601.2b's
			// alternative-cost provenance): any graveyard-origin cast bit —
			// FlagFlashback, FlagHarmonize or FlagEscaped — holds it. This is
			// the same bit test the Card.wasCastFromGraveyard filter predicate
			// and its compiled twin share (effects/filter.go,
			// effects/compiled_predicate.go); a nil/missing source reads
			// false, and so does a stack copy (IsCopy — a copy was never cast,
			// even though StackCopy preserves the original's flags). The
			// branch tokens resolve through resolveCountOperand, not splitDot:
			// the_final_days' YES branch is the SVar X
			// (Count$wasCastFromGraveyard.X.2, X = Count$ValidGraveyard
			// Creature.YouCtrl), the Compare head's evalCountOperand recursion
			// precedent; an unresolvable token degrades to 0, never wedges.
			yesTok, noTok, _ := strings.Cut(head[dot+1:], ".")
			holds := false
			if o := g.Obj(c.Source); o != nil && !o.IsCopy {
				holds = o.CastFlags&(state.FlagFlashback|state.FlagHarmonize|state.FlagEscaped) != 0
			}
			if holds {
				y, ok := resolveCountOperand(h, c, yesTok, depth)
				if !ok {
					y = 0
				}
				return y, true
			}
			n, ok := resolveCountOperand(h, c, noTok, depth)
			if !ok {
				n = 0
			}
			return n, true
		case "wasCastFromYourHandByYou":
			// The resolving source was cast from ITS OWN CONTROLLER's hand by
			// that controller (the Myojin cycle's etbCounter CheckSVar$ gate:
			// "enters with a divinity counter on it if you cast it from your
			// hand", 12 corpus carriers). An ordinary hand-origin cast carries
			// no CastFlags bit — the flags mark alternative costs and origins
			// only — so the provenance is the object's latest PutOnStack
			// (Host.WasCastFromHandByYou's log scan, replay-derivable like
			// CastThisTurn); a copy was never cast, and a card never put on
			// the stack (cheated into play) reads false, the same guards the
			// wasCastFromGraveyard case takes. The branch tokens resolve
			// through resolveCountOperand, the same machinery.
			yesTok, noTok, _ := strings.Cut(head[dot+1:], ".")
			holds := false
			if o := g.Obj(c.Source); o != nil && !o.IsCopy {
				if h != nil {
					holds = h.WasCastFromHandByYou(c.Source, o.Controller)
				}
			}
			if holds {
				y, ok := resolveCountOperand(h, c, yesTok, depth)
				if !ok {
					y = 0
				}
				return y, true
			}
			n2, ok := resolveCountOperand(h, c, noTok, depth)
			if !ok {
				n2 = 0
			}
			return n2, true
		case "wasCastFromYourHand":
			// The BARE (no "ByYou") hand-provenance branch head (task
			// castprov3, see_the_truth's SVar:X:Count$wasCastFromYourHand.1.3 —
			// "put one of those cards into your hand ... If this spell was cast
			// from anywhere other than your hand, put each of those cards into
			// your hand instead"): the resolving source's latest cast came from
			// a hand — ANY caster's hand, the player scoping the ByYou twin
			// carries being absent here. The same guards the ByYou case takes:
			// the provenance is the object's latest PutOnStack
			// (Host.WasCastFromHand's log scan, replay-derivable), a copy was
			// never cast, a card never put on the stack (cheated into play)
			// reads false. Branch tokens through resolveCountOperand, the same
			// machinery.
			yesTok, noTok, _ := strings.Cut(head[dot+1:], ".")
			holds := false
			if o := g.Obj(c.Source); o != nil && !o.IsCopy {
				holds = h.WasCastFromHand(c.Source)
			}
			if holds {
				y, ok := resolveCountOperand(h, c, yesTok, depth)
				if !ok {
					y = 0
				}
				return y, true
			}
			n3, ok := resolveCountOperand(h, c, noTok, depth)
			if !ok {
				n3 = 0
			}
			return n3, true
		case "IfCastInOwnMainPhase", "InOwnMainPhase":
			// CR "if you cast this spell during your main phase": the
			// yes/no branch head Forge's AbilityUtils reads as
			// Count$IfCastInOwnMainPhase.<numMain>.<numNotMain> (7 corpus
			// carriers: Return to Dust's TargetMax$ X, Might of Old
			// Krosa's NumAtt$/NumDef$, Haunting Hymn's and Careful
			// Consideration's NumCards$, Sulfurous Blast's and Summary
			// Judgment's NumDmg$). The reading is LIVE, matching
			// Forge's game.getPhaseHandler(): the current step must be a
			// main phase and the active player the resolving controller
			// -- NOT a stamp of the cast's phase, which would diverge
			// from Forge (a spell cast in a main phase but resolved in
			// another reads the resolution phase). The two spellings
			// differ only in the third conjunct: IfCastInOwnMainPhase
			// additionally requires the source to have been CAST (Forge
			// c.wasCast(), the Host.WasCast read -- the pending CR 601.2c
			// announcement ask counts, since Forge sets castFrom before
			// setupTargets), while bare InOwnMainPhase (Dose of Dawnglow's
			// Count$InOwnMainPhase.0.1 blight gate) skips that conjunct.
			// Branch tokens resolve through resolveCountOperand, the
			// sibling cases' machinery; a missing/invalid token degrades
			// to 0, never wedges.
			yesTok, noTok, _ := strings.Cut(head[dot+1:], ".")
			holds := g.Step.IsMain() && g.Active == c.Controller
			if holds && head[:dot] == "IfCastInOwnMainPhase" {
				// A copy was never cast (the sibling provenance cases' IsCopy
				// guard); an absent source reads false too. The rules-side
				// WasCast applies the same IsCopy guard, but the count head is
				// reachable with a synthetic Host, so the guard lives here.
				holds = false
				if o := g.Obj(c.Source); o != nil && !o.IsCopy {
					holds = h != nil && h.WasCast(c.Source)
				}
			}
			if holds {
				y, ok := resolveCountOperand(h, c, yesTok, depth)
				if !ok {
					y = 0
				}
				return y, true
			}
			n, ok := resolveCountOperand(h, c, noTok, depth)
			if !ok {
				n = 0
			}
			return n, true
		case "Morbid", "Monarch":
			y, n := splitDot(head[dot+1:])
			holds := false
			if head[:dot] == "Monarch" {
				holds = g.IsMonarch(c.Controller)
			} else {
				for _, en := range g.Entered {
					if en.To != state.ZGraveyard || en.From != state.ZBattlefield {
						continue
					}
					if o := g.Obj(en.Obj); o != nil && hasType(o, "Creature") {
						holds = true
						break
					}
				}
			}
			if holds {
				return y, true
			}
			return n, true
		case "Revolt":
			// CR 702.38's branch head (the corpus's two carriers: Lifecraft
			// Cavalry's SVar:Revolt:Count$Revolt.1.0 etbCounter gate and
			// Fatal Push's Count$Revolt.4.2 destroy bound): <yes> when a
			// permanent the resolving CONTROLLER controlled left the
			// battlefield this turn, else <no> -- the same Host predicate
			// the bare Condition$ Revolt gate and the rules-side Revolt$
			// clauses share, so the spellings cannot drift apart. Literal
			// branches, the Morbid/Monarch precedent.
			y, n := splitDot(head[dot+1:])
			if h.RevoltHolds(c.Controller) {
				return y, true
			}
			return n, true
		}
	}

	// Valid / ValidZone forms count objects in a zone matching a filter.
	// A `$<Property>` suffix sums that numeric property over the matches
	// instead of counting them -- Mosswort Bridge's gate
	// `Count$Valid Creature.YouCtrl$CardPower` ("creatures you control have
	// total power 10 or greater") is the corpus shape (62 raw lines over 61
	// files: CardPower 42, CardManaCost 13, CardToughness 5). CardTypes and
	// Colors are DISTINCT-set counts over the same matches, not sums:
	// Colors counts the distinct colours among the matched permanents
	// ("the number of colors among permanents you control", Shimmercreep's
	// Vivid et al., 31 raw corpus lines, bounded by five), read through
	// ColorMaskOf so an explicit Colors: line and Devoid's colourless
	// treatment agree with the colour predicates; its single corpus op
	// suffix /LimitMax.<n> (happily_ever_after) clamps the result. An
	// unrecognised property keeps the whole token as the spec -- the
	// pre-existing fail-closed behaviour, since such a token never matched
	// anyway -- and the Greatest/Least/Different variants are still out of
	// scope here. Colors's one corpus op suffix /LimitMax.<n> is honoured at
	// evalCountExprOK's generic /Op site (countColorsLimitMax), scoped to
	// Colors bodies.
	if zone, ok := countZone(head); ok {
		spec, prop, hasProp := strings.Cut(arg, "$")
		if !hasProp {
			spec, prop = arg, ""
		} else {
			prop = strings.TrimSpace(prop)
			switch prop {
			case "CardPower", "CardToughness", "CardManaCost", "CardTypes", "Colors":
			default:
				// Not a recognised property (GreatestCardPower,
				// DifferentNames, Least*, ...): keep the old whole-token
				// spec read.
				spec, prop = arg, ""
			}
		}
		// CardTypes is Tarmogoyf's distinct-card-type form, not a filter:
		// count each real card type (CR 205.1) represented among the
		// selected cards once. The map is read only through len, so its
		// iteration order never reaches an event or a view.
		var seenCardTypes map[string]bool
		if prop == "CardTypes" {
			seenCardTypes = make(map[string]bool)
		}
		// The bare wasCastFromYourHand qualifier (task castprov3, Approach of
		// the Second Sun's Count$ValidStack Card.wasCastFromYourHand+Self):
		// not a filter predicate — split out per CANDIDATE object through the
		// Host's log read before the ordinary match (the ByYou family never
		// needed this here because it had no Valid* carrier; a ByYou spec
		// still routes to its own helper's absence and fails closed as
		// before, unchanged).
		hasBareHand := !strings.Contains(spec, "wasCastFromYourHandByYou") && strings.Contains(spec, "wasCastFromYourHand")
		// token$DifferentCardNames (Sandsteppe War Riders' "bolster X, where X
		// is the number of differently named artifact tokens you control";
		// also Gimbal Gremlin Prodigy, Audience with Trostani, Neriv Crackling
		// Vanguard -- 4 raw corpus lines): a SET-level qualifier the
		// per-object filter cannot express -- the count is the number of
		// DISTINCT face names among the matching tokens, not the number of
		// tokens. Stripped here and rewritten to the plain `token` predicate
		// for the per-object match; the distinctness is a seen-names set at
		// this count site (the CardTypes/Colors distinct-count precedent).
		// The filter's own read (matchPositive's token$DifferentCardNames
		// case) is the per-object half -- "is a token" -- so a non-count read
		// of the qualifier admits every matching token and narrows nothing.
		var seenTokenNames map[string]bool
		if strings.Contains(spec, "token$DifferentCardNames") {
			spec = strings.ReplaceAll(spec, "token$DifferentCardNames", "token")
			seenTokenNames = make(map[string]bool)
		}
		// Colors folds each match's colour mask; read only through a
		// popcount at the end, so no per-colour ordering ever reaches an
		// event or a view.
		var colorsSeen ColorMask
		var n int32
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(zone, p) {
				matchSpec := spec
				if hasBareHand {
					s, ok := castFromHandAnyAdmitsFilter(h, spec, id)
					if !ok {
						continue
					}
					matchSpec = s
				}
				if !matchesZoneSpecCtx(g, matchSpec, id, c.SpecContext(c.Controller), zone) {
					continue
				}
				if prop == "" {
					if seenTokenNames != nil {
						if o := g.Obj(id); o != nil && o.Face() != nil {
							seenTokenNames[o.Face().Name] = true
						}
						continue
					}
					n++
					continue
				}
				o := g.Obj(id)
				if o == nil || o.Face() == nil {
					continue
				}
				switch prop {
				case "CardPower":
					n += int32(o.Face().Power()) + o.Counter("P1P1")
				case "CardToughness":
					n += int32(o.Face().Toughness()) + o.Counter("P1P1")
				case "CardManaCost":
					n += o.Face().Cmc()
				case "CardTypes":
					for _, typ := range o.Face().Types {
						if cardTypeWords[typ] {
							seenCardTypes[typ] = true
						}
					}
				case "Colors":
					colorsSeen |= ColorMaskOf(o)
				}
			}
		}
		if prop == "CardTypes" {
			return int32(len(seenCardTypes)), true
		}
		if seenTokenNames != nil {
			return int32(len(seenTokenNames)), true
		}
		if prop == "Colors" {
			n = int32(bits.OnesCount8(uint8(colorsSeen)))
		}
		return n, true
	}
	return 0, false
}

// countColorsLimitMax answers whether op is a LimitMax.<n> clamp on a
// Count$Valid/ValidZone body whose property is Colors -- Colors's one corpus
// op suffix (happily_ever_after's Permanent.YouCtrl$Colors/LimitMax.5). It is
// called from evalCountExprOK's generic /Op site, which cuts the suffix off
// the whole body BEFORE the head dispatch, so the countZone branch never sees
// it; keeping the clamp here scopes the new op to Colors bodies only (the
// summed properties carry no op in the corpus and keep the plain
// applyCountOp read, where an unknown op name is ignored and the base value
// stands).

// evalThisTurnEntered parses a ThisTurnEntered_<Dest>[_from_<Origin>]_<Valid>
// tail -- the split Forge's own parser applies (workingCopy[0] = the head, so
// parts[0] here is <Dest>; at most five underscore parts, and a <Valid> tail
// of more than one token is rejoined). An unknown destination zone or an
// empty valid fails closed to zero rather than counting everything.
func evalThisTurnEntered(g *state.Game, c *Ctx, rest string) (int32, bool) {
	parts := strings.Split(strings.TrimSpace(rest), "_")
	if len(parts) < 2 || len(parts) > 5 {
		return 0, false
	}
	dest, ok := zoneWords[parts[0]]
	if !ok {
		return 0, false
	}
	hasFrom := len(parts) >= 3 && parts[1] == "from"
	valid := ""
	if hasFrom {
		if len(parts) < 4 {
			return 0, false
		}
		origin, known := zoneWords[parts[2]]
		if !known {
			return 0, false
		}
		valid = strings.Join(parts[3:], "_")
		return countEntered(g, c, dest, &origin, valid)
	}
	valid = strings.Join(parts[1:], "_")
	return countEntered(g, c, dest, nil, valid)
}

// countEntered folds the per-add entry list over one destination zone (and
// optionally one origin zone), counting the entries whose object matches
// valid from the resolving controller's perspective.
func countEntered(g *state.Game, c *Ctx, dest state.Zone, origin *state.Zone, valid string) (int32, bool) {
	if valid == "" {
		return 0, false
	}
	var n int32
	for _, e := range g.Entered {
		if e.To != dest {
			continue
		}
		if origin != nil && e.From != *origin {
			continue
		}
		if MatchesSpecCtx(g, valid, e.Obj, c.SpecContext(c.Controller)) {
			n++
		}
	}
	return n, true
}

// objectProperty reads one Count$Valid-spec "$Property" aggregate term over
// a single object: its face value plus +1/+1 counters for power/toughness,
// the mana value for CardManaCost. An unknown property reads 0 — the same
// conservative no-op every unmodelled head here takes.
func objectProperty(g *state.Game, id state.ObjID, prop string) int32 {
	o := g.Obj(id)
	if o == nil || o.Face() == nil {
		return 0
	}
	switch strings.TrimSpace(prop) {
	case "CardPower":
		return int32(o.Face().Power()) + o.Counter("P1P1")
	case "CardToughness":
		return int32(o.Face().Toughness()) + o.Counter("P1P1")
	case "CardManaCost":
		return o.Face().ManaValue()
	}
	return 0
}

// modeledProperty reports whether a Count$Valid-spec "$Property" aggregate
// term is one objectProperty can evaluate; an unknown property read 0 — the
// same conservative no-op every unmodelled head here takes — and a gate over
// one must fail open rather than enforce that zero.
func modeledProperty(prop string) bool {
	switch strings.TrimSpace(prop) {
	case "CardPower", "CardToughness", "CardManaCost":
		return true
	}
	return false
}

// lifeExtreme answers PlayerCount...$LowestLifeTotal / $HighestLifeTotal:
// the lowest/highest CURRENT life total among the given players. Anything
// else — another property, or an empty group (no extreme exists) — reports
// (0, false): the caller degrades to unresolvable, so a gate over the shape
// fails open rather than enforcing a fake zero.
func lifeExtreme(g *state.Game, players []state.PlayerID, prop string) (int32, bool) {
	prop = strings.TrimSpace(prop)
	if prop != "LowestLifeTotal" && prop != "HighestLifeTotal" {
		return 0, false
	}
	best := int32(0)
	seen := false
	for _, p := range players {
		if int(p) < 0 || int(p) >= len(g.Players) {
			continue
		}
		life := g.Players[p].Life
		if !seen || (prop == "LowestLifeTotal" && life < best) || (prop == "HighestLifeTotal" && life > best) {
			best, seen = life, true
		}
	}
	if !seen {
		return 0, false
	}
	return best, true
}

// opponentGroup returns the living players other than the resolving
// controller — the group PlayerCountOpponents$ and (by the reading
// documented at its dispatch site) PlayerCountRegisteredOpponents$ both
// count over.
func opponentGroup(g *state.Game, c *Ctx) []state.PlayerID {
	var opps []state.PlayerID
	for _, p := range g.AliveFrom(0) {
		if p != c.Controller {
			opps = append(opps, p)
		}
	}
	return opps
}

// playerCountExtreme answers the PlayerCount<group>$<Property> properties
// that are not a life total (lifeExtreme's pair is tried first at the
// dispatch site). Two families, the corpus's measured population over these
// heads:
//
//   - HighestValid/LowestValid <spec> and the zone-scoped spellings
//     (HighestValidGraveyard, LowestValidHand, ...): the highest/lowest,
//     over the group, of the count of objects the member has in the named
//     zone that match the spec, each member counted from their OWN
//     perspective — a YouCtrl qualifier in the spec names the counted
//     member, not the resolving controller (Land Tax's "if an opponent
//     controls more lands than you": SVar Y =
//     PlayerCountOpponents$HighestValid Land.YouCtrl, SVarCompare$ GTX;
//     Defense of the Heart's PlayerCountOpponents$HighestValid
//     Creature.YouCtrl, GE3). An empty zone or an all-zero group is a
//     readable extreme of 0, never unresolvable — a count always has one
//     answer.
//   - HighestLifeLostThisTurn/LowestLifeLostThisTurn: the extreme, over the
//     group, of the Host's log-derived LifeLostThisTurn (Bloodchief
//     Ascension's "if an opponent lost 2 or more life this turn" gate).
//
// An unknown property reports (0, false): unresolvable, so a gate over one
// fails per its caller's documented direction rather than enforcing a fake
// zero. Iterating the given group slice in order (never a map) keeps the
// extreme deterministic; ties match, like lifeExtreme's.
func playerCountExtreme(h Host, g *state.Game, c *Ctx, players []state.PlayerID, prop, spec string) (int32, bool) {
	prop = strings.TrimSpace(prop)
	highest := false
	switch {
	case strings.HasPrefix(prop, "Highest"):
		highest = true
		prop = prop[len("Highest"):]
	case strings.HasPrefix(prop, "Lowest"):
		highest = false
		prop = prop[len("Lowest"):]
	default:
		return 0, false
	}
	if prop == "LifeLostThisTurn" {
		if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
			return 0, false
		}
		best, seen := int32(0), false
		for _, p := range players {
			v := h.LifeLostThisTurn(p)
			if !seen || (highest && v > best) || (!highest && v < best) {
				best, seen = v, true
			}
		}
		if !seen {
			return 0, false
		}
		return best, true
	}
	zone, ok := countZone(prop)
	if !ok || strings.TrimSpace(spec) == "" {
		return 0, false
	}
	best, seen := int32(0), false
	for _, p := range players {
		if int(p) < 0 || int(p) >= len(g.Players) {
			continue
		}
		var n int32
		for _, id := range g.Zone(zone, p) {
			if matchesZoneSpecCtx(g, spec, id, c.SpecContext(p), zone) {
				n++
			}
		}
		if !seen || (highest && n > best) || (!highest && n < best) {
			best, seen = n, true
		}
	}
	if !seen {
		return 0, false
	}
	return best, true
}

// evalCompare resolves a "Compare <Name> <OP><threshold>.<ifTrue>.<ifFalse>"
// body -- the spell-mastery / lieutenant form Forge encodes as
// `SVar:X:Count$Compare Y GE2.3.2` (Nissa's Pilgrimage: 2 basic Forests, or 3
// with two or more instants/sorceries in the graveyard). <Name> resolves the
// compared value: an SVar body evaluated recursively (Y's
// `Count$ValidGraveyard Instant.YouOwn,Sorcery.YouOwn`), else the token itself
// as an inline expression. <OP> is one of GE/GT/EQ/LE/LT and the threshold a
// plain integer; the branches are each an integer literal or an SVar name
// resolved the same way as <Name>, which closes the corpus's SVar-named
// branch singletons (`GE4.X.4`, `LT5.X.Z`, `GE1.Y.Z`, ...) for free. The
// no-parseable-threshold singletons (`GEMePlus.3.2`, `LTZ.2.0`) and the
// argument-less forms (`Count$Compare TronCheck`, ...) fail closed to zero
// exactly as before -- no semantics are invented for them.
func evalCompare(h Host, c *Ctx, arg string, depth int) int32 {
	name, rest, _ := strings.Cut(arg, " ")
	rest = strings.TrimSpace(rest)
	if name == "" || rest == "" {
		// Argument-less shapes (`Count$Compare TronCheck`) have nothing to
		// compare; degrade to zero rather than guessing.
		return 0
	}
	if len(rest) < 2 {
		return 0
	}
	op, tail := rest[:2], rest[2:]
	thTok, branches, _ := strings.Cut(tail, ".")
	th, err := strconv.Atoi(thTok)
	if err != nil {
		// GEMePlus.3.2 / LTZ.2.0: the threshold names an expression, not a
		// literal. Out of scope -- fail closed to zero, same as before.
		return 0
	}
	ifTok, elseTok, _ := strings.Cut(branches, ".")
	value := evalCountOperand(h, c, name, depth)
	var hit bool
	switch op {
	case "GE":
		hit = value >= int32(th)
	case "GT":
		hit = value > int32(th)
	case "EQ":
		hit = value == int32(th)
	case "LE":
		hit = value <= int32(th)
	case "LT":
		hit = value < int32(th)
	default:
		// Not one of the five comparison heads.
		return 0
	}
	if hit {
		return evalCountOperand(h, c, ifTok, depth)
	}
	return evalCountOperand(h, c, elseTok, depth)
}

// evalCountOperand resolves one Compare operand: an integer literal directly,
// an SVar body through the ordinary expression evaluator, else the token
// itself as an inline expression (EvalCount degrades a bare unknown word to
// zero, the convention every other head follows).
func evalCountOperand(h Host, c *Ctx, tok string, depth int) int32 {
	n, _ := resolveCountOperand(h, c, tok, depth)
	return n
}

// resolveCountOperand is evalCountOperand with an evaluated verdict: an
// integer literal directly, an SVar body through the ordinary expression
// evaluator at depth+1 (the same recursion bound evalCountOperand always
// carried -- a self-referential Compare SVar must terminate), else the token
// itself as an inline expression. ok is false only when nothing resolved --
// the caller that binds a value once (effects' SetChosenNumber$ read) turns
// that into its fail-closed Note.
func resolveCountOperand(h Host, c *Ctx, tok string, depth int) (int32, bool) {
	if n, err := strconv.Atoi(tok); err == nil {
		return int32(n), true
	}
	if c.SVars != nil {
		if body, ok := c.SVars[tok]; ok {
			return evalCountExprOK(h, c, body, depth+1)
		}
	}
	return evalCountExprOK(h, c, tok, depth+1)
}

// splitDot splits an "a.b" pair into two integers, defaulting either side to
// zero if it does not parse -- the same forgiving-not-panicking convention
// applyCountOp already follows.
func splitDot(s string) (a, b int32) {
	x, y, _ := strings.Cut(s, ".")
	av, _ := strconv.Atoi(x)
	bv, _ := strconv.Atoi(y)
	return int32(av), int32(bv)
}

// countZone maps a Count$ head to the zone it scopes over.
func countZone(head string) (state.Zone, bool) {
	switch head {
	case "Valid":
		return state.ZBattlefield, true
	case "ValidHand":
		return state.ZHand, true
	case "ValidGraveyard":
		return state.ZGraveyard, true
	case "ValidLibrary":
		return state.ZLibrary, true
	case "ValidExile":
		return state.ZExile, true
	case "ValidStack":
		return state.ZStack, true
	}
	return 0, false
}

// controlsAllUrzaLands reports whether the player controls at least one
// permanent of each Urza land subtype on the battlefield -- the "is the Urza
// lands assembly complete?" predicate Count$UrzaLands encodes. Subtypes are
// matched on the Types line, where Power-Plant is hyphenated: the card Name
// "Urza's Power Plant" is a different string and matching it would be the
// exact defect the UrzaLands head exists to avoid. Iterating the dense
// object arena in order (never a map) keeps the count deterministic; it scans
// every battlefield permanent regardless of who owns it, so the controller
// test is purely o.Controller.
func controlsAllUrzaLands(g *state.Game, controller state.PlayerID) bool {
	var mine, tower, plant bool
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || o.Controller != controller || o.Face() == nil {
			continue
		}
		switch {
		case hasSubtype(o, "Urza's Mine"):
			mine = true
		case hasSubtype(o, "Urza's Tower"):
			tower = true
		case hasSubtype(o, "Urza's Power-Plant"):
			plant = true
		}
		if mine && tower && plant {
			return true
		}
	}
	return false
}

// hasSubtype reports whether an object's type line contains the given
// (possibly multi-word) subtype as consecutive tokens. Forge writes a
// subtype such as "Urza's Mine" inside the space-separated Types line, so
// the engine's fields-split representation breaks it into ["Land","Urza's",
// "Mine"]; a single-token EqualFold is therefore wrong for these and must be
// a consecutive-token match. An empty target never matches.
func hasSubtype(o *state.Object, sub string) bool {
	f := o.Face()
	if f == nil {
		return false
	}
	words := strings.Fields(sub)
	if len(words) == 0 {
		return false
	}
	for i := 0; i+len(words) <= len(f.Types); i++ {
		match := true
		for j, w := range words {
			if !strings.EqualFold(f.Types[i+j], w) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// applyCountOp applies the /Op suffix of a Count$ expression. The arithmetic
// runs in int64 and the result clamps to [math.MinInt32, math.MaxInt32], so a
// huge count can neither overflow to a sign-flipped value nor panic. Task 20
// hygiene: the old raw-int32 version made MaxInt32 doubled, negated or nudged
// silently wrap.
func applyCountOp(n int32, op string) int32 {
	v := int64(n)
	switch {
	case strings.HasPrefix(op, "Plus"):
		if x, err := strconv.Atoi(strings.TrimPrefix(op[len("Plus"):], ".")); err == nil {
			v += int64(x)
		}
	case strings.HasPrefix(op, "Minus"):
		if x, err := strconv.Atoi(strings.TrimPrefix(op[len("Minus"):], ".")); err == nil {
			v -= int64(x)
		}
	case strings.HasPrefix(op, "Times."):
		if x, err := strconv.Atoi(op[len("Times."):]); err == nil {
			v *= int64(x)
		}
	case op == "Twice":
		v *= 2
	case op == "Thrice":
		// Stronghold Arena's Count$TimesKicked/Thrice: the script writes its
		// own arithmetic as the op suffix ("gain 3 life for each time it was
		// kicked" = 3 x the kicks). rules/replacement.go's replCountOp
		// already knows the word.
		v *= 3
	case op == "HalfDown":
		v /= 2
	case op == "HalfUp":
		v = (v + 1) / 2
	case op == "Negative":
		v = -v
	case strings.HasPrefix(op, "DivideEvenlyDown."):
		// Forge's AmountOperators.divideEvenlyDown: division by the named
		// divisor (Remembered$Amount/DivideEvenlyDown.2 -- the ImmediateTrigger
		// "one instance per pair of remembered tokens" shape, diregraf_horde
		// and faebloom_trick). A missing or non-positive divisor leaves the
		// value unchanged rather than dividing by zero. NOTE this is Go's
		// integer division, which TRUNCATES toward zero, not a true floor: the
		// two differ only for negative operands (-3/2 = -1 here, floor -2),
		// and every count this op reaches in the corpus is non-negative.
		if x, err := strconv.Atoi(op[len("DivideEvenlyDown."):]); err == nil && x > 0 {
			v /= int64(x)
		}
	}
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

// ApplyCountOp is applyCountOp's exported form, for callers outside effects
// (rules' cost-modifier amount read) that must apply the same /Op suffix the
// count grammar uses -- one shared arithmetic, so the two cannot drift.
func ApplyCountOp(n int32, op string) int32 {
	return applyCountOp(n, op)
}

// SetSVars binds a copy of the SVar table to a context. A nil input leaves
// c.SVars nil, preserving the defensive-copy convention established by
// copyTargets in context.go.
func SetSVars(c *Ctx, sv map[string]string) {
	if sv == nil {
		c.SVars = nil
		return
	}
	copied := make(map[string]string, len(sv))
	for k, v := range sv {
		copied[k] = v
	}
	c.SVars = copied
}

// aggregateCastProperty sums one numeric property over the matching casts'
// objects (the ARGUMENTED !CastSaSource$<Property> aggregate forms' shared
// read; task castprov2). The property vocabulary is the zone-count heads':
// CardManaCost sums the faces' converted costs, CardPower/CardToughness the
// engine's derived (layer-aware) characteristics; any other property is
// unresolvable (0, false) — the whole Count$ then degrades per its caller's
// documented direction. Measured population: CardManaCost x1
// (call_forth_the_tempest); the other two are supported for symmetry.
func aggregateCastProperty(h Host, ids []state.ObjID, prop string) (int32, bool) {
	g := h.Game()
	var n int32
	for _, id := range ids {
		switch prop {
		case "CardManaCost":
			if o := g.Obj(id); o != nil && o.Face() != nil {
				n += o.Face().Cmc()
			}
		case "CardPower":
			n += h.Power(id)
		case "CardToughness":
			n += h.Toughness(id)
		default:
			return 0, false
		}
	}
	return n, true
}
