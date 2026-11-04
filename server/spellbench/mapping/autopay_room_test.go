//go:build spellbench

package mapping

// A bounced Room keeps the door it left the battlefield as (CR 709.3), so the
// planner's plain-cast payment action names that face. AutoPay must map it as
// the normal cast of the object, not refuse the whole pose.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

type faceRoomReader struct {
	gamecfg.Reader
	board *state.Game
	log   *events.Log
}

func (r faceRoomReader) Board() *state.Game { return r.board }
func (r faceRoomReader) Log() *events.Log   { return r.log }
func (r faceRoomReader) Facts(id state.ObjID) gamecfg.Facts {
	if o := r.board.Obj(id); o != nil && o.Face() != nil {
		return gamecfg.Facts{Name: o.Face().Name}
	}
	return gamecfg.Facts{}
}

func TestAutoPayPlansBouncedRoomShownFace(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Underwater Tunnel")
	if !ok || len(card.Faces) != 2 {
		t.Fatalf("Underwater Tunnel has %d faces in the corpus", len(card.Faces))
	}
	board := &state.Game{Objs: []state.Object{{
		ID: 1, Card: card, FaceIdx: 1, Owner: 0, Controller: 0, Zone: state.ZHand,
	}}}
	if got := board.Obj(1).Face().Name; got != "Slimy Aquarium" {
		t.Fatalf("precondition: face 1 is %q, want the shown door", got)
	}
	r := faceRoomReader{board: board, log: new(events.Log)}
	tr := identity.New(r, secrets.NewGame(make([]byte, 32)))
	if err := tr.Sync(r); err != nil {
		t.Fatal(err)
	}
	env := &Env{G: &gamecfg.Game{R: r}, Obs: &observe.Projector{R: r, IDs: tr}}
	d := &decision.Decision{Player: 0, PaymentActions: []decision.PaymentAction{{
		ID:    "room",
		Cast:  decision.PlannedCast{Object: 1, Face: 1, Origin: "hand"},
		Plans: []decision.PaymentPlan{{}},
	}}}
	tx := &priorityTx{env: env, d: d}
	p := &Pose{}
	if _, err := tx.addPayments(p, map[castKey]*castGroup{}, nil); err != nil {
		t.Fatalf("a bounced Room's shown-face payment action was refused: %v", err)
	}
	if len(p.Candidates) != 1 {
		t.Fatalf("pose has %d candidates, want the planned cast", len(p.Candidates))
	}
	c := p.Candidates[0]
	if c.Sem.Kind != "cast_spell" || c.Sem.Fields["method"] != "normal" {
		t.Fatalf("planned cast is %v, want a normal cast_spell", c.Sem)
	}
	src, ok := c.Sem.Fields["source"].(protocol.ObjectRef)
	if !ok || src.CardName == nil || *src.CardName != "Slimy Aquarium" {
		t.Fatalf("planned cast source is %v, want the shown door", c.Sem.Fields["source"])
	}
}
