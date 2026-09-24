package policynet

import (
	"bytes"
	"encoding/json"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// entityFixture is valueFixture with an entity encoder: five cards across
// the four groups (one group of two so the max pool has a contest), the
// options referencing cards through EntA/EntB, and every entity block
// randomised (the upgrade's zero projection and zero option columns would
// make most gradients trivially zero).
func entityFixture(t *testing.T, seed uint64) (*Model, Example, LossConfig) {
	t.Helper()
	rng := rand.New(rand.NewPCG(seed, seed+1))
	m, ex, lc := valueFixture(rng)
	m.Features = FeaturesMZ
	if err := UpgradeEntity(m, 3, rand.New(rand.NewPCG(seed, 5))); err != nil {
		t.Fatal(err)
	}
	for _, blk := range [][]float32{m.EntP, m.HidW} {
		for i := range blk {
			if blk[i] == 0 {
				blk[i] = float32(rng.NormFloat64()) * 0.3
			}
		}
	}
	for i := range m.EntB {
		m.EntB[i] = float32(rng.NormFloat64()) * 0.1
	}
	mkCard := func(g uint8) EntityCard {
		raw := make([]float32, EntityRawWidth)
		for i := range raw {
			if rng.IntN(3) == 0 {
				raw[i] = float32(rng.NormFloat64())
			}
		}
		return EntityCard{Group: g, Raw: raw, Rows: []Feature{{Row: uint16(rng.IntN(64)), Value: 1}, {Row: uint16(rng.IntN(64)), Value: 0.5}}}
	}
	ex.State.Cards = []EntityCard{mkCard(0), mkCard(0), mkCard(1), mkCard(2), mkCard(3)}
	ex.Options[0].EntA = 1
	ex.Options[1].EntA, ex.Options[1].EntB = 2, 3
	ex.Options[2].EntB = 5
	avoidEntityKinks(m, ex.State)
	avoidKinks(m, ex)
	avoidValueKinks(m, ex.State)
	return m, ex, lc
}

// avoidEntityKinks shifts each per-card unit's bias until no card's
// pre-activation sits within 0.03 of the ReLU kink.
func avoidEntityKinks(m *Model, st State) {
	inW := m.entInW()
	for q := 0; q < m.EntK; q++ {
		for tries := 0; tries < 400; tries++ {
			ok := true
			for _, card := range st.Cards {
				u := make([]float32, inW)
				copy(u, card.Raw)
				for _, f := range card.Rows {
					for j := 0; j < m.H; j++ {
						u[EntityRawWidth+j] += m.Table[int(f.Row)*m.H+j] * f.Value
					}
				}
				z := float64(m.EntB[q])
				for i, ui := range u {
					z += float64(m.EntW[q*inW+i]) * float64(ui)
				}
				if math.Abs(z) < 0.03 {
					ok = false
				}
			}
			if ok {
				break
			}
			m.EntB[q] += 0.011
		}
	}
}

// TestEntityGradientFiniteDifference pins the entity backward pass (the
// option references, the sum and max pools, the projection, the per-card
// encoder and the name rows in the shared table) against a central finite
// difference, for the supervised loss and both PPO shapes.
func TestEntityGradientFiniteDifference(t *testing.T) {
	for _, mode := range []string{"supervised", "softmax", "subset"} {
		m, ex, lc := entityFixture(t, 41)
		if mode != "supervised" {
			for i := range ex.Options {
				ex.Options[i].Target.Preferred = false
			}
			now := m.Score(ex.State, ex.Options)
			old := make([]float32, len(now))
			for i := range now {
				old[i] = now[i] + 0.02*float32(i%3-1)
			}
			ex.PPO = &PPOTarget{Subset: mode == "subset", OldScores: old, Chosen: []bool{false, true, false, false}, Advantage: 0.7,
				BehaviourLogP: -1.1, HasBehaviour: true}
			if mode == "subset" {
				ex.PPO.Chosen = []bool{true, true, false, false}
			}
			lc.PPOClip, lc.PPOKL, lc.ValueBlend = 0, 0.3, 0
		}
		g := m.NewGrads()
		g.Zero()
		st := m.LossGrad(ex, lc, g)
		if st.Loss == 0 {
			t.Fatalf("%s: zero loss", mode)
		}
		nz := 0
		for _, v := range g.EntW {
			if v != 0 {
				nz++
			}
		}
		if nz == 0 {
			t.Fatalf("%s: no gradient reached the entity encoder", mode)
		}
		fdAll(t, m, ex, lc, g)
		fdCheck(t, m, ex, lc, "entity encoder", m.EntW, g.EntW)
		fdCheck(t, m, ex, lc, "entity encoder bias", m.EntB, g.EntB)
		fdCheck(t, m, ex, lc, "entity projection", m.EntP, g.EntP)
	}
}

// An upgraded model scores exactly as its mz source (zero projection, zero
// option columns), round-trips through the schema-4 checkpoint bit for bit,
// and the Scorer agrees with Model.Score on it.
func TestEntityUpgradeCheckpointAndScorer(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	m, ex, _ := valueFixture(rng)
	m.Features = FeaturesMZ
	m.Rows = TableRows
	m.Table = make([]float32, TableRows*m.H)
	fillUniform(m.Table, 0.05, rng)
	st := ex.State
	st.Cards = []EntityCard{{Group: 1, Raw: make([]float32, EntityRawWidth), Rows: []Feature{{Row: 7, Value: 1}}}}
	st.Cards[0].Raw[entOne] = 1
	opts := slices.Clone(ex.Options)
	opts[0].EntA = 1
	before := m.Score(st, opts)
	var v3 bytes.Buffer
	if err := WriteCheckpoint(m, &v3); err != nil {
		t.Fatal(err)
	}
	if err := UpgradeEntity(m, 4, rand.New(rand.NewPCG(1, 2))); err != nil {
		t.Fatal(err)
	}
	if err := UpgradeEntity(m, 4, rand.New(rand.NewPCG(1, 2))); err == nil {
		t.Fatal("a second upgrade must be refused")
	}
	after := m.Score(st, opts)
	if !slices.Equal(before, after) {
		t.Fatalf("upgrade changed scores: %v -> %v", before, after)
	}
	// Move the entity blocks so the round trip is not of zeros.
	for i := range m.EntP {
		m.EntP[i] = float32(i%7) * 0.01
	}
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	if v := buf.Bytes()[4]; v != CheckpointVersionEntity {
		t.Fatalf("entity checkpoint written as version %d", v)
	}
	if v := v3.Bytes()[4]; v != CheckpointVersion {
		t.Fatalf("mz checkpoint written as version %d", v)
	}
	back, err := LoadCheckpoint(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if back.Features != FeaturesEntity || back.EntK != 4 || back.InW != m.InW {
		t.Fatalf("loaded %s k=%d inW=%d", back.Features, back.EntK, back.InW)
	}
	if !reflect.DeepEqual(back.EntP, m.EntP) || !reflect.DeepEqual(back.EntW, m.EntW) || !reflect.DeepEqual(back.HidW, m.HidW) {
		t.Fatal("entity blocks did not round-trip")
	}
	want := m.Score(st, opts)
	if got := back.Score(st, opts); !slices.Equal(got, want) {
		t.Fatalf("round-tripped scores %v, want %v", got, want)
	}
	sc := NewScorer(back)
	if got := sc.Score(st, opts); !slices.Equal(got, want) {
		t.Fatalf("scorer %v, model %v", got, want)
	}
	if got := sc.Value(st); got != back.Value(st) {
		t.Fatalf("scorer value %v, model %v", got, back.Value(st))
	}
	// A schema-4 body under the mz hash is refused.
	bad := slices.Clone(buf.Bytes())
	copy(bad[8:16], v3.Bytes()[8:16])
	if _, err := LoadCheckpoint(bytes.NewReader(bad)); err == nil {
		t.Fatal("version 4 with the mz encoder hash must be refused")
	}
}

// The entity encoding reads the redacted view deterministically: one card
// per visible battlefield/hand/stack object in the fixed group order, the
// option references resolve (a block option's attacker is EntB), the mz bag
// is unchanged, and the hash differs from mz.
func TestEntityEncoding(t *testing.T) {
	v := goldenFixture()
	mz := EncodeStateWith(FeaturesMZ, v, 0, nil)
	st := EncodeStateWith(FeaturesEntity, v, 0, nil)
	if !reflect.DeepEqual(st.Sparse, mz.Sparse) || !reflect.DeepEqual(st.Dense, mz.Dense) {
		t.Fatal("entity must keep the mz bag and dense vector")
	}
	if len(st.Cards) == 0 || !reflect.DeepEqual(st, EncodeStateWith(FeaturesEntity, v, 0, nil)) {
		t.Fatalf("entity cards %d / not deterministic", len(st.Cards))
	}
	want := 0
	for _, p := range v.Players {
		want += len(p.Battlefield)
		if p.ID == 0 {
			want += len(p.Hand)
		}
	}
	for _, s := range v.Stack {
		if s.Card != nil {
			want++
		}
	}
	if len(st.Cards) != want {
		t.Fatalf("%d cards, want %d", len(st.Cards), want)
	}
	prev := uint8(0)
	for i, c := range st.Cards {
		if len(c.Raw) != EntityRawWidth || c.Raw[entOne] != 1 {
			t.Fatalf("card %d malformed", i)
		}
		if c.Group < prev {
			t.Fatalf("card %d group %d after %d", i, c.Group, prev)
		}
		prev = c.Group
	}
	_, idx := entityCards(v, 0)
	blk := decision.Option{Index: 1, Kind: "block", Obj: 6, Attacker: 9}
	o := EncodeOptionWith(FeaturesEntity, v, 0, decision.KBlockers, blk, 1, 2)
	if o.EntA != int32(idx[6]+1) || o.EntB != int32(idx[9]+1) {
		t.Fatalf("block refs %d/%d, want %d/%d", o.EntA, o.EntB, idx[6]+1, idx[9]+1)
	}
	mzo := EncodeOptionWith(FeaturesMZ, v, 0, decision.KBlockers, blk, 1, 2)
	o.EntA, o.EntB = 0, 0
	if !reflect.DeepEqual(o, mzo) {
		t.Fatal("entity option must be the mz option plus the references")
	}
	if p := EncodeOptionWith(FeaturesEntity, v, 0, decision.KTarget, decision.Option{Kind: "player", Obj: state.PlayerRef(1)}, 0, 1); p.EntA != 0 || p.EntB != 0 {
		t.Fatal("a player option references no card")
	}
	if EncoderHashFor(FeaturesEntity) == EncoderHashFor(FeaturesMZ) || FeaturesEntity.Diagnostic() {
		t.Fatal("entity must have its own hash and be checkpointable")
	}
	if fs, ok := FeaturesForHash(EncoderHashFor(FeaturesEntity)); !ok || fs != FeaturesEntity {
		t.Fatal("FeaturesForHash does not know entity")
	}
}

// TemperedLogProb at T = 1 is PPOLogProb, SampleSoftmax is the inverse CDF,
// and a behaviour log-probability replaces π_old in the ratio only.
func TestTemperedPolicyAndBehaviourRatio(t *testing.T) {
	scores := []float32{1.5, -0.5, 0.25, 3}
	in := []bool{true, true, true, false}
	opts := make([]Option, 4)
	for i := range opts {
		opts[i].Target.Labelled = in[i]
	}
	for _, subset := range []bool{false, true} {
		chosen := []bool{false, false, true, false}
		if subset {
			chosen = []bool{true, false, true, false}
		}
		a, ok1 := TemperedLogProb(scores, in, chosen, subset, 1)
		b, ok2 := PPOLogProb(opts, scores, chosen, subset)
		if !ok1 || !ok2 || math.Abs(a-b) > 1e-12 {
			t.Fatalf("subset %v: tempered %v, ppo %v", subset, a, b)
		}
	}
	// softmax(z/2) over the first three: cumulative bounds.
	z := []float64{0.75, -0.25, 0.125}
	lse := logSumExp(z)
	p0 := math.Exp(z[0] - lse)
	if SampleSoftmax(scores, in, 2, p0-1e-9) != 0 || SampleSoftmax(scores, in, 2, p0+1e-9) != 1 || SampleSoftmax(scores, in, 2, 0.999999) != 2 {
		t.Fatal("SampleSoftmax is not the inverse CDF in option order")
	}
	if !SampleInclusion(0, 1, 0.49) || SampleInclusion(0, 1, 0.51) {
		t.Fatal("SampleInclusion(0) must be a fair coin")
	}
	m, ex, lc := ppoFixture(t, false, 31)
	plain := m.Loss(ex, lc).PPO
	ex.PPO.BehaviourLogP, ex.PPO.HasBehaviour = plain.LogPOld-0.4, true
	beh := m.Loss(ex, lc).PPO
	if math.Abs(beh.LogRatio-(plain.LogRatio+0.4)) > 1e-12 || beh.KL != plain.KL || beh.LogPOld != plain.LogPOld {
		t.Fatalf("behaviour ratio %v (plain %v), kl %v/%v", beh.LogRatio, plain.LogRatio, beh.KL, plain.KL)
	}
}

// The corpus carries entity cards, references and the behaviour fields
// through a JSON round trip, and omits them all when unset (a greedy v1
// record's bytes are unchanged).
func TestOnPolicyEntityRoundTrip(t *testing.T) {
	m, ex, _ := entityFixture(t, 9)
	scores := m.Score(ex.State, ex.Options)
	lp := -0.75
	rec := OnPolicyRecord{RecordType: OnPolicyRecordType, SchemaVersion: OnPolicySchemaVersion, Kind: decision.KPriority,
		State: EncodeOnPolicyState(ex.State), Scores: scores, Chosen: []int{1}, OutcomeKnown: true, Outcome: 1,
		Temperature: 1.5, LogPBehaviour: &lp}
	for i, o := range ex.Options {
		rec.Options = append(rec.Options, EncodeOnPolicyOption(o, i != 3))
	}
	data, err := json.Marshal(rec)
	if err != nil {
		t.Fatal(err)
	}
	var back OnPolicyRecord
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	got, err := back.Example()
	if err != nil {
		t.Fatal(err)
	}
	if !got.PPO.HasBehaviour || got.PPO.BehaviourLogP != lp {
		t.Fatal("behaviour log-probability lost")
	}
	if !slices.Equal(m.Score(got.State, got.Options), scores) {
		t.Fatal("round-tripped entity example scores differently")
	}
	plain := OnPolicyRecord{RecordType: OnPolicyRecordType, State: OnPolicyState{Dense: []float32{1}}, Options: []OnPolicyOption{{Dense: []float32{0}}}}
	pd, _ := json.Marshal(plain)
	for _, key := range []string{"temperature", "logp_behaviour", "cards", "ent_a", "ent_b"} {
		if bytes.Contains(pd, []byte(`"`+key+`"`)) {
			t.Fatalf("unset %s must be omitted: %s", key, pd)
		}
	}
}
