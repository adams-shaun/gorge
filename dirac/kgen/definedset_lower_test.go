package kgen

import (
	"bytes"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// definedSetLIR lowers a synthetic card and returns its LIR dump.
func definedSetLIR(t *testing.T, src string) (*lCard, string) {
	t.Helper()
	c := abLower(t, src, false)
	var b bytes.Buffer
	dumpLIR(&b, c)
	return c, b.String()
}

// definedSetCards are the synthetic carriers of TestDefinedSetGrammar: one
// ability whose subject is the Defined$ value v.
func definedSetSrc(lines ...string) string {
	return strings.Join(append([]string{"Name:Probe", "ManaCost:1", "Types:Creature Bear", "PT:2/2"}, lines...), "\n") + "\n"
}

// TestDefinedSetGrammar: every grammar row lowers to its SET program and the
// instruction reads it through an rSet subject; every shape the kernel would
// play differently from gorge refuses with its own reason.
func TestDefinedSetGrammar(t *testing.T) {
	lowers := []struct {
		name, src, set string
	}{
		{"qualified base", definedSetSrc("A:AB$ Pump | Cost$ T | Defined$ Self.YouOwn | NumAtt$ +1"),
			"SET 0 SetSelf SetAndFilter(F="},
		{"controller", definedSetSrc("A:AB$ LoseLife | Cost$ T | Defined$ CardController | LifeAmount$ 1"),
			"SET 0 SetSelf SetControllerOf"},
		{"owner", definedSetSrc("T:Mode$ Attacks | ValidCard$ Creature.YouCtrl | TriggerZones$ Battlefield | Execute$ ProbeTrigLose",
			"SVar:ProbeTrigLose:DB$ LoseLife | LifeAmount$ 2 | Defined$ TriggeredAttackerOwner"),
			"SET 0 SetTriggerObj SetOwnerOf"},
		{"non", definedSetSrc("T:Mode$ SpellCast | ValidCard$ Card | TriggerZones$ Battlefield | Execute$ ProbeTrigDraw",
			"SVar:ProbeTrigDraw:DB$ Draw | Defined$ NonTriggeredCardController | NumCards$ 1"),
			"SET 0 SetTriggerPlayer SetPlayersComplement"},
		{"non over an owner", definedSetSrc("T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ ProbeTrigLose",
			"SVar:ProbeTrigLose:DB$ LoseLife | LifeAmount$ 1 | Defined$ NonTriggeredCardOwner"),
			"SET 0 SetTriggerObj SetOwnerOf SetPlayersComplement"},
		{"oppnon", definedSetSrc("A:AB$ Draw | Cost$ T | ValidTgts$ Opponent | NumCards$ 1 | SubAbility$ ProbeDBLose",
			"SVar:ProbeDBLose:DB$ LoseLife | LifeAmount$ 1 | Defined$ OppNonParentTarget"),
			"SET 0 SetTarget(0) SetController SetUnion SetPlayersComplement"},
		{"opponent", definedSetSrc("T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | TriggerZones$ Battlefield | Execute$ ProbeTrigGain",
			"SVar:ProbeTrigGain:DB$ GainLife | LifeAmount$ 2 | Defined$ TriggeredCardOpponent"),
			"SET 0 SetTriggerPlayer SetPlayersComplement"},
		{"opponent-of alias", definedSetSrc("T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature | TriggerZones$ Battlefield | Execute$ ProbeTrigGain",
			"SVar:ProbeTrigGain:DB$ GainLife | LifeAmount$ 2 | Defined$ Player.OpponentOf TriggeredCardController"),
			"SET 0 SetTriggerPlayer SetPlayersComplement"},
		{"you and a player never you", definedSetSrc("A:AB$ Draw | Cost$ T | Defined$ You & Opponent | NumCards$ 1"),
			"SET 0 SetController SetEachOpponent SetUnion"},
	}
	for _, tc := range lowers {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if len(c.Refusals) != 0 {
				t.Fatalf("refused: %s\n%s", abReasons(c), lir)
			}
			if !strings.Contains(lir, tc.set) {
				t.Fatalf("no %q line:\n%s", tc.set, lir)
			}
			if !strings.Contains(lir, "obj=Set0") {
				t.Fatalf("no instruction reads Set0:\n%s", lir)
			}
		})
	}
	refuses := []struct {
		name, src, reason string
	}{
		{"object union", definedSetSrc("T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Creature.Other | TriggerZones$ Battlefield | Execute$ ProbeTrigExile",
			"SVar:ProbeTrigExile:DB$ ChangeZone | Origin$ Graveyard | Destination$ Exile | Defined$ Self & TriggeredCard"),
			`Defined$ "Self & TriggeredCard": union order or duplicates differ from gorge`},
		{"a player, then you", definedSetSrc("A:AB$ Draw | Cost$ T | ValidTgts$ Player | NumCards$ 1 | SubAbility$ ProbeDBLose",
			"SVar:ProbeDBLose:DB$ LoseLife | LifeAmount$ 1 | Defined$ TargetedAndYou"),
			`Defined$ "TargetedAndYou": union order or duplicates differ from gorge`},
		{"qualifier over a multi-object slot", definedSetSrc("A:AB$ Pump | Cost$ T | ValidTgts$ Creature | TargetMin$ 0 | TargetMax$ 2 | SubAbility$ ProbeDBTap",
			"SVar:ProbeDBTap:DB$ Tap | Defined$ ParentTarget.Bear"),
			`Defined$ "ParentTarget.Bear": a qualifier over a multi-object slot`},
		{"complement of objects", definedSetSrc("A:AB$ LoseLife | Cost$ T | Defined$ NonSelf | LifeAmount$ 1"),
			`Defined$ "NonSelf": a complement of "Self", which can hold objects`},
		{"base with no set kind", definedSetSrc("A:AB$ LoseLife | Cost$ T | Defined$ EnchantedOwner | LifeAmount$ 1"),
			`Defined$ "EnchantedOwner": base Equipped has no set kind`},
		{"set read under an op that takes none", definedSetSrc("A:AB$ Mill | Cost$ T | Defined$ CardController | NumCards$ 1"),
			`Defined$ "CardController": a set read outside an op that takes one`},
		{"opponents of you", definedSetSrc("A:AB$ Draw | Cost$ T | Defined$ Player.OpponentOf You | NumCards$ 1"),
			`Defined$ "Player.OpponentOf You": Player.OpponentOf lowers only over TriggeredCardController (gorge names nobody over another referent but Remembered and ReplacedPlayer)`},
		{"opponents of a targeted player", definedSetSrc("A:AB$ Draw | Cost$ T | ValidTgts$ Player | NumCards$ 1 | SubAbility$ ProbeDBLose",
			"SVar:ProbeDBLose:DB$ Draw | NumCards$ 1 | Defined$ Player.OpponentOf TargetedPlayer"),
			`Defined$ "Player.OpponentOf TargetedPlayer": Player.OpponentOf lowers only over TriggeredCardController (gorge names nobody over another referent but Remembered and ReplacedPlayer)`},
		{"owner on a Drawn trigger", definedSetSrc("T:Mode$ Drawn | ValidCard$ Card.OppOwn | TriggerZones$ Battlefield | Execute$ ProbeTrigLose",
			"SVar:ProbeTrigLose:DB$ LoseLife | LifeAmount$ 1 | Defined$ NonTriggeredCardOwner"),
			`on a Drawn trigger, which binds no object`},
		{"no head", definedSetSrc("A:AB$ LoseLife | Cost$ T | Defined$ NoSuchReferent | LifeAmount$ 1"),
			`Defined$ "NoSuchReferent"`},
	}
	for _, tc := range refuses {
		t.Run(tc.name, func(t *testing.T) {
			c, lir := definedSetLIR(t, tc.src)
			if strings.Contains(lir, "  SET ") {
				t.Fatalf("a refused shape kept a SET line:\n%s", lir)
			}
			got := abReasons(c)
			if !slices.ContainsFunc(strings.Split(got, "\n"), func(r string) bool { return strings.HasSuffix(r, tc.reason) }) {
				t.Fatalf("want a refusal ending %q, got:\n%s", tc.reason, got)
			}
		})
	}
}

// TestDefinedSetBuildGuards: the builder binds a set instruction to its PC
// and interns the expression, and refuses, never emits, a set the kernel's
// execSet would panic on: an op it does not serve, and an ability that uses
// the slot it binds members to.
func TestDefinedSetBuildGuards(t *testing.T) {
	src := definedSetSrc("A:AB$ LoseLife | Cost$ T | Defined$ CardController | LifeAmount$ 1")
	build := func(edit func(c *lCard)) built {
		c := abLower(t, src, false)
		if len(c.Refusals) != 0 || len(c.Sets) != 1 {
			t.Fatalf("precondition: one lowered set, refusals %q", abReasons(c))
		}
		if edit != nil {
			edit(c)
		}
		return buildCard(newPoolTables(&lowered{cards: []*lCard{c}}, nil), c)
	}
	b := build(nil)
	if len(b.refusals) != 0 {
		t.Fatalf("refused: %+v", b.refusals)
	}
	ab := b.def.Abilities[0]
	if len(ab.SetRefs) != 1 || ab.SetRefs[0].PC != 0 || ab.Prog[0].Obj != kop.RefSet {
		t.Fatalf("SetRefs %+v, Prog[0].Obj %v", ab.SetRefs, ab.Prog[0].Obj)
	}
	wrongOp := build(func(c *lCard) { c.Abilities[0].Prog[0].Op = "Mill" })
	if !slices.ContainsFunc(wrongOp.refusals, func(r refusal) bool { return strings.Contains(r.Reason, "Defined$ set on kop Mill") }) {
		t.Errorf("a set under OpMill was not refused: %+v", wrongOp.refusals)
	}
	lastSlot := build(func(c *lCard) {
		for len(c.Abilities[0].Targets) < kop.MaxTargets {
			c.Abilities[0].Targets = append(c.Abilities[0].Targets, lTarget{F: &lFilter{Players: plBoth}, Min: cst(1), Max: cst(1), Mode: -1})
		}
	})
	if !slices.ContainsFunc(lastSlot.refusals, func(r refusal) bool { return strings.Contains(r.Reason, "last target slot") }) {
		t.Errorf("a set in an ability using every slot was not refused: %+v", lastSlot.refusals)
	}
	ret := abLower(t, definedSetSrc("T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | Execute$ ProbeTrigBack",
		"SVar:ProbeTrigBack:DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield | Defined$ Self.YouOwn"), false)
	if len(ret.Refusals) != 0 || len(ret.Sets) != 1 {
		t.Fatalf("precondition: the return lowers one set, refusals %q", abReasons(ret))
	}
	rb := buildCard(newPoolTables(&lowered{cards: []*lCard{ret}}, nil), ret)
	if !slices.ContainsFunc(rb.refusals, func(r refusal) bool { return strings.Contains(r.Reason, "Defined$ set on kop Return") }) {
		t.Errorf("a set under OpReturn was not refused: %+v", rb.refusals)
	}
}

// definedSetPools are the committed lists TestDefinedSetKeepsOldLowerings
// holds: every fixture pool, and the FDN, BLB and MBC lists.
func definedSetPools(t *testing.T) []string {
	t.Helper()
	ents, err := os.ReadDir("../cardset/pools/fixtures")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range ents {
		if e.IsDir() {
			out = append(out, path.Join("fixtures", e.Name()))
		}
	}
	return append(out, "fdn", "blb", "mbc")
}

// TestDefinedSetKeepsOldLowerings: with the set grammar off (the
// lowering as it was before it existed) and on, every committed fixture pool
// and the FDN, BLB and MBC lists build byte-identical tables, except the pools
// whose cards newly lower through a set; and in those, every card that holds
// no set lowers to the same LIR block, so its evidence key stands.
// It compares grammar off against on, not against merge-base digests: the
// off switch stands in for the base. noSets does not change playerSetSpec's
// Player.OpponentOf arms (the alias's early return, every other referent's
// refusal), and both refuse with the grammar off as at the base (only the
// refusal text differs), so no lowering moves.
func TestDefinedSetKeepsOldLowerings(t *testing.T) {
	cardsDir := ratchetCards(t)
	moved := map[string]bool{"fixtures/definedset": true}
	const key = "definedset-test-key"
	reasons := 0
	for _, name := range definedSetPools(t) {
		lb, err := os.ReadFile(filepath.Join("../cardset/pools", name, "cards.txt"))
		if err != nil {
			t.Fatal(err)
		}
		names := parseCardList(lb)
		on, err := loadCardNames(slices.Clone(names), cardsDir)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		off, err := loadCardNames(slices.Clone(names), cardsDir)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		off.noSets = true
		tname := path.Base(name)
		dOn, _, _, errOn := tableOf(on, tname, nil)
		dOff, _, _, errOff := tableOf(off, tname, nil)
		if (errOn == nil) != (errOff == nil) {
			t.Fatalf("%s: build error on %v, off %v", name, errOn, errOff)
		}
		if errOn != nil {
			continue
		}
		if len(dOff.Sets) != 0 {
			t.Fatalf("%s: the grammar off still built %d sets", name, len(dOff.Sets))
		}
		narrowed := blankNarrowedReasons(dOn, dOff)
		if len(dOn.Sets) == 0 && !bytes.Equal(cardset.EncodeTable(dOn, key), cardset.EncodeTable(dOff, key)) {
			t.Errorf("%s: no card reads a set, yet the table differs (cards: %v)", name, diffDefs(dOn, dOff))
		}
		if len(dOn.Sets) != 0 {
			if !moved[name] {
				t.Logf("%s: %d sets (a card newly lowers)", name, len(dOn.Sets))
			}
			sameUnsetLIR(t, name, on, off)
		}
		reasons += narrowed
	}
	if reasons == 0 {
		t.Error("no refusal narrowed: the catch-all was never reached")
	}
	t.Logf("%d catch-all refusals narrowed", reasons)
}

// blankNarrowedReasons clears, in both tables, the reason of every refused
// entry whose reason the grammar narrowed: the off build's reason names a
// Defined$ value and the two differ. It returns how many it cleared.
func blankNarrowedReasons(on, off *cardset.PoolData) int {
	n := 0
	for i := range off.Reason {
		if i < len(on.Reason) && on.Reason[i] != off.Reason[i] && strings.Contains(off.Reason[i], `Defined$ "`) {
			on.Reason[i], off.Reason[i] = "", ""
			n++
		}
	}
	return n
}

// diffDefs names the entries whose status, reason or definition differ.
func diffDefs(on, off *cardset.PoolData) []string {
	var out []string
	for i := range off.Names {
		if i >= len(on.Names) || on.Status[i] != off.Status[i] || on.Reason[i] != off.Reason[i] ||
			!reflect.DeepEqual(on.Defs[i], off.Defs[i]) {
			out = append(out, off.Names[i])
		}
	}
	return out
}

// sameUnsetLIR holds every card of the pool that lowers with no set and no
// refusal to the same LIR block with the grammar on and off.
func sameUnsetLIR(t *testing.T, name string, on, off *pool) {
	t.Helper()
	lOn, lOff := lowerPool(on), lowerPool(off)
	byName := map[string]*lCard{}
	for _, c := range lOff.cards {
		byName[c.Name] = c
	}
	checked := 0
	for _, c := range lOn.cards {
		o := byName[c.Name]
		if o == nil || len(c.Sets) != 0 || len(c.Refusals) != 0 || len(o.Refusals) != 0 {
			continue
		}
		var a, b bytes.Buffer
		dumpLIR(&a, c)
		dumpLIR(&b, o)
		if a.String() != b.String() {
			t.Errorf("%s: %s lowers differently with the set grammar on", name, c.Name)
		}
		checked++
	}
	if checked == 0 {
		t.Errorf("%s: no unset card compared", name)
	}
}

// definedSetExpected are the corpus cards the set grammar lowers whole (the
// all-blockers census of 2026-11-27, every card with a SET line and no
// refusal; Screams from Within's Return lowers, and the builder refuses it).
var definedSetExpected = []string{"Bounty Board"}

// TestDefinedSetCardsLower: each expected card lowers with no refusal, and its
// built table reads a set through a RefSet subject.
func TestDefinedSetCardsLower(t *testing.T) {
	cardsDir := ratchetCards(t)
	p, err := loadCardNames(slices.Clone(definedSetExpected), cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	l := lowerPool(p)
	if len(l.cards) != len(definedSetExpected) {
		t.Fatalf("%d cards lowered, want %d", len(l.cards), len(definedSetExpected))
	}
	pt := newPoolTables(l, nil)
	for _, c := range l.cards {
		if len(c.Refusals) != 0 {
			t.Errorf("%s refused: %s", c.Name, abReasons(c))
			continue
		}
		b := buildCard(pt, c)
		if len(b.refusals) != 0 {
			t.Errorf("%s build refused: %+v", c.Name, b.refusals)
			continue
		}
		reads := 0
		for _, ab := range b.def.Abilities {
			for _, r := range ab.SetRefs {
				if ab.Prog[r.PC].Obj == kop.RefSet {
					reads++
				}
			}
		}
		if reads == 0 {
			t.Errorf("%s: no RefSet operand in its programs", c.Name)
		}
	}
}
