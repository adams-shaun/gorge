package gorgebe

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
	"github.com/adams-shaun/gorge/engine/cards"
)

func mcProg(steps ...*ir.Step) *ir.Program { return &ir.Program{Steps: steps} }

func mcTokenCard(name, pt string, kws ...string) *cards.Card {
	return &cards.Card{Faces: []*cards.Face{{Name: name, Types: []string{"Creature", "Wurm"}, PT: pt, Keywords: kws}}}
}

func TestModesStep(t *testing.T) {
	gain := apStep(ir.VerbGainLife, func(s *ir.Step) { s.N = refTestVal(4) })
	draw := apStep(ir.VerbDraw, func(s *ir.Step) { s.N = refTestVal(1) })
	cm := func(mod func(*ir.Step)) *ir.Step {
		return apStep(ir.VerbChooseModes, func(s *ir.Step) {
			s.N = refTestVal(1)
			s.Modes = []*ir.Program{mcProg(gain), mcProg(draw)}
			if mod != nil {
				mod(s)
			}
		})
	}
	n, p, err := apRun(nil, nil, cm(nil))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "Charm", kvs("Choices", "DBGainLife,DBDraw"))
	if got := apSVar(p, "DBGainLife"); got != "DB$ GainLife | LifeAmount$ 4" {
		t.Errorf("mode SVar %q", got)
	}
	n, _, err = apRun(nil, nil, cm(func(s *ir.Step) { s.N = refTestVal(2) }))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "Charm", kvs("Choices", "DBGainLife,DBDraw", "CharmNum", "2"))
	_, _, err = apRun(nil, nil, cm(func(s *ir.Step) { s.Optional = true }))
	refTestUnprintable(t, err, "verb:choose-modes")
	_, _, err = apRun(nil, nil, cm(func(s *ir.Step) { s.Modes = s.Modes[:1] }))
	refTestUnprintable(t, err, "verb:choose-modes")
}

func TestDoubleCountersAndFight(t *testing.T) {
	n, _, err := apRun(nil, nil, apStep(ir.VerbDoubleCounters, func(s *ir.Step) { s.Obj = apSelf; s.Word = "P1P1" }))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "MultiplyCounter", kvs("Defined", "Self", "CounterType", "P1P1"))
	_, _, err = apRun(nil, nil, apStep(ir.VerbDoubleCounters, func(s *ir.Step) { s.Obj = apSelf }))
	refTestUnprintable(t, err, "verb:double-counters")

	tg := []*ir.Target{refTestObjTarget()}
	fight := apStep(ir.VerbFight, func(s *ir.Step) { s.Obj = apTgt0; s.Source = apSelf; s.Optional = true })
	n, _, err = apRun(nil, tg, fight)
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := n.get("Defined"); v != "Self" {
		t.Errorf("Defined %q", v)
	}
	if _, ok := n.get("ValidTgts"); !ok {
		t.Errorf("no ValidTgts: %v", n.Params)
	}
	if v, _ := n.get("OptionalDecider"); v != "You" {
		t.Errorf("OptionalDecider %q", v)
	}
	_, _, err = apRun(nil, nil, apStep(ir.VerbFight, func(s *ir.Step) { s.Obj = apSelf; s.Source = apSelf }))
	refTestUnprintable(t, err, "verb:fight")
}

func TestTokenCopyAndSpellCopy(t *testing.T) {
	haste := &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: "Haste"}}}}
	sac := &ir.Effect{Kind: ir.EffAddAbility, Abilities: []*ir.Ability{{
		Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvEndStep, Subject: refTestPred(ir.SubjPlayer, ir.PAny, nil)},
		Body: mcProg(apStep(ir.VerbSacrifice, func(s *ir.Step) { s.Obj = apSelf })),
	}}}
	n, _, err := apRun(nil, []*ir.Target{refTestObjTarget()}, apStep(ir.VerbCreateTokenCopy, func(s *ir.Step) {
		s.Obj = apTgt0
		s.N = refTestVal(1)
		s.Effects = []*ir.Effect{haste, sac}
	}))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := n.get("AddKeywords"); v != "Haste" {
		t.Errorf("AddKeywords %q", v)
	}
	if v, _ := n.get("AtEOTTrig"); v != "Sacrifice" {
		t.Errorf("AtEOTTrig %q", v)
	}
	n, _, err = apRun(nil, nil, apStep(ir.VerbCreateTokenCopy, func(s *ir.Step) { s.Obj = apSelf; s.N = refTestVal(1) }))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "CopyPermanent", kvs("Defined", "Self"))

	n, _, err = apRun(nil, nil, apStep(ir.VerbCopySpell, func(s *ir.Step) { s.Obj = apSelf; s.Who = apYou }))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "CopySpellAbility", kvs("Defined", "Parent", "Controller", "You", "MayChooseTarget", "True"))
	p := &pr{ev: ir.EvCast}
	n, _, err = printCopySpell(p, []*ir.Step{{Verb: ir.VerbCopySpell, Obj: &ir.Ref{Kind: ir.RefEventObject}, N: refTestVal(3)}})
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "CopySpellAbility", kvs("Defined", "TriggeredSpellAbility", "Amount", "3", "MayChooseTarget", "True"))
}

func TestDelayedTriggerShapes(t *testing.T) {
	exile := apStep(ir.VerbExile, func(s *ir.Step) { s.Obj = apTgt0 })
	back := &ir.Ability{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvNextEndStep}, Body: mcProg(apStep(ir.VerbMove, func(s *ir.Step) {
		s.Obj = &ir.Ref{Kind: ir.RefReceipt, Receipt: ir.RcExiled}
		s.To = ir.ZoneBattlefield
		s.Enter = &ir.Enter{UnderOwner: true}
	}))}
	n, p, err := apRun(nil, []*ir.Target{refTestObjTarget()}, exile, apStep(ir.VerbDelayedTrigger, func(s *ir.Step) { s.Abilities = []*ir.Ability{back} }))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := n.get("RememberChanged"); v != "True" {
		t.Errorf("exile does not remember: %v", n.Params)
	}
	d := n.Sub
	if d == nil || d.API != "DelayedTrigger" {
		t.Fatalf("chain %+v", n)
	}
	if v, _ := d.get("Mode"); v != "Phase" {
		t.Errorf("Mode %q", v)
	}
	if got := apSVar(p, "TrigReturn"); got != "DB$ ChangeZone | Origin$ Exile | Destination$ Battlefield | Defined$ DelayTriggerRememberedLKI" {
		t.Errorf("body %q", got)
	}
	if d.Sub == nil || d.Sub.API != "Cleanup" {
		t.Errorf("no cleanup after the delayed trigger")
	}
	_, _, err = apRun(nil, nil, apStep(ir.VerbDelayedTrigger, func(s *ir.Step) { s.Abilities = []*ir.Ability{back} }))
	refTestUnprintable(t, err, "verb:delayed-trigger-first")

	// A reflexive trigger after two tokens fires once per pair.
	tok := &ir.Face{Name: "Faerie", Types: []string{"Creature"}, Subtypes: []string{"Faerie"}, Power: &ir.PT{N: 1}, Toughness: &ir.PT{N: 1}}
	toks := map[string]*cards.Card{"x": {Faces: []*cards.Face{{Name: "Faerie", Types: []string{"Creature", "Faerie"}, PT: "1/1"}}}}
	refl := &ir.Ability{Kind: ir.AbTriggered, Trigger: &ir.Trigger{Event: ir.EvReflexive}, Body: mcProg(apStep(ir.VerbTap, func(s *ir.Step) { s.Obj = apSelf }))}
	n, _, err = apRun(toks, nil, apStep(ir.VerbCreateToken, func(s *ir.Step) { s.N = refTestVal(2); s.Token = tok }),
		apStep(ir.VerbDelayedTrigger, func(s *ir.Step) { s.Abilities = []*ir.Ability{refl} }))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := n.get("RememberOriginalTokens"); v != "True" {
		t.Errorf("tokens not remembered: %v", n.Params)
	}
	if v, _ := n.Sub.get("TriggerAmount"); v != "Remembered$Amount/DivideEvenlyDown.2" {
		t.Errorf("TriggerAmount %q", v)
	}
}

func TestTokenRunsAndAbilities(t *testing.T) {
	toks := map[string]*cards.Card{
		"a": mcTokenCard("Wurm Token", "3/3", "Deathtouch"),
		"b": mcTokenCard("Wurm Token", "3/3", "Lifelink"),
	}
	face := func(kw string) *ir.Face {
		return &ir.Face{Name: "Wurm Token", Types: []string{"Creature"}, Subtypes: []string{"Wurm"}, Power: &ir.PT{N: 3}, Toughness: &ir.PT{N: 3},
			Abilities: []*ir.Ability{{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: kw}}}}
	}
	mk := func(kw string) *ir.Step {
		return apStep(ir.VerbCreateToken, func(s *ir.Step) { s.N = refTestVal(1); s.Token = face(kw) })
	}
	n, _, err := apRun(toks, nil, mk("Deathtouch"), mk("Lifelink"))
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := n.get("TokenScript"); v != "a,b" || n.Sub != nil {
		t.Errorf("run printed %v (sub %v)", n.Params, n.Sub)
	}
	diff := mk("Lifelink")
	diff.N = refTestVal(2)
	_, _, err = apRun(toks, nil, mk("Deathtouch"), diff)
	refTestUnprintable(t, err, "shape:consecutive-tokens")
}

func TestChooseNumberAndEachColor(t *testing.T) {
	n, _, err := apRun(nil, nil, apStep(ir.VerbChooseNumber, func(s *ir.Step) { s.N = refTestVal(0) }))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "ChooseNumber", kvs("Defined", "You"))
	_, _, err = apRun(nil, nil, apStep(ir.VerbChooseNumber, func(s *ir.Step) { s.N = refTestVal(5) }))
	refTestUnprintable(t, err, "verb:choose-number")

	f := refTestAll(ir.SubjPermanent, refTestPerm(ir.PAny, nil), refTestRefPred(ir.SubjPermanent, ir.PControlledBy, refTestAllPlayers(refTestYou())))
	n, _, err = apRun(nil, nil, apStep(ir.VerbAddMana, func(s *ir.Step) { s.Word = "each-color-among"; s.Filter = f }))
	if err != nil {
		t.Fatal(err)
	}
	apWant(t, n, "Mana", kvs("Produced", "Special EachColorAmong_Valid Permanent.YouCtrl"))
}

func TestTokenReplacementDoubling(t *testing.T) {
	those := func() *ir.Step { return &ir.Step{Verb: ir.VerbCreateToken, Obj: &ir.Ref{Kind: ir.RefEventObject}} }
	repl := func(steps ...*ir.Step) *ir.Ability {
		return &ir.Ability{Kind: ir.AbReplacement, Body: mcProg(&ir.Step{Verb: ir.VerbDo, Then: mcProg(steps...)}), Trigger: &ir.Trigger{
			Event: ir.EvWouldCreateTokens, Subject: refTestPred(ir.SubjPermanent, ir.PAny, nil), Object: refTestYou(),
		}}
	}
	p := &pr{}
	if err := printReplacement(p, repl(those(), those())); err != nil {
		t.Fatal(err)
	}
	if len(p.lines) != 1 || p.lines[0] != "R:Event$ CreateToken | ActiveZones$ Battlefield | ValidToken$ Card.YouCtrl | ReplaceWith$ DBReplaceToken | EffectOnly$ True" {
		t.Errorf("lines %q", p.lines)
	}
	if got := apSVar(p, "DBReplaceToken"); got != "DB$ ReplaceToken | Type$ Amount" {
		t.Errorf("SVar %q", got)
	}
	err := printReplacement(&pr{}, repl(those()))
	refTestUnprintable(t, err, "verb:do")
}
