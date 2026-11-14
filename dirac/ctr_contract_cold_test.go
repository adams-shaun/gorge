package dirac

import (
	"testing"
	"unsafe"
)

// TestColdPrefixViewsDoNotAlias: with the plus-creature, extra-block,
// partner, attack and contract records all kept, every accessor's view lies
// inside the buffer and none overlaps another, so a contract put cannot
// change a plus-creature record or the other way round.
func TestColdPrefixViewsDoNotAlias(t *testing.T) {
	l := &coldLayout{ctrP: 1, xblk: MaxPerm, partner: 1, atkLast: MaxSeats, contract: MaxSeats}
	l.size()
	var s State
	s.layOut(l)
	type span struct {
		name   string
		lo, hi uintptr
	}
	var spans []span
	add := func(name string, p unsafe.Pointer, n int) {
		if p == nil || n == 0 {
			t.Fatalf("%s: missing view", name)
		}
		spans = append(spans, span{name, uintptr(p), uintptr(p) + uintptr(n)})
	}
	pc := s.plusCreCold()
	add("plusCre", unsafe.Pointer(pc), int(unsafe.Sizeof(*pc)))
	xb := s.xblkRecs()
	add("xblk", unsafe.Pointer(&xb[0]), len(xb)*int(unsafe.Sizeof(xb[0])))
	pt := s.partnerCold()
	add("partner", unsafe.Pointer(pt), int(unsafe.Sizeof(*pt)))
	al := s.atkLastCold()
	add("atkLast", unsafe.Pointer(&al[0]), len(al))
	ct := s.contractCold()
	add("contract", unsafe.Pointer(&ct[0]), len(ct))
	base := uintptr(unsafe.Pointer(&s.cold.buf[0]))
	end := base + uintptr(len(s.cold.buf))*8
	for i, a := range spans {
		if a.lo < base || a.hi > end {
			t.Errorf("%s lies outside the buffer", a.name)
		}
		for _, b := range spans[i+1:] {
			if a.lo < b.hi && b.lo < a.hi {
				t.Errorf("%s and %s overlap", a.name, b.name)
			}
		}
	}
}
