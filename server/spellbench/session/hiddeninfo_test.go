//go:build spellbench

package session_test

import (
	"encoding/json"
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/mapping"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/server/spellbench/session"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
	"github.com/adams-shaun/gorge/testkit/hiddeninfo"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// oracleExt is the v2 view extension with an oracle following its game; the
// oracle has to fold each decision before the seat reads what it was sent.
type oracleExt struct {
	*xview.Extender
	o      *hiddeninfo.Oracle
	g      *state.Game
	native *decision.Decision
}

func (x *oracleExt) Extend(env *mapping.Env, p *mapping.Pose, n uint64) (map[string]json.RawMessage, error) {
	if x.o == nil {
		x.o, x.g = hiddeninfo.New(env.G.E), env.G.E.G
	}
	x.o.Observe(p.Native)
	for _, f := range p.Followups {
		x.o.Observe(f)
	}
	x.native = p.Native
	return x.Extender.Extend(env, p, n)
}

// TestV2DecisionsNameNoHiddenCard checks what the v2 wire sends a seat at every
// pose, observation and view extension alike, in games with face-down cards and
// a monarch.
func TestV2DecisionsNameNoHiddenCard(t *testing.T) {
	reg := testcorpus.Registry(t)
	pairs := [][2]string{{"deadly-disguise", "death-n-taxes"}, {"deadly-disguise", "cavalry-charge"}, {"deadly-disguise", "uw-control"}, {"ur-delver", "uw-control"}, {"uw-control", "uw-tempo"}}
	var poses, faceDown int
	for i, p := range pairs {
		ext := &oracleExt{Extender: xview.New()}
		seat := "p0"
		req := &protocol.ResetReq{GameID: "g-h", Format: "pauper-bo1", MaxDecisions: 100000, MaxSteps: 4000,
			Rules: protocol.Rules{Mulligan: "london", StartingPlayer: "host_assigned", StartingSeat: &seat, Names: catalog.PoolNames()}}
		sec := make([]byte, 32)
		sec[0] = byte(i)
		g, err := session.Start(session.Config{Reg: reg, Ext: ext}, "g-h", req, secrets.NewGame(sec),
			[2][]*cards.Card{testutil.RepoDeck(t, reg, p[0]), testutil.RepoDeck(t, reg, p[1])})
		if err != nil {
			t.Fatalf("%v: %v", p, err)
		}
		r := rand.New(rand.NewPCG(uint64(i), 7))
		for {
			dec, term := g.Pending()
			if term != nil {
				break
			}
			poses++
			sd := dec.SeatDecision
			seat := state.PlayerID(0)
			if sd.ActingSeat == "p1" {
				seat = 1
			}
			leaks := ext.o.CheckJSON(seat, "observation", sd.Observation)
			if !hiddeninfo.PublicChoice(ext.native) {
				leaks = append(leaks, ext.o.CheckJSON(seat, "candidates", sd.Candidates)...)
				leaks = append(leaks, ext.o.CheckJSON(seat, "extensions", sd.Extensions)...)
			}
			for _, l := range leaks {
				t.Errorf("%v: %s", p, l)
			}
			if hiddeninfo.AnyFaceDown(ext.g) {
				faceDown++
			}
			k := r.IntN(len(sd.Candidates))
			if perr := g.Step(&protocol.StepReq{GameID: "g-h", ExpectedStep: dec.Step, CandidateID: uint64(k), Echo: echo(t, sd.Candidates[k].Semantic)}); perr != nil {
				t.Fatalf("%v: %v", p, perr)
			}
		}
	}
	t.Logf("%d games, %d poses, %d with a face-down card", len(pairs), poses, faceDown)
	if faceDown == 0 {
		t.Error("the games stopped reaching a face-down card")
	}
}
