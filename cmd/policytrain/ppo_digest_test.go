package main

import (
	"fmt"
	"hash/fnv"
	"math"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// fullDigest is policyDigest extended over the value head's blocks and the
// value readout: everything a supervised (non-PPO) run produces, value head
// included.
func fullDigest(res *Result) uint64 {
	h := fnv.New64a()
	var b [4]byte
	put := func(fs []float32) {
		for _, f := range fs {
			u := math.Float32bits(f)
			b[0], b[1], b[2], b[3] = byte(u), byte(u>>8), byte(u>>16), byte(u>>24)
			h.Write(b[:])
		}
	}
	m := res.Model
	put(m.Table)
	put(m.StateW)
	put(m.StateB)
	put(m.HidW)
	put(m.HidB)
	put(m.OutW)
	put([]float32{m.OutB, m.ResidualW})
	put(m.VHidW)
	put(m.VHidB)
	put(m.VOutW)
	put([]float32{m.VOutB})
	fmt.Fprint(h, res.Epochs, res.ByKind, res.TrainN, res.HoldoutN, res.Skipped)
	if res.Value != nil {
		fmt.Fprint(h, *res.Value)
	}
	return h.Sum64()
}

// supervisedDigestConfigs are two supervised runs that exercise every loss
// branch the PPO change sits beside: a value-on run with a per-kind BCE
// override and the residual prior (the pn11 gen-0 recipe's shape), and a
// hybrid run with override weighting.
func supervisedDigestConfigs() ([]policynet.Example, []Config) {
	corpus := valueCorpus(96)
	for i := range corpus {
		if i%2 == 0 {
			corpus[i].Kind = decision.KAttackers
		}
	}
	a := valueConfig(3)
	a.Epochs = 10
	a.Mode = policynet.LossCE
	a.ResidualInit = 2
	a.KindModes = map[decision.Kind]policynet.LossMode{decision.KAttackers: policynet.LossBCE}
	a.Clip = 1
	b := testConfig(9)
	b.Epochs = 10
	b.Mode = policynet.LossHybrid
	b.OverrideWeight = 3
	return corpus, []Config{a, b}
}

// TestSupervisedIsBitIdenticalToPrePPOTrainer pins that adding the PPO path
// moved nothing on the supervised path: with no PPO example in the corpus,
// the trained model (value head included), every epoch statistic, the value
// readout and the per-kind table are bit-identical to the trainer BEFORE the
// PPO path existed. The digests were measured on the pre-change trainer
// (base 76dd677b3) with this exact fullDigest and these configs.
func TestSupervisedIsBitIdenticalToPrePPOTrainer(t *testing.T) {
	corpus, cfgs := supervisedDigestConfigs()
	want := []uint64{0x190e379937975b44, 0x21532b90a7a5f28b}
	for i, cfg := range cfgs {
		res, err := Train(corpus, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if got := fullDigest(res); got != want[i] {
			t.Errorf("config %d: digest %#016x, want the pre-PPO trainer's %#016x", i, got, want[i])
		}
	}
}
