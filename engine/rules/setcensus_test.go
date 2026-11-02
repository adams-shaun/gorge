package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
)

// TestSetCensus reports, for a list of card names resolved through any Forge
// corpus, which fail to compile, name primitives the engine lacks, or carry
// params it never reads. It measures rather than asserts: an unreleased set's
// scripts change upstream faster than a baseline could follow, so
// scripts/set-census.sh drives it and the output is the record.
//
// GORGE_SET_CENSUS_CORPUS is the corpus directory and GORGE_SET_CENSUS_NAMES a
// file of card names, one per line.
func TestSetCensus(t *testing.T) {
	dir, list := os.Getenv("GORGE_SET_CENSUS_CORPUS"), os.Getenv("GORGE_SET_CENSUS_NAMES")
	if dir == "" || list == "" {
		t.Skip("GORGE_SET_CENSUS_CORPUS and GORGE_SET_CENSUS_NAMES are unset")
	}
	_, diags, err := cards.CompileDir(cards.CorpusDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	_, reads := measureParamCensus(t, nil)
	supported := effects.Supported()

	names := readSetNames(t, list)
	absent := 0
	byClass := map[string][]string{}
	clean := 0
	for _, name := range names {
		c, ok := reg.Lookup(name)
		if !ok {
			absent++
			continue
		}
		var gaps []string
		gaps = append(gaps, reg.Unsupported(c, supported)...)
		gaps = append(gaps, cardCensusLabels(c, reads, nil)...)
		if len(gaps) == 0 {
			clean++
			continue
		}
		for _, g := range gaps {
			byClass[g] = append(byClass[g], name)
		}
		t.Logf("CARD\t%s\t%s", name, strings.Join(gaps, ","))
	}
	for _, d := range diags {
		t.Logf("DIAG\t%s\t%s", filepath.Base(d.Path), d.Msg)
	}
	classes := make([]string, 0, len(byClass))
	for c := range byClass {
		classes = append(classes, c)
	}
	sort.Slice(classes, func(i, j int) bool {
		if len(byClass[classes[i]]) != len(byClass[classes[j]]) {
			return len(byClass[classes[i]]) > len(byClass[classes[j]])
		}
		return classes[i] < classes[j]
	})
	for _, c := range classes {
		t.Logf("CLASS\t%d\t%s\t%s", len(byClass[c]), c, strings.Join(byClass[c], "; "))
	}
	t.Logf("TOTAL\tnames=%d absent=%d clean=%d withgaps=%d classes=%d corpus-diagnostics=%d",
		len(names), absent, clean, len(names)-absent-clean, len(classes), len(diags))
}
