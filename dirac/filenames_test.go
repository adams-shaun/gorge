package dirac

import (
	"path/filepath"
	"regexp"
	"testing"
)

// coreFiles is the hand-written core and the infrastructure that has no one
// mechanic. Everything else is named for its mechanic; doc.go lists the topics.
var coreFiles = map[string]bool{
	"arena.go": true, "arenaobj.go": true, "cardflags.go": true, "cast.go": true, "chars.go": true,
	"chars_gen.go": true, "combat.go": true, "copy_gen.go": true, "dispatch.go": true, "doc.go": true,
	"driver.go": true, "engine.go": true, "exprs.go": true, "extop.go": true, "featview.go": true,
	"filters_gen.go": true, "gen.go": true, "hbotapi_combat.go": true, "hiddendeal.go": true,
	"interp.go": true, "interp_ask.go": true, "interp_gen.go": true, "legal.go": true,
	"legal_gen.go": true, "lockstepconfig.go": true, "mana.go": true, "marks.go": true, "occupancy.go": true,
	"pregame.go": true, "prep.go": true, "project.go": true, "projchars.go": true, "repl_gen.go": true,
	"resolve.go": true, "returnrem.go": true, "rng.go": true, "scratch.go": true, "script_gen.go": true,
	"searchapi.go": true, "seats.go": true, "seats_default.go": true, "seats_mp.go": true, "slotmask.go": true, "stallclass.go": true, "stallnames.go": true, "state.go": true,
	"statecopy.go": true, "statics_gen.go": true, "targets_gen.go": true, "triggers_gen.go": true,
	"trigpl.go": true, "turn.go": true, "turn_gen.go": true, "types.go": true, "zones.go": true,
}

var (
	topicFile    = regexp.MustCompile(`^(kw|act|mana|cost|cast|copy|face|modal|ctr|trig|repl|static|tgt|filter|value|zone|combat)_[a-z][a-z0-9]*(_test)?\.go$`)
	testFile     = regexp.MustCompile(`_test\.go$`)
	numberedStem = regexp.MustCompile(`[0-9](_test)?\.go$`)
	layerFile    = regexp.MustCompile(`layer[0-9](_test)?\.go$`)
	sortPrefixed = regexp.MustCompile(`^z`)
)

func TestDiracFileNames(t *testing.T) {
	names, err := filepath.Glob("*.go")
	if err != nil || len(names) == 0 {
		t.Fatalf("no Go files: %v", err)
	}
	for _, n := range names {
		named := coreFiles[n] || topicFile.MatchString(n)
		if !named && !testFile.MatchString(n) {
			t.Errorf("%s is neither core nor <topic>_<mechanic>.go", n)
		}
		if !named && sortPrefixed.MatchString(n) {
			t.Errorf("%s is named for where it sorts", n)
		}
		if numberedStem.MatchString(n) && !layerFile.MatchString(n) {
			t.Errorf("%s ends in a number: rounds of one mechanic share a file", n)
		}
	}
}
