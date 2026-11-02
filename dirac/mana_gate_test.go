package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEntersTappedCondition: a conditional enters-tapped land's ConditionPresent$
// count is made for the entering permanent's controller, EXCLUDING the
// entering land itself (gorge conditionMetBattlefield; the kernel clears the
// object from occ/ctrlM before the replacement runs).
func TestEntersTappedCondition(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Blooming Marsh", Count: 1},
		{Name: "Deathcap Glade", Count: 1},
		{Name: "Dedicated Commons", Count: 1},
		{Name: "Ajani, Caller of the Pride", Count: 1},
		{Name: "Plains", Count: 20},
	}
	set := cardsettest.Set(t, "enterstapped")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib, ok := id["Plains"]
	if !ok {
		t.Fatal("Plains not in the enterstapped table")
	}
	// play puts carrier in hand and plays it, returning whether it entered
	// tapped.
	play := func(carrier string, otherLands int, pwOwner int) bool {
		sc := newScenTableLib(t, tbl, lib, Config{NoElide: true})
		for i := 0; i < otherLands; i++ {
			sc.perm(id["Plains"], 0)
		}
		if pwOwner >= 0 {
			sc.perm(id["Ajani, Caller of the Pride"], uint8(pwOwner))
		}
		o := sc.hand(id[carrier], 0)
		sc.s.playLandNow(sc.k, o, 0)
		return sc.s.perms[sc.slot(o)].flags&pfTapped != 0
	}
	cases := []struct {
		name       string
		carrier    string
		otherLands int
		pwOwner    int
		wantTapped bool
	}{
		// fast land: enters untapped with two or fewer OTHER lands
		{"fast two others", "Blooming Marsh", 2, -1, false},
		{"fast three others", "Blooming Marsh", 3, -1, true},
		// slow land: enters tapped with one or fewer OTHER lands
		{"slow one other", "Deathcap Glade", 1, -1, true},
		{"slow two others", "Deathcap Glade", 2, -1, false},
		// planeswalker: only YOUR planeswalker untaps it
		{"pw own", "Dedicated Commons", 0, 0, false},
		{"pw opponent", "Dedicated Commons", 0, 1, true},
	}
	for _, c := range cases {
		if got := play(c.carrier, c.otherLands, c.pwOwner); got != c.wantTapped {
			t.Errorf("%s: tapped=%v, want %v", c.name, got, c.wantTapped)
		}
	}
}

// manaGateScen builds a enterstapped scenario at seat 0's main phase 1 with an empty
// hand and the given cards in the table.
func manaGateScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "enterstapped")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib, ok := id["Plains"]
	if !ok {
		t.Fatal("Plains not in the enterstapped table")
	}
	return newScenTableLib(t, tbl, lib, Config{NoElide: true}), id
}

// manaGateRes makes o the resolving ability's source for p (gorge's "the
// ability's host"): resSrcSlot then reads o and its current ver.
func manaGateRes(sc *scen, o Obj, p uint8) {
	sc.s.res.e = stackEntry{kind: seTrigger, obj: o, srcVer: sc.s.objs[o].ver, card: sc.s.objs[o].card, ctrl: p}
	sc.s.res.active = true
}

// TestSourceCounterPayCost: the B/C pay kinds read the resolving source's own state
// (gorge trigger_cost_subcounter.go:91, unless_cost.go:47, cumulative.go:1130)
// and the payer is the resolving ability's controller (UnlessPayer$ You).
func TestSourceCounterPayCost(t *testing.T) {
	// a fixed-kind SubCounter is payable only from the source's own
	// counters, of exactly that kind.
	t.Run("B1", func(t *testing.T) {
		sc, id := manaGateScen(t, []cardset.Entry{
			{Name: "Guiding Hydra", Count: 1},
			{Name: "Atraxa's Skitterfang", Count: 1},
			{Name: "Plains", Count: 20},
		})
		hydra := sc.perm(id["Guiding Hydra"], 0)
		manaGateRes(sc, hydra, 0)
		if sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 1, int16(kop.CtrP1P1), 0) {
			t.Error("Guiding Hydra without a +1/+1 counter: SubCounter payable")
		}
		hp := &sc.s.perms[sc.slot(hydra)]
		hp.addCtrN(sc.s, int(kop.CtrOil), 1)
		if sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 1, int16(kop.CtrP1P1), 0) {
			t.Error("Guiding Hydra with only an oil counter: +1/+1 SubCounter payable")
		}
		hp.addCtrN(sc.s, int(kop.CtrP1P1), 1)
		if !sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 1, int16(kop.CtrP1P1), 0) {
			t.Error("Guiding Hydra with a +1/+1 counter: SubCounter unpayable")
		}

		atraxa := sc.perm(id["Atraxa's Skitterfang"], 0)
		ap := &sc.s.perms[sc.slot(atraxa)]
		ap.subCtrN(sc.s, int(kop.CtrOil), ap.ctrN(sc.s, int(kop.CtrOil))) // it enters with three
		manaGateRes(sc, atraxa, 0)
		if sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 1, int16(kop.CtrOil), 0) {
			t.Error("Atraxa's Skitterfang without an oil counter: SubCounter payable")
		}
		ap.addCtrN(sc.s, int(kop.CtrOil), 1)
		if !sc.s.canPayGen(sc.k, 0, kop.PaySubCounter, 1, int16(kop.CtrOil), 0) {
			t.Error("Atraxa's Skitterfang with an oil counter: SubCounter unpayable")
		}
	})

	// Magmatic Sprinter's unless cost is paid by the ability's
	// controller (UnlessPayer$ You), never the opponent.
	t.Run("B2 payer", func(t *testing.T) {
		sc, id := manaGateScen(t, []cardset.Entry{
			{Name: "Magmatic Sprinter", Count: 1},
			{Name: "Plains", Count: 20},
		})
		sprinter := sc.perm(id["Magmatic Sprinter"], 1)
		sc.s.perms[sc.slot(sprinter)].addCtrN(sc.s, int(kop.CtrOil), 2)
		manaGateRes(sc, sprinter, 1)
		in := kop.Instr{Op: kop.OpUnlessPaid, Obj: kop.RefController, A: kop.Const(2), B: kop.Const(int(kop.CtrOil)), C: int16(kop.PaySubCounter)}
		if sc.s.execPay(sc.k, &in) {
			t.Fatal("the unless ask resolved instead of suspending")
		}
		if sc.s.res.ask != raGenPay || sc.s.res.askP != 1 || sc.s.g.pickP != 1 {
			t.Errorf("B2 payer: ask %d askP %d pickP %d, want raGenPay for the controller (1)", sc.s.res.ask, sc.s.res.askP, sc.s.g.pickP)
		}
		if !sc.s.canPayGen(sc.k, 1, kop.PaySubCounter, 2, int16(kop.CtrOil), 0) {
			t.Error("B2 with two oil counters: SubCounter unpayable")
		}
	})

	// C: a literal tapXType<N/Spec> is payable when N untapped candidates
	// exist; Card.Self is the source itself while untapped.
	t.Run("C", func(t *testing.T) {
		sc, id := manaGateScen(t, []cardset.Entry{
			{Name: "Raff, Weatherlight Stalwart", Count: 1},
			{Name: "Gitaxian Anatomist", Count: 1},
			{Name: "Grizzly Bears", Count: 1},
			{Name: "Plains", Count: 20},
		})
		raff := sc.perm(id["Raff, Weatherlight Stalwart"], 0)
		manaGateRes(sc, raff, 0)
		raffF := firstInstr(t, sc, id["Raff, Weatherlight Stalwart"], kop.OpIfPaid).F
		if sc.s.canPayGen(sc.k, 0, kop.PayTapX, 2, 0, raffF) {
			t.Error("Raff alone: tapXType<2/Creature> payable")
		}
		sc.perm(id["Grizzly Bears"], 0)
		if !sc.s.canPayGen(sc.k, 0, kop.PayTapX, 2, 0, raffF) {
			t.Error("Raff plus a Bear: tapXType<2/Creature> unpayable")
		}

		// a separate table: the Raff payment's creatures would otherwise
		// count for the Card.Self form
		sc2, id2 := manaGateScen(t, []cardset.Entry{
			{Name: "Gitaxian Anatomist", Count: 1},
			{Name: "Plains", Count: 20},
		})
		anatomist := sc2.perm(id2["Gitaxian Anatomist"], 0)
		manaGateRes(sc2, anatomist, 0)
		selfF := firstInstr(t, sc2, id2["Gitaxian Anatomist"], kop.OpIfPaid).F
		if !sc2.s.canPayGen(sc2.k, 0, kop.PayTapX, 1, 0, selfF) {
			t.Error("untapped Gitaxian Anatomist: Card.Self tapXType unpayable")
		}
		sc2.s.perms[sc2.slot(anatomist)].flags |= pfTapped
		if sc2.s.canPayGen(sc2.k, 0, kop.PayTapX, 1, 0, selfF) {
			t.Error("tapped Gitaxian Anatomist: Card.Self tapXType payable")
		}
	})
}

// TestVergeManaGate: a Verge's gated {R} is in the source's mana view only
// while its controller controls a Swamp or Mountain (gorge
// ManaActivationGateHolds, "you" = the activator); an opponent's Swamp does
// not open it, the gate is re-read after the land leaves, and a single gated
// ability (Mox Jasper) is withheld while its Dragon is absent.
func TestVergeManaGate(t *testing.T) {
	sc, id := manaGateScen(t, []cardset.Entry{
		{Name: "Blazemire Verge", Count: 1},
		{Name: "Mox Jasper", Count: 1}, {Name: "Dragon Hatchling", Count: 1},
		{Name: "Swamp", Count: 1}, {Name: "Plains", Count: 20},
	})
	mask := func(o Obj) uint8 {
		sl := sc.slot(o)
		return sc.s.srcMaskAt(sc.s.manaPCGen(sc.k, sl), sl)
	}
	const b, r = 1 << kop.B, 1 << kop.R
	v := sc.perm(id["Blazemire Verge"], 0)
	if got := mask(v); got != b {
		t.Fatalf("no Swamp/Mountain: mask %#x, want {B} %#x", got, b)
	}
	opp := sc.perm(id["Swamp"], 1)
	if got := mask(v); got != b {
		t.Fatalf("an opponent's Swamp: mask %#x, want {B} %#x", got, b)
	}
	mine := sc.perm(id["Swamp"], 0)
	if got := mask(v); got != b|r {
		t.Fatalf("own Swamp: mask %#x, want {B}{R} %#x", got, b|r)
	}
	sc.s.leave(sc.k, sc.slot(mine), kop.ZoneGraveyard, false)
	if got := mask(v); got != b {
		t.Fatalf("Swamp left: mask %#x, want {B} %#x", got, b)
	}
	_ = opp
	// a single gated ability: withheld without a Dragon
	m := sc.perm(id["Mox Jasper"], 0)
	if pc := sc.s.manaPCGen(sc.k, sc.slot(m)); pc.manaAb >= 0 {
		t.Fatalf("Mox Jasper without a Dragon offers mana (manaAb %d)", pc.manaAb)
	}
	sc.perm(id["Dragon Hatchling"], 1)
	if pc := sc.s.manaPCGen(sc.k, sc.slot(m)); pc.manaAb >= 0 {
		t.Fatalf("Mox Jasper with only the opponent's Dragon offers mana")
	}
	sc.perm(id["Dragon Hatchling"], 0)
	if pc := sc.s.manaPCGen(sc.k, sc.slot(m)); pc.manaAb < 0 {
		t.Fatalf("Mox Jasper with its own Dragon is withheld")
	}
}

// TestManaWindowGate: the mana window (windowTap, the kernel's ward/unless
// payment) sees a Verge's gated colour only while the gate holds. The kc10d
// lockstep's ward matchup (dirac/oracle/gatedmanaability_test.go, matchup 2) reaches
// windowTap with the gate on and off; this scenario reads the colour view.
//
// windowSrcMask is the exact view windowTap reads to pick a source colour
// (mana_window.go): gate off it is the first ability's colour alone, gate on the
// kmana2 union that includes the gated one. windowTap's multi-source colour
// choice still takes ability 0 (the documented kmana2 limitation), so the
// gated colour's actual production through windowTap is shown on a single
// gated ability (Mox Jasper), whose gate-on view is the plain ability.
func TestManaWindowGate(t *testing.T) {
	rbit := uint8(1 << kop.R)

	// gate off: no Swamp or Mountain under the Verge's controller
	sc, id := manaGateScen(t, []cardset.Entry{
		{Name: "Blazemire Verge", Count: 1}, {Name: "Plains", Count: 20},
	})
	v := sc.perm(id["Blazemire Verge"], 0)
	sl := sc.slot(v)
	if mask := sc.s.windowSrcMask(sc.k, sl); mask&rbit != 0 {
		t.Fatalf("gate off: the window's source mask %#x offers the gated {R}", mask)
	}
	c := cost(0, kop.R)
	if ok, _ := sc.s.windowTap(sc.k, 0, &c, false); ok {
		t.Fatal("gate off: windowTap paid {R} from Blazemire Verge (its gated ability was offered)")
	}

	// gate on: an own Swamp opens the gated {R} in the window's source view
	sc, id = manaGateScen(t, []cardset.Entry{
		{Name: "Blazemire Verge", Count: 1}, {Name: "Swamp", Count: 1}, {Name: "Plains", Count: 20},
	})
	v = sc.perm(id["Blazemire Verge"], 0)
	sc.perm(id["Swamp"], 0)
	sl = sc.slot(v)
	if mask := sc.s.windowSrcMask(sc.k, sl); mask&rbit == 0 {
		t.Fatalf("gate on: the window's source mask %#x lacks the gated {R}", mask)
	}
	c = cost(0, kop.B)
	if ok, _ := sc.s.windowTap(sc.k, 0, &c, false); !ok || !sc.tapped(v) {
		t.Fatal("gate on: windowTap did not use Blazemire Verge")
	}

	// a single gated ability is produced through windowTap: Mox Jasper's
	// gated any-colour ability pays an owed {R}
	sc, id = manaGateScen(t, []cardset.Entry{
		{Name: "Mox Jasper", Count: 1}, {Name: "Dragon Hatchling", Count: 1}, {Name: "Plains", Count: 20},
	})
	m := sc.perm(id["Mox Jasper"], 0)
	sc.perm(id["Dragon Hatchling"], 0)
	c = cost(0, kop.R)
	ok, _ := sc.s.windowTap(sc.k, 0, &c, false)
	if !ok || !sc.tapped(m) || sc.s.pl[0].pool[kop.R] == 0 {
		t.Fatalf("gate on: windowTap did not produce Mox Jasper's gated {R} (ok %v tapped %v pool %v)", ok, sc.tapped(m), sc.s.pl[0].pool)
	}
}

func kvsScen(t *testing.T, deck []cardset.Entry) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Set(t, "vs")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true}), id
}

// TestLowerColourVergeWindowAbility: gorge's "choose a mana ability" ask lists what the cost
// owes first, and the oracle answers the first option. With the gate on,
// Bleachbone (B then the gated W) makes the owed {W} and the in-order Blazemire
// (B then the gated R) the owed {R}; a failed payment's float takes ability 0.
func TestLowerColourVergeWindowAbility(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Bleachbone Verge", Count: 1}, {Name: "Blazemire Verge", Count: 1},
		{Name: "Plains", Count: 10}, {Name: "Swamp", Count: 10},
	}
	pool := func(sc *scen) [kop.NumManaKinds]uint8 { return sc.s.pl[0].pool }
	var onlyB [kop.NumManaKinds]uint8
	onlyB[kop.B] = 1

	t.Run("gate on, owes W", func(t *testing.T) {
		sc, id := kvsScen(t, deck)
		v := sc.perm(id["Bleachbone Verge"], 0)
		sc.perm(id["Swamp"], 0)
		if !sc.k.gen.anyVergeOrder {
			t.Fatal("anyVergeOrder not set")
		}
		c := cost(0, kop.W)
		sc.s.windowTap(sc.k, 0, &c, false)
		if !sc.tapped(v) || pool(sc)[kop.W] == 0 {
			t.Fatalf("window: tapped %v pool %v; want the owed {W} (gorge owedAbilitiesFirst)", sc.tapped(v), pool(sc))
		}
	})
	t.Run("float", func(t *testing.T) {
		sc, id := kvsScen(t, deck)
		v := sc.perm(id["Bleachbone Verge"], 0)
		sw := sc.perm(id["Swamp"], 0)
		sc.s.perms[sc.slot(sw)].flags |= pfTapped
		sc.s.tapAllForMana(sc.k, 0)
		if !sc.tapped(v) || pool(sc) != onlyB {
			t.Fatalf("float: tapped %v pool %v; want {B}", sc.tapped(v), pool(sc))
		}
	})
	t.Run("in-order Blazemire", func(t *testing.T) {
		sc, id := kvsScen(t, deck)
		v := sc.perm(id["Blazemire Verge"], 0)
		sc.perm(id["Swamp"], 0)
		c := cost(0, kop.R)
		sc.s.windowTap(sc.k, 0, &c, false)
		if !sc.tapped(v) || pool(sc)[kop.R] == 0 {
			t.Fatalf("Blazemire: tapped %v pool %v; want the owed {R}", sc.tapped(v), pool(sc))
		}
	})
	t.Run("gate off", func(t *testing.T) {
		sc, id := kvsScen(t, deck)
		v := sc.perm(id["Bleachbone Verge"], 0)
		c := cost(0, kop.B)
		sc.s.windowTap(sc.k, 0, &c, false)
		if !sc.tapped(v) || pool(sc) != onlyB && pool(sc) != ([kop.NumManaKinds]uint8{}) {
			t.Fatalf("gate off: tapped %v pool %v", sc.tapped(v), pool(sc))
		}
		if pool(sc)[kop.W] != 0 {
			t.Fatalf("gate off produced {W}: %v", pool(sc))
		}
	})
}
