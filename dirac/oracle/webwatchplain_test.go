package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Interdimensional Web Watch's mana is an artifact's, so gorge tags it typed
// and spends the plain units of a colour before it (pay.takeUnit). Spider-Ham
// cast from exile for {1}{G}, paid by a Suburban Sanctuary's {W} and Web
// Watch's {G}{W}, leaves the restricted {W} floating; Peter Parker's Camera in
// hand cannot spend it (CR 106.6). The repro is std-spm pair 371.

func TestWebWatchTypedUnitSpentAfterPlain(t *testing.T) {
	stdReproLockstep(t, "testdata/web_watch_plain_unit_first.repro.json", "std-spm")
}

// With the typed restricted unit spent before the plain unit of its colour the
// plain {W} floats instead and pays the Camera: the replay must diverge.
func TestWebWatchTypedUnitSpentAfterPlainMutation(t *testing.T) {
	old := dirac.DebugRestrTypedFirst
	dirac.DebugRestrTypedFirst = true
	defer func() { dirac.DebugRestrTypedFirst = old }()
	res := stdReproPlay(t, "testdata/web_watch_plain_unit_first.repro.json", "std-spm")
	if res.Div == nil {
		t.Fatal("mutation did not diverge")
	}
}
