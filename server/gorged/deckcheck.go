package gorged

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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
	rep := d.report(format, f.Name, f, unresolved, unsupported, c.engine)
	if c.diracOn() {
		rep.Problems = append(rep.Problems, diracDeckProblem)
		rep.Legal, rep.Playable = false, false
	}
	d.demand.record(c.engine, rep)
	return rep, nil
}

// report is what the server finds in f for format: its size, commanders and
// colour identity, every construction problem, the names the card pool does
// not hold and every card engine cannot play. unresolved and unsupported are
// census's (or the ManaBrew import's) answers for f.
func (d *deckImports) report(format host.Format, name string, f deck.File, unresolved []string, unsupported map[string][]string, engine string) protocol.DeckReport {
	rep := protocol.DeckReport{
		Name: name, Format: format.String(), Engine: engine,
		Commanders: append([]string{}, f.CommanderNames()...),
		Problems:   []string{}, Unknown: append([]string{}, unresolved...),
		Unsupported: []protocol.DeckReportCard{}, Substituted: []protocol.DeckSubstitution{},
	}
	for _, e := range f.Cards {
		rep.Cards += e.Count
	}
	rep.Identity = d.identity(f)
	rep.Problems = append(rep.Problems, d.problems(format, f, unresolved)...)
	counts := d.copiesByFrontFace(f)
	for n, needs := range unsupported {
		rep.Unsupported = append(rep.Unsupported, protocol.DeckReportCard{Name: n, Count: max(counts[n], 1), Needs: append([]string{}, needs...)})
	}
	sort.Slice(rep.Unsupported, func(i, j int) bool { return rep.Unsupported[i].Name < rep.Unsupported[j].Name })
	rep.Legal = len(rep.Problems) == 0 && len(rep.Unknown) == 0
	rep.Playable = rep.Legal && len(rep.Unsupported) == 0
	return rep
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
