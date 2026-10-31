package mapping_test

// The named-option catch-alls must not
// claim an ask whose whole-answer rule their one-pick candidates cannot
// express (MinSum, MaxSum/Budgeted, AllowNone); those keep the old unmapped
// fallback so the native bot answers them.

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
)

func TestNamedOptionCatchAllsSkipConstrainedAsks(t *testing.T) {
	env := namedOptionEnv(t)
	cases := []struct {
		name string
		d    *decision.Decision
		want string
	}{
		{
			// The teamwork ask (cast_begin.go): distinct objects and a MinSum
			// threshold. Its single picks cannot reach the floor.
			name: "MinSum",
			d: &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 2,
				MinSum: 5,
				Options: []decision.Option{
					{Index: 0, Kind: "teamwork", Obj: 7, Value: 2},
					{Index: 1, Kind: "teamwork", Obj: 8, Value: 3},
				}},
			want: "unmapped:choose/teamwork",
		},
		{
			// The charm/pawprint KChoose (charm.go): a MaxSum budget over the
			// modes, including the present-zero form.
			name: "budgeted modes",
			d: &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 2, Max: 2,
				MaxSum: 1, Budgeted: true,
				Options: []decision.Option{
					{Index: 0, Kind: "mode", Value: 1},
					{Index: 1, Kind: "mode", Value: 1},
				}},
			want: "unmapped:choose/mode",
		},
		{
			name: "second budget",
			d: &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 1,
				MaxSum2: 1, Budgeted2: true,
				Options: []decision.Option{
					{Index: 0, Kind: "mode", Value: 1},
					{Index: 1, Kind: "mode", Value: 2},
				}},
			want: "unmapped:choose/mode",
		},
		{
			// A Min of 0 makes the empty answer legal (soulbond's "you may
			// pair", draw up to N, copy-permanent's different names): the
			// single-pick candidates can never decline.
			name: "Min zero",
			d: &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 0, Max: 1,
				Options: []decision.Option{
					{Index: 0, Kind: "permanent", Obj: 7},
					{Index: 1, Kind: "permanent", Obj: 8},
				}},
			want: "unmapped:choose/permanent",
		},
		{
			name: "AllowNone without a sum",
			d: &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 1, Max: 2,
				AllowNone: true,
				Options: []decision.Option{
					{Index: 0, Kind: "primary"},
					{Index: 1, Kind: "bottom"},
				}},
			want: "unmapped:choose/bottom,primary",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mapping.Route(c.d); got != c.want {
				t.Fatalf("Route = %q, want %q", got, c.want)
			}
			if _, err := mapping.Begin(env, c.d); !errors.Is(err, mapping.ErrUnmapped) {
				t.Fatalf("Begin = %v, want ErrUnmapped", err)
			}

			// Precondition: the same option set without the whole-answer rule
			// is exactly what the catch-all would claim. Min 0 is part of that
			// rule, so the plain clone raises it to the smallest non-empty
			// answer the catch-all can commit.
			plain := *c.d
			plain.MinSum, plain.MaxSum, plain.MaxSum2 = 0, 0, 0
			plain.Budgeted, plain.Budgeted2, plain.AllowNone = false, false, false
			if plain.Min < 1 {
				plain.Min = 1
			}
			if got := mapping.Route(&plain); got != "choose/option" && got != "choose/object" {
				t.Fatalf("unconstrained Route = %q, want one of the named-option routes", got)
			}
		})
	}
}

// TestChooseOptionObjectMinExceedsOneUsesGroupRoute pins the second half of the
// guard: choose/object commits exactly one pick, so a Min>1 distinct-object ask
// takes choose/option's group substeps and its commit passes Decision.Validate.
func TestChooseOptionObjectMinExceedsOneUsesGroupRoute(t *testing.T) {
	env := namedOptionEnv(t)
	d := &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 2, Max: 2,
		Options: []decision.Option{
			{Index: 0, Kind: "card", Obj: 7},
			{Index: 1, Kind: "card", Obj: 8},
		}}
	if got := mapping.Route(d); got != "choose/option" {
		t.Fatalf("Route = %q, want choose/option", got)
	}
	tx, err := mapping.Begin(env, d)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Pose(); err != nil {
		t.Fatal(err)
	}
	if _, done, err := tx.Answer(0); err != nil || done {
		t.Fatalf("first pick: done %v, err %v", done, err)
	}
	if _, err := tx.Pose(); err != nil {
		t.Fatal(err)
	}
	commit, done, err := tx.Answer(0)
	if err != nil || !done {
		t.Fatalf("second pick: done %v, err %v", done, err)
	}
	if len(commit) != 1 || d.Validate(commit[0]) != nil {
		t.Fatalf("commit %v does not pass Decision.Validate: %v", commit, d.Validate(commit[0]))
	}
}

// TestNamedOptionMultiPickSkipsConstrainedAsks pins the hardening half of the
// guard: optionGroupTx models only the range and Repeatable rules, so a Min>1
// ask carrying a Group exclusivity/cap, a target-set property rule, a
// same-controller rule or a distinct-type assignment keeps the unmapped
// fallback rather than being offered a commit Decision.Validate rejects.
func TestNamedOptionMultiPickSkipsConstrainedAsks(t *testing.T) {
	env := namedOptionEnv(t)
	base := func() *decision.Decision {
		return &decision.Decision{Player: 0, Kind: decision.KChoose, Min: 2, Max: 2,
			Options: []decision.Option{
				{Index: 0, Kind: "mode"},
				{Index: 1, Kind: "mode"},
				{Index: 2, Kind: "mode"},
			}}
	}
	cases := []struct {
		name      string
		constrain func(*decision.Decision)
	}{
		{
			// Two same-Group options cannot both be picked (Validate's
			// "mutually exclusive" rule); optionGroupTx offers both.
			name: "Group exclusivity",
			constrain: func(d *decision.Decision) {
				d.Options[0].Group, d.Options[1].Group = "a", "a"
			},
		},
		{
			name: "set property rule",
			constrain: func(d *decision.Decision) {
				d.SetPropMode = decision.SetPropShared
			},
		},
		{
			name: "same controller rule",
			constrain: func(d *decision.Decision) {
				d.TargetsWithSameController = true
			},
		},
		{
			name: "distinct type picks",
			constrain: func(d *decision.Decision) {
				d.DistinctTypePicks = true
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := base()
			c.constrain(d)
			if got := mapping.Route(d); got != "unmapped:choose/mode" {
				t.Fatalf("constrained Route = %q, want unmapped:choose/mode", got)
			}
			if _, err := mapping.Begin(env, d); !errors.Is(err, mapping.ErrUnmapped) {
				t.Fatalf("Begin = %v, want ErrUnmapped", err)
			}
			// Precondition: without the constraint the ask is the
			// option-group route's, so the fallback is caused by it.
			if got := mapping.Route(base()); got != "choose/option" {
				t.Fatalf("unconstrained Route = %q, want choose/option", got)
			}
		})
	}
}
