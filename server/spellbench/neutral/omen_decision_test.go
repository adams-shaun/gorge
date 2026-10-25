package neutral_test

import (
	"encoding/json"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
)

// TestKernelOmenCastInvertsOtherMethod pins the inverse of mapping.CastMethod:
// an Omen cast has no Section 7.4 method of its own, so the kernel sends the
// catch-all "other" and the neutral seat must map it back to omen_alt for the
// card that actually has an Omen face (CR 720).
func TestKernelOmenCastInvertsOtherMethod(t *testing.T) {
	reg := testcorpus.Registry(t)
	ref, err := neutral.Resolve(reg, "Sagu Wildling")
	if err != nil {
		t.Fatal(err)
	}
	if ref.Card == nil || ref.Card.AlternateMode != "Omen" {
		t.Fatalf("Sagu Wildling is not an Omen card: %+v", ref.Card)
	}

	obs := `{"viewer":"p0","turn":3,"phase_step":"precombat_main","active_seat":"p0","priority_seat":"p0",
	 "passed_seats":[],"day_night":null,"stack":[],"pending_triggers":null,"known":[],
	 "players":[
	  {"seat":"p0","life":20,"poison":null,"counters":null,"mana_pool":{"W":0,"U":0,"B":0,"R":0,"G":0,"C":0},
	   "lands_played_this_turn":0,"mulligans_taken":0,"designations":[],"progress":null,"hand_count":1,"library_count":50,
	   "hand":[{"object_id":"o-sagu","card_name":"Sagu Wildling","owner_seat":"p0","controller_seat":"p0","zone":"hand",
	    "full_name":null,"face_down":false,"token":false,"copy":false,
	    "characteristics":{"supertypes":[],"types":["creature"],"subtypes":[],"colors":["green"],"mana_value":5,
	     "power":4,"toughness":4,"keywords":[]},"permanent":null,"exiled_by":null}],
	   "battlefield":[],"graveyard":[],"exile":[],"command":[]},
	  {"seat":"p1","life":20,"poison":null,"counters":null,"mana_pool":{"W":0,"U":0,"B":0,"R":0,"G":0,"C":0},
	   "lands_played_this_turn":0,"mulligans_taken":0,"designations":[],"progress":null,"hand_count":0,"library_count":50,
	   "hand":null,"battlefield":[],"graveyard":[],"exile":[],"command":[]}]}`
	src := `{"object_id":"o-sagu","card_name":"Sagu Wildling","owner_seat":"p0","controller_seat":"p0","zone":"hand"}`
	body := `{"acting_seat":"p0","seat_step":4,"group":{"group_id":4,"substep_index":0,"substep_count":1},
	 "context":{"kind":"priority","source":null,"purpose":null,"text":null,"rewind":false},"observation":` + obs + `,
	 "candidates":[
	  {"candidate_id":0,"semantic":{"kind":"pass"},"display_text":null},
	  {"candidate_id":1,"semantic":{"kind":"cast_spell","source":` + src + `,"method":"other"},"display_text":null}
	 ],"extensions":{}}`
	var sd protocol.SeatDecision
	if err := json.Unmarshal([]byte(body), &sd); err != nil {
		t.Fatal(err)
	}

	p, err := neutral.NewSession(reg).Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if p.Decision.Kind != decision.KPriority {
		t.Fatalf("decision kind %v", p.Decision.Kind)
	}
	for _, o := range p.Decision.Options {
		if o.Kind == "cast" && o.Mode == "omen_alt" {
			return
		}
	}
	t.Fatalf("no omen_alt cast option in %+v", p.Decision.Options)
}
