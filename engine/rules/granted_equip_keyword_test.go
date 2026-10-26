// A granted K:Equip (The Irencrag's Everflame, Heroes' Legacy: an Animate
// grants `Keywords$ Equip:3`). The grant lands in the derived keyword list;
// the printed expansion never sees it, so the equip ability was never
// offered. The granted body is the printed expander's own output
// (cards.GrantedKeywordAbility), so the offer is priced, timed and targeted
// like a printed Equip.
package rules

import (
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
