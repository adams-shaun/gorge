package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Braided Net's "tap another target nonland permanent; its
// activated abilities can't be activated for as long as it remains tapped"
// (gorge Effect Mode$ CantBeActivated | ValidCard$ Card.IsRemembered |
// ValidSA$ Activated, Duration$ UntilTargetedUntaps; CR 602.5, 611.2b) and
// Braided Quipu's LibraryPosition$ 2.
//
// The lock is a short list of eight objects, kept in padding bytes
// (genState.nlA and nlB, State.nlC: object+1, 0 empty, compacted to the
// front so nlA[0] == 0 means none; State is unchanged). A locked object's first untap, and its leaving
// the battlefield, drop its entry for good (gorge expireUntilUntaps), so a
// retap never revives it; the source leaving changes nothing. canActivate
// refuses every ability of a locked permanent (mana abilities included;
// gorge ValidSA$ Activated). A mana ability with no tap cost on a locked
// permanent would still be spendable here, so the lock stalls loudly when
// the target carries one (StallNetMana).

// prepNetLock arms the guards (genPrep.anyNetLock) when some program holds an
// ExtLockActs.
func prepNetLock(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Ext() == kop.ExtLockActs {
					g.anyNetLock = true
					return
				}
			}
		}
	}
}

const nlN = 8 // locks held at once

func (s *State) nlAt(i int) *Obj {
	switch {
	case i < 2:
		return &s.g.nlA[i]
	case i < 4:
		return &s.g.nlB[i-2]
	}
	return &s.nlC[i-4]
}

// netLocked reports whether object o is locked.
func (s *State) netLocked(o Obj) bool {
	for i := 0; i < nlN; i++ {
		if *s.nlAt(i) == o+1 {
			return true
		}
	}
	return false
}

// netDrop ends o's lock (its untap or departure). The guard at the call
// sites is s.g.nlA[0] != 0.
//
//go:noinline
func (s *State) netDrop(o Obj) {
	s.pgDropHost(o) // the Panharmonicon grants o hosts end with its untap or departure (act_grantpanharm.go)
	if debugNetMutate == 2 {
		return // test only: the lock survives the untap
	}
	var keep [nlN]Obj
	n := 0
	for i := 0; i < nlN; i++ {
		if v := *s.nlAt(i); v != 0 && v != o+1 {
			keep[n] = v
			n++
		}
	}
	for i := 0; i < nlN; i++ {
		*s.nlAt(i) = keep[i]
	}
}

// lockActs is ExtLockActs: lock the object in in.Obj if it is a tapped
// permanent (gorge's continuousLive: the effect lives only while the host
// is a tapped permanent).
//
//go:noinline
func (s *State) lockActs(k *prep, in *kop.Instr) {
	if debugNetMutate == 1 {
		return // test only: nothing is ever locked
	}
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot || s.perms[sl].flags&pfTapped == 0 {
		return
	}
	p := &s.perms[sl]
	cd := &k.t.Cards[p.card]
	for i := range cd.Abilities {
		if a := &cd.Abilities[i]; a.Kind == kop.AbMana && a.AFlags&kop.AfLatent == 0 && a.Cost.Flags&kop.CostTap == 0 {
			s.capFull(StallNetMana)
			return
		}
	}
	if s.netLocked(p.obj) {
		return
	}
	for i := 0; i < nlN; i++ {
		if e := s.nlAt(i); *e == 0 {
			*e = p.obj + 1
			s.touchSl(sl)
			return
		}
	}
	s.capFull(StallNetLock)
}

// toLibraryThird is OpToLibrary with kop.LibThird: the object goes into its
// owner's library, then sits third from the top (gorge
// libraryOrderPlacementAt: the offset is min(2, the other cards left)).
//
//go:noinline
func (s *State) toLibraryThird(k *prep, in *kop.Instr) {
	o := s.objRefObj(in.Obj)
	if o == NoObj || s.tokenLeftBattlefield(o) {
		return
	}
	s.moveRef(k, in.Obj, kop.ZoneLibrary, false)
	pl := &s.pl[s.objs[o].owner()]
	if s.objs[o].zone != kop.ZoneLibrary || pl.libLen() == 0 || pl.lib[pl.libLo] != o {
		return // a token that ceased to exist, or a replaced move
	}
	off := min(2, pl.libLen()-1)
	if debugNetMutate == 3 {
		off = min(1, off) // test only: second from the top
	}
	for i := 0; i < off; i++ {
		pl.lib[int(pl.libLo)+i] = pl.lib[int(pl.libLo)+i+1]
	}
	pl.lib[int(pl.libLo)+off] = o
}

// debugNetMutate is the lockstep mutation switch (test only): 1 nothing is
// locked, 2 the lock survives an untap, 3 Quipu goes second from the top.
var debugNetMutate uint8

// SetDebugNetMutate sets the mutation switch; tests only.
func SetDebugNetMutate(m uint8) { debugNetMutate = m }

func init() {
	stallNames[StallNetLock] = "netlock"
	stallNames[StallNetMana] = "netmana"
}
