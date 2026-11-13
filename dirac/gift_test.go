package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Gift (CR 702.174, kw_gift.go) on BLB's table: the election after the push,
// the promised slot ranges, the spliced gift, a permanent's gift trigger
// and Jolly Gerbils' "whenever you give a gift".

func giftScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Set(t, "blb")
	for _, n := range names {
		if i, ok := set.Lookup(n); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the BLB table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// castCard casts o from priority (its first cast mode).
func (sc *scen) castCard(t *testing.T, o Obj) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("cast of %d not offered: %v", o, d.Options[:d.N])
}

// gift answers the gift election.
func (sc *scen) gift(t *testing.T, promise bool) {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenGift || d.N != 2 {
		t.Fatalf("decision kind %d sub %d n %d, want the gift election", d.Kind, d.Sub, d.N)
	}
	m := uint8(0)
	if promise {
		m = 1
	}
	sc.s.Apply(MakeAction(AkChoose, 0, 0, m))
}

// resolveAll passes until the stack is empty, ordering triggers and
// choosing the first target when asked.
func (sc *scen) resolveAll(t *testing.T) {
	t.Helper()
	for i := 0; i < 60; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority && sc.s.sp == 0 && sc.s.npend == 0 {
			return
		}
		if d.Kind == DkPriority {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		sc.s.Apply(d.Options[0])
	}
	t.Fatal("the stack never emptied")
}

func TestGiftWearDownPromised(t *testing.T) {
	sc, id := giftScen(t, "Wear Down", "Forest", "Fountainport Bell", "Short Bow", "Jolly Gerbils")
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Jolly Gerbils"], 0)
	bell := sc.perm(id["Fountainport Bell"], 1)
	bow := sc.perm(id["Short Bow"], 1)
	wd := sc.hand(id["Wear Down"], 0)
	h0, h1 := sc.s.pl[0].handN, sc.s.pl[1].handN
	sc.castCard(t, wd)
	sc.gift(t, true)
	if sc.s.objs[wd].zone != kop.ZoneStack {
		t.Fatal("Wear Down is not on the stack at the gift election (CR 601.2a)")
	}
	if sc.s.stack[sc.s.sp-1].flags&sfPromised == 0 {
		t.Fatal("the promise is not on the stack entry")
	}
	sc.do(t, AkTarget, 0xff, uint8(bell))
	sc.do(t, AkTarget, 0xff, uint8(bow))
	sc.resolveAll(t)
	if sc.onBF(bell) || sc.onBF(bow) {
		t.Fatalf("bell alive=%v bow alive=%v; the promised Wear Down destroys both", sc.onBF(bell), sc.onBF(bow))
	}
	if got := sc.s.pl[1].handN; got != h1+1 {
		t.Fatalf("opponent hand %d, want %d (the gift: a card)", got, h1+1)
	}
	if got := sc.s.pl[0].handN; got != h0 {
		t.Fatalf("caster hand %d, want %d (Wear Down cast, Jolly Gerbils drew)", got, h0)
	}
}

func TestGiftWearDownDeclined(t *testing.T) {
	sc, id := giftScen(t, "Wear Down", "Forest", "Fountainport Bell", "Short Bow", "Jolly Gerbils")
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 0)
	}
	sc.perm(id["Jolly Gerbils"], 0)
	bell := sc.perm(id["Fountainport Bell"], 1)
	bow := sc.perm(id["Short Bow"], 1)
	wd := sc.hand(id["Wear Down"], 0)
	h0, h1 := sc.s.pl[0].handN, sc.s.pl[1].handN
	sc.castCard(t, wd)
	sc.gift(t, false)
	sc.do(t, AkTarget, 0xff, uint8(bell))
	if d := sc.s.Step(); d.Kind == DkTarget {
		t.Fatal("a second target asked without the gift")
	}
	sc.resolveAll(t)
	if sc.onBF(bell) || !sc.onBF(bow) {
		t.Fatalf("bell alive=%v bow alive=%v; want only the bell destroyed", sc.onBF(bell), sc.onBF(bow))
	}
	if sc.s.pl[1].handN != h1 || sc.s.pl[0].handN != h0-1 {
		t.Fatalf("hands %d/%d, want %d/%d: no gift, no Gerbils draw", sc.s.pl[0].handN, sc.s.pl[1].handN, h0-1, h1)
	}
}

// TestGiftElectionWithoutABranchTarget: Into the Flood Maw is offered when
// either election can be announced (the promised one: its second slot is
// chosen as it resolves); declining with no creature to return reverses
// the cast (gorge: "cast aborted: no legal target").
func TestGiftElectionWithoutABranchTarget(t *testing.T) {
	sc, id := giftScen(t, "Into the Flood Maw", "Island", "Fountainport Bell")
	sc.perm(id["Island"], 0)
	bell := sc.perm(id["Fountainport Bell"], 1)
	m := sc.hand(id["Into the Flood Maw"], 0)
	sc.castCard(t, m)
	sc.gift(t, false)
	if d := sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision kind %d after the declined election, want priority", d.Kind)
	}
	if sc.s.objs[m].zone != kop.ZoneHand || sc.s.sp != 0 {
		t.Fatalf("declined Into the Flood Maw: zone %d stack %d, want the cast reversed", sc.s.objs[m].zone, sc.s.sp)
	}
	sc.castCard(t, m)
	sc.gift(t, true)
	sc.resolveAll(t)
	if sc.onBF(bell) || sc.s.objs[bell].zone != kop.ZoneHand {
		t.Fatal("the promised Into the Flood Maw did not return the Bell")
	}
	fish := 0
	for m := sc.s.ctrlM[1]; m.any(); m = m.rest() {
		if p := &sc.s.perms[m.first()]; sc.s.objs[p.obj].of&ofToken != 0 && p.flags&pfTapped != 0 {
			fish++
		}
	}
	if fish != 1 {
		t.Fatalf("opponent tapped tokens %d, want the gift's tapped Fish", fish)
	}
}

// TestGiftPermanentTrigger: a promised Scrapshooter's "if the gift was
// promised" enter trigger and its gift trigger both go on the stack; the
// gift is given as its trigger is pushed (Jolly Gerbils draws).
func TestGiftPermanentTrigger(t *testing.T) {
	for _, promise := range []bool{false, true} {
		sc, id := giftScen(t, "Scrapshooter", "Forest", "Fountainport Bell", "Jolly Gerbils")
		for i := 0; i < 3; i++ {
			sc.perm(id["Forest"], 0)
		}
		sc.perm(id["Jolly Gerbils"], 0)
		bell := sc.perm(id["Fountainport Bell"], 1)
		ss := sc.hand(id["Scrapshooter"], 0)
		h0, h1 := sc.s.pl[0].handN, sc.s.pl[1].handN
		sc.castCard(t, ss)
		sc.gift(t, promise)
		sc.resolveAll(t)
		if !sc.onBF(ss) {
			t.Fatal("Scrapshooter did not enter")
		}
		want0, want1 := h0-1, h1
		if promise {
			want0, want1 = h0, h1+1
		}
		if sc.onBF(bell) == promise || sc.s.pl[0].handN != want0 || sc.s.pl[1].handN != want1 {
			t.Fatalf("promise=%v: bell alive=%v hands %d/%d, want alive=%v %d/%d", promise, sc.onBF(bell),
				sc.s.pl[0].handN, sc.s.pl[1].handN, !promise, want0, want1)
		}
	}
}
