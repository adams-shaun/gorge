//go:build dirac_mp

package oracle

import (
	"fmt"
	"math"
)

// The default class's ref bytes (dirac seats_default.go), which the wire form
// keeps in every build: instances below defStack, then sixteen stack
// abilities, the two seats, 0xFE (a departed stack target) and none.
const (
	defStack  = 0xEC
	defPlayer = 0xFC
	defNone   = 0xFF
	mpPacking = "mp"
)

// refFromDefault maps a default-class ref byte to this class's Ref.
func refFromDefault(b uint8) Ref {
	switch {
	case b == defNone:
		return RefNone
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
	case r >= RefPlayer && r < RefPlayer+2:
		return defPlayer + uint8(r-RefPlayer), true
	case r >= RefStack && r < RefStack+(defPlayer-defStack):
		return defStack + uint8(r-RefStack), true
	case r < defStack:
		return uint8(r), true
	}
	return 0, false
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
		src, ok1 := refToDefault(a.Src())
		tgt, ok2 := refToDefault(a.Tgt())
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

// actionsFromWire reads the wire form back into this class's Canons; a
// format-1 recording's stack and seat bytes (0xE0.., 0xF0..) first.
func actionsFromWire(words []uint64, packing string, format1 bool) ([]Canon, error) {
	if packing != "" && packing != mpPacking {
		return nil, fmt.Errorf("packing %q is unknown", packing)
	}
	if words == nil {
		return nil, nil
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
		src, tgt := uint8(w>>16), uint8(w>>8)
		if format1 {
			out[i] = Pack(CanonKind(w>>24), format1Ref(Ref(src)), format1Ref(Ref(tgt)), uint8(w))
			continue
		}
		out[i] = Pack(CanonKind(w>>24), refFromDefault(src), refFromDefault(tgt), uint8(w))
	}
	return out, nil
}
