package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// on the facestransform pool: a ChangeZone Transformed$
// True return enters the back face (CR 712.14a), a Mode$ Transformed
// trigger follows a battlefield flip (CR 701.27a), and a back face's mana
// value is its front face's (CR 712.8e).

// ftScen builds a scenario over the facestransform pool.
func ftScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	facestransformSet := cardsettest.Set(t, "facestransform")
	for _, n := range names {
		if i, ok := facestransformSet.Lookup(n); !ok || !facestransformSet.Supported()[i] {
			t.Fatalf("%s not supported in the facestransform table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := facestransformSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// TestEnterTransformed: Garland's graveyard activation returns it to the
// battlefield transformed, entering Chaos (the back face) rather than
// Garland.
func TestEnterTransformed(t *testing.T) {
	sc, id := ftScen(t, "Garland, Knight of Cornelia", "Swamp", "Mountain")
	sc.lands(0, id["Swamp"], 5)
	sc.lands(0, id["Mountain"], 2)
	front := id["Garland, Knight of Cornelia"]
	g := sc.grave(front, 0, 1)[0]
	if !sc.activateAb(g, 0) {
		t.Fatal("Garland's graveyard activation not offered")
	}
	sc.resolveAll(t)
	if !sc.onBF(g) {
		t.Fatal("Garland did not return to the battlefield")
	}
	face := sc.k.t.Cards[front].Faces[1]
	if got := sc.s.objs[g].card; got != face {
		t.Fatalf("returned as card %d, want the back face %d", got, face)
	}
	if pw, tg := sc.s.pt(sc.k, sc.slot(g)); pw != 5 || tg != 5 {
		t.Fatalf("Chaos P/T %d/%d, want 5/5", pw, tg)
	}
	if got := sc.k.t.Cards[face].Face; got != 1 {
		t.Fatalf("back face index %d, want 1", got)
	}
}

// TestEntersTransformed (CR 712.14a): a Saga's chapter III "exile this
// Saga, then return it transformed" enters the back face through the
// MvTransformed flag, and the entering flip never fires a Mode$ Transformed
// trigger. A card entering transformed also fires the back face's own ETB,
// not the front's: Raise a Chocobo's back face has an ETB and a Transformed
// trigger, so exactly one pending trigger after the entry must be the ETB.
func TestEntersTransformed(t *testing.T) {
	sc, id := ftScen(t, "The Modern Age", "Norn's Inquisitor", "Plains")
	sc.perm(id["Norn's Inquisitor"], 0)
	o := sc.perm(id["The Modern Age"], 0)
	sl := sc.slot(o)
	front := id["The Modern Age"]
	back := sc.k.t.Cards[front].Faces[1]
	old := DebugFaceTransformStats
	DebugFaceTransformStats = &FaceTransformStats{}
	defer func() { DebugFaceTransformStats = old }()
	if !sc.s.perms[sl].setCtrN(sc.s, int(kop.CtrLore), 2) {
		t.Fatal("could not set the Saga's lore count to 2")
	}
	sc.s.npend = 0
	if !sc.s.addLore(sc.k, sl, 1) {
		t.Fatal("chapter III did not trigger")
	}
	sc.resolveAll(t)
	if !sc.onBF(o) {
		t.Fatal("the Saga did not return to the battlefield")
	}
	if got := sc.s.objs[o].card; got != back {
		t.Fatalf("returned as card %d, want the back face %d", got, back)
	}
	if DebugFaceTransformStats.Enters != 1 {
		t.Fatalf("%d MvTransformed entries, want 1", DebugFaceTransformStats.Enters)
	}
	if DebugFaceTransformStats.Checks != 0 || DebugFaceTransformStats.Fires != 0 {
		t.Fatalf("the entering flip fired a Transformed trigger: checks %d fires %d",
			DebugFaceTransformStats.Checks, DebugFaceTransformStats.Fires)
	}

	// The back face's ETB, and not its Transformed trigger, fires when a
	// card enters transformed. Drive the entry route directly: Raise a
	// Chocobo has no self-return ability.
	sc2, id2 := ftScen(t, "Sidequest: Raise a Chocobo", "Plains")
	ch := id2["Sidequest: Raise a Chocobo"]
	chBack := sc2.k.t.Cards[ch].Faces[1]
	g := sc2.grave(ch, 0, 1)[0]
	sc2.s.res.e.obj = g
	sc2.s.npend = 0
	fl := int16(kop.ZoneBattlefield) | int16(kop.MvTransformed)<<3
	if sc2.s.execReturn(sc2.k, &kop.Instr{Op: kop.OpReturnEx, Obj: kop.RefSelf, C: fl}, NoSlot) {
		t.Fatal("execReturn asked for an Aura host")
	}
	if got := sc2.s.objs[g].card; got != chBack {
		t.Fatalf("entered as card %d, want the back face %d", got, chBack)
	}
	if sc2.s.npend != 0 {
		t.Fatalf("%d pending triggers after the entering flip, want none (the Transformed trigger is off-battlefield)", sc2.s.npend)
	}
	// The back face's own ETB is subscribed: a land entering now fires its
	// landfall ETB. The front face has no such trigger.
	land := sc2.obj(id2["Plains"], 0)
	sc2.s.enter(sc2.k, land, 0, false, kop.ZoneNone)
	if sc2.s.npend != 1 {
		t.Fatalf("%d pending triggers after a land entered, want the back face's ETB", sc2.s.npend)
	}
	pt := &sc2.s.pend[0]
	if pt.card != chBack {
		t.Fatalf("pending trigger from card %d, want the back face %d", pt.card, chBack)
	}
	if ev := sc2.k.t.Cards[pt.card].Abilities[pt.ab].Trig.Event; ev != kop.TrigETB {
		t.Fatalf("pending trigger event %d, want the back face's ETB", ev)
	}
}

// TestEntersTransformedAuraAskIdempotent: auraEntry runs after
// enterTransformed in execReturn, and an answered enchant ask re-enters
// execReturn; the Face != 0 guard must leave the object on its back face
// rather than flipping it a second time. No carrier in the pool has an Aura
// back face, so the guard is driven directly (it is what auraEntry relies
// on).
func TestEntersTransformedAuraAskIdempotent(t *testing.T) {
	sc, id := ftScen(t, "Garland, Knight of Cornelia", "Plains")
	front := id["Garland, Knight of Cornelia"]
	g := sc.grave(front, 0, 1)[0]
	back := sc.k.t.Cards[front].Faces[1]
	sc.s.enterTransformed(sc.k, g)
	if got := sc.s.objs[g].card; got != back {
		t.Fatalf("first entry card %d, want the back face %d", got, back)
	}
	sc.s.enterTransformed(sc.k, g)
	if got := sc.s.objs[g].card; got != back {
		t.Fatalf("second entry card %d, want the back face %d (the guard must be idempotent)", got, back)
	}
}

// TestTransformFiresTransformedTrigger: an Incubator token transforms into
// a Phyrexian and Norn's Inquisitor's Mode$ Transformed trigger puts a
// +1/+1 counter on it.
func TestTransformFiresTransformedTrigger(t *testing.T) {
	sc, id := ftScen(t, "Norn's Inquisitor", "Converter Beast", "Forest")
	norn := sc.perm(id["Norn's Inquisitor"], 0)
	sc.lands(0, id["Forest"], 6)
	cb := sc.hand(id["Converter Beast"], 0)
	sc.castCard(t, cb) // ETB: Incubate 2
	sc.resolveAll(t)
	var tok Obj = NoObj
	for o := Obj(0); int(o) < int(sc.s.nobj); o++ {
		ob := &sc.s.objs[o]
		if ob.zone == kop.ZoneBattlefield && sc.k.t.Cards[ob.card].Name == "incubator_c_0_0_a_phyrexian" {
			tok = o
			break
		}
	}
	if tok == NoObj {
		t.Fatal("Converter Beast made no Incubator token")
	}
	sl := sc.slot(tok)
	before := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1))
	if !sc.activateAb(tok, 0) {
		t.Fatal("the Incubator's transform activation not offered")
	}
	sc.resolveAll(t)
	face := sc.k.t.Cards[sc.s.objs[tok].card].Face
	if face != 1 {
		t.Fatalf("the Incubator is on face %d, want 1", face)
	}
	if n := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != before+1 {
		t.Fatalf("the transformed token has %d +1/+1 counters, want %d (Norn's trigger)", n, before+1)
	}
	if !sc.onBF(norn) {
		t.Fatal("Norn's Inquisitor left the battlefield")
	}
}

// TestBackFaceManaValueIsFront (CR 712.8e): a transforming DFC's back face
// has its front face's mana value. Treasure Map (MV 2) turns into Treasure
// Cove, a land whose own printed cost is empty; the prep table gives it MV 2,
// so a PredCMC filter, a Count and a target all read 2. The Restoration of
// Eiganjo (MV 3) is out of reach of a mana value 2 or less filter; Okiba
// Reckoner Raid (MV 1) is in reach, so the value is the front's, not a
// constant.
func TestBackFaceManaValueIsFront(t *testing.T) {
	sc, id := ftScen(t, "Treasure Map", "The Restoration of Eiganjo", "Okiba Reckoner Raid", "Plains")
	front := id["Treasure Map"]
	back := sc.k.t.Cards[front].Faces[1]
	if sc.k.t.Cards[back].Types&kop.TLand == 0 {
		t.Fatalf("Treasure Map back face types %#x, want a land (Treasure Cove)", sc.k.t.Cards[back].Types)
	}
	if got := sc.k.cards[back].total; got != 2 {
		t.Fatalf("Treasure Cove MV %d, want the front face's 2", got)
	}
	cove := kop.Filter{Types: kop.TLand, Pred: kop.PredCMCGE, PredN: 2}
	sc.k.t.Filters = append(sc.k.t.Filters, cove)
	fid := kop.FilterID(len(sc.k.t.Filters) - 1)
	o := sc.perm(back, 0)
	sl := sc.slot(o)
	if !sc.s.predPerm(sc.k, &cove, sl) {
		t.Error("Treasure Cove does not match a land with mana value >= 2 (its front cost)")
	}
	if n := sc.s.countPerm(sc.k, fid, evalCtx{ctrl: 0}); n != 1 {
		t.Errorf("Count(land MV >= 2) = %d, want 1", n)
	}
	spec := kop.TargetSpec{Kind: kop.TgtObject, Filter: fid}
	if !sc.s.targetOK(sc.k, &spec, 0, NoSlot, NoObj, TCode(o)) {
		t.Error("a land with mana value >= 2 is not a legal target for the MV >= 2 filter")
	}
	le2 := kop.Filter{Types: kop.TCreature, Pred: kop.PredCMCLE, PredN: 2}
	for _, tc := range []struct {
		name string
		mv   uint8
		ok   bool
	}{
		{"The Restoration of Eiganjo", 3, false},
		{"Okiba Reckoner Raid", 1, true},
	} {
		front := id[tc.name]
		back := sc.k.t.Cards[front].Faces[1]
		if got := sc.k.cards[back].total; got != tc.mv {
			t.Errorf("%s back face MV %d, want the front face's %d", tc.name, got, tc.mv)
		}
		o := sc.perm(back, 0)
		if got := sc.s.predPerm(sc.k, &le2, sc.slot(o)); got != tc.ok {
			t.Errorf("%s: PredCMCLE2 = %v, want %v", tc.name, got, tc.ok)
		}
	}
}

// TestCopyOfBackFaceMV0 pins the copy-of-a-back-face gap: gorge gives a
// copy permanent the copied face's printed mana value (0 for a back face),
// while the kernel reads the copied card's prep total (the front value,
// CR 712.8e). No table in scope has both a transforming back face and a
// copy program, so the difference is unreachable: the facestransform pool
// has no copy op, and FDN/BLB have no transforming back face.
func TestCopyOfBackFaceMV0(t *testing.T) {
	scan := func(name string, n int, def func(int) *kop.CardDef) (backs, copies int) {
		for i := 0; i < n; i++ {
			d := def(i)
			if d.FaceMode == kop.FaceTransform && d.Face != 0 {
				backs++
			}
			for _, a := range d.Abilities {
				for _, in := range a.Prog {
					if in.Op == kop.OpBecomeCopy || in.Op == kop.OpCopyAsToken || in.Op == kop.OpCopyAsTokenEx {
						copies++
					}
				}
			}
		}
		return backs, copies
	}
	facestransformSet := cardsettest.Set(t, "facestransform")
	if backs, copies := scan("facestransform", facestransformSet.NumCards()+facestransformSet.NumTokens(), facestransformSet.Def); backs == 0 || copies != 0 {
		t.Errorf("facestransform: %d back faces, %d copy programs, want back faces and no copy program", backs, copies)
	}
	fdnSet := cardsettest.Set(t, "fdn")
	blbSet := cardsettest.Set(t, "blb")
	for _, p := range []struct {
		name string
		n    int
		def  func(int) *kop.CardDef
	}{
		{"fdn", fdnSet.NumCards(), fdnSet.Def},
		{"blb", blbSet.NumCards(), blbSet.Def},
	} {
		if backs, _ := scan(p.name, p.n, p.def); backs != 0 {
			t.Errorf("%s: %d transforming back faces, want none (the copy programs there must not meet one)", p.name, backs)
		}
	}
}

// TestTransformCardRestamps: a non-token transforming DFC flips in place,
// gets a new layer timestamp (CR 613.7g), and keeps its counters, damage
// and tapped state (CR 701.27c).
func TestTransformCardRestamps(t *testing.T) {
	sc, id := ftScen(t, "Dowsing Device", "Plains")
	o := sc.perm(id["Dowsing Device"], 0)
	sl := sc.slot(o)
	sc.s.perms[sl].setCtrN(sc.s, int(kop.CtrP1P1), 2)
	sc.s.perms[sl].flags |= pfTapped
	sc.s.perms[sl].damage = 3
	oldStamp := sc.s.lstamp[sl]
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	front := id["Dowsing Device"]
	back := sc.k.t.Cards[front].Faces[1]
	if got := sc.s.objs[o].card; got != back {
		t.Fatalf("card %d, want the back face %d", got, back)
	}
	if sc.s.lstamp[sl] == oldStamp {
		t.Fatal("the layer timestamp did not move")
	}
	if n := sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)); n != 2 {
		t.Errorf("counters %d, want 2", n)
	}
	if sc.s.perms[sl].damage != 3 || sc.s.perms[sl].flags&pfTapped == 0 {
		t.Errorf("damage %d tapped %v, want 3 and tapped", sc.s.perms[sl].damage, sc.s.perms[sl].flags&pfTapped != 0)
	}
}

// TestTransformCardFaceResetOnLeave: a transformed card leaving the
// battlefield is front face up (CR 712.8a).
func TestTransformCardFaceResetOnLeave(t *testing.T) {
	sc, id := ftScen(t, "Dowsing Device", "Plains")
	o := sc.perm(id["Dowsing Device"], 0)
	sl := sc.slot(o)
	front := id["Dowsing Device"]
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if sc.s.objs[o].card == front {
		t.Fatal("did not transform")
	}
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if got := sc.s.objs[o].card; got != front {
		t.Fatalf("left as card %d, want the front face %d", got, front)
	}
	if got := sc.s.objs[o].zone; got != kop.ZoneGraveyard {
		t.Fatalf("zone %d, want graveyard", got)
	}
}

// TestTransformedTriggerFires: Sidequest: Raise a Chocobo's back-face
// Mode$ Transformed trigger follows a battlefield flip.
func TestTransformedTriggerFires(t *testing.T) {
	sc, id := ftScen(t, "Sidequest: Raise a Chocobo", "Forest")
	o := sc.perm(id["Sidequest: Raise a Chocobo"], 0)
	old := DebugFaceTransformStats
	DebugFaceTransformStats = &FaceTransformStats{}
	defer func() { DebugFaceTransformStats = old }()
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if DebugFaceTransformStats.Fires != 1 {
		t.Fatalf("queued %d Transformed triggers, want 1", DebugFaceTransformStats.Fires)
	}
}

// TestTransformedTriggerNotOffBattlefield: the same flip in a graveyard
// is a no-op and fires nothing (CR 701.27a).
func TestTransformedTriggerNotOffBattlefield(t *testing.T) {
	sc, id := ftScen(t, "Sidequest: Raise a Chocobo", "Forest")
	o := sc.grave(id["Sidequest: Raise a Chocobo"], 0, 1)[0]
	old := DebugFaceTransformStats
	DebugFaceTransformStats = &FaceTransformStats{}
	defer func() { DebugFaceTransformStats = old }()
	sc.s.res.e.obj = o
	sc.s.transform(sc.k, &kop.Instr{Op: kop.OpTransform, Obj: kop.RefSelf})
	if got := sc.s.objs[o].card; got != id["Sidequest: Raise a Chocobo"] {
		t.Fatalf("a graveyard card turned to %d", got)
	}
	if DebugFaceTransformStats.Checks != 0 || DebugFaceTransformStats.Fires != 0 {
		t.Fatalf("off-battlefield flip fired: checks %d fires %d", DebugFaceTransformStats.Checks, DebugFaceTransformStats.Fires)
	}
}

// TestEnterTransformedFiresNoTrigger: the enter-transformed flip happens
// off the battlefield, so it never reaches fireTransformed (CR 712.14a,
// gorge trigmatch/transformed.go: the object is not on the battlefield at
// match time).
func TestEnterTransformedFiresNoTrigger(t *testing.T) {
	sc, id := ftScen(t, "Garland, Knight of Cornelia", "Norn's Inquisitor", "Swamp", "Mountain")
	sc.perm(id["Norn's Inquisitor"], 0)
	sc.lands(0, id["Swamp"], 5)
	sc.lands(0, id["Mountain"], 2)
	g := sc.grave(id["Garland, Knight of Cornelia"], 0, 1)[0]
	old := DebugFaceTransformStats
	DebugFaceTransformStats = &FaceTransformStats{}
	defer func() { DebugFaceTransformStats = old }()
	if !sc.activateAb(g, 0) {
		t.Fatal("Garland's graveyard activation not offered")
	}
	sc.resolveAll(t)
	if DebugFaceTransformStats.Enters != 1 {
		t.Fatalf("%d MvTransformed entries, want 1", DebugFaceTransformStats.Enters)
	}
	if DebugFaceTransformStats.Checks != 0 || DebugFaceTransformStats.Fires != 0 {
		t.Fatalf("entering transformed fired a trigger: checks %d fires %d", DebugFaceTransformStats.Checks, DebugFaceTransformStats.Fires)
	}
}
