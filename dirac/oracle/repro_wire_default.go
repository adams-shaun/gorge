package oracle

import (
	"fmt"
	"math"
	"slices"

	"github.com/adams-shaun/gorge/dirac"
)

// The two-seat build's ref bytes (before 2026-11-18), which the wire form
// keeps: instances below defStack, then sixteen stack abilities, the two
// seats, 0xFE (a departed stack target) and none.
const (
	defStack  = 0xEC
	defPlayer = 0xFC
	defGone   = 0xFE
	defNone   = 0xFF
	mpPacking = "mp"
)

// refGone is the departed stack target's Ref (dirac.StackTargetGone).
const refGone = Ref(dirac.StackTargetGone)

// refFromDefault maps a wire ref byte to the kernel's Ref.
func refFromDefault(b uint8) Ref {
	switch {
	case b == defNone:
		return RefNone
	case b == defGone:
		return refGone
	case b >= defPlayer && b < defPlayer+2:
		return RefPlayer + Ref(b-defPlayer)
	case b >= defStack && b < defPlayer:
		return RefStack + Ref(b-defStack)
	}
	return Ref(b)
}

// refToDefault is refFromDefault's inverse, false for a ref no byte holds.
func refToDefault(r Ref) (uint8, bool) {
	switch {
	case r == RefNone:
		return defNone, true
	case r == refGone:
		return defGone, true
	case r >= RefPlayer && r < RefPlayer+2:
		return defPlayer + uint8(r-RefPlayer), true
	case r >= RefStack && r < RefStack+(defPlayer-defStack):
		return defStack + uint8(r-RefStack), true
	case r < defStack:
		return uint8(r), true
	}
	return 0, false
}

// valueKinds are the choose kinds whose target field holds a value, not a
// Ref: a label's vocabulary index or hash (type, creature_type, land_type:
// labelValue's high byte) and a card name's hash (name, whose source is the
// ordinal of two names that hash alike, or RefNone). The wire keeps such a
// byte as it is.
var valueKinds = func() map[CanonKind]bool {
	m := map[CanonKind]bool{}
	for _, k := range []string{"type", "creature_type", "land_type", "name"} {
		if c, ok := chooseCode(k); ok {
			m[ActChoose+CanonKind(c)] = true
		}
	}
	return m
}()

// nameKind is the choose:name action kind, whose source is an ordinal.
var nameKind = func() CanonKind {
	c, _ := chooseCode("name")
	return ActChoose + CanonKind(c)
}()

// fromDefault maps one wire field byte of an action of kind k to the kernel's
// Ref: a value field keeps its byte, a name ordinal keeps it unless it is
// none, and a Ref maps by refFromDefault.
func fromDefault(k CanonKind, b uint8, tgt bool) Ref {
	switch {
	case tgt && valueKinds[k]:
		return Ref(b)
	case !tgt && k == nameKind && b != defNone:
		return Ref(b)
	}
	return refFromDefault(b)
}

// toDefault is fromDefault's inverse, false for a field no byte holds.
func toDefault(k CanonKind, r Ref, tgt bool) (uint8, bool) {
	switch {
	case tgt && valueKinds[k], !tgt && k == nameKind && r != RefNone:
		return uint8(r), r <= math.MaxUint8
	}
	return refToDefault(r)
}

// format1Byte maps a format-1 recording's ref byte to the two-seat build's
// (format1Ref at one byte a field, as the two-seat build read it: every
// field, value fields included, so a format-1 game replays as it did there).
func format1Byte(b uint8) uint8 {
	switch {
	case b >= 0xE0 && b < 0xF0:
		return defStack + (b - 0xE0)
	case b == 0xF0 || b == 0xF1:
		return defPlayer + (b - 0xF0)
	}
	return b
}

// wireActions is as in the wire form (repro_wire.go): the default packing
// when every ref has a byte there, else this class's own packing, marked.
func wireActions(as []Canon) ([]uint64, string) {
	if as == nil {
		return nil, ""
	}
	words := make([]uint64, len(as))
	portable := true
	for i, a := range as {
		src, ok1 := toDefault(a.Kind(), a.Src(), false)
		tgt, ok2 := toDefault(a.Kind(), a.Tgt(), true)
		if !ok1 || !ok2 || a.Kind() > math.MaxUint8 {
			portable = false
			break
		}
		words[i] = uint64(a.Kind())<<24 | uint64(src)<<16 | uint64(tgt)<<8 | uint64(a.Mode())
	}
	if portable {
		return words, ""
	}
	for i, a := range as {
		words[i] = uint64(a)
	}
	return words, mpPacking
}

// actionsFromWire reads the wire form back into the kernel's Canons; a
// format-1 recording's stack and seat bytes (0xE0.., 0xF0..) first.
func actionsFromWire(words []uint64, packing string, format1 bool) ([]Canon, error) {
	if packing != "" && packing != mpPacking {
		return nil, fmt.Errorf("packing %q is unknown", packing)
	}
	if words == nil {
		return nil, nil
	}
	if packing == "" && slices.ContainsFunc(words, func(w uint64) bool { return w > math.MaxUint32 }) {
		// A word past 32 bits is no two-seat Canon: the recording is in the
		// four-seat packing, written before it was marked (the 2026-11-13
		// MBC pod residuals).
		packing = mpPacking
	}
	out := make([]Canon, len(words))
	for i, w := range words {
		if packing == mpPacking {
			out[i] = Canon(w)
			continue
		}
		if w > math.MaxUint32 {
			return nil, fmt.Errorf("action %d: %d does not fit the default packing's 32 bits", i, w)
		}
		k, src, tgt := CanonKind(w>>24), uint8(w>>16), uint8(w>>8)
		if format1 {
			src, tgt = format1Byte(src), format1Byte(tgt)
		}
		out[i] = Pack(k, fromDefault(k, src, false), fromDefault(k, tgt, true), uint8(w))
	}
	return out, nil
}
