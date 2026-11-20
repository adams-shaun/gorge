package cards

import (
	"strings"
	"testing"
)

// Each case is the upstream spelling of a real card's lines and the spelling
// the engine and kgen read. The scripts are trimmed to the lines that move.
func TestCanonicalizeUpstream(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"Ancient Runes: ControlledBy TriggeredPlayer in a Phase chain",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDmg\nSVar:TrigDmg:DB$ DealDamage | NumDmg$ X\nSVar:X:Count$Valid Artifact.ControlledBy TriggeredPlayer",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDmg\nSVar:TrigDmg:DB$ DealDamage | NumDmg$ X\nSVar:X:Count$Valid Artifact.ActivePlayerCtrl"},
		{"Asylum Visitor: the triggered player's hand",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDraw\nSVar:TrigDraw:DB$ Draw | NumCards$ TrigCount\nSVar:TrigCount:TriggeredPlayer$CardsInHand",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigDraw\nSVar:TrigDraw:DB$ Draw | NumCards$ TrigCount\nSVar:TrigCount:Count$ValidHand Card.ActivePlayerCtrl"},
		{"Oath of Druids: OpponentOf and the Than comparisons",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigOath\nSVar:TrigOath:DB$ Pump | ValidTgts$ Player.OpponentOf TriggeredPlayer+withMoreCreaturesThanTriggeredPlayer",
			"T:Mode$ Phase | Phase$ Upkeep | Execute$ TrigOath\nSVar:TrigOath:DB$ Pump | ValidTgts$ Player.OpponentToActive+withMoreCreaturesThanActive"},
		{"Razor Pendulum: a life-total check on the triggered player",
			"T:Mode$ Phase | Phase$ End of Turn | CheckSVar$ X | SVarCompare$ LE5 | Execute$ T\nSVar:X:TriggeredPlayer$LifeTotal",
			"T:Mode$ Phase | Phase$ End of Turn | LifeTotal$ ActivePlayer | LifeAmount$ LE5 | Execute$ T\nSVar:X:TriggeredPlayer$LifeTotal"},
		{"Citadel Siege: a trigger carried as an SVar body",
			"S:Mode$ Continuous | AddTrigger$ DT\nSVar:DT:Mode$ Phase | Phase$ BeginCombat | Execute$ Tap\nSVar:Tap:DB$ Tap | ValidTgts$ Creature.ControlledBy TriggeredPlayer",
			"S:Mode$ Continuous | AddTrigger$ DT\nSVar:DT:Mode$ Phase | Phase$ BeginCombat | Execute$ Tap\nSVar:Tap:DB$ Tap | ValidTgts$ Creature.ActivePlayerCtrl"},
		{"Kitt Kanto: reflexive trigger drops the remembered triggered player",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ Imm\nSVar:Imm:AB$ ImmediateTrigger | Cost$ tapXType<2/Creature> | RememberObjects$ TriggeredPlayer | Execute$ P\nSVar:P:DB$ Pump | ValidTgts$ Creature.ControlledBy DelayTriggerRemembered",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ Imm\nSVar:Imm:AB$ ImmediateTrigger | Cost$ tapXType<2/Creature> | Execute$ P\nSVar:P:DB$ Pump | ValidTgts$ Creature.ActivePlayerCtrl"},
		{"Peace in Our Time: an Effect keeps its RememberObjects",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ D\nSVar:D:DB$ Effect | StaticAbilities$ C | RememberObjects$ TriggeredPlayer",
			"T:Mode$ Phase | Phase$ BeginCombat | Execute$ D\nSVar:D:DB$ Effect | StaticAbilities$ C | RememberObjects$ TriggeredPlayer"},
		{"a non-Phase trigger keeps TriggeredPlayer",
			"T:Mode$ Drawn | Execute$ D\nSVar:D:DB$ Pump | ValidTgts$ Creature.ControlledBy TriggeredPlayer",
			"T:Mode$ Drawn | Execute$ D\nSVar:D:DB$ Pump | ValidTgts$ Creature.ControlledBy TriggeredPlayer"},
		{"Curse of Vitality",
			"T:Mode$ AttackersDeclared | Execute$ G\nSVar:G:DB$ GainLife | Defined$ Player.Opponent+controlsCreature.attacking Player.EnchantedBy",
			"T:Mode$ AttackersDeclared | Execute$ G\nSVar:G:DB$ GainLife | Defined$ TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy"},
		{"Guild Artisan",
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
