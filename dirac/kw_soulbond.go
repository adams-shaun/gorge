package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Soulbond (CR 702.95; gorge cards/kw_soulbond.go, engine/effects/soulbond.go
// effPair, events/apply_state_helpers.go applyPair / endPairing). Cold and
// out of line: a table with no OpExt ExtPair keeps no array (usePair) and
// nothing is ever paired.
//
// State: coldPair, one record per battlefield slot (perSlot: a slot that
// takes a new permanent starts unpaired): mate the partner object and ver its
// incarnation when the pair formed. A pairing is live while both records
// name each other and the mate is still that incarnation on the battlefield,
// so a departure (CR 400.7; gorge's MoveZone clears both halves) ends it with
// no write. A control change (CR 702.95e; gorge foldControlChange) and the
// state-based check of a member that stopped being a creature (gorge
// unpairNoncreatures) clear both records, so a later return of control or of
// the creature type does not revive it.
//
// Gorge's expansion supplies the two triggers (lowered as ordinary ETB
// triggers whose intervening if reads FwPaired); ExtPair is the common body:
// the unpaired battlefield creature may pair with another unpaired creature
// the trigger's controller controls (C 1: only the triggering entrant). The
// pick is optional (gorge's Min 0 Max 1 KChoose): one AkChoose per candidate
// in entry order, AkDone the decline.

// ChGenPair is the Soulbond partner pick (gorge effPair's "soulbond"
// KChoose, option kind "pair"): source = the chosen creature.
const ChGenPair ChoiceKind = 216

type pairRec struct {
	mate Obj
	ver  uint8
	on   bool
}

var coldPair = newCold[pairRec]("pair", func(g *genPrep) int {
	if g.pairOn() {
		return MaxPerm
	}
	return 0
}).perSlot()

// usePair is genPrep.uses' bit for a table where some program pairs.
var usePair = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.Ext() == kop.ExtPair {
					return true
				}
			}
		}
	}
	return false
})

// pairOn reports whether the table pairs (kw_soulbond.go's state is laid out).
func (g *genPrep) pairOn() bool { return g != nil && g.uses.has(usePair) }

// pairOf is the object the battlefield permanent at sl is paired with, NoObj
// when it is unpaired or the table never pairs.
func (s *State) pairOf(sl Slot) Obj {
	c := coldPair.of(s)
	if c == nil || sl == NoSlot {
		return NoObj
	}
	r := c[sl]
	if !r.on {
		return NoObj
	}
	m := &s.objs[r.mate]
	if m.zone != kop.ZoneBattlefield || m.ver != r.ver {
		return NoObj
	}
	b := c[m.slot]
	if !b.on || b.mate != s.perms[sl].obj || s.objs[b.mate].ver != b.ver {
		return NoObj
	}
	return r.mate
}

// Paired is the object the battlefield object o is paired with, NoObj when
// none (oracle).
func (s *State) Paired(o Obj) Obj {
	if int(o) >= int(s.nobj) || s.objs[o].zone != kop.ZoneBattlefield {
		return NoObj
	}
	return s.pairOf(s.objs[o].slot)
}

// pairClear ends sl's pairing on both halves (gorge endPairing).
func (s *State) pairClear(sl Slot) {
	c := coldPair.of(s)
	if c == nil {
		return
	}
	if m := s.pairOf(sl); m != NoObj {
		c[s.objs[m].slot] = pairRec{}
	}
	c[sl] = pairRec{}
}

// pairControlChanged ends the pairing of the permanent at sl when another
// player gains control of it (CR 702.95e; ctrlMove calls it only on a real
// change).
func (s *State) pairControlChanged(sl Slot) { s.pairClear(sl) }

// pairSBA ends the pairing of each paired permanent that is no longer a
// creature (CR 702.95e; gorge unpairNoncreatures). A phased-out permanent is
// not on the kernel's battlefield mask and keeps its pairing, as in gorge.
func (s *State) pairSBA(k *prep) {
	if coldPair.of(s) == nil {
		return
	}
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.pairOf(sl) != NoObj && !s.isCreature(k, sl) {
			s.pairClear(sl)
		}
	}
}

// pairSource is the resolving soulbond creature's slot, NoSlot when it is no
// longer an unpaired battlefield creature (gorge effPair's opening return).
func (s *State) pairSource(k *prep) Slot {
	sl := s.objRef(k, kop.RefSelf)
	if sl == NoSlot || !s.isCreature(k, sl) || s.pairOf(sl) != NoObj {
		return NoSlot
	}
	return sl
}

// pairCandidates writes the legal partners of src for the resolving
// controller, in entry order (gorge effPair's Zone(ZBattlefield, controller)
// walk and soulbondPartner): another unpaired creature that player controls;
// with restrict only the triggering entrant.
func (s *State) pairCandidates(k *prep, src Slot, restrict bool, buf *[MaxPerm]Slot) int {
	p := s.res.e.ctrl
	want := NoSlot
	if restrict {
		if want = s.objRef(k, kop.RefTriggerObj); want == NoSlot {
			return 0 // no remembered entrant: nothing to offer (gorge's restrictTo == 0 return)
		}
	}
	n := s.battlefieldOrder(p, s.ctrlM[p].and(s.occ), buf[:], 0)
	m := 0
	for i := 0; i < n; i++ {
		sl := buf[i]
		if sl == src || want != NoSlot && sl != want || !s.isCreature(k, sl) || s.pairOf(sl) != NoObj {
			continue
		}
		buf[m] = sl
		m++
	}
	return m
}

// pairExt is ExtPair (gorge effPair): with a legal partner the controller is
// asked; with none the program goes on.
//
//kmanifest:ext ExtPair
func (s *State) pairExt(k *prep, in *kop.Instr) (ok, done bool) {
	src := s.pairSource(k)
	if src == NoSlot {
		return true, true
	}
	var buf [MaxPerm]Slot
	if s.pairCandidates(k, src, in.C == 1, &buf) == 0 {
		return true, true
	}
	s.res.ask, s.res.askP = raPair, s.res.e.ctrl
	return false, true
}

// pairRestrict reports whether the resolving ExtPair narrows to the entrant.
func (s *State) pairRestrict(k *prep) bool {
	return s.prog(k)[s.res.pc].C == 1
}

// pairAskOptions poses the partner pick: each candidate, then the decline.
//
//kmanifest:askopts raPair
func (s *State) pairAskOptions(k *prep, d *Decision) {
	d.Sub = ChGenPair
	src := s.pairSource(k)
	if src != NoSlot {
		var buf [MaxPerm]Slot
		n := s.pairCandidates(k, src, s.pairRestrict(k), &buf)
		for i := 0; i < n; i++ {
			d.add(MakeAction(AkChoose, TCode(s.perms[buf[i]].obj), 0, 0))
		}
	}
	d.add(MakeAction(AkDone, 0, 0, 0))
}

// pairAskAnswer pairs the source with the pick when it is still a legal
// partner (gorge soulbondPair: a stale answer pairs nothing); AkDone declines.
//
//kmanifest:askapply raPair
func (s *State) pairAskAnswer(k *prep, a Action) {
	s.res.ask = raNone
	if a.Kind() == AkChoose {
		if src := s.pairSource(k); src != NoSlot {
			var buf [MaxPerm]Slot
			n := s.pairCandidates(k, src, s.pairRestrict(k), &buf)
			pick := Obj(a.Source())
			for i := 0; i < n; i++ {
				if s.perms[buf[i]].obj == pick {
					s.pairSet(src, buf[i])
					break
				}
			}
		}
	}
	s.continueRes(k, 0)
}

// pairSet writes the reciprocal pairing of the battlefield permanents at a
// and b (gorge applyPair).
func (s *State) pairSet(a, b Slot) {
	c := coldPair.of(s)
	oa, ob := s.perms[a].obj, s.perms[b].obj
	c[a] = pairRec{mate: ob, ver: s.objs[ob].ver, on: true}
	c[b] = pairRec{mate: oa, ver: s.objs[oa].ver, on: true}
	s.touchSl(a)
	s.touchSl(b)
}

// fwPaired: FwPaired, gorge predicates["Paired"]: the battlefield permanent
// is paired.
//
//kmanifest:fword FwPaired
func (s *State) fwPaired(c *fctx, l *kop.FLit) bool {
	return c.perm() && s.pairOf(c.sl) != NoObj
}

// fwPairedWith: FwPairedWith, gorge predicates["PairedWith"]: the battlefield
// permanent is the reading source's partner.
//
//kmanifest:fword FwPairedWith
func (s *State) fwPairedWith(c *fctx, l *kop.FLit) bool {
	if !c.perm() || c.src == NoSlot || !s.occ.has(c.src) {
		return false
	}
	return s.pairOf(c.src) == s.perms[c.sl].obj
}
