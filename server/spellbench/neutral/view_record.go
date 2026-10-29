package neutral

// This file holds the view cache's record key: the deep copy stored alongside
// a built card and the equality test that decides whether the next
// observation's record can reuse that card. The deep copy is what makes an
// in-place mutation of an observation (a counter, a power pointer) visible:
// a shallow copy would share the same maps, slices and pointed-to values, so
// both the stored record and the new one would change and the cache would
// reuse a stale card.

import (
	"slices"

	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// cloneObjectRecord returns a deep copy of r, including every slice, map and
// pointer ObjectRecord reaches.
func cloneObjectRecord(r protocol.ObjectRecord) protocol.ObjectRecord {
	out := r
	out.CardName = cloneStringPtr(r.CardName)
	out.FullName = cloneStringPtr(r.FullName)
	out.Characteristics = cloneCharacteristics(r.Characteristics)
	out.Permanent = clonePermanent(r.Permanent)
	out.ExiledBy = cloneObjectRefPtr(r.ExiledBy)
	return out
}

// sameObjectRecord reports whether a and b hold the same record, comparing
// every field with slices by element, maps by key and pointers by presence
// and pointed-to value. The order maps are walked never reaches the result.
func sameObjectRecord(a, b protocol.ObjectRecord) bool {
	return a.ObjectID == b.ObjectID &&
		sameStringPtr(a.CardName, b.CardName) &&
		a.OwnerSeat == b.OwnerSeat &&
		a.ControllerSeat == b.ControllerSeat &&
		a.Zone == b.Zone &&
		sameStringPtr(a.FullName, b.FullName) &&
		a.FaceDown == b.FaceDown &&
		a.Token == b.Token &&
		a.Copy == b.Copy &&
		sameCharacteristics(a.Characteristics, b.Characteristics) &&
		samePermanent(a.Permanent, b.Permanent) &&
		sameObjectRefPtr(a.ExiledBy, b.ExiledBy)
}

func cloneStringPtr(s *string) *string {
	if s == nil {
		return nil
	}
	v := *s
	return &v
}

func cloneInt32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func cloneUint32Ptr(v *uint32) *uint32 {
	if v == nil {
		return nil
	}
	n := *v
	return &n
}

func cloneObjectRefPtr(o *protocol.ObjectRef) *protocol.ObjectRef {
	if o == nil {
		return nil
	}
	v := *o
	v.CardName = cloneStringPtr(o.CardName)
	return &v
}

func cloneTargetRefPtr(t *protocol.TargetRef) *protocol.TargetRef {
	if t == nil {
		return nil
	}
	v := *t
	v.Player = cloneStringPtr(t.Player)
	v.Object = cloneObjectRefPtr(t.Object)
	return &v
}

func cloneCharacteristics(c *protocol.Characteristics) *protocol.Characteristics {
	if c == nil {
		return nil
	}
	v := *c
	v.Supertypes = slices.Clone(c.Supertypes)
	v.Types = slices.Clone(c.Types)
	v.Subtypes = slices.Clone(c.Subtypes)
	v.Colors = slices.Clone(c.Colors)
	v.Power = cloneInt32Ptr(c.Power)
	v.Toughness = cloneInt32Ptr(c.Toughness)
	v.Keywords = slices.Clone(c.Keywords)
	return &v
}

func clonePermanent(p *protocol.Permanent) *protocol.Permanent {
	if p == nil {
		return nil
	}
	v := *p
	v.Counters = cloneCounters(p.Counters)
	v.AttachedTo = cloneTargetRefPtr(p.AttachedTo)
	v.AttackTarget = cloneTargetRefPtr(p.AttackTarget)
	v.BlockedAttackers = slices.Clone(p.BlockedAttackers)
	for i := range v.BlockedAttackers {
		v.BlockedAttackers[i].CardName = cloneStringPtr(v.BlockedAttackers[i].CardName)
	}
	v.Statuses = slices.Clone(p.Statuses)
	v.ClassLevel = cloneUint32Ptr(p.ClassLevel)
	if p.Chosen != nil {
		v.Chosen = make([]map[string]string, len(p.Chosen))
		for i, m := range p.Chosen {
			if m == nil {
				continue
			}
			cm := make(map[string]string, len(m))
			for k, s := range m {
				cm[k] = s
			}
			v.Chosen[i] = cm
		}
	}
	return &v
}

func cloneCounters(m map[string]uint32) map[string]uint32 {
	if m == nil {
		return nil
	}
	out := make(map[string]uint32, len(m))
	for k, n := range m {
		out[k] = n
	}
	return out
}

func sameStringPtr(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameInt32Ptr(a, b *int32) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameUint32Ptr(a, b *uint32) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}

func sameObjectRefPtr(a, b *protocol.ObjectRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.ObjectID == b.ObjectID &&
		sameStringPtr(a.CardName, b.CardName) &&
		a.OwnerSeat == b.OwnerSeat &&
		a.ControllerSeat == b.ControllerSeat &&
		a.Zone == b.Zone
}

func sameTargetRefPtr(a, b *protocol.TargetRef) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return sameStringPtr(a.Player, b.Player) && sameObjectRefPtr(a.Object, b.Object)
}

func sameCharacteristics(a, b *protocol.Characteristics) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return slices.Equal(a.Supertypes, b.Supertypes) &&
		slices.Equal(a.Types, b.Types) &&
		slices.Equal(a.Subtypes, b.Subtypes) &&
		slices.Equal(a.Colors, b.Colors) &&
		a.ManaValue == b.ManaValue &&
		sameInt32Ptr(a.Power, b.Power) &&
		sameInt32Ptr(a.Toughness, b.Toughness) &&
		slices.Equal(a.Keywords, b.Keywords)
}

func samePermanent(a, b *protocol.Permanent) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.Tapped == b.Tapped &&
		a.SummoningSick == b.SummoningSick &&
		a.Damage == b.Damage &&
		sameCounters(a.Counters, b.Counters) &&
		sameTargetRefPtr(a.AttachedTo, b.AttachedTo) &&
		a.Attacking == b.Attacking &&
		sameTargetRefPtr(a.AttackTarget, b.AttackTarget) &&
		a.Blocking == b.Blocking &&
		sameObjectRefs(a.BlockedAttackers, b.BlockedAttackers) &&
		a.PhasedOut == b.PhasedOut &&
		slices.Equal(a.Statuses, b.Statuses) &&
		sameUint32Ptr(a.ClassLevel, b.ClassLevel) &&
		sameChosen(a.Chosen, b.Chosen)
}

func sameCounters(a, b map[string]uint32) bool {
	if len(a) != len(b) {
		return false
	}
	for k, n := range a {
		bn, ok := b[k]
		if !ok || bn != n {
			return false
		}
	}
	return true
}

func sameObjectRefs(a, b []protocol.ObjectRef) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !sameObjectRefPtr(&a[i], &b[i]) {
			return false
		}
	}
	return true
}

func sameChosen(a, b []map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if len(a[i]) != len(b[i]) {
			return false
		}
		for k, v := range a[i] {
			bv, ok := b[i][k]
			if !ok || bv != v {
				return false
			}
		}
	}
	return true
}
