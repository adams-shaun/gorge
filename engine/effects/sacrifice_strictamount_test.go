package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// strictDeclineHost serves every posed decision from the tape with a decline
// (the "Don't sacrifice" mode, index 1) and records what was posed.
type strictDeclineHost struct {
	*fakeHost
	posed []*decision.Decision
}

func (h *strictDeclineHost) AskCount() uint64 { return uint64(len(h.posed)) }

func (h *strictDeclineHost) TapeAnswer(d *decision.Decision) (decision.Intent, bool) {
	cp := *d
	cp.Options = append([]decision.Option(nil), d.Options...)
	h.posed = append(h.posed, &cp)
	return decision.Intent{Choices: []int{1}}, true
}

// TestSacrificeStrictAmountMandatory pins StrictAmount$ True as "exactly
// Amount$, or none" for the corpus's mandatory carriers (Scorched Ruins and
// Lotus Vale: SacValid$ Land.untapped; Infernal Denizen: SacValid$ Swamp),
// and the optional carrier (Giant Opportunity) keeps its yes/no ask.
func TestSacrificeStrictAmountMandatory(t *testing.T) {
	const swamp = "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"
	const island = "Name:Island\nTypes:Basic Land Island\nOracle:x\n"
	const food = "Name:Food\nTypes:Artifact Food\nOracle:x\n"
	carriers := []struct {
		name, valid, card string
	}{
		{"Scorched Ruins", "Land.untapped", island},
		{"Lotus Vale", "Land.untapped", island},
		{"Infernal Denizen", "Swamp", swamp},
	}
	for _, cr := range carriers {
		for _, eligible := range []int{1, 2} {
			h := newHost(t, 2)
			var ids []state.ObjID
			for i := 0; i < eligible; i++ {
				ids = append(ids, putBattlefield(h, 0, cr.card))
			}
			// Precondition: every placed permanent is on the battlefield and
			// matches SacValid$, so a partial sacrifice is the only way one moves.
			for _, id := range ids {
				if sacZone(h, id) != state.ZBattlefield || h.g.Obj(id).Tapped {
					t.Fatalf("%s: setup permanent %d not an untapped battlefield permanent", cr.name, id)
				}
			}
			c := &Ctx{Source: 1, Controller: 0}
			effSacrifice(h, c, sacrificeParams(map[string]string{
				"Defined": "You", "SacValid": cr.valid, "Amount": "2",
				"StrictAmount": "True", "RememberSacrificed": "True",
			}))
			want := state.ZBattlefield
			if eligible == 2 {
				want = state.ZGraveyard
			}
			for _, id := range ids {
				if got := sacZone(h, id); got != want {
					t.Fatalf("%s with %d eligible: permanent zone = %v, want %v", cr.name, eligible, got, want)
				}
			}
			if eligible == 1 && len(h.log) != 0 {
				t.Fatalf("%s with 1 eligible: want no events, got %+v", cr.name, h.log)
			}
		}
	}

	// Giant Opportunity: Optional$ + StrictAmount$ still poses the yes/no
	// ask, and declining sacrifices nothing.
	fh := newHost(t, 2)
	foods := []state.ObjID{putBattlefield(fh, 0, food), putBattlefield(fh, 0, food)}
	h := &strictDeclineHost{fakeHost: fh}
	effSacrifice(h, &Ctx{Source: 1, Controller: 0}, sacrificeParams(map[string]string{
		"Defined": "You", "SacValid": "Food", "Amount": "2", "Optional": "True",
		"StrictAmount": "True", "RememberSacrificed": "True",
	}))
	if len(h.posed) != 1 || h.posed[0].Kind != decision.KModes || h.posed[0].ResumeKind != "sacrifice_optional" {
		t.Fatalf("Giant Opportunity: posed %+v, want one sacrifice_optional KModes", h.posed)
	}
	for _, id := range foods {
		if got := sacZone(fh, id); got != state.ZBattlefield {
			t.Fatalf("Giant Opportunity declined: Food zone = %v, want battlefield", got)
		}
	}
}
