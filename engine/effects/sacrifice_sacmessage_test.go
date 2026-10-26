package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestSacMessageNamesTheEdictNoun drives Sheoldred's Edict's compiled mode-one
// Sacrifice (SacMessage$ "nontoken creature") through effSacrifice. The
// opponent controls two nontoken creatures, so the mandatory Amount$ 1 ask
// must name the printed noun rather than the generic "permanent(s)".
func TestSacMessageNamesTheEdictNoun(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	card, ok := r.Lookup("Sheoldred's Edict")
	if !ok {
		t.Fatal("corpus has no Sheoldred's Edict")
	}
	sac := cards.ResolveSVar(card.Faces[0].SVars, "SacNontoken")
	if sac == nil {
		t.Fatal("Sheoldred's Edict has no SacNontoken mode body")
	}
	if got := strings.TrimSpace(sac.Params["SacMessage"]); got != "nontoken creature" {
		t.Fatalf("SacNontoken SacMessage = %q, want nontoken creature (precondition)", got)
	}

	h := newHost(t, 2)
	src := putBattlefield(h, 0, "Name:Sheoldred's Edict\nTypes:Instant\nOracle:x\n")
	first := putBattlefield(h, 1, "Name:First Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	second := putBattlefield(h, 1, "Name:Second Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	if h.g.Obj(first).Zone != state.ZBattlefield || h.g.Obj(second).Zone != state.ZBattlefield {
		t.Fatalf("precondition: opponents' victims not both on the battlefield (%v %v)",
			h.g.Obj(first).Zone, h.g.Obj(second).Zone)
	}

	c := &Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	effSacrifice(h, c, sac)

	if h.lastAsk == nil {
		t.Fatal("edict posed no KChoose")
	}
	if h.lastAsk.Kind != decision.KChoose || h.lastAsk.Player != 1 {
		t.Fatalf("edict ask = %+v, want a KChoose asked of the opponent", h.lastAsk)
	}
	if len(h.lastAsk.Options) != 2 {
		t.Fatalf("edict options = %d, want the opponent's 2 nontoken creatures", len(h.lastAsk.Options))
	}
	if got, want := h.lastAsk.Prompt, "Choose 1 nontoken creature to sacrifice"; got != want {
		t.Fatalf("edict prompt = %q, want %q", got, want)
	}
}

// TestSacMessageAbsentKeepsPermanentPrompt pins the fallback: a Sacrifice
// without SacMessage$ keeps the pre-existing generic prompt byte for byte.
func TestSacMessageAbsentKeepsPermanentPrompt(t *testing.T) {
	h := newHost(t, 2)
	src := putBattlefield(h, 0, "Name:Plain Edict\nTypes:Instant\nOracle:x\n")
	first := putBattlefield(h, 1, "Name:First Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	second := putBattlefield(h, 1, "Name:Second Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	if h.g.Obj(first).Zone != state.ZBattlefield || h.g.Obj(second).Zone != state.ZBattlefield {
		t.Fatalf("precondition: opponents' victims not both on the battlefield (%v %v)",
			h.g.Obj(first).Zone, h.g.Obj(second).Zone)
	}

	sac := sacrificeParams(map[string]string{"Defined": "Opponent", "SacValid": "Creature"})
	if _, present := sac.Params["SacMessage"]; present {
		t.Fatal("precondition: fixture carries SacMessage")
	}
	c := &Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	effSacrifice(h, c, sac)

	if h.lastAsk == nil {
		t.Fatal("plain sacrifice posed no KChoose")
	}
	if got, want := h.lastAsk.Prompt, "Choose 1 permanent(s) to sacrifice"; got != want {
		t.Fatalf("plain prompt = %q, want %q", got, want)
	}
}

// TestSacMessageOptionalAskNamesNoun pins the optional rendering: the
// "up to ... or none" form names SacMessage$ and pluralises it for a batch
// greater than one.
func TestSacMessageOptionalAskNamesNoun(t *testing.T) {
	h := newHost(t, 2)
	src := putBattlefield(h, 0, "Name:Optional Edict\nTypes:Instant\nOracle:x\n")
	first := putBattlefield(h, 1, "Name:First Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	second := putBattlefield(h, 1, "Name:Second Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	if h.g.Obj(first).Zone != state.ZBattlefield || h.g.Obj(second).Zone != state.ZBattlefield {
		t.Fatalf("precondition: opponents' victims not both on the battlefield (%v %v)",
			h.g.Obj(first).Zone, h.g.Obj(second).Zone)
	}

	sac := sacrificeParams(map[string]string{
		"Defined":    "Opponent",
		"SacValid":   "Creature",
		"Optional":   "True",
		"Amount":     "2",
		"SacMessage": "nontoken creature",
	})
	c := &Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	effSacrifice(h, c, sac)

	if h.lastAsk == nil {
		t.Fatal("optional sacrifice posed no KChoose")
	}
	if h.lastAsk.Min != 0 || h.lastAsk.Max != 2 {
		t.Fatalf("optional ask = %+v, want a 0..2 choice", h.lastAsk)
	}
	if got, want := h.lastAsk.Prompt, "Choose up to 2 nontoken creatures to sacrifice, or none"; got != want {
		t.Fatalf("optional prompt = %q, want %q", got, want)
	}
}

// TestSacMessageOptionalStrictAskNamesNoun pins the strict Optional$ yes/no
// ask: its "Sacrifice N ...?" text names SacMessage$ in place of permanent(s).
func TestSacMessageOptionalStrictAskNamesNoun(t *testing.T) {
	h := newHost(t, 2)
	src := putBattlefield(h, 0, "Name:Strict Optional Edict\nTypes:Instant\nOracle:x\n")
	victim := putBattlefield(h, 1, "Name:Victim\nManaCost:1 G\nTypes:Creature Beast\nPT:2/2\nOracle:x\n")
	if h.g.Obj(victim).Zone != state.ZBattlefield {
		t.Fatalf("precondition: opponent's victim not on the battlefield (%v)", h.g.Obj(victim).Zone)
	}

	sac := sacrificeParams(map[string]string{
		"Defined":      "Opponent",
		"SacValid":     "Creature",
		"Optional":     "True",
		"StrictAmount": "True",
		"SacMessage":   "nontoken creature",
	})
	c := &Ctx{Source: src, Controller: 0,
		Targets: []state.Target{{Player: 1, IsPlayer: true}}, TargetsOffered: true}
	effSacrifice(h, c, sac)

	if h.lastAsk == nil {
		t.Fatal("strict optional sacrifice posed no yes/no ask")
	}
	if h.lastAsk.Kind != decision.KModes {
		t.Fatalf("strict optional ask = %+v, want KModes", h.lastAsk)
	}
	if got, want := h.lastAsk.Prompt, "Sacrifice 1 nontoken creature?"; got != want {
		t.Fatalf("strict optional prompt = %q, want %q", got, want)
	}
}
