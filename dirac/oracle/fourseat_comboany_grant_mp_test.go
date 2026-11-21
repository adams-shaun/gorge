package oracle

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// Four-seat Commander pods on the edh table whose decks hold Axebane Guardian
// ("{T}: Add X mana in any combination of colors", X the creatures with
// defender you control) beside a granted mana ability: Citanul Hierophants'
// "creatures you control have '{T}: Add {G}'", Rishkar's counters (creatures
// with a +1/+1 counter have the same) and Abundant Growth's any-colour land.
// The table was refused up front (comboanymix); gorge's census prices neither
// the Guardian's production nor the grant, so a play only they pay is searched
// by one prefix search that activates both in battlefield order (rules
// potential_script.go scriptSearch.visit), the printed Combo Any ability and the
// grant as the answers of one "choose a mana ability" ask when the Guardian holds
// the grant (CR 605.3a, 601.2g). The kernel's grant search takes the Guardian as
// a candidate of its own (mana_grantcomboany.go), and gorge's PotentialMana
// prices the Guardian open, so the walk lists every play.

// comboAnyGrantDecks are the pods' decks: the commander, a land, and the cards
// that make the Guardian, a grant and a defender meet on the battlefield early.
var comboAnyGrantDecks = []struct {
	name, commander string
	grant           string
	// reflect: the family's reflected sources (Fellwar Stone, Exotic Orchard,
	// Plaza of Heroes) and a basic land of another colour in each seat
	reflect bool
}{
	{"citanul", "Rishkar, Peema Renegade", "Citanul Hierophants", false},
	{"rishkar", "Rishkar, Peema Renegade", "", false},
	{"abundant", "Rishkar, Peema Renegade", "Abundant Growth", false},
	{"reflected", "Rishkar, Peema Renegade", "", true},
}

// comboAnyGrantPod is a four-seat pod of the deck family d on the edh table.
func comboAnyGrantPod(t *testing.T, i int) (names []string, c *Corpus) {
	t.Helper()
	d := comboAnyGrantDecks[i]
	base, _ := testCorpus(t)
	c = &Corpus{Reg: base.Reg, decks: map[string][]*cards.Card{}}
	files := map[string]deck.File{}
	for s := 0; s < 4; s++ {
		n := fmt.Sprintf("axegrant-%s-%d", d.name, s)
		names = append(names, n)
		es := []deck.Entry{{Name: d.commander, Count: 1}, {Name: "Axebane Guardian", Count: 12},
			{Name: "Wall of Blossoms", Count: 10}, {Name: "Jaddi Offshoot", Count: 6}, {Name: "Towering Titan", Count: 8},
			{Name: "Seedborn Muse", Count: 4}, {Name: "Arboreal Grazer", Count: 4}}
		if d.grant != "" {
			es = append(es, deck.Entry{Name: d.grant, Count: 10})
		}
		if d.reflect {
			es = append(es, deck.Entry{Name: "Fellwar Stone", Count: 8}, deck.Entry{Name: "Exotic Orchard", Count: 6}, deck.Entry{Name: "Plaza of Heroes", Count: 4})
			es = append(es, deck.Entry{Name: [4]string{"Island", "Swamp", "Mountain", "Plains"}[s], Count: 14})
		}
		n0 := 0
		for _, e := range es {
			n0 += e.Count
		}
		es = append(es, deck.Entry{Name: "Forest", Count: 100 - n0})
		files[n] = deck.File{Name: n, Format: "commander", Commander: d.commander, Cards: es}
	}
	c.AddDecks(files)
	return names, c
}

// comboAnyGrantRun plays seeds 1..n of family i and returns the first divergence
// (nil when none) with the Combo Any steps the prefixes played.
func comboAnyGrantRun(t *testing.T, i, n int) (div *Divergence, seed uint64, steps int64) {
	t.Helper()
	set := edhSet(t)
	names, c := comboAnyGrantPod(t, i)
	before := dirac.GrantManaStats.ComboAny.Load()
	for seed = 1; seed <= uint64(n); seed++ {
		m, tbl, err := SetMatchupN(set, c, names, 0)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{MaxDecisions: 4000, MaxTurns: 100}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, AffordableTargetPolicy(RandomPolicy(seed)), rc)
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("%s seed %d: the kernel stalled: %q", comboAnyGrantDecks[i].name, seed, res.Stall)
		}
		if res.Div != nil {
			return res.Div, seed, dirac.GrantManaStats.ComboAny.Load() - before
		}
	}
	return nil, 0, dirac.GrantManaStats.ComboAny.Load() - before
}

// TestFourSeatComboAnyBesideGrantLockstep: each family plays its pods with no
// divergence and no kernel stall, and the prefixes activated the Guardian beside
// the grant (dirac.GrantManaStats.ComboAny), so the interaction was reached.
func TestFourSeatComboAnyBesideGrantLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	for i, d := range comboAnyGrantDecks {
		div, seed, steps := comboAnyGrantRun(t, i, 6)
		if div != nil {
			t.Fatalf("%s seed %d: divergence at %d: %s %s %v", d.name, seed, div.At, div.What, div.Field, div.Diff)
		}
		t.Logf("%s: %d Combo Any steps beside a grant", d.name, steps)
		if steps == 0 {
			t.Errorf("%s: no prefix activated the Guardian beside a grant: the interaction went unexercised", d.name)
		}
	}
}

// TestFourSeatComboAnyBesideGrantMutation: leaving the Guardian out of the
// grant search's candidates (dirac.DebugComboAnyGrantNoGap), or reading no
// reflected colour for the reflected family, must diverge in every family, which shows each pod reaches a play only the Guardian and the grant pay.
func TestFourSeatComboAnyBesideGrantMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	defer func() { dirac.DebugComboAnyGrantNoGap = false; dirac.SetDebugReflMutate(0) }()
	for i, d := range comboAnyGrantDecks {
		// the grant families leave the Guardian out of the grant search; the
		// reflected one reads no reflected colour (mana_reflected.go)
		dirac.DebugComboAnyGrantNoGap = !d.reflect
		if d.reflect {
			dirac.SetDebugReflMutate(1)
		} else {
			dirac.SetDebugReflMutate(0)
		}
		div, seed, _ := comboAnyGrantRun(t, i, 6)
		if div == nil {
			t.Errorf("%s: the mutation did not diverge in 6 seeds", d.name)
			continue
		}
		t.Logf("%s: diverged at seed %d decision %d: %s %s", d.name, seed, div.At, div.What, div.Field)
	}
}
