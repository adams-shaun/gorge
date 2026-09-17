package effects

import (
	"math"
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
	if strings.HasPrefix(raw, "TriggerCount$") || strings.HasPrefix(raw, "ReplaceCount$") {
		return sign * EvalCount(h, c, raw), true
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
	// A Remembered$... expression answers a question about the objects this
	// resolving spell/ability has remembered so far (Ctx.Remembered): the one
	// head this build models is Amount -- the number of remembered objects,
	// which is Swift Silence's "Draw a card for each spell countered this
	// way" (SVar:X:Remembered$Amount after effCounter's RememberCountered$
	// True appended every countered spell). The /Op suffix is applied the
	// same way Count$ applies it. An unmodelled head degrades to zero.
	if body, ok := strings.CutPrefix(expr, "Remembered$"); ok {
		return evalRememberedOK(h, c, strings.TrimSpace(body))
	}
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
		if field != "DamageAmount" && field != "Amount" {
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
		n = applyCountOp(n, op)
	}
	return n, ok2
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

// evalRefProperty resolves one "<Ref>$<Property>[...][/Op]" count body over
// the objects a target reference names. Refs: Targeted/ParentTarget/
// ThisTargetedCard name the resolving ability's chosen targets;
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
	var ts []state.Target
	switch ref {
	case "Targeted", "ParentTarget", "ParentTargeted", "ThisTargetedCard":
		ts = c.Targets
	case "TriggeredCard", "TriggeredCardLKICopy", "TriggeredNewCardLKICopy",
		"TriggeredSpellAbility", "TriggeredAttacker", "TriggeredAttackerLKICopy",
		"TriggeredTargetLKICopy", "DelayTriggerRemembered",
		"DelayTriggerRememberedLKI", "RememberedLKI":
		ts = c.Remembered
	case "Remembered":
		ts = c.Remembered
	default:
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
		case prop == "Valid" || strings.HasPrefix(prop, "Valid "):
			spec := strings.TrimSpace(strings.TrimPrefix(prop, "Valid"))
			if (lki && MatchesObjectCtx(g, spec, o, c.SpecContext(c.Controller))) ||
				(!lki && MatchesSpecCtx(g, spec, t.Obj, c.SpecContext(c.Controller))) {
				n++
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
	}

	// PlayerCount<Players|Opponents>$<Property> — the life-total extremes
	// (Vampire Lacerator's ConditionCheckSVar$ OpponentSmallest:
	// PlayerCountOpponents$LowestLifeTotal, GE11 — "you lose 1 life unless an
	// opponent has 10 or less life"). "Players" spans every living player,
	// "Opponents" every living player but the resolving controller, the same
	// groups the bare PlayerCountPlayers/PlayerCountOpponents heads count. A
	// property other than the two life extremes is NOT resolvable: (0, false)
	// — the same verdict Count$Valid's UnknownPredicates takes — so a gate
	// over one fails OPEN (the caller's documented direction) rather than
	// enforcing a fake zero. An empty group also fails unresolvable
	// (lifeExtreme reports no extreme), for the same reason: a threshold
	// compared against an absent extreme is not readable either.
	if rest, ok := strings.CutPrefix(head, "PlayerCountPlayers$"); ok {
		if n, ok2 := lifeExtreme(g, g.AliveFrom(0), rest); ok2 {
			return n, true
		}
		return 0, false
	}
	if rest, ok := strings.CutPrefix(head, "PlayerCountOpponents$"); ok {
		var opps []state.PlayerID
		for _, p := range g.AliveFrom(0) {
			if p != c.Controller {
				opps = append(opps, p)
			}
		}
		if n, ok2 := lifeExtreme(g, opps, rest); ok2 {
			return n, true
		}
		return 0, false
	}

	// ThisTurnCast_<spec> counts the spells cast this turn matching a Forge
	// spec (Count$ThisTurnCast_Card.YouCtrl — the "first/second spell you
	// cast" family): the caster scope is the controller when the spec carries
	// a You* qualifier, everyone otherwise.
	if rest, ok := strings.CutPrefix(head, "ThisTurnCast_"); ok {
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
	// dominant spellings; the exotic predicates — Delirium, Blessing, Void,
	// Adamant_<n>.<colour> — stay unmodelled and degrade to zero). Morbid is
	// CR 702.53's "a creature died this turn": a creature entered a graveyard
	// FROM THE BATTLEFIELD this turn, folded off the same state.Entered list
	// ThisTurnEntered_ reads (a battlefield→graveyard MoveZone is exactly a
	// death, sacrifice included), so a replay derives the identical answer.
	// Monarch is the resolving controller's current designation (the same
	// state g.IsMonarch answers for a CheckDefinedPlayer$ .isMonarch spec).
	if dot := strings.IndexByte(head, '.'); dot > 0 {
		switch head[:dot] {
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
		}
	}

	// Valid / ValidZone forms count objects in a zone matching a filter.
	// A `$<Property>` suffix sums that numeric property over the matches
	// instead of counting them -- Mosswort Bridge's gate
	// `Count$Valid Creature.YouCtrl$CardPower` ("creatures you control have
	// total power 10 or greater") is the corpus shape (62 raw lines over 61
	// files: CardPower 42, CardManaCost 13, CardToughness 5). An unrecognised
	// property keeps the whole token as the spec -- the pre-existing
	// fail-closed behaviour, since such a token never matched anyway -- and
	// the Greatest/Least/Different/Colors variants are out of scope here.
	if zone, ok := countZone(head); ok {
		spec, prop, hasProp := strings.Cut(arg, "$")
		if !hasProp {
			spec, prop = arg, ""
		} else {
			prop = strings.TrimSpace(prop)
			switch prop {
			case "CardPower", "CardToughness", "CardManaCost":
			default:
				// Not a summed property (GreatestCardPower, DifferentNames,
				// Colors, ...): keep the old whole-token spec read.
				spec, prop = arg, ""
			}
		}
		var n int32
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(zone, p) {
				if !matchesZoneSpecCtx(g, spec, id, c.SpecContext(c.Controller), zone) {
					continue
				}
				if prop == "" {
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
				}
			}
		}
		return n, true
	}
	return 0, false
}

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
	if n, err := strconv.Atoi(tok); err == nil {
		return int32(n)
	}
	if c.SVars != nil {
		if body, ok := c.SVars[tok]; ok {
			return evalCountExpr(h, c, body, depth+1)
		}
	}
	return evalCountExpr(h, c, tok, depth+1)
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
	case op == "HalfDown":
		v /= 2
	case op == "HalfUp":
		v = (v + 1) / 2
	case op == "Negative":
		v = -v
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
