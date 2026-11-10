// A granted K:Equip (The Irencrag's Everflame, Heroes' Legacy: an Animate
// grants `Keywords$ Equip:3`). The grant lands in the derived keyword list;
// the printed expansion never sees it, so the equip ability was never
// offered. The granted body is the printed expander's own output
// (cards.GrantedKeywordAbility), so the offer is priced, timed and targeted
// like a printed Equip.
package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestGrantedEquipOffersEquip(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const relic = "Name:Everflame Stand-In\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Animate | Cost$ 0 | Defined$ Self | Keywords$ Equip:3 | Duration$ Permanent | SpellDescription$ Gain equip.\n" +
		"Oracle:x\n"
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{card(t, relic), bears}, []*cards.Card{bears})
	relicID := moveByName(t, e, 0, "Everflame Stand-In", state.ZBattlefield)
	mine := moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	theirs := moveByName(t, e, 1, "Grizzly Bears", state.ZBattlefield)

	// Without the grant there is no equip offer (and the relic prints none).
	if e.G.Obj(relicID).Face().HasKeyword("Equip") {
		t.Fatal("precondition: the artifact prints Equip, so the offer is not the granted one")
	}
	e.pending = nil
	e.priorityRound()
	if _, ok := grantedKeywordOptionOK(e, relicID, "Equip:3"); ok {
		t.Fatal("equip offered before the Animate grant resolved")
	}
	animate := -1
	for _, o := range e.Pending().Options {
		if o.Kind == "ability" && o.Obj == relicID && o.Keyword == "" && o.Ability >= 0 {
			animate = o.Index
		}
	}
	if animate < 0 {
		t.Fatalf("precondition: the Animate ability is not offered: %+v", e.Pending().Options)
	}
	submitChoices(t, e, animate)
	passUntilStackEmpty(t, e, 20)
	if param, ok := e.derivedKeywordParam(relicID, "Equip"); !ok || param != "3" {
		t.Fatalf("precondition: derived Equip grant = (%q, %v), want (\"3\", true)", param, ok)
	}

	addMana(t, e, 0, "RRR")
	e.pending = nil
	e.priorityRound()
	opt, ok := grantedKeywordOptionOK(e, relicID, "Equip:3")
	if !ok {
		t.Fatalf("the granted Equip:3 is not offered: %+v", e.Pending().Options)
	}
	if !opt.Attach {
		t.Fatal("the granted equip option is not flagged as an attach ability")
	}
	ab := cards.GrantedKeywordAbility("Equip:3")
	if ab.ParamStr(cards.PKSorcerySpeed) != "True" || ab.ParamStr(cards.PKValidTgts) != "Creature.YouCtrl" {
		t.Fatalf("granted body = %v, want sorcery-speed Attach to Creature.YouCtrl", ab.Params)
	}

	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the equip target ask", d)
	}
	var mineIdx = -1
	for _, o := range d.Options {
		if o.Obj == theirs {
			t.Fatalf("equip offers the opponent's creature: %+v", d.Options)
		}
		if o.Obj == mine {
			mineIdx = o.Index
		}
	}
	if mineIdx < 0 {
		t.Fatalf("equip does not offer the controller's creature: %+v", d.Options)
	}
	submitChoices(t, e, mineIdx)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(relicID).AttachedTo; got != mine {
		t.Fatalf("relic attached to %d, want the controller's creature %d", got, mine)
	}
	if e.G.Players[0].Pool.Total() != 0 {
		t.Fatalf("equip left mana in the pool; {3} was not charged")
	}
}

// The Irencrag's Animate removes all other abilities AND grants Equip:3 in one
// effect. CR 613.1f keeps the granted keyword (it is the later layer-6 grant),
// so the equip must be offered although the printed mana ability is gone.
func TestIrencragEverflameOffersGrantedEquip(t *testing.T) {
	t.Parallel()
	crag := tokenReplCorpusCard(t, "The Irencrag")
	legend := cardByName(t, "Name:Grant Test Legend\nTypes:Legendary Creature Human\nPT:2/2\nOracle:x\n")
	e, cfg := tokenReplGame(t, 9, crag, legend)
	cragID := moveSeededCard(t, e, 0, crag, state.ZBattlefield)
	legendID := moveSeededCard(t, e, 0, legend, state.ZBattlefield)
	addMana(t, e, 0, "")
	for i := 0; i < 40; i++ {
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		if d.Kind == decision.KPriority {
			if len(e.G.Stack) == 0 {
				break
			}
			passOnce(t, e)
			continue
		}
		idx := d.Options[0].Index
		for _, o := range d.Options {
			if strings.EqualFold(o.Kind, "yes") || strings.EqualFold(o.Label, "yes") {
				idx = o.Index
			}
		}
		submitChoices(t, e, idx)
	}
	if got := e.Name(cragID); got != "Everflame, Heroes' Legacy" {
		t.Fatalf("precondition: The Irencrag did not become Everflame (name %q)", got)
	}
	if !e.printedAbilitiesLost(e.G.Obj(cragID)) {
		t.Fatal("precondition: Everflame has not lost its printed abilities")
	}
	if param, ok := e.derivedKeywordParam(cragID, "Equip"); !ok || param != "3" {
		t.Fatalf("precondition: derived Equip = (%q, %v), want (\"3\", true)", param, ok)
	}
	if p := e.Power(legendID); p != 2 {
		t.Fatalf("precondition: the unequipped legend is %d power, want 2", p)
	}
	// Only the granted keyword escapes the removal: no printed mana ability.
	if got := e.availableManaAbilitiesUsing(&actionStaticSource{e: e}, 0, cragID); len(got) != 0 {
		t.Fatalf("Everflame still has %d mana abilities after losing all abilities", len(got))
	}

	addMana(t, e, 0, "BBB")
	e.pending = nil
	e.priorityRound()
	opt, ok := grantedKeywordOptionOK(e, cragID, "Equip:3")
	if !ok {
		t.Fatalf("the granted Equip:3 is not offered: %+v", e.Pending().Options)
	}
	if !opt.Attach {
		t.Fatal("the granted equip option is not flagged as an attach ability")
	}
	for _, o := range e.Pending().Options {
		if o.Obj == cragID && o.Keyword == "" {
			t.Fatalf("a printed ability of Everflame is offered: %+v", o)
		}
	}
	submitChoices(t, e, opt.Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want the equip target ask", d)
	}
	tgt := -1
	for _, o := range d.Options {
		if o.Obj == legendID {
			tgt = o.Index
		}
	}
	if tgt < 0 {
		t.Fatalf("equip does not offer the legend: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Obj(cragID).AttachedTo; got != legendID {
		t.Fatalf("Everflame attached to %d, want the legend %d", got, legendID)
	}
	if p, tg := e.Power(legendID), e.Toughness(legendID); p != 5 || tg != 5 {
		t.Fatalf("equipped legend is %d/%d, want 5/5", p, tg)
	}
	replayCheck(t, e, cfg)
}

// A grant older than a RemoveAllAbilities$ effect is gone from the derived
// list, so the fix (which reads the derived list) must not offer it.
func TestGrantedEquipLostWithOlderRemoval(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const relic = "Name:Equip Then Strip\nManaCost:3\nTypes:Artifact\n" +
		"A:AB$ Animate | Cost$ 0 | Defined$ Self | Keywords$ Equip:3 | Duration$ Permanent | SpellDescription$ Gain equip.\n" +
		"A:AB$ Animate | Cost$ 0 | Defined$ Self | RemoveAllAbilities$ True | Duration$ Permanent | SpellDescription$ Lose all abilities.\n" +
		"Oracle:x\n"
	bears := lookup(t, reg, "Grizzly Bears")
	e := corpusEngine(t, reg, []*cards.Card{card(t, relic), bears}, []*cards.Card{bears})
	relicID := moveByName(t, e, 0, "Equip Then Strip", state.ZBattlefield)
	moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	for ab := 0; ab < 2; ab++ {
		e.pending = nil
		e.priorityRound()
		submitChoices(t, e, abilityOption(t, e, relicID, ab).Index)
		passUntilStackEmpty(t, e, 20)
	}
	if !e.printedAbilitiesLost(e.G.Obj(relicID)) {
		t.Fatal("precondition: the relic has not lost its abilities")
	}
	if param, ok := e.derivedKeywordParam(relicID, "Equip"); ok {
		t.Fatalf("precondition: the older Equip grant survived the removal (%q)", param)
	}
	addMana(t, e, 0, "RRR")
	e.pending = nil
	e.priorityRound()
	if _, ok := grantedKeywordOptionOK(e, relicID, "Equip:3"); ok {
		t.Fatalf("Equip:3 offered although a later effect removed it: %+v", e.Pending().Options)
	}
}

// TestGrantedKeywordHeadSetsAgree pins the rules-side head gate
// (grantedKWHeads, the precheck that skips the layer walk) to the heads
// cards.GrantedKeywordAbility synthesizes, so a head added on one side cannot
// be silently skipped on the other.
func TestGrantedKeywordHeadSetsAgree(t *testing.T) {
	t.Parallel()
	have := map[string]bool{}
	for _, h := range grantedKWHeads {
		have[h.S] = true
	}
	names := cards.GrantedKeywordHeadNames()
	if len(have) != len(names) {
		t.Fatalf("grantedKWHeads has %d heads, cards.GrantedKeywordHeadNames %d", len(have), len(names))
	}
	for _, n := range names {
		if !have[n] {
			t.Errorf("cards synthesizes %q but grantedKWHeads omits it", n)
		}
	}
}
