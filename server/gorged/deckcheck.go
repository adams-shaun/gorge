package gorged

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/server/host"
	"github.com/adams-shaun/gorge/server/host/httpapi"
	"github.com/adams-shaun/gorge/server/manabrew"
	"github.com/adams-shaun/gorge/server/protocol"

	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

// diracDeckProblem is the problem every report carries under -engine dirac,
// which seats only the decks it checked at startup.
const diracDeckProblem = "this server's engine plays only its startup decks"

// checkDeck answers POST /api/decks/check: it reads one submitted deck and
// reports on it without storing it or creating a table. Only a body the
// server cannot read is an error; an illegal or unplayable deck is a report.
func (c config) checkDeck(req httpapi.DeckCheckRequest) (protocol.DeckReport, error) {
	format := host.FormatCommander
	if req.Format != "" {
		f, err := host.ParseFormat(req.Format)
		if err != nil {
			return protocol.DeckReport{}, err
		}
		format = f
	}
	d := c.imports
	var (
		f           deck.File
		unresolved  []string
		unsupported map[string][]string
	)
	if req.Text != "" {
		tl, err := deck.ParseText([]byte(req.Text))
		if err != nil {
			return protocol.DeckReport{}, err
		}
		f = tl.File("")
		unresolved, unsupported = d.census(f)
	} else {
		var md mb.Deck
		if err := json.Unmarshal(req.Deck, &md); err != nil {
			return protocol.DeckReport{}, fmt.Errorf("deck is not a ManaBrew deck: %w", err)
		}
		var rep manabrew.DeckImportReport
		f, rep, _ = manabrew.ImportDeck(d.reg, md, d.supported)
		unresolved, unsupported = rep.Unresolved, rep.Unsupported
	}
	orig := f
	var (
		substituted []protocol.DeckSubstitution
		problem     string
	)
	if req.Substitute && len(unresolved) == 0 {
		f, substituted, problem = substitute(d.reg, f, unsupported)
	}
	rep := d.report(format, f.Name, f, orig, unresolved, unsupported, d.engine, substituted)
	if problem != "" {
		rep.Problems = append(rep.Problems, problem)
		rep.Legal, rep.Playable = false, false
	}
	if c.diracOn() {
		rep.Problems = append(rep.Problems, diracDeckProblem)
		rep.Legal, rep.Playable = false, false
	}
	d.demand.record(rep)
	return rep, nil
}

// maxSubstituted is the most distinct unsupported card names a deck may ask
// the server to replace with basic lands.
const maxSubstituted = 3

// substitute replaces each unsupported card in f with a basic land of the
// deck's colour identity, when the deck names at most maxSubstituted of them
// and none is a commander. It returns the new file, the list of swaps in
// sorted unsupported-name order, and a problem line when it refused to
// substitute (empty when it applied or had nothing to do). The caller passes
// the original census so the report still names what the engine lacks.
func substitute(reg *cards.Registry, f deck.File, unsupported map[string][]string) (deck.File, []protocol.DeckSubstitution, string) {
	if len(unsupported) == 0 {
		return f, nil, ""
	}
	if len(unsupported) > maxSubstituted {
		return f, nil, fmt.Sprintf("%d cards this engine cannot play yet; at most %d can be substituted", len(unsupported), maxSubstituted)
	}
	for _, n := range f.CommanderNames() {
		key := n
		if c, ok := reg.Lookup(n); ok {
			key = frontFaceName(c, n)
		}
		if _, bad := unsupported[key]; bad {
			return f, nil, fmt.Sprintf("commander %q cannot be played by this engine yet", n)
		}
	}
	var mask uint8
	names := f.CommanderNames()
	if len(names) == 0 {
		for _, e := range f.Cards {
			names = append(names, e.Name)
		}
	}
	for _, n := range names {
		if c, ok := reg.Lookup(n); ok {
			mask |= c.ColourIdentity()
		}
	}
	basics := identityBasics(mask)
	names = make([]string, 0, len(unsupported))
	for n := range unsupported {
		names = append(names, n)
	}
	sort.Strings(names)
	to, swaps := make(map[string]string, len(names)), make([]protocol.DeckSubstitution, len(names))
	for i, n := range names {
		in := basics[i%len(basics)]
		to[n] = in
		swaps[i] = protocol.DeckSubstitution{Out: n, In: in}
	}
	counts := map[string]int{}
	out := f
	out.Cards = substituteEntries(reg, f.Cards, to, counts)
	out.Sideboard = substituteEntries(reg, f.Sideboard, to, counts)
	for i, n := range names {
		swaps[i].Count = max(counts[n], 1)
	}
	return out, swaps, ""
}

// substituteEntries replaces every entry whose front-face name is a key of to
// with that key's basic land, keeping its count and slot, and adds each
// replacement's copies to counts.
func substituteEntries(reg *cards.Registry, entries []deck.Entry, to map[string]string, counts map[string]int) []deck.Entry {
	if len(entries) == 0 {
		return entries
	}
	out := make([]deck.Entry, len(entries))
	copy(out, entries)
	for i, e := range out {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			continue
		}
		key := frontFaceName(c, e.Name)
		in, ok := to[key]
		if !ok {
			continue
		}
		counts[key] += e.Count
		out[i] = deck.Entry{Name: in, Count: e.Count}
	}
	return out
}

// identityBasics is the basic lands of a colour identity, in WUBRG order; a
// colourless identity uses Wastes.
func identityBasics(mask uint8) []string {
	var out []string
	for _, b := range []struct {
		bit  uint8
		name string
	}{
		{cards.ColourWhite, "Plains"}, {cards.ColourBlue, "Island"}, {cards.ColourBlack, "Swamp"},
		{cards.ColourRed, "Mountain"}, {cards.ColourGreen, "Forest"},
	} {
		if mask&b.bit != 0 {
			out = append(out, b.name)
		}
	}
	if len(out) == 0 {
		out = []string{"Wastes"}
	}
	return out
}

// report is what the server finds in f for format: its size, commanders and
// colour identity, every construction problem, the names the card pool does
// not hold and every card engine cannot play. unresolved and unsupported are
// census's (or the ManaBrew import's) answers for f; source is the deck before
// substitution, so Identity and Unsupported describe the submitted deck while
// f's problems judge the substituted one; substituted lists what f replaced.
func (d *deckImports) report(format host.Format, name string, f, source deck.File, unresolved []string, unsupported map[string][]string, engine string, substituted []protocol.DeckSubstitution) protocol.DeckReport {
	rep := protocol.DeckReport{
		Name: name, Format: format.String(), Engine: engine,
		Commanders: append([]string{}, f.CommanderNames()...),
		Problems:   []string{}, Unknown: append([]string{}, unresolved...),
		Unsupported: []protocol.DeckReportCard{}, Substituted: append([]protocol.DeckSubstitution{}, substituted...),
	}
	for _, e := range f.Cards {
		rep.Cards += e.Count
	}
	rep.Identity = d.identity(source)
	rep.Problems = append(rep.Problems, d.problems(format, f, unresolved)...)
	counts := d.copiesByFrontFace(source)
	for n, needs := range unsupported {
		rep.Unsupported = append(rep.Unsupported, protocol.DeckReportCard{Name: n, Count: max(counts[n], 1), Needs: append([]string{}, needs...)})
	}
	sort.Slice(rep.Unsupported, func(i, j int) bool { return rep.Unsupported[i].Name < rep.Unsupported[j].Name })
	rep.Legal = len(rep.Problems) == 0 && len(rep.Unknown) == 0
	rep.Playable = rep.Legal && (len(rep.Unsupported) == 0 || len(rep.Substituted) > 0)
	rep.Dirac = d.diracVerdict(f)
	return rep
}

// diracSupport is what a report reads of the dirac engine: a deck's support
// card by card (adapter.Engine).
type diracSupport interface {
	DeckSupport([]*cards.Card) adapter.DeckVerdict
}

// diracVerdict is dirac's verdict on f's resolved main deck, nil when the
// server judges no deck on dirac. Names the pool does not hold are skipped,
// as problems skips them.
func (d *deckImports) diracVerdict(f deck.File) *protocol.DeckEngineVerdict {
	if d.dirac == nil {
		return nil
	}
	var resolved []*cards.Card
	for _, e := range f.Cards {
		c, ok := d.reg.Lookup(e.Name)
		if !ok {
			continue
		}
		for range e.Count {
			resolved = append(resolved, c)
		}
	}
	v := d.dirac.DeckSupport(resolved)
	out := &protocol.DeckEngineVerdict{
		Engine: adapter.Name, Set: v.Set, Playable: v.Err == nil,
		Unsupported: make([]protocol.DeckEngineCard, 0, len(v.Cards)), Problems: []string{},
	}
	for _, c := range v.Cards {
		out.Unsupported = append(out.Unsupported, protocol.DeckEngineCard{Name: c.Name, Count: c.Count, Status: c.Status, Reason: c.Reason})
	}
	if len(v.Cards) == 0 && v.Err != nil {
		out.Problems = append(out.Problems, v.Err.Error())
	}
	return out
}

// problems is checkImportedLegality's verdict on f, one line per violation.
// The cards the pool does not hold are left out first: the report names them
// under unknown, and the legality check would stop at the first one.
func (d *deckImports) problems(format host.Format, f deck.File, unresolved []string) []string {
	skip := map[string]bool{}
	for _, n := range unresolved {
		skip[n] = true
	}
	lf := f
	if lf.Name == "" {
		lf.Name = "deck"
	}
	lf.Cards = nil
	for _, e := range f.Cards {
		if !skip[e.Name] {
			lf.Cards = append(lf.Cards, e)
		}
	}
	err := checkImportedLegality(d.reg, format, lf)
	if err == nil {
		return nil
	}
	lines := strings.Split(err.Error(), "\n  ")
	if len(lines) > 1 && strings.HasSuffix(lines[0], ":") {
		lines = lines[1:]
	}
	return lines
}

// identity is the deck's colour identity as a subset of WUBRG: the union of
// its commanders' for a commander deck, of every resolved card's otherwise.
func (d *deckImports) identity(f deck.File) string {
	names := f.CommanderNames()
	if len(names) == 0 {
		for _, e := range f.Cards {
			names = append(names, e.Name)
		}
	}
	var m uint8
	for _, n := range names {
		if c, ok := d.reg.Lookup(n); ok {
			m |= c.ColourIdentity()
		}
	}
	var b strings.Builder
	for _, col := range []struct {
		bit uint8
		sym byte
	}{{cards.ColourWhite, 'W'}, {cards.ColourBlue, 'U'}, {cards.ColourBlack, 'B'}, {cards.ColourRed, 'R'}, {cards.ColourGreen, 'G'}} {
		if m&col.bit != 0 {
			b.WriteByte(col.sym)
		}
	}
	return b.String()
}

// copiesByFrontFace counts f's copies of each resolved card, main deck and
// sideboard, under the front-face name census keys unsupported cards by.
func (d *deckImports) copiesByFrontFace(f deck.File) map[string]int {
	out := map[string]int{}
	for _, list := range [][]deck.Entry{f.Cards, f.Sideboard} {
		for _, e := range list {
			if c, ok := d.reg.Lookup(e.Name); ok {
				out[frontFaceName(c, e.Name)] += e.Count
			}
		}
	}
	return out
}
