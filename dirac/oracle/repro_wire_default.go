package oracle

import (
	"fmt"
	"math"
)

// wireActions is as in the wire form (repro_wire.go): the kernel's own
// packing, which needs no marker.
func wireActions(as []Canon) ([]uint64, string) {
	if as == nil {
		return nil, ""
	}
	words := make([]uint64, len(as))
	for i, a := range as {
		words[i] = uint64(a)
	}
	return words, ""
}

// actionsFromWire reads the wire form back; a format-1 recording's refs are
// mapped to the current ones.
func actionsFromWire(words []uint64, packing string, format1 bool) ([]Canon, error) {
	if packing != "" {
		return nil, fmt.Errorf("packing %q is not this build's: replay it at a commit whose kernel packs it", packing)
	}
	if words == nil {
		return nil, nil
	}
	out := make([]Canon, len(words))
	for i, w := range words {
		if w > math.MaxUint32 {
			return nil, fmt.Errorf("action %d: %d does not fit the kernel's 32 bits", i, w)
		}
		out[i] = Canon(w)
	}
	if format1 {
		out = format1Actions(out)
	}
	return out, nil
}
