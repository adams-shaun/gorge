package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// replayCmd5Repro replays a kfuzz -kernel cmd5 recording (testdata/repro/<name>)
// in lockstep on the cmd5 table, unverified cards allowed as the slice ratchet
// plays them, and checks the recording is the divergence it was kept for: at
// decision at, in field. It returns the replay's result.
func replayCmd5Repro(t *testing.T, name string, at int, field string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Divergence == nil || r.Divergence.At != at || r.Divergence.Field != field {
		t.Fatalf("%s is not the divergence at %d in %s: %+v", name, at, field, r.Divergence)
	}
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table or corpus (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatalf("set cmd5: %v", err)
	}
	set = cardset.AllowUnverified(set)
	m, err := r.Matchup(synthReg(t))
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
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// requireCmd5ReproClean fails unless the replay plays past decision at with no
// divergence and no kernel stall.
func requireCmd5ReproClean(t *testing.T, res Result, at int) {
	t.Helper()
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) {
		t.Fatalf("the kernel stalled: %q", res.Stall)
	}
	if res.Decisions <= at {
		t.Fatalf("the game ended at decision %d, not past %d", res.Decisions, at)
	}
}

// TestCmd5MazemindSacrificedBeforeItsExile: Mazemind Tome's fourth page
// counter puts its state trigger on the stack, and its controller sacrifices
// the Tome to Sai, Master Thopterist in response. The trigger's "exile it. If
// you do, you gain 4 life" finds no Tome on the battlefield, so nothing is
// exiled and no life is gained (CR 603.8, 608.2c). The kernel seeded the
// trigger's remembered set with its source and gained the 4 life anyway.
func TestCmd5MazemindSacrificedBeforeItsExile(t *testing.T) {
	t.Parallel()
	res := replayCmd5Repro(t, "cmd5_mazemind_sacrificed_before_its_exile", 324, "Life[1]")
	requireCmd5ReproClean(t, res, 324)
}

// TestCmd5ChargerPayUnreachableBySignet: Hellkite Charger attacks with every
// land tapped and one Arcane Signet untapped. "You may pay {5}{R}{R}" cannot
// be paid from one mana (CR 118.3), so gorge offers only the decline, as the
// kernel does; gorge's payment bound used to price the Signet's
// commander-identity mana unbounded in every colour and offered the pay.
func TestCmd5ChargerPayUnreachableBySignet(t *testing.T) {
	t.Parallel()
	res := replayCmd5Repro(t, "cmd5_charger_pay_unreachable_by_signet", 497, "options")
	requireCmd5ReproClean(t, res, 497)
}

// TestCmd5HellkiteXCountsSignet: Steel Hellkite's {X} ability is activated
// with an Arcane Signet untapped beside the lands. The Signet makes one mana of
// a colour in its controller's commander's identity (CR 903.4), which pays
// generic {X}, so gorge offers X up to the kernel's bound; its cast window
// used to price the Signet as no mana and offered one less.
func TestCmd5HellkiteXCountsSignet(t *testing.T) {
	t.Parallel()
	res := replayCmd5Repro(t, "cmd5_hellkite_x_counts_signet", 250, "options")
	requireCmd5ReproClean(t, res, 250)
}

// TestCmd5GhaltaTaxNeedsGrantedMana: Ghalta, Primal Hunger waits in the
// command zone with one earlier cast, so it costs {10}{G}{G} less six for
// the creatures' power plus the {2} tax: {6}{G}{G} (CR 601.2f, 903.8). Six
// Forests pay {6}{G}; the eighth mana is Rishkar's granted creature mana,
// which gorge's payment planner never witnesses, so under the autopay
// surface gorge lists no cast. The kernel priced the planner's cost without
// the tax ({4}{G}{G}, which the Forests pay) and offered it.
func TestCmd5GhaltaTaxNeedsGrantedMana(t *testing.T) {
	t.Parallel()
	res := replayCmd5Repro(t, "cmd5_ghalta_tax_needs_granted_mana", 564, "options")
	requireCmd5ReproClean(t, res, 564)
}
