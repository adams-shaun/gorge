package oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Scenario is a parsed scripted start: section 1 of
// docs/design/scripted-scenario-starts.md. One JSON file per scenario,
// <card-slug>.<n>.json, is parsed into one of these; Corpus.ScenarioMatchup
// validates it against the corpus and lays out the decks and the start
// (Matchup.Start) both engines play from. The Decisions of the design are
// binding: D3 (ordinary shuffle + deal, then setup, the listed library cards
// on top), D6 (turn and phase come from play, not from setup) and D7 (the
// active player is chosen by seed, not by a field) shape the format.
type Scenario struct {
	Format   string                  `json:"format"`
	Focus    string                  `json:"focus"`
	Keys     []string                `json:"keys,omitempty"`
	Players  int                     `json:"players,omitempty"`
	Game     string                  `json:"game,omitempty"`
	Active   int                     `json:"active,omitempty"`
	Turn     int                     `json:"turn,omitempty"`
	Phase    string                  `json:"phase,omitempty"`
	MaxTurns int                     `json:"max_turns,omitempty"`
	Filler   string                  `json:"filler,omitempty"`
	Opponent string                  `json:"opponent,omitempty"`
	Seats    map[string]ScenarioSeat `json:"setup,omitempty"`
}

// ScenarioSeat is one seat of a scenario's setup block. The field names are
// oracleSeat's (D5): hand, battlefield, tapped, back_face, graveyard, exile,
// library, library_top, counters, life, attached, commanders. Every card is
// named, not indexed; the layout (layoutSeat) gives each mention its own deck
// copy, in a fixed enumeration order, and the start's indices point at those
// copies. Tapped, back_face and counters apply to every battlefield placement
// of the named card, as oracleSeat's do; attached names the Auras, each
// attached to its to (a battlefield placement of the same seat, or "pN:Name"
// of seat N); commanders names a seat's commander, a placement or a
// command-zone card.
//
// face_down, tokens and Equipment attached are v2 (T11): an unknown field is
// refused, so a v1 loader cannot silently drop one.
type ScenarioSeat struct {
	Hand        []string                    `json:"hand,omitempty"`
	Battlefield []string                    `json:"battlefield,omitempty"`
	BackFace    []string                    `json:"back_face,omitempty"`
	Tapped      []string                    `json:"tapped,omitempty"`
	Graveyard   []string                    `json:"graveyard,omitempty"`
	Exile       []string                    `json:"exile,omitempty"`
	Library     []string                    `json:"library,omitempty"`
	LibraryTop  []string                    `json:"library_top,omitempty"`
	Counters    map[string]map[string]int32 `json:"counters,omitempty"`
	Life        *int32                      `json:"life,omitempty"`
	Attached    []ScenarioAttach            `json:"attached,omitempty"`
	Commanders  []string                    `json:"commanders,omitempty"`
}

// ScenarioAttach is one attached entry: the Aura card, attached to its to
// (a battlefield placement of the same seat, or "pN:Name" of seat N).
type ScenarioAttach struct {
	Card string `json:"card"`
	To   string `json:"to"`
}

// The scenario format and its closed vocabularies (section 1). A value
// outside one is refused, naming it.
const (
	scenarioFormat = "gorge-start-1"

	// scenarioDeckSize is a scenario deck's size (section 1): the named cards
	// plus filler to 40. Named cards past 40 grow the deck; past MaxDeck the
	// loader refuses.
	scenarioDeckSize = 40

	// scenarioMaxDeck is dirac's deck capacity (dirac/state.go MaxDeck),
	// spelled here so the refusal names the bound the kernel enforces.
	scenarioMaxDeck = 100
)

var (
	scenarioPhases  = []string{"upkeep", "main1", "combat", "main2", "end"}
	scenarioGames   = []string{"constructed", "commander"}
	scenarioOppents = []string{"explore", "heuristic", "passive"}
)

// ParseScenario parses one scenario file's bytes: JSON, unknown fields
// refused, format gorge-start-1. It applies the section-1 defaults (two
// players, constructed, turn 1 main1, 12 turns, basics filler); Validate
// checks the vocabularies and the refusals.
func ParseScenario(raw []byte) (*Scenario, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var sc Scenario
	if err := dec.Decode(&sc); err != nil {
		return nil, fmt.Errorf("scenario: %w", err)
	}
	if sc.Format != scenarioFormat {
		return nil, fmt.Errorf("scenario: format %q: want %q", sc.Format, scenarioFormat)
	}
	if sc.Players == 0 {
		sc.Players = 2
	}
	if sc.Game == "" {
		sc.Game = "constructed"
	}
	if sc.Turn == 0 {
		sc.Turn = 1
	}
	if sc.Phase == "" {
		sc.Phase = "main1"
	}
	if sc.MaxTurns == 0 {
		sc.MaxTurns = 12
	}
	if sc.Filler == "" {
		sc.Filler = "basics"
	}
	return &sc, nil
}

// GameSeed is the game seed a run on seed plays this scenario with: the first
// seed >= seed whose CR 103.1 toss names the scenario's active seat (D7),
// rules.SeedForStarter. A later run checks the engine's starting player
// against Active after the game is built.
func (sc *Scenario) GameSeed(seed uint64) uint64 {
	return rules.SeedForStarter(seed, sc.Players, sc.Active)
}

// Validate checks sc against the corpus: the vocabularies, the focus card and
// its keys, and the section-1 refusals -- a card the scenario's table cannot
// hold (a name the corpus does not resolve), a battlefield card with an
// as-enters choice (gorge would park the entry where dirac has no matching
// state), a focus key whose ability reads history placement cannot fake
// (kicked, cast-from, entered-this-turn), and a seat with more than MaxDeck
// cards. Every error names the card it is about.
func (sc *Scenario) Validate(reg *cards.Registry) error {
	if sc.Players != 2 && sc.Players != 4 {
		return fmt.Errorf("scenario %s: players %d: want 2 or 4", sc.Focus, sc.Players)
	}
	if sc.Game == "commander" {
		if sc.Players != 4 {
			return fmt.Errorf("scenario %s: game commander: want 4 players, not %d (Commander is four players, never fewer)", sc.Focus, sc.Players)
		}
	} else if !slices.Contains(scenarioGames, sc.Game) {
		return fmt.Errorf("scenario %s: game %q: want one of %s", sc.Focus, sc.Game, strings.Join(scenarioGames, "|"))
	} else if sc.Players == 4 {
		return fmt.Errorf("scenario %s: 4 players: want game commander", sc.Focus)
	}
	if sc.Active < 0 || sc.Active >= sc.Players {
		return fmt.Errorf("scenario %s: active %d: want 0..%d", sc.Focus, sc.Active, sc.Players-1)
	}
	if sc.Turn < 1 {
		return fmt.Errorf("scenario %s: turn %d: want >= 1", sc.Focus, sc.Turn)
	}
	if !slices.Contains(scenarioPhases, sc.Phase) {
		return fmt.Errorf("scenario %s: phase %q: want one of %s", sc.Focus, sc.Phase, strings.Join(scenarioPhases, "|"))
	}
	if sc.MaxTurns < 1 {
		return fmt.Errorf("scenario %s: max_turns %d: want >= 1", sc.Focus, sc.MaxTurns)
	}
	if !slices.Contains(scenarioOppents, sc.Opponent) {
		return fmt.Errorf("scenario %s: opponent %q: want one of %s", sc.Focus, sc.Opponent, strings.Join(scenarioOppents, "|"))
	}
	if sc.Filler != "basics" && !strings.HasPrefix(sc.Filler, "deck:") {
		return fmt.Errorf("scenario %s: filler %q: want basics or deck:<name>", sc.Focus, sc.Filler)
	}
	focus, err := sc.focusCard(reg)
	if err != nil {
		return err
	}
	if err := sc.validateKeys(focus); err != nil {
		return err
	}
	for p := 0; p < sc.Players; p++ {
		seat, ok := sc.Seats[fmt.Sprintf("p%d", p)]
		if !ok {
			continue
		}
		if err := sc.validateSeat(reg, p, seat); err != nil {
			return err
		}
		if n := seat.mentions(); n > scenarioMaxDeck {
			return fmt.Errorf("scenario %s: p%d: %d named cards: a seat holds %d", sc.Focus, p, n, scenarioMaxDeck)
		}
	}
	for _, key := range scenarioSortedKeys(sc.Seats) {
		n, err := strconv.Atoi(strings.TrimPrefix(key, "p"))
		if err != nil || n < 0 || n >= sc.Players || key != fmt.Sprintf("p%d", n) {
			return fmt.Errorf("scenario %s: setup seat %q: want p0..p%d", sc.Focus, key, sc.Players-1)
		}
	}
	return nil
}

// focusCard resolves the scenario's focus card, which every scenario names.
func (sc *Scenario) focusCard(reg *cards.Registry) (*cards.Card, error) {
	if strings.TrimSpace(sc.Focus) == "" {
		return nil, fmt.Errorf("scenario: no focus")
	}
	c, ok := reg.Lookup(sc.Focus)
	if !ok {
		return nil, fmt.Errorf("scenario %s: the scenario's table cannot hold it: no card by that name", sc.Focus)
	}
	return c, nil
}

// validateKeys checks sc.Keys against the focus card's coverage inventory
// (CoverageKeys): a key the card does not have is refused, and a key whose
// ability reads history placement cannot fake is refused.
func (sc *Scenario) validateKeys(focus *cards.Card) error {
	keys := CoverageKeys(focus)
	for _, k := range sc.Keys {
		if !slices.Contains(keys, k) {
			return fmt.Errorf("scenario %s: key %s: the card does not have it", sc.Focus, k)
		}
		if mark := keyReadsHistory(focus, k); mark != "" {
			return fmt.Errorf("scenario %s: key %s: reads %s, which placement cannot fake", sc.Focus, k, mark)
		}
	}
	return nil
}

// historyMarkers are the script spellings of the history a placed permanent
// has none of: the per-object cast flags the filter and Count heads read
// (kicked covers Count$Kicked and Condition$ Kicked; bargained, surged,
// prowled and escaped are effects/compiled_predicate.go's flag words; foretold
// is filter.go's and Count$Foretold's; the evoked/dashed-style flags are read
// only by rules' alt-cost code, not by a script word, so they are not here),
// what entered the battlefield this or last turn (Count$ThisTurnEntered_ /
// Count$LastTurnEntered_) and how the card was cast (wascast covers wasCast,
// wasCastByYou and the wasCastFrom family, cast_provenance.go). Matched
// case-insensitively over the key's whole script text, a safe over-match: a
// refused key costs a scenario, an accepted one credits a key placement
// cannot reach.
var historyMarkers = []string{"kicked", "bargained", "surged", "prowled", "escaped", "foretold",
	"thisturnentered", "lastturnentered", "wascast"}

// keyReadsHistory returns the first history marker the key's script text
// reads, "" when it reads none.
func keyReadsHistory(c *cards.Card, key string) string {
	blob, ok := keyBlob(c, key)
	if !ok {
		return ""
	}
	low := strings.ToLower(blob)
	for _, m := range historyMarkers {
		if strings.Contains(low, m) {
			return m
		}
	}
	return ""
}

// keyBlob is the script text behind one coverage key of c: for an AB ability
// its params and sub-ability chain, for a trigger its own params (a
// ValidCard$ Card.Self+kicked reads history from the trigger, not the body)
// plus its effect chain and the SVar bodies its params name. "" false when
// the key names no slot of the card. The slots come from coverageInventory,
// the one enumeration CoverageKeys is built on, so the two cannot disagree on
// an index.
func keyBlob(c *cards.Card, key string) (string, bool) {
	for _, slot := range coverageInventory(c) {
		if slot.key != key {
			continue
		}
		var fi, idx int
		var kind rune
		if _, err := fmt.Sscanf(key, "f%d/%c%d", &fi, &kind, &idx); err != nil {
			return "", false
		}
		f := c.Faces[fi]
		if kind != 't' {
			return saBlob(f, slot.sa), true
		}
		t := f.Triggers[idx]
		var b strings.Builder
		for _, pk := range scenarioSortedKeys(t.Params) {
			b.WriteString(t.Params[pk])
			b.WriteString(" ")
			blobSVars(f, t.Params[pk], &b, 0)
		}
		b.WriteString(saBlob(f, slot.sa))
		return b.String(), true
	}
	return "", false
}

// saBlob is one ability's script text: its params, then its sub-ability
// chain's, each with the SVar bodies its values name.
func saBlob(f *cards.Face, a *cards.SA) string {
	var b strings.Builder
	for depth := 0; a != nil && depth < maxNameChooserDepth; depth++ {
		for _, pk := range scenarioSortedKeys(a.Params) {
			b.WriteString(a.Params[pk])
			b.WriteString(" ")
			blobSVars(f, a.Params[pk], &b, 0)
		}
		a = a.Sub
	}
	return b.String()
}

// blobSVars appends the bodies of f's SVars whose names appear as words in v,
// depth-capped so an SVar naming itself cannot loop.
func blobSVars(f *cards.Face, v string, b *strings.Builder, depth int) {
	if depth > 8 || f == nil {
		return
	}
	for _, w := range strings.FieldsFunc(v, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '_')
	}) {
		body, ok := f.SVars[w]
		if !ok {
			continue
		}
		b.WriteString(body)
		b.WriteString(" ")
		blobSVars(f, body, b, depth+1)
	}
}

// mentions is how many cards the seat names: each mention in the copy lists
// is one deck copy (a commander naming an already-placed card reuses that
// copy, so this over-counts by a placed commander's), so a seat naming more
// than MaxDeck cards is refused.
func (s ScenarioSeat) mentions() int {
	return len(s.Hand) + len(s.Battlefield) + len(s.Graveyard) + len(s.Exile) +
		len(s.Library) + len(s.LibraryTop) + len(s.Attached) + len(s.Commanders)
}

// validateSeat checks one seat's setup block against the corpus: every name
// resolves (the table cannot hold a name it has no card for), tapped,
// back_face and counters name battlefield placements, each attached entry's
// Aura resolves, names a bearer, and a commander is resolvable, at most two a
// seat (CR 903.3c). A battlefield placement with an as-enters choice (an Aura
// placed by attached too) is refused: gorge would park the entry where dirac
// has no matching state.
func (sc *Scenario) validateSeat(reg *cards.Registry, p int, s ScenarioSeat) error {
	cannotHold := func(where, name string) error {
		return fmt.Errorf("scenario %s: p%d %s %s: the scenario's table cannot hold it: no card by that name", sc.Focus, p, where, name)
	}
	bf := map[string]int{}
	for _, name := range s.Battlefield {
		c, ok := reg.Lookup(name)
		if !ok {
			return cannotHold("battlefield", name)
		}
		if rules.ETBChoiceAsks(c) {
			return fmt.Errorf("scenario %s: p%d battlefield %s: asks an as-enters choice, which placement cannot answer", sc.Focus, p, name)
		}
		bf[strings.ToLower(name)]++
	}
	for i := range s.Attached {
		a := &s.Attached[i]
		c, ok := reg.Lookup(a.Card)
		if !ok {
			return cannotHold("attached", a.Card)
		}
		if rules.ETBChoiceAsks(c) {
			return fmt.Errorf("scenario %s: p%d attached %s: asks an as-enters choice, which placement cannot answer", sc.Focus, p, a.Card)
		}
		seat, name, err := attachRef(a.To)
		if err != nil {
			return fmt.Errorf("scenario %s: p%d attached %s to %q: %v", sc.Focus, p, a.Card, a.To, err)
		}
		if seat == -1 {
			seat = p
		}
		if seat < 0 || seat >= sc.Players {
			return fmt.Errorf("scenario %s: p%d attached %s to %q: no seat p%d", sc.Focus, p, a.Card, a.To, seat)
		}
		if _, ok := reg.Lookup(name); !ok {
			return fmt.Errorf("scenario %s: p%d attached %s to %q: the scenario's table cannot hold it: no card by that name", sc.Focus, p, a.Card, a.To)
		}
	}
	for _, tagged := range [...][2]any{{"tapped", s.Tapped}, {"back_face", s.BackFace}} {
		kind, list := tagged[0].(string), tagged[1].([]string)
		for _, name := range list {
			if _, ok := reg.Lookup(name); !ok {
				return cannotHold(kind, name)
			}
			if bf[strings.ToLower(name)] == 0 {
				return fmt.Errorf("scenario %s: p%d %s %s: names no battlefield placement", sc.Focus, p, kind, name)
			}
		}
	}
	for _, name := range scenarioSortedKeys(s.Counters) {
		if _, ok := reg.Lookup(name); !ok {
			return cannotHold("counters", name)
		}
		if bf[strings.ToLower(name)] == 0 {
			return fmt.Errorf("scenario %s: p%d counters %s: names no battlefield placement", sc.Focus, p, name)
		}
		for _, kind := range scenarioSortedKeys(s.Counters[name]) {
			if strings.TrimSpace(kind) == "" {
				return fmt.Errorf("scenario %s: p%d counters %s: empty counter kind", sc.Focus, p, name)
			}
			if n := s.Counters[name][kind]; n <= 0 {
				return fmt.Errorf("scenario %s: p%d counters %s %s: %d: want at least 1", sc.Focus, p, name, kind, n)
			}
		}
	}
	if s.Life != nil && *s.Life <= 0 {
		return fmt.Errorf("scenario %s: p%d life %d: want at least 1", sc.Focus, p, *s.Life)
	}
	if n := len(s.Commanders); n > 2 {
		return fmt.Errorf("scenario %s: p%d: %d commanders: a seat has at most two (CR 903.3c)", sc.Focus, p, n)
	}
	for _, name := range s.Commanders {
		if _, ok := reg.Lookup(name); !ok {
			return cannotHold("commanders", name)
		}
	}
	return nil
}

// attachRef splits an attach bearer ref: "pN:Name" or "Name". seat -1 means
// the Aura's own seat.
func attachRef(ref string) (seat int, name string, err error) {
	seat = -1
	name = strings.TrimSpace(ref)
	if !strings.Contains(name, ":") {
		return seat, name, nil
	}
	v := strings.SplitN(name, ":", 2)
	n, e := strconv.Atoi(strings.TrimPrefix(v[0], "p"))
	if e != nil {
		return -1, "", fmt.Errorf("want \"Name\" or \"pN:Name\"")
	}
	return n, strings.TrimSpace(v[1]), nil
}

// sortedKeys is m's keys, sorted (no map range reaches an order).
func scenarioSortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// scenarioBasics is the five basics in WUBRG order, the colour bits of
// cards.ColourWhite..Green.
var scenarioBasics = [5]string{"Plains", "Island", "Swamp", "Mountain", "Forest"}

// basicsFor is the basics of a colour mask, WUBRG order; a mask with no
// colour (a colourless or artifact focus) takes all five.
func basicsFor(mask uint8) []string {
	var out []string
	for b := 0; b < 5; b++ {
		if mask&(1<<uint(b)) != 0 {
			out = append(out, scenarioBasics[b])
		}
	}
	if len(out) == 0 {
		return scenarioBasics[:]
	}
	return out
}

// ScenarioMatchup validates sc (Validate) and builds the matchup a scenario
// plays: each seat's deck (the named cards plus filler to 40, D3), the
// matchup over them, and the start (Matchup.Start) that both engines open in
// -- gorge's rules.Config.Start, copied into the config by Matchup.Config;
// the kernel's RealKernel builds its own start from the same indices (T4).
// The matchup plays mulligans 0 (the setup hook runs only then).
//
// The deck enumeration, per seat, is the seat's mentions in list order --
// hand, battlefield, graveyard, exile, library then library_top, the attached
// Auras, then a commander that names no placement -- each mention its own
// copy, then the filler. A start index is the deck index of the mention it
// places, so the same scenario resolves to the same instance on both engines
// (Matchup.InstanceCard).
func (c *Corpus) ScenarioMatchup(sc *Scenario, mulligans int) (*Matchup, error) {
	if err := sc.Validate(c.Reg); err != nil {
		return nil, err
	}
	if mulligans != 0 {
		return nil, fmt.Errorf("scenario %s: mulligans %d: scripted starts play mulligans 0", sc.Focus, mulligans)
	}
	plans := make([]*seatPlan, sc.Players)
	for p := range plans {
		pl, err := c.layoutSeat(sc, p)
		if err != nil {
			return nil, err
		}
		plans[p] = pl
	}
	spec := &rules.StartSpec{Seats: make([]rules.StartSeat, sc.Players)}
	for p := range plans {
		seat, err := c.startSeatOf(sc, p, plans)
		if err != nil {
			return nil, err
		}
		spec.Seats[p] = seat
	}
	decks := make([][]*cards.Card, sc.Players)
	for p := range plans {
		decks[p] = plans[p].deck
	}
	var m *Matchup
	var err error
	if sc.Players == 2 {
		m, err = NewMatchupSide(c.Reg, [2]string{"p0", "p1"}, [2][]*cards.Card{decks[0], decks[1]}, [2][]*cards.Card{}, mulligans)
	} else {
		m, err = NewMatchupN(c.Reg, seatNames(sc.Players), decks, mulligans)
	}
	if err != nil {
		return nil, err
	}
	if sc.Game == "commander" {
		m.Commander = true
		for p := 0; p < sc.Players; p++ {
			if cmd, partner := plans[p].commanderPair(); cmd >= 0 {
				m.setSeatCommanders(p, cmd, partner)
			}
		}
	}
	m.Start = spec
	return m, nil
}

// seatNames is the seat names of a players-seat scenario, "p0" on.
func seatNames(players int) []string {
	out := make([]string, players)
	for i := range out {
		out[i] = fmt.Sprintf("p%d", i)
	}
	return out
}

// seatPlan is one seat's laid-out deck and placements: deck[i] is the card at
// deck index i, byName a name's deck indices in enumeration order, fieldByName
// the battlefield ones, and the per-list placement indices in list order.
type seatPlan struct {
	deck        []*cards.Card
	byName      map[string][]int
	fieldByName map[string][]int
	hand, field []int
	grave       []int
	exile       []int
	lib         []int
	tapped      []int
	backFace    []int
	counters    []rules.StartCounter
	attach      []attPlan
	cmds        []int // commander deck indices, in commanders order
	life        *int32
}

// attPlan is one attached entry: the Aura's deck index (resolved at
// enumeration) and its bearer ref, resolved once every seat is laid out.
type attPlan struct {
	auraIx int
	toSeat int
	to     string
}

// commanderPair is the seat's commander and partner deck indices, -1 -1 when
// it names none.
func (pl *seatPlan) commanderPair() (int, int) {
	switch len(pl.cmds) {
	case 0:
		return -1, -1
	case 1:
		return pl.cmds[0], -1
	default:
		return pl.cmds[0], pl.cmds[1]
	}
}

// layoutSeat enumerates seat p's deck (the mentions, then the filler) and
// records where each placement landed.
func (c *Corpus) layoutSeat(sc *Scenario, p int) (*seatPlan, error) {
	seat := sc.Seats[fmt.Sprintf("p%d", p)]
	pl := &seatPlan{
		byName:      map[string][]int{},
		fieldByName: map[string][]int{},
		life:        seat.Life,
	}
	add := func(name string) (int, error) {
		card, ok := c.Reg.Lookup(name)
		if !ok {
			return 0, fmt.Errorf("scenario %s: p%d %s: the scenario's table cannot hold it: no card by that name", sc.Focus, p, name)
		}
		ix := len(pl.deck)
		pl.deck = append(pl.deck, card)
		key := strings.ToLower(name)
		pl.byName[key] = append(pl.byName[key], ix)
		return ix, nil
	}
	addField := func(name string) (int, error) {
		ix, err := add(name)
		if err != nil {
			return 0, err
		}
		pl.fieldByName[strings.ToLower(name)] = append(pl.fieldByName[strings.ToLower(name)], ix)
		pl.field = append(pl.field, ix)
		return ix, nil
	}
	for _, name := range seat.Hand {
		ix, err := add(name)
		if err != nil {
			return nil, err
		}
		pl.hand = append(pl.hand, ix)
	}
	for _, name := range seat.Battlefield {
		if _, err := addField(name); err != nil {
			return nil, err
		}
	}
	for _, list := range [...][2]any{{"graveyard", seat.Graveyard}, {"exile", seat.Exile}} {
		for _, name := range list[1].([]string) {
			ix, err := add(name)
			if err != nil {
				return nil, err
			}
			if list[0] == "graveyard" {
				pl.grave = append(pl.grave, ix)
			} else {
				pl.exile = append(pl.exile, ix)
			}
		}
	}
	for _, name := range append(slices.Clone(seat.Library), seat.LibraryTop...) {
		ix, err := add(name)
		if err != nil {
			return nil, err
		}
		pl.lib = append(pl.lib, ix)
	}
	for i, a := range seat.Attached {
		// The Aura reuses the seat's battlefield copy when one is there (the
		// k-th of the name, k = this entry's ordinal among the name's
		// attached entries); else it is its own copy, placed after the
		// unattached permanents (StartSpec.Attach).
		key := strings.ToLower(a.Card)
		k := 0
		for j := 0; j < i; j++ {
			if strings.EqualFold(seat.Attached[j].Card, a.Card) {
				k++
			}
		}
		ix, err := func() (int, error) {
			if k < len(pl.fieldByName[key]) {
				return pl.fieldByName[key][k], nil
			}
			return addField(a.Card)
		}()
		if err != nil {
			return nil, err
		}
		seatN, to, err := attachRef(a.To)
		if err != nil {
			return nil, fmt.Errorf("scenario %s: p%d attached %s to %q: %v", sc.Focus, p, a.Card, a.To, err)
		}
		if seatN == -1 {
			seatN = p
		}
		pl.attach = append(pl.attach, attPlan{auraIx: ix, toSeat: seatN, to: to})
	}
	for _, name := range seat.Tapped {
		for _, ix := range pl.fieldByName[strings.ToLower(name)] {
			if !slices.Contains(pl.tapped, ix) {
				pl.tapped = append(pl.tapped, ix)
			}
		}
	}
	for _, name := range seat.BackFace {
		for _, ix := range pl.fieldByName[strings.ToLower(name)] {
			if !slices.Contains(pl.backFace, ix) {
				pl.backFace = append(pl.backFace, ix)
			}
		}
	}
	for _, name := range scenarioSortedKeys(seat.Counters) {
		for _, ix := range pl.fieldByName[strings.ToLower(name)] {
			for _, kind := range scenarioSortedKeys(seat.Counters[name]) {
				pl.counters = append(pl.counters, rules.StartCounter{Card: ix, Kind: kind, N: seat.Counters[name][kind]})
			}
		}
	}
	sort.Slice(pl.counters, func(i, j int) bool {
		a, b := pl.counters[i], pl.counters[j]
		return a.Card < b.Card || a.Card == b.Card && a.Kind < b.Kind
	})
	for _, name := range seat.Commanders {
		key := strings.ToLower(name)
		switch n := len(pl.byName[key]); {
		case n == 1:
			pl.cmds = append(pl.cmds, pl.byName[key][0])
		case n == 0:
			// a command-zone commander: its own deck copy, placed by
			// genesis' commander seating, by no placement list
			ix, err := add(name)
			if err != nil {
				return nil, err
			}
			pl.cmds = append(pl.cmds, ix)
		default:
			return nil, fmt.Errorf("scenario %s: p%d commanders %s: names %d placements: place exactly once", sc.Focus, p, name, n)
		}
	}
	if err := c.fillSeat(sc, p, pl); err != nil {
		return nil, err
	}
	if len(pl.deck) > scenarioMaxDeck {
		return nil, fmt.Errorf("scenario %s: p%d: %d cards: a seat holds %d", sc.Focus, p, len(pl.deck), scenarioMaxDeck)
	}
	return pl, nil
}

// fillSeat grows the laid-out deck to scenarioDeckSize cards with the
// scenario's filler: basics in the focus card's colours (all five for a
// colourless focus), cycling WUBRG, or a pool deck's cards in file order
// (refused when the deck runs short).
func (c *Corpus) fillSeat(sc *Scenario, p int, pl *seatPlan) error {
	if len(pl.deck) >= scenarioDeckSize {
		return nil
	}
	need := scenarioDeckSize - len(pl.deck)
	if sc.Filler == "basics" {
		focus, ok := c.Reg.Lookup(sc.Focus)
		if !ok {
			return fmt.Errorf("scenario %s: the scenario's table cannot hold it: no card by that name", sc.Focus)
		}
		basics := basicsFor(focus.ColourIdentity())
		for i := 0; len(pl.deck) < scenarioDeckSize; i++ {
			name := basics[i%len(basics)]
			card, ok := c.Reg.Lookup(name)
			if !ok {
				return fmt.Errorf("scenario %s: p%d filler %s: the basic %s is not in the corpus", sc.Focus, p, sc.Filler, name)
			}
			pl.deck = append(pl.deck, card)
		}
		return nil
	}
	pool, err := c.Deck(strings.TrimPrefix(sc.Filler, "deck:"))
	if err != nil {
		return fmt.Errorf("scenario %s: filler %s: %v", sc.Focus, sc.Filler, err)
	}
	if len(pool) < need {
		return fmt.Errorf("scenario %s: filler %s: %d cards, %d short of %d", sc.Focus, sc.Filler, len(pool), need-len(pool), scenarioDeckSize)
	}
	pl.deck = append(pl.deck, pool[:need]...)
	return nil
}

// startSeatOf builds seat p's StartSeat from its laid-out plan. The attached
// entries are resolved here, when every seat's placements exist: the Aura's
// bearer is the first battlefield placement of the named card in the named
// seat (a bearer of another seat is written "pN:Name"), and a bearer no seat
// has placed is refused.
func (c *Corpus) startSeatOf(sc *Scenario, p int, plans []*seatPlan) (rules.StartSeat, error) {
	pl := plans[p]
	seat := rules.StartSeat{
		Hand:        append([]int(nil), pl.hand...),
		Battlefield: append([]int(nil), pl.field...),
		Graveyard:   append([]int(nil), pl.grave...),
		Exile:       append([]int(nil), pl.exile...),
		Library:     append([]int(nil), pl.lib...),
		Tapped:      append([]int(nil), pl.tapped...),
		BackFace:    append([]int(nil), pl.backFace...),
		Counters:    append([]rules.StartCounter(nil), pl.counters...),
		Life:        pl.life,
	}
	for _, a := range pl.attach {
		bearerPl := plans[a.toSeat]
		ixs, ok := bearerPl.fieldByName[strings.ToLower(a.to)]
		if !ok || len(ixs) == 0 {
			return seat, fmt.Errorf("scenario %s: p%d attached to %q: names no battlefield placement of p%d", sc.Focus, p, a.to, a.toSeat)
		}
		seat.Attach = append(seat.Attach, rules.StartAttach{Card: a.auraIx, ToSeat: a.toSeat, To: ixs[0]})
	}
	return seat, nil
}
