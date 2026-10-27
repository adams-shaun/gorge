package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g241: a permanent that enters as a copy also gets the copied permanent's
// own "as enters, choose ..." elections (CR 707.2, 614.12), answered by the
// entering object's controller and recorded on the entering object.

// copyEntryScenario casts p0's template on p0's turn (answering tmplAnswer),
// then p1 casts a copier on p1's turn and answers copyAnswers in order. The
// strict runner fails on any unconsumed answer, so a copier that does not pose
// the copied choice fails here.
func copyEntryScenario(template, tmplMana, tmplAnswer, copier, copierMana, after string, copyAnswers ...string) string {
	picks := make([]string, 0, len(copyAnswers))
	for _, a := range copyAnswers {
		picks = append(picks, `{"kind":"choose","pick":["`+a+`"]}`)
	}
	return `{"name":"cr-g241","cr":["707.2","614.12"],"why":"cr-g241","setup":{` +
		`"p0":{"hand":["` + template + `"]},"p1":{"hand":["` + copier + `","Ornithopter"],"library":["Forest","Forest"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:` + template + `","mana":"` + tmplMana + `"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["` + tmplAnswer + `"]}]},` +
		`{"op":"pass_to","seat":0,"step":"upkeep","active":"p1"},{"op":"pass_to","seat":1,"step":"main1","active":"p1"},` +
		`{"op":"cast","seat":1,"card":"p1:` + copier + `","mana":"` + copierMana + `"},` +
		`{"op":"resolve","seat":1,"answers":[` + strings.Join(picks, ",") + `]}` + after + `],"expect":[]}`
}

// ornithopterAfter casts p1's Ornithopter after the copy entered.
const ornithopterAfter = `,{"op":"cast","seat":1,"card":"p1:Ornithopter","mana":""},{"op":"resolve","seat":1}`

func TestCloneCopyOfChooseTypePosesTypeAsk(t *testing.T) {
	t.Parallel()
	// The original's answer (Elf) differs from the copy's (Thopter), so the
	// draw below can only come from the copy's own recorded type.
	run := runETBCloneScenario(t, copyEntryScenario("Kindred Discovery", "CCCUU", "Elf",
		"Clever Impersonator", "CCUU", ornithopterAfter, "p0:Kindred Discovery", "Thopter"))
	e := run.e
	orig := findOn(t, e, state.ZBattlefield, 0, "Kindred Discovery")
	cp := findOn(t, e, state.ZBattlefield, 1, "Kindred Discovery")
	if got := e.G.Obj(orig).ChosenType; got != "Elf" {
		t.Fatalf("precondition: original chose %q, want Elf", got)
	}
	if got := e.G.Obj(cp).ChosenType; got != "Thopter" {
		t.Fatalf("copy chose %q, want its own answer Thopter", got)
	}
	findOn(t, e, state.ZBattlefield, 1, "Ornithopter")
	// p1's hand is the Forest drawn for the turn plus the Forest the copied
	// trigger drew for the Ornithopter (the decline test pins the 1-card case).
	if h := e.G.Zone(state.ZHand, 1); len(h) != 2 {
		t.Fatalf("p1 hand %v, want 2 Forests (turn draw + the copied trigger's draw)", h)
	}
	replayCheck(t, e, run.cfg)
}

func TestCloneCopyDeclinedAsksNoTemplateChoice(t *testing.T) {
	t.Parallel()
	run := runETBCloneScenario(t, copyEntryScenario("Kindred Discovery", "CCCUU", "Elf",
		"Clever Impersonator", "CCUU", ornithopterAfter, "Enter as itself"))
	e := run.e
	// The 0/0 Impersonator entered as itself and died: its graveyard card
	// carries no chosen type and the election asked nothing further.
	self := findOn(t, e, state.ZGraveyard, 1, "Clever Impersonator")
	if got := e.G.Obj(self).ChosenType; got != "" {
		t.Fatalf("declined copy recorded a chosen type %q", got)
	}
	if d := e.Pending(); d != nil && d.Kind == decision.KChoose {
		t.Fatalf("a declined election left a choose pending: %q %+v", d.Prompt, d.Options)
	}
	// No copied trigger: p1's hand is only the turn's draw.
	if h := e.G.Zone(state.ZHand, 1); len(h) != 1 {
		t.Fatalf("p1 hand %v, want 1 (turn draw only, nothing drawn off the Ornithopter)", h)
	}
	replayCheck(t, e, run.cfg)
}

func TestCloneCopyOfChooseColorUsesOwnAnswer(t *testing.T) {
	t.Parallel()
	run := runETBCloneScenario(t, copyEntryScenario("Voice of All", "CCWW", "Red",
		"Clever Impersonator", "CCUU", "", "p0:Voice of All", "Blue"))
	e := run.e
	orig := findOn(t, e, state.ZBattlefield, 0, "Voice of All")
	cp := findOn(t, e, state.ZBattlefield, 1, "Voice of All")
	if got := e.G.Obj(orig).ChosenColor; got != "R" {
		t.Fatalf("precondition: original chose %q, want R", got)
	}
	if got := e.G.Obj(cp).ChosenColor; got != "U" {
		t.Fatalf("copy chose %q, want its own answer U (not the original's R)", got)
	}
	replayCheck(t, e, run.cfg)
}

// TestCloneCopyStandardCopierOfChooseColor: Superior Spider-Man (graveyard
// template, standard copier) copies Painter's Servant and is asked its own
// colour.
func TestCloneCopyStandardCopierOfChooseColor(t *testing.T) {
	t.Parallel()
	raw := `{"name":"spm-painter","cr":["707.2","614.12"],"why":"cr-g241","setup":{` +
		`"p0":{"hand":["Superior Spider-Man"]},"p1":{"graveyard":["Painter's Servant"]}},` +
		`"steps":[{"op":"cast","seat":0,"card":"p0:Superior Spider-Man","mana":"CCUB"},` +
		`{"op":"resolve","seat":0,"answers":[{"kind":"choose","pick":["p1:Painter's Servant"]},{"kind":"choose","pick":["Green"]}]}],"expect":[]}`
	run := runETBCloneScenario(t, raw)
	id := findOn(t, run.e, state.ZBattlefield, 0, "Superior Spider-Man")
	if got := run.e.G.Obj(id).ChosenColor; got != "G" {
		t.Fatalf("copy chose %q, want G", got)
	}
	replayCheck(t, run.e, run.cfg)
}
