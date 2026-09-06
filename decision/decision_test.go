package decision

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

func sample() *Decision {
	return &Decision{
		Seq: 7, Player: 1, Kind: KPriority, Min: 1, Max: 1,
		Options: []Option{
			{Index: 0, Kind: "cast", Label: "Cast Lightning Bolt", Obj: 3},
			{Index: 1, Kind: "pass", Label: "Pass priority"},
		},
	}
}

func TestValidateRejectsWrongPlayer(t *testing.T) {
	err := sample().Validate(Intent{Seq: 7, Player: 0, Choices: []int{1}})
	if err == nil || !strings.Contains(err.Error(), "player") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsWrongSequence(t *testing.T) {
	err := sample().Validate(Intent{Seq: 6, Player: 1, Choices: []int{1}})
	if err == nil || !strings.Contains(err.Error(), "seq") {
		t.Fatalf("err = %v", err)
	}
}

func TestValidateRejectsOutOfRangeAndDuplicates(t *testing.T) {
	if err := sample().Validate(Intent{Seq: 7, Player: 1, Choices: []int{2}}); err == nil {
		t.Error("accepted an out-of-range option index")
	}
	if err := sample().Validate(Intent{Seq: 7, Player: 1, Choices: []int{-1}}); err == nil {
		t.Error("accepted a negative option index")
	}
	multi := sample()
	multi.Min, multi.Max = 0, 2
	if err := multi.Validate(Intent{Seq: 7, Player: 1, Choices: []int{1, 1}}); err == nil {
		t.Error("accepted a duplicate choice")
	}
}

func TestValidateEnforcesArity(t *testing.T) {
	d := sample()
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: nil}); err == nil {
		t.Error("accepted too few choices")
	}
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: []int{0, 1}}); err == nil {
		t.Error("accepted too many choices")
	}
	if err := d.Validate(Intent{Seq: 7, Player: 1, Choices: []int{0}}); err != nil {
		t.Errorf("rejected a valid intent: %v", err)
	}
}

func TestChosenReturnsOptionsInIntentOrder(t *testing.T) {
	d := sample()
	d.Min, d.Max = 0, 2
	got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{1, 0}})
	if len(got) != 2 || got[0].Kind != "pass" || got[1].Kind != "cast" {
		t.Fatalf("Chosen = %+v", got)
	}
}

func TestZeroOptionDecisionAcceptsEmptyIntent(t *testing.T) {
	d := &Decision{Seq: 1, Player: 0, Kind: KBlockers, Min: 0, Max: 0}
	if err := d.Validate(Intent{Seq: 1, Player: 0}); err != nil {
		t.Fatalf("err = %v", err)
	}
	_ = state.PlayerID(0)
}

// TestOptionPlayer0AlwaysEmitted verifies that Player field is emitted even when
// the value is 0, since 0 is a valid seat. Obj field still has omitempty because
// ObjID 0 means "no object".
func TestOptionPlayer0AlwaysEmitted(t *testing.T) {
	opt := Option{
		Index:  0,
		Kind:   "test",
		Label:  "Test",
		Player: 0, // Player 0 is valid
		Obj:    0, // Obj 0 means "no object"
	}
	data, err := json.Marshal(opt)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}
	jsonStr := string(data)

	// The player field should be present
	if !strings.Contains(jsonStr, `"player":0`) {
		t.Errorf("JSON missing 'player':0, got: %s", jsonStr)
	}

	// The obj field should NOT be present (omitempty applies)
	if strings.Contains(jsonStr, `"obj"`) {
		t.Errorf("JSON should not contain 'obj' field when 0, got: %s", jsonStr)
	}
}

// TestChosenBoundsChecking verifies that Chosen returns nil for out-of-range indices
// and does not panic, and follows the all-or-nothing rule.
func TestChosenBoundsChecking(t *testing.T) {
	d := sample()
	d.Min, d.Max = 0, 2

	// Test index equal to len(Options)
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{2}}); got != nil {
		t.Errorf("Expected nil for index 2 (len=%d), got %+v", len(d.Options), got)
	}

	// Test negative index
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{-1}}); got != nil {
		t.Error("Expected nil for negative index, got non-nil")
	}

	// Test large index
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{1000}}); got != nil {
		t.Error("Expected nil for large index, got non-nil")
	}

	// Test mixed valid and invalid indices (all-or-nothing: should return nil)
	if got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{0, 2}}); got != nil {
		t.Error("Expected nil for mixed valid/invalid indices, got non-nil")
	}
}

// TestNewPanicsOnMisindexedOptions is the guard for finding bi: it is a
// convention today that Option.Index equals the option's position, and
// rules/cast.go:570 and rules/mulligan.go:110 are the two sites where a future
// edit breaks that identity silently -- after which Chosen returns a different
// option than the client named, with nothing failing anywhere. New makes the
// broken state unrepresentable: a Decision whose Options[i].Index != i cannot
// be built on the sanctioned path at all, because New panics. This test is the
// named witness for that guard.
func TestNewPanicsOnMisindexedOptions(t *testing.T) {
	mustPanic := func(t *testing.T, opts []Option) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Fatal("New accepted mis-indexed options without panicking")
			}
		}()
		New(1, KPriority, "choose", 1, 1, opts)
	}
	// A single option whose Index drifts off its position zero.
	mustPanic(t, []Option{{Index: 1, Kind: "pass", Label: "Pass", Player: 1}})
	// A later option drifting away from the flowing cast options it follows.
	mustPanic(t, []Option{
		{Index: 0, Kind: "cast", Label: "A", Player: 1},
		{Index: 3, Kind: "cast", Label: "B", Player: 1},
	})
	// The same, with the first option the drifting one.
	mustPanic(t, []Option{
		{Index: 2, Kind: "pass", Label: "Pass", Player: 1},
		{Index: 1, Kind: "cast", Label: "A", Player: 1},
	})
}

// TestNewAcceptsWellFormedOptions pins the normal path through New: a fully
// legal option list still constructs, and the options survive intact, so the
// identity guard did not break the common case.
func TestNewAcceptsWellFormedOptions(t *testing.T) {
	opts := []Option{
		{Index: 0, Kind: "pass", Label: "Pass", Player: 0},
		{Index: 1, Kind: "cast", Label: "Cast", Player: 0},
	}
	d := New(0, KPriority, "choose", 1, 1, opts)
	if d.Player != 0 || d.Kind != KPriority || d.Min != 1 || d.Max != 1 || d.Prompt != "choose" {
		t.Fatalf("New did not preserve its arguments: %+v", d)
	}
	if d.Options[0].Index != 0 || d.Options[1].Index != 1 {
		t.Fatalf("New did not preserve well-formed options: %+v", d.Options)
	}
}

// TestChosenHappyPath verifies that valid intents still return the expected options
// in order, so the bounds guard did not break the normal case.
func TestChosenHappyPath(t *testing.T) {
	d := sample()
	d.Min, d.Max = 0, 2

	// Test with valid choices
	got := d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{1, 0}})
	if got == nil {
		t.Fatal("Expected non-nil for valid intent, got nil")
	}
	if len(got) != 2 {
		t.Fatalf("Expected 2 options, got %d", len(got))
	}
	if got[0].Kind != "pass" || got[0].Index != 1 {
		t.Errorf("First option should be pass (index 1), got %+v", got[0])
	}
	if got[1].Kind != "cast" || got[1].Index != 0 {
		t.Errorf("Second option should be cast (index 0), got %+v", got[1])
	}

	// Test with single valid choice
	got = d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{0}})
	if got == nil {
		t.Fatal("Expected non-nil for valid single choice, got nil")
	}
	if len(got) != 1 || got[0].Kind != "cast" {
		t.Fatalf("Expected single cast option, got %+v", got)
	}

	// Test with empty choices (still valid if Min is 0)
	got = d.Chosen(Intent{Seq: 7, Player: 1, Choices: []int{}})
	if got == nil {
		t.Fatal("Expected non-nil for empty valid choices, got nil")
	}
	if len(got) != 0 {
		t.Fatalf("Expected empty result, got %d options", len(got))
	}
}

// TestBlockOptionAttackerRoundTripsJSON is the Part-B contract: a block
// option's Attacker rides on the wire (the browser's declare-blockers step
// must see which attacker each blocker covers). omitempty keeps the field
// off options that have no attacker, the same zero-value rule as Obj.
func TestBlockOptionAttackerRoundTripsJSON(t *testing.T) {
	block := Option{Index: 0, Kind: "block", Label: "Bear blocks Bear", Obj: 3, Attacker: 9, Player: 1}
	b, err := json.Marshal(block)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"attacker":9`) {
		t.Fatalf("block option JSON missing attacker: %s", s)
	}
	var back Option
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Attacker != 9 {
		t.Fatalf("round-trip lost Attacker: got %d, want 9", back.Attacker)
	}
	plain := Option{Index: 1, Kind: "pass", Label: "Pass", Player: 1}
	pb, err := json.Marshal(plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(pb), "attacker") {
		t.Fatalf("a non-block option leaked an attacker field: %s", pb)
	}
}

// TestAbilityOptionRoundTripsJSON is the Part-A contract for Option.Ability:
// an activated-ability option carries the index into the source face's
// Abilities, so a client can anchor an ability popup to the exact ability
// offered -- kind:"ability" and a label tie it to nothing on the card.
func TestAbilityOptionRoundTripsJSON(t *testing.T) {
	opt := Option{Index: 4, Kind: "ability", Label: "Bear: rumble", Obj: 7, Ability: 3, Player: 1}
	b, err := json.Marshal(opt)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"ability":3`) {
		t.Fatalf("ability option JSON missing ability: %s", s)
	}
	var back Option
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Ability != 3 {
		t.Fatalf("round-trip lost Ability: got %d, want 3", back.Ability)
	}
}

// TestKickedCastModeRoundTripsJSON is the Part-A contract for Option.Mode: a
// kicked/surged/flashback/miracle cast carries its payment kind, so the
// client can distinguish it from an ordinary cast.
func TestKickedCastModeRoundTripsJSON(t *testing.T) {
	opt := Option{Index: 2, Kind: "cast", Label: "Cast Burst Lightning with kicker", Obj: 5, Mode: "kicked", Player: 1}
	b, err := json.Marshal(opt)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"mode":"kicked"`) {
		t.Fatalf("kicked cast JSON missing mode: %s", s)
	}
	var back Option
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Mode != "kicked" {
		t.Fatalf("round-trip lost Mode: got %q, want %q", back.Mode, "kicked")
	}
}

// TestAltCostIndexOptionRoundTripsJSON is the Part-A contract for
// Option.AltCostIndex: a "cast" option paying an AlternativeCost static's
// cost carries its index (i+1 naming alternativeCosts[i]), so a client can
// say which of several costs the option pays.
func TestAltCostIndexOptionRoundTripsJSON(t *testing.T) {
	opt := Option{Index: 3, Kind: "cast", Label: "Cast Tormod's Crypt for its alternative cost", Obj: 9, AltCostIndex: 2, Player: 1}
	b, err := json.Marshal(opt)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"alt_cost_index":2`) {
		t.Fatalf("alternative-cost cast JSON missing alt_cost_index: %s", s)
	}
	var back Option
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.AltCostIndex != 2 {
		t.Fatalf("round-trip lost AltCostIndex: got %d, want 2", back.AltCostIndex)
	}
}

// TestXOptionAmountRoundTripsJSON is the Part-A contract for Option.Amount:
// an "x" choose option carries its X value, which is not its Index (the
// option's position in the list).
func TestXOptionAmountRoundTripsJSON(t *testing.T) {
	opt := Option{Index: 3, Kind: "x", Label: "X = 4", Obj: 5, Amount: 4, Player: 1}
	b, err := json.Marshal(opt)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"amount":4`) {
		t.Fatalf("x option JSON missing amount: %s", s)
	}
	if s := string(b); strings.Contains(s, `"amount":3`) {
		t.Fatalf("x option emitted its Index as its Amount: %s", s)
	}
	var back Option
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Amount != 4 {
		t.Fatalf("round-trip lost Amount: got %d, want 4", back.Amount)
	}
}

// TestDecisionSourceRoundTripsJSON is the Part-A contract for
// Decision.Source: the object a decision resolves for rides on the wire, so
// a prompt can always name its source (survey #18).
func TestDecisionSourceRoundTripsJSON(t *testing.T) {
	d := Decision{Seq: 11, Player: 1, Kind: KChoose, Min: 1, Max: 1, Source: 12,
		Options: []Option{{Index: 0, Kind: "x", Label: "X = 1", Amount: 1}}}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	if s := string(b); !strings.Contains(s, `"source":12`) {
		t.Fatalf("decision JSON missing source: %s", s)
	}
	var back Decision
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.Source != 12 {
		t.Fatalf("round-trip lost Source: got %d, want 12", back.Source)
	}
}

// TestResumeFieldsStayOffTheWire is the guard that keeps the engine's
// mid-resolution bookkeeping private: ResumeKind and ResumeSA are how an
// effects.Host.Ask decision suspends and re-enters the resolution it
// interrupted -- ResumeSA carries a *cards.SA, and the pair is selected by
// the engine only inside rules. Neither may ever reach a client, even with
// non-zero values set, which is also exactly what a later "helpful" change
// exposing one of them would trip.
func TestResumeFieldsStayOffTheWire(t *testing.T) {
	d := Decision{Seq: 9, Player: 0, Kind: KModes, ResumeKind: "modes",
		ResumeSA: &cards.SA{Kind: "SP", API: "Charm", Params: map[string]string{"Choices$": "a,b"}}}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{`"resume_kind":`, `"resume_sa":`} {
		if strings.Contains(string(b), forbidden) {
			t.Fatalf("marshalled decision leaked engine internal %s: %s", forbidden, b)
		}
	}
}

// TestZeroValueOptionsOmitNewFields is the zero-value contract: an option
// with nothing to report (an ordinary cast, a pass, a non-x choose) and a
// decision with no source object marshal exactly as they did before the
// Part-A fields existed, so today's payloads are unchanged for everything
// that has no value to report.
func TestZeroValueOptionsOmitNewFields(t *testing.T) {
	d := Decision{Seq: 4, Player: 2, Kind: KPriority, Min: 1, Max: 1,
		Options: []Option{{Index: 0, Kind: "pass", Label: "Pass", Player: 2},
			{Index: 1, Kind: "cast", Label: "Cast Bear", Obj: 5, Player: 2}}}
	b, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{`"mode":`, `"ability":`, `"alt_cost_index":`, `"amount":`, `"source":`} {
		if strings.Contains(string(b), key) {
			t.Fatalf("a zero-valued option/decision emitted %s: %s", key, b)
		}
	}
}

func TestChooseValidatesLikeAnyDecision(t *testing.T) {
	d := &Decision{Seq: 1, Player: 0, Kind: KChoose, Min: 0, Max: 2, Options: []Option{{Index: 0, Kind: "exile"}, {Index: 1, Kind: "exile"}, {Index: 2, Kind: "exile"}}}
	if err := d.Validate(Intent{Seq: 1, Player: 0, Choices: []int{}}); err != nil {
		t.Fatal(err)
	}
	if err := d.Validate(Intent{Seq: 1, Player: 0, Choices: []int{0, 1, 2}}); err == nil {
		t.Fatal("three of max two accepted")
	}
	if KChoose != "choose" {
		t.Fatal("wire name")
	}
}
