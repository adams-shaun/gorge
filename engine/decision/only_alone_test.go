package decision

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestOnlyAloneRestrictedRepairsTheBotAnswer pins CR 506.5's inverse polarity
// (Master of Cruelties' "CARDNAME can only attack alone.") at the decision
// layer: the bot's two-attacker answer (built as if it ignored the fact) runs
// through the shared repair and then through the wire validator, and passes
// only as the repaired sole-attacker answer; an unrestricted option serialises
// byte-identically to before.
func TestOnlyAloneRestrictedRepairsTheBotAnswer(t *testing.T) {
	d := Decision{
		Kind: KAttackers,
		Options: []Option{
			{Index: 0, Kind: "attacker", Obj: 7, OnlyAloneRestricted: true},
			{Index: 1, Kind: "attacker", Obj: 8},
		},
	}
	// The wire validator refuses the pair answer the bot would build if it
	// ignored the published fact.
	if d.AloneLegal([]int{0, 1}) {
		t.Fatal("only-attack-alone option accepted beside another attacker")
	}
	if err := d.validateAlone([]int{0, 1}); err == nil {
		t.Fatal("validateAlone accepted the pair answer")
	} else if !strings.Contains(err.Error(), "can only attack alone") {
		t.Errorf("validateAlone message: %v", err)
	}
	// The shared repair (botpolicy.Clamp runs FitAlone) drops the restricted
	// creature, and the repaired answer passes the same validator.
	repaired := d.FitAlone([]int{0, 1})
	if len(repaired) != 1 || repaired[0] != 1 {
		t.Fatalf("FitAlone({0, 1}) = %v, want {1}", repaired)
	}
	if err := d.validateAlone(repaired); err != nil {
		t.Fatalf("repaired answer refused: %v", err)
	}
	if !d.AloneLegal([]int{0}) {
		t.Fatal("the lone only-attack-alone declaration must be legal")
	}
	// The lone (legal) answer survives FitAlone unchanged.
	if got := d.FitAlone([]int{0}); len(got) != 1 || got[0] != 0 {
		t.Fatalf("FitAlone({0}) = %v, want {0}", got)
	}
	// A blockers decision ignores the attacker-only flag.
	blk := Decision{Kind: KBlockers, Options: []Option{
		{Index: 0, Kind: "block", Obj: 7, OnlyAloneRestricted: true},
		{Index: 1, Kind: "block", Obj: 8, Attacker: 9},
	}}
	if !blk.AloneLegal([]int{0, 1}) {
		t.Fatal("OnlyAloneRestricted leaked into the blockers decision")
	}
	// Wire shape: the fact serialises, and an ordinary option stays silent.
	restricted, err := json.Marshal(Option{Kind: "attacker", OnlyAloneRestricted: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(restricted), `"only_alone_restricted":true`) {
		t.Errorf("restricted option: %s", restricted)
	}
	ordinary, err := json.Marshal(Option{Kind: "attacker"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(ordinary), "only_alone_restricted") {
		t.Errorf("ordinary option: %s", ordinary)
	}
}

// TestOnlyAloneRestrictedMixedAnswerRepairs pins the mixed case: a can't-
// attack-alone creature (AloneRestricted) beside an only-attack-alone creature
// (OnlyAloneRestricted). Dropping the latter leaves the former alone, so the
// repair must drop both; the repaired answer passes the wire validator.
func TestOnlyAloneRestrictedMixedAnswerRepairs(t *testing.T) {
	d := Decision{
		Kind: KAttackers,
		Options: []Option{
			{Index: 0, Kind: "attacker", Obj: 7, AloneRestricted: true},
			{Index: 1, Kind: "attacker", Obj: 8, OnlyAloneRestricted: true},
		},
	}
	if err := d.validateAlone([]int{0, 1}); err == nil {
		t.Fatal("validateAlone accepted the mixed pair")
	}
	got := d.FitAlone([]int{0, 1})
	if len(got) != 0 {
		t.Fatalf("FitAlone({0, 1}) = %v, want empty", got)
	}
	if err := d.validateAlone(got); err != nil {
		t.Fatalf("repaired answer refused: %v", err)
	}
	// With a third, unrestricted attacker the only-alone one drops and the rest stand.
	d.Options = append(d.Options, Option{Index: 2, Kind: "attacker", Obj: 9})
	got = d.FitAlone([]int{0, 1, 2})
	if err := d.validateAlone(got); err != nil {
		t.Fatalf("repaired three-attacker answer %v refused: %v", got, err)
	}
}
