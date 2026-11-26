package kgen

import "testing"

// The bot-owned pins: the total tables a ticket no longer carries (brief
// B2, 11-26). TestRemspanRiseWarnsOutsideStrict holds the remspan census's
// two modes apart on table pairs; TestFallExplainedByNewRefusals holds
// judgePin's fall explanation apart on in-memory inputs.

// TestRemspanRiseWarnsOutsideStrict drives remspanJudge on a table pair: a
// rise (and a fall) passes outside DIRAC_PIN_STRICT=1 and warns, strict fails
// on any move, equal passes both.
func TestRemspanRiseWarnsOutsideStrict(t *testing.T) {
	want := map[string]int{"imp.cards": 5, "rem.cards": 10}
	for _, c := range []struct {
		name   string
		got    map[string]int
		strict bool
		pass   bool
		warns  bool // whether there is a move to report (the caller warns, or errors under strict)
	}{
		{"equal", want, false, true, false},
		{"rise, non-strict warns", map[string]int{"imp.cards": 5, "rem.cards": 11}, false, true, true},
		{"rise, strict fails", map[string]int{"imp.cards": 5, "rem.cards": 11}, true, false, true},
		{"fall, non-strict warns", map[string]int{"imp.cards": 5, "rem.cards": 9}, false, true, true},
		{"fall, strict fails", map[string]int{"imp.cards": 5, "rem.cards": 9}, true, false, true},
		{"new bucket, non-strict warns", map[string]int{"imp.cards": 5, "rem.cards": 10, "rem.leaves": 2}, false, true, true},
		{"new bucket, strict fails", map[string]int{"imp.cards": 5, "rem.cards": 10, "rem.leaves": 2}, true, false, true},
		{"vanished bucket, non-strict warns", map[string]int{"imp.cards": 5}, false, true, true},
		{"vanished bucket, strict fails", map[string]int{"imp.cards": 5}, true, false, true},
	} {
		moved, fail := remspanJudge(want, c.got, c.strict)
		if fail == c.pass {
			t.Errorf("%s: fail = %v, want pass %v", c.name, fail, c.pass)
		}
		if (len(moved) > 0) != c.warns {
			t.Errorf("%s: moved = %q, want warn %v", c.name, moved, c.warns)
		}
	}
}

// TestFallExplainedByNewRefusals drives judgePin on in-memory inputs: a fall
// of 2 with 2 newly refused cards of the list passes outside strict, with 1 it
// fails, and strict fails on any move.
func TestFallExplainedByNewRefusals(t *testing.T) {
	pin := [2]int{100, 120}
	for _, c := range []struct {
		name       string
		got        [2]int
		newRefused []string
		strict     bool
		pass       bool
	}{
		{"equal", pin, nil, false, true},
		{"fall of 2, two new refusals", [2]int{98, 120}, []string{"A", "B"}, false, true},
		{"fall of 2, one new refusal", [2]int{98, 120}, []string{"A"}, false, false},
		{"fall of 2, no new refusals", [2]int{98, 120}, nil, false, false},
		{"fall of 1, two new refusals", [2]int{99, 120}, []string{"A", "B"}, false, true},
		{"fall of 3, two new refusals", [2]int{97, 120}, []string{"A", "B"}, false, false},
		{"rise", [2]int{101, 120}, nil, false, true},
		{"fall of 2, two new refusals, strict", [2]int{98, 120}, []string{"A", "B"}, true, false},
		{"rise, strict", [2]int{101, 120}, nil, true, false},
		{"equal, strict", pin, nil, true, true},
	} {
		if err := judgePin("BLB", pin, c.got, c.strict, c.newRefused); (err == nil) != c.pass {
			t.Errorf("%s: err = %v, want pass %v", c.name, err, c.pass)
		}
	}
}
