package oracle

import (
	"reflect"
	"testing"
)

// implicatesMatchup is three cards in two decks: objects 0-1 are deck 0's Alpha
// and Beta, objects 2-3 deck 1's Beta and Gamma, and anything past them a token.
func implicatesMatchup() *Matchup {
	return &Matchup{Table: []string{"Alpha", "Beta", "Gamma"}, Deck: [2][]CardIndex{{0, 1}, {1, 2}}}
}

func TestImplicatesNamesTheDifferingOption(t *testing.T) {
	m := implicatesMatchup()
	d := &Divergence{What: "options", Field: "options",
		RefOptions: []string{"pass(-,-,0)", "activate(#3,-,2)", "cast(#0,#3,0)"},
		KerOptions: []string{"pass(-,-,0)", "cast(#0,#3,0)", "land(#1,-,0)"},
		// a differing option outranks everything below it
		Diff:        []string{"Cards[0].Zone: ref=1 kernel=2"},
		LastSourced: "cast(#0,-,0)"}
	if got, want := d.Implicates(m), []string{"Gamma", "Beta"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Implicates = %v, want %v: only the options one side offers, gorge's first", got, want)
	}
	// Both copies of one card name it once; a target-only option names its target.
	d = &Divergence{What: "options", RefOptions: []string{"cast(#1,-,0)", "cast(#2,-,0)", "target(-,#3,0)"}}
	if got, want := d.Implicates(m), []string{"Beta", "Gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Implicates = %v, want %v", got, want)
	}
	// A kind mismatch's option lists are not a difference in options.
	d = &Divergence{What: "decision", Field: "kind", RefOptions: []string{"cast(#1,-,0)"}, KerOptions: []string{"attack(#3,-,0)"}, LastSourced: "cast(#0,-,0)"}
	if got, want := d.Implicates(m), []string{"Alpha"}; !reflect.DeepEqual(got, want) {
		t.Errorf("decision kind: Implicates = %v, want %v", got, want)
	}
	// An option about a token names no card, so the next rule answers.
	d = &Divergence{What: "options", RefOptions: []string{"activate(#9,-,0)"}, LastSourced: "cast(#3,-,0)"}
	if got, want := d.Implicates(m), []string{"Gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("token option: Implicates = %v, want %v", got, want)
	}
}

func TestImplicatesSkipsPosVictims(t *testing.T) {
	m := implicatesMatchup()
	d := &Divergence{What: "projection", Field: "Cards[0].Pos", Diff: []string{
		"NStack: ref=1 kernel=0",
		"Cards[0].Pos: ref=3 kernel=4",
		"Cards[1].Pos: ref=4 kernel=3",
		"Cards[3].Zone: ref=1 kernel=2",
		"Cards[2].Zone: ref=1 kernel=2",
	}, LastSourced: "cast(#0,-,0)"}
	if got, want := d.Implicates(m), []string{"Gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Implicates = %v, want %v: the first Cards[] field that is not Pos", got, want)
	}
	// Nothing but victims: the last sourced action answers, not a victim.
	d = &Divergence{What: "projection", Diff: []string{"Cards[1].Pos: ref=4 kernel=3"}, LastSourced: "cast(#3,-,0)"}
	if got, want := d.Implicates(m), []string{"Gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Implicates = %v, want %v", got, want)
	}
	// Nothing at all names a card.
	if got := (&Divergence{What: "projection", Diff: []string{"NInst: ref=1 kernel=2"}}).Implicates(m); got != nil {
		t.Errorf("Implicates = %v, want none", got)
	}
	if got := (&Divergence{What: "options", RefOptions: []string{"cast(#0,-,0)"}}).Implicates(nil); got != nil {
		t.Errorf("nil matchup: Implicates = %v, want none", got)
	}
}

func TestInstanceCard(t *testing.T) {
	m := implicatesMatchup()
	m.Deck[1][1] = NoCardIndex
	for o, want := range map[int]string{-1: "", 0: "Alpha", 1: "Beta", 2: "Beta", 3: "", 4: "", 99: ""} {
		if got := m.InstanceCard(o); got != want {
			t.Errorf("InstanceCard(%d) = %q, want %q", o, got, want)
		}
	}
}

// scriptSide poses its decisions in order, then ends the game.
type scriptSide struct {
	name string
	ds   []CDecision
	seq  int
}

func (s *scriptSide) SideName() string { return s.name }
func (s *scriptSide) Step() CDecision {
	if s.seq < len(s.ds) {
		return s.ds[s.seq]
	}
	return CDecision{Kind: DNone}
}
func (s *scriptSide) Apply(Canon)            { s.seq++ }
func (s *scriptSide) Terminal() (bool, int8) { return true, 0 }
func (s *scriptSide) Decided() (bool, int8)  { return true, 0 }
func (s *scriptSide) Project(*Projection)    {}
func (s *scriptSide) Dirty() bool            { return false }
func (s *scriptSide) Turn() int              { return 1 }
func (s *scriptSide) Err() error             { return nil }

// A pass has no source, so a divergence after a cast and some passes still
// reaches the cast: Divergence.Implicates needs it once Result.Actions is gone.
func TestLockstepRecordsLastSourcedAction(t *testing.T) {
	cast, pass := Pack(ActCast, Ref(3), RefNone, 0), Pack(ActPass, RefNone, RefNone, 0)
	one := func(c Canon) CDecision { return CDecision{Kind: DPriority, Options: []Canon{c}} }
	lead := &scriptSide{name: "gorge", ds: []CDecision{one(cast), one(pass), one(pass), one(pass)}}
	follow := &scriptSide{name: "kernel", ds: []CDecision{one(cast), one(pass), one(pass), one(cast)}}
	res := LockstepLed(nil, 1, lead, follow, FirstPolicy(), RunConfig{})
	if res.Div == nil || res.Div.What != "options" {
		t.Fatalf("want an options divergence, got %+v", res.Div)
	}
	if want := cast.String(); res.Div.LastSourced != want {
		t.Errorf("LastSourced = %q, want %q", res.Div.LastSourced, want)
	}
	if got, want := (&Divergence{What: "projection", LastSourced: res.Div.LastSourced}).Implicates(implicatesMatchup()), []string{"Gamma"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Implicates = %v, want %v", got, want)
	}
	if got := lastSourced([]Canon{pass, Pack(ActTarget, RefNone, Ref(3), 0), Pack(ActTarget, RefStack, RefNone, 0)}); got != "" {
		t.Errorf("lastSourced with no object source = %q, want none", got)
	}
}

// coverageOf is a game's coverage: one card with the given plays and keys used.
func coverageOf(card string, plays int, keys []string, used ...string) *Coverage {
	c := NewCoverage()
	cc := &CardCoverage{Cast: plays, Used: map[string]int{}, Keys: keys}
	for _, k := range used {
		cc.Used[k]++
	}
	c.Cards[card] = cc
	c.Games = 1
	return c
}

func TestCoverageCountsCleanGamesOnly(t *testing.T) {
	merged := NewCoverage()
	for _, g := range []struct {
		card string
		res  Result
	}{
		{"Clean", Result{Over: true}},
		{"Capped", Result{Stall: "turns"}}, // agreed for its whole length
		{"Diverged", Result{Div: &Divergence{What: "options"}}},
		{"Panicked", Result{Div: &Divergence{What: "kernel", Field: "panic"}}},
		{"AtCapacity", Result{Stall: KernelStallPrefix + "board"}}, // a declared limit: correct up to there
		{"Unmodelled", Result{Stall: KernelStallPrefix + "replleave", StallUnmodelled: true}},
	} {
		merged.MergeClean(coverageOf(g.card, 1, nil), &g.res)
	}
	if merged.Games != 3 || merged.Cards["Clean"] == nil || merged.Cards["Capped"] == nil || merged.Cards["AtCapacity"] == nil {
		t.Errorf("clean games not merged: games %d, cards %v", merged.Games, merged.names())
	}
	for _, n := range []string{"Diverged", "Panicked", "Unmodelled"} {
		if merged.Cards[n] != nil {
			t.Errorf("%s was merged: coverage counts only clean games", n)
		}
	}
	// The same run through Fuzz's rule: a card seen only in the dirty games has no evidence.
	if got := merged.Evidence(); len(got) != 0 {
		t.Errorf("one play each: evidence %v, want none", got)
	}
}

func TestCoverageEvidenceFloor(t *testing.T) {
	c := NewCoverage()
	for _, g := range []*Coverage{
		coverageOf("Too Few", EvidenceMinPlays-1, nil),
		coverageOf("Enough", EvidenceMinPlays, nil),
		coverageOf("Key Missed", EvidenceMinPlays, []string{"f0/a0", "f0/a1"}, "f0/a0"),
		coverageOf("All Keys", EvidenceMinPlays+1, []string{"f0/a0", "f0/a1"}, "f0/a0", "f0/a1"),
	} {
		c.Merge(g)
	}
	land := NewCoverage()
	land.Cards["Land"] = &CardCoverage{Land: EvidenceMinPlays, Used: map[string]int{}}
	c.Merge(land)
	if got, want := c.Evidence(), []string{"All Keys", "Enough", "Land"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Evidence = %v, want %v", got, want)
	}
}
