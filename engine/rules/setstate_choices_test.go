package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cr-g224: a SetState with Choices$ (no Defined$) asks the controller which
// permanent to change instead of silently changing nothing (Sunder the
// Gateway's "transform an Incubator token you control", Hustle // Bustle's
// "you may turn a creature you control face up").

// sunderToPick casts Sunder the Gateway's Incubate-then-transform mode and
// returns the pick it poses.
func sunderToPick(t *testing.T) (*Engine, Config, *decision.Decision) {
	t.Helper()
	sunder := tokenReplCorpusCard(t, "Sunder the Gateway")
	e, cfg := tokenReplGame(t, 5, sunder)
	searchMoveByName(t, e, "Sunder the Gateway", state.ZHand)
	addMana(t, e, 0, "WWWWWUUUUU")
	castSpellOption(t, e, "Sunder the Gateway")
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("after cast: %+v, want the mode ask", d)
	}
	mode := -1
	for _, o := range d.Options {
		if strings.HasPrefix(o.Label, "Incubate 2, then transform") {
			mode = o.Index
		}
	}
	if mode < 0 {
		t.Fatalf("transform mode not offered: %+v", d.Options)
	}
	submitChoices(t, e, mode)
	d = passUntilAskKind(t, e, decision.KChoose, 20)
	if d.ResumeKind != "setstate_pick" {
		t.Fatalf("pending = %+v, want the setstate_pick ask", d)
	}
	return e, cfg, d
}

func TestSunderTheGatewayTransformsTheIncubator(t *testing.T) {
	e, cfg, d := sunderToPick(t)
	if len(d.Options) != 1 {
		t.Fatalf("pick options = %+v, want exactly the new Incubator", d.Options)
	}
	inc := e.G.Obj(d.Options[0].Obj)
	if inc == nil || inc.Zone != state.ZBattlefield || inc.Face().Name != "Incubator Token" || inc.FaceIdx != 0 {
		t.Fatalf("precondition: offered object %+v is not the front-face Incubator", inc)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 20)
	inc = e.G.Obj(d.Options[0].Obj)
	if inc.FaceIdx != 1 {
		t.Fatalf("Incubator face = %d, want the transformed back face", inc.FaceIdx)
	}
	if got := e.Power(inc.ID); got != 2 {
		t.Fatalf("transformed Incubator power = %d, want 2 (0/0 + two +1/+1 counters)", got)
	}
	replayCheck(t, e, cfg)
}

func TestSunderTheGatewayPickIsMandatory(t *testing.T) {
	e, cfg, d := sunderToPick(t)
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("pick Min/Max = %d/%d, want 1/1", d.Min, d.Max)
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: nil}); err == nil {
		t.Fatalf("declining a mandatory pick was accepted")
	}
	submitChoices(t, e, d.Options[0].Index)
	replayCheck(t, e, cfg)
}

// bustleToPick casts Bustle with a manifested Grizzly Bears face down and
// returns the optional pick.
func bustleToPick(t *testing.T) (*Engine, Config, *decision.Decision, state.ObjID) {
	t.Helper()
	hb := tokenReplCorpusCard(t, "Hustle // Bustle")
	bears := tokenReplCorpusCard(t, "Grizzly Bears")
	e, cfg := tokenReplGame(t, 7, hb, bears)
	toMain1(t, e)
	id := manifestCard(t, e, 0, "Grizzly Bears")
	if o := e.G.Obj(id); o == nil || !o.FaceDown || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears %+v is not face down on the battlefield", o)
	}
	hid := searchMoveByName(t, e, "Hustle", state.ZHand)
	addMana(t, e, 0, "RRRRRR")
	alt := splitOption(t, e, hid, "split_alt")
	if alt == nil {
		t.Fatalf("Bustle offer missing: %+v", castOptions(t, e))
	}
	submitChoices(t, e, alt.Index)
	// The two hybrid {R/G} symbols each ask how to pay; answer red.
	d := e.Pending()
	for i := 0; i < 4 && d != nil && d.Kind == decision.KChoose && d.ResumeKind == "" &&
		len(d.Options) > 0 && strings.HasPrefix(d.Options[0].Kind, "pay_"); i++ {
		submitChoices(t, e, 0)
		d = e.Pending()
	}
	d = passUntilAskKind(t, e, decision.KChoose, 20)
	if d.ResumeKind != "setstate_pick" {
		t.Fatalf("pending = %+v, want the setstate_pick ask", d)
	}
	return e, cfg, d, id
}

func TestHustleBustleMayTurnAFaceDownCreatureUp(t *testing.T) {
	e, cfg, d, id := bustleToPick(t)
	if d.Min != 0 || d.Max != 1 || len(d.Options) != 1 || d.Options[0].Obj != id {
		t.Fatalf("pick = %+v, want an optional pick offering the face-down creature", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	passUntilStackEmpty(t, e, 20)
	if e.G.Obj(id).FaceDown {
		t.Fatalf("creature is still face down after being picked")
	}
	replayCheck(t, e, cfg)

	// Declining the optional pick leaves it face down.
	e2, cfg2, d2, id2 := bustleToPick(t)
	submitChoices(t, e2)
	passUntilStackEmpty(t, e2, 20)
	if !e2.G.Obj(id2).FaceDown {
		t.Fatalf("creature turned up although the pick was declined (%+v)", d2)
	}
	replayCheck(t, e2, cfg2)
}
