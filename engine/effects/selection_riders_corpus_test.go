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

// These corpus ratchets pin every file carrying one of the selection rider
// parameters this ticket made readable. A carrier that disappears means the
// corpus moved the card text; a new one means the mechanism's blast radius
// grew and needs review, not a silent allow-list edit.
func TestSelectionRiderCorpusCarriers(t *testing.T) {
	testutil.CorpusRegistry(t) // absent .cards must skip rather than pass vacuously
	root := filepath.Join("..", "..", ".cards", "cardsfolder")
	checks := []struct {
		name string
		api  string
		key  string
		want []string
	}{
		{name: "Dig.WithDifferentPowers", api: "Dig", key: "WithDifferentPowers$", want: []string{
			"r/rip_spawn_hunter.txt",
		}},
		{name: "Discard.RevealNumber", api: "Discard", key: "RevealNumber$", want: []string{
			"a/acquisitions_expert.txt",
			"b/bala_ged_thief.txt",
			"b/blackmail.txt",
			"c/cabal_interrogator.txt",
			"d/disciple_of_phenax.txt",
			"h/hollow_specter.txt",
			"k/klaw_sonic_subjugator.txt",
			"m/mires_toll.txt",
			"n/noggin_whack.txt",
			"p/pulse_of_the_dross.txt",
			"t/thieving_sprite.txt",
		}},
		{name: "ChangeZoneAll.WithCountersType", api: "ChangeZoneAll", key: "WithCountersType$", want: []string{
			"g/ghost_vacuum.txt",
			"s/shilgengar_sire_of_famine.txt",
		}},
		{name: "Fight.ExcessSVar", api: "Fight", key: "ExcessSVar$", want: []string{
			"r/rhinos_rampage.txt",
			"t/the_last_agni_kai.txt",
		}},
		{name: "RepeatEach.StartingWith", api: "RepeatEach", key: "StartingWith$", want: []string{
			"a/alliance_of_arms.txt",
			"c/collective_voyage.txt",
			"d/disciple_of_caelus_nin.txt",
			"e/eureka.txt",
			"f/fortunate_few.txt",
			"h/hypergenesis.txt",
			"l/liege_of_the_hollows.txt",
			"m/mana_charged_dragon.txt",
			"m/manifold_insights.txt",
			"m/minds_aglow.txt",
			"m/morphic_tide.txt",
			"p/plague_of_vermin.txt",
			"p/protection_racket.txt",
			"r/reality_shaping.txt",
			"r/rejoin_the_fight.txt",
			"s/shared_trauma.txt",
			"t/the_legend_of_yangchen_avatar_yangchen.txt",
			"t/thieves_auction.txt",
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
					if strings.Contains(line, check.api) && strings.Contains(line, check.key) {
						rel, _ := filepath.Rel(root, path)
						got = append(got, filepath.ToSlash(rel))
						break
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
