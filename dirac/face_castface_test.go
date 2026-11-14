package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Casting a face (face_castface.go) on the generated faces pool: an
// Adventure from the hand into exile and its front from there, a split
// card's right half, the face turned back on an undo and on every stack
// exit, and a split card's mana value off the stack.

// faceCasts lists the face casts (AkCast with castFace) offered for o:
// each one's face (the action's target).
func faceCasts(d *Decision, o Obj) []uint8 {
	var out []uint8
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) && a.Mode()&castFace != 0 {
			out = append(out, uint8(a.Target()))
		}
	}
	return out
}

// buildToStack answers the cast being built (its first option each time)
// until priority comes back with the spell on the stack.
func (sc *scen) buildToStack(t *testing.T) {
	t.Helper()
	for i := 0; i < 20; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && sc.s.ctl == ctlPriority {
			return
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("the cast never finished")
}

// TestAdventureCastAndRecast: Murderous Rider offers its Adventure (Swift
// End) from the hand; cast, it is the Adventure on the stack, and
// resolving it exiles the card as Murderous Rider on an adventure (gorge
// spellRestZone, CR 715.4), whose front is then cast from exile (gorge
// "adventure_recast") and enters the battlefield.
func TestAdventureCastAndRecast(t *testing.T) {
	sc, id := facesScen(t, "Murderous Rider", "Grizzly Bears", "Swamp")
	// The oracle surface: both the Adventure from the hand and the recast
	// from exile are priced from the untapped Swamps, not the empty pool.
	sc.s.cfg |= cfgGorgeUnpriced
	front := id["Murderous Rider"]
	face := sc.k.t.Cards[front].Faces[1]
	if sc.k.t.Cards[face].FaceMode != kop.FaceAdventure || sc.k.t.Cards[face].Face != 1 {
		t.Fatal("Swift End is not the Adventure face")
	}
	for i := 0; i < 6; i++ {
		sc.perm(id["Swamp"], 0)
	}
	gb := sc.perm(id["Grizzly Bears"], 1)
	mr := sc.hand(front, 0)
	d := sc.s.Step()
	if got := faceCasts(&d, mr); len(got) != 1 || got[0] != 1 {
		t.Fatalf("face casts %v, want [1]", got)
	}
	sc.s.Apply(MakeAction(AkCast, TCode(mr), 1, castFace))
	sc.s.Step() // pushed (CR 601.2a), then its target asked
	if sc.s.objs[mr].card != face || sc.s.objs[mr].zone != kop.ZoneStack {
		t.Fatal("the Adventure is not on the stack")
	}
	sc.resolveAll(t)
	if sc.onBF(gb) {
		t.Fatal("Swift End did not destroy Grizzly Bears")
	}
	ob := &sc.s.objs[mr]
	if ob.zone != kop.ZoneExile || ob.card != front || ob.of&ofAdventure == 0 {
		t.Fatalf("after resolving: zone %d card front %v on an adventure %v", ob.zone, ob.card == front, ob.of&ofAdventure != 0)
	}
	d = sc.s.Step()
	if got := faceCasts(&d, mr); len(got) != 1 || got[0] != 0 {
		t.Fatalf("recasts %v, want [0]", got)
	}
	DebugFacePoolOnly = true
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 0 {
		t.Fatal("recast offered with the pool-only mutation and an empty pool")
	}
	DebugFacePoolOnly = false
	sc.s.Apply(MakeAction(AkCast, TCode(mr), 0, castFace))
	sc.s.Step()
	if sc.s.objs[mr].of&ofAdventure != 0 || sc.s.objs[mr].zone != kop.ZoneStack {
		t.Fatal("still on an adventure off exile")
	}
	sc.resolveAll(t)
	if !sc.onBF(mr) || sc.s.cardAt(sc.slot(mr)) != front {
		t.Fatal("Murderous Rider did not enter from exile")
	}
}

// TestAdventureCounteredToGraveyard: an Adventure countered on the stack
// goes to its owner's graveyard as its front, not on an adventure (gorge
// spellFizzleZone); an undone face cast returns the card to the hand as
// its front (gorge pc.faceBefore, CR 733.1).
func TestAdventureCounteredToGraveyard(t *testing.T) {
	sc, id := facesScen(t, "Murderous Rider", "Grizzly Bears", "Swamp")
	front := id["Murderous Rider"]
	for i := 0; i < 3; i++ {
		sc.perm(id["Swamp"], 0)
	}
	sc.perm(id["Grizzly Bears"], 1)
	mr := sc.hand(front, 0)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(mr), 1, castFace))
	sc.s.Step()
	sc.s.reverseCast(sc.k)
	if ob := &sc.s.objs[mr]; ob.zone != kop.ZoneHand || ob.card != front {
		t.Fatalf("undone: zone %d, front %v", ob.zone, ob.card == front)
	}
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(mr), 1, castFace))
	sc.buildToStack(t)
	if sc.s.sp != 1 {
		t.Fatalf("stack %d, want the Adventure", sc.s.sp)
	}
	sc.s.counterAt(sc.k, 0)
	if ob := &sc.s.objs[mr]; ob.zone != kop.ZoneGraveyard || ob.card != front || ob.of&ofAdventure != 0 {
		t.Fatalf("countered: zone %d, front %v, on an adventure %v", ob.zone, ob.card == front, ob.of&ofAdventure != 0)
	}
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 0 {
		t.Fatal("a face cast offered from the graveyard")
	}
}

// TestFaceCastCensus: prices every face route (Omen, Prepare,
// Sneak, Adventure, split, modal) as a potential play, so a face cast is
// offered from the mana census (the floating pool plus untapped sources).
// The pool-only mutation restores the old offer for every route.
func TestFaceCastCensus(t *testing.T) {
	defer func() { DebugFacePoolOnly = false }()
	sc, id := omenScen(t, "Dirgur Island Dragon", "Island")
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	dirgur := sc.hand(id["Dirgur Island Dragon"], 0)
	sc.s.cfg |= cfgGorgeUnpriced
	if d := sc.s.Step(); len(faceCasts(&d, dirgur)) != 1 {
		t.Fatal("omen not offered from the untapped lands and an empty pool")
	}
	DebugFacePoolOnly = true
	if d := sc.s.Step(); len(faceCasts(&d, dirgur)) != 0 {
		t.Fatal("omen offered with the pool-only mutation and an empty pool")
	}
	DebugFacePoolOnly = false
	sc.s.pl[0].pool[kop.U] = 2
	if d := sc.s.Step(); len(faceCasts(&d, dirgur)) != 1 {
		t.Fatal("omen not offered with its cost floating")
	}

	// An Adventure face, a split half and a modal back are priced the same
	// way; only the routes this code newly priced carry the planner limits.
	for _, c := range []struct {
		name  string
		lands []string
	}{
		{"Murderous Rider", []string{"Swamp", "Swamp", "Swamp"}},
		{"Fire", []string{"Island", "Island"}},
		{"Kolvori, God of Kinship", []string{"Forest", "Forest", "Forest", "Forest", "Forest", "Forest"}},
	} {
		DebugFacePoolOnly = false
		fc, fid := facesScen(t, c.name, "Grizzly Bears", c.lands[0])
		for _, l := range c.lands {
			fc.perm(fid[l], 0)
		}
		fc.perm(fid["Grizzly Bears"], 1)
		o := fc.hand(fid[c.name], 0)
		fc.s.cfg |= cfgGorgeUnpriced
		if d := fc.s.Step(); len(faceCasts(&d, o)) != 1 {
			t.Fatalf("%s: face not offered from the untapped lands and an empty pool", c.name)
		}
		DebugFacePoolOnly = true
		if d := fc.s.Step(); len(faceCasts(&d, o)) != 0 {
			t.Fatalf("%s: face offered with the pool-only mutation and an empty pool", c.name)
		}
	}
}

// TestFaceCastCensusOffSurface: off the oracle surface (the default,
// cfgGorgeUnpriced clear) a face cast is the plain rules census, so gorge's
// planner limits do not apply: Swift End is offered from three untapped
// Swamps while an artifact W floats (CR 601.2h). On the oracle surface the
// same board is still offered, an artifact-tagged unit being ordinary mana to
// the plan. So is a restricted unit the cast may not spend: it neither removes
// the offer nor funds it (gorge pay.PlanPool, CR 106.6), so two Swamps are one
// short of Swift End's {1}{B}{B}.
func TestFaceCastCensusOffSurface(t *testing.T) {
	sc, id := facesScen(t, "Murderous Rider", "Grizzly Bears", "Swamp")
	var swamp Obj
	for i := 0; i < 3; i++ {
		swamp = sc.perm(id["Swamp"], 0)
	}
	sc.perm(id["Grizzly Bears"], 1)
	mr := sc.hand(id["Murderous Rider"], 0)
	sc.s.pl[0].pool[kop.W] = 1
	sc.s.g.poolArt[0] = 1 << kop.W
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 1 {
		t.Fatal("off the oracle surface: adventure not offered from 3 Swamps with an artifact W floating")
	}
	sc.s.cfg |= cfgGorgeUnpriced
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 1 {
		t.Fatal("on the oracle surface: adventure not offered although the potential query accepts the artifact pool")
	}
	sc.s.pl[0].pool[kop.W] = 0
	sc.s.g.poolArt[0] = 0
	sc.s.setRestr(0, 1, kop.W, 1)
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 1 {
		t.Fatal("on the oracle surface: adventure not offered from 3 Swamps with restricted mana floating")
	}
	sc.s.perms[sc.slot(swamp)].flags |= pfTapped
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 0 {
		t.Fatal("on the oracle surface: restricted mana funded the adventure past 2 Swamps")
	}
}

// TestFaceCastScriptWalk: a scripted-prefix walk (availX set) does not by
// itself withhold a face cast -- the script dead-end check decides, and with
// no restricted source the census still pays. Restricted mana floating that the
// cast may not spend changes neither: the Swamps still pay, and two are short.
func TestFaceCastScriptWalk(t *testing.T) {
	sc, id := facesScen(t, "Murderous Rider", "Grizzly Bears", "Swamp")
	var swamp Obj
	for i := 0; i < 3; i++ {
		swamp = sc.perm(id["Swamp"], 0)
	}
	sc.perm(id["Grizzly Bears"], 1)
	mr := sc.hand(id["Murderous Rider"], 0)
	sc.s.cfg |= cfgGorgeUnpriced
	sc.s.g.availX = availGaps
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 1 {
		t.Fatal("adventure dropped by a scripted-prefix walk with no restricted source")
	}
	sc.s.setRestr(0, 1, kop.W, 1)
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 1 {
		t.Fatal("adventure not offered from 3 Swamps with restricted mana floating")
	}
	sc.s.perms[sc.slot(swamp)].flags |= pfTapped
	if d := sc.s.Step(); len(faceCasts(&d, mr)) != 0 {
		t.Fatal("restricted mana funded the adventure past 2 Swamps")
	}
}

// TestSplitHalfCast: Fire // Ice offers Ice from the hand (gorge
// "split_alt"); cast, Ice is on the stack, and it resolves into the
// graveyard as Fire // Ice, whose mana value there is the halves' sum
// (CR 709.4b: 2 + 2) while the front alone counts on the stack.
func TestSplitHalfCast(t *testing.T) {
	sc, id := facesScen(t, "Fire", "Grizzly Bears", "Island", "Mountain")
	front := id["Fire"]
	ice := sc.k.t.Cards[front].Faces[1]
	if !sc.k.gen.anySplit {
		t.Fatal("anySplit not set")
	}
	for i := 0; i < 2; i++ {
		sc.perm(id["Island"], 0)
		sc.perm(id["Mountain"], 0)
	}
	sc.perm(id["Grizzly Bears"], 1)
	fi := sc.hand(front, 0)
	if mv := sc.s.splitMV(sc.k, front, fi, 2); mv != 4 {
		t.Fatalf("mana value in the hand %d, want 4", mv)
	}
	d := sc.s.Step()
	if got := faceCasts(&d, fi); len(got) != 1 || got[0] != 1 {
		t.Fatalf("face casts %v, want [1]", got)
	}
	hand := sc.s.pl[0].handN
	sc.s.Apply(MakeAction(AkCast, TCode(fi), 1, castFace))
	sc.s.Step()
	if sc.s.objs[fi].card != ice {
		t.Fatal("Ice is not on the stack")
	}
	sc.resolveAll(t)
	if ob := &sc.s.objs[fi]; ob.zone != kop.ZoneGraveyard || ob.card != front {
		t.Fatalf("after resolving: zone %d, front %v", ob.zone, ob.card == front)
	}
	if sc.s.pl[0].handN != hand { // Fire // Ice left the hand, Ice drew one
		t.Fatalf("hand %d, want %d", sc.s.pl[0].handN, hand)
	}
	if mv := sc.s.splitMV(sc.k, front, fi, 2); mv != 4 {
		t.Fatalf("mana value in the graveyard %d, want 4", mv)
	}
	f2 := sc.hand(front, 0)
	sc.castMode(t, f2, 0)
	sc.s.Step()
	if mv := sc.s.splitMV(sc.k, front, f2, 2); sc.s.objs[f2].zone != kop.ZoneStack || mv != 2 {
		t.Fatalf("Fire on the stack: mana value %d, want 2", mv)
	}
}

// TestSplitCardCountsBothHalvesAsEvidence: Fuss // Bother in the graveyard is
// nine mana value toward a collect evidence cost or a ward-evidence payment,
// its halves' sum (CR 709.4b, 701.59a), not the front's three.
func TestSplitCardCountsBothHalvesAsEvidence(t *testing.T) {
	sc, id := facesScen(t, "Fuss", "Grizzly Bears")
	fuss := sc.grave(id["Fuss"], 0, 1)[0]
	if mv := sc.s.graveMV(sc.k, fuss); mv != 9 {
		t.Fatalf("Fuss // Bother in the graveyard counts %d toward evidence, want 9", mv)
	}
	bear := sc.grave(id["Grizzly Bears"], 0, 1)[0]
	if mv := sc.s.graveMV(sc.k, bear); mv != 2 {
		t.Fatalf("Grizzly Bears in the graveyard counts %d toward evidence, want 2", mv)
	}
	if tot := sc.s.graveTotalMV(sc.k, 0); tot != 11 {
		t.Fatalf("graveyard total %d, want 11", tot)
	}
}
