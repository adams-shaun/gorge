package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Two-door Rooms (face_rooms.go) on the generated rooms pool.

func roomsScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "rooms")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the rooms table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	var lib [20]CardID
	for i := range lib {
		lib[i] = id["Plains"]
	}
	s := new(State)
	s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pls := &s.pl[p]
		for pls.handN > 0 {
			o := pls.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pls.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

// unlocks lists the AkUnlock options' doors for object o.
func unlocks(d *Decision, o Obj) []uint8 {
	var out []uint8
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkUnlock && a.Source() == TCode(uint8(o)) {
			out = append(out, a.Mode())
		}
	}
	return out
}

// TestRoomCastEachDoor: the hand offers the other door (room_alt); cast,
// the Room enters as that door's plain def (its door unlocked) and fires
// its UnlockDoor trigger on entry.
func TestRoomCastEachDoor(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Island")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	if cd.FaceMode != kop.FaceRoom || len(cd.Faces) != 8 {
		t.Fatal("Bottomless Pool is not a Room")
	}
	for i := 0; i < 5; i++ {
		sc.perm(id["Island"], 0)
	}
	o := sc.hand(front, 0)
	d := sc.s.Step()
	if got := faceCasts(&d, o); len(got) != 1 || got[0] != 1 {
		t.Fatalf("room_alt casts %v, want [1]", got)
	}
	sc.s.Apply(MakeAction(AkCast, TCode(o), 1, castFace))
	sc.buildToStack(t)
	if sc.s.objs[o].card != cd.Faces[1] {
		t.Fatal("Locker Room is not on the stack")
	}
	sc.resolveAll(t)
	if !sc.onBF(o) || sc.s.perms[sc.slot(o)].card != cd.Faces[1] {
		t.Fatal("Locker Room did not enter as its plain def")
	}
}

// TestRoomUncastEntryAndUnlock: a Room entering other than as a cast spell
// has no door unlocked; the unlock is offered at sorcery timing for each
// locked door when the pool pays, swaps the def,
// fires that door's UnlockDoor and resets the passes.
func TestRoomUncastEntryAndUnlock(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Grizzly Bears")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	o := sc.perm(front, 0)
	if c := sc.s.perms[sc.slot(o)].card; c != cd.Faces[kop.RoomState(0, 0)] {
		t.Fatal("an uncast Room entered with a door unlocked")
	}
	if mv := sc.s.cmcPerm(sc.k, sc.slot(o)); mv != 0 {
		t.Fatalf("no-door MV %d, want 0", mv)
	}
	d := sc.s.Step()
	if got := unlocks(&d, o); len(got) != 0 {
		t.Fatalf("unlocks %v with an empty pool", got)
	}
	sc.s.pl[0].pool[kop.U] = 5
	d = sc.s.Step()
	if got := unlocks(&d, o); len(got) != 2 || got[0]+got[1] != 1 {
		t.Fatalf("unlocks %v, want doors 0 and 1", got)
	}
	sc.perm(id["Grizzly Bears"], 1)
	sc.s.passes = 1
	sc.s.Apply(MakeAction(AkUnlock, TCode(o), 0, 0))
	if c := sc.s.perms[sc.slot(o)].card; c != cd.Faces[kop.RoomState(0, 1)] {
		t.Fatal("unlocking door 0 did not swap to (0,{0})")
	}
	if sc.s.passes != 0 || sc.s.pl[0].pool[kop.U] != 4 || sc.s.npend != 1 {
		t.Fatalf("passes %d pool U %d pending %d", sc.s.passes, sc.s.pl[0].pool[kop.U], sc.s.npend)
	}
	sc.s.npend = 0
	d = sc.s.Step()
	if got := unlocks(&d, o); len(got) != 0 {
		t.Fatalf("unlocks %v: Locker Room costs 4U, the pool has 4U", got)
	}
	sc.s.pl[0].pool[kop.U] = 5
	sc.s.Apply(MakeAction(AkUnlock, TCode(o), 0, 1))
	if c := sc.s.perms[sc.slot(o)].card; c != cd.Faces[kop.RoomState(0, 3)] {
		t.Fatal("unlocking door 1 did not swap to (0,{0,1})")
	}
	if mv := sc.s.cmcPerm(sc.k, sc.slot(o)); mv != 6 {
		t.Fatalf("both-doors MV %d, want 6", mv)
	}
	if sc.s.npend != 0 {
		t.Fatal("Locker Room has no UnlockDoor trigger, yet one queued")
	}
}

// TestRoomFacePersists: a room_alt Room keeps face 1 when it leaves the
// battlefield (the plain face-1 def), and its mana value off the stack and
// battlefield is the halves' sum.
func TestRoomFacePersists(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	o := sc.obj(cd.Faces[1], 0)
	sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneStack)
	sc.s.npend = 0
	if sc.s.perms[sl].card != cd.Faces[1] {
		t.Fatal("a resolved Locker Room did not keep its door unlocked")
	}
	sc.s.removePerm(sc.k, sl, kop.ZoneGraveyard, false)
	if sc.s.objs[o].card != cd.Faces[1] || sc.s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatal("leaving the battlefield turned the Room to face 0")
	}
	if mv := sc.s.objMV(sc.k, cd.Faces[1], o); mv != 6 {
		t.Fatalf("graveyard MV %d, want 6 (the halves' sum)", mv)
	}
}

// TestRoomReversal: a reversed room_alt cast turns the card back to the
// face it showed (gorge pc.faceBefore); a reversed plain cast of a Room
// showing face 1 leaves it on face 1 (no proposal flip, faceBefore nil).
func TestRoomReversal(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Island")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	for i := 0; i < 5; i++ {
		sc.perm(id["Island"], 0)
	}
	o := sc.hand(front, 0)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(o), 1, castFace))
	sc.s.Step()
	if sc.s.objs[o].card != cd.Faces[1] {
		t.Fatal("the room_alt cast did not turn Locker Room up")
	}
	sc.s.reverseCast(sc.k)
	if ob := &sc.s.objs[o]; ob.zone != kop.ZoneHand || ob.card != front {
		t.Fatalf("reversed room_alt: zone %d, face 0 %v", ob.zone, ob.card == front)
	}
	// a fresh game: the reversal above holds a no-progress repeat back
	sc, id = roomsScen(t, "Bottomless Pool", "Island")
	cd = &sc.k.t.Cards[id["Bottomless Pool"]]
	for i := 0; i < 5; i++ {
		sc.perm(id["Island"], 0)
	}
	o1 := sc.hand(cd.Faces[1], 0)
	sc.castMode(t, o1, 0)
	sc.s.Step()
	if sc.s.objs[o1].card != cd.Faces[1] {
		t.Fatal("a plain cast of a face-1 Room did not cast Locker Room")
	}
	sc.s.reverseCast(sc.k)
	if ob := &sc.s.objs[o1]; ob.zone != kop.ZoneHand || ob.card != cd.Faces[1] {
		t.Fatalf("reversed plain cast: zone %d, face 1 %v", ob.zone, ob.card == cd.Faces[1])
	}
}

// TestRoomFaceSurvivesCounterAndBounce: a countered room_alt Room goes to
// the graveyard on face 1 (the split stack-exit reset excludes Rooms); a
// bounced face-1 Room returns to the hand on face 1 and its next plain
// cast casts door 1.
func TestRoomFaceSurvivesCounterAndBounce(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Island")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	for i := 0; i < 10; i++ {
		sc.perm(id["Island"], 0)
	}
	o := sc.hand(front, 0)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(o), 1, castFace))
	sc.buildToStack(t)
	sc.s.counterAt(sc.k, 0)
	if ob := &sc.s.objs[o]; ob.zone != kop.ZoneGraveyard || ob.card != cd.Faces[1] {
		t.Fatalf("countered: zone %d, face 1 %v", ob.zone, ob.card == cd.Faces[1])
	}
	b := sc.obj(cd.Faces[1], 0)
	sl := sc.s.enter(sc.k, b, 0, false, kop.ZoneStack)
	sc.s.npend = 0
	sc.s.removePerm(sc.k, sl, kop.ZoneHand, false)
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneHand || ob.card != cd.Faces[1] {
		t.Fatalf("bounced: zone %d, face 1 %v", ob.zone, ob.card == cd.Faces[1])
	}
	sc.castMode(t, b, 0)
	sc.buildToStack(t)
	if sc.s.objs[b].zone != kop.ZoneStack || sc.s.objs[b].card != cd.Faces[1] {
		t.Fatal("the bounced Room's plain cast did not cast Locker Room")
	}
}

// TestRoomFace1Planned: gorge's cast planner prices the face the object
// shows (rules payment_plan.go planCastPaymentChecked, CR 709.3), so a Room
// back in the hand showing door 1 has a planned plain cast when untapped
// Islands and an empty pool can pay it.
func TestRoomFace1Planned(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Island")
	cd := &sc.k.t.Cards[id["Bottomless Pool"]]
	for i := 0; i < 6; i++ {
		sc.perm(id["Island"], 0)
	}
	b := sc.obj(cd.Faces[1], 0)
	sl := sc.s.enter(sc.k, b, 0, false, kop.ZoneStack)
	sc.s.npend = 0
	sc.s.removePerm(sc.k, sl, kop.ZoneHand, false)
	if ob := &sc.s.objs[b]; ob.zone != kop.ZoneHand || ob.card != cd.Faces[1] {
		t.Fatalf("bounced: zone %d, face 1 %v", ob.zone, ob.card == cd.Faces[1])
	}
	if sc.s.rbAny(0) || sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatal("the pool is not empty")
	}
	if !sc.s.FeatPlannedCast(0, b, 0) {
		t.Fatal("gorge's cast planner did not plan a face-1 Room's plain cast")
	}
}

// TestRoomMV: a Room's mana value is the halves' sum in the hand, the
// cast door's on the stack, and the unlocked doors' on the battlefield.
func TestRoomMV(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Island")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	for i := 0; i < 5; i++ {
		sc.perm(id["Island"], 0)
	}
	o := sc.hand(front, 0)
	if mv := sc.s.objMV(sc.k, front, o); mv != 6 {
		t.Fatalf("hand MV %d, want 6", mv)
	}
	h1 := sc.hand(cd.Faces[1], 0)
	if mv := sc.s.objMV(sc.k, cd.Faces[1], h1); mv != 6 {
		t.Fatalf("face-1 hand MV %d, want 6", mv)
	}
	sc.s.Step()
	sc.s.Apply(MakeAction(AkCast, TCode(o), 1, castFace))
	sc.buildToStack(t)
	if mv := sc.s.objMV(sc.k, sc.s.objs[o].card, o); mv != 5 {
		t.Fatalf("stack MV %d, want 5 (Locker Room)", mv)
	}
	for _, c := range []struct {
		card CardID
		want int
	}{{front, 1}, {cd.Faces[1], 5}, {cd.Faces[kop.RoomState(1, 1)], 1}} {
		b := sc.obj(c.card, 0)
		sl := sc.s.enter(sc.k, b, 0, false, kop.ZoneStack)
		sc.s.npend = 0
		if mv := sc.s.cmcPerm(sc.k, sl); mv != c.want {
			t.Fatalf("one-door battlefield MV %d, want %d", mv, c.want)
		}
	}
}

// TestRoomUnlockOffer: the unlock is offered at sorcery timing only, in
// [1-Face, Face] order, and only for a locked door.
func TestRoomUnlockOffer(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	o0 := sc.perm(front, 0)
	o1 := sc.perm(cd.Faces[1], 0)
	if sc.s.objs[o1].card != cd.Faces[kop.RoomState(1, 0)] {
		t.Fatal("an uncast face-1 Room entered with a door unlocked")
	}
	sc.s.pl[0].pool[kop.U] = 5
	// the walk's order (Step then sorts the options canonically)
	var raw Decision
	sc.s.offerUnlocks(sc.k, &raw, 0)
	if got := unlocks(&raw, o0); len(got) != 2 || got[0] != 1 || got[1] != 0 {
		t.Fatalf("face-0 Room unlocks %v, want [1 0]", got)
	}
	if got := unlocks(&raw, o1); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("face-1 Room unlocks %v, want [0 1]", got)
	}
	d := sc.s.Step()
	if got := unlocks(&d, o0); len(got) != 2 {
		t.Fatalf("face-0 Room unlocks %v at sorcery timing, want both doors", got)
	}
	sc.s.step = kop.StepEnd
	d = sc.s.Step()
	if got := unlocks(&d, o0); len(got) != 0 {
		t.Fatalf("unlocks %v in the end step", got)
	}
	sc.s.step = kop.StepMain1
	sc.s.Apply(MakeAction(AkUnlock, TCode(o0), 0, 0))
	sc.s.npend = 0
	d = sc.s.Step()
	if got := unlocks(&d, o0); len(got) != 0 {
		t.Fatalf("unlocks %v: door 0 is unlocked, Locker Room costs more than the pool's 4U", got)
	}
	sc.s.pl[0].pool[kop.U] = 5
	d = sc.s.Step()
	if got := unlocks(&d, o0); len(got) != 1 || got[0] != 1 {
		t.Fatalf("unlocks %v, want only the locked door 1", got)
	}
	// an unlocked door is not unlocked again, nor paid for
	c := sc.s.perms[sc.slot(o0)].card
	sc.s.Apply(MakeAction(AkUnlock, TCode(o0), 0, 0))
	if sc.s.perms[sc.slot(o0)].card != c || sc.s.pl[0].pool[kop.U] != 5 || sc.s.npend != 0 {
		t.Fatal("unlocking an unlocked door changed the game")
	}
}

// TestRoomUnlockDoorTrigger: a door's UnlockDoor fires as the cast door
// enters and as the door is unlocked, never on an uncast entry or a spell
// copy's entry, and never again once the door is unlocked.
func TestRoomUnlockDoorTrigger(t *testing.T) {
	sc, id := roomsScen(t, "Bottomless Pool", "Grizzly Bears")
	front := id["Bottomless Pool"]
	cd := &sc.k.t.Cards[front]
	sc.perm(id["Grizzly Bears"], 1)
	// cast door 0 enters: its UnlockDoor fires
	o := sc.obj(front, 0)
	sl := sc.s.enter(sc.k, o, 0, false, kop.ZoneStack)
	if sc.s.npend != 1 {
		t.Fatalf("cast-door entry queued %d, want 1", sc.s.npend)
	}
	sc.s.npend = 0
	// unlocking door 1 (no UnlockDoor of its own) does not fire door 0's again
	sc.s.pl[0].pool[kop.U] = 5
	sc.s.Apply(MakeAction(AkUnlock, TCode(o), 0, 1))
	if sc.s.perms[sl].card != cd.Faces[kop.RoomState(0, 3)] || sc.s.npend != 0 {
		t.Fatalf("unlocking Locker Room queued %d", sc.s.npend)
	}
	// uncast entry: nothing
	u := sc.obj(front, 0)
	sc.s.enter(sc.k, u, 0, false, kop.ZoneNone)
	if sc.s.npend != 0 {
		t.Fatalf("uncast entry queued %d", sc.s.npend)
	}
	// a spell copy resolving: no door, nothing (gorge CastDoor needs !IsCopy)
	c := sc.obj(front, 0)
	sc.s.objs[c].of |= ofCopy
	csl := sc.s.enter(sc.k, c, 0, false, kop.ZoneStack)
	if sc.s.perms[csl].card != cd.Faces[kop.RoomState(0, 0)] || sc.s.npend != 0 {
		t.Fatalf("spell copy: no-door def %v, queued %d", sc.s.perms[csl].card == cd.Faces[kop.RoomState(0, 0)], sc.s.npend)
	}
	// Locker Room cast (face 1), then door 0 unlocked: door 0's UnlockDoor
	// is the other door's line (past RoomSplit) and fires
	l := sc.obj(cd.Faces[1], 0)
	lsl := sc.s.enter(sc.k, l, 0, false, kop.ZoneStack)
	if sc.s.npend != 0 {
		t.Fatalf("Locker Room entry queued %d", sc.s.npend)
	}
	sc.s.pl[0].pool[kop.U] = 1
	sc.s.Apply(MakeAction(AkUnlock, TCode(l), 0, 0))
	if sc.s.perms[lsl].card != cd.Faces[kop.RoomState(1, 3)] || sc.s.npend != 1 {
		t.Fatalf("unlocking Bottomless Pool on a face-1 Room queued %d, want 1", sc.s.npend)
	}
}

// TestRoomLockedDoorInert: Locker Room's combat-damage draw does not fire
// while its door is locked, and fires once it is the live door.
func TestRoomLockedDoorInert(t *testing.T) {
	for _, c := range []struct {
		face  int
		draws int
	}{{0, 0}, {1, 1}} {
		sc, id := roomsScen(t, "Bottomless Pool", "Grizzly Bears")
		cd := &sc.k.t.Cards[id["Bottomless Pool"]]
		r := sc.obj(cd.Faces[c.face], 0)
		sc.s.enter(sc.k, r, 0, false, kop.ZoneStack)
		sc.resolveAll(t)
		bear := sc.perm(id["Grizzly Bears"], 0)
		n := sc.s.pl[0].handN
		sc.combat(t, []Obj{bear}, nil)
		sc.resolveAll(t)
		if got := int(sc.s.pl[0].handN - n); got != c.draws {
			t.Errorf("Room on face %d: %d draws after combat damage, want %d", c.face, got, c.draws)
		}
	}
}
