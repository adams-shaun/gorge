package neutral

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// TestTypeScopedCantBlockLeavesTheBlockerOut is the blocking half of Doomed
// Artisan's static: the Sculpture on the board is not a blocker, so the
// whole-declaration pool holds no pair for it.
func TestTypeScopedCantBlockLeavesTheBlockerOut(t *testing.T) {
	s := fixtureSession(t,
		"Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n",
		"Name:Fixture Keeper\nTypes:Creature Human\nPT:1/1\nS:Mode$ CantAttack,CantBlock | ValidCard$ Sculpture.YouCtrl | Description$ Fixture bar.\n",
		"Name:Fixture Statue\nTypes:Artifact Creature Sculpture\nPT:0/4\n")
	att := obsRef("o-att", "Fixture Bear", "p0")
	cur := obsRef("o-keeper", "Fixture Keeper", "p1")
	statue := obsPerm("o-statue", "Fixture Statue", "p1", untapped(), "artifact", "creature")
	statue.Characteristics.Subtypes = []string{"sculpture"}
	obs := testObservation("p1", "declare_blockers",
		[]protocol.ObjectRecord{obsPerm("o-att", "Fixture Bear", "p0", attackingAt("p1"), "creature")},
		[]protocol.ObjectRecord{obsPerm("o-keeper", "Fixture Keeper", "p1", untapped(), "creature"), statue})
	sd := protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(cur, &att)}}}
	p := translated(t, s, sd)
	if p.Decision.Kind != decision.KBlockers {
		t.Fatalf("decision kind %q, want blockers", p.Decision.Kind)
	}
	for _, o := range p.Decision.Options {
		if o.Obj == s.B.IDs.Of("o-statue") {
			t.Fatalf("the barred Sculpture has a block pair: %+v", p.Decision.Options)
		}
	}
}

// TestRequiredAttackerReadsOtherPermanents: a creature can be made to attack by
// an Aura on it or by a static on an opposing permanent. Neither is on the
// creature's own face.
func TestRequiredAttackerReadsOtherPermanents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
		attach bool
	}{
		{"aura", "Name:Fixture Curse\nK:Enchant:Creature\nTypes:Aura Enchantment\nS:Mode$ MustAttack | Description$ Fixture duty. | ValidCreature$ Creature.EnchantedBy\n", true},
		{"opposing static", "Name:Fixture Curse\nTypes:Enchantment\nS:Mode$ MustAttack | ValidCreature$ Creature.OppCtrl | Description$ Fixture duty.\n", false},
		{"type scope", "Name:Fixture Curse\nTypes:Enchantment\nS:Mode$ MustAttack | ValidCreature$ Creature.Bear+Other+OppCtrl | Description$ Fixture duty.\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := fixtureSession(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n", tc.source)
			bear := obsRef("o-bear", "Fixture Bear", "p0")
			curse := obsPerm("o-curse", "Fixture Curse", "p1", untapped(), "enchantment")
			if tc.attach {
				on := protocol.ObjectTarget(bear)
				curse.Permanent = &protocol.Permanent{AttachedTo: &on}
			}
			obs := testObservation("p0", "declare_attackers",
				[]protocol.ObjectRecord{obsPerm("o-bear", "Fixture Bear", "p0", untapped(), "creature")},
				[]protocol.ObjectRecord{curse})
			player := protocol.PlayerTarget("p1")
			sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepIndex: 0, SubstepCount: 1},
				Context: protocol.Context{Kind: "choice"}, Observation: obs,
				Candidates: []protocol.Candidate{{Semantic: protocol.DeclareAttack(bear, &player)}}}
			p := translated(t, s, sd)
			for _, o := range p.Decision.Options {
				if o.Kind == "attacker" && o.Obj == s.B.IDs.Of("o-bear") && o.Required {
					return
				}
			}
			t.Fatalf("the Bear is not a required attacker: %+v", p.Decision.Options)
		})
	}
}

// blockFixture poses the first block substep of a Fixture Pest attacking a
// Fixture Bear, the attacker's script being the test's.
func blockFixture(t *testing.T, pest string, keywords ...string) xview.Payload {
	t.Helper()
	s := fixtureSession(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n", pest)
	att := obsPerm("o-pest", "Fixture Pest", "p0", attackingAt("p1"), "creature")
	att.Characteristics.Keywords = keywords
	cur := obsRef("o-bear", "Fixture Bear", "p1")
	attRef := att.ObjectRef
	obs := testObservation("p1", "declare_blockers", []protocol.ObjectRecord{att},
		[]protocol.ObjectRecord{obsPerm("o-bear", "Fixture Bear", "p1", untapped(), "creature")})
	return translated(t, s, protocol.SeatDecision{ActingSeat: "p1", Group: protocol.Group{SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{{Semantic: protocol.DeclareBlock(cur, &attRef)}}})
}

// TestBlockOptionsPublishWhatNativeDoes: the bot declines its combat simulation
// on an option with a blocker bound, so the option has to publish the same
// bounds native does: an attacker's MinMaxBlocker Max$, and Menace as a floor of
// two.
func TestBlockOptionsPublishWhatNativeDoes(t *testing.T) {
	p := blockFixture(t, "Name:Fixture Pest\nTypes:Creature Pest\nPT:1/1\nS:Mode$ MinMaxBlocker | ValidCard$ Card.Self | Max$ 1 | Description$ Fixture bound.\n")
	if o := p.Decision.Options[0]; o.MinBlockers != 0 || o.MaxBlockers != 1 {
		t.Fatalf("Max$ 1 attacker: option bounds %d..%d, want 0..1", o.MinBlockers, o.MaxBlockers)
	}
	p = blockFixture(t, "Name:Fixture Pest\nTypes:Creature Pest\nPT:1/1\nK:Menace\n", "menace")
	if o := p.Decision.Options[0]; o.MinBlockers != 2 || o.MaxBlockers != 0 {
		t.Fatalf("Menace attacker: option bounds %d..%d, want 2..0", o.MinBlockers, o.MaxBlockers)
	}
}

// TestEnlistIsItsOwnAsk: the wire poses Enlist as an optional tap with a finish
// for the decline; gorge asks it as enlist options with the decline first. The
// tap arm of the bot would take the creature.
func TestEnlistIsItsOwnAsk(t *testing.T) {
	s := fixtureSession(t, "Name:Fixture Beast\nTypes:Creature Beast\nPT:4/5\nK:Enlist\n", "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n")
	beast, bear := obsRef("o-beast", "Fixture Beast", "p0"), obsRef("o-bear", "Fixture Bear", "p0")
	obs := testObservation("p0", "declare_attackers", []protocol.ObjectRecord{
		obsPerm("o-beast", "Fixture Beast", "p0", untapped(), "creature"),
		obsPerm("o-bear", "Fixture Bear", "p0", untapped(), "creature"),
	}, nil)
	choice := protocol.ObjectTarget(bear)
	sd := protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepCount: 1}, Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{
			{Semantic: protocol.SelectObject(&beast, "tap", choice, 0, 0, 1)},
			{Semantic: protocol.FinishSelection(&beast, "tap", 0)},
		}}
	p := translated(t, s, sd)
	if o := p.Decision.Options[0]; o.Kind != "enlist" || o.Obj != 0 {
		t.Fatalf("first option %+v, want the decline", o)
	}
	if o := p.Decision.Options[1]; o.Kind != "enlist" || o.Obj != s.B.IDs.Of("o-bear") {
		t.Fatalf("second option %+v, want the enlist of the Bear", o)
	}
	if op := p.Ops[1]; op.Op != "choose" || op.Option != 0 {
		t.Fatalf("finish translates to %+v, want the decline option", op)
	}
}

// TestHandDiscardsKeepNativeKinds: an effect's discard is gorge's modal pick
// even when the wire names no source, and a cleanup discard stays a choose. A
// hand-origin Hidden$ ChangeZone is a hand_move, not the generic exile arm.
func TestHandDiscardsKeepNativeKinds(t *testing.T) {
	ask := func(s *Session, step string, src *protocol.ObjectRef, purpose string) xview.Payload {
		card := obsRef("o-card", "Fixture Bear", "p0")
		card.Zone = "hand"
		obs := testObservation("p0", step, nil, nil)
		obs.Players[0].Hand = []protocol.ObjectRecord{{ObjectRef: card}}
		return translated(t, s, protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepCount: 1},
			Context: protocol.Context{Kind: "choice"}, Observation: obs,
			Candidates: []protocol.Candidate{{Semantic: protocol.SelectObject(src, purpose, protocol.ObjectTarget(card), 0, 1, 1)}}})
	}
	bear := "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n"
	if p := ask(fixtureSession(t, bear), "declare_attackers", nil, "discard"); p.Decision.Kind != decision.KModes {
		t.Fatalf("an effect's sourceless discard is kind %q, want modes", p.Decision.Kind)
	}
	if p := ask(fixtureSession(t, bear), "cleanup", nil, "discard"); p.Decision.Kind != decision.KChoose {
		t.Fatalf("a cleanup discard is kind %q, want choose", p.Decision.Kind)
	}
	s := fixtureSession(t, bear, "Name:Fixture Taker\nTypes:Sorcery\nA:SP$ ChangeZone | Origin$ Hand | Destination$ Exile | Hidden$ True | SpellDescription$ Fixture take.\n")
	taker := obsRef("o-taker", "Fixture Taker", "p1")
	taker.Zone = "stack"
	card := obsRef("o-card", "Fixture Bear", "p0")
	card.Zone = "hand"
	o := testObservation("p0", "precombat_main", nil, nil)
	o.Players[0].Hand = []protocol.ObjectRecord{{ObjectRef: card}}
	o.Stack = []protocol.StackEntry{{ObjectRef: taker, StackKind: "spell"}}
	p := translated(t, s, protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: o,
		Candidates: []protocol.Candidate{{Semantic: protocol.SelectObject(&taker, "exile", protocol.ObjectTarget(card), 0, 1, 1)}}})
	if k := p.Decision.Options[0].Kind; k != "hand_move" {
		t.Fatalf("a hand-origin Hidden$ ChangeZone offers %q, want hand_move", k)
	}
}

// TestEffectOptionalIsMarked: an emblem's "you may" trigger is gorge's
// EffectOptional election, which the bot declines. The wire says only that a
// trigger of the walker is optional.
func TestEffectOptionalIsMarked(t *testing.T) {
	s := fixtureSession(t, "Name:Fixture Walker\nManaCost:2 G\nTypes:Legendary Planeswalker Fixture\nLoyalty:3\n"+
		"A:AB$ Effect | Cost$ SubCounter<3/LOYALTY> | Planeswalker$ True | Triggers$ Later | Duration$ Permanent | SpellDescription$ Fixture emblem.\n"+
		"SVar:Later:Mode$ Phase | Phase$ End of Turn | ValidPlayer$ You | Execute$ Body | OptionalDecider$ You | TriggerDescription$ Fixture may.\n"+
		"SVar:Body:DB$ Draw | NumCards$ 1\n")
	src := obsRef("o-trigger", "Fixture Walker", "p0")
	src.Zone = "stack"
	obs := testObservation("p0", "end_step", nil, nil)
	obs.Stack = []protocol.StackEntry{{ObjectRef: src, StackKind: "triggered_ability"}}
	p := translated(t, s, protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: obs,
		Candidates: []protocol.Candidate{
			{Semantic: protocol.ChooseBoolean(&src, "optional_trigger", true)},
			{Semantic: protocol.ChooseBoolean(&src, "optional_trigger", false)},
		}})
	if p.Decision.Kind != decision.KTriggerOptional || !p.Facts.EffectOptional {
		t.Fatalf("decision kind %q, EffectOptional %v", p.Decision.Kind, p.Facts.EffectOptional)
	}
}

// TestDestinationChoiceIsItsOwnAsk: the wire poses a DestinationAlternative$ as
// a labelled option; gorge's ask names the alternate, which the bot takes.
func TestDestinationChoiceIsItsOwnAsk(t *testing.T) {
	s := fixtureSession(t, "Name:Fixture Bear\nTypes:Creature Bear\nPT:2/2\n")
	src := obsRef("o-src", "Fixture Bear", "p0")
	label := "destination"
	opt := func(i uint32) protocol.Candidate {
		return protocol.Candidate{Semantic: protocol.ChooseOption(&src, "effect_option", i, 2, &label)}
	}
	p := translated(t, s, protocol.SeatDecision{ActingSeat: "p0", Group: protocol.Group{SubstepCount: 1},
		Context: protocol.Context{Kind: "choice"}, Observation: testObservation("p0", "precombat_main", nil, nil),
		Candidates: []protocol.Candidate{opt(0), opt(1)}})
	if o := p.Decision.Options[1]; p.Decision.Kind != decision.KChoose || o.Kind != "destination" || o.Mode != "alternate" {
		t.Fatalf("decision kind %q, second option %+v", p.Decision.Kind, o)
	}
}
