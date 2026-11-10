package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// filterpredicateformsForms: lowerings, one carrier
// per class, checked like staticConditionForms (no refusal, or the kept refusal, and a
// lowered-IR fragment).
var filterpredicateformsForms = []formCase{
	{card: "Bionic Blow", form: "ValidTgtsDesc$ is description text (gorge never reads it)"},
	{card: "Bant Sureblade", form: "MultiColor (PredMultiColor)", lir: "Other cmp(7 0 0  )"},
	{card: "Board the Weatherlight", form: "Historic (PredHistoric)", lir: "F={types=0x0 cmp(8 0 0  )}"},
	{card: "Armory Veteran", form: "equipped (PredEquipped)", lir: "F={types=0x0 Self cmp(9 0 0  )}"},
	{card: "Ancestral Blade", form: "RememberTokens$ True (TokRememberObj: the token joins the remembered set)", lir: "CreateToken obj=You A=1 C=4 CS=w_1_1_soldier\n     1 Attach obj=Remembered"},
	{card: "Tribal Flames", form: "Count$Domain (a sum of five at-least-one basic-type counts)", lir: "A=Sum(Sum(Sum(Sum(CountAtLeast({types=0x0 sub=Plains ctrl=1},1,0),CountAtLeast({types=0x0 sub=Island ctrl=1},1,0))"},
	{card: "Akki Ronin", form: "Attacks Alone$ True (Tf2Alone)", lir: "trigger Attacks"},
	{card: "Isperia, Supreme Judge", form: "Attacks Attacked$ You,Planeswalker.YouCtrl (Tf2AtkYou|Tf2AtkYourPW)", lir: "trigger Attacks"},
	{card: "Cursed Land", form: "Phase ValidPlayer$ Player.EnchantedController (WhoseEnchCtrl)", lir: "trigger Phase F=nil F2=nil whose=3"},
	{card: "Esper Sentinel", form: "ValidActivatingPlayer$ Player.Opponent (= Opponent)"},
	{card: "Haughty Djinn", form: "CDA SetPower$ only (toughness printed)", lir: "owner=1 | types=0x40 owner=1}) T=4"},
	{card: "Cloudshift", form: "ChangeZone RememberTargets$ (a flicker: the exiled target, read later in the chain)", lir: "0 Exile obj=Target0 CS=Remember|from=Battlefield\n     1 Return obj=Remembered"},
	{card: "Journey to Nowhere", form: "RememberTargets$ read by its own leaves trigger (CardDef.RemPersist)", lir: "rempersist"},
	{card: "Duelist's Heritage", form: "AttackersDeclared with no AttackingPlayer$ (any attacking player)", lir: "trigger AttackersDeclared"},
	{card: "Biomantic Mastery", form: "ValidTgts$ with Defined$ You: the target is declared, Defined$ is the drawer (the refusal moves on to the targeted-player value)", refused: "ThisTargetedPlayer$Valid Creature.YouCtrl of a slot that is not opponents-only"},
	{card: "Blood Lust", form: "Count$Compare of a non-battlefield value (ExCompare)", lir: "B=-Compare(op=1,n=4,ToughnessOf(Target0)-1,Branches(4,ToughnessOf(Target0)-1))"},
	{card: "Ensoul Artifact", form: "override static without RemoveCardTypes$ (AddType$ adds)", lir: "override{pt=5/5 addtypes=2}"},
	{card: "In Bolas's Clutches", form: "override static AddType$ Legendary (a supertype adds, not a subtype)", lir: "override{addtypes=256}"},
	{card: "Leyline of Singularity", form: "global override static AddType$ Legendary (a supertype adds)", lir: "override{addtypes=256}"},
	{card: "On Serra's Wings", form: "override static AddType$ Legendary beside an additive P/T pump (two statics: layer 7c + layer 4; the pump outlives an ability loss, CR 613.6)", lir: "  static Continuous F={types=0x0 EnchantedBy} F2=nil cond=true P=1 T=1 kw=[] addsub=[] amount=- zone=0 outlivesloss cost={0}\n  static Override F={types=0x0 EnchantedBy} F2=nil cond=true P=- T=- kw=[] addsub=[] amount=- zone=0 override{addtypes=256 kw=Flying&Vigilance&Lifelink} cost={0}"},
	{card: "Mesa Lynx", form: "a printed static's EffectZone$ Battlefield (the default)"},
	{card: "Glistener Seer", form: "SubCounter<1/OIL/CARDNAME> (the source, gorge SubCounterTargetsSource)"},
	{card: "Catalyst Elemental", form: "mana Produced$ G G (one colour twice = two of it)"},
	{card: "Azorius Signet", form: "mana W U (simultaneous colours from a fee beside {T}: a first-only filter gap)"},
	{card: "Into the Story", form: "PlayerCountOpponents$HighestCardsInGraveyard (the opponent's graveyard)"},
	{card: "Company Commander", form: "PlayerCountOpponents$Amount (one opponent)"},
	{card: "Bloodghast", form: "PlayerCountOpponents$LowestLifeTotal (the opponent's life)"},
	{card: "Brass-Talon Chimera", form: "Pump KW$ Duration$ Permanent (a permanent keyword override)", lir: "CS=Permanent"},
	{card: "Leyline of the Guildpact", form: "AddType$ AllBasicLandType (the five basic land types, gorge appendLandTypes order)", lir: "addsub=[Plains Island Swamp Mountain Forest]"},
	{card: "Prismatic Omen", form: "AddType$ AllBasicLandType (the five basic land types, gorge appendLandTypes order)", lir: "addsub=[Plains Island Swamp Mountain Forest]"},
	{card: "Dryad of the Ilysian Grove", form: "AddType$ AllBasicLandType (the five basic land types, gorge appendLandTypes order)", lir: "addsub=[Plains Island Swamp Mountain Forest]"},
	{card: "Nylea's Presence", form: "AddType$ AllBasicLandType (the five basic land types, gorge appendLandTypes order)", lir: "addsub=[Plains Island Swamp Mountain Forest]"},
	{card: "Aeromunculus", form: "Adapt$ (offer gate + resolution guard, P1P1 on the source)", lir: "cond=Present({types=0x0 Self cmp(3 1 1  P1P1)},op3,0)\n     0 SkipIf if=Present({types=0x0 Self cmp(3 1 1  P1P1)},op3,0) skip=1\n     1 PutCounter obj=Self A=1 CS=P1P1"},
}

func TestFilterPredicateAndTriggerParamForms(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range filterpredicateformsForms {
		names = append(names, f.card)
	}
	list := filepath.Join(dir, "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	lw := lowerPool(p)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, f := range filterpredicateformsForms {
		c := byName[f.card]
		if c == nil {
			t.Errorf("%s: not lowered", f.card)
			continue
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if f.refused == "" && got != "" {
			t.Errorf("%s (%s): refused: %s", f.card, f.form, got)
		}
		if f.refused != "" && !strings.Contains(got, f.refused) {
			t.Errorf("%s (%s): want refusal %q, got %q", f.card, f.form, f.refused, got)
		}
		if f.lir == "" {
			continue
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		for _, tn := range c.Tokens {
			for _, tc := range lw.tokens {
				if tc.Name == tn {
					dumpLIR(&sb, tc)
				}
			}
		}
		if !strings.Contains(sb.String(), f.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", f.card, f.form, f.lir, sb.String())
		}
	}
}
