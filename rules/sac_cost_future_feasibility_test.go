package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestDargoSacChoicePreservesLaterArtifactPart(t *testing.T) {
	e, spell, ids := dargoEngine(t, []string{
		"Name:Anvil\nTypes:Artifact\nOracle:x\n",
		"Name:Crab\nTypes:Creature\nPT:1/1\nOracle:x\n",
	}, "RRRRR")
	var artifact, creature state.ObjID
	for _, id := range ids {
		o := e.G.Obj(id)
		if o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: candidate %d in %s, want battlefield", id, o.Zone)
		}
		if strings.Contains(strings.Join(o.Face().Types, " "), "Artifact") {
			artifact = id
		}
		if strings.Contains(strings.Join(o.Face().Types, " "), "Creature") {
			creature = id
		}
	}
	if artifact == 0 || creature == 0 || artifact == creature {
		t.Fatalf("precondition: distinct artifact and creature required: artifact=%d creature=%d", artifact, creature)
	}
	base := withSpellAbilityExtras(e.G.Obj(spell).Face(), e.castOfferBase(0, spell))
	extra := ParseCost("Sac<X/Artifact>")
	if len(base.Sac) != 1 || len(extra.Sac) != 1 || base.Sac[0].Spec == extra.Sac[0].Spec {
		t.Fatalf("precondition: expected different broad and artifact-only Sac parts: base=%+v extra=%+v", base.Sac, extra.Sac)
	}
	pc := &pendingCast{player: 0, card: spell, from: state.ZHand, ability: -1, cost: base.Plus(extra), x: 1, xDone: true}
	e.cast = pc
	if !e.sacAsk() {
		t.Fatal("sacrifice choice did not suspend")
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("pending = %+v, want one-sacrifice choice", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != creature {
		t.Fatalf("first sacrifice options = %+v, want only creature %d (artifact %d must remain for next part)", d.Options, creature, artifact)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
		t.Fatalf("the feasible sacrifice answer does not validate: %v", err)
	}
	submitChoices(t, e, d.Options[0].Index)
	next := e.Pending()
	if next == nil || next.Kind != decision.KChoose || len(next.Options) != 1 || next.Options[0].Obj != artifact {
		t.Fatalf("second cast sacrifice choice = %+v, want the reserved artifact %d", next, artifact)
	}
	submitChoices(t, e, next.Options[0].Index)
	if e.G.Obj(artifact).Zone != state.ZGraveyard || e.G.Obj(creature).Zone != state.ZGraveyard || e.G.Obj(spell).Zone != state.ZStack {
		t.Fatalf("cast did not complete after feasible answers: spell=%s artifact=%s creature=%s", e.G.Obj(spell).Zone, e.G.Obj(artifact).Zone, e.G.Obj(creature).Zone)
	}
}

func TestManaAbilitySacChoicePreservesLaterArtifactPart(t *testing.T) {
	outlet := "Name:Outlet\nTypes:Enchantment\n" +
		"A:AB$ Mana | Cost$ Sac<1/Artifact;Creature/artifact or creature> Sac<1/Artifact> | Produced$ R | SpellDescription$ Add {R}.\nOracle:x\n"
	artifactCreature := "Name:Artifact Crab\nTypes:Artifact Creature\nPT:1/1\nOracle:x\n"
	creatureScript := "Name:Crab\nTypes:Creature\nPT:1/1\nOracle:x\n"
	e, _, _ := newFixtureDeck(t, 82, outlet, artifactCreature, creatureScript)
	source := moveSeeded(t, e, 0, outlet, state.ZBattlefield)
	artifact := putCreature(t, e, 0, artifactCreature)
	creature := putCreature(t, e, 0, creatureScript)
	for _, id := range []state.ObjID{source, artifact, creature} {
		if e.G.Obj(id).Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d is in %s, want battlefield", id, e.G.Obj(id).Zone)
		}
	}
	if artifact == creature || !strings.Contains(strings.Join(e.G.Obj(artifact).Face().Types, " "), "Artifact") || !strings.Contains(strings.Join(e.G.Obj(artifact).Face().Types, " "), "Creature") || !strings.Contains(strings.Join(e.G.Obj(creature).Face().Types, " "), "Creature") || strings.Contains(strings.Join(e.G.Obj(creature).Face().Types, " "), "Artifact") {
		t.Fatalf("precondition: distinct artifact creature and ordinary creature required: %d %d", artifact, creature)
	}
	e.pending = nil
	e.Advance()
	if len(e.G.Obj(source).Face().Abilities) == 0 {
		t.Fatalf("precondition: fixture did not compile mana ability: face=%+v", e.G.Obj(source).Face())
	}
	ability := activateOption(t, e, source)
	submitChoices(t, e, ability)
	first := e.Pending()
	if first == nil || first.Kind != decision.KChoose {
		t.Fatalf("precondition: mana ability must ask for its broad first sacrifice, pending=%+v", first)
	}
	if len(first.Options) != 1 || first.Options[0].Obj != creature {
		t.Fatalf("first mana sacrifice options = %+v, want only creature %d while retaining artifact %d", first.Options, creature, artifact)
	}
	if err := first.Validate(decision.Intent{Seq: first.Seq, Player: first.Player, Choices: []int{first.Options[0].Index}}); err != nil {
		t.Fatalf("the feasible mana sacrifice answer does not validate: %v", err)
	}
	submitChoices(t, e, first.Options[0].Index)
	if e.G.Obj(artifact).Zone != state.ZGraveyard || e.G.Obj(creature).Zone != state.ZGraveyard {
		t.Fatalf("mana activation did not settle both distinct sacrifices: pending=%+v artifact=%s creature=%s", e.Pending(), e.G.Obj(artifact).Zone, e.G.Obj(creature).Zone)
	}
	if e.G.Players[0].Pool[state.MR] != 1 {
		t.Fatalf("mana pool = %+v, want the activation's red mana", e.G.Players[0].Pool)
	}
}
