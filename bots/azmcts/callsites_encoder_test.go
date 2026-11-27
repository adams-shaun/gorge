package azmcts

import (
	"sync/atomic"
	"testing"

	"github.com/adams-shaun/gorge/bots/encoder"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// recordingEncoder is the encoder the test network names. It delegates to the
// built-in entity encoder, so every call site's result is the same bits it
// produced before E6, and counts the states and options encoded through it.
type recordingEncoder struct {
	inner   encoder.ViewEncoder
	states  atomic.Int64
	options atomic.Int64
}

func (r *recordingEncoder) Schema() encoder.Schema {
	s := r.inner.Schema()
	s.Name = "azmcts-test-recording"
	s.Pinned = 0
	return s
}

func (r *recordingEncoder) EncodeState(buf *policynet.StateBuf, v *view.View, seat state.PlayerID, diag *encoder.Diag) policynet.State {
	r.states.Add(1)
	return r.inner.EncodeState(buf, v, seat, diag)
}

func (r *recordingEncoder) EncodeOption(v *view.View, seat state.PlayerID, d decision.Kind, o decision.Option, idx, n int) policynet.Option {
	r.options.Add(1)
	return r.inner.EncodeOption(v, seat, d, o, idx, n)
}

// fixedExternal is a scorer with fixed answers: the test reads which encoder
// produced the state/options handed to it, not the network's numbers.
type fixedExternal struct{}

func (fixedExternal) Score(policynet.State, []policynet.Option) []float32 { return []float32{0, 1, 2} }
func (fixedExternal) Value(policynet.State) float32                       { return 0.25 }
func (fixedExternal) ValueVec(policynet.State) []float32                  { return []float32{0.6, 0.3, 0.1} }

// TestCallSitesUseNetEncoder: every call site that encodes a state or options
// for a network uses the encoder the network names (Model.Encoder), not the
// FeatureSet's encoder directly. The recording encoder's counts are the
// evidence: a call site still calling policynet.EncodeStateWith/EncodeOptionsWith
// with net.Features would encode the same bytes but never reach the net's
// encoder.
func TestCallSitesUseNetEncoder(t *testing.T) {
	rec := &recordingEncoder{inner: encoder.ForFeatureSet(policynet.FeaturesEntity)}
	encoder.Register(rec)
	name := rec.Schema().Name
	m := policynet.NewExternal(fixedExternal{}, policynet.FeaturesEntity)
	m.Encoder = name
	m.ValueK = 3 // two seats and a draw: netLeafVec can read the vector

	cfg := testConfig(t, "mono-red-goblins", "mono-green-stompy", 4242)
	e, d, in := botPosition(t, cfg, "", 0, 400)

	before := rec.states.Load()
	if got := leafValue(m, e, d.Player, nil); got != 0.25 {
		t.Fatalf("leafValue = %v, want the named encoder's network value 0.25", got)
	}
	if rec.states.Load() == before {
		t.Fatal("leafValue encoded the leaf without the encoder the net names")
	}

	before = rec.states.Load()
	if _, _, ok := netLeafVec(m, e, d.Player, nil); !ok {
		t.Fatal("netLeafVec fell back: the network has a K-way head and enough seats")
	}
	if rec.states.Load() == before {
		t.Fatal("netLeafVec encoded the leaf without the encoder the net names")
	}

	beforeStates, beforeOptions := rec.states.Load(), rec.options.Load()
	priorsWithLeaf(m, e, d, in, "", []cand{{in: in}}, new(view.View), false, nil)
	if rec.states.Load() == beforeStates {
		t.Fatal("priorsWithLeaf encoded the point without the encoder the net names")
	}
	if rec.options.Load() == beforeOptions {
		t.Fatal("priorsWithLeaf encoded the options without the encoder the net names")
	}
}
