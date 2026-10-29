package rules

// The cr-pc-sacrifice-changenum leaves: Forge's SacrificeEffect reads only
// Amount$ (default 1) and never ChangeNum$; Glacial Chasm's ETB
// `DB$ Sacrifice | SacValid$ Land | ChangeNum$ 1` is therefore inert script
// noise, not a second amount. A non-1 ChangeNum$ is a shape this resolver
// cannot honour, so Face.Primitives reports it as unsupported instead of
// letting a future script pass silently.

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestSacrificeChangeNumIsInert(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	chasm, ok := reg.Lookup("Glacial Chasm")
	if !ok {
		t.Fatal("corpus fixture: Glacial Chasm missing")
	}

	// Precondition: the corpus really spells ChangeNum$ 1 on the ETB
	// Sacrifice, and the inert value is not itself refused as unsupported.
	var sac *cards.SA
	for _, tr := range chasm.Faces[0].Triggers {
		if tr.Mode == "ChangesZone" && tr.Effect != nil && tr.Effect.API == "Sacrifice" {
			sac = tr.Effect
			break
		}
	}
	if sac == nil || sac.Params["SacValid"] != "Land" || sac.Params["ChangeNum"] != "1" {
		t.Fatalf("corpus pin moved: Glacial Chasm sacrifice = %+v", sac)
	}
	if slices.Contains(chasm.Faces[0].Primitives(), "api:Sacrifice.ChangeNum") {
		t.Fatal("Glacial Chasm's ChangeNum$ 1 was refused; the Forge default is 1")
	}

	e, cfg := kr3Game(t, 901, []*cards.Card{chasm}, nil)
	// A second land is already on the battlefield, so the entering Chasm
	// leaves a real one-of-two sacrifice choice: exactly one land is taken.
	mountain := kr3Move(t, e, 0, "Mountain", state.ZBattlefield)
	chasmID := kr3Move(t, e, 0, "Glacial Chasm", state.ZBattlefield)
	e.pending = nil
	e.priorityRound()
	d := kr3Next(t, e)
	if d == nil || d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("Glacial Chasm ETB decision = %+v, want one-of-two land sacrifice", d)
	}
	offered := kr3OptionObjs(d)
	if !slices.Contains(offered, mountain) || !slices.Contains(offered, chasmID) {
		t.Fatalf("sacrifice options = %v, want both lands %d and %d", offered, mountain, chasmID)
	}
	if d := kr3Answer(t, e, kr3OptionObj(t, d, mountain)); d != nil {
		t.Fatalf("unexpected follow-up after the sacrifice = %+v", d)
	}
	if got := e.G.Obj(mountain).Zone; got != state.ZGraveyard {
		t.Fatalf("the chosen land's zone = %s, want graveyard", got)
	}
	if got := e.G.Obj(chasmID).Zone; got != state.ZBattlefield {
		t.Fatalf("the unchosen land's zone = %s, want battlefield (ChangeNum$ must not size the sacrifice)", got)
	}
	replayCheck(t, e, cfg)

	// The refuse arm: a synthetic Sacrifice carrying ChangeNum$ 2 (Forge
	// never reads it, gorge has no amount logic for it) is reported as an
	// unsupported primitive, while the same body with ChangeNum$ 1 stays
	// playable.
	probe := func(changeNum string) []string {
		c := card(t, "Name:ChangeNum Probe\nManaCost:2\nTypes:Creature Bear\nPT:2/2\n"+
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigSac | TriggerDescription$ x\n"+
			"SVar:TrigSac:DB$ Sacrifice | SacValid$ Land | ChangeNum$ "+changeNum+"\nOracle:x\n")
		return c.Faces[0].Primitives()
	}
	if p := probe("1"); slices.Contains(p, "api:Sacrifice.ChangeNum") {
		t.Fatalf("ChangeNum$ 1 was refused as unsupported: %v", p)
	}
	if p := probe("2"); !slices.Contains(p, "api:Sacrifice.ChangeNum") {
		t.Fatalf("ChangeNum$ 2 was silently accepted; primitives = %v", p)
	}
}
