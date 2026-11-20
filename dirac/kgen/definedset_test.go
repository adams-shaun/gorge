package kgen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// ksetCard lowers a synthetic card with one root ability (root, with the
// SubAbility sub) or one trigger effect (trig with Execute -> trigBody).
func ksetCard(t *testing.T, typ, root, sub string, svars map[string]string, trig *cards.Trigger) *lCard {
	t.Helper()
	svars["DBX"] = sub
	face := &cards.Face{Name: "Synthetic", ManaCost: "1 B", Types: []string{typ}, SVars: svars}
	if trig != nil {
		trig.Effect = cards.ResolveSVar(svars, "Trig")
		if trig.Effect == nil {
			t.Fatalf("trigger body did not resolve")
		}
		face.Triggers = []cards.Trigger{*trig}
	} else {
		r := cards.ResolveSVar(map[string]string{"R": root}, "R")
		if r == nil {
			t.Fatalf("root %q did not resolve", root)
		}
		if sub != "" {
			r.Params["SubAbility"] = "DBX"
			r.Sub = cards.ResolveSVar(svars, "DBX")
			if r.Sub == nil {
				t.Fatalf("sub %q did not resolve", sub)
			}
		}
		face.Abilities = []*cards.SA{r}
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}

// ksetDump is the card's full LIR text.
func ksetDump(c *lCard) string {
	var b bytes.Buffer
	dumpLIR(&b, c)
	return b.String()
}

// ksetWantLowers fails unless the card lowers and its LIR prints a SET line
// containing each want substring.
func ksetWantLowers(t *testing.T, c *lCard, want ...string) {
	t.Helper()
	if len(c.Refusals) != 0 {
		t.Fatalf("refused: %+v", c.Refusals)
	}
	dump := ksetDump(c)
	if !strings.Contains(dump, "SET 0 ") {
		t.Fatalf("no SET line in LIR:\n%s", dump)
	}
	for _, w := range want {
		if !strings.Contains(dump, w) {
			t.Errorf("SET line lacks %q:\n%s", w, dump)
		}
	}
}

// TestDefinedSetGrammar: one case per compositional Defined$ form, each
// lowering through the set grammar and printing the SET program it produced.
func TestDefinedSetGrammar(t *testing.T) {
	t.Run("Remembered filter", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ DBX",
			"DB$ Pump | Defined$ Remembered.Creature | NumAtt$ +1", map[string]string{}, nil)
		ksetWantLowers(t, c, "BaseRef(Remembered)", "Inter(")
	})
	t.Run("TriggeredDefendingPlayer and You", func(t *testing.T) {
		tr := &cards.Trigger{Mode: "Attacks", Params: map[string]string{"ValidCard": "Card.Self", "Execute": "Trig"}}
		// three parts: the specialised two-player Draw union declines, so the
		// set grammar lowers the join.
		c := ksetCard(t, "", "R", "DB$ Draw | Defined$ TriggeredDefendingPlayer & You & Player | NumCards$ 1", map[string]string{
			"R":    "DB$ Draw",
			"Trig": "DB$ Draw | Defined$ TriggeredDefendingPlayer & You & Player | NumCards$ 1",
		}, tr)
		ksetWantLowers(t, c, "Union", "Defending", "You", "EachPlayer")
	})
	t.Run("Targeted and Self", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ Pump | ValidTgts$ Creature | Defined$ Targeted & Self | NumAtt$ +1", "", map[string]string{}, nil)
		ksetWantLowers(t, c, "Union", "BaseRef(Target", "BaseRef(Self")
	})
	t.Run("RememberedOwner projection", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ ChangeZone | ValidTgts$ Creature | Origin$ Battlefield | Destination$ Exile | RememberChanged$ True | SubAbility$ DBX",
			"DB$ Draw | Defined$ RememberedOwner | NumCards$ 1", map[string]string{}, nil)
		ksetWantLowers(t, c, "BaseRef(Remembered)", "Owner")
	})
	t.Run("ParentTargetedController projection", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ Destroy | ValidTgts$ Creature | SubAbility$ DBX",
			"DB$ Draw | Defined$ ParentTargetedController | NumCards$ 1", map[string]string{}, nil)
		ksetWantLowers(t, c, "BaseRef(Target", "Controller")
	})
	t.Run("Non complement", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ Draw | Defined$ NonTriggeredCardOwner | NumCards$ 1", "", map[string]string{}, nil)
		ksetWantLowers(t, c, "ComplementPlayers")
	})
	t.Run("LKICopy union", func(t *testing.T) {
		tr := &cards.Trigger{Mode: "Attacks", Params: map[string]string{"ValidCard": "Card.Self", "Execute": "Trig"}}
		c := ksetCard(t, "", "R", "DB$ Pump | Defined$ TriggeredAttackerLKICopy & Self | NumAtt$ +1", map[string]string{
			"R":    "DB$ Pump",
			"Trig": "DB$ Pump | Defined$ TriggeredAttackerLKICopy & Self | NumAtt$ +1",
		}, tr)
		ksetWantLowers(t, c, "Union", "LKI")
	})
	t.Run("Valid filter", func(t *testing.T) {
		c := ksetCard(t, "Sorcery", "SP$ Tap | Defined$ Valid Creature.YouCtrl", "", map[string]string{}, nil)
		ksetWantLowers(t, c, "BaseValid(")
	})
}

// TestDefinedSetBuildsRefSet: the LIR set reaches kop as a Table.Sets entry and
// the instruction's Obj becomes RefSet with a SetRef on the ability.
func TestDefinedSetBuildsRefSet(t *testing.T) {
	c := ksetCard(t, "Sorcery", "SP$ Tap | Defined$ Valid Creature.YouCtrl", "", map[string]string{}, nil)
	if len(c.Refusals) != 0 {
		t.Fatalf("refused: %+v", c.Refusals)
	}
	_, bs, pt, err := buildPool(&lowered{cards: []*lCard{c}})
	if err != nil {
		t.Fatal(err)
	}
	if len(pt.sets) < 2 || len(pt.sets[1].Ops) == 0 {
		t.Fatalf("Table.Sets has %d entries, want an interned set", len(pt.sets))
	}
	var ab *kop.Ability
	for i := range bs[0].def.Abilities {
		if len(bs[0].def.Abilities[i].SetRefs) != 0 {
			ab = &bs[0].def.Abilities[i]
			break
		}
	}
	if ab == nil {
		t.Fatal("no ability carries a SetRef")
	}
	sr := ab.SetRefs[0]
	if int(sr.PC) >= len(ab.Prog) || ab.Prog[sr.PC].Obj != kop.RefSet {
		t.Fatalf("SetRef PC %d does not name a RefSet instruction", sr.PC)
	}
	if int(sr.Set) >= len(pt.sets) {
		t.Fatalf("SetRef set %d out of range", sr.Set)
	}
}

// TestDefinedSetKeepsOldLowerings: on the FDN, BLB and MBC lists, every card
// the set fallback did not newly lower keeps the exact LIR key it has with the
// fallback disabled. (The brief names an edh list; this repo has none.)
func TestDefinedSetKeepsOldLowerings(t *testing.T) {
	heavyParallel(t)
	cardsDir := ratchetCards(t)
	for _, list := range []string{"FDN", "BLB", "MBC"} {
		t.Run(list, func(t *testing.T) {
			names, err := readCardList(testSetlistPath(t, list))
			if err != nil {
				t.Fatal(err)
			}
			now, err := LIRKeysFor(cardsDir, names)
			if err != nil {
				t.Fatal(err)
			}
			old, accepted, err := lirKeysNoSet(cardsDir, names)
			if err != nil {
				t.Fatal(err)
			}
			checked := 0
			for name, key := range old {
				if !accepted[name] {
					continue // refused before: the fallback may change its key
				}
				if now[name] != key {
					t.Errorf("%s: old lowering key moved %s -> %s", name, key, now[name])
				}
				checked++
			}
			if checked < 50 {
				t.Fatalf("%s: only %d accepted cards compared", list, checked)
			}
		})
	}
}

// lirKeysNoSet lowers names with defined()'s set fallback disabled, returning
// the keys and the names whose lowering had no refusals.
func lirKeysNoSet(cardsDir string, names []string) (map[string]string, map[string]bool, error) {
	p, err := loadCardNames(append([]string(nil), names...), cardsDir)
	if err != nil {
		return nil, nil, err
	}
	p.noSetFallback = true
	lw := lowerPool(p)
	all := append(append([]*lCard(nil), lw.cards...), lw.tokens...)
	keys := LIRKeys(all)
	accepted := map[string]bool{}
	for _, c := range all {
		if len(c.Refusals) == 0 {
			accepted[c.Name] = true
		}
	}
	return keys, accepted, nil
}
