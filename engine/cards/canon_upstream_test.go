package cards

import (
	"strings"
	"testing"
)

// Each case is a synthetic probe script in the upstream spelling and the
// spelling the engine and kgen read, trimmed to the lines that move.
func TestCanonicalizeUpstream(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"ControlledBy TriggeredPlayer in a Phase chain",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ PrbHit\nSVar:PrbHit:DB$ DealDamage | NumDmg$ PrbN\nSVar:PrbN:Count$Valid Artifact.ControlledBy TriggeredPlayer",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ PrbHit\nSVar:PrbHit:DB$ DealDamage | NumDmg$ PrbN\nSVar:PrbN:Count$Valid Artifact.ActivePlayerCtrl"},
		{"the triggered player's hand",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ PrbDraw\nSVar:PrbDraw:DB$ Draw | NumCards$ PrbHand\nSVar:PrbHand:TriggeredPlayer$CardsInHand",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ PrbDraw\nSVar:PrbDraw:DB$ Draw | NumCards$ PrbHand\nSVar:PrbHand:Count$ValidHand Card.ActivePlayerCtrl"},
		{"OpponentOf and the Than comparisons",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ PrbPump\nSVar:PrbPump:DB$ Pump | ValidTgts$ Player.OpponentOf TriggeredPlayer+withMoreCreaturesThanTriggeredPlayer",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ PrbPump\nSVar:PrbPump:DB$ Pump | ValidTgts$ Player.OpponentToActive+withMoreCreaturesThanActive"},
		{"a life-total check on the triggered player",
			"T:Mode$ Phase | Phase$ End of Turn | Execute$ PrbGo | CheckSVar$ PrbLife | SVarCompare$ LE5\nSVar:PrbLife:TriggeredPlayer$LifeTotal",
			"T:Mode$ Phase | Phase$ End of Turn | Execute$ PrbGo | LifeTotal$ ActivePlayer | LifeAmount$ LE5\nSVar:PrbLife:TriggeredPlayer$LifeTotal"},
		{"a trigger carried as an SVar body",
			"S:Mode$ Continuous | AddTrigger$ PrbTrig\nSVar:PrbTrig:Mode$ Phase | Phase$ BeginCombat | Execute$ PrbTap\nSVar:PrbTap:DB$ Tap | ValidTgts$ Creature.ControlledBy TriggeredPlayer",
			"S:Mode$ Continuous | AddTrigger$ PrbTrig\nSVar:PrbTrig:Mode$ Phase | Phase$ BeginCombat | Execute$ PrbTap\nSVar:PrbTap:DB$ Tap | ValidTgts$ Creature.ActivePlayerCtrl"},
		{"a reflexive trigger drops the remembered triggered player",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ PrbImm\nSVar:PrbImm:AB$ ImmediateTrigger | RememberObjects$ TriggeredPlayer | Cost$ tapXType<2/Creature> | Execute$ PrbPmp\nSVar:PrbPmp:DB$ Pump | ValidTgts$ Creature.ControlledBy DelayTriggerRemembered",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ PrbImm\nSVar:PrbImm:AB$ ImmediateTrigger | Cost$ tapXType<2/Creature> | Execute$ PrbPmp\nSVar:PrbPmp:DB$ Pump | ValidTgts$ Creature.ActivePlayerCtrl"},
		{"an Effect keeps its RememberObjects",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ PrbEff\nSVar:PrbEff:DB$ Effect | RememberObjects$ TriggeredPlayer | StaticAbilities$ PrbStat",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ PrbEff\nSVar:PrbEff:DB$ Effect | RememberObjects$ TriggeredPlayer | StaticAbilities$ PrbStat"},
		{"a non-Phase trigger keeps TriggeredPlayer",
			"T:Mode$ Drawn | Execute$ PrbOther\nSVar:PrbOther:DB$ Pump | ValidTgts$ Creature.ControlledBy TriggeredPlayer",
			"T:Mode$ Drawn | Execute$ PrbOther\nSVar:PrbOther:DB$ Pump | ValidTgts$ Creature.ControlledBy TriggeredPlayer"},
		{"the attacking player of an enchanted player",
			"T:Mode$ AttackersDeclared | Execute$ PrbGain\nSVar:PrbGain:DB$ GainLife | Defined$ Player.Opponent+controlsCreature.attacking Player.EnchantedBy",
			"T:Mode$ AttackersDeclared | Execute$ PrbGain\nSVar:PrbGain:DB$ GainLife | Defined$ TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"},
		{"a bounce then an Effect naming the target and its owner maps back to the LKI form",
			"T:Mode$ ChangesZone | Destination$ Battlefield | Execute$ PrbBounce\nSVar:PrbBounce:DB$ ChangeZone | ValidTgts$ Creature.OppCtrl | Origin$ Battlefield | Destination$ Hand | SubAbility$ PrbEff\nSVar:PrbEff:DB$ Effect | RememberObjects$ Targeted & TargetedOwner | StaticAbilities$ PrbStat | Duration$ UntilYourNextTurn",
			"T:Mode$ ChangesZone | Destination$ Battlefield | Execute$ PrbBounce\nSVar:PrbBounce:DB$ ChangeZone | ValidTgts$ Creature.OppCtrl | Origin$ Battlefield | Destination$ Hand | SubAbility$ PrbEff | RememberLKI$ True\nSVar:PrbEff:DB$ Effect | RememberObjects$ RememberedLKI & RememberedOwner | StaticAbilities$ PrbStat | Duration$ UntilYourNextTurn | SubAbility$ DBCanonTargetedOwnerCleanup\nSVar:DBCanonTargetedOwnerCleanup:DB$ Cleanup | ClearRemembered$ True\n"},
		{"an Effect with no moving parent keeps Targeted & TargetedOwner",
			"SVar:PrbEff:DB$ Effect | RememberObjects$ Targeted & TargetedOwner | StaticAbilities$ PrbStat",
			"SVar:PrbEff:DB$ Effect | RememberObjects$ Targeted & TargetedOwner | StaticAbilities$ PrbStat"},
		{"the defending player's life against the greatest opposing life",
			"S:Mode$ Continuous | CheckSVar$ TriggeredDefendingPlayer$LifeTotal | SVarCompare$ GECount$OppGreatestLifeTotal",
			"S:Mode$ Continuous | Condition$ NoOpponentHasMoreLifeThanAttacked"},
	}
	for _, c := range cases {
		got := string(canonicalizeUpstream([]byte(c.in)))
		if got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestCanonicalAPIAndNonActive(t *testing.T) {
	sa, _ := parseSA("x", "SP$ Bookkeeping | RememberAll$ Creature.cmcLEX | RememberZone$ Graveyard")
	if sa.API != "PumpAll" || sa.Params["ValidCards"] != "Creature.cmcLEX" || sa.Params["PumpZone"] != "Graveyard" || sa.Params["RememberPumped"] != "True" {
		t.Errorf("Day of Black Sun / Phyrexian Grimoire: %+v", sa)
	}
	for _, api := range []string{"TargetOnly", "Nothing", "EndOfTurn", "Bookkeeping"} {
		if sa, _ := parseSA("x", "DB$ "+api+" | Defined$ Self"); sa.API != "Pump" {
			t.Errorf("%s -> %s", api, sa.API)
		}
	}
	if sa, _ := parseSA("x", "DB$ PutCounter | Defined$ TargetedCard"); sa.Params["Defined"] != "ParentTarget" {
		t.Errorf("Mind Spiral: %+v", sa)
	}
	if sa, _ := parseSA("x", "DB$ PutCounter | Defined$ ParentTarget.Creature"); sa.Params["Defined"] != "ParentTarget" {
		t.Errorf("Sygg's Command: %+v", sa)
	}
	p := parseParams("Caster$ Player.!Active | ValidPlayer$ Player.Opponent+!Active")
	if !strings.HasSuffix(p["Caster"], "NonActive") || p["ValidPlayer"] != "Player.Opponent+NonActive" {
		t.Errorf("!Active: %v", p)
	}
}
