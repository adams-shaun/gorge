package dirac

import (
	"testing"
	"unsafe"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The State bar reads what a game on a pool really holds: the fixed part and that
// pool's cold arrays.
func TestEveryPoolFitsTheStateBar(t *testing.T) {
	check := func(name string, set cardset.Set) int {
		tbl, err := set.Pool()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		defer ReleaseTable(tbl)
		var s State
		s.Init(tbl, nil, nil, 0, Config{})
		n := int(unsafe.Sizeof(s)) + 8*len(s.cold.buf)
		if n > stateBar {
			t.Errorf("%s: %d bytes, the bar is %d", name, n, stateBar)
		}
		return n
	}
	peak, pools := 0, 0
	for _, name := range cardset.Names() {
		peak = max(peak, check(name, cardsettest.Set(t, name)))
		pools++
	}
	for _, name := range cardsettest.SynthNames(t) {
		peak = max(peak, check(name, cardsettest.Synth(t, name)))
		pools++
	}
	t.Logf("largest of %d pools: %d bytes", pools, peak)
}
