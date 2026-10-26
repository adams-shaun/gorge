package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A Killer Among Us is the only Standard carrier of ChooseType's Secretly$
// True (cr-g126): its "secretly choose Human, Merfolk, or Goblin" must reach
// the chooser only. The Choose event stays the ordinary "type" shape but
// carries Secret and Player, so view.RedactEvent serves every other seat the
// event's shape; the sacrifice ability's RevealChosen cost is the public
// reveal.
func TestKillerAmongUsChoiceHiddenFromOpponentView(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t)
	o := e.G.AddObject(corpusAlternativeCard(t, "A Killer Among Us"), 1)
	o.Zone = state.ZHand
	id := o.ID
	e.G.SetZone(state.ZHand, 1, append(e.G.Zone(state.ZHand, 1), id))
	e.G.Turn, e.G.Active, e.G.Priority = 3, 1, 1
	e.G.Step = state.StepMain1
	e.G.Players[1].Pool[state.MC], e.G.Players[1].Pool[state.MG] = 4, 1
	submitOption(t, e, "", "Cast A Killer Among Us")
	passUntilAsk(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosetype" ||
		d.Player != 1 {
		t.Fatalf("expected the secret creature-type ask, got %+v", d)
	}
	idx := optionByLabel(d.Options, "Goblin")
	if idx < 0 {
		t.Fatalf("no Goblin option in %+v", d.Options)
	}
	submitChoices(t, e, idx)
	finishCast(t, e, id)

	var choose *events.Event
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind == events.Choose && ev.Counter == "type" && ev.Obj == id {
			choose = ev
		}
	}
	if choose == nil {
		t.Fatal("no Choose event for the secretly chosen type")
	}
	if !choose.Secret || choose.Player != 1 || choose.Text != "Goblin" {
		t.Fatalf("Choose = %+v, want Secret, Player 1, Text Goblin", *choose)
	}
	opp := view.RedactEvent(e.G, *choose, 0)
	if opp.Text != "" || opp.Obj != 0 || !opp.Secret || opp.Player != 1 {
		t.Fatalf("opponent's redacted Choose = %+v, want the shape only", opp)
	}
	own := view.RedactEvent(e.G, *choose, 1)
	if own.Text != "Goblin" || own.Obj != id || !own.Secret {
		t.Fatalf("chooser's own Choose = %+v, want the full answered event", own)
	}
	if o := e.G.Obj(id); o == nil || o.ChosenType != "Goblin" {
		t.Fatalf("ChosenType = %+v, want Goblin", o)
	}
}

// TestKillerAmongUsRevealChosenCostRevealsType pins the reveal half: paying
// the sacrifice ability's RevealChosen<Type/creature type> cost emits a
// PUBLIC Note naming the secretly chosen type, so the opponent's redacted
// view keeps it. The Goblin token the ETB made is the chosen type and the
// target, so it gets the three +1/+1 counters.
func TestKillerAmongUsRevealChosenCostRevealsType(t *testing.T) {
	t.Parallel()
	e := handEngineTokens(t, corpusAlternativeCard(t, "A Killer Among Us"))
	id := e.G.Zone(state.ZHand, 0)[0]
	driveToTurn3Main(t, e)
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MG] = 4, 1
	submitOption(t, e, "", "Cast A Killer Among Us")
	passUntilAsk(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosetype" {
		t.Fatalf("expected the secret creature-type ask, got %+v", d)
	}
	idx := optionByLabel(d.Options, "Goblin")
	if idx < 0 {
		t.Fatalf("no Goblin option in %+v", d.Options)
	}
	submitChoices(t, e, idx)
	finishCast(t, e, id)

	var token state.ObjID
	for _, oid := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(oid)
		if o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Goblin Token" {
			token = oid
		}
	}
	if token == 0 {
		t.Fatal("the ETB did not create the Goblin token")
	}
	driveToStep(t, e, 5, 0, state.StepDeclareAttackers)
	passToKind(t, e, decision.KAttackers)
	submitAttackersOnly(t, e, token)
	activateRevealAbility(t, e, 0, id)
	submitTargetFor(t, e, token)
	drainChoices(t, e, 40)

	notes := notesWithPrefix(e, "revealed the chosen creature type:")
	if len(notes) != 1 {
		t.Fatalf("chosen-type reveal Notes = %+v, want exactly one", notes)
	}
	if notes[0].Secret {
		t.Fatalf("reveal Note is Secret: %+v", notes[0])
	}
	if notes[0].Text != "revealed the chosen creature type: Goblin" {
		t.Fatalf("reveal Note text = %q, want the chosen Goblin", notes[0].Text)
	}
	opp := view.RedactEvent(e.G, notes[0], 1)
	if opp.Text != notes[0].Text || opp.Secret {
		t.Fatalf("opponent's reveal Note = %+v, want the public line intact", opp)
	}
	if o := e.G.Obj(token); o == nil || o.Counter("P1P1") != 3 {
		t.Fatalf("targeted Goblin token counters = %+v, want P1P1=3", o)
	}
}

// TestChooseTypeWithoutSecretlyStaysPublic pins the other direction: a
// ChooseType with no Secretly$ keeps the historical public Choose event, so
// an unconditional Secret would fail here.
func TestChooseTypeWithoutSecretlyStaysPublic(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Haunting Voyage"))
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MC] = 2
	foretellIt(t, e, id)
	driveToTurn3Main(t, e)
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 5, 2
	submitOption(t, e, "foretell_cast", "Cast Haunting Voyage (foretold)")
	passUntilAsk(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosetype" {
		t.Fatalf("expected the mid-resolution creature-type ask, got %+v", d)
	}
	idx := optionByLabel(d.Options, "Zombie")
	if idx < 0 {
		t.Fatalf("no Zombie option in %+v", d.Options)
	}
	submitChoices(t, e, idx)
	finishCast(t, e, id)

	var choose *events.Event
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind == events.Choose && ev.Counter == "type" && ev.Obj == id {
			choose = ev
		}
	}
	if choose == nil {
		t.Fatal("no Choose event for the non-secret type choice")
	}
	if choose.Secret || choose.Text != "Zombie" {
		t.Fatalf("non-secret Choose = %+v, want the public Zombie event", *choose)
	}
}

// TestSecretlyCorpusCensus pins the exact Secretly$ True corpus set and the
// API that carries it. ChooseType, ChooseNumber and Vote honour the parameter
// today; ChooseCard, ChoosePlayer and GenericChoice are the measured open set
// (a new carrier of either group fails here so the follow-up is filed, not
// missed).
func TestSecretlyCorpusCensus(t *testing.T) {
	t.Parallel()
	root := filepath.Join(testutil.CorpusDirOrSkip(t), "cardsfolder")
	want := map[string]string{
		"a_killer_among_us":                "ChooseType",
		"call_to_the_void":                 "ChooseCard",
		"cirdan_the_shipwright":            "Vote",
		"cunning_azurescale_divining_dive": "GenericChoice",
		"elrond_of_the_white_council":      "Vote",
		"emissary_of_grudges":              "ChoosePlayer",
		"expert_level_safe":                "ChooseNumber",
		"goblin_game":                      "ChooseNumber",
		"guardian_archon":                  "ChoosePlayer",
		"itazura_lingering_wick":           "ChooseNumber",
		"menacing_ogre":                    "ChooseNumber",
		"mob_verdict":                      "Vote",
		"nashi_illusion_gadgeteer":         "ChooseCard",
		"negan_the_cold_blooded":           "ChooseCard",
		"ominous_lockbox":                  "ChooseNumber",
		"prisoners_dilemma":                "GenericChoice",
		"speedbrood_stalker":               "ChooseCard",
		"stalking_leonin":                  "ChoosePlayer",
		"the_toymakers_trap":               "ChooseNumber",
		"trap_the_trespassers":             "Vote",
		"truth_or_consequences":            "Vote",
		"vault_11_voters_dilemma":          "Vote",
		"wheel_of_misfortune":              "ChooseNumber",
	}
	apiLine := regexp.MustCompile(`(?:^|[ :])(?:SP|AB|DB|ST)\$ ([A-Za-z]+)`)
	got := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(b), "\n") {
			if !strings.Contains(line, "Secretly$ True") {
				continue
			}
			m := apiLine.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("%s: Secretly$ True line names no API: %q", path, line)
			}
			stem := strings.TrimSuffix(d.Name(), ".txt")
			if prev, ok := got[stem]; ok && prev != m[1] {
				t.Fatalf("%s: Secretly$ APIs %q and %q", stem, prev, m[1])
			}
			got[stem] = m[1]
		}
		return nil
	})
	if err != nil {
		t.Fatalf("corpus census: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("Secretly$ corpus files = %d, want %d: got %v", len(got), len(want), got)
	}
	for stem, api := range want {
		if got[stem] != api {
			t.Errorf("%s: Secretly$ API = %q, want %q", stem, got[stem], api)
		}
	}
	for stem, api := range got {
		if want[stem] != api {
			t.Errorf("new Secretly$ carrier %s (%s) -- extend the honoured set or file the follow-up", stem, api)
		}
	}
	honoured := map[string]bool{"ChooseType": true, "ChooseNumber": true, "Vote": true}
	open := map[string]bool{"ChooseCard": true, "ChoosePlayer": true, "GenericChoice": true}
	for stem, api := range got {
		if !honoured[api] && !open[api] {
			t.Errorf("%s: Secretly$ API %q is neither honoured nor in the measured open set", stem, api)
		}
	}
}
