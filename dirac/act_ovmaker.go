package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// The maker of a granted ability. A granted activation is offered as
// activate(holder, maker) (gorge decision.Option.GrantSource, CR 114.4): the
// object of the card that made the effect, wherever it now is -- Banishing
// Knack in the graveyard for "{T}: return target nonland permanent" granted to
// a creature, Evolution Vat on the battlefield. An override record that grants
// an ability (Animate Abilities$) is made by a card whose object no ovRec
// field holds (ovRec has no spare bytes, and a wider one moves every pinned
// State layout), so the object is kept in a cold ledger keyed by the record's
// layer timestamp, present only for a table with an override that grants
// (TestNoNewFixedStateFields). A record's entry dies with the record: noteOvMk
// drops the entries no live override record carries before it adds one.

// ovMkN is the entries the ledger holds: at most the override records that
// grant at once.
const ovMkN = 32

// ovMkRec is one entry: the record stamped ts was made by object mk1-1 (0 = empty).
type ovMkRec struct {
	ts  uint16
	mk1 Obj
}

var coldOvMk = newCold[ovMkRec]("ovMk", nil)

// prepOvMk sizes the ledger for a table with an override that grants an
// ability.
func prepOvMk(t *kop.Table, p *prep) {
	for i := range t.Overrides {
		if t.Overrides[i].Grant[0] != 0 {
			coldOvMk.set(&p.cold, ovMkN)
			return
		}
	}
}

// ovMkOn reports whether s's table keeps the ledger.
func (s *State) ovMkOn() bool { return coldOvMk.count(s.cold.layout) > 0 }

// ovMkOf is the object that made the override record stamped ts, NoObj when the
// ledger holds none (a record whose maker is not kept: a copy grant).
func (s *State) ovMkOf(ts uint16) Obj {
	for _, e := range coldOvMk.of(s) {
		if e.mk1 != 0 && e.ts == ts {
			return e.mk1 - 1
		}
	}
	return NoObj
}

// noteOvMk records that the new override record stamped ts was made by mk,
// first dropping the entries no live override record carries. False when the
// ledger is full.
func (s *State) noteOvMk(ts uint16, mk Obj) bool {
	r := coldOvMk.of(s)
	free := -1
	for i := range r {
		if r[i].mk1 != 0 && !s.ovTsLive(r[i].ts) {
			r[i] = ovMkRec{}
		}
		if r[i].mk1 == 0 && free < 0 {
			free = i
		}
	}
	if free < 0 {
		return false
	}
	r[free] = ovMkRec{ts: ts, mk1: mk + 1}
	return true
}

// ovTsLive reports whether a live override record carries timestamp ts.
func (s *State) ovTsLive(ts uint16) bool {
	for i := 0; i < int(s.g.nfx); i++ {
		if r := &s.g.fx[i]; r.kind == fxOverride && r.r.ts == ts {
			return true
		}
	}
	return false
}

// ovMkMatch is the merge test of addOvRec for a record that grants: the
// existing record stamped ts was made by the same object as the new one.
func (s *State) ovMkMatch(ts uint16, mk Obj) bool { return s.ovMkOf(ts) == mk }

// grantMakerObj is the object that made the live override record on sl
// granting ability ab of card c (the activation's host object), NoObj when no
// such record keeps one. The record is the one ownGrantAlive found live.
func (s *State) grantMakerObj(k *prep, sl Slot, c CardID, ab uint8, lost bool, lts uint16) Obj {
	if !s.ovMkOn() {
		return NoObj
	}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.sl != sl || r.kind != fxOverride || r.r.idx == 0 || r.r.src != c || r.r.copyGrant() || lost && r.r.ts < lts {
			continue
		}
		ov := &k.t.Overrides[r.r.idx-1]
		if ov.LoseAbilities && !ov.SetLandSub {
			continue
		}
		for _, g := range ov.Grant {
			if g == ab+1 {
				if mk := s.ovMkOf(r.r.ts); mk != NoObj {
					return mk
				}
			}
		}
	}
	return NoObj
}

// grantMakerCard is the card whose ability ab a granted activation by o
// hosted at host reads when the host is the maker of an override record on o
// (not o itself, and not a permanent granting by attachment): the record's
// card. False for every other grant, whose body is the host's own.
func (s *State) grantMakerCard(k *prep, o, host Obj, ab uint8) (CardID, bool) {
	if host == o || host == NoObj || !s.ovMkOn() || int(o) >= int(s.nobj) {
		return 0, false
	}
	ob := &s.objs[o]
	if ob.zone != kop.ZoneBattlefield || ob.slot == NoSlot || !s.fxM.has(ob.slot) {
		return 0, false
	}
	for i := 0; i < int(s.g.nfx); i++ {
		r := &s.g.fx[i]
		if r.sl != ob.slot || r.kind != fxOverride || r.r.idx == 0 || r.r.copyGrant() {
			continue
		}
		for _, g := range k.t.Overrides[r.r.idx-1].Grant {
			if g == ab+1 && s.ovMkOf(r.r.ts) == host {
				return r.r.src, true
			}
		}
	}
	return 0, false
}
