package effects

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestOptionalElectionCorpusCarriers pins the 16 corpus files carrying the
// five optional-election shapes cr-g128 repaired. The API token is the first
// field after "$ ", so a SubAbility$ named DBAnimate or a trigger line that
// merely mentions Animate/DealDamage does not count: only the resolving line
// itself must carry both the API and its key.
func TestOptionalElectionCorpusCarriers(t *testing.T) {
	testutil.CorpusRegistry(t) // absent .cards must skip rather than pass vacuously
	root := filepath.Join("..", "..", ".cards", "cardsfolder")
	checks := []struct {
		name string
		api  string
		keys []string
		want []string
	}{
		{name: "Animate.Optional", api: "Animate", keys: []string{"Optional$ True"}, want: []string{
			"c/crawling_barrens.txt",
			"e/ebony_fly.txt",
			"n/nissa_of_shadowed_boughs.txt",
			"s/shadow_puppeteers.txt",
			"s/sita_varma_masked_racer.txt",
			"t/the_irencrag.txt",
		}},
		{name: "AlterAttribute.Optional", api: "AlterAttribute", keys: []string{"Optional$ True"}, want: []string{
			"d/deadly_complication.txt",
			"r/rubblebelt_braggart.txt",
		}},
		{name: "DealDamage.OptionalDecider", api: "DealDamage", keys: []string{"OptionalDecider$"}, want: []string{
			"c/channel_harm.txt",
			"m/master_of_the_wild_hunt.txt",
			"r/requiem_monolith.txt",
			"s/spark_mage.txt",
			"v/valakut_the_molten_pinnacle.txt",
		}},
		{name: "Dig.DestZone2Optional", api: "Dig", keys: []string{"DestZone2Optional$ True"}, want: []string{
			"k/kellan_daring_traveler_journey_on.txt",
			"n/nylea_keen_eyed.txt",
		}},
		{name: "Mana.Chooser", api: "Mana", keys: []string{"Chooser$ You"}, want: []string{
			"r/radiant_lotus.txt",
		}},
	}
	for _, check := range checks {
		t.Run(check.name, func(t *testing.T) {
			var got []string
			err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if d.IsDir() || filepath.Ext(path) != ".txt" {
					return nil
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				for _, line := range strings.Split(string(data), "\n") {
					idx := strings.Index(line, "$ ")
					if idx < 0 {
						continue
					}
					token := strings.TrimSpace(strings.SplitN(line[idx+2:], "|", 2)[0])
					if token != check.api {
						continue
					}
					for _, key := range check.keys {
						if strings.Contains(line, key) {
							rel, _ := filepath.Rel(root, path)
							got = append(got, filepath.ToSlash(rel))
							break
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			slices.Sort(got)
			got = slices.Compact(got)
			if !slices.Equal(got, check.want) {
				t.Errorf("carrier files = %v, want %v", got, check.want)
			}
		})
	}
}
