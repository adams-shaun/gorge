package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Commander curses' second leg (TriggeredAttackingPlayer.Opponent+
// controlsCreature.attacking Player.EnchantedBy) and Inertia's
// trigger-player TapOrUntap, on synthetic scripts shaped like the corpus
// carriers. The leg lowering lives in lower_attackleg.go.

const kccDefEndUntap = "T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ TrigUntap"
const kccDefEndTap = "T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ TrigTapOrUntap"
const kccAtkEnch = "T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | TriggerZones$ Battlefield | Execute$ Trig"
const kccLeg = "TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"

func TestAttackLegLowering(t *testing.T) {
	// Curse of Bounty's shape: the "you untap" leg lowers first, then the
	// attacking-player leg ("each opponent attacking that player untaps..."),
	// guarded so the curse's own controller (the declaring player on its own
	// turn) is not an "opponent attacking that player".
	why, ab, pt := ktqBuildSV(t, kccAtkEnch,
		"SVar:Trig:DB$ UntapAll | ValidCards$ Permanent.nonLand | Defined$ You | SubAbility$ DBUntapLeg",
		"SVar:DBUntapLeg:DB$ UntapAll | ValidCards$ Permanent.nonLand | Defined$ "+kccLeg)
	if why != "" {
		t.Fatalf("the attacking-player leg refused: %s", why)
	}
	if len(ab.Prog) != 3 {
		t.Fatalf("the attacking-player leg lowered %d instructions, want the two untaps and one guard: %v", len(ab.Prog), ab.Prog)
	}
	if ab.Prog[1].Op != kop.OpSkipUnless || ab.Prog[1].C == 0 || ab.Prog[1].A.N != 1 {
		t.Fatalf("want a skip guard over the attacking-player leg, got %+v", ab.Prog[1])
	}
	c := pt.conds[ab.Prog[1].C]
	if c.Kind != kop.CondAnd {
		t.Fatalf("the guard must be the conjunction of the opponent's-turn and still-attacking halves, got %+v", c)
	}
	turn, present := pt.conds[c.Sub[0]], pt.conds[c.Sub[1]]
	if turn.Kind != kop.CondYourTurn || !turn.Neg {
		t.Fatalf("the guard must skip the controller's own turn, got %+v", turn)
	}
	// The resolution-time half: the trigger player must still control a
	// creature attacking THIS Aura's enchanted player.
	if present.Kind != kop.CondPresent || present.Neg {
		t.Fatalf("the guard must test a present attacker, got %+v", present)
	}
	if f := &pt.filters[int(present.F)]; f.Pred != kop.PredWord7 || f.PredN != kop.W7AttackingEnchTrig {
		t.Fatalf("the present-attacker filter must be W7AttackingEnchTrig, got Pred %d PredN %d", f.Pred, f.PredN)
	}
	if ab.Prog[0].Op != kop.OpUntapAll || ab.Prog[2].Op != kop.OpUntapAll {
		t.Fatalf("want the two UntapAll legs, got %v", ab.Prog)
	}
	found := false
	for id := ab.Prog[2].F; id != 0; {
		f := &pt.filters[int(id)]
		if f.Pred == kop.PredTrigPlayerCtrl {
			found = true
			break
		}
		id = f.Or
	}
	if !found {
		t.Fatalf("the attacking-player untap is not scoped to the trigger player: %+v", ab.Prog[2])
	}
	// The same leg on a trigger that is not an EnchantedBy AttackersDeclared
	// is not this leg: the ordinary TriggeredAttackingPlayer rule refuses it.
	why, _, _ = ktqBuildSV(t, kccDefEndUntap, "SVar:TrigUntap:DB$ UntapAll | ValidCards$ Permanent.nonLand | Defined$ "+kccLeg)
	if why == "" {
		t.Fatal("the attacking-player leg lowered off an AttackersDeclared trigger")
	}
}

func TestAttackLegParamGate(t *testing.T) {
	lc := &lowerCtx{ab: &lAbility{Trigger: &lTrigger{Event: "AttackersDeclared", DefPlayer: 4}}}
	for _, api := range []string{"Token", "Draw", "GainLife", "UntapAll"} {
		param := "Defined"
		if api == "Token" {
			param = "TokenOwner"
		}
		p := newParams(map[string]string{param: kccLeg})
		got, ok := lc.attackLegParamFor(&cards.SA{API: api}, p)
		if !ok || got != param {
			t.Errorf("%s$ %s: got (%q, %v), want (%q, true)", api, param, got, ok, param)
		}
	}
	p := newParams(map[string]string{"TokenOwner": "You"})
	if _, ok := lc.attackLegParamFor(&cards.SA{API: "Token"}, p); ok {
		t.Error("a non-second-leg TokenOwner$ matched the attack-leg gate")
	}
	// The trigger must be the per-defender Player.EnchantedBy shape.
	lc.ab.Trigger.DefPlayer = 2
	p = newParams(map[string]string{"TokenOwner": kccLeg})
	if _, ok := lc.attackLegParamFor(&cards.SA{API: "Token"}, p); ok {
		t.Error("a different AttackedTarget$ shape matched the attack-leg gate")
	}
}

func TestAttackLegRefusesUnmodelledParam(t *testing.T) {
	why, ab, _ := ktqBuildSV(t, kccAtkEnch,
		"SVar:Trig:DB$ UntapAll | ValidCards$ Permanent.nonLand | Defined$ "+kccLeg+" | RememberTapped$ True")
	if why == "" || ab.Prog != nil {
		t.Fatalf("a second-leg param with a side effect lowered: %q", why)
	}
	if !strings.Contains(why, "unread params [RememberTapped]") {
		t.Errorf("RememberTapped$ on the second leg: refusals %q, want the unread-param refusal", why)
	}
}

func TestAttackLegReceivers(t *testing.T) {
	// Draw and GainLife name the trigger player as their acting player, not
	// the controller (whatever seat the declaring player is).
	for _, c := range []struct{ api, param string }{{"Draw", "Defined"}, {"GainLife", "Defined"}} {
		why, ab, _ := ktqBuildSV(t, kccAtkEnch,
			"SVar:Trig:DB$ "+c.api+" | "+c.param+"$ "+kccLeg)
		if why != "" {
			t.Fatalf("%s: refused: %s", c.api, why)
		}
		if len(ab.Prog) != 2 || ab.Prog[0].Op != kop.OpSkipUnless || ab.Prog[0].A.N != 1 {
			t.Fatalf("%s: want one guarded instruction, got %v", c.api, ab.Prog)
		}
		want := kop.OpDraw
		if c.api == "GainLife" {
			want = kop.OpGainLife
		}
		if ab.Prog[1].Op != want || ab.Prog[1].Obj != kop.RefTriggerPlayer {
			t.Fatalf("%s: acting player is %v, want the trigger player", c.api, ab.Prog[1].Obj)
		}
	}
}

func TestTapOrUntapTriggerPlayer(t *testing.T) {
	// Curse of Inertia's shape: the target ask goes to the attacking player
	// (TgfTrigChooser) while the tap records the trigger player as its
	// provenance (OpTapOrUntap's W), both read off an AttackersDeclared
	// trigger only.
	why, ab, _ := ktqBuildSV(t,
		"T:Mode$ AttackersDeclared | AttackedTarget$ Player.EnchantedBy | Execute$ TrigTapOrUntap | TriggerZones$ Battlefield",
		"SVar:TrigTapOrUntap:DB$ TapOrUntap | TargetingPlayer$ TriggeredAttackingPlayer | ValidTgts$ Permanent | Tapper$ TriggeredAttackingPlayer")
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
		"SVar:TrigTapOrUntap:DB$ TapOrUntap | TargetingPlayer$ TriggeredAttackingPlayer | ValidTgts$ Permanent | Tapper$ TriggeredAttackingPlayer")
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
