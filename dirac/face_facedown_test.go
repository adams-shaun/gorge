package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newKfacedownScen is newScenTable over the facedown pool: one of each
// carrier (Kolaghan Stormsinger is the Megamorph, Bolrac-Clan Basher and
// Basilica Stalker the expensive Disguises) and a library of Zoetic Caverns.
func newKfacedownScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Kolaghan Stormsinger", Count: 1},
		{Name: "Bolrac-Clan Basher", Count: 1},
		{Name: "Basilica Stalker", Count: 1},
		{Name: "Zoetic Cavern", Count: 20},
	}
	set := cardsettest.Set(t, "facedown")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib := make([]CardID, 20)
	for i := range lib {
		lib[i] = id["Zoetic Cavern"]
	}
	s := new(State)
	s.Init(tbl, lib, lib, 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// A face-down spell on the stack has no name, mana value or colour
// (CR 708.4; gorge's Object.FaceDownHidden). TgtSpell must pass the stack
// object to the filter matcher, or a spell like "counter target spell named
// Kolaghan Stormsinger" would see the printed card.
func TestFaceDownStackSpellHidesPrintedCharacteristics(t *testing.T) {
	sc, id := newKfacedownScen(t)
	kol := id["Kolaghan Stormsinger"]
	o := sc.obj(kol, 0)
	sc.s.objs[o].zone = kop.ZoneStack
	sc.s.objs[o].slot = slotFaceDownSpell // as pushBuild marks a face-down cast
	sc.s.push(stackEntry{kind: seSpell, obj: o, card: kol, ab: 255, mode: seFaceDown | kop.MorphMegamorph, ctrl: 0})

	add := func(f kop.Filter) kop.FilterID {
		sc.k.t.Filters = append(sc.k.t.Filters, f)
		return kop.FilterID(len(sc.k.t.Filters) - 1)
	}
	named := add(kop.Filter{Pred: kop.PredNamed, PredN: int16(kol)})
	red := add(kop.Filter{Colors: kop.ColR})
	mv1 := add(kop.Filter{Pred: kop.PredCMCEQ, PredN: 1})
	for _, tc := range []struct {
		name string
		f    kop.FilterID
	}{
		{"name", named},
		{"colour", red},
		{"mana value", mv1},
	} {
		if !sc.s.matchCardGen(sc.k, tc.f, kol, 0, 0, NoObj) {
			t.Errorf("printed %s does not match the face-up card", tc.name)
		}
		if sc.s.matchCardAt(sc.k, tc.f, kol, 0, 0, o) {
			t.Errorf("face-down stack spell matched its printed %s", tc.name)
		}
	}
	spec := kop.TargetSpec{Kind: kop.TgtSpell, Filter: named}
	if sc.s.targetOK(sc.k, &spec, 0, NoSlot, NoObj, TCode(o)) {
		t.Error("TgtSpell accepted a face-down spell by its printed name")
	}
	sc.s.stack[0].mode = 0 // face up: the printed name is visible again
	sc.s.objs[o].slot = NoSlot
	if !sc.s.targetOK(sc.k, &spec, 0, NoSlot, NoObj, TCode(o)) {
		t.Error("TgtSpell rejected a face-up spell by its printed name")
	}
}

// The turn-face-up special action must be offered for an expensive
// Disguise carrier when its Morph cost is floating, not only for the cheap
// Morph/Megamorph cards.
func TestFaceDownDisguiseTurnUpOffered(t *testing.T) {
	sc, id := newKfacedownScen(t)
	for _, tc := range []struct {
		name string
		pool [kop.NumManaKinds]uint8 // W U B R G C
	}{
		{"Bolrac-Clan Basher", [kop.NumManaKinds]uint8{0, 0, 0, 2, 0, 3}},
		{"Basilica Stalker", [kop.NumManaKinds]uint8{0, 0, 1, 0, 0, 4}},
	} {
		o := sc.obj(id[tc.name], 0)
		sc.s.enterFaceDown(sc.k, o, 0, kop.MorphDisguise)
		sc.s.pl[0].pool = tc.pool
		var d Decision
		sc.s.offerTurnFaceUp(sc.k, &d, 0)
		if d.N == 0 {
			t.Errorf("%s: turn-face-up not offered with %v floating", tc.name, tc.pool)
		}
	}
}

// A face-down permanent's base is a 2/2 colourless creature, but a
// type-setting or P/T-setting effect still applies over it (CR 708.2,
// 613.1d, 613.4). The face-down types ride on ovGenTy (typesAt reads gty)
// and the rest comes from the walk (typesGen, basePTGen); an override
// record applies over the base.
func TestFaceDownTypeAndPTOverrideOrder(t *testing.T) {
	sc, id := newKfacedownScen(t)
	o := sc.obj(id["Kolaghan Stormsinger"], 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphMegamorph)
	sc.s.perms[sl].flags &^= pfSick
	if got := sc.s.typesAt(sc.k, sl); got != kop.TCreature {
		t.Fatalf("face-down types = %#x, want creature", got)
	}
	if got := sc.s.typesGen(sc.k, sl); got != kop.TCreature {
		t.Fatalf("face-down typesGen = %#x, want creature", got)
	}
	if bp, bt := sc.s.basePTGen(sc.k, sl); bp != 2 || bt != 2 {
		t.Fatalf("face-down base P/T = %d/%d, want 2/2", bp, bt)
	}
	if c := sc.s.colorsAt(sc.k, sl); c != 0 {
		t.Fatalf("face-down colours = %#x, want colourless", c)
	}
	p := &sc.s.perms[sl]
	p.ovr.flags |= ovTypes
	p.ovr.types = kop.TLand
	p.ovr.flags |= ovSetPT
	p.ovr.setP, p.ovr.setT = 3, 4
	sc.s.refreshTy(sc.k)
	if got := sc.s.typesAt(sc.k, sl); got != kop.TLand {
		t.Fatalf("face-down types with a type override = %#x, want land", got)
	}
	if bp, bt := sc.s.basePTGen(sc.k, sl); bp != 3 || bt != 4 {
		t.Fatalf("face-down base P/T with a set-P/T override = %d/%d, want 3/4", bp, bt)
	}
}

// newKfacedownPoolScen is newScenTable over the whole pool, including the
// layer-6 statics (Humility) the ward-suppression test needs.
func newKfacedownPoolScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "facedown")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	id := make(map[string]CardID, set.NumCards())
	for i := 0; i < set.NumCards(); i++ {
		id[set.Names()[i]] = set.PoolID(i)
	}
	return sc, id
}

func kfacedownWardTrigger(t *testing.T, s *State, src, target Obj) {
	t.Helper()
	e := stackEntry{kind: seSpell, obj: src, ctrl: 1, nt: 1}
	e.tgt[0] = TCode(target)
	s.checkWard(s.k(), &e)
}

// A face-down Disguise permanent has ward {2} from its base keywords
// (CR 702.168b). A face-down permanent's keywords come from the layer-6
// walk (its ovr.walk flag: no hot accessor tests face-down), which must
// grant it, and checkWard must read the keyword rather than assuming every
// Disguise permanent still has it.
func TestFaceDownDisguiseWardFastPath(t *testing.T) {
	sc, id := newKfacedownScen(t)
	o := sc.obj(id["Basilica Stalker"], 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphDisguise)
	if !sc.s.l6On(sc.k, sl) {
		t.Fatal("a face-down permanent's keywords did not come from the walk")
	}
	if got := sc.s.kw(sc.k, sl); got&kop.KwWard == 0 {
		t.Fatalf("face-down Disguise keywords %#x lack ward", got)
	}
	kol := sc.obj(id["Kolaghan Stormsinger"], 1)
	ksl := sc.s.enterFaceDown(sc.k, kol, 1, kop.MorphMegamorph)
	if got := sc.s.kw(sc.k, ksl); got&kop.KwWard != 0 {
		t.Fatalf("face-down Megamorph keywords %#x have ward", got)
	}
}

// A manifested Disguise card is face down because of manifest (CR 701.40a),
// not a disguise cast; CR 701.40d gives it only the turn-up procedures, so
// it has no ward {2} (CR 702.168a-b). Before the fix kw6 read the card's
// MorphFamily and stamped KwWard, diverging at
// projection Cards[n].Chars.Keywords [ref=0 kernel=131072].
func TestFaceDownManifestedDisguiseNoWard(t *testing.T) {
	sc, id := newKfacedownScen(t)
	o := topCard(sc, id["Basilica Stalker"])
	sc.s.res.e.ctrl = 0
	in := kop.Instr{Op: kop.OpManifest, Obj: kop.RefController, A: kop.Const(1)}
	if !sc.s.exec(sc.k, &in) {
		t.Fatal("Manifest suspended")
	}
	sl := sc.s.objs[o].slot
	if !sc.s.faceDown(sc.k, sl) || !sc.s.manifested(sl) {
		t.Fatalf("not a manifested face-down permanent (zone %d)", sc.s.objs[o].zone)
	}
	if got := sc.s.kw(sc.k, sl); got&kop.KwWard != 0 {
		t.Fatalf("manifested Basilica Stalker keywords %#x have ward; CR 701.40a lists no ward", got)
	}
}

// The Disguise ward is a base keyword, so a layer-6 "loses all abilities"
// effect removes it, and a face-up permanent no longer has it at all.
func TestFaceDownDisguiseWardSuppressed(t *testing.T) {
	sc, id := newKfacedownPoolScen(t)
	basilica := id["Basilica Stalker"]
	o := sc.obj(basilica, 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphDisguise)
	if got := sc.s.kw(sc.k, sl); got&kop.KwWard == 0 {
		t.Fatalf("face-down Disguise keywords %#x lack ward", got)
	}
	kfacedownWardTrigger(t, sc.s, sc.obj(id["Shock"], 1), o)
	if n := sc.s.NumPending(); n != 1 {
		t.Fatalf("ward triggered %d times, want 1", n)
	}
	if _, card, ab, _, _ := sc.s.PendingTrigger(0); card != basilica || ab != WardAbility {
		t.Fatalf("pending trigger card=%d ab=%d, want Basilica Stalker's ward", card, ab)
	}
	if got := sc.s.PendingTriggerAmount(0); got != 2 {
		t.Fatalf("ward amount = %d, want 2", got)
	}

	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{0, 0, 1, 0, 0, 4}
	sc.s.turnFaceUp(sc.k, o, 0)
	if sc.s.faceDown(sc.k, sl) {
		t.Fatal("Basilica Stalker stayed face down after the turn-face-up")
	}
	if got := sc.s.kw(sc.k, sl); got&kop.KwWard != 0 {
		t.Fatalf("face-up Basilica Stalker keywords %#x still have ward", got)
	}
	sc.s.npend = 0
	kfacedownWardTrigger(t, sc.s, sc.obj(id["Shock"], 1), o)
	if n := sc.s.NumPending(); n != 0 {
		t.Fatalf("face-up Basilica Stalker triggered ward %d times", n)
	}

	o2 := sc.obj(basilica, 0)
	sl2 := sc.s.enterFaceDown(sc.k, o2, 0, kop.MorphDisguise)
	sc.perm(id["Humility"], 1)
	if !sc.s.faceDown(sc.k, sl2) {
		t.Fatal("Humility turned the face-down permanent face up")
	}
	if got := sc.s.kw(sc.k, sl2); got&kop.KwWard != 0 {
		t.Fatalf("face-down Disguise keywords %#x keep ward under Humility", got)
	}
	sc.s.npend = 0
	kfacedownWardTrigger(t, sc.s, sc.obj(id["Shock"], 1), o2)
	if n := sc.s.NumPending(); n != 0 {
		t.Fatalf("Humility did not suppress the ward (%d triggers)", n)
	}
}

// Count$Valid ...$Colors folds the matches' ColorMaskOf, so a face-down
// permanent contributes no colours (CR 708.2) even though its printed face
// is red.
func TestFaceDownAggColors(t *testing.T) {
	sc, id := newKfacedownScen(t)
	down := sc.obj(id["Kolaghan Stormsinger"], 0)
	sc.s.enterFaceDown(sc.k, down, 0, kop.MorphMegamorph)
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(sc.k.t.Filters) - 1)
	agg := func() int {
		x := kop.Expr{Kind: kop.ExValidAgg, N: kop.AggColors, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if got := agg(); got != 0 {
		t.Fatalf("face-down permanent contributed %d colours, want 0", got)
	}
	sc.perm(id["Kolaghan Stormsinger"], 0)
	if got := agg(); got != 1 {
		t.Fatalf("face-up red plus face-down permanent = %d colours, want 1", got)
	}
}

// A face-down noncreature card (Zoetic Cavern is a land) turned face up
// while attacking stops being a creature, so the next state-based check
// removes it from combat (CR 506.4; gorge's SBA pass, combat_removal.go).
// The pool's Humility changes no types, so the face-down gate must arm the
// check itself (prepFaceDown: tyCombat).
func TestFaceDownTurnUpLeavesCombat(t *testing.T) {
	sc, id := newKfacedownPoolScen(t)
	o := sc.obj(id["Zoetic Cavern"], 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphMorph)
	p := &sc.s.perms[sl]
	p.flags &^= pfSick
	p.flags |= pfAttacking | pfBlocked
	sc.s.nAttackers, sc.s.declared = 1, true
	if !sc.k.gen.tyCombat {
		t.Fatal("a face-down table does not arm the CR 506.4 check")
	}
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{0, 0, 0, 0, 0, 2}
	sc.s.turnFaceUp(sc.k, o, 0)
	if sc.s.faceDown(sc.k, sl) {
		t.Fatal("Zoetic Cavern stayed face down")
	}
	if ty := sc.s.typesAt(sc.k, sl); ty&kop.TCreature != 0 || ty&kop.TLand == 0 {
		t.Fatalf("face-up Zoetic Cavern types %#x, want a noncreature land", ty)
	}
	sc.s.sba(sc.k)
	if p.flags&pfAttacking != 0 || sc.s.nAttackers != 0 {
		t.Fatalf("face-up noncreature still attacking (flags %#x, attackers %d)", p.flags, sc.s.nAttackers)
	}
}

// Turning a permanent face up changes it, so it must be marked (marks.go:
// every mutation marks) for every family, not only Megamorph's counter:
// ProjectDelta and the cached view hashes otherwise reuse the face-down
// object.
func TestFaceDownTurnUpMarks(t *testing.T) {
	sc, id := newKfacedownPoolScen(t)
	for _, tc := range []struct {
		name string
		fam  uint8
		pool [kop.NumManaKinds]uint8
	}{
		{"Zoetic Cavern", kop.MorphMorph, [kop.NumManaKinds]uint8{0, 0, 0, 0, 0, 2}},
		{"Basilica Stalker", kop.MorphDisguise, [kop.NumManaKinds]uint8{0, 0, 1, 0, 0, 4}},
	} {
		o := sc.obj(id[tc.name], 0)
		sc.s.enterFaceDown(sc.k, o, 0, tc.fam)
		h0 := sc.s.ObjHash(o)
		sc.s.SetEpoch(7)
		t0 := sc.s.Touches()
		sc.s.pl[0].pool = tc.pool
		sc.s.turnFaceUp(sc.k, o, 0)
		if sc.s.faceDown(sc.k, sc.s.objs[o].slot) {
			t.Fatalf("%s stayed face down", tc.name)
		}
		if !sc.s.Changed(o) || !sc.s.ChangedSince(o, t0) {
			t.Errorf("%s: turnFaceUp did not mark the permanent", tc.name)
		}
		if sc.s.ObjHash(o) == h0 {
			t.Errorf("%s: the object hash did not change", tc.name)
		}
	}
}

// A face-down spell's stack mark (slotFaceDownSpell) is how faceDownAt
// sees it without a stack walk; the card leaving the stack for any zone
// drops it.
func TestFaceDownSpellStackMark(t *testing.T) {
	sc, id := newKfacedownScen(t)
	o := sc.obj(id["Kolaghan Stormsinger"], 0)
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: id["Kolaghan Stormsinger"], ab: 255, mode: seFaceDown | kop.MorphMegamorph}}
	sc.s.pushBuild(sc.k)
	if !sc.s.faceDownAt(sc.k, o) {
		t.Fatal("a face-down spell pushed to the stack is not face down")
	}
	sc.s.sp--
	sc.s.objs[o].zone = kop.ZoneNone
	sc.s.place(o, kop.ZoneGraveyard, false)
	if sc.s.objs[o].slot != NoSlot || sc.s.faceDownAt(sc.k, o) {
		t.Fatalf("the countered card kept the face-down mark (slot %d)", sc.s.objs[o].slot)
	}
}

// a face-down object has no mana cost (CR 708.2, 708.4),
// so PredHasXCost (gorge `hasXCost`) is false for it on the stack and on the
// battlefield even when its printed cost has {X} (Hooded Hydra, X G G).
func TestFaceDownHasNoXCost(t *testing.T) {
	sc, id := newKfacedownScen(t)
	kol := id["Kolaghan Stormsinger"]
	defer func(x uint8) { sc.k.cards[kol].cost.X = x }(sc.k.cards[kol].cost.X)
	sc.k.cards[kol].cost.X = 1 // a printed {X}, as Hooded Hydra's
	sc.k.t.Filters = append(sc.k.t.Filters, kop.Filter{Pred: kop.PredHasXCost})
	hasX := kop.FilterID(len(sc.k.t.Filters) - 1)

	o := sc.obj(kol, 0)
	if !sc.s.matchCardAt(sc.k, hasX, kol, 0, 0, o) {
		t.Fatal("a face-up card with {X} in its cost does not match hasXCost")
	}
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, obj: o, card: kol, ab: 255, mode: seFaceDown | kop.MorphMegamorph}}
	sc.s.pushBuild(sc.k)
	if !sc.s.faceDownAt(sc.k, o) {
		t.Fatal("the pushed face-down spell is not face down")
	}
	if sc.s.matchCardAt(sc.k, hasX, kol, 0, 0, o) {
		t.Error("a face-down spell matched hasXCost by its printed {X}")
	}
	sc.s.sp--
	sc.s.objs[o].zone = kop.ZoneNone
	sc.s.place(o, kop.ZoneGraveyard, false)

	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphMegamorph)
	if sc.s.matchPermGen(sc.k, hasX, sl, 0, NoSlot, layerFull) {
		t.Error("a face-down permanent matched hasXCost by its printed {X}")
	}
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{0, 0, 0, 1, 0, 0} // megamorph {R}
	sc.s.turnFaceUp(sc.k, o, 0)
	if sc.s.faceDown(sc.k, sl) {
		t.Fatal("Kolaghan Stormsinger stayed face down")
	}
	if !sc.s.matchPermGen(sc.k, hasX, sl, 0, NoSlot, layerFull) {
		t.Error("the turned-up permanent lost its printed {X}")
	}
}
