package effects

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// atEOTCarrierAPIs are the primitives whose resolution registers an AtEOT$
// rider. Each one registers through scheduleAtEOT (or, for CopyPermanent, its
// own group registration), so one resolution that affects several objects with
// a plain sacrifice is one delayed triggered ability naming all of them (CR
// 603.7), as Forge's registerDelayedTrigger creates it. Effect is the Effect
// card's own end-step rider, not a per-object one.
var atEOTCarrierAPIs = map[string]bool{
	"Animate": true, "ChangeZone": true, "ChangeZoneAll": true, "CopyPermanent": true,
	"Effect": true, "Pump": true, "PumpAll": true, "Token": true,
}

// atEOTUnmodelledValues are carried by a script but have no rider: scheduleAtEOT
// says so in one Note per call and the affected object stays (the shuffle-into-
// library arm).
var atEOTUnmodelledValues = map[string]bool{"Library": true}

// TestAtEOTCorpusCensus walks every Forge card script and asserts every AtEOT$
// carrier is a primitive that groups its objects (atEOTCarrierAPIs) with a
// value that has a scheduled rider. A carrier of a new primitive fails until
// it is routed through the group registration and listed.
func TestAtEOTCorpusCensus(t *testing.T) {
	dir := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	apiRe := regexp.MustCompile(`(?:SP|AB|DB)\$ *([A-Za-z]+)`)
	valRe := regexp.MustCompile(`AtEOT\$ *([A-Za-z]+)`)
	seen := map[string]int{}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(b), "\n") {
			v := valRe.FindStringSubmatch(line)
			if v == nil {
				continue
			}
			api := "?"
			if m := apiRe.FindStringSubmatch(line); m != nil {
				api = m[1]
			}
			seen[api+" "+v[1]]++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	var bad []string
	for k := range seen {
		api, val, _ := strings.Cut(k, " ")
		if !atEOTCarrierAPIs[api] {
			bad = append(bad, "primitive "+k+" is not a registered AtEOT$ carrier")
		}
		if !cards.AtEOTValueSupported(val) && !atEOTUnmodelledValues[val] {
			bad = append(bad, "value "+k+" has no scheduled rider")
		}
	}
	sort.Strings(bad)
	if len(bad) != 0 {
		t.Fatalf("unclassified AtEOT$ carriers:\n%s", strings.Join(bad, "\n"))
	}
}
