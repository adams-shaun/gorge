package gorged

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/protocol"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// repoCommanderText renders a repo Commander deck as a plain-text decklist,
// one "1 Name" line per copy beside the commander, and returns the lines too.
func repoCommanderText(t *testing.T) (commander string, lines []string) {
	t.Helper()
	name := testutil.CommanderAcceptanceDeckNames()[0]
	f := testutil.RepoDeckFile(t, name)
	cmdrs := f.CommanderNames()
	if len(cmdrs) != 1 {
		t.Fatalf("%s names commanders %q, want one", name, cmdrs)
	}
	for _, e := range f.Cards {
		if e.Name == cmdrs[0] {
			continue
		}
		for range e.Count {
			lines = append(lines, "1 "+e.Name)
		}
	}
	if len(lines) != 99 {
		t.Fatalf("%s has %d cards beside its commander, want 99", name, len(lines))
	}
	return cmdrs[0], lines
}

func listText(commander string, lines []string) string {
	return "Commander\n1 " + commander + "\n\n" + strings.Join(lines, "\n") + "\n"
}

// TestDeckCheckReportsEveryCard: a repo Commander deck as text is legal and
// playable; a misspelt name is unknown; an off-identity card is a problem
// naming it; and every card a reduced supported map cannot play is listed
// with its needs, thirteen of them as readily as one.
func TestDeckCheckReportsEveryCard(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	c := config{engine: "gorge", imports: newDeckImports(reg)}
	check := func(text string) protocol.DeckReport {
		t.Helper()
		rep, err := c.checkDeck(httpapi.DeckCheckRequest{Text: text})
		if err != nil {
			t.Fatalf("checkDeck: %v", err)
		}
		return rep
	}
	cmdr, lines := repoCommanderText(t)

	rep := check(listText(cmdr, lines))
	if !rep.Legal || !rep.Playable || rep.Cards != 100 || rep.Format != "commander" || !slices.Equal(rep.Commanders, []string{cmdr}) || rep.Identity == "" {
		t.Fatalf("the repo deck: %+v, want legal, playable, 100 cards, commander %s", rep, cmdr)
	}
	if rep.Problems == nil || rep.Unknown == nil || rep.Unsupported == nil || rep.Substituted == nil {
		t.Fatalf("a list is nil: %+v", rep)
	}

	// One name misspelt.
	typo := slices.Clone(lines)
	typo[0] = "1 Sol Rnig"
	rep = check(listText(cmdr, typo))
	if rep.Legal || rep.Playable || !slices.Equal(rep.Unknown, []string{"Sol Rnig"}) {
		t.Fatalf("a misspelt name: legal %v, unknown %q; want not legal and unknown [Sol Rnig]", rep.Legal, rep.Unknown)
	}

	// An off-identity card in place of a basic.
	basic := slices.IndexFunc(lines, func(l string) bool {
		c, ok := reg.Lookup(strings.TrimPrefix(l, "1 "))
		return ok && c.Faces[0].IsBasic()
	})
	if basic < 0 {
		t.Fatal("the repo deck has no basic land to swap")
	}
	commander, _ := reg.Lookup(cmdr)
	off := ""
	for _, n := range []string{"Lightning Bolt", "Counterspell", "Giant Growth", "Swords to Plowshares", "Dark Ritual"} {
		if c, ok := reg.Lookup(n); ok && c.ColourIdentity()&^commander.ColourIdentity() != 0 {
			off = n
			break
		}
	}
	if off == "" {
		t.Fatalf("no off-identity card for %s", cmdr)
	}
	swapped := slices.Clone(lines)
	swapped[basic] = "1 " + off
	rep = check(listText(cmdr, swapped))
	if rep.Legal || len(rep.Unknown) != 0 || len(rep.Problems) == 0 || !strings.Contains(strings.Join(rep.Problems, "\n"), off) {
		t.Fatalf("an off-identity %s: legal %v, problems %q; want not legal and a problem naming it", off, rep.Legal, rep.Problems)
	}
	for _, p := range rep.Problems {
		if strings.Contains(p, "commander deck invalid") {
			t.Errorf("problem %q carries the error's header", p)
		}
	}

	// One primitive withheld: the first nonbasic card's.
	var nonbasic []*cards.Card
	seen := map[string]bool{}
	for _, l := range lines {
		c, ok := reg.Lookup(strings.TrimPrefix(l, "1 "))
		if ok && !c.Faces[0].IsBasic() && len(c.Primitives()) > 0 && !seen[frontFaceName(c, "")] {
			seen[frontFaceName(c, "")] = true
			nonbasic = append(nonbasic, c)
		}
	}
	if len(nonbasic) < 13 {
		t.Fatalf("the repo deck has %d distinct nonbasic cards with primitives, want at least 13", len(nonbasic))
	}
	withhold := func(cs []*cards.Card) {
		c.imports.supported = newDeckImports(reg).supported
		for _, card := range cs {
			delete(c.imports.supported, card.Primitives()[0])
		}
	}
	withhold(nonbasic[:1])
	rep = check(listText(cmdr, lines))
	first := frontFaceName(nonbasic[0], "")
	i := slices.IndexFunc(rep.Unsupported, func(u protocol.DeckReportCard) bool { return u.Name == first })
	if rep.Playable || !rep.Legal || i < 0 || !slices.Contains(rep.Unsupported[i].Needs, nonbasic[0].Primitives()[0]) || rep.Unsupported[i].Count < 1 {
		t.Fatalf("withholding %s: playable %v, unsupported %+v; want %s listed needing it", nonbasic[0].Primitives()[0], rep.Playable, rep.Unsupported, first)
	}

	withhold(nonbasic[:13])
	rep = check(listText(cmdr, lines))
	want := map[string]bool{}
	for _, l := range append([]string{"1 " + cmdr}, lines...) {
		if card, ok := reg.Lookup(strings.TrimPrefix(l, "1 ")); ok && len(reg.Unsupported(card, c.imports.supported)) > 0 {
			want[frontFaceName(card, "")] = true
		}
	}
	if len(want) < 13 || len(rep.Unsupported) != len(want) {
		t.Fatalf("withholding 13 cards' primitives: %d listed, want all %d", len(rep.Unsupported), len(want))
	}
	if !slices.IsSortedFunc(rep.Unsupported, func(a, b protocol.DeckReportCard) int { return strings.Compare(a.Name, b.Name) }) {
		t.Errorf("unsupported is not sorted by name: %+v", rep.Unsupported)
	}
	for _, u := range rep.Unsupported {
		if !want[u.Name] || len(u.Needs) == 0 {
			t.Errorf("unsupported %+v: not one of the withheld cards, or no needs", u)
		}
	}
}
