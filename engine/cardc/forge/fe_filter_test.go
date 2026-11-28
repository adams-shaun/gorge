package forge

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

const (
	obj    = ir.SubjObject
	player = ir.SubjPlayer
)

func p(s ir.Subject, pr ir.Pred, src string) *ir.Filter {
	return &ir.Filter{Subject: s, Op: ir.FPred, Pred: pr, Src: src}
}

func w(f *ir.Filter, word string) *ir.Filter    { f.Word = word; return f }
func r(f *ir.Filter, ref *ir.Ref) *ir.Filter    { f.Ref = ref; return f }
func z(f *ir.Filter, zs ...ir.Zone) *ir.Filter  { f.Zones = ir.MakeZoneSet(zs...); return f }
func c(f *ir.Filter, cl ir.ColorSet) *ir.Filter { f.Colors = cl; return f }
func n(s ir.Subject, a *ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: s, Op: ir.FNot, Args: []*ir.Filter{a}}
}
func all(s ir.Subject, a ...*ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: s, Op: ir.FAll, Args: a}
}
func anyF(s ir.Subject, a ...*ir.Filter) *ir.Filter {
	return &ir.Filter{Subject: s, Op: ir.FAny, Args: a}
}
func cmpF(f *ir.Filter, op ir.CmpOp, v int32) *ir.Filter {
	f.Cmp, f.Value = op, &ir.Value{Kind: ir.VConst, N: v}
	return f
}

func ref(k ir.RefKind) *ir.Ref       { return &ir.Ref{Kind: k} }
func mem(m ir.Memory) *ir.Ref        { return &ir.Ref{Kind: ir.RefMemory, Memory: m} }
func card() *ir.Filter               { return p(obj, ir.PAny, "Card") }
func creature() *ir.Filter           { return w(p(obj, ir.PCardType, "Creature"), "Creature") }
func onCard(a *ir.Filter) *ir.Filter { return all(obj, card(), a) }
func opps() *ir.Ref {
	return &ir.Ref{Kind: ir.RefAll, Filter: r(&ir.Filter{Subject: player, Op: ir.FPred, Pred: ir.POpponentOf}, ref(ir.RefYou))}
}

// TestLowerFilterTable lowers one filter written for each table row and
// compares the tree.
func TestLowerFilterTable(t *testing.T) {
	cases := []struct {
		text string
		want *ir.Filter
	}{
		// Bases.
		{"Card", card()},
		{"Permanent", p(obj, ir.PPermanentCard, "Permanent")},
		{"Creature", creature()},
		{"Legendary", w(p(obj, ir.PSupertype, "Legendary"), "Legendary")},
		{"Goblin", w(p(obj, ir.PSubtype, "Goblin"), "Goblin")},
		{"nonLand", n(obj, w(p(obj, ir.PCardType, "nonLand"), "Land"))},
		{"Emblem", p(obj, ir.PEmblem, "Emblem")},
		{"Spell", z(p(ir.SubjSpell, ir.PAny, "Spell"), ir.ZoneStack)},
		{"Player", p(player, ir.PAny, "Player")},
		{"Opponent", r(p(player, ir.POpponentOf, "Opponent"), ref(ir.RefYou))},
		{"You", r(p(player, ir.PPlayerIs, "You"), ref(ir.RefYou))},
		{"Any", anyF(obj,
			z(w(p(ir.SubjPermanent, ir.PCardType, "Any"), "Creature"), ir.ZoneBattlefield),
			z(w(p(ir.SubjPermanent, ir.PCardType, "Any"), "Planeswalker"), ir.ZoneBattlefield),
			z(w(p(ir.SubjPermanent, ir.PCardType, "Any"), "Battle"), ir.ZoneBattlefield),
			p(player, ir.PAny, "Any"))},
		// Whole-spec Self / Other, alternatives, the subject rule.
		{"Self", onCard(r(p(obj, ir.PIs, "Self"), ref(ir.RefSelf)))},
		{"Other", onCard(n(obj, r(p(obj, ir.PIs, "Other"), ref(ir.RefSelf))))},
		{"Creature,Artifact", anyF(obj, creature(), w(p(obj, ir.PCardType, "Artifact"), "Artifact"))},
		{"Creature,Player", anyF(obj, creature(), p(player, ir.PAny, "Player"))},
		{"Spell,Spell.YouCtrl", z(anyF(ir.SubjSpell, z(p(ir.SubjSpell, ir.PAny, "Spell"), ir.ZoneStack),
			z(all(ir.SubjSpell, z(p(ir.SubjSpell, ir.PAny, "Spell"), ir.ZoneStack),
				r(p(ir.SubjSpell, ir.PControlledBy, "YouCtrl"), ref(ir.RefYou))), ir.ZoneStack)), ir.ZoneStack)},
		{"Creature.YouCtrl+powerGE4", all(obj, creature(),
			r(p(obj, ir.PControlledBy, "YouCtrl"), ref(ir.RefYou)), cmpF(p(obj, ir.PPower, "powerGE4"), ir.CmpGE, 4))},
		// Predicates.
		{"Card.StrictlySelf", onCard(r(p(obj, ir.PIs, "StrictlySelf"), ref(ir.RefSelf)))},
		{"Card.StrictlyOther", onCard(n(obj, r(p(obj, ir.PIs, "StrictlyOther"), ref(ir.RefSelf))))},
		{"Card.YouDontCtrl", onCard(n(obj, r(p(obj, ir.PControlledBy, "YouDontCtrl"), ref(ir.RefYou))))},
		{"Card.OppCtrl", onCard(r(p(obj, ir.PControlledBy, "OppCtrl"), opps()))},
		{"Card.YouOwn", onCard(r(p(obj, ir.POwnedBy, "YouOwn"), ref(ir.RefYou)))},
		{"Card.YouDontOwn", onCard(n(obj, r(p(obj, ir.POwnedBy, "YouDontOwn"), ref(ir.RefYou))))},
		{"Card.OppOwn", onCard(r(p(obj, ir.POwnedBy, "OppOwn"), opps()))},
		{"Card.ActivePlayerCtrl", onCard(r(p(obj, ir.PControlledBy, "ActivePlayerCtrl"), ref(ir.RefActivePlayer)))},
		{"Card.DefenderCtrl", onCard(r(p(obj, ir.PControlledBy, "DefenderCtrl"), ref(ir.RefDefendingPlayer)))},
		{"Card.ControlledBy TriggeredDefendingPlayer", onCard(r(p(obj, ir.PControlledBy,
			"ControlledBy TriggeredDefendingPlayer"), ref(ir.RefDefendingPlayer)))},
		{"Card.ControlledBy TriggeredPlayer", onCard(r(p(obj, ir.PControlledBy, "ControlledBy TriggeredPlayer"),
			ref(ir.RefEventPlayer)))},
		{"Player.Opponent", all(player, p(player, ir.PAny, "Player"), r(p(player, ir.POpponentOf, "Opponent"), ref(ir.RefYou)))},
		{"Card.Elf", onCard(w(p(obj, ir.PSubtype, "Elf"), "Elf"))},
		{"Card.nonCreature", onCard(n(obj, w(p(obj, ir.PCardType, "nonCreature"), "Creature")))},
		{"Card.Red", onCard(c(p(obj, ir.PColor, "Red"), ir.ColorR))},
		{"Card.nonBlue", onCard(n(obj, c(p(obj, ir.PColor, "nonBlue"), ir.ColorU)))},
		{"Card.Colorless", onCard(p(obj, ir.PColorless, "Colorless"))},
		{"Card.nonColorless", onCard(n(obj, p(obj, ir.PColorless, "nonColorless")))},
		{"Card.MultiColor", onCard(p(obj, ir.PMulticolored, "MultiColor"))},
		{"Card.MonoColor", onCard(p(obj, ir.PMonocolored, "MonoColor"))},
		{"Card.Historic", onCard(anyF(obj, w(p(obj, ir.PCardType, "Historic"), "Artifact"),
			w(p(obj, ir.PSupertype, "Historic"), "Legendary"), w(p(obj, ir.PSubtype, "Historic"), "Saga")))},
		{"Card.Outlaw", onCard(anyF(obj, w(p(obj, ir.PSubtype, "Outlaw"), "Assassin"),
			w(p(obj, ir.PSubtype, "Outlaw"), "Mercenary"), w(p(obj, ir.PSubtype, "Outlaw"), "Pirate"),
			w(p(obj, ir.PSubtype, "Outlaw"), "Rogue"), w(p(obj, ir.PSubtype, "Outlaw"), "Warlock")))},
		{"Card.token", onCard(p(obj, ir.PToken, "token"))},
		{"Card.nonToken", onCard(n(obj, p(obj, ir.PToken, "nonToken")))},
		{"Card.tapped", onCard(p(obj, ir.PTapped, "tapped"))},
		{"Card.untapped", onCard(n(obj, p(obj, ir.PTapped, "untapped")))},
		{"Card.!tapped", onCard(n(obj, p(obj, ir.PTapped, "!tapped")))},
		{"Card.attacking", onCard(p(obj, ir.PAttacking, "attacking"))},
		{"Card.blocking", onCard(p(obj, ir.PBlocking, "blocking"))},
		{"Card.blocked", onCard(p(obj, ir.PBlocked, "blocked"))},
		{"Card.unblocked", onCard(p(obj, ir.PUnblocked, "unblocked"))},
		{"Card.attackingYou", onCard(r(p(obj, ir.PAttacking, "attackingYou"), ref(ir.RefYou)))},
		{"Card.blockingSource", onCard(r(p(obj, ir.PBlocking, "blockingSource"), ref(ir.RefSelf)))},
		{"Card.blockedBySource", onCard(r(p(obj, ir.PBlockedBy, "blockedBySource"), ref(ir.RefSelf)))},
		{"Card.faceDown", onCard(p(obj, ir.PFaceDown, "faceDown"))},
		{"Card.faceUp", onCard(n(obj, p(obj, ir.PFaceDown, "faceUp")))},
		{"Card.equipped", onCard(p(obj, ir.PEquipped, "equipped"))},
		{"Card.enchanted", onCard(p(obj, ir.PEnchanted, "enchanted"))},
		{"Card.modified", onCard(p(obj, ir.PModified, "modified"))},
		{"Card.HasCounters", onCard(p(obj, ir.PHasAnyCounter, "HasCounters"))},
		{"Card.kicked", onCard(p(obj, ir.PKicked, "kicked"))},
		{"Card.suspended", onCard(p(obj, ir.PSuspended, "suspended"))},
		{"Card.hasXCost", onCard(p(obj, ir.PHasXInCost, "hasXCost"))},
		{"Card.IsCommander", onCard(p(obj, ir.PIsCommander, "IsCommander"))},
		{"Card.TopLibrary", onCard(p(obj, ir.PTopOfLibrary, "TopLibrary"))},
		{"Card.EnchantedBy", onCard(r(p(obj, ir.PIs, "EnchantedBy"), &ir.Ref{Kind: ir.RefAttachedTo, Of: ref(ir.RefSelf)}))},
		{"Card.EquippedBy", onCard(r(p(obj, ir.PIs, "EquippedBy"), &ir.Ref{Kind: ir.RefAttachedTo, Of: ref(ir.RefSelf)}))},
		{"Card.AttachedBy", onCard(r(p(obj, ir.PIs, "AttachedBy"), &ir.Ref{Kind: ir.RefAttachedTo, Of: ref(ir.RefSelf)}))},
		{"Card.cmcLE3", onCard(cmpF(p(obj, ir.PManaValue, "cmcLE3"), ir.CmpLE, 3))},
		{"Card.toughnessLT2", onCard(cmpF(p(obj, ir.PToughness, "toughnessLT2"), ir.CmpLT, 2))},
		{"Card.powerEQ0", onCard(cmpF(p(obj, ir.PPower, "powerEQ0"), ir.CmpEQ, 0))},
		{"Card.cmcGT1", onCard(cmpF(p(obj, ir.PManaValue, "cmcGT1"), ir.CmpGT, 1))},
		{"Card.cmcNE5", onCard(cmpF(p(obj, ir.PManaValue, "cmcNE5"), ir.CmpNE, 5))},
		{"Card.counters_GE1_P1P1", onCard(w(cmpF(p(obj, ir.PCounters, "counters_GE1_P1P1"), ir.CmpGE, 1), "+1/+1"))},
		{"Card.counters_EQ0_M1M1", onCard(w(cmpF(p(obj, ir.PCounters, "counters_EQ0_M1M1"), ir.CmpEQ, 0), "-1/-1"))},
		{"Card.counters_GE3_LOYALTY", onCard(w(cmpF(p(obj, ir.PCounters, "counters_GE3_LOYALTY"), ir.CmpGE, 3), "loyalty"))},
		{"Card.withFlying", onCard(w(p(obj, ir.PHasKeyword, "withFlying"), "Flying"))},
		{"Card.withFirst Strike", onCard(w(p(obj, ir.PHasKeyword, "withFirst Strike"), "First strike"))},
		{"Card.withoutDouble Strike", onCard(n(obj, w(p(obj, ir.PHasKeyword, "withoutDouble Strike"), "Double strike")))},
		{"Card.hasKeywordTrample", onCard(w(p(obj, ir.PHasKeyword, "hasKeywordTrample"), "Trample"))},
		{"Card.inZoneGraveyard", z(card(), ir.ZoneGraveyard)},
		{"Card.YouOwn+inZoneExile", z(all(obj, card(), r(p(obj, ir.POwnedBy, "YouOwn"), ref(ir.RefYou))), ir.ZoneExile)},
		{"Card.ThisTurnEntered", onCard(p(obj, ir.PEnteredThisTurn, "ThisTurnEntered"))},
		{"Card.ThisTurnEnteredFrom_Graveyard", onCard(z(p(obj, ir.PEnteredThisTurn, "ThisTurnEnteredFrom_Graveyard"), ir.ZoneGraveyard))},
		{"Card.attackedThisTurn", onCard(p(obj, ir.PAttackedThisTurn, "attackedThisTurn"))},
		{"Card.wasDealtDamageThisTurn", onCard(p(obj, ir.PDealtDamageThisTurn, "wasDealtDamageThisTurn"))},
		{"Card.wasCast", onCard(p(obj, ir.PWasCast, "wasCast"))},
		{"Card.wasCastByYou", onCard(r(p(obj, ir.PWasCast, "wasCastByYou"), ref(ir.RefYou)))},
		{"Card.wasCastFromYourHand", onCard(z(p(obj, ir.PWasCast, "wasCastFromYourHand"), ir.ZoneHand))},
		{"Card.wasCastFromYourHandByYou", onCard(r(z(p(obj, ir.PWasCast, "wasCastFromYourHandByYou"), ir.ZoneHand), ref(ir.RefYou)))},
		{"Card.wasCastFromExile", onCard(z(p(obj, ir.PWasCast, "wasCastFromExile"), ir.ZoneExile))},
		{"Card.wasCastFromGraveyard", onCard(z(p(obj, ir.PWasCast, "wasCastFromGraveyard"), ir.ZoneGraveyard))},
		{"Card.wasCastFromYourGraveyard", onCard(r(z(p(obj, ir.PWasCast, "wasCastFromYourGraveyard"), ir.ZoneGraveyard), ref(ir.RefYou)))},
		{"Card.IsRemembered", onCard(r(p(obj, ir.PIs, "IsRemembered"), mem(ir.MemRemembered)))},
		{"Card.IsImprinted", onCard(r(p(obj, ir.PIs, "IsImprinted"), mem(ir.MemImprinted)))},
		{"Card.IsTriggerRemembered", onCard(r(p(obj, ir.PIs, "IsTriggerRemembered"), mem(ir.MemTriggerRemembered)))},
		{"Card.ChosenCard", onCard(r(p(obj, ir.PIs, "ChosenCard"), mem(ir.MemChosenCard)))},
		{"Card.ExiledWithSource", onCard(r(p(obj, ir.PIs, "ExiledWithSource"), mem(ir.MemExiledWithSource)))},
		{"Card.nonChosenCard", onCard(n(obj, r(p(obj, ir.PIs, "nonChosenCard"), mem(ir.MemChosenCard))))},
		{"Card.ChosenType", onCard(r(p(obj, ir.PSubtype, "ChosenType"), mem(ir.MemChosenType)))},
		{"Card.ChosenColor", onCard(r(p(obj, ir.PColor, "ChosenColor"), mem(ir.MemChosenColor)))},
		{"Card.NamedCard", onCard(r(p(obj, ir.PName, "NamedCard"), mem(ir.MemNamedCard)))},
		{"Card.sameName", onCard(r(p(obj, ir.PSharesName, "sameName"), ref(ir.RefSelf)))},
		{"Card.namedTest_Name;Two", onCard(w(p(obj, ir.PName, "namedTest_Name;Two"), "Test Name,Two"))},
		// A comma part continuing a name.
		{"Card.namedSir Test, the Example", onCard(w(p(obj, ir.PName, "namedSir Test, the Example"), "Sir Test, the Example"))},
	}
	seen := map[string]bool{}
	for _, tc := range cases {
		if seen[tc.text] {
			t.Fatalf("case %q written twice", tc.text)
		}
		seen[tc.text] = true
		if err := ir.ValidateFilter(tc.want); err != nil {
			t.Errorf("%q: the expected tree does not validate: %v", tc.text, err)
		}
		got, rf := LowerFilter(tc.text)
		if rf != nil {
			t.Errorf("LowerFilter(%q) refused: %v", tc.text, rf)
			continue
		}
		if !ir.Equal(got, tc.want) {
			t.Errorf("LowerFilter(%q)\n got %v\nwant %v", tc.text, got, tc.want)
		}
		if err := ir.ValidateFilter(got); err != nil {
			t.Errorf("LowerFilter(%q) does not validate: %v", tc.text, err)
		}
	}
}

// TestLowerFilterRefusals refuses one filter written for each refusal kind,
// naming the first token it cannot read with digits folded.
func TestLowerFilterRefusals(t *testing.T) {
	cases := []struct {
		text string
		kind RefusalKind
		atom string
	}{
		{"", RefuseEmpty, ""},
		{"  ", RefuseEmpty, ""},
		{"EACH Creature", RefuseEachForm, "EACH"},
		{"Creature,Wombatish", RefuseUnknownWord, "Wombatish"},
		{"creature", RefuseUnknownWord, "creature"},
		{"SpellAbility.YouCtrl", RefuseBaseAbility, "SpellAbility"},
		{"Activated", RefuseBaseAbility, "Activated"},
		{"TargetedCard.Creature", RefuseBaseReferent, "TargetedCard"},
		{"CARDNAME", RefuseBaseReferent, "CARDNAME"},
		{"Creature.Self,Self", RefuseBaseReferent, "Self"},
		{"Creature.fooBar2", RefuseUnknownPredicate, "fooBarN"},
		{"Creature.!inZoneHand", RefuseUnknownPredicate, "!inZoneHand"},
		{"Creature.inZoneNowhere", RefuseUnknownPredicate, "inZoneNowhere"},
		{"Card.Opponent", RefuseUnknownPredicate, "Opponent"},
		{"Creature.withHovering", RefuseUnknownKeyword, "withHovering"},
		{"Creature.counters_GE1_WIDGET", RefuseUnknownCounter, "counters_GEN_WIDGET"},
		{"Creature.cmcLEX", RefuseBoundNamesSVar, "cmcLEX"},
		{"Creature.counters_GEY_P1P1", RefuseBoundNamesSVar, "counters_GEY_PNPN"},
		{"Creature.ControlledBy Somebody", RefuseNeedsReference, "ControlledBy Somebody"},
		{"Creature.OwnedBy Somebody", RefuseNeedsReference, "OwnedBy Somebody"},
		{"Creature.TargetedPlayerCtrl", RefuseNeedsReference, "TargetedPlayerCtrl"},
		{"Creature.!PairedWith", RefuseNeedsReference, "!PairedWith"},
		{"Creature.sharesTypeWith Valid Creature.YouCtrl", RefuseNestedFilter, "sharesTypeWith Valid Creature.YouCtrl"},
		{"Creature.IsValidThing", RefuseNestedFilter, "IsValidThing"},
	}
	for k := RefuseNone + 1; k < refusalKindCount; k++ {
		found := k >= firstValueRefusal // LowerValue's kinds: TestLowerValueRefusals
		for _, tc := range cases {
			found = found || tc.kind == k
		}
		if !found {
			t.Errorf("refusal kind %s has no case", k)
		}
		if got, ok := ParseRefusalKind(k.String()); !ok || got != k {
			t.Errorf("ParseRefusalKind(%q) = %v, %v", k.String(), got, ok)
		}
	}
	for _, tc := range cases {
		f, rf := LowerFilter(tc.text)
		if rf == nil {
			t.Errorf("LowerFilter(%q) = %v, want a %s refusal", tc.text, f, tc.kind)
			continue
		}
		if rf.Kind != tc.kind || rf.Atom != tc.atom {
			t.Errorf("LowerFilter(%q) refused %v, want %s: %s", tc.text, rf, tc.kind, tc.atom)
		}
		if want := tc.kind.String() + ": " + tc.atom; rf.Error() != want {
			t.Errorf("Error() = %q, want %q", rf.Error(), want)
		}
	}
}
