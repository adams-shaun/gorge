package dirac

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestWardPaysInBattlefieldOrder: a ward cost is paid as gorge's mana
// window is answered (rules/ward.go wardManaDecision; the oracle's
// payWindowAnswer): the first untapped mana sources in the payer's
// battlefield list, not the cast planner's pick, which keeps the Islands
// for the blue card in hand (pairs 9 96 98 and 105 more in the 10k slice).
func TestWardPaysInBattlefieldOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "Bake into a Pie", Count: 1}, {Name: "Tolarian Terror", Count: 2}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 10}}
	fdnSet := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := fdnSet.Lookup(e.Name); !ok || !fdnSet.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	isl := [2]Obj{sc.perm(id["Island"], 0), sc.perm(id["Island"], 0)}
	var sw []Obj
	for i := 0; i < 6; i++ {
		sw = append(sw, sc.perm(id["Swamp"], 0))
	}
	terror := sc.perm(id["Tolarian Terror"], 1)
	sc.hand(id["Tolarian Terror"], 0)
	pie := sc.hand(id["Bake into a Pie"], 0)
	sc.do(t, AkCast, uint8(pie), 0xff)
	for i := 0; i < 12; i++ {
		d := s.Step()
		if d.Kind == DkTarget {
			s.Apply(MakeAction(AkTarget, 0, TCode(terror), 0))
			continue
		}
		if d.Kind == DkChoose && d.Sub == ChWardPay {
			for _, a := range d.Options[:d.N] {
				if a.Mode() == 1 {
					s.Apply(a)
				}
			}
			break
		}
		s.Apply(d.Options[0])
	}
	if tappedN(sc, isl[:]) != 2 {
		t.Fatalf("ward {2} tapped %d of the first-listed Islands, want both (Swamps tapped %d)", tappedN(sc, isl[:]), tappedN(sc, sw))
	}
}

// TestPayWindowTapsLowestColour: a window payment (ward, unless) taps a
// two-colour source for its lowest colour whatever the cost still owes. The
// window's decisions carry no ManaPayment, so the oracle's window answer
// (payWindowAnswer) has no owed colour to prefer: Wind-Scarred Crag pays {R}
// with W, the pool keeps the W, and the payment is short.
func TestPayWindowTapsLowestColour(t *testing.T) {
	deck := []cardset.Entry{{Name: "Wind-Scarred Crag", Count: 1}, {Name: "Mountain", Count: 5}}
	fdnSet := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := fdnSet.Lookup(e.Name); !ok || !fdnSet.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	crag := sc.perm(id["Wind-Scarred Crag"], 0)
	sc.s.perms[sc.slot(crag)].flags &^= pfTapped // it entered long ago
	c := cost(0, kop.R)
	_, short := sc.s.payWindow(sc.k, 0, &c)
	var want [kop.NumManaKinds]uint8
	want[kop.W] = 1
	if !sc.tapped(crag) || !short || sc.s.pl[0].pool != want {
		t.Fatalf("Crag tapped %v, short %v, pool left %v: want W made and floating, the {R} unpaid", sc.tapped(crag), short, sc.s.pl[0].pool)
	}
}

// TestWardPayWithBloomTender: a ward {2} payment window activates Bloom
// Tender (one mana of each colour among its controller's permanents) as
// well as Llanowar Elves, as gorge's window does (the pair 1107: Bite
// Down on Cackling Prowler). Before, the kernel's census skipped the
// Tender, found {2} unaffordable and countered the spell.
func TestWardPayWithBloomTender(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Bite Down", Count: 1}, {Name: "Bloom Tender", Count: 1}, {Name: "Llanowar Elves", Count: 1}, {Name: "Cackling Prowler", Count: 1}, {Name: "Forest", Count: 8}})
	sc.perm(id["Forest"], 0)
	sc.perm(id["Forest"], 0)
	sc.perm(id["Llanowar Elves"], 0)
	bt := sc.perm(id["Bloom Tender"], 0)
	pr := sc.perm(id["Cackling Prowler"], 1)
	bd := sc.hand(id["Bite Down"], 0)
	cast := false
	sc.stepTo(t, 30, func() bool { return cast && sc.s.objs[bd].zone == kop.ZoneGraveyard }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			switch {
			case a.Kind() == AkCast && a.Source() == TCode(uint8(bd)):
				cast = true
				return a, true
			case d.Kind == DkTarget && a.Target() == TCode(uint8(pr)):
				return a, true
			case d.Kind == DkChoose && d.Sub == ChWardPay && a.Mode() == 1:
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if !cast || sc.s.objs[bd].zone != kop.ZoneGraveyard {
		t.Fatalf("Bite Down cast %v zone %d", cast, sc.s.objs[bd].zone)
	}
	if !sc.tapped(bt) {
		t.Fatal("Bloom Tender untapped: the ward {2} was not paid with it")
	}
	if sc.onBF(pr) && sc.s.perms[sc.slot(pr)].damage == 0 {
		t.Fatal("Cackling Prowler undamaged: Bite Down was countered by ward")
	}
}

// TestWardFailedPaySacrificesTreasure: the payer chooses to pay ward {2}
// with only a Treasure untapped; gorge's mana window is answered by
// activating every source (the Treasure, sacrificed, its first colour W)
// before it gives up, so the spell is countered with {W} floating and the
// Treasure gone (oracle payWindowAnswer; pair 34385 and four more). Before,
// the kernel's failed payment skipped Treasures.
func TestWardFailedPaySacrificesTreasure(t *testing.T) {
	deck := []cardset.Entry{{Name: "Bake into a Pie", Count: 1}, {Name: "Tolarian Terror", Count: 1}, {Name: "Goldvein Pick", Count: 1}, {Name: "Swamp", Count: 10}}
	fdnSet := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := fdnSet.Lookup(e.Name); !ok || !fdnSet.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	treasure := NoCard
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == "c_a_treasure_sac" || tbl.Cards[i].Name == "Treasure" {
			treasure = CardID(i)
		}
	}
	if treasure == NoCard || int(v.Plains) >= len(tbl.Cards) {
		t.Skip("no Treasure token in the table")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	s := sc.s
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	tr := sc.perm(treasure, 0)
	s.objs[tr].of |= ofToken
	terror := sc.perm(id["Tolarian Terror"], 1)
	pie := sc.hand(id["Bake into a Pie"], 0)
	sc.do(t, AkCast, uint8(pie), 0xff)
	for i := 0; i < 12; i++ {
		d := s.Step()
		if d.Kind == DkTarget {
			s.Apply(MakeAction(AkTarget, 0, TCode(terror), 0))
			continue
		}
		if d.Kind == DkChoose && d.Sub == ChWardPay {
			s.Apply(MakeAction(AkChoose, 0, 0, 1))
			if sc.onBF(tr) || s.pl[0].pool[kop.W] != 1 || s.objs[pie].zone != kop.ZoneGraveyard {
				t.Fatalf("after the failed ward payment: Treasure on the battlefield %v, {W} floating %d, Bake into a Pie zone %d; want it gone, 1, the graveyard", sc.onBF(tr), s.pl[0].pool[kop.W], s.objs[pie].zone)
			}
			return
		}
		s.Apply(d.Options[0])
	}
	t.Fatal("no ward ask")
}

// TestCastWindowWithholdsUselessSource: a cast's CR 601.2g mana window
// (a lowered or free cast paying the rest of its cost) offers only the
// sources that can still add a colour the payment absorbs (gorge's
// paymentSourceUseful): once the pool covers the generic, a Mountain is
// withheld while only a {B} pip is owed. Eaten Alive's {3}{B} rider with
// {B} floating (cost {3}{B}{B}): three Mountains and the Swamp, nothing
// floating. Before, the kernel tapped all five Mountains first and left
// RR floating (pair 10163; 20476 with an Island).
func TestCastWindowWithholdsUselessSource(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5}})
	var mt [5]Obj
	for i := range mt {
		mt[i] = sc.perm(id["Mountain"], 0)
	}
	sw := sc.perm(id["Swamp"], 0)
	sc.s.pl[0].pool[kop.B] = 1
	c := kop.ManaCost{Generic: 3}
	c.Pips[kop.B] = 2
	sc.s.payCastWindow(sc.k, 0, &c, false, false)
	n := 0
	for _, o := range mt {
		if sc.tapped(o) {
			n++
		}
	}
	if n != 3 || !sc.tapped(sw) || sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Fatalf("Mountains tapped %d, Swamp tapped %v, pool left %v: want 3, the Swamp, nothing floating", n, sc.tapped(sw), sc.s.pl[0].pool)
	}
	// an any-type rider (Tinybones' stash) keeps every source offered:
	// gorge's clamp fails open
	sc2, id2 := newObjScen(t, []cardset.Entry{{Name: "Mountain", Count: 5}, {Name: "Swamp", Count: 5}})
	for range mt {
		sc2.perm(id2["Mountain"], 0)
	}
	sc2.perm(id2["Swamp"], 0)
	sc2.s.pl[0].pool[kop.B] = 1
	sc2.s.payCastWindow(sc2.k, 0, &c, true, false)
	if sc2.s.pl[0].pool[kop.R] != 2 {
		t.Fatalf("any-type window pool %v, want the open window's RR floating", sc2.s.pl[0].pool)
	}
}

// TestWardGrantedKeepsPrintedLifeWard: Zul'Ashur (printed ward--pay 2 life)
// enchanted by Combat Research (granted ward {1}) queues the life ward and then
// the granted one when the opponent targets it (CR 113.2c).
func TestWardGrantedKeepsPrintedLifeWard(t *testing.T) {
	deck := []cardset.Entry{{Name: "Zul Ashur, Lich Lord", Count: 1}, {Name: "Combat Research", Count: 1}, {Name: "Shock", Count: 1}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}}
	kwardSet := cardsettest.Set(t, "ward")
	tbl, da, _, err := kwardSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	s, k := sc.s, sc.k
	if k.gen == nil || !k.gen.anyWardGrant {
		t.Fatal("table has no granted ward")
	}
	z := sc.perm(id["Zul Ashur, Lich Lord"], 0)
	cr := sc.perm(id["Combat Research"], 0)
	s.attachTo(k, sc.slot(cr), sc.slot(z))
	s.refreshTy(k)
	shock := sc.obj(id["Shock"], 1)
	e := &stackEntry{obj: shock, ctrl: 1}
	s.npend = 0
	s.targetedOne(k, e, uint8(z))
	if s.npend != 2 || s.pend[0].ab != abWard || s.pend[0].trigAmt != -2 || s.pend[1].trigAmt != 1|wardGrantedBits {
		t.Fatalf("pending %d: %+v, want the pay-2-life ward, then the granted ward {1}", s.npend, s.pend[:s.npend])
	}
}

// TestWardPayTapAnotherPermanent: the EOE-s193 ward {2}. The payer's only
// untapped mana after casting is one Mountain and Gene Pollinator ("{T}, tap an
// untapped permanent you control: add one mana of any colour"), whose cost the
// payment census leaves out. gorge's window taps the Mountain, then offers
// Gene Pollinator and asks which permanent to tap; the ward is paid and the
// spell resolves (gaining control). Before, the pay answer fell to the
// unpayable arm, tapped only the Mountain and countered the spell.
func TestWardPayTapAnotherPermanent(t *testing.T) {
	sc, id := synthScen(t, "std-eoe", "Systems Override", "Mountain", "Gene Pollinator", "Frontline War-Rager", "Harmonious Grovestrider", "Forest")
	var mt [4]Obj
	for i := range mt {
		mt[i] = sc.perm(id["Mountain"], 0)
	}
	gp := sc.perm(id["Gene Pollinator"], 0)
	spare := sc.perm(id["Frontline War-Rager"], 0)
	for i := 0; i < 2; i++ {
		sc.perm(id["Forest"], 1) // Harmonious Grovestrider's lands-you-control P/T
	}
	warded := sc.perm(id["Harmonious Grovestrider"], 1)
	spell := sc.hand(id["Systems Override"], 0)
	cast := false
	sc.stepTo(t, 60, func() bool { return sc.s.objs[spell].zone == kop.ZoneGraveyard && sc.s.sp == 0 }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			a := d.Options[i]
			switch {
			case a.Kind() == AkCast && a.Source() == TCode(uint8(spell)):
				cast = true
				return a, true
			case d.Kind == DkTarget && a.Target() == TCode(uint8(warded)):
				return a, true
			case d.Kind == DkChoose && d.Sub == ChWardPay && a.Mode() == 1:
				return a, true
			case d.Kind == DkChoose && d.Sub == ChWindowTap && a.Target() == TCode(uint8(spare)):
				return a, true
			}
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Kind() == AkPass {
				return d.Options[i], true
			}
		}
		return 0, false
	})
	if !cast {
		t.Fatal("Systems Override was never cast")
	}
	if !sc.tapped(gp) || !sc.tapped(spare) {
		t.Fatalf("Gene Pollinator tapped %v, spare tapped %v: the ward {2} was not paid through Gene Pollinator", sc.tapped(gp), sc.tapped(spare))
	}
	if !sc.s.ctrlM[0].has(sc.slot(warded)) {
		t.Fatal("Harmonious Grovestrider is not controlled by p0: the spell was countered by the ward")
	}
}

// TestStackedWardTriggersLeaveSameStack: Realm-Scorcher Hellkite activates
// three times at a Ward {2} Toadstool Admirer wearing a Royal Role (Ward {1}),
// so each activation queues two ward triggers (CR 603.3b). With one spare
// mana the top trigger (Ward {1} of the third activation) is paid; the next
// (Ward {2}, same activation) cannot be and counters it; the Role's Ward {1}
// of the second activation counters that one; its Ward {2} trigger then has
// nothing left to counter and leaves with no ask, as gorge's does (kfuzz
// WOE-s222). dirac answered it with the first activation's still-targeting
// ability, asked for a payment, and kept the trigger on the stack.
func TestStackedWardTriggersLeaveSameStack(t *testing.T) {
	deck := []cardset.Entry{{Name: "Realm-Scorcher Hellkite", Count: 1}, {Name: "Toadstool Admirer", Count: 1}, {Name: "Redtooth Genealogist", Count: 1}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 4}}
	sc, id := xScen(t, "std-woe", deck)
	s := sc.s
	adm := sc.perm(id["Toadstool Admirer"], 1)
	hk := sc.perm(id["Realm-Scorcher Hellkite"], 0)
	gen := sc.hand(id["Redtooth Genealogist"], 1)
	for i := 0; i < 3; i++ {
		sc.perm(id["Forest"], 1)
	}
	for i := 0; i < 7; i++ {
		sc.perm(id["Mountain"], 0)
	}
	// p1 mints the Role on its Admirer, then p0 holds priority with 7 Mountains
	s.active, s.prio = 1, 1
	sc.do(t, AkCast, uint8(gen), 0xff)
	for i := 0; i < 40 && !(s.sp == 0 && s.ctl == ctlPriority && s.passes == 0 && s.prio == 1 && s.nobj > 0 && roleOn(sc, adm)); i++ {
		d := s.Step()
		switch {
		case d.Kind == DkTarget:
			s.Apply(MakeAction(AkTarget, 0, TCode(adm), 0))
		case d.Kind == DkPriority:
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			s.Apply(d.Options[0])
		}
	}
	if !roleOn(sc, adm) {
		t.Fatal("precondition: the Admirer wears no Role")
	}
	if s.sp != 0 {
		t.Fatalf("precondition: %d entries on the stack after the Role", s.sp)
	}
	s.active, s.prio, s.passes, s.ctl = 0, 0, 0, ctlPriority
	acts := 0
	var sizes, asks []int
	last := -1
	for i := 0; i < 200 && !(acts == 3 && s.sp == 0); i++ {
		d := s.Step()
		switch {
		case d.Kind == DkTarget:
			s.Apply(MakeAction(AkTarget, 0, TCode(adm), 0))
		case d.Kind == DkChoose && d.Sub == ChWardPay:
			asks = append(asks, int(s.sp))
			for _, a := range d.Options[:d.N] {
				if a.Mode() == 1 {
					s.Apply(a)
				}
			}
		case d.Kind == DkPriority && acts < 3 && int(s.sp) == 3*acts && s.prio == 0:
			for j := 0; j < int(d.N); j++ {
				if a := d.Options[j]; a.Kind() == AkActivate && a.Source() == TCode(uint8(hk)) {
					s.Apply(a)
					acts++
					break
				}
			}
		case d.Kind == DkPriority:
			if int(s.sp) != last {
				sizes = append(sizes, int(s.sp))
				last = int(s.sp)
			}
			s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			s.Apply(d.Options[0])
		}
	}
	// gorge (WOE-s222): 9 entries, then 8, 6 and 4 after the first three
	// resolutions, and 3 after the fourth, which asked nothing
	if got := fmt.Sprint(sizes); !strings.HasPrefix(got, "[9 8 6 4 3 ") {
		t.Fatalf("stack after each resolution %v, want 9 8 6 4 3 ...", sizes)
	}
	if got := fmt.Sprint(asks); got != "[9 8 6 3]" {
		t.Fatalf("ward pay asked with %v on the stack, want [9 8 6 3]: the trigger of a countered ability asks nothing", asks)
	}
}

func roleOn(sc *scen, o Obj) bool {
	for i := Obj(0); i < Obj(sc.s.nobj); i++ {
		ob := &sc.s.objs[i]
		if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot && sc.s.perms[ob.slot].attached == sc.slot(o) {
			return true
		}
	}
	return false
}
