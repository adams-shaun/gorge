package effects

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestIsTargetingValidIsBattlefieldOnlyClass pins the class: Forge resolves
// `IsTargeting Valid <spec>` through AbilityUtils.getDefinedObjects, whose
// `Valid ` form draws candidates from the battlefield only (CR 109.2, 110.1:
// "a creature you control" is a permanent). Every corpus carrier's argument
// therefore normalises to a spec pinned to the battlefield, while the bare
// forms (`IsTargeting Player`, `Self`, ...) stay zone-free. The carrier list
// is the census: a new carrier must be added here knowingly.
func TestIsTargetingValidIsBattlefieldOnlyClass(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ arg, want string }{
		{"Valid Creature.YouCtrl", "Creature.YouCtrl+inZoneBattlefield"},
		{"Valid Permanent,Player", "Permanent+inZoneBattlefield,Player+inZoneBattlefield"},
		{"Valid Permanent.YouCtrl~Other", "Permanent.YouCtrl+Other+inZoneBattlefield"},
		{"Valid Self", "Card.Self+inZoneBattlefield"},
		{"Player", "Player"},
		{"Creature.YouCtrl", "Creature.YouCtrl"},
		{"Self", "Card.Self"},
	} {
		got, ok := spellIsTargetingInner(tc.arg)
		if !ok || got != tc.want {
			t.Errorf("spellIsTargetingInner(%q) = %q, %v; want %q", tc.arg, got, ok, tc.want)
		}
	}

	dir := testutil.CorpusDirOrSkip(t)
	re := regexp.MustCompile(`IsTargeting (Valid [^,|\n]*)`)
	carriers := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(dir, "cardsfolder", "*", "*.txt"))
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			carriers[strings.TrimSuffix(filepath.Base(f), ".txt")] = true
			spec, ok := spellIsTargetingInner(strings.TrimSpace(m[1]))
			if !ok {
				t.Errorf("%s: %q is not a readable IsTargeting argument", f, m[1])
				continue
			}
			for alt := range strings.SplitSeq(spec, ",") {
				if !strings.Contains(alt, "inZoneBattlefield") {
					t.Errorf("%s: %q normalises to %q, not pinned to the battlefield", f, m[1], spec)
				}
			}
		}
	}
	got := make([]string, 0, len(carriers))
	for c := range carriers {
		got = append(got, c)
	}
	slices.Sort(got)
	want := []string{"danitha_sword_of_hope", "flash_photography", "head_of_the_class", "orvar_the_all_form", "professor_hojo", "shiko_and_narset_unified", "timely_ward"}
	if !slices.Equal(got, want) {
		t.Errorf("IsTargeting Valid carriers = %v; want %v", got, want)
	}
}
