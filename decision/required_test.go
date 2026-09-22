package decision

import (
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// reviewBoard is the attackprop1 review's reachable shape: two required
// creatures, each offered at a {2} defender and a free one, under a {3}
// budget.
func reviewBoard() *Decision {
	return &Decision{Kind: KAttackers, Min: 0, Max: 4, MaxSum: 3, Options: []Option{
		{Index: 0, Kind: "attacker", Obj: 1, Player: 0, Value: 2, Required: true},
		{Index: 1, Kind: "attacker", Obj: 2, Player: 0, Value: 2, Required: true},
		{Index: 2, Kind: "attacker", Obj: 1, Player: 2, Required: true},
		{Index: 3, Kind: "attacker", Obj: 2, Player: 2, Required: true},
	}}
}

// TestRequiredQuotaCountsCheapestPairs: the quota is how many required Objs
// the budget and Max can carry, cheapest pair per Obj first.
func TestRequiredQuotaCountsCheapestPairs(t *testing.T) {
	d := reviewBoard()
	if q := d.RequiredQuota(); q != 2 {
		t.Fatalf("quota = %d, want 2 (both free pairs fit)", q)
	}
	// Only the dear pairs: one fits the {3} budget, two do not.
	dear := &Decision{Max: 4, MaxSum: 3, Options: d.Options[:2]}
	if q := dear.RequiredQuota(); q != 1 {
		t.Fatalf("dear-only quota = %d, want 1", q)
	}
	// A Max ceiling bounds it too.
	capped := reviewBoard()
	capped.Max = 1
	if q := capped.RequiredQuota(); q != 1 {
		t.Fatalf("Max-1 quota = %d, want 1", q)
	}
	// No budget: every required Obj.
	free := reviewBoard()
	free.MaxSum = 0
	if q := free.RequiredQuota(); q != 2 {
		t.Fatalf("budget-free quota = %d, want 2", q)
	}
}

// TestFitRequiredRepairsTheReviewAnswer: the bot's price-blind preference
// (both creatures at the dear defender, {4} > {3}) is rebuilt into an answer
// that meets the quota, fits the budget, and keeps the preferred defender for
// the one creature the budget can carry.
func TestFitRequiredRepairsTheReviewAnswer(t *testing.T) {
	d := reviewBoard()
	got := d.FitRequired([]int{0, 1})
	if want := []int{0, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("FitRequired = %v, want %v", got, want)
	}
	if err := d.Validate(Intent{Choices: got}); err != nil {
		t.Fatalf("repaired answer fails Validate: %v", err)
	}
	if n := d.RequiredChosen(got); n < d.RequiredQuota() {
		t.Fatalf("repaired answer covers %d required Objs, quota %d", n, d.RequiredQuota())
	}
}

// TestFitRequiredLeavesSatisfiedAnswersUntouched: an answer already within
// Max, the budget and the quota is returned as-is (same order), so every
// budget-free, requirement-satisfied answer is byte-identical.
func TestFitRequiredLeavesSatisfiedAnswersUntouched(t *testing.T) {
	d := &Decision{Max: 3, Options: []Option{
		{Index: 0, Obj: 1}, {Index: 1, Obj: 2, Required: true}, {Index: 2, Obj: 3},
	}}
	in := []int{2, 1, 0}
	if got := d.FitRequired(in); !reflect.DeepEqual(got, in) {
		t.Fatalf("FitRequired = %v, want %v unchanged", got, in)
	}
}

// TestFitRequiredAlwaysMeetsItsOwnQuota is the class guard: over many
// random decisions (required and free Objs, several priced pairs each, random
// budgets and ceilings) and random preferred answers, FitRequired's output
// always passes Validate AND covers RequiredQuota distinct required Objs with
// at most one option per Obj -- the two checks the engine applies. An
// answer built by the shared rule can never be one the engine rejects.
func TestFitRequiredAlwaysMeetsItsOwnQuota(t *testing.T) {
	r := rand.New(rand.NewPCG(508, 1))
	for iter := 0; iter < 5000; iter++ {
		d := &Decision{Kind: KAttackers, Max: r.IntN(5), MaxSum: r.IntN(7)}
		nObj := 1 + r.IntN(4)
		nDef := 1 + r.IntN(3)
		for obj := 1; obj <= nObj; obj++ {
			req := r.IntN(2) == 0
			for def := 0; def < nDef; def++ {
				v := 0
				if d.MaxSum > 0 {
					v = r.IntN(4)
				}
				d.Options = append(d.Options, Option{Index: len(d.Options), Kind: "attacker",
					Obj: state.ObjID(obj), Player: state.PlayerID(def), Value: v, Required: req})
			}
		}
		var pref []int
		for i := range d.Options {
			if r.IntN(2) == 0 {
				pref = append(pref, i)
			}
		}
		r.Shuffle(len(pref), func(i, j int) { pref[i], pref[j] = pref[j], pref[i] })
		// A preference may name one Obj twice (a client bug); FitRequired keeps
		// what is already valid, so only check one-per-Obj when it rebuilt.
		got := d.FitRequired(pref)
		if err := d.Validate(Intent{Choices: got}); err != nil {
			t.Fatalf("iter %d: %+v pref %v -> %v fails Validate: %v", iter, d, pref, got, err)
		}
		if n, q := d.RequiredChosen(got), d.RequiredQuota(); n < q {
			t.Fatalf("iter %d: %+v pref %v -> %v covers %d required Objs, quota %d", iter, d, pref, got, n, q)
		}
		if reflect.DeepEqual(got, pref) {
			continue
		}
		objs := map[state.ObjID]bool{}
		for _, c := range got {
			if objs[d.Options[c].Obj] {
				t.Fatalf("iter %d: rebuilt %v names Obj %d twice", iter, got, d.Options[c].Obj)
			}
			objs[d.Options[c].Obj] = true
		}
	}
}
