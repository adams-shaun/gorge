package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Commander curses' second leg (TriggeredAttackingPlayer.Opponent+
// controlsCreature.attacking Player.EnchantedBy) and Inertia's
// trigger-player TapOrUntap, on synthetic scripts shaped like the corpus
// carriers. The skip and the arms live in lower_attackleg.go and
// lower_tapuntap.go.

const kccDefEndUntap = "T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ TrigUntap"
const kccDefEndTap = "T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ TrigTapOrUntap"

func TestAttackLegSkipLowering(t *testing.T) {
	// Curse of Bounty's shape: the "you untap" leg lowers, the empty
	// attacking-player leg ("each opponent attacking that player untaps...")
	// adds no instruction — its beneficiary set is empty at every seat
	// count, so the leg is the no-op gorge plays it as.
	why, ab, _ := ktqBuildSV(t,
		"T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | Execute$ TrigUntap | TriggerZones$ Battlefield",
		"SVar:TrigUntap:DB$ UntapAll | Defined$ You | ValidCards$ Permanent.nonLand | SubAbility$ DBUntap",
		"SVar:DBUntap:DB$ UntapAll | Defined$ TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy | ValidCards$ Permanent.nonLand")
	if why != "" {
		t.Fatalf("the empty attacking-player leg refused: %s", why)
	}
	if len(ab.Prog) != 1 || ab.Prog[0].Op != kop.OpUntapAll {
		t.Fatalf("the skipped leg left instructions: %d, first %v", len(ab.Prog), ab.Prog)
	}
	// the same leg on a trigger that is not AttackersDeclared is not a
	// provable no-op and refuses under the ordinary TriggeredAttackingPlayer
	// rule (the referent is only the attacking player there)
	why, _, _ = ktqBuildSV(t, kccDefEndUntap,
		"SVar:TrigUntap:DB$ UntapAll | Defined$ TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy | ValidCards$ Permanent.nonLand")
	if why == "" {
		t.Fatal("the empty leg lowered off an AttackersDeclared trigger")
	}
}

func TestAttackLegSkipAllowlist(t *testing.T) {
	leg := "SVar:DBUntap:DB$ UntapAll | Defined$ TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"
	// a param the allowlist reads (ValidCards$, which scales with the empty
	// player set) skips with the node
	why, ab, _ := ktqBuildSV(t,
		"T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | Execute$ TrigUntap | TriggerZones$ Battlefield",
		"SVar:TrigUntap:DB$ UntapAll | Defined$ You | ValidCards$ Permanent.nonLand | SubAbility$ DBUntap",
		leg+" | ValidCards$ Permanent.nonLand")
	if why != "" || len(ab.Prog) != 1 {
		t.Fatalf("an allowlisted param on the skipped leg: %q, %d instrs", why, len(ab.Prog))
	}
	// RememberTapped$ records the untapped objects — with no players untapped
	// it would record nothing, but the side effect is not proportional to the
	// player set (a later leg may read it), so the node refuses
	why, _, _ = ktqBuildSV(t,
		"T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | Execute$ TrigUntap | TriggerZones$ Battlefield",
		"SVar:TrigUntap:DB$ UntapAll | Defined$ You | ValidCards$ Permanent.nonLand | SubAbility$ DBUntap",
		leg+" | RememberTapped$ True")
	if !strings.Contains(why, "skipped empty attacking-player leg with RememberTapped$") {
		t.Errorf("RememberTapped$ on the skipped leg: refusals %q, want the skip-with-param refusal", why)
	}
}

func TestTapOrUntapTriggerPlayer(t *testing.T) {
	// Curse of Inertia's shape: the target ask goes to the attacking player
	// (TgfTrigChooser) while the tap records the trigger player as its
	// provenance (OpTapOrUntap's W), both read off an AttackersDeclared
	// trigger only.
	why, ab, _ := ktqBuildSV(t,
		"T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | Execute$ TrigTapOrUntap | TriggerZones$ Battlefield",
		"SVar:TrigTapOrUntap:DB$ TapOrUntap | ValidTgts$ Permanent | TargetingPlayer$ TriggeredAttackingPlayer | Tapper$ TriggeredAttackingPlayer")
	if why != "" {
		t.Fatalf("the trigger-player tap-or-untap refused: %s", why)
	}
	if len(ab.Prog) != 1 || ab.Prog[0].Op != kop.OpTapOrUntap {
		t.Fatalf("the tap-or-untap lowered %d instructions: %v", len(ab.Prog), ab.Prog)
	}
	if ab.Prog[0].W != kop.RefTriggerPlayer {
		t.Errorf("Tapper$ did not lower to the trigger player's provenance: W=%d", ab.Prog[0].W)
	}
	if ab.NTargets != 1 || ab.Targets[0].TFlags&kop.TgfTrigChooser == 0 {
		t.Errorf("TargetingPlayer$ did not set the target ask's chooser: NTargets=%d flags=%v",
			ab.NTargets, ab.Targets[0].TFlags)
	}
	// elsewhere the same arms refuse by name
	why, _, _ = ktqBuildSV(t, kccDefEndTap,
		"SVar:TrigTapOrUntap:DB$ TapOrUntap | ValidTgts$ Permanent | TargetingPlayer$ TriggeredAttackingPlayer | Tapper$ TriggeredAttackingPlayer")
	if !strings.Contains(why, "outside an AttackersDeclared trigger") {
		t.Errorf("the arms off an AttackersDeclared trigger: refusals %q, want the named refusal", why)
	}
	// TargetingPlayer$ without a target ask has no ask to move
	why, _, _ = ktqBuildSV(t,
		"T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | Execute$ TrigTapOrUntap | TriggerZones$ Battlefield",
		"SVar:TrigTapOrUntap:DB$ TapOrUntap | Defined$ Self | TargetingPlayer$ TriggeredAttackingPlayer")
	if !strings.Contains(why, "TargetingPlayer$ without a target ask") {
		t.Errorf("TargetingPlayer$ without a target ask: refusals %q, want the named refusal", why)
	}
}
