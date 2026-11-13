package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// two departure reads of the per-turn zone-add ring
// (value_enteredledger.go).
//
//   - ExRevolt (gorge revoltThisTurn): some MoveZone of this turn left the
//     battlefield and the moved object's Controller, read NOW, is the reader.
//     gorge's comment calls that the controller at the moment it left, but
//     foldMoveZone resets Controller to Owner on every move to a zone other
//     than the battlefield or stack (engine/events/apply_zone.go:712 and
//     :819), so an object that is no longer on the battlefield answers with
//     its OWNER, and one that came back answers with its controller now.
//     The kernel copies gorge (a Gorge finding in the report): a stolen
//     permanent that died counts for its owner, not the thief.
//   - ExLeftZone (gorge evalCov3HeadLeftZoneThisTurn): the entries whose
//     origin is the named zone, counted once per object, the spec matched
//     against the object where it now lives.
//
// Both rings record only for a table with one of the kinds (prepareGen).

// revoltHolds reports whether player p controlled a permanent as it left the
// battlefield this turn. A token that ceased still counts (gorge never deletes
// an object; the ring keeps it from recycling).
//
//go:noinline
func (s *State) revoltHolds(p uint8) bool {
	p = SeatIx(p)
	for i := uint8(0); i < s.g.enteredN; i++ {
		e := &s.cold.entered[i]
		if e.from != uint8(kop.ZoneBattlefield) {
			continue
		}
		c := e.leftController()
		if DebugRevoltOwner {
			c = s.objs[e.o].owner()
		}
		if DebugRevoltAnyone || SeatIx(c) == p {
			return true
		}
	}
	return false
}

// leftZone evaluates Expr ExLeftZone: N is the origin zone, F the spec.
//
//go:noinline
func (s *State) leftZone(k *prep, x *kop.Expr, ctx evalCtx) int {
	var seen [maxEntered]Obj
	ns := 0
	for i := uint8(0); i < s.g.enteredN; i++ {
		e := &s.cold.entered[i]
		if e.from != uint8(x.N) {
			continue
		}
		dup := false
		if !DebugLeftZoneNoDedupe {
			for j := 0; j < ns; j++ {
				if seen[j] == e.o {
					dup = true
					break
				}
			}
		}
		if dup {
			continue
		}
		ob := &s.objs[e.o]
		if e.from == uint8(kop.ZoneBattlefield) {
			if !s.matchDeparture(k, x.F, e, ctx) {
				continue
			}
		} else if !s.matchCardEntered(k, x.F, ob.card, ob.owner(), ctx.ctrl, e.o, ctx.src) {
			continue
		}
		seen[ns] = e.o
		ns++
	}
	// A spell priced in its graveyard has not moved yet but moves before the
	// cost is determined (CR 601.2a, 601.2f), so a graveyard count includes it
	// (gorge Host.PricedFromGraveyard); once cast it is in the ring already.
	if x.N == int16(kop.ZoneGraveyard) && ctx.uncounted != 0 {
		if o := ctx.uncounted - 1; s.objs[o].zone == kop.ZoneGraveyard {
			dup := false
			for j := 0; j < ns; j++ {
				dup = dup || seen[j] == o
			}
			if !dup && s.matchCardEntered(k, x.F, s.objs[o].card, s.objs[o].owner(), ctx.ctrl, o, ctx.src) {
				ns++
			}
		}
	}
	return ns
}

// DebugRevoltOwner is the mutation test's switch (TestLeftzoneMutation):
// revoltHolds reads the entry object's owner instead of who it left under.
var DebugRevoltOwner bool

// DebugLeftZoneNoDedupe makes leftZone count every entry, not every object.
var DebugLeftZoneNoDedupe bool

// DebugRevoltAnyone makes revoltHolds ignore whose permanent left (Void's
// reading).
var DebugRevoltAnyone bool
