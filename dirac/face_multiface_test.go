package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Multi-face cards (face_multiface.go) on the generated faces pool: a modal
// DFC's land back face is played from the hand (gorge's "modal_land") and
// the card is its front again once it leaves the battlefield.

func facesScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 8}}
	set := cardsettest.Set(t, "faces")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Fatalf("%s not supported in the faces table", n)
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

// landPlays lists the AkPlayLand options' modes for object o.
func landPlays(d *Decision, o Obj) []uint8 {
	var out []uint8
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkPlayLand && a.Source() == TCode(uint8(o)) {
			out = append(out, a.Mode())
		}
	}
	return out
}

// TestModalLandOffer: a nonland-front modal DFC offers only its back land
// (mode 1); a Pathway offers its front land and its back land; neither
// once the turn's land is played (gorge legal_walk_hand.go modalLandBack).
func TestModalLandOffer(t *testing.T) {
	sc, id := facesScen(t, "Akoum Warrior", "Cragcrown Pathway")
	if !sc.k.gen.anyFaces {
		t.Fatal("anyFaces not set")
	}
	aw := sc.hand(id["Akoum Warrior"], 0)
	cp := sc.hand(id["Cragcrown Pathway"], 0)
	d := sc.s.Step()
	if got := landPlays(&d, aw); len(got) != 1 || got[0] != 1 {
		t.Fatalf("Akoum Warrior land plays %v, want [1]", got)
	}
	if got := landPlays(&d, cp); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("Cragcrown Pathway land plays %v, want [0 1]", got)
	}
	sc.s.Apply(MakeAction(AkPlayLand, TCode(cp), 0, 0))
	d = sc.s.Step()
	if got := landPlays(&d, aw); len(got) != 0 {
		t.Fatalf("a second land play offered: %v", got)
	}
	if sc.s.objs[cp].card != id["Cragcrown Pathway"] {
		t.Fatal("the front land play turned a face")
	}
}

// TestModalLandPlay: the back land enters as the back face (Akoum Teeth
// enters tapped; a red mana land), counts as the land play, and the card
// is Akoum Warrior again in its owner's hand and graveyard (CR 712.4d;
// gorge resets FaceIdx when it leaves the battlefield).
func TestModalLandPlay(t *testing.T) {
	sc, id := facesScen(t, "Akoum Warrior")
	front := id["Akoum Warrior"]
	back := sc.k.t.Cards[front].Faces[1]
	aw := sc.hand(front, 0)
	sc.s.Step()
	sc.s.Apply(MakeAction(AkPlayLand, TCode(aw), 0, 1))
	if !sc.onBF(aw) {
		t.Fatal("not on the battlefield")
	}
	sl := sc.slot(aw)
	if sc.s.cardAt(sl) != back || sc.s.objs[aw].card != back {
		t.Fatal("the permanent is not its back face")
	}
	if sc.s.perms[sl].flags&pfTapped == 0 {
		t.Fatal("Akoum Teeth did not enter tapped")
	}
	if sc.s.isCreature(sc.k, sl) || sc.k.t.Cards[back].Types&kop.TLand == 0 {
		t.Fatal("the back face is not a land")
	}
	if sc.s.pl[0].landsPlayed != 1 {
		t.Fatal("the land play was not counted")
	}
	var p Projection
	sc.s.Project(&p)
	if p.Objs[aw].Card != back {
		t.Fatal("projection: not the back face")
	}
	sc.s.leave(sc.k, sl, kop.ZoneHand, false)
	if sc.s.objs[aw].card != front {
		t.Fatal("back in hand: not the front face")
	}
	o := sc.perm(back, 0) // a battlefield back face that dies
	sc.s.leave(sc.k, sc.slot(o), kop.ZoneGraveyard, false)
	if sc.s.objs[o].card != front || sc.s.objs[o].zone != kop.ZoneGraveyard {
		t.Fatal("in the graveyard: not the front face")
	}
}

// TestSacrificedPowerOverTarget: Kazuul's Fury sacrificing Grizzly Bears
// deals 2 to a targeted Hill Giant: the sacrificed creature's last known
// power (RefSacrificed), not the target's, which a targeted spell's
// resolution used to snapshot over it (lockstep faces matchup 0 seed 18).
func TestSacrificedPowerOverTarget(t *testing.T) {
	sc, id := facesScen(t, "Kazuul's Fury", "Grizzly Bears", "Hill Giant", "Mountain")
	for i := 0; i < 3; i++ {
		sc.perm(id["Mountain"], 0)
	}
	gb := sc.perm(id["Grizzly Bears"], 0)
	hg := sc.perm(id["Hill Giant"], 1)
	kf := sc.hand(id["Kazuul's Fury"], 0)
	sc.castCard(t, kf)
	sc.do(t, AkTarget, 0xff, uint8(hg))
	sc.do(t, AkChoose, uint8(gb), 0xff)
	for i := 0; i < 4 && sc.s.sp > 0; i++ {
		sc.do(t, AkPass, 0xff, 0xff)
	}
	if !sc.onBF(hg) || sc.s.perms[sc.slot(hg)].damage != 2 {
		t.Fatalf("Hill Giant on the battlefield %v, want 2 damage", sc.onBF(hg))
	}
}
