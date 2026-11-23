package dirac

import (
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// startTable is vanilla's table plus a transforming creature and an Aura.
func startTable() (tbl *kop.Table, front, back, aura CardID) {
	t := *v.Table
	t.Name = "initnstart"
	front = CardID(len(v.Table.Cards))
	back, aura = front+1, front+2
	t.Cards = append(slices.Clone(v.Table.Cards),
		kop.CardDef{Name: "Test Front", Types: kop.TCreature, Power: 1, Toughness: 1,
			FaceMode: kop.FaceTransform, Faces: []CardID{front, back}},
		kop.CardDef{Name: "Test Back", Types: kop.TCreature, Power: 4, Toughness: 4,
			FaceMode: kop.FaceTransform, Faces: []CardID{front, back}, Face: 1},
		kop.CardDef{Name: "Test Aura", Types: kop.TEnchantment | kop.TAura},
	)
	return &t, front, back, aura
}

// startDeck is a 40-card deck cycling pat, so index i is pat[i%len(pat)].
func startDeck(pat ...CardID) []CardID {
	d := make([]CardID, 40)
	for i := range d {
		d[i] = pat[i%len(pat)]
	}
	return d
}

func startGame(tbl *kop.Table, seed uint64, st *Start) (*State, [MaxSeats][]CardID) {
	var decks [MaxSeats][]CardID
	decks[0] = startDeck(v.Plains, v.Forest, v.GrizzlyBears, v.ElvishVisionary, v.EmberWalker, v.LightningBolt, v.HillGiant, v.Island)
	decks[1] = startDeck(v.Island, v.Plains, v.SavannahLions, v.GrizzlyBears)
	s := new(State)
	s.InitNStart(tbl, decks, 2, seed, Config{NoElide: true}, st)
	return s, decks
}

func libOf(s *State, p uint8) []Obj {
	pl := &s.pl[p]
	return slices.Clone(pl.lib[pl.libLo:pl.libHi])
}

func handOf(s *State, p uint8) []Obj {
	pl := &s.pl[p]
	return slices.Clone(pl.hand[:pl.handN])
}

// TestInitNStartNilIsInitN: a nil start is byte-for-byte InitN.
func TestInitNStartNilIsInitN(t *testing.T) {
	tbl, _, _, _ := startTable()
	a, decks := startGame(tbl, 7, nil)
	b := new(State)
	b.InitN(tbl, decks, 2, 7, Config{NoElide: true})
	if !reflect.DeepEqual(a, b) {
		t.Fatal("InitNStart(nil) differs from InitN")
	}
}

// TestInitNStartPlacement: every zone, tapped, counters, life and the
// settled (not summoning sick) permanents.
func TestInitNStartPlacement(t *testing.T) {
	tbl, _, _, _ := startTable()
	life := int32(13)
	st := &Start{Seats: []StartSeat{{
		Hand:        []int{7, 3},
		Battlefield: []int{0, 4, 2},
		Graveyard:   []int{5, 6},
		Exile:       []int{1},
		Library:     []int{9, 8},
		Tapped:      []int{0},
		Counters:    []StartCounter{{Card: 2, Kind: kop.CtrP1P1, N: 3}},
		Life:        &life,
	}, {
		Battlefield: []int{2},
	}}}
	s, _ := startGame(tbl, 11, st)
	if !slices.Equal(handOf(s, 0), []Obj{7, 3}) {
		t.Fatalf("hand %v, want [7 3]", handOf(s, 0))
	}
	for _, c := range []struct {
		o  Obj
		z  kop.Zone
		pl uint8
	}{{0, kop.ZoneBattlefield, 0}, {4, kop.ZoneBattlefield, 0}, {2, kop.ZoneBattlefield, 0},
		{5, kop.ZoneGraveyard, 0}, {6, kop.ZoneGraveyard, 0}, {1, kop.ZoneExile, 0}, {40 + 2, kop.ZoneBattlefield, 1}} {
		if got := s.ObjZone(c.o); got != c.z {
			t.Errorf("object %d in zone %d, want %d", c.o, got, c.z)
		}
	}
	if g := s.pl[0].grave[:s.pl[0].graveN]; !slices.Equal(g, []Obj{5, 6}) {
		t.Errorf("graveyard %v, want [5 6]", g)
	}
	if !s.Tapped(0) || s.Tapped(2) || s.Tapped(4) {
		t.Errorf("tapped: %v %v %v, want true false false", s.Tapped(0), s.Tapped(2), s.Tapped(4))
	}
	if n := s.Counters(2, kop.CtrP1P1); n != 3 {
		t.Errorf("+1/+1 counters %d, want 3", n)
	}
	if n := s.Counters(4, kop.CtrLoyalty); n != 3 {
		t.Errorf("planeswalker loyalty %d, want its printed 3 (entry grants stay)", n)
	}
	for _, o := range []Obj{0, 4, 2, 42} {
		if s.perms[s.objs[o].slot].flags&pfSick != 0 {
			t.Errorf("object %d is summoning sick", o)
		}
	}
	if s.Life(0) != 13 || s.Life(1) != 20 {
		t.Errorf("life %d/%d, want 13/20", s.Life(0), s.Life(1))
	}
	if s.npend != 0 {
		t.Errorf("%d triggers pending after the start", s.npend)
	}
	// the first player's turn begins from the scripted position
	if s.turn != 1 {
		t.Errorf("turn %d, want 1", s.turn)
	}
}

// TestInitNStartLibraryOrder: the listed cards top first, then the post-shuffle
// order minus every placed object, then the returned unlisted opening-hand
// cards in hand order (design D3).
func TestInitNStartLibraryOrder(t *testing.T) {
	tbl, _, _, _ := startTable()
	for seed := uint64(1); seed <= 4; seed++ {
		base, _ := startGame(tbl, seed, nil)
		bl, bh := libOf(base, 0), handOf(base, 0)
		if len(bl) != 33 || len(bh) != 7 {
			t.Fatalf("baseline lib %d hand %d", len(bl), len(bh))
		}
		keep := bh[2] // a dealt card kept in hand
		top := bl[5]  // listed library cards: one from the library ...
		top2 := bh[4] // ... one from the dealt hand
		plc := bl[1]  // placed on the battlefield
		plc2 := bh[0] // placed in the graveyard, from the dealt hand
		_ = keep
		idx := func(o Obj) int { return int(o) } // seat 0's offset is 0
		st := &Start{Seats: []StartSeat{{
			Hand:        []int{idx(keep)},
			Battlefield: []int{idx(plc)},
			Graveyard:   []int{idx(plc2)},
			Library:     []int{idx(top), idx(top2)},
		}}}
		s, _ := startGame(tbl, seed, st)
		want := []Obj{top, top2}
		placed := []Obj{keep, plc, plc2, top, top2}
		for _, o := range bl {
			if !slices.Contains(placed, o) {
				want = append(want, o)
			}
		}
		for _, o := range bh {
			if !slices.Contains(placed, o) {
				want = append(want, o)
			}
		}
		if got := libOf(s, 0); !slices.Equal(got, want) {
			t.Fatalf("seed %d library %v, want %v", seed, got, want)
		}
		if got := handOf(s, 0); !slices.Equal(got, []Obj{keep}) {
			t.Fatalf("seed %d hand %v, want [%d]", seed, got, keep)
		}
		// the seat the start names no cards for is untouched
		if !slices.Equal(libOf(s, 1), libOf(base, 1)) || !slices.Equal(handOf(s, 1), handOf(base, 1)) {
			t.Fatalf("seed %d: seat 1 changed", seed)
		}
	}
}

// TestInitNStartDropsTriggers: an ETB creature placed by the start queues
// nothing, where the same entry outside a start does (design D4).
func TestInitNStartDropsTriggers(t *testing.T) {
	tbl, _, _, _ := startTable()
	// precondition: Elvish Visionary's ETB is queued by an ordinary entry
	sc := newScenTable(t, tbl, Config{NoElide: true})
	o := sc.obj(v.ElvishVisionary, 0)
	sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
	if sc.s.npend == 0 {
		t.Fatal("precondition: an ordinary Elvish Visionary entry queues no trigger")
	}
	st := &Start{Seats: []StartSeat{{Battlefield: []int{3}}}} // idx 3 is Elvish Visionary
	s, _ := startGame(tbl, 5, st)
	if s.objs[3].zone != kop.ZoneBattlefield {
		t.Fatal("Elvish Visionary is not on the battlefield")
	}
	if s.npend != 0 {
		t.Fatalf("%d triggers pending, want none", s.npend)
	}
	if s.pl[0].handN != 0 {
		t.Fatalf("hand %d after a start naming no hand, want 0", s.pl[0].handN)
	}
}

// TestInitNStartBackFaceAndAttach: a back face enters on its back face, and
// an Aura enters attached to a permanent of either seat.
func TestInitNStartBackFaceAndAttach(t *testing.T) {
	tbl, front, back, aura := startTable()
	var decks [MaxSeats][]CardID
	decks[0] = startDeck(v.Plains, front, aura)
	decks[1] = startDeck(v.Island, v.GrizzlyBears)
	st := &Start{Seats: []StartSeat{{
		Battlefield: []int{1},
		BackFace:    []int{1},
		Attach:      []StartAttach{{Card: 2, HostSeat: 1, HostCard: 1}},
	}, {
		Battlefield: []int{1},
	}}}
	s := new(State)
	s.InitNStart(tbl, decks, 2, 3, Config{NoElide: true}, st)
	if got := s.objs[1].card; got != back {
		t.Fatalf("card %d on the battlefield, want the back face %d", got, back)
	}
	if s.objs[1].zone != kop.ZoneBattlefield || s.objs[2].zone != kop.ZoneBattlefield {
		t.Fatal("the back-face card or the Aura is not on the battlefield")
	}
	host := s.objs[40+1].slot
	if got := s.perms[s.objs[2].slot].attached; got != host {
		t.Fatalf("Aura attached to slot %d, want %d", got, host)
	}
	if s.perms[s.objs[2].slot].flags&pfSick != 0 {
		t.Fatal("the Aura is summoning sick")
	}
}

// TestInitNStartDeterministic: the same start and seed reproduce the same
// state, and the game then plays on through a few decisions.
func TestInitNStartDeterministic(t *testing.T) {
	tbl, _, _, _ := startTable()
	st := &Start{Seats: []StartSeat{{
		Hand: []int{0, 1}, Battlefield: []int{2, 3}, Library: []int{5, 4},
	}}}
	a, _ := startGame(tbl, 9, st)
	b, _ := startGame(tbl, 9, st)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("same seed and start gave different states")
	}
	for i := 0; i < 12; i++ {
		d := a.Step()
		if d.Kind == DkNone {
			break
		}
		a.Apply(d.Options[0])
	}
}

// TestInitNStartRefusals: malformed starts panic rather than build a state
// the two engines would read differently.
func TestInitNStartRefusals(t *testing.T) {
	tbl, _, _, _ := startTable()
	bad := map[string]*Start{
		"out of range":  {Seats: []StartSeat{{Hand: []int{40}}}},
		"twice":         {Seats: []StartSeat{{Hand: []int{1}, Graveyard: []int{1}}}},
		"tapped off bf": {Seats: []StartSeat{{Hand: []int{1}, Tapped: []int{1}}}},
		"counter off bf": {Seats: []StartSeat{{Hand: []int{1},
			Counters: []StartCounter{{Card: 1, Kind: kop.CtrP1P1, N: 1}}}}},
		"too many seats": {Seats: []StartSeat{{}, {}, {}}},
	}
	for name, st := range bad {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: no panic", name)
				}
			}()
			startGame(tbl, 1, st)
		}()
	}
}
