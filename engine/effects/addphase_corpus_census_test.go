package effects

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestAddPhaseCorpusValuesResolveWithoutNote pins that effAddPhase's loud
// degrades (the Notes at the ExtraPhase$/AfterPhase$/FollowedBy$/NumPhases$/
// ExtraPhaseDelayedTrigger$ sites) are fail-closed guards no corpus card
// reaches: every AddPhase SA, printed or SVar body, carries values the
// parsers resolve. A failure names the card and the value.
func TestAddPhaseCorpusValuesResolveWithoutNote(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	heads := ModelledValueHeads()
	seen := 0
	// A printed ability reached again through an SVar is the same carrier:
	// report each failing card and value once.
	reported := map[string]bool{}
	fail := func(format string, args ...any) {
		if msg := fmt.Sprintf(format, args...); !reported[msg] {
			reported[msg] = true
			t.Error(msg)
		}
	}
	for _, card := range reg.AllCards() {
		for _, face := range card.Faces {
			var sas []*cards.SA
			var chain func(sa *cards.SA, depth int)
			chain = func(sa *cards.SA, depth int) {
				for ; sa != nil && depth < 32; sa, depth = sa.Sub, depth+1 {
					sas = append(sas, sa)
				}
			}
			for _, sa := range face.Abilities {
				chain(sa, 0)
			}
			names := make([]string, 0, len(face.SVars))
			for name := range face.SVars {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				chain(cards.ResolveSVar(face.SVars, name), 0)
			}
			for _, sa := range sas {
				if sa.API != "AddPhase" {
					continue
				}
				seen++
				who := face.Name + ": "
				if _, _, note := parseExtraPhaseValue(sa.ParamStr(cards.PKExtraPhase)); note != "" {
					fail("%sExtraPhase$ %q reaches a Note: %s", who, sa.ParamStr(cards.PKExtraPhase), note)
				}
				for _, k := range []struct {
					key  cards.ParamKey
					name string
				}{{cards.PKAfterPhase, "AfterPhase"}, {cards.PKFollowedBy, "FollowedBy"}} {
					raw := strings.TrimSpace(sa.ParamStr(k.key))
					if raw == "" {
						continue
					}
					if set, unknown := state.ParsePhases(raw); len(unknown) > 0 || set.Empty() {
						fail("%s%s$ %q has unknown tokens %v", who, k.name, raw, unknown)
					}
				}
				if raw, ok := sa.Param(cards.PKNumPhases); ok {
					raw = strings.TrimSpace(raw)
					_, err := strconv.Atoi(raw)
					head, isCount := cards.ValueHead(raw)
					if err != nil && !strings.HasPrefix(raw, "TriggerCount$") && !(isCount && slices.Contains(heads, head)) {
						fail("%sNumPhases$ %q is neither a literal nor a modelled head", who, raw)
					}
				}
				if name := strings.TrimSpace(sa.ParamStr(cards.PKExtraPhaseDelayedTrigger)); name != "" {
					if _, _, ok := delayedTriggerSpec(&Ctx{SVars: face.SVars}, name); !ok {
						fail("%sExtraPhaseDelayedTrigger$ %q does not resolve through delayedTriggerSpec", who, name)
					}
				}
			}
		}
	}
	// Vacuity guard: the corpus carries 58 AddPhase lines (a SVar-reached
	// body is visited more than once); fewer means the walk lost its carriers.
	if seen < 50 {
		t.Fatalf("walked %d AddPhase SAs, want at least 50 (is .cards linked?)", seen)
	}
}
