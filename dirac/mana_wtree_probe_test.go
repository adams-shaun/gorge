package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// fullReproBoard mirrors the divergence board of tail-08 pair 10 (seed
// 13196970261271822978): p1's permanents and p0's, The World Tree among them.
func fullReproBoard(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	set := cardsettest.Synth(t, "tail-08")
	var deck []cardset.Entry
	for _, e := range []cardset.Entry{
		{Name: "The World Tree", Count: 1},
		{Name: "Swamp", Count: 9},
		{Name: "Forest", Count: 5},
		{Name: "Sunlit Marsh", Count: 1},
		{Name: "Library", Count: 1},
		{Name: "Dwarven Hold", Count: 1},
		{Name: "Ana Disciple", Count: 1},
		{Name: "Propagator Drone", Count: 1},
		{Name: "Steel Wall", Count: 1},
		{Name: "Yukora, the Prisoner", Count: 1},
		{Name: "Hornet Nest", Count: 1},
		{Name: "Mind Slash", Count: 1},
		{Name: "That Which Was Taken", Count: 1},
		{Name: "Puca's Covenant", Count: 1},
		{Name: "Pierce Strider", Count: 1},
	} {
		deck = append(deck, e)
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if id["The World Tree"] == 0 {
		t.Skip("The World Tree is not in the tail-08 table")
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true})
	// p1's board: ten untapped lands (one with two storage and a divinity
	// counter), Steel Wall, Yukora, Hornet Nest, Ana Disciple, Propagator
	// Drone.
	p1 := []string{"The World Tree", "Sunlit Marsh", "Library", "Dwarven Hold"}
	for _, n := range p1 {
		sc.perm(id[n], 1)
	}
	for i := 0; i < 3; i++ {
		sc.perm(id["Swamp"], 1)
		sc.perm(id["Forest"], 1)
	}
	hold := NoSlot
	for o := Obj(0); o < Obj(sc.s.nobj); o++ {
		if ob := &sc.s.objs[o]; ob.zone == kop.ZoneBattlefield && tbl.Cards[ob.card].Name == "Dwarven Hold" {
			hold = ob.slot
		}
	}
	sc.s.perms[hold].addCtrN(sc.s, int(kop.CtrStorage), 2)
	// every p1 permanent untapped and ready
	for o := Obj(0); o < Obj(sc.s.nobj); o++ {
		if ob := &sc.s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.acts>>objOwnerShift == 1 {
			sc.s.perms[ob.slot].flags &^= pfTapped | pfSick
		}
	}
	return sc, id
}

func TestWTreeProbeFullBoard(t *testing.T) {
	sc, id := fullReproBoard(t)
	k := sc.k
	wt := sc.slotOf(id["The World Tree"], 1)
	cd := &k.t.Cards[sc.s.cardAt(wt)]
	var av avail
	sc.s.availFor(k, 1, &av)
	t.Logf("avail p1: total=%d n=%d masks=%v cnts=%v", av.total, av.n, av.mask[:av.n], av.cnt[:av.n])
	t.Logf("canActivate(WT ab1)=%v canTapForMana=%v ownManaUsable(WT)=%v",
		sc.s.canActivate(k, wt, 1, &cd.Abilities[1], true, &av), sc.s.canTapForMana(k, wt), sc.s.ownManaUsable(k, wt))
	hold := sc.slotOf(id["Dwarven Hold"], 1)
	pc := sc.s.manaPCGen(k, hold)
	t.Logf("Hold pc: manaAb=%d manaCol=%d manaN=%d ownManaUsable=%v",
		pc.manaAb, pc.manaCol, pc.manaN, sc.s.ownManaUsable(k, hold))
	d := sc.s.Step()
	for i := 0; i < 40 && (d.Kind != DkPriority || d.Player != 1); i++ {
		if d.N > 0 {
			sc.s.Apply(d.Options[0])
		} else {
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
		d = sc.s.Step()
	}
	if d.Kind != DkPriority || d.Player != 1 {
		t.Fatalf("no p1 priority decision reached: kind %d player %d", d.Kind, d.Player)
	}
	t.Logf("priority player=%d", d.Player)
	wtObj := sc.s.perms[wt].obj
	offered := 0
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkActivate && a.Source() == TCode(wtObj) {
			offered++
		}
	}
	// the WT activation's grant-search verdict and node count
	wa := sc.k.t.Cards[sc.s.cardAt(wt)].Abilities[1]
	t.Logf("canActivate=%v grantSearch: nodes=%d np=%d found=%v",
		sc.s.canActivate(k, wt, 1, &wa, true, &av), dbgGrantSrch.nodes, dbgGrantSrch.np, dbgGrantSrch.found)
	for i, ln := range dbgGrantSrch.trace {
		t.Logf("trace[%d]: %s", i, ln)
	}
	ctr := sc.s.scriptCtr(k, 1)
	if ca := k.ctrAbOf(sc.s.cardAt(hold)); ca != nil {
		t.Logf("Hold ctrAb: flags=%d ctr=%d N=%d ctrN=%d store=%v dead=%v actUsed=%d tapped=%v",
			ca.Cost.Flags, ca.Cost.Ctr, ca.Cost.N, sc.s.perms[hold].ctrN(sc.s, int(ca.Cost.Ctr)), kop.StoreManaAb(ca), deadCtrAb(ca), sc.s.perms[hold].actUsed, sc.s.perms[hold].flags&pfTapped != 0)
	}
	t.Logf("scriptCtr=%v holdIn=%v ctrAb(Hold)=%v scriptAny=%v availX=%d",
		ctr, ctr.has(hold), k.ctrAbOf(sc.s.cardAt(hold)) != nil, sc.s.scriptAny(sc.k), sc.s.g.availX)
	var sd Decision
	wasX := sc.s.g.availX
	sc.s.scriptOffers(k, &sd, new(Decision), 1)
	sc.s.g.availX = wasX
	soffered := 0
	for i := 0; i < int(sd.N); i++ {
		a := sd.Options[i]
		if a.Kind() == AkActivate && a.Source() == TCode(wtObj) {
			soffered++
		}
	}
	t.Logf("scriptOffers: n=%d wtOffers=%d", sd.N, soffered)
	t.Fatalf("offered=%d of %d options", offered, d.N)
}

// objAt finds the battlefield slot of card c under seat p.
func (sc *scen) slotOf(c CardID, p uint8) Slot {
	for o := Obj(0); o < Obj(sc.s.nobj); o++ {
		if ob := &sc.s.objs[o]; ob.zone == kop.ZoneBattlefield && ob.card == c && ob.acts>>objOwnerShift == p {
			return ob.slot
		}
	}
	return NoSlot
}

func (sc *scen) objs(o int) Obj { return Obj(o) }

func (sc *scen) tapPerm(c CardID, p uint8) {
	sl := sc.slotOf(c, p)
	if sl == NoSlot {
		return
	}
	sc.s.perms[sl].flags |= pfTapped
}
