package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Tests for the scripted-scenario loader and validator (design
// scripted-scenario-starts, section 1 and ticket T2). The example JSON is the
// design's section-1 example with corpus-available cards (Helix Pinnacle is
// not in this corpus; Nantuko Shade is, with its f0/a0 pump ability).
const scenarioExample = `{"format":"gorge-start-1","focus":"Nantuko Shade","keys":["f0/a0"],
 "players":2,"game":"constructed","active":0,"turn":1,"phase":"main1","max_turns":12,
 "filler":"basics","opponent":"explore",
 "setup":{"p0":{"hand":["Nantuko Shade"],"battlefield":["Nantuko Shade","Swamp","Swamp","Swamp"],
                "tapped":["Swamp"],"counters":{"Nantuko Shade":{"P1P1":2}},
                "library":["Swamp"],"graveyard":[],"exile":[],"life":17},
          "p1":{"battlefield":["Grizzly Bears"]}}}`

// TestScenarioParse is the section-1 format: the example parses to its
// fields, the defaults apply, and a wrong format, an unknown field and
// malformed JSON are refused.
func TestScenarioParse(t *testing.T) {
	sc, err := ParseScenario([]byte(scenarioExample))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sc.Format != "gorge-start-1" || sc.Focus != "Nantuko Shade" {
		t.Fatalf("format/focus: %q %q", sc.Format, sc.Focus)
	}
	if len(sc.Keys) != 1 || sc.Keys[0] != "f0/a0" {
		t.Fatalf("keys: %v", sc.Keys)
	}
	if sc.Players != 2 || sc.Game != "constructed" || sc.Active != 0 || sc.Turn != 1 ||
		sc.Phase != "main1" || sc.MaxTurns != 12 || sc.Filler != "basics" || sc.Opponent != "explore" {
		t.Fatalf("header: players %d game %q active %d turn %d phase %q max_turns %d filler %q opponent %q",
			sc.Players, sc.Game, sc.Active, sc.Turn, sc.Phase, sc.MaxTurns, sc.Filler, sc.Opponent)
	}
	p0, ok := sc.Seats["p0"]
	if !ok {
		t.Fatal("setup p0 missing")
	}
	want := strings.Join([]string{"Nantuko Shade", "Nantuko Shade", "Swamp", "Swamp", "Swamp"}, "|")
	if got := strings.Join(append(append([]string(nil), p0.Hand...), p0.Battlefield...), "|"); got != want {
		t.Fatalf("p0 hand+battlefield: %q: want %q", got, want)
	}
	if len(p0.Tapped) != 1 || p0.Tapped[0] != "Swamp" {
		t.Fatalf("p0 tapped: %v", p0.Tapped)
	}
	if p0.Counters["Nantuko Shade"]["P1P1"] != 2 {
		t.Fatalf("p0 counters: %v", p0.Counters)
	}
	if len(p0.Library) != 1 || p0.Library[0] != "Swamp" || len(p0.Graveyard) != 0 || len(p0.Exile) != 0 {
		t.Fatalf("p0 zones: %v %v %v", p0.Library, p0.Graveyard, p0.Exile)
	}
	if p0.Life == nil || *p0.Life != 17 {
		t.Fatalf("p0 life: %v", p0.Life)
	}
	p1, ok := sc.Seats["p1"]
	if !ok || len(p1.Battlefield) != 1 || p1.Battlefield[0] != "Grizzly Bears" {
		t.Fatalf("p1: %+v", p1)
	}

	// Defaults: a header-less scenario is 2 players, constructed, turn 1
	// main1, 12 turns, basics.
	sc, err = ParseScenario([]byte(`{"format":"gorge-start-1","focus":"X","setup":{}}`))
	if err != nil {
		t.Fatalf("parse defaults: %v", err)
	}
	if sc.Players != 2 || sc.Game != "constructed" || sc.Turn != 1 || sc.Phase != "main1" ||
		sc.MaxTurns != 12 || sc.Filler != "basics" {
		t.Fatalf("defaults: players %d game %q turn %d phase %q max_turns %d filler %q",
			sc.Players, sc.Game, sc.Turn, sc.Phase, sc.MaxTurns, sc.Filler)
	}

	for name, raw := range map[string]string{
		"format":        `{"format":"gorge-start-2","focus":"X"}`,
		"unknown field": `{"format":"gorge-start-1","focus":"X","setup":{"p0":{"tokens":[{"script":"Bear"}]}}}`,
		"malformed":     `{"format":"gorge-start-1",`,
	} {
		if _, err := ParseScenario([]byte(raw)); err == nil {
			t.Errorf("%s: parsed, want refusal", name)
		}
	}
}

// scenarioValidateCase is one refusal or acceptance of Validate, over the
// shared corpus.
type scenarioValidateCase struct {
	name    string
	mutate  func(sc *Scenario)
	wantErr string // "" accepts
}

func baseValidateScenario() *Scenario {
	sc, err := ParseScenario([]byte(scenarioExample))
	if err != nil {
		panic(err)
	}
	return sc
}

func TestScenarioValidate(t *testing.T) {
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	// Preconditions the cases below lean on: the focus card has f0/a0 and
	// not a history-reading key; the refusal exemplars are what they claim.
	shade, ok := reg.Lookup("Nantuko Shade")
	if !ok {
		t.Fatal("Nantuko Shade not in corpus")
	}
	if keys := CoverageKeys(shade); !strings.Contains(strings.Join(keys, " "), "f0/a0") {
		t.Fatalf("Nantuko Shade keys %v: want f0/a0", keys)
	}
	mosq, ok := reg.Lookup("Heartstabber Mosquito")
	if !ok {
		t.Fatal("Heartstabber Mosquito not in corpus")
	}
	if keys := CoverageKeys(mosq); !strings.Contains(strings.Join(keys, " "), "f0/t0") {
		t.Fatalf("Heartstabber Mosquito keys %v: want f0/t0", keys)
	}
	gob, ok := reg.Lookup("Zhur-Taa Goblin")
	if !ok {
		t.Fatal("Zhur-Taa Goblin not in corpus")
	}
	if !rules.ETBChoiceAsks(gob) {
		t.Fatal("Zhur-Taa Goblin: want an as-enters choice (riot)")
	}

	// The pay-life as-enters shape (a non-keyword R:Event$ Moved body) and
	// the wasCast history spellings are what the findings measured missing.
	proc, ok := reg.Lookup("Phyrexian Processor")
	if !ok {
		t.Fatal("Phyrexian Processor not in corpus")
	}
	if !rules.ETBChoiceAsks(proc) {
		t.Fatal("Phyrexian Processor: want an as-enters choice (pay life)")
	}
	for _, name := range []string{"Zacama, Primal Calamity", "Wedding Ring"} {
		card, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s not in corpus", name)
		}
		if keys := CoverageKeys(card); !strings.Contains(strings.Join(keys, " "), "f0/t0") {
			t.Fatalf("%s keys %v: want f0/t0", name, keys)
		}
	}

	cases := []scenarioValidateCase{
		{name: "example", wantErr: ""},
		{name: "pay-life as-enters", wantErr: "as-enters choice",
			mutate: func(sc *Scenario) {
				setSeat(sc, "p1", func(s *ScenarioSeat) { s.Battlefield = []string{"Phyrexian Processor"} })
			}},
		{name: "wasCastByYou key", wantErr: "wascast",
			mutate: func(sc *Scenario) {
				sc.Focus = "Zacama, Primal Calamity"
				sc.Keys = []string{"f0/t0"}
			}},
		{name: "bare wasCast key", wantErr: "wascast",
			mutate: func(sc *Scenario) {
				sc.Focus = "Wedding Ring"
				sc.Keys = []string{"f0/t0"}
			}},
		{name: "padded setup key", wantErr: `setup seat "p01"`,
			mutate: func(sc *Scenario) { sc.Seats["p01"] = ScenarioSeat{Hand: []string{"Swamp"}} }},
		{name: "zero counters", wantErr: "want at least 1",
			mutate: func(sc *Scenario) {
				setSeat(sc, "p0", func(s *ScenarioSeat) { s.Counters = map[string]map[string]int32{"Nantuko Shade": {"P1P1": 0}} })
			}},
		{name: "zero life", wantErr: "life 0",
			mutate: func(sc *Scenario) {
				zero := int32(0)
				setSeat(sc, "p0", func(s *ScenarioSeat) { s.Life = &zero })
			}},
		{name: "unknown key", wantErr: "f0/a9",
			mutate: func(sc *Scenario) { sc.Keys = []string{"f0/a9"} }},
		{name: "history key", wantErr: "kicked",
			mutate: func(sc *Scenario) {
				sc.Focus = "Heartstabber Mosquito"
				sc.Keys = []string{"f0/t0"}
			}},
		{name: "unresolvable card", wantErr: "cannot hold",
			mutate: func(sc *Scenario) {
				setSeat(sc, "p1", func(s *ScenarioSeat) { s.Battlefield = []string{"Not a Card Name"} })
			}},
		{name: "as-enters choice", wantErr: "as-enters choice",
			mutate: func(sc *Scenario) {
				setSeat(sc, "p1", func(s *ScenarioSeat) { s.Battlefield = []string{"Zhur-Taa Goblin"} })
			}},
		{name: "tapped off battlefield", wantErr: "names no battlefield placement",
			mutate: func(sc *Scenario) { setSeat(sc, "p0", func(s *ScenarioSeat) { s.Tapped = []string{"Grizzly Bears"} }) }},
		{name: "counters off battlefield", wantErr: "names no battlefield placement",
			mutate: func(sc *Scenario) {
				setSeat(sc, "p0", func(s *ScenarioSeat) { s.Counters = map[string]map[string]int32{"Grizzly Bears": {"P1P1": 1}} })
			}},
		{name: "players 3", wantErr: "want 2 or 4",
			mutate: func(sc *Scenario) { sc.Players = 3 }},
		{name: "commander at 2", wantErr: "four players, never fewer",
			mutate: func(sc *Scenario) { sc.Game = "commander" }},
		{name: "4 players constructed", wantErr: "want game commander",
			mutate: func(sc *Scenario) {
				sc.Players = 4
				sc.Seats["p2"], sc.Seats["p3"] = ScenarioSeat{}, ScenarioSeat{}
			}},
		{name: "bad phase", wantErr: `phase "draw"`,
			mutate: func(sc *Scenario) { sc.Phase = "draw" }},
		{name: "bad opponent", wantErr: `opponent "random"`,
			mutate: func(sc *Scenario) { sc.Opponent = "random" }},
		{name: "bad filler", wantErr: `filler "sevens"`,
			mutate: func(sc *Scenario) { sc.Filler = "sevens" }},
		{name: "over MaxDeck", wantErr: "a seat holds 100",
			mutate: func(sc *Scenario) {
				lib := make([]string, 101)
				for i := range lib {
					lib[i] = "Forest"
				}
				setSeat(sc, "p0", func(s *ScenarioSeat) { s.Library = lib })
			}},
		{name: "commander pod", wantErr: "",
			mutate: func(sc *Scenario) {
				sc.Players = 4
				sc.Game = "commander"
				sc.Seats["p2"], sc.Seats["p3"] = ScenarioSeat{}, ScenarioSeat{}
				setSeat(sc, "p0", func(s *ScenarioSeat) { s.Commanders = []string{"Nantuko Shade"} })
			}},
	}
	for _, tc := range cases {
		sc := baseValidateScenario()
		if tc.mutate != nil {
			tc.mutate(sc)
		}
		err := sc.Validate(reg)
		switch {
		case tc.wantErr == "" && err != nil:
			t.Errorf("%s: %v, want acceptance", tc.name, err)
		case tc.wantErr != "" && err == nil:
			t.Errorf("%s: accepted, want %q", tc.name, tc.wantErr)
		case tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr):
			t.Errorf("%s: %v: want %q", tc.name, err, tc.wantErr)
		}
	}

	// An empty focus and a no-card focus are refused, naming the card.
	bare, err := ParseScenario([]byte(`{"format":"gorge-start-1","opponent":"explore"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := bare.Validate(reg); err == nil || !strings.Contains(err.Error(), "no focus") {
		t.Errorf("no focus: %v", err)
	}
	sc := baseValidateScenario()
	sc.Focus = "Unreal Card"
	if err := sc.Validate(reg); err == nil || !strings.Contains(err.Error(), "Unreal Card") {
		t.Errorf("bad focus: %v", err)
	}

	// D7: the game seed is the first seed >= seed whose toss names the
	// active seat. From one base seed the two seats' picks differ (a seed's
	// toss is one seat), and the pick is never below the base.
	sc = baseValidateScenario()
	sc.Active = 1
	s1 := sc.GameSeed(7)
	if s1 < 7 {
		t.Fatalf("GameSeed 7: %d", s1)
	}
	sc.Active = 0
	if s0 := sc.GameSeed(7); s0 == s1 {
		t.Fatalf("active 0 and 1 agree on seed %d", s0)
	}
}

// TestScenarioLayout is the deck layout and the start: the mentions land at
// their enumeration indices, the filler fills to 40, the start's indices
// point at those copies, and the matchup carries the start into the gorge
// config.
func TestScenarioLayout(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	c, err := OpenCorpus(cardsDir, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	sc, err := ParseScenario([]byte(scenarioExample))
	if err != nil {
		t.Fatal(err)
	}
	m, err := c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	if m.Start == nil || len(m.Start.Seats) != 2 {
		t.Fatalf("m.Start: %+v", m.Start)
	}
	if n := len(m.Decks[0]); n != 40 {
		t.Fatalf("p0 deck: %d cards, want 40", n)
	}
	if n := len(m.Decks[1]); n != 40 {
		t.Fatalf("p1 deck: %d cards, want 40", n)
	}
	name := func(d []*cards.Card, i int) string { return cardName(d[i]) }
	// p0's enumeration: hand, battlefield, graveyard, exile, library, then
	// the basics filler (Nantuko Shade is black: Swamps).
	for i, want := range map[int]string{0: "Nantuko Shade", 1: "Nantuko Shade", 2: "Swamp", 3: "Swamp", 4: "Swamp", 5: "Swamp", 39: "Swamp"} {
		if got := name(m.Decks[0], i); got != want {
			t.Fatalf("p0 deck[%d]: %s, want %s", i, got, want)
		}
	}
	// p1 names only Grizzly Bears; its filler is the focus card's colours
	// too (Swamps).
	if got := name(m.Decks[1], 0); got != "Grizzly Bears" {
		t.Fatalf("p1 deck[0]: %s, want Grizzly Bears", got)
	}
	if got := name(m.Decks[1], 39); got != "Swamp" {
		t.Fatalf("p1 deck[39]: %s, want Swamp (the focus card's colours)", got)
	}
	if !strings.Contains(strings.Join(m.Table, "|"), "Nantuko Shade") {
		t.Fatalf("table %v: no Nantuko Shade", m.Table)
	}
	seat := m.Start.Seats[0]
	if got := seat.Hand; len(got) != 1 || got[0] != 0 {
		t.Fatalf("p0 start hand: %v, want [0]", got)
	}
	if got := seat.Battlefield; !equalInts(got, []int{1, 2, 3, 4}) {
		t.Fatalf("p0 start battlefield: %v, want [1 2 3 4]", got)
	}
	// Tapped applies to every battlefield placement of the name.
	if got := seat.Tapped; !equalInts(got, []int{2, 3, 4}) {
		t.Fatalf("p0 start tapped: %v, want [2 3 4]", got)
	}
	if got := seat.Library; !equalInts(got, []int{5}) {
		t.Fatalf("p0 start library: %v, want [5]", got)
	}
	if seat.Counters == nil || len(seat.Counters) != 1 || seat.Counters[0] != (rules.StartCounter{Card: 1, Kind: "P1P1", N: 2}) {
		t.Fatalf("p0 start counters: %+v, want [{1 P1P1 2}]", seat.Counters)
	}
	if seat.Life == nil || *seat.Life != 17 {
		t.Fatalf("p0 start life: %v", seat.Life)
	}
	if got := m.Start.Seats[1].Battlefield; !equalInts(got, []int{0}) {
		t.Fatalf("p1 start battlefield: %v, want [0]", got)
	}
	// The gorge side gets the start through Matchup.Config.
	if cfg := m.Config(7); cfg.Start != m.Start {
		t.Fatal("m.Config: Start not carried")
	}

	// An Aura attached to the other seat's creature: with no battlefield copy
	// of its own it is a copy after the library mentions, joins the seat's
	// battlefield, and the start's Attach points at the bearer's deck index.
	aura := baseValidateScenario()
	setSeat(aura, "p0", func(s *ScenarioSeat) {
		s.Attached = []ScenarioAttach{{Card: "Pacifism", To: "p1:Grizzly Bears"}}
	})
	am, err := c.ScenarioMatchup(aura, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := name(am.Decks[0], 6); got != "Pacifism" {
		t.Fatalf("aura p0 deck[6]: %s, want Pacifism", got)
	}
	as := am.Start.Seats[0]
	if got := as.Battlefield; !equalInts(got, []int{1, 2, 3, 4, 6}) {
		t.Fatalf("aura p0 battlefield: %v, want [1 2 3 4 6]", got)
	}
	if len(as.Attach) != 1 || as.Attach[0] != (rules.StartAttach{Card: 6, ToSeat: 1, To: 0}) {
		t.Fatalf("aura p0 attach: %+v, want [{6 1 0}]", as.Attach)
	}
	// A bearer no seat placed is refused.
	noBearer := baseValidateScenario()
	setSeat(noBearer, "p0", func(s *ScenarioSeat) {
		s.Attached = []ScenarioAttach{{Card: "Pacifism", To: "p1:Llanowar Elves"}}
	})
	if _, err := c.ScenarioMatchup(noBearer, 0); err == nil {
		t.Fatal("aura with no bearer placement: accepted")
	}

	// Commanders: a name with no placement is its own copy after the
	// mentions; a name placed once reuses that copy.
	cmdr := baseValidateScenario()
	cmdr.Players, cmdr.Game = 4, "commander"
	cmdr.Seats["p2"], cmdr.Seats["p3"] = ScenarioSeat{}, ScenarioSeat{}
	setSeat(cmdr, "p0", func(s *ScenarioSeat) {
		s.Hand, s.Tapped, s.Counters = nil, nil, nil
		s.Battlefield = []string{"Swamp"}
		s.Commanders = []string{"Nantuko Shade"}
	})
	setSeat(cmdr, "p1", func(s *ScenarioSeat) { s.Commanders = []string{"Grizzly Bears"} })
	cm, err := c.ScenarioMatchup(cmdr, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !cm.Commander {
		t.Fatal("commander scenario: Matchup.Commander unset")
	}
	// p0: Swamp (0), library Swamp (1), then the command-zone Nantuko Shade.
	if cm.Commanders[0] != 2 || cm.Partners[0] != -1 {
		t.Fatalf("p0 commander %d partner %d, want 2 -1", cm.Commanders[0], cm.Partners[0])
	}
	// p1 places Grizzly Bears (0) and names it: it reuses that copy.
	if cm.Commanders[1] != 0 {
		t.Fatalf("p1 commander %d, want 0 (the placed copy)", cm.Commanders[1])
	}

	// Mulligans 0 only.
	if _, err := c.ScenarioMatchup(sc, 1); err == nil || !strings.Contains(err.Error(), "mulligans 0") {
		t.Fatalf("mulligans 1: %v", err)
	}

	// A deck:<name> filler takes the pool deck's cards, in file order.
	filler := deck.File{Name: "layout-filler", Format: "constructed",
		Cards: []deck.Entry{{Name: "Forest", Count: 50}}}
	c.AddDecks(map[string]deck.File{"layout-filler": filler})
	sc.Filler = "deck:layout-filler"
	m, err = c.ScenarioMatchup(sc, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i := 6; i < 40; i++ {
		if got := name(m.Decks[0], i); got != "Forest" {
			t.Fatalf("deck filler deck[%d]: %s, want Forest", i, got)
		}
	}
	// A pool deck too short for the remainder is refused.
	c.AddDecks(map[string]deck.File{"short-filler": {Name: "short-filler", Format: "constructed",
		Cards: []deck.Entry{{Name: "Forest", Count: 10}}}})
	sc.Filler = "deck:short-filler"
	if _, err := c.ScenarioMatchup(sc, 0); err == nil || !strings.Contains(err.Error(), "short of 40") {
		t.Fatalf("short filler: %v", err)
	}
}

// equalInts is a == b, for index lists.
func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setSeat mutates seat pN of sc's setup (a map value is not addressable).
func setSeat(sc *Scenario, seat string, f func(*ScenarioSeat)) {
	s := sc.Seats[seat]
	f(&s)
	sc.Seats[seat] = s
}
