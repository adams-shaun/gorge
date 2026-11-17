package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const dominateRepro = "dominate_x_bound_target_offered_past_a_dual_land"

// CR 601.2b, 601.2c: Dominate ({X}{1}{U}{U}, target creature with mana value X
// or less) is castable only while some X the sources can pay leaves it a legal
// target. The recorded game holds Island, Island and Witherbloom Campus untapped
// with only a mana value 1 and a mana value 3 creature in play: three mana, so X
// is 0 and no creature qualifies. gorge's X-bound target sweep prices X = 1
// against the potential-mana over-bound (the Campus counted in both of its
// colours) and offers the cast; its payment planner then floats only
// {U}{U}{G} and the cast is gone (tail-66 pair 1). Bounding the sweep by the
// planner is gorge's hot legal walk plus a dirac mirror past the 200-line
// refusal line, so the card is refused by name and the recorded game still shows
// the divergence with it implicated.
func TestMBCDominateReprosLockstep(t *testing.T) {
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	for i := range entries {
		e := &entries[i]
		if e.Card != "Dominate" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != dominateRepro {
			t.Fatalf("Dominate's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, dominateRepro)
		}
		checkRefusedRepro(t, e)
		return
	}
	t.Fatal("Dominate has no dirac/support/refused entry: its X-bound target is offered at an over-counted X")
}

// CR 700.8: Cascade Seer (a Wizard) beside Game-Trail Changeling (every
// creature type) is a party of two, so its scry 2 orders two cards. The
// matchup's subtype table kept none of the four party roles (nothing named
// them in a filter), so the kernel saw the Seer fill no role and counted a
// party of one, offering one card to arrange where gorge offered two
// (decision 203, pair 350 of tail-39).
func TestMBCPartyRolesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "party_roles_kept_in_matchup")
}
