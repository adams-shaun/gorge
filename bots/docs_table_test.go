package bots_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all"
)

// TestBotsDocsTableListsEveryRegisteredBot keeps the overview table on
// docs/site/bots/compare.md in step with the registry: each registered bot has a
// row, with a verdict in the fair-or-clairvoyant column, and no row names a bot
// that is not registered. An Env policy's row must say it reads decklists.
func TestBotsDocsTableListsEveryRegisteredBot(t *testing.T) {
	raw, err := os.ReadFile("../docs/site/bots/compare.md")
	if err != nil {
		t.Fatal(err)
	}
	rowName := regexp.MustCompile("^\\| `([a-z0-9-]+)` \\|")
	rows := map[string]string{}
	main, _, _ := strings.Cut(string(raw), "\n## Formats")
	for _, line := range strings.Split(main, "\n") {
		if m := rowName.FindStringSubmatch(line); m != nil {
			rows[m[1]] = line
		}
	}
	for _, e := range bots.Entries() {
		row, ok := rows[e.Name]
		if !ok {
			t.Errorf("registered bot %q has no row in docs/site/bots/compare.md", e.Name)
			continue
		}
		cells := strings.Split(strings.Trim(row, "| "), " | ")
		if len(cells) != 6 {
			t.Errorf("row for %q has %d cells, want 6", e.Name, len(cells))
			continue
		}
		verdict := cells[5]
		if !strings.HasPrefix(verdict, "fair") && !strings.HasPrefix(verdict, "clairvoyant") {
			t.Errorf("row for %q: fair-or-clairvoyant cell %q starts with neither word", e.Name, verdict)
		}
		if e.ReadsDecklists() && !strings.Contains(verdict, "decklist") {
			t.Errorf("row for %q: the policy reads decklists and its verdict does not say so", e.Name)
		}
		if e.Search && !strings.Contains(cells[1]+cells[2], "gorge only") {
			t.Errorf("row for %q: a searching policy runs on gorge only, and its row does not say so", e.Name)
		}
	}
	for name := range rows {
		if _, ok := bots.Lookup(name); !ok {
			t.Errorf("docs/site/bots/compare.md has a row for %q, which is not registered", name)
		}
	}
}
