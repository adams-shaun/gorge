package effects

// Clone with Choices$ + ValidTgts$ and no CloneTarget$.
//
// Forge's CloneEffect reads the Choices$ pick as the copy SOURCE and, on a
// targeting ability with no CloneTarget$, the SA's own target as the BECOME
// operand: "target token you control becomes a copy of it" (Kaya, Spirits'
// Justice), "target creature becomes a copy of that creature" (Cytoshape,
// Duplication Device). Before the fix cloneBecome's absent-CloneTarget$ arm
// always answered Self, so Kaya turned herself into the chosen creature.

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cloneChoicesTargetBecomeCarriers is the measured corpus set of Clone SAs
// carrying Choices$ and ValidTgts$ with no CloneTarget$. A new carrier must be
// checked against Forge's become arm before it joins.
var cloneChoicesTargetBecomeCarriers = []string{
	"Cytoshape",
	"Duplication Device",
	"Kaya, Spirits' Justice",
}

func TestCloneChoicesTargetBecomeCensus(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	got := map[string]bool{}
	sawClone := 0
	visit := func(name string, sa *cards.SA) {
		for ; sa != nil; sa = sa.Sub {
			if sa.API != "Clone" {
				continue
			}
			sawClone++
			cp := CloneOf(sa)
			shape := cp.Choices != "" && cp.CloneTarget == "" && strings.TrimSpace(sa.ParamStr(cards.PKValidTgts)) != ""
			if shape != cp.ChoicesTargetBecome {
				t.Errorf("%s: Choices$+ValidTgts$ without CloneTarget$ = %v but ChoicesTargetBecome = %v", name, shape, cp.ChoicesTargetBecome)
			}
			if shape {
				got[name] = true
			}
		}
	}
	for _, card := range reg.AllCards() {
		for _, f := range card.Faces {
			for _, sa := range f.Abilities {
				visit(f.Name, sa)
			}
			names := make([]string, 0, len(f.SVars))
			for n := range f.SVars {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				if strings.Contains(f.SVars[n], "Clone") {
					visit(f.Name, cards.ResolveSVar(f.SVars, n))
				}
			}
		}
	}
	if sawClone < 150 {
		t.Fatalf("census saw only %d Clone SAs: the scan is not reading the corpus", sawClone)
	}
	var have []string
	for n := range got {
		have = append(have, n)
	}
	sort.Strings(have)
	if strings.Join(have, "|") != strings.Join(cloneChoicesTargetBecomeCarriers, "|") {
		t.Fatalf("Choices$+ValidTgts$ Clone carriers = %v, want %v", have, cloneChoicesTargetBecomeCarriers)
	}
}

// clonePermanentEvents returns h's ClonePermanent events.
func clonePermanentEvents(h *fakeHost) []events.Event {
	var out []events.Event
	for _, e := range h.log {
		if e.Kind == events.ClonePermanent {
			out = append(out, e)
		}
	}
	return out
}

// TestCloneKayaTargetTokenBecomesCopy: Kaya's TrigCopy copies the exiled
// creature card onto the TARGET token, never onto Kaya herself.
func TestCloneKayaTargetTokenBecomesCopy(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kaya, ok := reg.Lookup("Kaya, Spirits' Justice")
	if !ok || kaya == nil {
		t.Fatal("missing corpus card Kaya, Spirits' Justice")
	}
	body := cards.ResolveSVar(kaya.Faces[0].SVars, "TrigCopy")
	if body == nil || body.Params["CloneTarget"] != "" || body.Params["ValidTgts"] == "" || body.Params["Choices"] == "" {
		t.Fatalf("precondition: Kaya TrigCopy shape %+v", body)
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	exiled := mkCard(t, "Name:Fixture Kaya Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	tok := mkCard(t, "Name:Fixture Kaya Token\nManaCost:0\nTypes:Creature Spirit\nPT:1/1\nOracle:x\n")
	kayaID := h.g.AddObject(kaya, 0).ID
	tokID := h.g.AddObject(tok, 0).ID
	exileID := h.g.AddObject(exiled, 0).ID
	h.g.Obj(tokID).IsToken = true
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{kayaID, tokID})
	h.g.Obj(kayaID).Zone = state.ZBattlefield
	h.g.Obj(tokID).Zone = state.ZBattlefield
	h.g.SetZone(state.ZExile, 0, []state.ObjID{exileID})
	h.g.Obj(exileID).Zone = state.ZExile
	if kayaID == tokID || h.g.Obj(tokID).Zone != state.ZBattlefield || h.g.Obj(kayaID).Zone != state.ZBattlefield {
		t.Fatal("precondition: Kaya and the target token are distinct battlefield objects")
	}

	h.askResult = true
	Resolve(h, &Ctx{Source: kayaID, Controller: 0, Targets: []state.Target{{Obj: tokID}},
		TargetsOffered: true, Remembered: []state.Target{{Obj: exileID}},
		SVars: kaya.Faces[0].SVars}, body)

	evs := clonePermanentEvents(h)
	if len(evs) != 1 {
		t.Fatalf("ClonePermanent events = %d, want 1; notes %v", len(evs), cloneNoteTexts(h))
	}
	if evs[0].Obj != tokID {
		t.Fatalf("the copy landed on %d, want the target token %d (Kaya is %d)", evs[0].Obj, tokID, kayaID)
	}
	if len(evs[0].IDs) == 0 || evs[0].IDs[0] != exileID {
		t.Fatalf("copy source %v, want the exiled card %d", evs[0].IDs, exileID)
	}
}

// TestCloneCytoshapeTargetBecomesChosen: Cytoshape's spell copies the chosen
// creature onto the target creature; the spell's own source never becomes a
// copy, and the chosen creature is unchanged.
func TestCloneCytoshapeTargetBecomesChosen(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	cyto, ok := reg.Lookup("Cytoshape")
	if !ok || cyto == nil || len(cyto.Faces[0].Abilities) == 0 {
		t.Fatal("missing corpus card Cytoshape")
	}
	body := cyto.Faces[0].Abilities[0]
	if body.API != "Clone" || body.Params["Choices"] == "" || body.Params["ValidTgts"] == "" || body.Params["CloneTarget"] != "" {
		t.Fatalf("precondition: Cytoshape is a Choices$+ValidTgts$ Clone, got %+v", body.Params)
	}
	h := &fakeHost{g: state.NewGame(names(2))}
	model := mkCard(t, "Name:Fixture Cyto Model\nManaCost:4 G\nTypes:Creature Beast\nPT:5/5\nOracle:x\n")
	victim := mkCard(t, "Name:Fixture Cyto Victim\nManaCost:G\nTypes:Legendary Creature Elf\nPT:1/1\nOracle:x\n")
	srcID := h.g.AddObject(cyto, 0).ID
	modelID := h.g.AddObject(model, 1).ID
	victimID := h.g.AddObject(victim, 0).ID
	h.g.SetZone(state.ZBattlefield, 0, []state.ObjID{victimID})
	h.g.SetZone(state.ZBattlefield, 1, []state.ObjID{modelID})
	h.g.Obj(modelID).Zone = state.ZBattlefield
	h.g.Obj(victimID).Zone = state.ZBattlefield

	// No answer: the no-host Choices$ stand-in picks the first eligible
	// object. The legendary target is outside Choices$ Creature.nonLegendary,
	// so the model is the only eligible source.
	Resolve(h, &Ctx{Source: srcID, Controller: 0, Targets: []state.Target{{Obj: victimID}},
		TargetsOffered: true}, body)

	evs := clonePermanentEvents(h)
	if len(evs) != 1 {
		t.Fatalf("ClonePermanent events = %d, want 1; notes %v", len(evs), cloneNoteTexts(h))
	}
	if evs[0].Obj != victimID {
		t.Fatalf("the copy landed on %d, want the target %d (source %d)", evs[0].Obj, victimID, srcID)
	}
}
