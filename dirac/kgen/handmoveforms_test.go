package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// handmoveformsCarriers is one card per form:
// each must lower with no refusal, and its lowered IR must contain the
// fragment.
var handmoveformsCarriers = map[string]struct{ form, lir string }{
	"Aboshan, Cephalid Emperor": {"api:TapAll ValidCards$ (OpTapAll)", "TapAll F={types=0x2 WithoutFlying}"},
	"Ashmouth Hound":            {"Mode$ AttackerBlockedByCreature, both anchors (Tf2PerPair)", "pairs anchor-blocker=true"},
	"Bottle of Suleiman":        {"api:FlipCoin, Win and Lose branches", "RollDie A=2 B=0 C=1 // flip a coin: 1 heads (win), 2 tails\n     1 SkipIf if=Value(op3,RollResult,1) skip=1"},
	"Balduvian Rage":            {"DelayedTrigger Phase$ Upkeep NextTurn$ (kop.DelayUpkeep)", "DelayedTrigger C=1 CS=NextUpkeep"},
	"Peek":                      {"api:RevealHand (Look$: gorge's look_ack, OpLookAck)", "F={players=3} unique=false same=false diff=false divided=0 spell=false\n     0 LookAck obj=Target0 // look at the hand\n     1 Draw obj=You A=1"},
	"Brittle Effigy":            {"cost Exile<1/CARDNAME> (CostExileSelf)", "ExileSelf"},
	"Renewed Faith":             {"Mode$ Cycled ValidCard$ Card.Self (Tf2Cycled, zone graveyard)", "zone=3 step= combat=0 opt=true cond=true nth=0 first=false once=false yourturn=false oppturn=false minatk=0\n    cycled"},
	"Lightning Rift":            {"Mode$ Cycled ValidCard$ Card on the battlefield (Tf2Cycled)", "trigger Discarded F={types=0x0} F2=nil whose=0 zone=1 step= combat=0 opt=false cond=true nth=0 first=false once=false yourturn=false oppturn=false minatk=0\n    cycled"},
	"Cathartic Operation":       {"api:Seek Num$ 2 (OpSeek)", "Seek obj=You A=2 F="},
	"Arboreal Grazer":           {"ChangeType$ Hand->Battlefield, markerless \"you may put\" (optional pick)", "ChooseMove obj=You A=1 C=1 CS=from=Hand|Tapped F={types=0x1 ctrl=1}"},
	"Aether Vial":               {"ChangeType$ Hand->Battlefield, Optional$ True (SrConfirm)", "CS=from=Hand|Confirm"},
	"Ambush Krotiq":             {"Hidden$ ChangeType$ Battlefield->Hand (a battlefield hidden pick)", "ChooseMove obj=You A=1 C=2 CS=from=Battlefield|Mandatory"},
	"Castigate":                 {"ChangeType$ with ValidTgts$: a target opponent's hand to exile, Chooser$ You", "ChooseMove obj=Target0 A=1 C=5 CS=from=Hand|Mandatory|ChooserYou"},
	"Angelic Renewal":           {"a triggered ability's Cost$ Sac<1/CARDNAME> (kop.PaySacSelf)", "IfPaid obj=You A=1 CS=SacrificeSelf"},
	"Angrath, Captain of Chaos": {"api:Amass Type$ Zombies (OpAmass)", "Amass obj=You A=2 CS=b_0_0_zombie_army // also a Zombie"},
	"Alabaster Dragon":          {"ChangeZone Shuffle$ True into the library (kop.LibTopShuffle)", "ToLibrary obj=TriggeredNew CS=TopShuffle|from=Graveyard"},
	"Armored Kincaller":         {"api:Reveal from the hand, Optional$ (a SrReveal choose-move)", "CS=from=Hand|Mandatory|Confirm|Remember|Reveal"},
	"Aura of Silence":           {"stat:RaiseCost Activator$ Opponent (StatIncreaseCost, StRaiseOpp)", "static RaiseCost F={types=0x4 | types=0x8} F2=nil cond=true P=- T=- kw=[] addsub=[] amount=2 zone=0 flags=[ActivatorOpp]"},
	"Chill":                     {"stat:RaiseCost for any caster", "static RaiseCost F={types=0x0 color=R}"},
	"Dragonlord Dromoka":        {"stat:CantBeCast Caster$ Opponent, Condition$ PlayerTurn (StatCantBeCast)", "static CantBeCast F={types=0x0} F2=nil cond=YourTurn() P=- T=- kw=[] addsub=[] amount=- zone=0 flags=[ActivatorOpp]"},
	"A.I.M. Scientists":         {"api:Connive (OpConnive + OpConniveCounters)", "Connive obj=Self A=1\n     1 ConniveCounters obj=Self"},
	"Chimney Imp":               {"ChangeType$ with ValidTgts$: a target opponent's hand card on top of their library", "ChooseMove obj=Target0 A=1 C=4 CS=from=Hand|Mandatory"},
	"Debt to the Kami":          {"Hidden$ battlefield pick of a target opponent (SrOwnZone)", "ChooseMove obj=Target0 A=1 C=5 CS=from=Battlefield|Mandatory|OwnZone"},
	"Scrabbling Claws":          {"Hidden$ graveyard pick of a target player to exile", "ChooseMove obj=Target0 A=1 C=5 CS=from=Graveyard|Mandatory|OwnZone"},
	"Blood Funnel":              {"Counter Defined$ TriggeredSpellAbility in a SpellCast trigger", "1 Counter obj=Triggered"},
	"Dawnglare Invoker":         {"api:TapAll with a player ValidTgts$", "TapAll obj=Target0 F={types=0x2}"},
}

// handmoveformsRefused is one card per form left refused, with a
// fragment of the expected refusal reason.
var handmoveformsRefused = map[string]string{
	"Barrow-Blade":      "AttackerBlockedByCreature ValidCard", // an attachment anchor
	"Astrolabe":         "delayed upkeep trigger in a mana ability",
	"Black Tulip":       "mana ability with an exile-self cost",
	"Bazaar of Wonders": "sharesNameWith", // the counter's condition, not the Defined$
}

func lowerNamed(t *testing.T, names []string) []*lCard {
	t.Helper()
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	return lowerPool(p).cards
}

func TestHandMoveAndVerbFormsAccepted(t *testing.T) {
	var names []string
	for n := range handmoveformsCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := handmoveformsCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, want.form, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", c.Name, want.form, want.lir, sb.String())
		}
	}
	if seen != len(handmoveformsCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(handmoveformsCarriers))
	}
}

func TestHandMoveAndVerbFormsRefused(t *testing.T) {
	if len(handmoveformsRefused) == 0 {
		return
	}
	var names []string
	for n := range handmoveformsRefused {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := handmoveformsRefused[c.Name]
		if !ok {
			continue
		}
		seen++
		found := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no refusal containing %q (refusals %v)", c.Name, want, c.Refusals)
		}
	}
	if seen != len(handmoveformsRefused) {
		t.Errorf("lowered %d of %d refused carriers", seen, len(handmoveformsRefused))
	}
}
