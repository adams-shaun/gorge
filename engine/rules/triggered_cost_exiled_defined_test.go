package rules

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestTriggeredCostExileFeedsExiledCardsDefined(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	e, _, ids := exileAnyGraveBoard(t, reg, "Lamplight Phoenix", "Hill Giant", "", "")
	ph := ids["Lamplight Phoenix"]
	giant := ids["Hill Giant"]
	e.emit(events.Event{Kind: events.MoveZone, Obj: ph, From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	e.putTriggersOnStack()
	e.pending = nil
	e.resolveTop()
	d := waitForCostWindow(t, e)
	submitChoices(t, e, d.Options[0].Index)
	for i := 0; i < 4; i++ {
		d = e.Pending()
		if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 {
			break
		}
		t.Logf("ask %d kind=%s opts=%d", i, d.Options[0].Kind, len(d.Options))
		submitChoices(t, e, d.Options[0].Index)
	}
	settleResolve(e)
	passUntilStackEmpty(t, e, 20)
	t.Logf("PHOENIX zone=%v GIANT zone=%v", e.G.Obj(ph).Zone, e.G.Obj(giant).Zone)
	if e.G.Obj(ph).Zone != state.ZBattlefield {
		t.Errorf("Phoenix not returned: zone=%v", e.G.Obj(ph).Zone)
	}
}

// TestTriggeredCostExileReadersCensus pins the class behind Lamplight Phoenix
// (CR 118.12, 603.4): every corpus trigger whose executed ability pays a cost
// with an Exile<...> part and whose body reads the exiled cards back
// (ExiledCards / Exiled$). The set is the contract for the paid-cost list the
// triggered-cost window records in castExiled (cumulative.go); a new member
// must be covered by a scenario test and a dirac lowering or a kgen refusal.
func TestTriggeredCostExileReadersCensus(t *testing.T) {
	dir := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	var got []string
	files, err := filepath.Glob(filepath.Join(dir, "*", "*.txt"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no corpus scripts under %s: %v", dir, err)
	}
	svar := regexp.MustCompile(`^SVar:(\w+):(.*)$`)
	exec := regexp.MustCompile(`Execute\$ (\w+)`)
	cost := regexp.MustCompile(`Cost\$[^|]*Exile`)
	reads := regexp.MustCompile(`ExiledCards|Exiled\$`)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(string(raw), "\n")
		sv := map[string]string{}
		for _, l := range lines {
			if m := svar.FindStringSubmatch(l); m != nil {
				sv[m[1]] = m[2]
			}
		}
		for _, l := range lines {
			if !strings.HasPrefix(l, "T:") {
				continue
			}
			m := exec.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			if b := sv[m[1]]; cost.MatchString(b) && reads.MatchString(b) {
				got = append(got, filepath.Base(f))
				break
			}
		}
	}
	sort.Strings(got)
	want := []string{"conspiracy_theorist.txt", "lamplight_phoenix.txt", "timothar_baron_of_bats.txt"}
	if !slices.Equal(got, want) {
		t.Fatalf("triggered exile-cost readers changed:\n got  %v\n want %v\ncover the new card with a scenario test, dirac lowering or kgen refusal, then update this list", got, want)
	}
}
