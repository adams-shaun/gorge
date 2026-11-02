package kgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// nine Count$ value shapes (package 1, Part A). One
// carrier per shape, checked like validaggregateformsForms.
var countHeadsForms = []formCase{
	{card: "All-Fates Scroll", form: "Land.YouCtrl$DifferentCardNames (AggDistinctNames)", lir: "A=ValidAgg({types=0x1 ctrl=1},agg=9)"},
	{card: "Audience with Trostani", form: "Creature.YouCtrl+token$DifferentCardNames: the filter keeps +token", lir: "A=ValidAgg({types=0x2 ctrl=1 Token},agg=9)"},
	{card: "Loot, the Key to Everything", form: "$CardTypes (AggCardTypes)", lir: ",agg=10)"},
	{card: "Toph, the Blind Bandit", form: "$CardCounters.P1P1 (AggSumCounters, Ctr P1P1)", lir: "P=ValidAgg({types=0x1 ctrl=1},agg=11,P1P1)"},
	{card: "Triumphant Chomp", form: "/LimitMin.2: v >= 2 keeps v, else 2", lir: "A=Compare(op=1,n=2,ValidAgg({types=0x0 sub=Dinosaur ctrl=1},agg=1),Branches(ValidAgg({types=0x0 sub=Dinosaur ctrl=1},agg=1),2))"},
	{card: "Flow State", form: "SVar$Z1/Plus.Z2 over /LimitMax.1: v <= 1 keeps v, else 1", lir: "Sum(Compare(op=2,n=1,CountGraveyard({types=0x20 owner=1}),Branches(CountGraveyard({types=0x20 owner=1}),1)),Compare(op=2,n=1,"},
	{card: "The Goose Mother", form: "Count$xPaid/HalfUp: (X+1)/2", lir: "Sum(X,1)/2"},
	{card: "Cid, Timeless Artificer", form: "ValidBattlefield,Graveyard: battlefield + graveyard", lir: "P=Sum(Count({types=0x0 sub=Artificer ctrl=1}),CountGraveyard({types=0x0 sub=Artificer ctrl=1}))"},
	{card: "Armored Armadillo", form: "Count$CardToughness is the source's toughness", lir: "A=ToughnessOf(Self)"},
	{card: "Molten Note", form: "CastTotalManaSpent in a spell's own chain", lir: "DealDamage obj=Target0 A=CastManaSpent"},
	{card: "Cache Grab", form: "Remembered$Valid Squirrel: the remembered objects matching, where they are now", lir: "RefCount({types=0x0 sub=Squirrel},Remembered)"},
	// the >= 1 gate of Count$CountersAddedThisTurn <Kind> You <Spec>
	{card: "Beast, Erudite Aerialist", form: "CountersAddedThisTurn P1P1 You Card.Self (ExCtrAdded, >= 1)", lir: "cond=Value(op1,CtrAdded,1)"},
	// PlayerCountPropertyYou$SacrificedThisTurn <Permanent|Artifact>
	{card: "Sawblade Skinripper", form: "SacrificedThisTurn Permanent (ExSacrificedThisTurn N 0)", lir: "DealDamage obj=Target0 A=SacrificedThisTurn(n=0)"},
	{card: "Furtive Courier", form: "SacrificedThisTurn Artifact (ExSacrificedThisTurn N 1)", lir: "cond=Value(op1,SacrificedThisTurn(n=1),1)"},
	// PlayerCountPropertyYou$CardsDiscardedThisTurn
	{card: "Misty Knight, Hero for Hire", form: "CardsDiscardedThisTurn (ExDiscardedThisTurn)", lir: "Draw obj=You A=DiscardedThisTurn"},
	// Count$LastStateBattlefieldWithFallback in a spell's
	// own program (the frozen cast-time battlefield count, ExCastBattlefield)
	{card: "Steer Clear", form: "LastStateBattlefieldWithFallback Permanent.Mount+YouCtrl (ExCastBattlefield, a Count$Compare arm)", lir: "Compare(op=1,n=1,CastBattlefield,Branches(4,2))"},
	{card: "Faerie Fencing", form: "LastStateBattlefieldWithFallback Faerie.YouCtrl (ExCastBattlefield, an SVar of xPaid/Plus)", lir: "Compare(op=1,n=1,CastBattlefield,Branches(3,0))"},
}

func TestCountHeadsForms(t *testing.T) {
	checkForms(t, countHeadsForms)
	// Astelli: CastTotalManaSpent in a self-ETB trigger sets SpentBound
	// (kernel Tf2SpentBound), and not XBound.
	cardsDir := ratchetCards(t)
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte("Astelli Reclaimer\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, c := range lowerPool(p).cards {
		if c.Name != "Astelli Reclaimer" {
			continue
		}
		if len(c.Refusals) != 0 {
			t.Fatalf("Astelli refused: %v", c.Refusals)
		}
		for _, a := range c.Abilities {
			if a.Trigger != nil && a.Trigger.SpentBound && !a.Trigger.XBound {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("Astelli Reclaimer: no ETB trigger with SpentBound")
	}
}

// TestCountHeadsStillRefused: the shapes this package leaves refused keep
// their refusal strings (never a guess).
func TestCountHeadsStillRefused(t *testing.T) {
	checkForms(t, []formCase{
		{card: "Bat Colony", form: "CastTotalManaSpent Cave (filtered form)", refused: "CastTotalManaSpent"},
		{card: "Summon: Brynhildr", form: "CountersAddedThisTurn LORE You Card.EffectSource (an Effect emblem static has no source object)", refused: "CountersAddedThisTurn"},
		{card: "Backstreet Bruiser", form: "CardCounters.ALL has no kernel read", refused: `Count$Valid aggregate "CardCounters.ALL"`},
	})
}

// TestCountHeadsModifierRefusals: the modifier arms refuse what they do not
// read exactly.
func TestCountHeadsModifierRefusals(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{"Y": "Count$YourLifeTotal"}}}
	for _, body := range []string{
		"Count$CardPower/LimitMin.Y",       // an SVar operand
		"Count$CardPower/LimitMax.",        // empty operand
		"Count$CardPower/LimitMin",         // no operand
		"Count$YourLifeTotal/HalfUp.1",     // HalfUp takes none
		"SVar$Missing",                     // names no SVar
		"Count$YourLifeTotal/Twice/HalfUp", // HalfUp after a modifier
	} {
		if v, err := lc.svarValue(body, 0); err == nil {
			t.Errorf("%q lowered to %s; want a refusal", body, v)
		}
	}
	if v, err := lc.svarValue("Count$CardPower/LimitMin.2", 0); err != nil || v.Kind != vCompare || v.Op != opGE || v.N != 2 {
		t.Errorf("LimitMin.2: got %s (op %d n %d), %v; want Compare GE 2", v, v.Op, v.N, err)
	}
	if v, err := lc.svarValue("Count$CardPower/LimitMax.1", 0); err != nil || v.Kind != vCompare || v.Op != opLE || v.N != 1 {
		t.Errorf("LimitMax.1: got %s (op %d n %d), %v; want Compare LE 1", v, v.Op, v.N, err)
	}
	if v, err := lc.svarValue("SVar$Y/Plus.1", 0); err != nil || v.Kind != vLifeOf || v.Add != 1 {
		t.Errorf("SVar$Y/Plus.1: got %s, %v; want the SVar's value +1", v, err)
	}
}

// TestCtrAddedModifierRefused: the kernel reads Count$CountersAddedThisTurn as
// a 0/1 presence flag, so a modifier on the head (an amount) refuses.
func TestCtrAddedModifierRefused(t *testing.T) {
	cardsDir := ratchetCards(t)
	dir := t.TempDir()
	src, err := os.ReadFile(filepath.Join(cardsDir, "cardsfolder", "l", "lasting_tarfire.txt"))
	if err != nil {
		t.Fatal(err)
	}
	for i, mod := range []string{"", "/Minus.1", "/DivideEvenlyDown.2", "/Plus.1", "/Twice"} {
		sub := filepath.Join(dir, fmt.Sprint(i))
		if err := os.MkdirAll(filepath.Join(sub, "cardsfolder", "l"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := strings.Replace(string(src), "Count$CountersAddedThisTurn Any You Creature", "Count$CountersAddedThisTurn Any You Creature"+mod, 1)
		if mod != "" && body == string(src) {
			t.Fatal("Lasting Tarfire's SVar:X changed shape")
		}
		if err := os.WriteFile(filepath.Join(sub, "cardsfolder", "l", "lasting_tarfire.txt"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		list := filepath.Join(sub, "cards.txt")
		if err := os.WriteFile(list, []byte("Lasting Tarfire\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		p, err := loadCardList(list, sub)
		if err != nil {
			t.Fatal(err)
		}
		all, _, _, err := buildPool(lowerPool(p))
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, c := range all {
			for _, r := range c.Refusals {
				got = append(got, r.Reason)
			}
		}
		if mod == "" { // the control: the unmodified head lowers
			if len(got) != 0 {
				t.Errorf("Lasting Tarfire (control): refused: %q", got)
			}
			continue
		}
		if !strings.Contains(strings.Join(got, "; "), "CountersAddedThisTurn") {
			t.Errorf("Lasting Tarfire%s: want a CountersAddedThisTurn refusal, got %q", mod, got)
		}
	}
}
