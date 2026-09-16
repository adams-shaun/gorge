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
	return evalCountExpr(h, c, expr, 0)
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
	if h == nil || c == nil {
		return 0
	}
	if depth > maxCountDepth {
		return 0
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
		return n
	}
	// A Sacrificed$... expression answers "the sacrificed object's" head (CR
	// 608.2g last-known-information): power, toughness, mana value, or the
	// number of objects sacrificed. It reads the LKI snapshot captured at the
	// instant of the sacrifice (Ctx.Sacrificed), never the live object -- a
	// graveyard object has no layer-derived P/T and Move has reset its
	// counters. The /Op suffix (e.g. Sacrificed$Amount/Plus.1) is applied the
	// same way Count$ applies it.
	if body, ok := strings.CutPrefix(expr, "Sacrificed$"); ok {
		return evalSacrificed(c, strings.TrimSpace(body))
	}
	// A Remembered$... expression answers a question about the objects this
	// resolving spell/ability has remembered so far (Ctx.Remembered): the one
	// head this build models is Amount -- the number of remembered objects,
	// which is Swift Silence's "Draw a card for each spell countered this
	// way" (SVar:X:Remembered$Amount after effCounter's RememberCountered$
	// True appended every countered spell). The /Op suffix is applied the
	// same way Count$ applies it. An unmodelled head degrades to zero.
	if body, ok := strings.CutPrefix(expr, "Remembered$"); ok {
		return evalRemembered(h, c, strings.TrimSpace(body))
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
		return evalTriggerCount(c, strings.TrimSpace(body))
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
		var n int32
		if body, ok2 := c.SVars[strings.TrimSpace(name)]; ok2 {
			n = evalCountExpr(h, c, body, depth+1)
		} else if v, ok2 := rollPublished(c, strings.TrimSpace(name)); ok2 {
			n = v
		}
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n
	}
	// ReplaceCount$ reads the event currently being replaced. Damage
	// replacement bodies use both the bare DamageAmount form (Vigor, Purity,
	// Hostility) and arithmetic suffixes (Fiery Emancipation, Angel of
	// Suffering). Rules carries the amount in Ctx so every supported body API,
	// not only ReplaceEffect itself, sees the same in-flight value.
	if body, ok := strings.CutPrefix(expr, "ReplaceCount$"); ok {
		field, op, hasOp := strings.Cut(strings.TrimSpace(body), "/")
		if field != "DamageAmount" && field != "Amount" {
			return 0
		}
		n := c.ReplacementAmount
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n
	}
	body, ok := strings.CutPrefix(expr, "Count$")
	if !ok {
		if n, err := strconv.Atoi(expr); err == nil {
			return int32(n)
		}
		// A bare SVar-name body (Spark Fiend's StoreSVar Expression$ Result)
		// resolves a DB$ RollDice publication of this same resolution -- the
		// same name the SVar$ indirection resolves above, in the one shape a
		// corpus body carries a bare runtime name. Anything else still
		// degrades to zero.
		if v, ok := rollPublished(c, strings.TrimSpace(expr)); ok {
			return v
		}
		return 0
	}
	body, op, hasOp := strings.Cut(body, "/")
	n := evalCountBody(h, c, strings.TrimSpace(body), depth)
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n
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
func evalTriggerCount(c *Ctx, body string) int32 {
	body, op, hasOp := strings.Cut(body, "/")
	var n int32
	switch strings.TrimSpace(body) {
	case "DamageAmount", "LifeAmount", "Amount":
		n = c.TriggerAmount
	default:
		// Result (die-roll/dice), ScryNum and ScryBottom (scry events) are
		// heads whose triggering events this build does not raise, so they
		// stay zero -- the same conservative no-op as before the prefix was
		// recognised.
		return 0
	}
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n
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
func evalSacrificed(c *Ctx, body string) int32 {
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
		// conservative same-as-before no-op the brief scopes out.
		return 0
	}
	if hasOp {
		n = applyCountOp(n, op)
	}
	return n
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

func evalRemembered(h Host, c *Ctx, body string) int32 {
	body, op, hasOp := strings.Cut(body, "/")
	switch strings.TrimSpace(body) {
	case "Amount":
		n := int32(len(c.Remembered))
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n
	}
	// A Remembered$CardPower / CardToughness / CardManaCost / CardCounters.
	// / Valid body is the shared <Ref>$<Property> family (evalRefProperty);
	// evalCountExpr routes it here first only because the Remembered$
	// prefix cut wins. An unmodelled property still degrades to zero, the
	// same conservative no-op evalSacrificed's default takes.
	if n, ok := evalRefProperty(h, c, "Remembered$"+body); ok {
		if hasOp {
			n = applyCountOp(n, op)
		}
		return n
	}
	return 0
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

func evalCountBody(h Host, c *Ctx, body string, depth int) int32 {
	g := h.Game()
	head, arg, _ := strings.Cut(body, " ")
	arg = strings.TrimSpace(arg)

	switch head {
	case "Compare":
		return evalCompare(h, c, arg, depth)
	case "xPaid":
		return c.X
	case "YourLifeTotal":
		if c.Controller < 0 || int(c.Controller) >= len(g.Players) {
			return 0
		}
		return g.Players[c.Controller].Life
	case "PlayerCountPlayers":
		return int32(g.AliveCount())
	case "PlayerCountOpponents":
		return int32(g.AliveCount() - 1)
	case "ThisTurnCast":
		// Task 17 (Storm): spells cast this turn by anyone, read off the
		// log via h.CastThisTurn() so a replay derives the same count. The
		// classic idiom is Count$ThisTurnCast/Minus1 (storm copies the spell
		// once per spell cast before it, i.e. everyone's casts minus itself).
		return int32(h.CastThisTurn())
	case "RememberedSize":
		return int32(len(c.Remembered))
	case "LifeOppsLostThisTurn":
		// The total life the controller's OPPONENTS have lost this turn
		// (Rakdos, Lord of Riots). Each opponent's loss comes from the Host's
		// log-derived LifeLostThisTurn, so the count is replay-derivable.
		if c.Controller < 0 {
			return 0
		}
		var n int32
		for _, p := range g.AliveFrom(0) {
			if p != c.Controller {
				n += h.LifeLostThisTurn(p)
			}
		}
		return n
	case "CardPower":
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			return refPower(h, o, false)
		}
		return 0
	case "CardToughness":
		if o := g.Obj(c.Source); o != nil && o.Face() != nil {
			return refToughness(h, o, false)
		}
		return 0
	}

	// ThisTurnCast_<spec> counts the spells cast this turn matching a Forge
	// spec (Count$ThisTurnCast_Card.YouCtrl — the "first/second spell you
	// cast" family): the caster scope is the controller when the spec carries
	// a You* qualifier, everyone otherwise.
	if rest, ok := strings.CutPrefix(head, "ThisTurnCast_"); ok {
		return int32(h.SpellsCastThisTurnMatching(c.Controller, rest))
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
			return yes
		}
		return no
	}

	// CardCounters.<KIND> counts a counter kind on the source.
	if kind, ok := strings.CutPrefix(head, "CardCounters."); ok {
		if o := g.Obj(c.Source); o != nil {
			return o.Counter(kind)
		}
		return 0
	}
	// Kicked.<yes>.<no> is <yes> when the source was kicked, else <no>.
	if rest, ok := strings.CutPrefix(head, "Kicked."); ok {
		yes, no := splitDot(rest)
		if o := g.Obj(c.Source); o != nil && o.CastFlags&state.FlagKicked != 0 {
			return yes
		}
		return no
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
			return assembled
		}
		return notAssembled
	}

	// Valid / ValidZone forms count objects in a zone matching a filter.
	// A "$<Property>" suffix (Count$Valid Creature.YouCtrl$CardPower —
	// Ghalta, Primal Hunger) aggregates the named numeric property over the
	// matching objects instead of counting them: CardPower/CardToughness
	// sum the face value plus P1P1 counters, CardManaCost sums mana value.
	// Without the suffix this stays the plain count. A property this build
	// cannot evaluate degrades to the plain count (the value the suffix
	// never influenced), never a hard error.
	if zone, ok := countZone(head); ok {
		spec, prop, hasProp := strings.Cut(arg, "$")
		var n int32
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(zone, p) {
				if matchesZoneSpecCtx(g, spec, id, c.SpecContext(c.Controller), zone) {
					if !hasProp {
						n++
						continue
					}
					n += objectProperty(g, id, prop)
				}
			}
		}
		return n
	}
	return 0
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
