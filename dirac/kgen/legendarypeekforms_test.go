package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// legendarypeekformsForms: forms needing a small kernel addition, checked as staticConditionForms are.
var legendarypeekformsForms = []formCase{
	{card: "Price of Fame", form: "Creature.Legendary (F2Legendary)", lir: "F2={types=0x2 Legendary}"},
	{card: "Arclight Phoenix", form: "Count$ThisTurnCast_Instant.YouCtrl,Sorcery.YouCtrl lowers onto the cast ring", lir: "ThisTurnCastSpec"},
	{card: "Battlewise Hoplite", form: "Heroic: SpellCast TargetsValid$ Card.Self (Tf2TargetsFilter2)", lir: "trigger SpellCast"},
	{card: "Legolas, Master Archer", form: "TargetsValid$ zone-free alternative lowers (ZoneAny)", lir: "trigger SpellCast"},
	{card: "Aven Windreader", form: "PeekAndReveal public reveal (OpPeek C=PeekPublic, no look_ack)", lir: "Peek obj=Target0 A=1 CS=Public"},
	{card: "Druidic Satchel", form: "PeekAndReveal public reveal + RememberRevealed (PeekPublic|PeekRemember)", lir: "CS=Public|Remember"},
}

func checkForms(t *testing.T, forms []formCase) {
	t.Helper()
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	var names []string
	for _, f := range forms {
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
	for _, f := range forms {
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
		if !strings.Contains(sb.String(), f.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", f.card, f.form, f.lir, sb.String())
		}
	}
}

func TestLegendaryAndPeekForms(t *testing.T) { checkForms(t, legendarypeekformsForms) }

// TestLegendaryFlagFilter: Legendary with a type head is the type AND the
// F2Legendary bit (a Legendary head alone still rides in Types).
func TestLegendaryFlagFilter(t *testing.T) {
	cb := &cardBuilder{}
	for _, tc := range []struct {
		expr  string
		types kop.TypeMask
	}{
		{"Creature.Legendary", kop.TCreature},
		{"Legendary.Creature", kop.TCreature},
		{"Card.Legendary+Artifact", kop.TArtifact},
	} {
		f, err := parseFilter(tc.expr, func(string) bool { return false }, func(string) bool { return false })
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		kf, err := cb.clause(f.Clauses[0], kop.ZoneBattlefield)
		if err != nil {
			t.Fatalf("%s: %v", tc.expr, err)
		}
		if kf.Types != tc.types || kf.Flags2 != kop.F2Legendary {
			t.Errorf("%s: types %#x flags2 %#x, want %#x + F2Legendary", tc.expr, kf.Types, kf.Flags2, tc.types)
		}
	}
}

// TestThisTurnCastCount: Count$ThisTurnCast_<spec> lowers onto the kernel's
// per-push cast ring (kop.ExThisTurnCastSpec) with gorge's
// You-scope substring test, the !CastSaSource exclusion and the
// wasCastFromYourHand[ByYou], wasCastFromExile and bare wasCastFromGraveyard
// provenance bits, the $CardTypes aggregate (with its
// source exclusion) and, in a SpellCast trigger body, ControlledBy
// TriggeredActivator; every spec the ring or the kernel's matchers do not
// read exactly still refuses.
func TestThisTurnCastCount(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}
	for spec, bits := range map[string]int{
		"Card.YouCtrl":                    kop.CastSpecYou,
		"Card.OppCtrl":                    0, // no "You": every player's casts, the object's controller now an opponent
		"Card":                            0,
		"Instant.YouCtrl,Sorcery.YouCtrl": kop.CastSpecYou,
		"Card.Instant+YouCtrl,Card.Sorcery+YouCtrl":                             kop.CastSpecYou,
		"Creature.YouCtrl":                                                      kop.CastSpecYou,
		"Card.nonCreature+YouCtrl":                                              kop.CastSpecYou,
		"Card.cmcGE4+YouCtrl":                                                   kop.CastSpecYou,
		"Card.OppCtrl+Blue,Card.OppCtrl+Black":                                  0,
		"Card.wasCastFromYourHandByYou":                                         kop.CastSpecYou | kop.CastSpecHand, // "You" is in the provenance token itself
		"Card.YouCtrl+wasCastFromYourHand":                                      kop.CastSpecYou | kop.CastSpecHand,
		"Card.YouCtrl+!wasCastFromYourHand":                                     kop.CastSpecYou | kop.CastSpecNotHand,
		"Card.YouCtrl+!wasCastFromYourHandByYou":                                kop.CastSpecYou | kop.CastSpecNotHand,
		"Card.YouCtrl+!CastSaSource":                                            kop.CastSpecYou | kop.CastSpecExclude,
		"Card.!CastSaSource":                                                    kop.CastSpecExclude,
		"Card.ControlledBy Player.Active":                                       0, // Weftwalking: the active player's casts (cmpActiveCtrl)
		"Card.ActivePlayerCtrl":                                                 0, // read off the pushed object itself on both matchers
		"Creature.ActivePlayerCtrl":                                             0,
		"Card.Historic+YouCtrl":                                                 kop.CastSpecYou,
		"Card.YouCtrl+hasXCost":                                                 kop.CastSpecYou, // false face down on both matchers
		"Card.YouCtrl+CastSa Spell.ManaFromTreasure":                            kop.CastSpecYou | kop.CastSpecTreasure,
		"Creature.YouCtrl+CastSa Spell.ManaFromTreasure":                        kop.CastSpecYou | kop.CastSpecTreasure,
		"Card.!CastSaSource+Instant+YouCtrl,Card.!CastSaSource+Sorcery+YouCtrl": kop.CastSpecYou | kop.CastSpecExclude,
		"Instant.YouCtrl+!CastSaSource,Sorcery.YouCtrl":                         kop.CastSpecYou | kop.CastSpecExclude, // gorge strips it from every alternative
		"Card.YouCtrl+wasCastFromExile":                                         kop.CastSpecYou | kop.CastSpecExile,   // Visions of Phyrexia, Wild-Magic Sorcerer
		"Card.YouCtrl+wasCastFromGraveyard":                                     kop.CastSpecYou | kop.CastSpecGrave,   // Laboratory Drudge: the CastFlags read
		"Instant.wasCastFromExile,Sorcery.wasCastFromExile":                     kop.CastSpecExile,
		"Card.YouCtrl$CardTypes":                                                kop.CastSpecYou | kop.CastSpecTypes, // April O'Neil
	} {
		v, err := lc.countValue("ThisTurnCast_"+spec, 0)
		if err != nil {
			t.Errorf("%s: %v", spec, err)
			continue
		}
		if v.Kind != vThisTurnCastSpec || v.N != bits || v.F == nil {
			t.Errorf("%s: got %s N=%d, want ThisTurnCastSpec N=%d with a filter", spec, v, v.N, bits)
		}
	}
	for _, spec := range []string{
		"Card.YouCtrl+wasCastFromYourHand+!wasCastFromYourHand", "Card.YouCtrl+!wasCastFromYourHand,Card.OppCtrl",
		"Card.YouCtrl+!wasCastFromExile", "Card.YouCtrl+wasCastFromYourGraveyard", "Card.YouCtrl+wasCastFromGraveyard,Card.OppCtrl",
		"Instant.wasCastFromExile,Sorcery", "Card.YouCtrl+wasCastByYou",
		"Card.YouCtrl+!CastSaSource$CardManaCost", "Instant.YouCtrl,Sorcery.YouCtrl$GreatestCardManaCost",
		"Card.Self", "Creature.nonLemur+withFlying+YouCtrl",
		"Card.YouCtrl+CastSa Spell.ManaFromTreasure,Card.YouCtrl",
		"Card.YouCtrl+CastSa", "Card.nonCreature+ControlledBy TriggeredActivator",
		"Card.RememberedPlayerCtrl+sharesNameWith",
		"Card.YouCtrl+wasCastFromYourHand,Card.OppCtrl",
	} {
		if _, err := lc.countValue("ThisTurnCast_"+spec, 0); err == nil {
			t.Errorf("%s: want a refusal", spec)
		}
	}
}

// TestThisTurnCastCtrlTrig: +ControlledBy TriggeredActivator lowers
// only inside a SpellCast trigger body for any activating player (gorge's
// SpecContext.TriggerContext); outside one it keeps the spaced-qualifier
// refusal, and a mixed-alternative spelling refuses.
func TestThisTurnCastCtrlTrig(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}
	lc.ab = &lAbility{Kind: akTriggered, Trigger: &lTrigger{Event: "SpellCast", Whose: whoAny}}
	lc.trigBody = true
	v, err := lc.thisTurnCastValue("Card.nonCreature+ControlledBy TriggeredActivator")
	if err != nil {
		t.Fatalf("SpellCast trigger body: %v", err)
	}
	if v.Kind != vThisTurnCastSpec || v.N != kop.CastSpecCtrlTrig || v.F == nil {
		t.Fatalf("got %s N=%d, want ThisTurnCastSpec N=%d with a filter", v, v.N, kop.CastSpecCtrlTrig)
	}
	lc.trigBody = false
	if _, err := lc.thisTurnCastValue("Card.nonCreature+ControlledBy TriggeredActivator"); err == nil {
		t.Errorf("outside a trigger body: want a refusal")
	}
	lc.trigBody = true
	lc.ab.Trigger.Event = "Attacks"
	if _, err := lc.thisTurnCastValue("Card.nonCreature+ControlledBy TriggeredActivator"); err == nil {
		t.Errorf("outside a SpellCast trigger: want a refusal")
	}
	lc.ab.Trigger.Event, lc.ab.Trigger.Whose = "SpellCast", whoOpp
	if _, err := lc.thisTurnCastValue("Card.nonCreature+ControlledBy TriggeredActivator"); err == nil {
		t.Errorf("a restricted activating player: want a refusal")
	}
	lc.ab.Trigger.Whose = whoAny
	if _, err := lc.thisTurnCastValue("Card.Creature+ControlledBy TriggeredActivator,Card.Instant"); err == nil {
		t.Errorf("the token on some alternatives only: want a refusal")
	}
}

// TestThisTurnActivated: only Activated.YouCtrl+inZoneGraveyard (Laboratory
// Drudge, kop.ExThisTurnActGrave) and Activated.Exhaust+YouCtrl (Elvish
// Refueler, kop.ExThisTurnActExhaust) lower; every other
// Count$ThisTurnActivated_ spec refuses with a named reason.
func TestThisTurnActivated(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}
	v, err := lc.countValue("ThisTurnActivated_Activated.YouCtrl+inZoneGraveyard", 0)
	if err != nil || v.Kind != vThisTurnActGrave {
		t.Fatalf("got %v err %v, want ThisTurnActGrave", v, err)
	}
	for _, spec := range []string{
		"Activated.YouCtrl", "Activated.Equip+YouCtrl", "Activated.Cycling+YouCtrl", "Activated.Loyalty",
		"Activated.YouCtrl+inZoneBattlefield", "Activated.inZoneGraveyard", "Activated.YouCtrl+inZoneGraveyard,Activated.OppCtrl",
	} {
		_, err := lc.countValue("ThisTurnActivated_"+spec, 0)
		if err == nil || !strings.Contains(err.Error(), "ThisTurnActivated") {
			t.Errorf("%s: want a named refusal, got %v", spec, err)
		}
	}
}

// TestHasXReduceCostLowers: Zimone, Infinite Analyst's ThisTurnCast hasXCost
// count and its ReduceCost of hasXCost spells lower with no refusal: the
// kernel's X ask prices X before the floor, so a reduction larger than the
// printed generic pays into {X} as gorge's xAsk does (CR 601.2f).
func TestHasXReduceCostLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Zimone, Infinite Analyst"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("Zimone, Infinite Analyst refused: %s", r.Reason)
	}
}
