package oracle

import (
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Ad Nauseam's RepeatOptional$ (gorge effRepeat's repeat_optional, "Repeat
// this process?"; dirac/act_repeatoptional.go). The recorded game casts it
// in turn 18's upkeep and answers the election yes until the library is
// empty: the pass that takes the last card lost 7 life, so the pass over the
// empty library writes a different AFLifeLost and gorge asks once more
// (decision 434) before the next empty pass ends the loop unasked. The
// mutation drops the election, so the body runs once, and the game parts at
// the first ask (decision 412).
func TestMBCAdNauseamRepeatOptionalLockstep(t *testing.T) {
	stdReproNamed(t, "ad_nauseam_repeat_optional")
	t.Run("mutation", func(t *testing.T) {
		r, err := ReadRepro(filepath.Join("testdata", "repro", "ad_nauseam_repeat_optional.repro.json"))
		if err != nil {
			t.Fatal(err)
		}
		dirac.DebugRepeatOptionalOnce = true
		defer func() { dirac.DebugRepeatOptionalOnce = false }()
		res := replayDecodedRes(t, synthReg(t), globSet(t, r.Pool, r), r)
		if res.Div == nil {
			t.Fatal("the mutation that drops the repeat election did not diverge: the game no longer reaches Ad Nauseam's ask")
		}
	})
}
