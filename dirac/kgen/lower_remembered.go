package kgen

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// chainReadsRemembered reports whether an SA chain, or an SVar one of its
// lines names, mentions a remembered set.
func (lc *lowerCtx) chainReadsRemembered(sa *cards.SA) bool {
	for n := sa; n != nil; n = n.Sub {
		if strings.Contains(n.Line, "Remember") {
			return true
		}
		for _, tok := range strings.FieldsFunc(n.Line, func(r rune) bool {
			return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
		}) {
			if v, ok := lc.face.SVars[tok]; ok && strings.Contains(v, "Remember") {
				return true
			}
		}
	}
	return false
}

const noteRememberDiscarded = "remember discarded"

// rememberDiscarded reads Discard's RememberDiscarded$: gorge
// discardAndRememberEvent adds each discarded card to the resolution's
// remembered set (and the source's; a reader in another ability is refused
// by refuseCrossAbilityRemembered). The note makes kremWrites see it.
func rememberDiscarded(ps *params, in *lInstr) error {
	v, ok := ps.get("RememberDiscarded")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("RememberDiscarded %q", v)
	}
	in.Note = noteRememberDiscarded
	return nil
}

const noteRememberSacrificed = "remember sacrificed"

// rememberSacrificed reads Sacrifice's RememberSacrificed$ (gorge
// effSacrifice appends each sacrificed permanent to the remembered set).
func rememberSacrificed(ps *params, in *lInstr) error {
	v, ok := ps.get("RememberSacrificed")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("RememberSacrificed %q", v)
	}
	in.Note = noteRememberSacrificed
	return nil
}

// refuseCtxRememberedCount refuses an ability whose Remembered$Amount (the
// resolution's own remembered list, gorge evalRememberedOK ->
// rememberedExcludingCapture) shares its program with a RememberRemoved$
// RemoveCounter: gorge records those removals only on the source's
// persistent list (removecounter.go rememberRemoved), which
// Remembered$Amount does not read, while the kernel's RememberedCount adds
// the removed count (remCnt).
func (lc *lowerCtx) refuseCtxRememberedCount() {
	for _, a := range lc.out.Abilities {
		ctx, removed := false, false
		for _, in := range a.Prog {
			s := in.String()
			ctx = ctx || strings.Contains(s, "RememberedCount(ctx)")
			removed = removed || in.Note == "remember removed"
		}
		if ctx && removed {
			lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "Remembered$Amount beside RememberRemoved$ (gorge keeps the removed counters off the resolution's list)")
		}
	}
}

// rememberParam reads a Remember*$ param that only means True
// (Surveil RememberMoved$, SacrificeAll RememberSacrificed$); any other value
// refuses. Draw's RememberDrawn$ also takes AllReplaced (rememberDrawn).
func rememberParam(ps *params, name string) (bool, error) {
	v, ok := ps.get(name)
	if !ok {
		return false, nil
	}
	if v != "True" {
		return false, refuse("%s %q", name, v)
	}
	return true, nil
}

// rememberDrawn reads Draw's RememberDrawn$: True remembers the card the
// draw itself moved, AllReplaced every card the draw and its replacement body
// moved into the drawing player's hand (gorge draw.go recordDrawn).
func rememberDrawn(ps *params) (remember, all bool, err error) {
	v, ok := ps.get("RememberDrawn")
	if !ok {
		return false, false, nil
	}
	switch v {
	case "True":
		return true, false, nil
	case "AllReplaced":
		return true, true, nil
	default:
		return false, false, refuse("RememberDrawn %q", v)
	}
}

// remember3 maps a Draw/Surveil/SacrificeAll CS "Remember" (or a Draw's
// "RememberAll") to its remember bits on the instruction; any other CS refuses.
func remember3(cs string, ki *kop.Instr) error {
	switch cs {
	case "":
	case "Remember":
		ki.C |= kop.MvRemember
	case "RememberAll":
		ki.C |= kop.MvRemember | kop.DrawRememberAll
	default:
		return fmt.Errorf("remember write %q", cs)
	}
	return nil
}

// api:Attach Object$ Remembered | Defined$ Self.
// gorge attachObjectRemembered (effects/attach.go) attaches EVERY object of
// the resolution's remembered set; the kernel's OpAttachObj reads rem[0].
// The kernel attaches every remembered object too (attachRemembered,
// dirac/tgt_attachremembered.go), so a token-doubling replacement making
// several tokens from the one writer still matches; what the kernel cannot
// see is gorge's extra entries (the triggering object first), so the
// lowering admits the shape only behind a single one-object writer.

// kattremOneWriter reports whether the current ability's program, before
// the instruction being emitted, holds exactly one remembered-set writer
// and it writes at most one object: a token of constant amount 1, or a
// one-card move. It scans only the ability's own program: a remembered set
// written outside it (an ability cost's Remember*, a trigger-level remember)
// is not seen; no corpus card has that shape with this Attach.
func (lc *lowerCtx) kattremOneWriter() bool {
	n := 0
	var w lInstr
	for _, in := range lc.ab.Prog {
		if kremWrites(in) {
			n++
			w = in
		}
	}
	if n != 1 || !w.A.isConst() || w.A.N != 1 {
		return false
	}
	return w.Op == "CreateToken" || w.Op == "ChooseMove"
}

// Destroy's RememberDestroyed$, PutCounter's
// RememberAmount$ and an ImmediateTrigger's IsPresent$ (Shredder's
// Technique, Wishing Well, Earthbender Ascension).

// noteRememberDestroyed marks a single Destroy whose destroyed target joins
// the remembered set (kremWrites reads the "remember" in it).
const noteRememberDestroyed = "remember destroyed"

// notePutRememberAmount marks a PutCounter whose landed counters count into
// the remembered set (RememberAmount$).
const notePutRememberAmount = "remember amount"

// destroyRememberDestroyed reads a single Destroy's RememberDestroyed$ True
// (gorge effDestroy, engine/effects/zone_bulk.go: the target is remembered
// only when the destroy moved it to a graveyard -- an indestructible,
// regenerated, shielded or replaced-away target is not). C bit 1 carries it
// (bit 0 is NoRegen$; kernel destroyRemember). Only a one-object target
// slot lowers; RememberLKI$ beside it refuses (gorge captures LKI too).
func destroyRememberDestroyed(ps *params, in *lInstr) error {
	v, ok := ps.get("RememberDestroyed")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("RememberDestroyed %q", v)
	}
	if _, lki := ps.m["RememberLKI"]; lki {
		return refuse("Destroy RememberDestroyed$ with RememberLKI$")
	}
	if in.Obj.Kind != rTarget {
		return refuse("Destroy RememberDestroyed$ of %v", in.Obj)
	}
	if tm, ok := ps.m["TargetMax"]; ok && strings.TrimSpace(tm) != "1" {
		return refuse("Destroy RememberDestroyed$ with TargetMax$ %q", tm)
	}
	in.C |= 2
	in.Note = noteRememberDestroyed
	return nil
}

// putCounterRememberAmount reads a single-object PutCounter's
// RememberAmount$ True (gorge counters_put.go putCounterLanded: the
// recipient is appended once per counter that LANDED, replacements and
// prevention deciding the count). The kernel adds the landed count to the
// resolution's remembered count (remCnt), so only a count read
// (RememberedCount) may follow: refuseRememberAmountObjects refuses a card
// that reads the remembered objects. B=1 carries it.
func putCounterRememberAmount(ps *params, in *lInstr) error {
	v, ok := ps.get("RememberAmount")
	if !ok {
		return nil
	}
	if v != "True" {
		return refuse("RememberAmount %q", v)
	}
	for _, k := range []string{"Choices", "UpTo", "RememberPut", "CounterNumPerDefined"} {
		if _, ok := ps.m[k]; ok {
			return refuse("PutCounter RememberAmount$ with %s$", k)
		}
	}
	if in.Obj.Kind != rSelf && in.Obj.Kind != rTarget {
		return refuse("PutCounter RememberAmount$ on %v", in.Obj)
	}
	if tm, ok := ps.m["TargetMax"]; ok && strings.TrimSpace(tm) != "1" {
		return refuse("PutCounter RememberAmount$ with TargetMax$ %q", tm)
	}
	in.B = cst(1)
	in.Note = notePutRememberAmount
	return nil
}

// refuseRememberAmountObjects refuses a card with a RememberAmount$
// PutCounter that reads the remembered OBJECTS anywhere (gorge appends the
// recipient n times; the kernel records only the count).
func (lc *lowerCtx) refuseRememberAmountObjects() {
	amount := false
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			amount = amount || isRememberAmountWriter(in)
		}
	}
	if !amount {
		return
	}
	noCount := func(s string) string { return strings.ReplaceAll(s, "RememberedCount", "") }
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			if isRememberAmountWriter(in) {
				continue
			}
			if kremReads(noCount(in.String())) {
				lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "RememberAmount$ beside a remembered-object read (the kernel records the landed count, not the recipient n times)")
				return
			}
		}
	}
	for _, s := range lc.out.Statics {
		if kremReads(noCount(fmt.Sprint(s.F))) || kremReads(noCount(fmt.Sprint(s.Cond))) {
			lc.refuseNode("card", "RememberAmount$ beside a static remembered read")
			return
		}
	}
}

// immediateIsPresent consumes an ImmediateTrigger's IsPresent$ when it is
// the same SA's ConditionPresent$ text exactly: gorge effImmediateTrigger
// (engine/effects/immediate.go) never reads IsPresent$, only the shared
// ConditionPresent$ gate applies, so the two agree (Earthbender
// Ascension). Anything else refuses.
func immediateIsPresent(ps *params) error {
	ip, ok := ps.m["IsPresent"]
	if !ok {
		return nil
	}
	for _, k := range []string{"PresentCompare", "PresentZone", "PresentDefined", "IsPresent2"} {
		if _, ok := ps.m[k]; ok {
			return refuse("ImmediateTrigger IsPresent$ %q with %s$", ip, k)
		}
	}
	cp, ok := ps.m["ConditionPresent"]
	if !ok || strings.TrimSpace(cp) != strings.TrimSpace(ip) {
		return refuse("ImmediateTrigger IsPresent$ %q", ip)
	}
	ps.consume("IsPresent")
	return nil
}

// The kernel's remembered set is per resolution (dirac/resolve.go clears it
// as every resolution begins). gorge's also persists on the SOURCE object
// (effects eventRemember / clearEventRemembered, read back through
// sourceRemembered with its Incarnation snapshot), so an ability that reads
// Remembered without writing it in its own program reads what ANOTHER
// ability of the card remembered in an earlier resolution: Hoarding Dragon's
// dies trigger returns the card its ETB exiled, Dance of Many's leave
// trigger exiles the token its ETB copied. In the kernel that reader finds an
// empty set, an inexact lowering, so the card refuses
// (kremember). Loops that read the set their own body writes
// (Grindstone, Sphinx's Tutelage) are same-resolution and stay lowered.

var (
	kremTokRemember = regexp.MustCompile(`^CreateToken .*\bC=(\d+)`)
	// CastFree reads the remembered set only for the remembered population
	// (obj=Remembered, caught by the substring test below); a targeted or
	// filter population reads a target slot or a zone, not
	// Remembered. TwoPiles still walks the set implicitly.
	kremCastRead = regexp.MustCompile(`^TwoPiles\b`)
)

// kremReads: the line reads the resolution's remembered set (an
// obj=Remembered operand, a Remembered/NotRemembered filter or cond ref,
// RememberedCount, or an op that walks the set implicitly).
func kremReads(l string) bool {
	l2 := strings.NewReplacer("DelayedRemembered", "", "ClearRemembered", "").Replace(l)
	return strings.Contains(l2, "Remembered") || kremCastRead.MatchString(l)
}

// hasRememberWriter reports whether the ability's program so far holds a line
// that writes the resolution's remembered set. A triggered ability's plain
// Remembered group is its fire-time capture unless a link of this same
// resolution wrote its own set (DigUntil RememberFound$ True in Will Riker,
// First in the Field's beginning-of-combat chain).
func (lc *lowerCtx) hasRememberWriter() bool {
	if lc.ab == nil {
		return false
	}
	for _, in := range lc.ab.Prog {
		if kremWrites(in) {
			return true
		}
	}
	return false
}

// kremWrites: the line adds to the resolution's remembered set.
func kremWrites(in lInstr) bool {
	s := in.String()
	if strings.Contains(in.CS, "Remember") || strings.Contains(in.Note, "remember") {
		return true
	}
	if m := kremTokRemember.FindStringSubmatch(s); m != nil {
		var n int
		fmt.Sscan(m[1], &n)
		return n&4 != 0 // kop.TokRememberObj
	}
	if in.Op == "Counter" && in.B.isConst() && in.B.N&kop.CtrRemember != 0 { // RememberChanged$ on a stack move (lower_stackexile.go)
		return true
	}
	if in.Op == "Amass" && in.B.N != 0 { // RememberAmass$ (B.N bit 15; gorge effects/amass.go PKRememberAmass)
		return true
	}
	if in.Op == "Seek" && in.C&1 != 0 { // RememberFound$
		return true
	}
	if in.Op == "Discover" && in.C == 0 { // the discovered card
		return true
	}
	if in.Op == "Cloak" && in.C&kop.CfRemember != 0 { // RememberCloaked$
		return true
	}
	if in.Op == "ManifestDread" && in.C&kop.CfRemember != 0 { // RememberManifested$
		return true
	}
	if isRememberAmountWriter(in) { // RememberAmount$
		return true
	}
	if in.Op == "PutCounter" && in.Note == notePutRememberPut { // RememberPut$
		return true
	}
	if in.Op == "CounterAll" { // RememberCounteredSA$: the sweep's count joins remCnt
		return true
	}
	return in.Op == "DestroyAll" && in.C&1 != 0 // RememberDestroyed$
}

// remHandMoveWrites: a remember writer (a hand move's, battlefield or
// target-player pick's or reveal's SrRemember, a seek's RememberFound$).
func remHandMoveWrites(in lInstr) bool {
	return in.Op == "ChooseMove" && strings.Contains(in.CS, "Remember") || in.Op == "Seek" && in.C&1 != 0
}

// refuseCrossAbilityRemembered refuses each ability that reads the
// remembered set but never writes it in its own program. A card with a hand-move
// remember writer also refuses a static or replacement that reads a
// remembered set (Mesmeric Fiend's leave return reads what its ETB took).
func (lc *lowerCtx) refuseCrossAbilityRemembered() {
	if lc.remPersistShape() {
		// the leaves trigger reads the enters trigger's set (lower_rempersist.go)
		lc.out.RemPersist, lc.out.RemSeed, lc.out.RemKeep = true, ^uint32(0), ^uint32(0)
		return
	}
	if seed, keep, ok := lc.remPersistSaga(); ok {
		// a later chapter reads what an earlier one remembered (lower_rempersist.go)
		lc.out.RemPersist, lc.out.RemSeed, lc.out.RemKeep = true, seed, keep
		return
	}
	c3c := false
	for _, a := range lc.out.Abilities {
		for _, in := range a.Prog {
			c3c = c3c || remHandMoveWrites(in)
		}
	}
	if c3c {
		for _, s := range lc.out.Statics {
			if kremReads(fmt.Sprint(s.F)) || kremReads(fmt.Sprint(s.Cond)) {
				lc.refuseNode("card", "a static reads Remembered written by an ability (the kernel's remembered set is per resolution; gorge's persists on the source)")
				return
			}
		}
		for _, r := range lc.out.Repls {
			for _, in := range r.Prog {
				if kremReads(in.String()) {
					lc.refuseNode("card", "a replacement reads Remembered written by an ability (the kernel's remembered set is per resolution; gorge's persists on the source)")
					return
				}
			}
		}
	}
	for _, a := range lc.out.Abilities {
		if !a.RemTgt {
			continue
		}
		for _, in := range a.Prog {
			if kremReads(in.String()) {
				lc.refuseNode("card", "a Remembered read beside a Pump RememberObjects$ ThisTargetedCard")
				return
			}
		}
	}
	for _, a := range lc.out.Abilities {
		var lines []string
		if a.Trigger != nil {
			t := a.Trigger
			lines = append(lines, fmt.Sprint(t.F), fmt.Sprint(t.F2), fmt.Sprint(t.Cond))
		}
		lines = append(lines, fmt.Sprint(a.ActCond))
		for _, t := range a.Targets {
			lines = append(lines, fmt.Sprint(t.F))
		}
		writes := false
		for _, in := range a.Prog {
			lines = append(lines, in.String())
			writes = writes || kremWrites(in)
		}
		if writes {
			continue
		}
		for _, l := range lines {
			if kremReads(l) {
				lc.refuseNode(fmt.Sprintf("ab[%d]", a.Index), "reads Remembered written by another ability (the kernel's remembered set is per resolution; gorge's persists on the source)")
				break
			}
		}
	}
}

// three remember writes and their readers.
//
//   - Pump RememberObjects$ ThisTargetedCard (gorge effects/pump.go
//     rememberDefinedObjects) on the chain's root link, read by a later
//     EachDamage DefinedDamagers$ Remembered: the remembered set is that
//     link's target slot, so the reader lowers to the kernel's damager-slot
//     mask. Each Pump on the ability's own chain remembers its own slot, so
//     Graceful Takedown's two Pumps union into one damager mask.
//   - Attach RememberAttached$ True (gorge attachTo): the attaching object
//     joins the remembered set when the attach happens (attach in
//     lower_effect.go, OpAttach C MvRemember).
//   - ImmediateTrigger RememberObjects$ Targeted (gorge effImmediateTrigger:
//     the instance remembers the parent's target) read by the body's Fight
//     Defined$ DelayTriggerRemembered (Novel Nunchaku, Curse of the
//     Werefox): the capture is the target's object and ver
//     (dirac/zone_rememberwrites.go reflexCapture), so a creature that
//     changed zones since (a new object, CR 400.7) does not fight.

// pumpRememberTargeted reads a Pump's RememberObjects$ after lc.pump; t0 is
// lc.lastTarget() before it.
func (lc *lowerCtx) pumpRememberTargeted(ps *params, t0 int) error {
	v, ok := ps.m["RememberObjects"]
	if !ok {
		return nil
	}
	if v = strings.TrimSpace(v); v == "TargetedController" {
		if err := lc.pumpRememberOppController(t0); err != nil {
			return err
		}
		ps.consume("RememberObjects")
		return nil
	}
	if v != "ThisTargetedCard" {
		return refuse("Pump RememberObjects$ %q", v)
	}
	if lc.depth != 1 || lc.mode >= 0 {
		// gorge's ThisTargetedCard is the link's own answer (a sub link's
		// PickedTargets, the root's Ctx.Targets: effects/context.go
		// definedSpecThisTargetedCard), so on the ability's own chain each
		// Pump remembers its own slot (Graceful Takedown's two Pumps);
		// a nested or modal chain stays unread
		return refuse("Pump RememberObjects$ ThisTargetedCard in a nested or modal chain")
	}
	t := lc.lastTarget()
	if t < 0 || t == t0 {
		return refuse("Pump RememberObjects$ ThisTargetedCard without its own target")
	}
	lc.remTgt |= 1 << t
	lc.ab.RemTgt = true
	ps.consume("RememberObjects")
	return nil
}

// pumpRememberOppController lowers a Pump's RememberObjects$
// TargetedController over the link's own opponent-controlled target slot
// (Rules of Acquisition's third rule: the permanent's controller as the
// Pump resolves draws after you have gained control of it). gorge's Pump
// adds the target's controller, a player, to the remembered list
// (effects/pump.go rememberDefinedObjects), and a later link's Defined$
// Remembered names that player. The kernel's remembered set holds objects
// only, but here the player is fixed: the slot's filter makes the target
// opponent-controlled, a target that no longer is is illegal when the
// spell resolves (CR 608.2b), so in two players the remembered player is
// the opponent, captured before any later link changes control. It lowers
// only when the rest of the chain reads the set solely as a Draw's
// Defined$ Remembered and ends with Cleanup ClearRemembered$ True, so no
// other reader sees the player; on those links defined lowers Defined$
// Remembered as the opponent (remOppLink).
func (lc *lowerCtx) pumpRememberOppController(t0 int) error {
	t := lc.lastTarget()
	if t < 0 || t == t0 {
		return refuse("Pump RememberObjects$ TargetedController without its own target")
	}
	f := lc.ab.Targets[t].F
	if f == nil || f.Players != 0 || len(f.Clauses) == 0 {
		return refuse("Pump RememberObjects$ TargetedController over a slot that may name a player")
	}
	for _, c := range f.Clauses {
		if c.Ctrl != whoOpp {
			return refuse("Pump RememberObjects$ TargetedController over a slot not limited to opponents' objects")
		}
	}
	lc.twoSeat("the remembered target controller read as the one opponent")
	var links []*cards.SA
	cleared := false
	for n := lc.curNode.Sub; n != nil; n = n.Sub {
		if cleared {
			return refuse("Pump RememberObjects$ TargetedController: a link after the Cleanup")
		}
		for k, v := range n.Params {
			if !strings.Contains(v, "Remember") && !strings.Contains(k, "Remember") {
				continue
			}
			switch {
			case n.API == "Draw" && k == "Defined" && strings.TrimSpace(v) == "Remembered":
			case n.API == "Cleanup" && k == "ClearRemembered" && strings.TrimSpace(v) == "True":
				cleared = true
			default:
				return refuse("Pump RememberObjects$ TargetedController read other than as a Draw's Defined$")
			}
		}
		if lc.chainReadsRememberedSVar(n) {
			return refuse("Pump RememberObjects$ TargetedController beside an SVar naming a remembered set")
		}
		links = append(links, n)
	}
	if !cleared {
		return refuse("Pump RememberObjects$ TargetedController without a closing Cleanup ClearRemembered$")
	}
	lc.remOppLinks = links
	return nil
}

// chainReadsRememberedSVar reports whether a card SVar the link's line names
// mentions a remembered set.
func (lc *lowerCtx) chainReadsRememberedSVar(n *cards.SA) bool {
	for _, tok := range strings.FieldsFunc(n.Line, func(r rune) bool {
		return !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_')
	}) {
		if tok == n.Params["SubAbility"] {
			continue // the next link, walked on its own
		}
		if v, ok := lc.face.SVars[tok]; ok && strings.Contains(v, "Remember") {
			return true
		}
	}
	return false
}

// remOppLink reports whether the link being lowered follows a Pump
// RememberObjects$ TargetedController (pumpRememberOppController).
func (lc *lowerCtx) remOppLink() bool {
	for _, n := range lc.remOppLinks {
		if n == lc.curNode {
			return true
		}
	}
	return false
}

// eachDamageRemembered is EachDamage DefinedDamagers$ Remembered's damager
// slots: the slots the chain's Pumps remembered, ascending.
func (lc *lowerCtx) eachDamageRemembered() ([]int, error) {
	if lc.remTgt == 0 {
		return nil, refuse("EachDamage DefinedDamagers$ Remembered without a Pump RememberObjects$ ThisTargetedCard")
	}
	var slots []int
	for t := 0; lc.remTgt>>t != 0; t++ {
		if lc.remTgt&(1<<t) != 0 {
			slots = append(slots, t)
		}
	}
	return slots, nil
}

// reflexRememberTargeted reads an ImmediateTrigger's RememberObjects$
// Targeted before its body lowers: the parent's one battlefield object
// target is captured into the pending trigger (lInstr Obj). Absent, or a
// Remembered… value (lower_trigremembered.go), returns the zero ref; another
// value stays unread.
//
// RememberObjects$ ParentTarget (Thorin, Mountain-king) is gorge's
// parentLinkTargets at the untargeted ImmediateTrigger: the nearest targeting
// link's set, the chain's last target slot (the Attach's own creature), which
// must be one battlefield object.
func (lc *lowerCtx) reflexRememberTargeted(ps *params, lat *lAbility, n lValue) (lRef, error) {
	v, ok := ps.m["RememberObjects"]
	if v = strings.TrimSpace(v); ok && v == "ParentTarget" {
		return lc.reflexRememberParentTarget(ps, lat, n)
	}
	if !ok || v != "Targeted" {
		return lRef{}, nil
	}
	if len(lc.chainT) != 1 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ Targeted over %d slots", len(lc.chainT))
	}
	slot := lc.chainT[0]
	t := lc.ab.Targets[slot]
	if t.Zone != zBattlefield || t.AtRes || t.F == nil || t.F.Players != 0 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ Targeted of a non-battlefield target")
	}
	if !n.isConst() || n.N != 1 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ Targeted with TriggerAmount$ %s", n)
	}
	if _, ok := ps.m["RememberEach"]; ok {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ Targeted with RememberEach$")
	}
	ps.consume("RememberObjects")
	lat.ReflexObj = true
	return lRef{Kind: rTarget, N: slot}, nil
}

// reflexRememberParentTarget is reflexRememberTargeted's RememberObjects$
// ParentTarget arm: the chain's last target slot, one battlefield object.
func (lc *lowerCtx) reflexRememberParentTarget(ps *params, lat *lAbility, n lValue) (lRef, error) {
	slot := lc.lastTarget()
	if slot < 0 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ ParentTarget without a parent target")
	}
	t := lc.ab.Targets[slot]
	if t.Zone != zBattlefield || t.AtRes || t.F == nil || t.F.Players != 0 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ ParentTarget of a non-battlefield target")
	}
	if !t.Max.isConst() || t.Max.N != 1 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ ParentTarget over a several-object slot")
	}
	if !n.isConst() || n.N != 1 {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ ParentTarget with TriggerAmount$ %s", n)
	}
	if _, ok := ps.m["RememberEach"]; ok {
		return lRef{}, refuse("ImmediateTrigger RememberObjects$ ParentTarget with RememberEach$")
	}
	ps.consume("RememberObjects")
	lat.ReflexObj = true
	return lRef{Kind: rTarget, N: slot}, nil
}

// rememberLKINote marks a Destroy whose target's last known information is
// remembered (RememberLKI$ + AlwaysRemember$; Gleeful Demolition).
const rememberLKINote = "remember LKI"

// destroyRememberLKI reads Destroy's RememberLKI$ True with AlwaysRemember$
// True (gorge effDestroy remembers the target's LKI whether or not it was
// destroyed): the remembered object is the destroy's own target, which a
// later ConditionDefined$ RememberedLKI reads (rememberedLKIRef). Only a
// single object target lowers; either key alone refuses.
func destroyRememberLKI(ps *params, in *lInstr) error {
	_, lki := ps.get("RememberLKI")
	_, always := ps.get("AlwaysRemember")
	if !lki && !always {
		return nil
	}
	if !ps.is("RememberLKI", "True") || !ps.is("AlwaysRemember", "True") || in.Obj.Kind != rTarget {
		return refuse("Destroy RememberLKI$/AlwaysRemember$ shape")
	}
	in.Note = rememberLKINote
	return nil
}

// rememberedLKIRef resolves ConditionDefined$ RememberedLKI to the target
// of the chain's remember-LKI Destroy (the only remembered object; gorge's
// LKI copy is matched as the kernel matches a moved target, its card in
// the new zone with its last known controller). ok=false: not that word,
// or no such Destroy in the chain.
func (lc *lowerCtx) rememberedLKIRef(d string) (lRef, bool, error) {
	if strings.TrimSpace(d) != "RememberedLKI" {
		return lRef{}, false, nil
	}
	for i := len(lc.ab.Prog) - 1; i >= 0; i-- {
		if in := lc.ab.Prog[i]; in.Op == "Destroy" && in.Note == rememberLKINote {
			return in.Obj, true, nil
		}
	}
	// no remember-LKI Destroy: defined() reads it as the remembered set
	// with a RememberLKI$ move's captured controller
	return lRef{}, false, nil
}
