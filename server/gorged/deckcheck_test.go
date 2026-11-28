package gorged

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/server/host"
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

// repoCommanderTextFor renders one named repo Commander deck as a plain-text
// decklist and returns both the text and the file the parser reads back from
// it, so a test withholds exactly the cards the server will census.
func repoCommanderTextFor(t *testing.T, name string) (string, deck.File) {
	t.Helper()
	f := testutil.RepoDeckFile(t, name)
	cmdrs := f.CommanderNames()
	if len(cmdrs) != 1 {
		t.Fatalf("%s names commanders %q, want one", name, cmdrs)
	}
	var lines []string
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
	text := listText(cmdrs[0], lines)
	return text, parseDeckText(t, text)
}

func parseDeckText(t *testing.T, text string) deck.File {
	t.Helper()
	tl, err := deck.ParseText([]byte(text))
	if err != nil {
		t.Fatalf("ParseText: %v", err)
	}
	return tl.File("")
}

func constructedText(f deck.File) string {
	var lines []string
	for _, e := range f.Cards {
		for range e.Count {
			lines = append(lines, "1 "+e.Name)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

// deckCardRefs counts, per distinct card name, how many cards in f reference
// each primitive or value head: a key used by exactly one card can be
// withheld to make that card and no other unsupported.
func deckCardRefs(reg *cards.Registry, f deck.File) map[string]int {
	usage := map[string]int{}
	seen := map[string]bool{}
	names := append([]string{}, f.CommanderNames()...)
	for _, e := range f.Cards {
		names = append(names, e.Name)
	}
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			continue
		}
		key := frontFaceName(c, n)
		if seen[key] {
			continue
		}
		seen[key] = true
		refs := map[string]bool{}
		for _, p := range c.Primitives() {
			refs[p] = true
		}
		for _, p := range c.ValueHeads() {
			refs[p] = true
		}
		for p := range refs {
			usage[p]++
		}
	}
	return usage
}

// withholdUniquePrimitives removes one card-only primitive from the supported
// map for n distinct cards of f and returns the map and their front-face
// names, sorted. The caller's census then finds exactly those n names
// unsupported.
func withholdUniquePrimitives(t *testing.T, reg *cards.Registry, f deck.File, n int) (map[string]bool, []string) {
	t.Helper()
	sup := newDeckImports(reg).supported
	usage := deckCardRefs(reg, f)
	cmdrs := map[string]bool{}
	for _, c := range f.CommanderNames() {
		cmdrs[cards.NormalizeName(c)] = true
	}
	picked := map[string]bool{}
	var outs []string
	for _, e := range f.Cards {
		if len(outs) == n {
			break
		}
		c, ok := reg.Lookup(e.Name)
		if !ok || cmdrs[cards.NormalizeName(e.Name)] || len(c.Faces) > 0 && c.Faces[0].IsBasic() {
			continue
		}
		key := frontFaceName(c, e.Name)
		if picked[key] || len(reg.Unsupported(c, sup)) > 0 {
			continue
		}
		for _, p := range c.Primitives() {
			if usage[p] == 1 {
				delete(sup, p)
				picked[key] = true
				outs = append(outs, key)
				break
			}
		}
	}
	if len(outs) != n {
		t.Fatalf("withheld %d cards, want %d", len(outs), n)
	}
	sort.Strings(outs)
	return sup, outs
}

// withholdCardPrimitive withholds a key only name references, making that one
// card unsupported.
func withholdCardPrimitive(t *testing.T, reg *cards.Registry, f deck.File, name string) map[string]bool {
	t.Helper()
	sup := newDeckImports(reg).supported
	usage := deckCardRefs(reg, f)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("%s is not in the corpus", name)
	}
	for _, p := range c.Primitives() {
		if usage[p] == 1 {
			delete(sup, p)
			return sup
		}
	}
	t.Fatalf("%s has no primitive only it references", name)
	return nil
}

func TestIdentityBasics(t *testing.T) {
	for _, tc := range []struct {
		mask uint8
		want []string
	}{
		{0, []string{"Wastes"}},
		{cards.ColourWhite, []string{"Plains"}},
		{cards.ColourBlue | cards.ColourBlack, []string{"Island", "Swamp"}},
		{cards.ColourWhite | cards.ColourBlue | cards.ColourBlack | cards.ColourRed | cards.ColourGreen,
			[]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}},
	} {
		if got := identityBasics(tc.mask); !slices.Equal(got, tc.want) {
			t.Errorf("identityBasics(%#x) = %q, want %q", tc.mask, got, tc.want)
		}
	}
}

// TestSubstituteUpToThree: a mono-black Commander deck with two unsupported
// names and consent is admitted with two Swamps in their slots; a two-colour
// constructed deck's three swaps alternate its two basics in WUBRG order; and
// the Commander deck without consent is refused deck_refused.
func TestSubstituteUpToThree(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	text, f := repoCommanderTextFor(t, "foundations-wretched-ranks")
	sup, outs := withholdUniquePrimitives(t, reg, f, 2)
	c := config{engine: "gorge", imports: newDeckImports(reg)}
	c.imports.supported = sup

	rep, err := c.checkDeck(httpapi.DeckCheckRequest{Format: "commander", Text: text, Substitute: true})
	if err != nil {
		t.Fatalf("checkDeck: %v", err)
	}
	if !rep.Legal || !rep.Playable {
		t.Fatalf("substituted commander deck: legal %v playable %v, problems %q", rep.Legal, rep.Playable, rep.Problems)
	}
	if len(rep.Substituted) != 2 {
		t.Fatalf("substituted = %+v, want 2 swaps", rep.Substituted)
	}
	for _, s := range rep.Substituted {
		if s.In != "Swamp" {
			t.Errorf("swap %+v: in = %q, want Swamp for a mono-black deck", s, s.In)
		}
	}
	if len(rep.Unsupported) != 2 {
		t.Fatalf("unsupported = %+v, want the two original names", rep.Unsupported)
	}
	for _, u := range rep.Unsupported {
		if !slices.Contains(outs, u.Name) {
			t.Errorf("unsupported %q is not one of the withheld cards %q", u.Name, outs)
		}
	}

	got, admitted, err := c.imports.check(host.FormatCommander, httpapi.SeatDeck{Seat: 0, Text: text, Substitute: true})
	if err != nil {
		t.Fatalf("seat a substituted deck: %v", err)
	}
	if len(admitted.Substituted) != 2 {
		t.Fatalf("seat report's substituted = %+v, want 2", admitted.Substituted)
	}
	if len(got.Cards) != 100 {
		t.Fatalf("the seated deck has %d cards, want 100", len(got.Cards))
	}
	held := map[string]bool{}
	for _, card := range got.Cards {
		held[frontFaceName(card, "")] = true
	}
	for _, out := range outs {
		if held[out] {
			t.Errorf("the seated deck still holds %q", out)
		}
	}

	_, noConsent, err := c.imports.check(host.FormatCommander, httpapi.SeatDeck{Seat: 0, Text: text})
	var refused httpapi.DeckRefused
	if !errors.As(err, &refused) {
		t.Fatalf("without consent: %v, want DeckRefused", err)
	}
	if len(noConsent.Substituted) != 0 || noConsent.Playable {
		t.Fatalf("without consent: substituted %+v playable %v, want none and false", noConsent.Substituted, noConsent.Playable)
	}

	f2 := testutil.RepoDeckFile(t, "ur-delver")
	text2 := constructedText(f2)
	sup2, _ := withholdUniquePrimitives(t, reg, f2, 3)
	c2 := config{engine: "gorge", imports: newDeckImports(reg)}
	c2.imports.supported = sup2
	rep2, err := c2.checkDeck(httpapi.DeckCheckRequest{Format: "constructed", Text: text2, Substitute: true})
	if err != nil {
		t.Fatalf("checkDeck constructed: %v", err)
	}
	if !rep2.Playable || len(rep2.Substituted) != 3 {
		t.Fatalf("two-colour substituted deck: playable %v substituted %+v, want playable and 3 swaps", rep2.Playable, rep2.Substituted)
	}
	identity := c2.imports.identity(f2)
	if len(identity) != 2 {
		t.Fatalf("ur-delver identity = %q, want two colours", identity)
	}
	basicFor := map[byte]string{'W': "Plains", 'U': "Island", 'B': "Swamp", 'R': "Mountain", 'G': "Forest"}
	basics := make([]string, len(identity))
	for i := range identity {
		basics[i] = basicFor[identity[i]]
	}
	for i, s := range rep2.Substituted {
		if want := basics[i%len(basics)]; s.In != want {
			t.Errorf("swap %d = %+v, want in %q (%s's WUBRG basics)", i, s, want, identity)
		}
	}
}

// TestSubstituteRefusesFour: four distinct unsupported names is one too many;
// the report keeps none of them substituted and names the cap.
func TestSubstituteRefusesFour(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	text, f := repoCommanderTextFor(t, "foundations-wretched-ranks")
	sup, _ := withholdUniquePrimitives(t, reg, f, 4)
	c := config{engine: "gorge", imports: newDeckImports(reg)}
	c.imports.supported = sup

	const want = "4 cards this engine cannot play yet; at most 3 can be substituted"
	rep, err := c.checkDeck(httpapi.DeckCheckRequest{Format: "commander", Text: text, Substitute: true})
	if err != nil {
		t.Fatalf("checkDeck: %v", err)
	}
	if rep.Playable || len(rep.Substituted) != 0 || !slices.Contains(rep.Problems, want) {
		t.Fatalf("four unsupported cards: playable %v substituted %+v problems %q; want refused with %q", rep.Playable, rep.Substituted, rep.Problems, want)
	}

	_, _, err = c.imports.check(host.FormatCommander, httpapi.SeatDeck{Seat: 0, Text: text, Substitute: true})
	var refused httpapi.DeckRefused
	if !errors.As(err, &refused) || !slices.Contains(refused.Report.Problems, want) {
		t.Fatalf("seat four unsupported cards: %v, report %+v; want DeckRefused naming the cap", err, refused.Report)
	}
}

// TestSubstituteNeverTouchesACommander: an unsupported commander is a named
// problem, never a basic land in the command zone.
func TestSubstituteNeverTouchesACommander(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	const cmdr = "Ghoulcaller Gisa"
	text := "Commander\n1 " + cmdr + "\n\n1 Swamp\n"
	f := parseDeckText(t, text)
	sup := withholdCardPrimitive(t, reg, f, cmdr)
	c := config{engine: "gorge", imports: newDeckImports(reg)}
	c.imports.supported = sup

	want := fmt.Sprintf("commander %q cannot be played by this engine yet", cmdr)
	rep, err := c.checkDeck(httpapi.DeckCheckRequest{Format: "commander", Text: text, Substitute: true})
	if err != nil {
		t.Fatalf("checkDeck: %v", err)
	}
	if rep.Playable || len(rep.Substituted) != 0 || !slices.Contains(rep.Problems, want) {
		t.Fatalf("unsupported commander: playable %v substituted %+v problems %q; want %q", rep.Playable, rep.Substituted, rep.Problems, want)
	}
	if !slices.Equal(rep.Commanders, []string{cmdr}) || rep.Cards != 2 {
		t.Fatalf("report %+v: want the commander and 2 cards unchanged", rep)
	}

	_, _, err = c.imports.check(host.FormatCommander, httpapi.SeatDeck{Seat: 0, Text: text, Substitute: true})
	var refused httpapi.DeckRefused
	if !errors.As(err, &refused) || !slices.Contains(refused.Report.Problems, want) {
		t.Fatalf("seat an unsupported commander: %v, report %+v; want DeckRefused naming it", err, refused.Report)
	}
}

// TestDeckDemandLogOnSubstitutedImport: substitution changes what is seated,
// never what the demand log records: the original unsupported names.
func TestDeckDemandLogOnSubstitutedImport(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	text, f := repoCommanderTextFor(t, "foundations-wretched-ranks")
	sup, outs := withholdUniquePrimitives(t, reg, f, 2)
	dir := t.TempDir()
	c := config{engine: "gorge", dir: dir, deckDemandLog: true, imports: newDeckImports(reg)}
	c.imports.supported = sup
	c.imports.demand = c.demandLog()
	c.imports.demand.now = func() time.Time { return time.Date(2026, 11, 27, 0, 0, 0, 0, time.UTC) }

	_, reps, err := c.importSeatDecks(httpapi.CreateGameOptions{
		Format: host.FormatCommander,
		Decks:  []httpapi.SeatDeck{{Seat: 0, Text: text, Substitute: true}},
	})
	if err != nil {
		t.Fatalf("importSeatDecks: %v", err)
	}
	if len(reps) != 1 || len(reps[0].Substituted) != 2 {
		t.Fatalf("reports = %+v, want one report with 2 swaps", reps)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "demand", "unsupported.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	if len(lines) != 1 {
		t.Fatalf("demand log has %d lines, want 1: %q", len(lines), raw)
	}
	var line demandLine
	if err := json.Unmarshal([]byte(lines[0]), &line); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(line.Unsupported, outs) || line.Engine != "gorge" {
		t.Fatalf("demand line = %+v, want engine gorge and original unsupported %q", line, outs)
	}
	for _, in := range []string{"Swamp", "Plains", "Island", "Mountain", "Forest", "Wastes"} {
		if slices.Contains(line.Unsupported, in) {
			t.Errorf("demand log recorded the substituted basic %q: %+v", in, line.Unsupported)
		}
	}
}
