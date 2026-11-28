package kgen

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Defined$ compositions as kernel set expressions (design
// 2026-11-19-dirac-operand-algebras K3, kop setexpr.go). A Defined$ value
// defined() has no arm for is parsed here with gorge's grammar
// (engine/effects/context.go knownDefinedTargets, definedSpec,
// triggeredCardQualified, nonComplementTargets, oppNonComplementTargets,
// definedAliases) onto one postfix kop.SetExpr over the bases defined()
// already lowers. The ref it returns, rSet, names an entry of the card's
// Sets; the builder interns it into Table.Sets and binds it to the
// instruction's PC (build_setexpr.go). A shape the kernel would walk in a
// different order or with different duplicates from gorge refuses.

// lSet is one set expression of a card: a postfix program over lSetOps.
type lSet struct{ Ops []lSetOp }

// lSetOp is one step of an lSet: its kind, its argument (a target slot) and,
// for SetAndFilter, the filter the builder interns as the argument.
type lSetOp struct {
	Kind kop.SetOpKind
	Arg  uint16
	F    *lFilter
}

func (s lSet) String() string {
	parts := make([]string, len(s.Ops))
	for i, op := range s.Ops {
		switch {
		case op.F != nil:
			parts[i] = fmt.Sprintf("%s(F=%s%s)", op.Kind, op.F, setFilterExtra(op.F))
		case op.Kind == kop.SetTarget || op.Kind == kop.SetTargets:
			parts[i] = fmt.Sprintf("%s(%d)", op.Kind, op.Arg)
		default:
			parts[i] = op.Kind.String()
		}
	}
	return strings.Join(parts, " ")
}

// setFilterExtra is what of a set filter lClause.String omits but a SET
// line must show, so two qualifiers that differ print differently.
func setFilterExtra(f *lFilter) string {
	var b strings.Builder
	for i, c := range f.Clauses {
		if c.Name != "" {
			fmt.Fprintf(&b, " c%d.name=%q", i, c.Name)
		}
		if c.CtrlTgt != 0 {
			fmt.Fprintf(&b, " c%d.ctrltgt=%d", i, c.CtrlTgt)
		}
		if c.CmpV != nil {
			fmt.Fprintf(&b, " c%d.cmpv=%s", i, *c.CmpV)
		}
		if c.CmpFilter != nil {
			fmt.Fprintf(&b, " c%d.cmpf=%s%s", i, c.CmpFilter, setFilterExtra(c.CmpFilter))
		}
		if c.BasePermanent {
			fmt.Fprintf(&b, " c%d.basePermanent", i)
		}
		if c.TheirHand {
			fmt.Fprintf(&b, " c%d.theirHand", i)
		}
	}
	return b.String()
}

// kopSetExpr is s with every filter argument 0: the shape SetDepth reads.
func (s lSet) kopSetExpr() kop.SetExpr {
	e := make(kop.SetExpr, len(s.Ops))
	for i, op := range s.Ops {
		e[i] = kop.SetOp{Kind: op.Kind, Arg: op.Arg}
	}
	return e
}

// setTerm is a parsed part of a Defined$ value: its program and what it can
// hold by construction.
type setTerm struct {
	ops     []lSetOp
	players bool // it holds players only
	one     bool // it holds at most one member
	you     bool // it is exactly the resolving controller
	notYou  bool // it never holds the resolving controller
}

func (t setTerm) then(ops ...lSetOp) []lSetOp {
	return append(append([]lSetOp(nil), t.ops...), ops...)
}

// errNoSetHead is a part with no head the grammar or defined() reads.
type errNoSetHead struct{ v string }

func (e errNoSetHead) Error() string { return fmt.Sprintf("%q names no referent", e.v) }

// setAPIs are the apis whose kop op takes a RefSet subject (dirac
// value_setexpr.go setOp): a Defined$ set is read only while one of them
// lowers. The emitted op is checked again (build_setexpr.go kopTakesSet).
var setAPIs = map[string]bool{
	"ChangeZone": true, "Pump": true, "PutCounter": true, "DealDamage": true, "Destroy": true,
	"Sacrifice": true, "Tap": true, "Untap": true, "GainLife": true, "LoseLife": true,
	"Draw": true, "Discard": true, "GainControl": true, "Animate": true,
}

// setAPI marks the api being lowered as a set reader or not, and returns the
// restore of the previous mark (a nested api inside a handler has its own).
func (lc *lowerCtx) setAPI(api string) func() {
	prev := lc.setOK
	lc.setOK = setAPIs[api]
	return func() { lc.setOK = prev }
}

// definedSet is defined()'s last arm: v as a set expression, or the refusal
// naming why not. The bare "Defined$ %q" is kept for a value with no head.
func (lc *lowerCtx) definedSet(v string) (lRef, error) {
	if lc.inSet || lc.p != nil && lc.p.noSets {
		return lRef{}, refuse("Defined$ %q", v)
	}
	t, err := lc.setTerm(v, false)
	if err != nil {
		if nh, ok := err.(errNoSetHead); ok && nh.v == v {
			return lRef{}, refuse("Defined$ %q", v)
		}
		return lRef{}, refuse("Defined$ %q: %s", v, err.Error())
	}
	set := lSet{Ops: t.ops}
	if d, ok := set.kopSetExpr().SetDepth(); !ok || d > kop.MaxSetDepth {
		return lRef{}, refuse("Defined$ %q: a set program deeper than the kernel's %d", v, kop.MaxSetDepth)
	}
	if !lc.setOK {
		return lRef{}, refuse("Defined$ %q: a set read outside an op that takes one", v)
	}
	for i, s := range lc.out.Sets {
		if reflect.DeepEqual(s, set) {
			return lRef{Kind: rSet, N: i}, nil
		}
	}
	lc.out.Sets = append(lc.out.Sets, set)
	return lRef{Kind: rSet, N: len(lc.out.Sets) - 1}, nil
}

// errOpponentOfRef refuses Player.OpponentOf over any referent but its
// alias's: gorge reads the prefix only through definedAliases and the
// Remembered / ReplacedPlayer player-filter qualifiers (memory bases,
// dk-setexpr-04), and names nobody over any other referent.
var errOpponentOfRef = errors.New("Player.OpponentOf lowers only over TriggeredCardController (gorge names nobody over another referent but Remembered and ReplacedPlayer)")

// definedSetAliases are gorge's definedAliases that defined() does not read
// itself: each is its expansion, parsed again.
var definedSetAliases = map[string]string{
	"TargetedAndYou": "TargetedPlayer & You",
	"Player.OpponentOf TriggeredCardController": "TriggeredCardOpponent",
}

// setTerm parses v in gorge's order: aliases, a union, the OppNon and Non
// complements, the opponent forms, the controller and owner spellings, a
// qualified base, then a single referent. cmpl is set when the result feeds a
// complement, which ignores order and duplicates.
func (lc *lowerCtx) setTerm(v string, cmpl bool) (setTerm, error) {
	if exp, ok := definedSetAliases[v]; ok {
		return lc.setTerm(exp, cmpl)
	}
	if strings.Contains(v, " & ") {
		return lc.setUnion(v)
	}
	if rest, ok := strings.CutPrefix(v, "OppNon"); ok && rest != "" {
		return lc.setOppNon(rest)
	}
	if rest, ok := strings.CutPrefix(v, "Non"); ok && rest != "" {
		return lc.setNon(rest)
	}
	if strings.HasPrefix(v, "Player.OpponentOf ") {
		return setTerm{}, errOpponentOfRef
	}
	if ref, ok := definedSetOpponent[v]; ok {
		return lc.setOpponentOf(ref)
	}
	if s, ok := definedSetSuffix[v]; ok {
		return lc.setCtrlOwner(s.base, s.op, cmpl)
	}
	if base, qual, ok := strings.Cut(v, "."); ok && definedSetQualBases[base] {
		return lc.setQualified(base, qual)
	}
	return lc.setBase(v)
}

// setUnion is "A & B": gorge concatenates the parts in script order and
// de-duplicates players at first appearance (playerIDsFromTargets); the
// kernel's SetUnion keeps no order and holds each member once. They agree
// only for You first and a player who is never you.
func (lc *lowerCtx) setUnion(v string) (setTerm, error) {
	parts := strings.Split(v, " & ")
	if len(parts) != 2 {
		return setTerm{}, errors.New("union order or duplicates differ from gorge")
	}
	a, err := lc.setTerm(strings.TrimSpace(parts[0]), false)
	if err != nil {
		return setTerm{}, err
	}
	b, err := lc.setTerm(strings.TrimSpace(parts[1]), false)
	if err != nil {
		return setTerm{}, err
	}
	if !a.players || !b.players || !a.you || !b.notYou {
		return setTerm{}, errors.New("union order or duplicates differ from gorge")
	}
	return setTerm{ops: append(a.then(), b.then(lSetOp{Kind: kop.SetUnion})...), players: true}, nil
}

// complementPart parses a complement's referent, which must hold players
// only: gorge maps its objects to their players first, the kernel's
// SetPlayersComplement drops them.
func (lc *lowerCtx) complementPart(rest string) (setTerm, error) {
	t, err := lc.setTerm(rest, true)
	if err != nil {
		return setTerm{}, err
	}
	if !t.players {
		return setTerm{}, fmt.Errorf("a complement of %q, which can hold objects", rest)
	}
	return t, nil
}

// setNon is Non<ref>: every player still in the game whom ref does not name
// (gorge nonComplementTargets).
func (lc *lowerCtx) setNon(rest string) (setTerm, error) {
	t, err := lc.complementPart(rest)
	if err != nil {
		return setTerm{}, err
	}
	return setTerm{ops: t.then(lSetOp{Kind: kop.SetPlayersComplement}), players: true}, nil
}

// setOppNon is OppNon<ref>: the controller's opponents whom ref does not name
// (gorge oppNonComplementTargets), everyone not in ref plus you.
func (lc *lowerCtx) setOppNon(rest string) (setTerm, error) {
	t, err := lc.complementPart(rest)
	if err != nil {
		return setTerm{}, err
	}
	ops := t.then(lSetOp{Kind: kop.SetController}, lSetOp{Kind: kop.SetUnion}, lSetOp{Kind: kop.SetPlayersComplement})
	return setTerm{ops: ops, players: true, notYou: true}, nil
}

// definedSetOpponent are gorge's whole-value opponent spellings and the one
// player whose opponents each names (definedSpecTriggeredCardOpponent: the
// TriggeredCardController resolver's player).
var definedSetOpponent = map[string]string{
	"TriggeredCardOpponent": "TriggeredCardController",
}

// setOpponentOf is the opponents of one player: every other player still in
// the game, at any seat count.
func (lc *lowerCtx) setOpponentOf(ref string) (setTerm, error) {
	t, err := lc.setTerm(ref, false)
	if err != nil {
		return setTerm{}, err
	}
	if !t.players || !t.one {
		return setTerm{}, fmt.Errorf("the opponents of %q, which is not one player", ref)
	}
	return setTerm{ops: t.then(lSetOp{Kind: kop.SetPlayersComplement}), players: true}, nil
}

// definedSetSuffix are gorge's whole-value controller and owner spellings
// defined() has no arm for (definedSpecCodes), each the base it reads and
// the step that maps its object to a player.
var definedSetSuffix = map[string]struct {
	base string
	op   kop.SetOpKind
}{
	"CardController":           {"Self", kop.SetControllerOf},           // definedSpecCardController
	"TriggeredCardOwner":       {"TriggeredCard", kop.SetOwnerOf},       // definedSpecTriggeredCardOwner (inside NonTriggeredCardOwner)
	"TriggeredAttackerOwner":   {"TriggeredAttacker", kop.SetOwnerOf},   // definedSpecTriggeredAttackerOwner
	"OriginalHostController":   {"OriginalHost", kop.SetControllerOf},   // definedSpecOriginalHostController
	"EnchantedOwner":           {"Enchanted", kop.SetOwnerOf},           // definedSpecEnchantedOwner
	"ChosenCardController":     {"ChosenCard", kop.SetControllerOf},     // definedSpecChosenCardController
	"RememberedOwner":          {"Remembered", kop.SetOwnerOf},          // definedSpecRememberedOwner
	"ReplacedCardController":   {"ReplacedCard", kop.SetControllerOf},   // definedSpecReplacedCardController
	"ReplacedTargetController": {"ReplacedTarget", kop.SetControllerOf}, // definedSpecReplacedTargetController
}

// setCtrlOwner is a base's controller or owner. Gorge lists one player per
// object in object order, the kernel each player once in seat order: they
// agree when the base names at most one object, or the result feeds a
// complement.
func (lc *lowerCtx) setCtrlOwner(base string, op kop.SetOpKind, cmpl bool) (setTerm, error) {
	t, err := lc.setTerm(base, false)
	if err != nil {
		return setTerm{}, err
	}
	if t.players {
		return setTerm{}, fmt.Errorf("%s over players", op)
	}
	if lc.ab != nil && lc.ab.Trigger != nil && lc.ab.Trigger.Event == "Drawn" && op == kop.SetOwnerOf && t.ops[0].Kind == kop.SetTriggerObj {
		// a Drawn trigger binds no object: gorge's owner is the drawing
		// player (triggeredCardOwnerRef), the kernel's set would be empty
		return setTerm{}, fmt.Errorf("%s of %q on a Drawn trigger, which binds no object", op, base)
	}
	if !t.one && !cmpl {
		return setTerm{}, fmt.Errorf("%s over %q, which can name more than one object", op, base)
	}
	return setTerm{ops: t.then(lSetOp{Kind: op}), players: true, one: t.one}, nil
}

// definedSetQualBases are the bases gorge reads a ".<qualifier>" over
// (triggeredCardQualified's codes and definedSpec's Targeted. prefix).
var definedSetQualBases = map[string]bool{
	"Self": true, "Targeted": true, "ParentTarget": true, "ParentTargeted": true,
	"TriggeredCard": true, "TriggeredCardLKICopy": true, "TriggeredCards": true,
	"TriggeredNewCard": true, "TriggeredNewCardLKICopy": true,
	"TriggeredAttacker": true, "TriggeredAttackerLKICopy": true,
}

// setQualified is <base>.<qualifier>: the base's objects the qualifier
// admits, read as a Card filter (gorge prefixes "Card.").
func (lc *lowerCtx) setQualified(base, qual string) (setTerm, error) {
	t, err := lc.setTerm(base, false)
	if err != nil {
		return setTerm{}, err
	}
	if t.players {
		return setTerm{}, fmt.Errorf("a qualifier over the players %q names", base)
	}
	if !t.one && t.ops[0].Kind == kop.SetTarget {
		return setTerm{}, fmt.Errorf("a qualifier over a multi-object slot")
	}
	f, err := lc.filter("Card." + qual)
	if err != nil {
		return setTerm{}, fmt.Errorf("qualifier %q: %v", qual, err)
	}
	if f.Players != 0 || len(f.Clauses) == 0 {
		return setTerm{}, fmt.Errorf("qualifier %q is not an object filter", qual)
	}
	return setTerm{ops: t.then(lSetOp{Kind: kop.SetAndFilter, F: f}), one: t.one}, nil
}

// setBase is a single referent defined() lowers, as its set kind.
func (lc *lowerCtx) setBase(v string) (setTerm, error) {
	prev := lc.inSet
	lc.inSet = true
	r, err := lc.defined(v)
	lc.inSet = prev
	if err != nil {
		if err.Error() == fmt.Sprintf("Defined$ %q", v) {
			return setTerm{}, errNoSetHead{v}
		}
		return setTerm{}, fmt.Errorf("part %q: %v", v, err)
	}
	switch r.Kind {
	case rSelf:
		return setTerm{ops: []lSetOp{{Kind: kop.SetSelf}}, one: true}, nil
	case rYou:
		return setTerm{ops: []lSetOp{{Kind: kop.SetController}}, players: true, one: true, you: true}, nil
	case rTriggered:
		return setTerm{ops: []lSetOp{{Kind: kop.SetTriggerObj}}, one: true}, nil
	case rTriggeredSource:
		return setTerm{ops: []lSetOp{{Kind: kop.SetTriggerSource}}, one: true}, nil
	case rTriggeredPlayer:
		return setTerm{ops: []lSetOp{{Kind: kop.SetTriggerPlayer}}, players: true, one: true}, nil
	case rEachOpp:
		return setTerm{ops: []lSetOp{{Kind: kop.SetEachOpponent}}, players: true, notYou: true}, nil
	case rEachPlayer:
		return setTerm{ops: []lSetOp{{Kind: kop.SetEachPlayer}}, players: true}, nil
	case rTarget:
		return lc.setTargetBase(r.N)
	}
	return setTerm{}, fmt.Errorf("base %s has no set kind", refNames[r.Kind])
}

// setTargetBase is target slot n: its legal objects and players in slot
// order. A slot of players only, or of one object at most, is said so.
func (lc *lowerCtx) setTargetBase(n int) (setTerm, error) {
	if n < 0 || n >= len(lc.ab.Targets) || n >= kop.MaxTargets-1 {
		return setTerm{}, fmt.Errorf("target slot %d", n)
	}
	tg := lc.ab.Targets[n]
	t := setTerm{ops: []lSetOp{{Kind: kop.SetTarget, Arg: uint16(n)}}}
	t.one = tg.Min.isConst() && tg.Max.isConst() && tg.Min.N <= 1 && tg.Max.N <= 1
	if f := tg.F; f != nil && f.Players != 0 && len(f.Clauses) == 0 {
		t.players = true
		t.notYou = f.Players == plOpp
	}
	return t, nil
}
