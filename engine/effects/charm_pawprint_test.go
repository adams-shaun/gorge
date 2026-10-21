package effects

// CharmPawprints' presence and pricing semantics (CR 700.2i, CR 107.18):
// ok is FALSE only when the root carries no Pawprint$ at all (no budget
// exists, the ask stays unbudgeted) or the present value is unparseable
// (loud; the budget is never silently dropped); every PRESENT parseable
// value -- a zero included -- is a real budget the ask sites publish, as
// Decision.Budgeted with MaxSum, so a root Pawprint$ 0 fences the modes at
// 0 {P} instead of reading absent. A mode body without its own Pawprint$ is
// free (the Forge default 0); an unparseable mode cost is loud and priced 0.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

func pawprintCharmSA(pawprint string, present bool) *cards.SA {
	params := map[string]string{"Choices": "mA,mB"}
	if present {
		params["Pawprint"] = pawprint
	}
	return &cards.SA{API: "Charm", Params: params}
}

func TestCharmPawprintPresenceAndPricing(t *testing.T) {
	t.Parallel()
	svars := map[string]string{
		"mA": "DB$ Draw | NumCards$ 1 | Pawprint$ 1",
		"mB": "DB$ Draw | NumCards$ 2 | Pawprint$ 0",
	}
	names := []string{"mA", "mB"}

	if budget, values, ok := CharmPawprints(nil, 0, pawprintCharmSA("", false), svars, names); ok {
		t.Fatalf("absent root Pawprint$: budget %d values %v ok -- want no budget", budget, values)
	}
	if budget, values, ok := CharmPawprints(nil, 0, pawprintCharmSA("0", true), svars, names); !ok {
		t.Fatal("present root Pawprint$ 0: ok=false -- a present budget must never read absent")
	} else if budget != 0 {
		t.Fatalf("present root Pawprint$ 0: budget = %d, want 0", budget)
	} else if len(values) != 2 || values[0] != 1 || values[1] != 0 {
		t.Fatalf("present root Pawprint$ 0: values = %v, want [1 0]", values)
	}
	if budget, values, ok := CharmPawprints(nil, 0, pawprintCharmSA("5", true), svars, names); !ok || budget != 5 {
		t.Fatalf("present root Pawprint$ 5: budget %d ok %v, want 5 true", budget, ok)
	} else if len(values) != 2 || values[0] != 1 || values[1] != 0 {
		t.Fatalf("present root Pawprint$ 5: values = %v, want [1 0]", values)
	}
	if budget, values, ok := CharmPawprints(nil, 0, pawprintCharmSA("notanumber", true), svars, names); ok {
		t.Fatalf("unparseable root Pawprint$: budget %d values %v ok -- want no budget", budget, values)
	}
	// An unparseable MODE cost is loud (the nil host skips the Note) and
	// prices the mode 0 rather than fabricating a price.
	svars["mC"] = "DB$ Draw | NumCards$ 3 | Pawprint$ abc"
	if budget, values, ok := CharmPawprints(nil, 0, pawprintCharmSA("5", true), svars, []string{"mA", "mC"}); !ok || budget != 5 {
		t.Fatalf("unparseable mode cost: budget %d ok %v, want 5 true", budget, ok)
	} else if len(values) != 2 || values[0] != 1 || values[1] != 0 {
		t.Fatalf("unparseable mode cost: values = %v, want [1 0]", values)
	}
}
