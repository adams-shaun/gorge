package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// scratch probe: what diverges with DebugNoTapMana on/off?
func TestZZZProbeTapX(t *testing.T) {
	if testing.Short() {
		t.Skip("probe")
	}
	r, err := ReadRepro(filepath.Join("testdata", "repro", "zzz_probe_tapx.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	run := func(name string, dbg bool) {
		t.Run(name, func(t *testing.T) {
			old := dirac.DebugNoTapMana
			dirac.DebugNoTapMana = dbg
			defer func() { dirac.DebugNoTapMana = old }()
			pol := &ScriptPolicy{Actions: r.Actions}
			res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, pol)
			t.Logf("skipped=%d next=%d len=%d firstSkipAt=%d", pol.Skipped, pol.next, len(r.Actions), pol.firstSkipAt)
			if res.Div != nil {
				t.Errorf("div: at=%d what=%q field=%q diff=%v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			} else {
				t.Logf("no divergence")
			}
		})
	}
	run("off", false)
	run("on", true)
}
