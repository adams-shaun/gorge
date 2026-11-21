package oracle

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestMBCSkyshroudVampireCastOfferPinned pins the divergence this recording
// captured at main 9784c19a51 (tail-73-c000 pair 27, seed 367018): at
// decision 275 gorge offers cast(#68{Skyshroud Vampire},-,0) where the
// kernel does not. gorge is right per CR 601.2g-h: p1's only fifth mana is
// Shadowblood Ridge's "{1}, {T}: Add {B}{R}", a once-only paid mana ability
// whose fee the census sources fund while it taps (Ichorid {3}{B} and
// Oathsworn Knight {1}{B}{B} are offered by both engines; the fifth mana is
// exactly the Ridge's).
//
// The kernel prices the Ridge (kop.FilterManaAb models Skycloud Expanse's
// shape) and its filter route does offer the cast on a filter-only board;
// the game's board also holds Untaidake, the Cloud Keeper ({T}, Pay 2 life:
// Add {C}{C}, spend only on legendary spells), a restricted source gap and
// a life gap. Beside it the filter route is gated off
// (mana_mixgaps.go mixBoard excludes scriptRestr gaps; the any-route's
// search, mana_window.go scriptDeadEndAny, lists no filter slot), so no
// kernel route searches the Ridge and the gaps walk prices the cast without
// its units. gorge's one prefix search (potentialPrefixScript) activates
// every census gap, funds the Ridge's {1} from a floated Forest, and offers.
//
// The fix (one search over every census gap, filters included, funding each
// fee from the floated mana or a census source) flips this test to the
// plain lockstep replay: replayReproValue over the same recording.
func TestMBCSkyshroudVampireCastOfferPinned(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "tail-73")
	r, err := ReadRepro(filepath.Join("testdata", "repro", "skyshroud_vampire_priority_cast_offer.repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.Config.GorgeAutopaySurface = false
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	pol := &ScriptPolicy{Actions: r.Actions}
	res := Lockstep(m, r.Seed, k, pol, rc)
	// The precondition: the recorded prefix really was played, so the
	// divergence below is the engines' own disagreement at decision 275, not
	// a script that silently stopped early.
	if pol.next != len(r.Actions) || pol.Skipped != 0 {
		t.Fatalf("precondition: script served %d of %d actions (%d skipped)", pol.next, len(r.Actions), pol.Skipped)
	}
	if res.Decisions < 275 {
		t.Fatalf("precondition: replay stopped at %d decisions, before the divergence at 275", res.Decisions)
	}
	if res.Div == nil {
		t.Fatal("the kernel now offers the Skyshroud Vampire cast: flip this pin to replayReproValue over the same recording (the fix's lockstep test)")
	}
	div := res.Div
	if div.At != 275 || div.What != "options" || div.Field != "options" || div.Lead != "gorge" {
		t.Fatalf("divergence moved: at %d what %s field %s lead %s (want 275/options/options/gorge): %v", div.At, div.What, div.Field, div.Lead, div.Diff)
	}
	// Options render as cast(#<obj>,-,0): obj #68 is the Skyshroud Vampire in
	// hand ( gorge's one extra option is exactly its cast; the two casts both
	// engines price, Ichorid {3}{B} and Oathsworn Knight {1}{B}{B}, are the
	// common cast entries).
	refCast := slices.Contains(div.RefOptions, "cast(#68,-,0)")
	kerCast := slices.Contains(div.KerOptions, "cast(#68,-,0)")
	if !refCast || kerCast {
		t.Fatalf("divergence is not the missing kernel cast: gorge offers it = %v, kernel offers it = %v\nref %v\nker %v", refCast, kerCast, div.RefOptions, div.KerOptions)
	}
	// Everything else agreed.
	ker := slices.Clone(div.KerOptions)
	for _, o := range div.RefOptions {
		if o == "cast(#68,-,0)" {
			continue
		}
		if !slices.Contains(ker, o) {
			t.Fatalf("gorge option %q is not the kernel's: the divergence is wider than the missing cast\nref %v\nker %v", o, div.RefOptions, div.KerOptions)
		}
	}
}
