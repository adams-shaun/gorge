package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// exileCounterRider reports whether the dies-to-exile body w is the plain
// exile move of ReplacedCard that also leaves counters on the exiled card:
// WithCountersType$ on the move itself (Dauthi Voidwalker's VOID, Draugr
// Necromancer's ICE, Rayami's BLOOD), or RememberChanged$ True plus a
// PutCounter sub-ability on the Remembered card (Ravenloft Adventurer's HIT).
// A WithCountersAmount$ with no kind is not a counter rider, and a bare
// PutCounter on something else is not the rider chain. The bodies this
// predicate catches that exileBody does not lower (a chain with another shape,
// or WithCountersType$ plus a sub-ability) are refused by name rather than by
// the generic "Moved to Graveyard shape".
func exileCounterRider(w *cards.SA) bool {
	if w == nil || w.API != "ChangeZone" {
		return false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" {
		return false
	}
	switch bp.str("Origin") {
	case "", "All", "Battlefield":
	default:
		return false
	}
	if _, ok := bp.get("WithCountersType"); ok {
		return true
	}
	if _, ok := bp.get("WithCountersAmount"); ok {
		return false // an amount with no kind is not a counter rider
	}
	if w.Sub == nil || w.Sub.API != "PutCounter" || w.Sub.Params["Defined"] != "Remembered" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(bp.str("RememberChanged")), "True")
}

// exileMoveHead reports whether w is a ChangeZone of ReplacedCard to Exile
// from the battlefield (or no origin), the head every dies-to-exile body
// shares, and returns its parameters.
func exileMoveHead(w *cards.SA) (*params, bool) {
	if w == nil || w.API != "ChangeZone" {
		return nil, false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" {
		return nil, false
	}
	switch bp.str("Origin") {
	case "", "All", "Battlefield":
	default:
		return nil, false
	}
	return bp, true
}

// exileTokenRider reports whether w is the dies-to-exile move of ReplacedCard
// whose linked sub-ability chain is exactly one DB$ Token node for the
// replacing permanent's controller (Kalitas's Zombie): a single token from a
// script, no amount, no counters on the exiled card. It returns the Token
// node. The kernel runs the node's prog after the exile move
// (dirac/repl_exilerider.go).
func exileTokenRider(w *cards.SA) (*cards.SA, bool) {
	bp, ok := exileMoveHead(w)
	if !ok || w.Sub == nil {
		return nil, false
	}
	if _, ok := bp.get("WithCountersType"); ok {
		return nil, false
	}
	if _, ok := bp.get("WithCountersAmount"); ok {
		return nil, false
	}
	if h, ok := bp.get("Hidden"); ok && !strings.EqualFold(h, "True") {
		return nil, false
	}
	bp.consume("Defined", "Destination", "Origin", "Hidden", "SubAbility")
	if len(bp.unread()) != 0 {
		return nil, false
	}
	if !exileTokenNode(w.Sub) {
		return nil, false
	}
	return w.Sub, true
}

// exileTokenNode reports whether t is exactly one token from a script for the
// replacing permanent's controller, the body both token riders share.
func exileTokenNode(t *cards.SA) bool {
	if t == nil || t.API != "Token" || t.Sub != nil {
		return false
	}
	tp := newParams(t.Params)
	script := strings.TrimSpace(tp.str("TokenScript"))
	if script == "" || strings.Contains(script, ",") || tp.str("TokenOwner") != "You" {
		return false
	}
	if _, ok := tp.get("TokenAmount"); ok {
		return false
	}
	return true
}

// exileReflexTokenRider reports whether w is the dies-to-exile move of
// ReplacedCard whose linked sub-ability is an ImmediateTrigger (CR 603.12)
// whose Execute$ resolves to the same single-token body exileTokenRider
// accepts (Head of the Hunt's Wolf, Nemata's Saproling). The ImmediateTrigger
// must carry only Execute$ and the descriptive TriggerDescription$: an amount,
// a remember capture or Static$ is not this rider. It returns the resolved
// Token node, the body the kernel puts on the stack as a reflexive trigger.
func exileReflexTokenRider(w *cards.SA, face *cards.Face) (*cards.SA, bool) {
	bp, ok := exileMoveHead(w)
	if !ok || face == nil || w.Sub == nil {
		return nil, false
	}
	if _, ok := bp.get("WithCountersType"); ok {
		return nil, false
	}
	if _, ok := bp.get("WithCountersAmount"); ok {
		return nil, false
	}
	if h, ok := bp.get("Hidden"); ok && !strings.EqualFold(h, "True") {
		return nil, false
	}
	bp.consume("Defined", "Destination", "Origin", "Hidden", "SubAbility")
	if len(bp.unread()) != 0 {
		return nil, false
	}
	it := w.Sub
	if it.API != "ImmediateTrigger" || it.Sub != nil {
		return nil, false
	}
	for _, k := range sortedParamKeys(it.Params) {
		if k != "Execute" && k != "TriggerDescription" {
			return nil, false
		}
	}
	t := cards.ResolveSVar(face.SVars, strings.TrimSpace(it.Params["Execute"]))
	if !exileTokenNode(t) {
		return nil, false
	}
	return t, true
}

// exileRiderClaim classifies a dies-to-exile body exileBody rejected.
// claimed is false when the body is no rider shape and the caller falls
// through to the other Moved-to-Graveyard lowerings. A claimed body is either
// a token rider (token true, err nil) -- the direct Token chain or the
// ImmediateTrigger reflexive form whose Execute$ resolves on face -- or
// refused by name: the counter riders without a lowering and any other
// sub-ability chain name their first node, never the generic "Moved to
// Graveyard shape".
func exileRiderClaim(w *cards.SA, face *cards.Face) (token, claimed bool, err error) {
	if exileCounterRider(w) {
		return false, true, refuse("repl-hard: dies-to-exile counter rider")
	}
	if _, ok := exileTokenRider(w); ok {
		return true, true, nil
	}
	if _, ok := exileReflexTokenRider(w, face); ok {
		return true, true, nil
	}
	if _, ok := exileMoveHead(w); ok && w.Sub != nil {
		return false, true, refuse("repl-hard: dies-to-exile rider %s", w.Sub.API)
	}
	return false, false, nil
}

// exileRiderProg lowers the token rider's chain into rl.Prog: a latent
// ability the kernel runs after the exile move, the replacing permanent's
// controller as "You" and the exiled card as ReplacedCard. The any-zone path
// has no runtime hook, so it is refused by name.
func (lc *lowerCtx) exileRiderProg(w *cards.SA, anyZone bool, rl *lRepl) error {
	if anyZone {
		return refuse("repl-hard: dies-to-exile rider on the any-zone path")
	}
	ab := &lAbility{Kind: akLatent}
	if err := lc.withAbility(ab, func() error { return lc.chain(w.Sub, nil) }); err != nil {
		return err
	}
	rl.Prog = ab.Prog
	return nil
}

// exileCounterChainBody lowers the RememberChanged$ chain form of the
// dies-to-exile counter rider (Ravenloft Adventurer): the plain exile move of
// ReplacedCard with RememberChanged$ True, a PutCounter of one counter on
// Remembered, and a Cleanup of the remembered card. It is the same replacement
// as WithCountersType$ (the exiled card keeps the counter), so it returns the
// same kind and count exileBody's body-parameter form does. Any other
// parameter, a count other than one, a kind outside the exile-counter store or
// a longer chain is not this form.
func exileCounterChainBody(w *cards.SA) (ctr string, n uint8, ok bool) {
	pc := w.Sub
	if pc == nil || pc.API != "PutCounter" || pc.Sub == nil || pc.Sub.Sub != nil {
		return "", 0, false
	}
	cl := pc.Sub
	if cl.API != "Cleanup" {
		return "", 0, false
	}
	bp := newParams(w.Params)
	if bp.str("Defined") != "ReplacedCard" || bp.str("Destination") != "Exile" ||
		!strings.EqualFold(bp.str("RememberChanged"), "True") {
		return "", 0, false
	}
	if o := bp.str("Origin"); o != "" && o != "All" && o != "Battlefield" {
		return "", 0, false
	}
	if h, has := bp.get("Hidden"); has && !strings.EqualFold(h, "True") {
		return "", 0, false
	}
	bp.consume("Defined", "Destination", "Origin", "Hidden", "RememberChanged", "SubAbility")
	if len(bp.unread()) != 0 {
		return "", 0, false
	}
	pp := newParams(pc.Params)
	if pp.str("Defined") != "Remembered" {
		return "", 0, false
	}
	if v, has := pp.get("CounterNum"); has && strings.TrimSpace(v) != "1" {
		return "", 0, false
	}
	ctr = pp.str("CounterType")
	switch strings.ToUpper(ctr) {
	case "VOID", "ICE", "BLOOD", "HIT":
	default:
		return "", 0, false
	}
	pp.consume("Defined", "CounterType", "CounterNum", "SubAbility")
	if len(pp.unread()) != 0 {
		return "", 0, false
	}
	cp := newParams(cl.Params)
	if !strings.EqualFold(cp.str("ClearRemembered"), "True") {
		return "", 0, false
	}
	cp.consume("ClearRemembered")
	if len(cp.unread()) != 0 {
		return "", 0, false
	}
	return ctr, 1, true
}
