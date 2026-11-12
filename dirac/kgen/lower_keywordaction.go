package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// scry lowers api:Scry (gorge effScry: ScryNum$, default 1, for Defined$
// players, default You).
func (lc *lowerCtx) scry(ps *params) error {
	who, err := lc.objOrTarget(ps, "You", zNone)
	if err != nil {
		return err
	}
	n, err := lc.amount(ps, "ScryNum", 1)
	if err != nil {
		return err
	}
	lc.emit(lInstr{Op: "Scry", Obj: who, A: n})
	return nil
}

// elementalBend lowers the Firebending expansion's marker, DB$ ElementalBend
// | Verb$ fire, as nothing: gorge's ElementalBend is a pure marker event
// (effects/elementalbend.go, events/apply.go), read only by trig:ElementalBend
// and the per-turn all-four ledger. trig:ElementalBend and that ledger must
// stay refused while this is a no-op. Only the node manaEffect vetted as the
// Firebending marker lowers; anywhere else it refuses.
func (lc *lowerCtx) elementalBend(ps *params) error {
	if lc.curNode == nil || lc.curNode != lc.fbMarker {
		return refuse("api ElementalBend")
	}
	ps.consume("Verb")
	return nil
}

// clueToken is gorge's investigate token script (effects/investigate.go
// clueTokenKey).
const clueToken = "c_a_clue_draw"

// investigate lowers api:Investigate (gorge effInvestigate, CR 701.16a):
// Num$ Clue tokens for the controller, each its own TokenCreate (gorge
// investigateFor emits one per Clue, outside a zone batch), so a constant
// Num$ lowers to that many one-token CreateToken ops and a computed Num$ to
// one one-token CreateToken repeated by the value (read once, as gorge's
// Num is). Defined$ Player.withMostTypeCreature (gorge playerHasMost over
// AliveFrom(controller)) runs the mint once per player who controls the
// most creatures, controller first, both on a tie. Other Defined$ and
// ValidTgts$ than You, Optional$ and RememberInvestigatingPlayers$ refuse.
func (lc *lowerCtx) investigate(ps *params) error {
	most := false
	if d, ok := ps.get("Defined"); ok {
		switch strings.TrimSpace(d) {
		case "You":
		case "Player.withMostTypeCreature":
			most = true
		default:
			return refuse("Investigate Defined$ %q", d)
		}
	}
	if _, ok := lc.p.reg.Token(clueToken); !ok {
		return refuse("token script %q not in the corpus", clueToken)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	who := lRef{Kind: rEach}
	if !most {
		if who, err = lc.investigator(ps); err != nil {
			return err
		}
	}
	var body []lInstr
	switch {
	case zeroOne(n):
		if n.Kind == vOppsLostLife {
			lc.twoSeat("the opponents who lost life counted as 0 or 1")
		}
		// one CreateToken of 0 or 1 is the TokenCreate stream of 0 or 1
		// one-token ops (a 0/1 Num$, e.g. a Compare or the opponents who
		// lost life)
		body = []lInstr{{Op: "CreateToken", Obj: who, A: n, CS: clueToken}}
	case !n.isConst():
		body = []lInstr{{Op: "Repeat", A: n, Skip: 1}, {Op: "CreateToken", Obj: who, A: cst(1), CS: clueToken}}
	case n.N < 1 || n.N > 8:
		return refuse("Investigate Num$ %q", ps.str("Num"))
	default:
		for i := 0; i < n.N; i++ {
			body = append(body, lInstr{Op: "CreateToken", Obj: who, A: cst(1), CS: clueToken})
		}
	}
	lc.out.Tokens = append(lc.out.Tokens, clueToken)
	if most {
		lc.emit(lInstr{Op: "RepeatEach", Obj: lRef{Kind: rEachPlayer}, C: 1, Skip: len(body), B: cst(kop.PlayerPredMostCreatures)})
	}
	for _, in := range body {
		lc.emit(in)
	}
	return nil
}

// investigator is the investigating player: the resolving controller, or the
// one player the Investigate's own ValidTgts$ names (gorge effInvestigate
// resolves actingPlayers: Defined$, else the targeted players, else the
// controller). Only a plain one-player target slot is read; a count rider
// would leave the investigator ambiguous here.
func (lc *lowerCtx) investigator(ps *params) (lRef, error) {
	vt, ok := ps.get("ValidTgts")
	if !ok || strings.TrimSpace(vt) == "" {
		return lRef{Kind: rYou}, nil
	}
	for _, k := range []string{"TargetMin", "TargetMax", "TargetUnique", "TgtZone"} {
		if _, ok := ps.get(k); ok {
			return lRef{}, refuse("Investigate ValidTgts$ with %s$", k)
		}
	}
	pf, err := lc.filter(vt)
	if err != nil {
		return lRef{}, err
	}
	if pf.Players == 0 || len(pf.Clauses) != 0 {
		return lRef{}, refuse("Investigate ValidTgts$ %q", vt)
	}
	ps.consume("ValidTgts", "TgtPrompt")
	return lRef{Kind: rTarget, N: lc.addTarget(lTarget{F: pf, Zone: zNone})}, nil
}

// amass lowers api:Amass (gorge effAmass): the controller amasses Num$ of
// Type$ (default Army): their first printed Army, else a new 0/0 black
// <Type> Army token (b_0_0_<type>_army, or b_0_0_army when the corpus has
// no such script), gets the counters and "is also a <Type>" (a permanent
// override). RememberAmass$ remembers it. Defined$ refuses.
func (lc *lowerCtx) amass(ps *params) error {
	if d, ok := ps.get("Defined"); ok {
		return refuse("Amass Defined$ %q", d)
	}
	typ := strings.TrimSpace(ps.str("Type"))
	if typ == "" {
		typ = "Army"
	}
	if !lc.known(typ) || !lc.known("Army") {
		return refuse("Amass Type$ %q", typ)
	}
	key := "b_0_0_" + strings.ToLower(typ) + "_army"
	if _, ok := lc.p.reg.Token(key); !ok {
		key = "b_0_0_army"
	}
	if _, ok := lc.p.reg.Token(key); !ok {
		return refuse("token script %q not in the corpus", key)
	}
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	in := lInstr{Op: "Amass", Obj: lRef{Kind: rYou}, A: n, CS: key}
	if ps.str("RememberAmass") != "" {
		in.B = cst(1)
	}
	lc.out.overrides = append(lc.out.overrides, &lOverride{AddSubtypes: []string{typ}})
	in.C = len(lc.out.overrides) - 1
	in.Note = "also a " + typ
	lc.out.Tokens = append(lc.out.Tokens, key)
	lc.emit(in)
	return nil
}

// connive lowers api:Connive of one permanent (gorge effConnive): Defined$
// Self (or absent), a triggering card, or one target; ConniveNum$ a constant
// (default 1). OpConnive draws and discards, OpConniveCounters counts the
// nonland discards. Several connivers refuse.
func (lc *lowerCtx) connive(ps *params) error {
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rSelf, rTriggered:
	case rTarget:
		if t := lc.ab.Targets[obj.N]; t.Max.Kind != vNone && !(t.Max.isConst() && t.Max.N == 1) {
			return refuse("Connive of several targets")
		}
	default:
		return refuse("Connive of %s", obj)
	}
	n, err := lc.amount(ps, "ConniveNum", 1)
	if err != nil {
		return err
	}
	if !n.isConst() || n.N < 1 {
		return refuse("ConniveNum %v", n)
	}
	lc.emit(lInstr{Op: "Connive", Obj: obj, A: n})
	lc.emit(lInstr{Op: "ConniveCounters", Obj: obj})
	return nil
}

// debuff lowers api:Debuff (gorge effDebuff: each Defined$ object on the
// battlefield -- the source when none is named -- gets a layer-6
// continuous effect removing Keywords$, until end of turn, until the
// controller's next turn (CR 611.2b, Fear of Falling) or with Duration$
// Permanent for good): an override with RemoveKw, the UntilYourNextTurn
// one with the next-turn boundary dropNextTurnOv ends. Keywords without a
// kernel bit, other durations, AllSuffixKeywords$ and Activator$ refuse.
func (lc *lowerCtx) debuff(ps *params) error {
	ks, err := removeKeywords(ps.str("Keywords"))
	if err != nil {
		return err
	}
	dur := "EOT"
	if d, ok := ps.get("Duration"); ok {
		switch d {
		case "Permanent":
			dur = "Permanent"
		case "UntilYourNextTurn":
			dur = "UntilYourNextTurn"
		default:
			return refuse("Debuff Duration %q", d)
		}
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rSelf, rTriggered, rEquipped:
	case rTarget:
		t := lc.ab.Targets[obj.N]
		if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
			return refuse("Debuff of target slot %d", obj.N)
		}
	default:
		return refuse("Debuff of %s", obj)
	}
	lc.emitOverride(obj, &lOverride{RemoveKw: ks}, dur)
	return nil
}

// explore lowers api:Explore (gorge effExplore: each Defined$ object --
// the source by default -- explores Num$ times, one OpExplore per
// explore). A constant Num$ up to 4, or a value (Jadelight Spelunker's X)
// repeating one explore; one explorer.
func (lc *lowerCtx) explore(ps *params) error {
	n, err := lc.amount(ps, "Num", 1)
	if err != nil {
		return err
	}
	if n.isConst() && (n.N < 1 || n.N > 4) {
		return refuse("Explore Num %v", n)
	}
	obj, err := lc.objOrTarget(ps, "Self", zBattlefield)
	if err != nil {
		return err
	}
	switch obj.Kind {
	case rSelf, rTriggered:
	case rTarget:
		t := lc.ab.Targets[obj.N]
		if t.F == nil || len(t.F.Clauses) == 0 || t.Zone != zBattlefield || !t.Max.isConst() || t.Max.N > 1 {
			return refuse("Explore of target slot %d", obj.N)
		}
	case rRemembered:
		if err := lc.exploreRememberedOne(); err != nil { // lower_exploreremembered.go
			return err
		}
	default:
		return refuse("Explore of %s", obj)
	}
	if !n.isConst() {
		// "explores X times" (gorge exploreOnce per pass): the one explore
		// repeated by the value, read when the loop starts
		if obj.Kind == rTarget {
			return refuse("Explore of a target slot Num %v", n)
		}
		lc.emit(lInstr{Op: "Repeat", A: n, Skip: 1})
		lc.emit(lInstr{Op: "Explore", Obj: obj})
		return nil
	}
	for i := 0; i < n.N; i++ {
		lc.emit(lInstr{Op: "Explore", Obj: obj})
	}
	return nil
}

// colorWordSet is gorge effects.colorLetters: a colour-word list (entries
// split on "," then " & ") as a WUBRG letter set; "All" is every colour,
// "Colorless" the empty set; any other word fails (ChosenColor included).
func colorWordSet(list string) (string, bool) {
	var set [5]bool
	for _, entry := range strings.Split(list, ",") {
		for _, word := range strings.Split(strings.TrimSpace(entry), " & ") {
			switch strings.ToLower(strings.TrimSpace(word)) {
			case "":
			case "all":
				set = [5]bool{true, true, true, true, true}
			case "colorless":
			case "white":
				set[0] = true
			case "blue":
				set[1] = true
			case "black":
				set[2] = true
			case "red":
				set[3] = true
			case "green":
				set[4] = true
			default:
				return "", false
			}
		}
	}
	var b strings.Builder
	for i := range set {
		if set[i] {
			b.WriteByte("WUBRG"[i])
		}
	}
	return b.String(), true
}

// discover lowers api:Discover (gorge effDiscover: the controller exiles
// from the top until a nonland card with mana value Num$ or less, the rest
// to the bottom; the found card may be cast without paying its mana cost
// (gorge's generated Play: Optional$, WithoutManaCost$, one card), else it
// goes to hand (effDiscoverBottom); the remembered set is cleared unless
// RememberDiscovered$). Lowered as OpDiscover, OpCastFree over the
// remembered card, OpDiscover's tail and ClearRemembered. Defined$ refuses.
func (lc *lowerCtx) discover(ps *params) error {
	if _, ok := ps.get("Defined"); ok {
		return refuse("Discover Defined$")
	}
	n, err := lc.amount(ps, "Num", 0)
	if err != nil {
		return err
	}
	if _, ok := ps.m["Num"]; !ok {
		return refuse("Discover without Num$")
	}
	keep := false
	if v, ok := ps.get("RememberDiscovered"); ok {
		if v != "True" {
			return refuse("Discover RememberDiscovered %q", v)
		}
		keep = true
	}
	lc.emit(lInstr{Op: "Discover", Obj: lRef{Kind: rYou}, A: n})
	lc.emit(lInstr{Op: "CastFree", Obj: lRef{Kind: rRemembered}, C: zExile, CS: "Optional|from=Exile", Note: "the discovered card, once"})
	lc.emit(lInstr{Op: "Discover", C: 1, Obj: lRef{Kind: rYou}, A: n, Note: "the discovered card still exiled to hand"})
	if !keep {
		lc.emit(lInstr{Op: "ClearRemembered"})
	}
	return nil
}
