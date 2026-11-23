package decision

import (
	"slices"
	"testing"
)

// TestRequiredBoundAttackerWithoutAPartnerLapses pins CR 508.1d for the
// declaration rules beyond can't-attack-alone: a goaded Okk-style attacker with
// no qualifying partner, and a goaded Master-of-Cruelties-style attacker beside
// another goaded creature, still have an answer Validate accepts and the bot's
// Clamp path (FitRequired then FitAlone) produces.
func TestRequiredBoundAttackerWithoutAPartnerLapses(t *testing.T) {
	okk := &Decision{Kind: KAttackers, Max: 2, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Required: true, CompanionNeed: 1, CompanionFrom: []int{}},
		{Index: 1, Kind: "attacker", Obj: 2},
	}}
	okkFar := &Decision{Kind: KAttackers, Max: 1, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Required: true, CompanionNeed: 1, CompanionFrom: []int{1}},
		{Index: 1, Kind: "attacker", Obj: 2},
	}}
	master := &Decision{Kind: KAttackers, Max: 2, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Required: true, AloneOnly: true},
		{Index: 1, Kind: "attacker", Obj: 2, Required: true},
	}}
	masterOnly := &Decision{Kind: KAttackers, Max: 1, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Required: true, AloneOnly: true},
	}}
	for name, tc := range map[string]struct {
		d    *Decision
		want int // RequiredQuota
	}{
		"Okk, no qualifying partner":        {okk, 0},
		"Okk, partner past the Max ceiling": {okkFar, 0},
		"Master beside a goaded creature":   {master, 1},
		"Master alone":                      {masterOnly, 1},
	} {
		d := tc.d
		if got := d.RequiredQuota(); got != tc.want {
			t.Errorf("%s: quota %d, want %d", name, got, tc.want)
		}
		for _, pref := range [][]int{nil, {0}, {0, 1}} {
			if slices.ContainsFunc(pref, func(c int) bool { return c >= len(d.Options) }) {
				continue
			}
			got := d.FitAlone(d.FitRequired(pref))
			if err := d.Validate(Intent{Seq: d.Seq, Player: d.Player, Choices: got}); err != nil {
				t.Errorf("%s: Clamp(%v) = %v: %v", name, pref, got, err)
			}
		}
	}
	// The goaded second creature is the answer when Master can't join it.
	if got := master.FitAlone(master.FitRequired(nil)); !slices.Equal(got, []int{1}) {
		t.Errorf("Master beside a goaded creature: answer %v, want [1]", got)
	}
}

// TestRequiredCompanionAttackerGetsAQualifyingPartner is the other half: a
// goaded Okk-style attacker with a qualifying creature available is declared
// beside it, never beside a creature it doesn't count.
func TestRequiredCompanionAttackerGetsAQualifyingPartner(t *testing.T) {
	d := &Decision{Kind: KAttackers, Max: 3, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Required: true, CompanionNeed: 1, CompanionFrom: []int{2}},
		{Index: 1, Kind: "attacker", Obj: 2, Value: 0},
		{Index: 2, Kind: "attacker", Obj: 3, Value: 1},
	}}
	got := d.FitAlone(d.FitRequired(nil))
	if !slices.Contains(got, 0) || !slices.Contains(got, 2) {
		t.Fatalf("answer %v, want Okk beside the qualifying creature", got)
	}
	if err := d.Validate(Intent{Seq: d.Seq, Player: d.Player, Choices: got}); err != nil {
		t.Fatal(err)
	}
}

// TestBlockSearchMemoKeepsCompanionBlockerIdentity: Okk-style blockers need a
// qualifying blocker beside them, so a team's legality depends on WHICH
// blockers it holds, not how many block each attacker. The memoised team search
// must still publish a team that obeys the most requirements.
func TestBlockSearchMemoKeepsCompanionBlockerIdentity(t *testing.T) {
	mk := func() *Decision {
		// Attackers 10 and 11 each must be blocked. Blockers 1 (Okk-like, needs
		// blocker 3 to also block) and 2 (free) are interchangeable by count;
		// blocker 3 is the only qualifying companion and can block only 11.
		opts := []Option{
			{Index: 0, Kind: "block", Obj: 1, Attacker: 10, BlockMust: true, CompanionNeed: 1, CompanionFrom: []int{3}},
			{Index: 1, Kind: "block", Obj: 1, Attacker: 11, BlockMust: true, CompanionNeed: 1, CompanionFrom: []int{3}},
			{Index: 2, Kind: "block", Obj: 2, Attacker: 10, BlockMust: true},
			{Index: 3, Kind: "block", Obj: 3, Attacker: 11, BlockMust: true},
		}
		return &Decision{Kind: KBlockers, Max: 4, Options: opts}
	}
	d := mk()
	if d.blockSearchMemoizable(-1) {
		// memoisation must be off: the team's legality reads blocker identity.
		t.Fatal("blockSearchMemoizable reported true with a companion-bound blocker")
	}
	got := d.FitRequired(nil)
	if err := d.Validate(Intent{Seq: d.Seq, Player: d.Player, Choices: got}); err != nil {
		t.Fatalf("FitRequired = %v: %v", got, err)
	}
	if n := d.RequiredChosen(got); n != 3 {
		t.Fatalf("team %v obeys %d requirements, want 3 (1 and 2 on 10, 3 on 11)", got, n)
	}
}
