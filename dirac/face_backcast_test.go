package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// A land-front Adventure (CR 715.3a, gorge) and a modal
// DFC's nonland back face (CR 712.8, gorge) on the faces pool.

// TestLandFrontAdventureCast: Lindblum, Industrial Regency offers its land
// play and its Adventure from the hand (gorge appendAdventureAlt in the
// hand walk's land branch), the Adventure even after the turn's land play;
// resolved, the card rests in exile as the land, on an adventure, with no
// recast (CR 305.1: gorge offers none for a land main face) and a land
// play from exile while the turn's drop remains (CR 715.3d, gorge).
// Playing it moves the land to the battlefield as its front, clears the
// adventure and consumes the drop.
func TestLandFrontAdventureCast(t *testing.T) {
	sc, id := facesScen(t, "Lindblum, Industrial Regency", "Mountain")
	front := id["Lindblum, Industrial Regency"]
	face := sc.k.t.Cards[front].Faces[1]
	if sc.k.t.Cards[face].FaceMode != kop.FaceAdventure || sc.k.t.Cards[face].Face != 1 || !sc.k.cards[front].land {
		t.Fatal("not a land-front Adventure")
	}
	for i := 0; i < 3; i++ {
		sc.perm(id["Mountain"], 0)
	}
	lb := sc.hand(front, 0)
	d := sc.s.Step()
	if got := landPlays(&d, lb); len(got) != 1 || got[0] != 0 {
		t.Fatalf("land plays %v, want [0]", got)
	}
	if got := faceCasts(&d, lb); len(got) != 1 || got[0] != 1 {
		t.Fatalf("face casts %v, want [1]", got)
	}
	sc.s.pl[0].landsPlayed = 1
	d = sc.s.Step()
	if got := landPlays(&d, lb); len(got) != 0 {
		t.Fatalf("land plays after the land drop %v, want none", got)
	}
	if got := faceCasts(&d, lb); len(got) != 1 || got[0] != 1 {
		t.Fatalf("face casts after the land drop %v, want [1]", got)
	}
	perms := sc.s.occ.count()
	sc.s.Apply(MakeAction(AkCast, TCode(lb), 1, castFace))
	sc.buildToStack(t)
	if sc.s.objs[lb].card != face || sc.s.objs[lb].zone != kop.ZoneStack {
		t.Fatal("the Adventure is not on the stack")
	}
	sc.resolveAll(t)
	if sc.s.occ.count() != perms+1 {
		t.Fatalf("permanents %d, want %d (a Wizard token)", sc.s.occ.count(), perms+1)
	}
	ob := &sc.s.objs[lb]
	if ob.zone != kop.ZoneExile || ob.card != front || ob.of&ofAdventure == 0 {
		t.Fatalf("after resolving: zone %d card front %v on an adventure %v", ob.zone, ob.card == front, ob.of&ofAdventure != 0)
	}
	sc.s.pl[0].landsPlayed = 0
	d = sc.s.Step()
	if got := faceCasts(&d, lb); len(got) != 0 {
		t.Fatalf("recasts %v, want none for a land", got)
	}
	if got := landPlays(&d, lb); len(got) != 1 || got[0] != 0 {
		t.Fatalf("land plays from exile %v, want [0] (CR 715.3d)", got)
	}
	sc.s.pl[0].landsPlayed = 1
	d = sc.s.Step()
	if got := landPlays(&d, lb); len(got) != 0 {
		t.Fatalf("land plays from exile after the drop %v, want none", got)
	}
	sc.s.pl[0].landsPlayed = 0
	sc.s.Apply(MakeAction(AkPlayLand, TCode(lb), 0, 0))
	if !sc.onBF(lb) || sc.s.cardAt(sc.slot(lb)) != front || sc.s.pl[0].landsPlayed != 1 {
		t.Fatalf("played from exile: on battlefield %v, front %v, lands played %d",
			sc.onBF(lb), sc.s.cardAt(sc.slot(lb)) == front, sc.s.pl[0].landsPlayed)
	}
	if sc.s.objs[lb].of&ofAdventure != 0 {
		t.Fatal("still on an adventure after the land play")
	}
}

// TestModalSpellBackCast: Kolvori, God of Kinship offers its back, The
// Ringhart Crest, from the hand (gorge "modal_spell", CR 712.8), never as
// a land play; cast, the back is on the stack and resolves into a
// permanent with the back's characteristics (a noncreature artifact, CR
// 712.8a); leaving the battlefield it is Kolvori again (CR 712.4d).
func TestModalSpellBackCast(t *testing.T) {
	sc, id := facesScen(t, "Kolvori, God of Kinship", "Forest")
	front := id["Kolvori, God of Kinship"]
	back := sc.k.t.Cards[front].Faces[1]
	if sc.k.t.Cards[back].FaceMode != kop.FaceModal || sc.k.t.Cards[back].Face != 1 || sc.k.cards[back].land {
		t.Fatal("The Ringhart Crest is not a nonland modal back")
	}
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	kv := sc.hand(front, 0)
	d := sc.s.Step()
	if got := landPlays(&d, kv); len(got) != 0 {
		t.Fatalf("land plays %v, want none", got)
	}
	if got := faceCasts(&d, kv); len(got) != 1 || got[0] != 1 {
		t.Fatalf("face casts %v, want [1]", got)
	}
	sc.s.Apply(MakeAction(AkCast, TCode(kv), 1, castFace))
	sc.buildToStack(t)
	if sc.s.objs[kv].card != back || sc.s.objs[kv].zone != kop.ZoneStack {
		t.Fatal("The Ringhart Crest is not on the stack")
	}
	sc.resolveAll(t)
	if !sc.onBF(kv) {
		t.Fatal("The Ringhart Crest did not enter")
	}
	sl := sc.slot(kv)
	if sc.s.cardAt(sl) != back || sc.s.isCreature(sc.k, sl) || sc.k.t.Cards[back].Types&kop.TArtifact == 0 {
		t.Fatal("the permanent is not the back face artifact")
	}
	sc.s.leave(sc.k, sl, kop.ZoneGraveyard, false)
	if ob := &sc.s.objs[kv]; ob.zone != kop.ZoneGraveyard || ob.card != front {
		t.Fatalf("left: zone %d, front %v", ob.zone, ob.card == front)
	}
}

// TestModalSpellBackResetsOffStack: Pestilent Cauldron's back, Restorative
// Burst, is a sorcery that exiles itself as it resolves; off the stack the
// card is Pestilent Cauldron again (CR 712.8a, gorge), in exile as
// after a counter in the graveyard (an undone face cast: unturnFace,
// TestAdventureCounteredToGraveyard).
func TestModalSpellBackResetsOffStack(t *testing.T) {
	sc, id := facesScen(t, "Pestilent Cauldron", "Forest")
	front := id["Pestilent Cauldron"]
	back := sc.k.t.Cards[front].Faces[1]
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	pc := sc.hand(front, 0)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(pc), 1, castFace))
	sc.buildToStack(t)
	if sc.s.sp != 1 || sc.s.objs[pc].card != back {
		t.Fatal("Restorative Burst is not on the stack")
	}
	sc.s.counterAt(sc.k, 0)
	if ob := &sc.s.objs[pc]; ob.zone != kop.ZoneGraveyard || ob.card != front {
		t.Fatalf("countered: zone %d, front %v", ob.zone, ob.card == front)
	}
	// resolved, in a fresh scenario: the sorcery exiles itself
	sc, id = facesScen(t, "Pestilent Cauldron", "Forest")
	front = id["Pestilent Cauldron"]
	for i := 0; i < 5; i++ {
		sc.perm(id["Forest"], 0)
	}
	pc = sc.hand(front, 0)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(pc), 1, castFace))
	sc.buildToStack(t)
	sc.resolveAll(t)
	if ob := &sc.s.objs[pc]; ob.zone != kop.ZoneExile || ob.card != front {
		t.Fatalf("resolved: zone %d, front %v, life %d", ob.zone, ob.card == front, sc.s.pl[0].life)
	}
}
