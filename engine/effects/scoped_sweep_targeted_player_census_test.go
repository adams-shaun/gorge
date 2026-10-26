package effects

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// scopedSweepAPIs are the *All primitives whose battlefield sweep is scoped by
// targetPlayerKindScope. The census below pins every corpus carrier of a
// `Defined$ Targeted`/`TargetedPlayer` inheritance on one of them, so a new
// carrier has to be confirmed against that helper instead of silently
// sweeping every seat (the Sleep bug).
var scopedSweepAPIs = map[string]bool{
	"PumpAll":       true,
	"AnimateAll":    true,
	"DamageAll":     true,
	"TapOrUntapAll": true,
	"DestroyAll":    true,
}

func TestScopedSweepDefinedTargetedPlayerCensus(t *testing.T) {
	dir := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	got := map[string]bool{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(b), "\n") {
			api, defined := scopedSweepDefinedParam(line)
			if !scopedSweepAPIs[api] {
				continue
			}
			if !definedScopesToTargetedPlayer(&cards.SA{Params: map[string]string{"Defined": defined}}) {
				continue
			}
			got[filepath.Base(path)] = true
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk corpus: %v", err)
	}
	want := []string{"kiora_bests_the_sea_god.txt", "sleep.txt"}
	var names []string
	for n := range got {
		names = append(names, n)
	}
	sort.Strings(names)
	sort.Strings(want)
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("scoped-sweep Defined$ Targeted carriers = %v, want %v\n"+
			"a new carrier must be confirmed against targetPlayerKindScope and this list updated", names, want)
	}
}

// scopedSweepDefinedParam returns line's API token (the word after the first
// "$") and its Defined$ value, or "" when the line has none.
func scopedSweepDefinedParam(line string) (api, defined string) {
	_, rest, ok := strings.Cut(line, "$")
	if !ok {
		return "", ""
	}
	head, _, _ := strings.Cut(strings.TrimSpace(rest), "|")
	api = strings.TrimSpace(head)
	for _, part := range strings.Split(line, "|") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "Defined$"); ok {
			defined = strings.TrimSpace(v)
		}
	}
	return api, defined
}
