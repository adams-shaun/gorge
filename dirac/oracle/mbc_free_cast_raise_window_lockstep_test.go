package oracle

import "testing"

// A may-play free cast (Urza, Lord High Artificer's "{5}: ... you may play
// that card without paying its mana cost") still owes a target-dependent
// raise (Kasmina, Enigmatic Mentor's "spells your opponents cast that target
// a creature or planeswalker you control cost {2} more"). The target menu
// prices each candidate with the raise and, when the floating pool falls
// short, asks the CR 601.2g window; a source the census cannot price
// (Urza's tap-another-artifact ability, with no untapped artifact) leaves it
// incomplete, and the menu keeps its candidates (castWindowCensusComplete).
// The kernel's free-cast arm once skipped that window ask and withheld every
// taxed candidate (dr-mbc-baaa pair 111, decision 883).
func TestMBCAngelicPageReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_raise_window")
}

// precondition: the recording's pool is the one the evidence loop dealt, so
// the game above is the pair the ticket named.
func TestFreeCastRaiseWindowReproPool(t *testing.T) {
	t.Parallel()
	r, err := ReadRepro("testdata/repro/free_cast_raise_window.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "dr-mbc-baaa" {
		t.Fatalf("precondition: repro pool %q, want dr-mbc-baaa", r.Pool)
	}
}
