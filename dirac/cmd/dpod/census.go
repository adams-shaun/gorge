package main

// dpod -census: for every Commander deck file in a directory, whether dirac
// seats four copies of it in a Commander pod, exactly and with up to 3
// substituted copies, and which cards are in the way.
//
// decks.tsv, one row per deck (deck_id is the file stem):
//
//	cards, nonbasic     card copies in the deck, and those that are not a basic land
//	legal               yes / no (ValidateCommander failed with every card found) /
//	                    unknown (a card is not in the corpus)
//	exact               1: legal, every card supported, four copies build, the
//	                    table is not two-seat only, and the first step of a
//	                    4-seat Commander game does not stall
//	sub3                1: exact, or the same holds once the blocking cards are
//	                    replaced by the substitute basic and they total 3 copies
//	                    or fewer (a blocking commander makes it 0)
//	block_cards/copies  the blocking cards, and their copies
//	not_lowered ...     blocking cards per status
//	substitute          the replacement basic (deck.SubstituteBasic)
//	error               any other table-build error, else empty
//
// blockers.tsv, one row per blocking card of a deck: its copies, whether it
// is a commander (1/0), its status, its refusal class (classes: all of them,
// joined by " | ") and the reason.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// censusMaxRounds bounds a deck's substitution fixed point.
const censusMaxRounds = 64

// censusSub3 is the most substituted copies a sub3 deck may have.
const censusSub3 = 3

// censusStatuses are the blocker statuses, in decks.tsv column order.
var censusStatuses = [...]string{"not_lowered", "unverified", "refused", "two_seat", "interaction", "not_in_corpus"}

const (
	decksHeader    = "deck_id\tcards\tnonbasic\tlegal\texact\tsub3\tblock_cards\tblock_copies\tnot_lowered\tunverified\trefused\ttwo_seat\tinteraction\tnot_in_corpus\tsubstitute\terror"
	blockersHeader = "deck_id\tcard\tcopies\tcommander\tstatus\tclass\tclasses\treason"
)

// censusDeck is one deck file of the census.
type censusDeck struct {
	id string
	f  deck.File
}

// blocker is one card in a deck's way.
type blocker struct {
	card      string
	copies    int
	commander bool
	status    string
	class     string
	classes   string
	reason    string
}

// deckRow is one decks.tsv row and the deck's blockers.
type deckRow struct {
	id              string
	cards, nonbasic int
	legal           string
	exact, sub3     bool
	substitute, err string
	blockers        map[string]*blocker // by card
	commanders      map[string]bool     // normalized commander names
	counts          map[string]int      // card -> copies, by the deck's spelling
	commanderBlocks bool
	nameOf          map[string]string // deck spelling -> canonical (front face) name
	entries         []deck.Entry      // the deck's cards, in deck order
}

func runCensus(cardsDir, dir, out string, chunk int, stdout io.Writer) error {
	if out == "" {
		return fmt.Errorf("-census needs -census-out")
	}
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		return err
	}
	return census(reg, dir, out, chunk, stdout)
}

// census writes out/decks.tsv and out/blockers.tsv for the Commander deck
// files in dir, building one card table per chunk of decks.
func census(reg *cards.Registry, dir, out string, chunk int, stdout io.Writer) error {
	if chunk < 1 {
		chunk = 1
	}
	decks, err := readCensusDecks(dir)
	if err != nil {
		return err
	}
	var rows []*deckRow
	for i := 0; i < len(decks); i += chunk {
		rows = append(rows, censusChunk(reg, decks[i:min(i+chunk, len(decks))])...)
	}
	return writeCensus(reg, decks, rows, out, stdout)
}

// readCensusDecks reads dir/*.json in file-name order, keeping the files
// deck.Parse reads that name a commander.
func readCensusDecks(dir string) ([]censusDeck, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []censusDeck
	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := deck.Parse(raw)
		if err != nil || len(f.CommanderNames()) == 0 {
			continue
		}
		out = append(out, censusDeck{id: deck.Stem(p), f: f})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].id < out[j].id })
	return out, nil
}

// censusChunk censuses decks over one table of their distinct cards (and
// their substitute basics), releasing every matchup table it builds.
func censusChunk(reg *cards.Registry, decks []censusDeck) []*deckRow {
	var pool []*cards.Card
	seen := map[string]bool{}
	add := func(name string) {
		if c, ok := reg.Lookup(name); ok && len(c.Faces) > 0 && c.Faces[0] != nil && !seen[c.Faces[0].Name] {
			seen[c.Faces[0].Name] = true
			pool = append(pool, c)
		}
	}
	rows := make([]*deckRow, len(decks))
	for i, d := range decks {
		rows[i] = newDeckRow(reg, d)
		for _, e := range d.f.Cards {
			add(e.Name)
		}
		add(rows[i].substitute)
	}
	if len(pool) == 0 {
		for _, r := range rows {
			r.err = "no card of the deck is in the corpus"
		}
		return rows
	}
	pd, err := kgen.TableFromCards(reg, pool, "census")
	if err != nil {
		for _, r := range rows {
			r.err = err.Error()
		}
		return rows
	}
	idx := make(map[string]int, len(pd.Names))
	for i, n := range pd.Names {
		if i < len(pd.Status) {
			idx[n] = i
		}
	}
	set := cardset.NewDataSet(pd)
	cfg := dirac.LockstepConfig(dirac.SurfaceAutopay)
	cfg.Commander = true
	st := &game{cfg: cfg}
	for i, d := range decks {
		censusDeckRow(reg, pd, idx, set, st, d, rows[i])
	}
	return rows
}

func newDeckRow(reg *cards.Registry, d censusDeck) *deckRow {
	r := &deckRow{
		id:         d.id,
		substitute: deck.SubstituteBasic(d.f, reg),
		blockers:   map[string]*blocker{},
		commanders: map[string]bool{},
		counts:     map[string]int{},
		nameOf:     map[string]string{},
		entries:    d.f.Cards,
	}
	for _, n := range d.f.CommanderNames() {
		r.commanders[cards.NormalizeName(n)] = true
	}
	return r
}

// censusDeckRow fills r: legality, the by-status blockers, then the
// substitution fixed point over four copies of the deck.
func censusDeckRow(reg *cards.Registry, pd *cardset.PoolData, idx map[string]int, set cardset.Set, gm *game, d censusDeck, r *deckRow) {
	allFound := true
	for _, e := range d.f.Cards {
		r.cards += e.Count
		if !isBasic(e.Name) {
			r.nonbasic += e.Count
		}
		r.counts[e.Name] += e.Count
		c, ok := reg.Lookup(e.Name)
		if !ok {
			allFound = false
			r.block(e.Name, "not_in_corpus", "status:not_in_corpus", nil, "not in the corpus")
			continue
		}
		name := c.Faces[0].Name
		r.nameOf[e.Name] = name
		i, ok := idx[name]
		if !ok {
			r.err = fmt.Sprintf("%q is not in the census table", name)
			continue
		}
		r.blockStatus(e.Name, pd.Status[i], pd.Reason[i])
	}
	switch {
	case !allFound:
		r.legal = "unknown"
	case d.f.ValidateCommander(reg) != nil:
		r.legal = "no"
	default:
		r.legal = "yes"
	}
	if r.err != "" {
		return
	}
	ok := r.fixedPoint(reg, set, gm, d.f)
	r.exact = ok && r.legal == "yes" && len(r.blockers) == 0
	r.sub3 = r.exact || ok && !r.commanderBlocks && r.blockCopies() <= censusSub3 && r.substitutedLegal(reg, d.f)
}

// blockStatus records the card at table status st as a blocker, unless it is supported.
func (r *deckRow) blockStatus(card string, st cardset.Status, reason string) {
	switch st {
	case cardset.StatusNotLowered:
		cls := kgen.RefusalClasses(strings.TrimPrefix(reason, "not lowered: "))
		if len(cls) == 0 {
			cls = []string{"status:not_lowered"}
		}
		r.block(card, "not_lowered", cls[0], cls, reason)
	case cardset.StatusUnverified:
		r.block(card, "unverified", "status:unverified", nil, reason)
	case cardset.StatusRefused:
		r.block(card, "refused", "status:refused", nil, reason)
	}
}

func (r *deckRow) block(card, status, class string, classes []string, reason string) {
	if _, dup := r.blockers[card]; dup {
		return
	}
	if len(classes) == 0 {
		classes = []string{class}
	}
	cmd := r.commanders[cards.NormalizeName(card)]
	r.commanderBlocks = r.commanderBlocks || cmd
	r.blockers[card] = &blocker{card: card, copies: r.counts[card], commander: cmd, status: status, class: class, classes: strings.Join(classes, " | "), reason: reason}
}

func (r *deckRow) blockCopies() int {
	n := 0
	for _, b := range r.blockers {
		n += b.copies
	}
	return n
}

// deckCard is the deck entry spelled name: by its own spelling or its canonical one.
func (r *deckRow) deckCard(name string) (string, bool) {
	if _, ok := r.counts[name]; ok {
		return name, true
	}
	for _, e := range r.entries {
		if r.nameOf[e.Name] == name {
			return e.Name, true
		}
	}
	return "", false
}

// substitutable reports whether the deck card can be replaced by the substitute basic.
func (r *deckRow) substitutable(card string) bool {
	return !r.commanders[cards.NormalizeName(card)] && !isBasic(card)
}

// substituted is f with every blocking card replaced, copy for copy, by the substitute basic.
func (r *deckRow) substituted(f deck.File) deck.File {
	out := f
	out.Cards = nil
	for _, e := range f.Cards {
		if _, bad := r.blockers[e.Name]; bad {
			out.Cards = append(out.Cards, deck.Entry{Name: r.substitute, Count: e.Count})
		} else {
			out.Cards = append(out.Cards, e)
		}
	}
	return out
}

func (r *deckRow) substitutedLegal(reg *cards.Registry, f deck.File) bool {
	return r.substituted(f).ValidateCommander(reg) == nil
}

// fixedPoint builds four copies of the substituted deck until the table
// builds, is not two-seat only and starts without a stall, adding a blocker
// per refusal. It reports whether that point was reached; a blocking
// commander or basic, or a failure no substitution answers, ends it false.
func (r *deckRow) fixedPoint(reg *cards.Registry, set cardset.Set, gm *game, f deck.File) bool {
	for round := 0; round < censusMaxRounds; round++ {
		if r.commanderBlocks || r.basicBlocks() {
			return false
		}
		sf := r.substituted(f)
		var es []cardset.Entry
		for _, e := range sf.Cards {
			name := e.Name
			if canon, ok := r.nameOf[name]; ok {
				name = canon
			} else if c, ok := reg.Lookup(name); ok {
				name = c.Faces[0].Name
			}
			es = append(es, cardset.Entry{Name: name, Count: e.Count})
		}
		decks := make([][]cardset.Entry, podSeats)
		for s := range decks {
			decks[s] = es
		}
		tbl, ids, err := set.MatchupN(decks)
		grew, done := r.absorb(err)
		if done {
			return false
		}
		if grew {
			continue
		}
		if dirac.TwoSeatOnly(tbl) {
			grew = r.blockTwoSeat(tbl)
			dirac.ReleaseTable(tbl)
			if !grew {
				r.err = "the table is two-seat only"
				return false
			}
			continue
		}
		ok := r.startsClean(gm, tbl, ids, sf)
		dirac.ReleaseTable(tbl)
		return ok
	}
	r.err = fmt.Sprintf("substitution did not converge in %d rounds", censusMaxRounds)
	return false
}

// basicBlocks reports a blocking basic land, which no basic replaces.
func (r *deckRow) basicBlocks() bool {
	for _, b := range r.blockers {
		if isBasic(b.card) {
			return true
		}
	}
	return false
}

// absorb turns a MatchupN error into blockers: grew when it added one, done
// when nothing substitutable answers it (r.err then says why, unless a
// commander blocks).
func (r *deckRow) absorb(err error) (grew, done bool) {
	if err == nil {
		return false, false
	}
	var re *cardset.RefusedError
	var ir *cardset.InteractionRefusal
	switch {
	case errors.As(err, &re):
		card, ok := r.deckCard(re.Card)
		if !ok {
			r.err = err.Error()
			return false, true
		}
		before := len(r.blockers)
		r.blockStatus(card, re.Status, re.Reason)
		if len(r.blockers) == before {
			r.err = err.Error()
			return false, true
		}
		return true, false
	case errors.As(err, &ir):
		n := 0
		for _, c := range ir.Cards {
			card, ok := r.deckCard(c)
			if !ok || !r.substitutable(card) {
				continue
			}
			if _, dup := r.blockers[card]; !dup {
				r.block(card, "interaction", "status:interaction", nil, ir.Error())
				n++
			}
		}
		if n == 0 {
			r.err = err.Error()
			return false, true
		}
		return true, false
	}
	r.err = err.Error()
	return false, true
}

// blockTwoSeat adds the cards dirac.TwoSeatCauses names, reporting whether it added any.
func (r *deckRow) blockTwoSeat(tbl *dirac.Table) bool {
	n := 0
	for id, cs := range dirac.TwoSeatCauses(tbl) {
		if len(cs) == 0 {
			continue
		}
		card, ok := r.deckCard(tbl.Cards[id].Name)
		if !ok || !r.substitutable(card) {
			continue
		}
		if _, dup := r.blockers[card]; !dup {
			r.block(card, "two_seat", "status:two_seat", nil, strings.Join(cs, ", "))
			n++
		}
	}
	return n > 0
}

// startsClean seats four copies of sf at a Commander game and reports whether
// its first step leaves no stall.
func (r *deckRow) startsClean(gm *game, tbl *dirac.Table, ids [][]dirac.CardID, sf deck.File) bool {
	var decks [dirac.MaxSeats][]dirac.CardID
	cfg := gm.cfg
	for i := range cfg.Commanders {
		cfg.Commanders[i] = [2]int16{-1, -1}
	}
	names := sf.CommanderNames()
	for s := 0; s < podSeats; s++ {
		decks[s] = ids[s]
		for k, n := range names {
			if k >= 2 {
				break
			}
			want := cards.NormalizeName(n)
			for j, id := range ids[s] {
				if cards.NormalizeName(tbl.Cards[id].Name) == want {
					cfg.Commanders[s][k] = int16(j)
					break
				}
			}
			if cfg.Commanders[s][k] < 0 {
				r.err = fmt.Sprintf("commander %q not found in its kernel deck", n)
				return false
			}
		}
	}
	gm.s.InitN(tbl, decks, podSeats, 1, cfg)
	gm.s.StepInto(&gm.d)
	if why := gm.s.StallReason(); why != dirac.StallNone {
		r.err = "stall at the first step: " + dirac.StallName(why)
		return false
	}
	return true
}

// tsvField makes s one TSV value: tabs and newlines become spaces.
func tsvField(s string) string {
	return strings.NewReplacer("\t", " ", "\r", " ", "\n", " ").Replace(s)
}

func boolField(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// writeCensus writes the two tables and the stdout summary line.
func writeCensus(reg *cards.Registry, decks []censusDeck, rows []*deckRow, out string, stdout io.Writer) error {
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].id < rows[j].id })
	var dl, bl []string
	legal, exact, sub3 := 0, 0, 0
	blocking := map[string]bool{}
	for _, r := range rows {
		counts := map[string]int{}
		for _, b := range r.blockers {
			counts[b.status]++
		}
		f := []string{r.id, strconv.Itoa(r.cards), strconv.Itoa(r.nonbasic), r.legal, boolField(r.exact), boolField(r.sub3),
			strconv.Itoa(len(r.blockers)), strconv.Itoa(r.blockCopies())}
		for _, s := range censusStatuses {
			f = append(f, strconv.Itoa(counts[s]))
		}
		f = append(f, r.substitute, r.err)
		dl = append(dl, tsvJoin(f))
		if r.legal == "yes" {
			legal++
		}
		if r.exact {
			exact++
		}
		if r.sub3 {
			sub3++
		}
		var bs []string
		for _, b := range r.blockers {
			blocking[canonical(reg, b.card)] = true
			bs = append(bs, tsvJoin([]string{r.id, b.card, strconv.Itoa(b.copies), boolField(b.commander), b.status, b.class, b.classes, b.reason}))
		}
		sort.Strings(bs)
		bl = append(bl, bs...)
	}
	distinct := map[string]bool{}
	for _, d := range decks {
		for _, e := range d.f.Cards {
			distinct[canonical(reg, e.Name)] = true
		}
	}
	if err := writeTSV(filepath.Join(out, "decks.tsv"), decksHeader, dl); err != nil {
		return err
	}
	if err := writeTSV(filepath.Join(out, "blockers.tsv"), blockersHeader, bl); err != nil {
		return err
	}
	_, err := fmt.Fprintf(stdout, "census: decks=%d legal=%d exact=%d sub3=%d distinct_cards=%d blocking_cards=%d\n",
		len(rows), legal, exact, sub3, len(distinct), len(blocking))
	return err
}

// canonical is the card's front-face name, or name itself when the corpus lacks it.
func canonical(reg *cards.Registry, name string) string {
	if c, ok := reg.Lookup(name); ok && len(c.Faces) > 0 && c.Faces[0] != nil {
		return c.Faces[0].Name
	}
	return name
}

func tsvJoin(f []string) string {
	for i := range f {
		f[i] = tsvField(f[i])
	}
	return strings.Join(f, "\t")
}

func writeTSV(path, header string, rows []string) error {
	var b strings.Builder
	b.WriteString(header)
	b.WriteByte('\n')
	for _, r := range rows {
		b.WriteString(r)
		b.WriteByte('\n')
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}
