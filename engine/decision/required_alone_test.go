package decision

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// aloneAttackers is an attackers ask whose first option is a required creature
// that can't attack alone; partners are further creatures, unrestricted.
func aloneAttackers(partners int) *Decision {
	opts := []Option{{Index: 0, Kind: "attacker", Obj: 1, Required: true, AloneRestricted: true}}
	for i := 0; i < partners; i++ {
		opts = append(opts, Option{Index: i + 1, Kind: "attacker", Obj: state.ObjID(i + 2)})
	}
	return &Decision{Kind: KAttackers, Min: 0, Max: len(opts), Options: opts}
}

// TestFitRequiredBringsAPartnerForARequiredLoneAttacker pins CR 508.1d over CR
// 506.5: a requirement is obeyed whenever some legal declaration obeys it, and
// a creature that can't attack alone can attack beside another. Dropping it
// instead leaves an answer that fails the quota.
func TestFitRequiredBringsAPartnerForARequiredLoneAttacker(t *testing.T) {
	d := aloneAttackers(2)
	if got := d.RequiredQuota(); got != 1 {
		t.Fatalf("quota %d, want 1", got)
	}
	for _, pref := range [][]int{nil, {}, {0}, {2}} {
		got := d.FitAlone(d.FitRequired(pref))
		if !slices.Contains(got, 0) || !d.AloneLegal(got) || d.RequiredChosen(got) != 1 {
			t.Fatalf("FitRequired(%v) = %v, want the required creature beside a partner", pref, got)
		}
		if err := d.Validate(Intent{Seq: d.Seq, Player: d.Player, Choices: got}); err != nil {
			t.Fatalf("FitRequired(%v) = %v: %v", pref, got, err)
		}
	}
}

// TestRequiredLoneAttackerWithoutAPartnerIsNotRequired pins the other half of
// CR 508.1d: with no legal partner no declaration can obey the requirement
// without breaking the restriction, so declaring nothing is the legal answer.
func TestRequiredLoneAttackerWithoutAPartnerIsNotRequired(t *testing.T) {
	cases := map[string]*Decision{
		"no other creature": aloneAttackers(0),
	}
	capped := aloneAttackers(1)
	capped.Max = 1
	cases["a one-attacker ceiling"] = capped
	priced := aloneAttackers(1)
	priced.Options[1].Value = 3
	priced.MaxSum = 2
	cases["a partner past the budget"] = priced
	second := aloneAttackers(0)
	second.Options = append(second.Options, Option{Index: 1, Kind: "attacker", Obj: 1, Player: 1, Required: true, AloneRestricted: true})
	second.Max = 2
	cases["the same creature against another defender"] = second
	for name, d := range cases {
		if got := d.RequiredQuota(); got != 0 {
			t.Errorf("%s: quota %d, want 0", name, got)
		}
		if got := d.FitAlone(d.FitRequired(nil)); len(got) != 0 {
			t.Errorf("%s: answer %v, want none", name, got)
		}
	}
}
