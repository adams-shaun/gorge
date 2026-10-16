// Package personatest writes a synthetic draft-RL net and its set table for
// tests: deterministic small random weights over a table of the given card
// names, so a net: drafter spec loads without any training artefact.
package personatest

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/limited"
	"github.com/adams-shaun/gorge/limited/draftfeat"
	"github.com/adams-shaun/gorge/limited/draftnet"
)

// Write writes dir/net.dpkw and dir/table.json for set over names and
// returns the weights path.
func Write(dir, set string, names []string) (string, error) {
	t := &draftfeat.SetTable{Set: set, Model: draftfeat.DefaultModel()}
	for i, n := range names {
		t.Cards = append(t.Cards, draftfeat.Card{Name: n, Mask: uint8(1 << (i % 5)), MV: uint8(1 + i%6), Rarity: limited.Common, V: float64(i%7) / 7})
	}
	tb, err := t.Encode()
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, "table.json"), tb, 0o644); err != nil {
		return "", err
	}
	if t, err = draftfeat.DecodeSetTable(tb); err != nil {
		return "", err
	}
	const D, FF = draftnet.D, draftnet.FF
	shapes := []struct {
		name string
		dims []int
	}{
		{"type.emb", []int{draftnet.NTypes, D}}, {"card.emb", []int{len(names), draftnet.CardEmb}},
		{"card.facts.w", []int{draftnet.FactDim, draftnet.CardEmb}}, {"card.facts.b", []int{draftnet.CardEmb}},
		{"pool.pick", []int{draftnet.MaxPool, D}}, {"cand.w", []int{draftnet.CandDim, D}}, {"cand.b", []int{D}},
		{"seat.off", []int{draftnet.MaxOthers, D}}, {"seat.w", []int{draftnet.SeatDim, D}}, {"seat.b", []int{D}},
		{"ctx.w", []int{draftnet.CtxDim, D}}, {"ctx.b", []int{D}},
	}
	for l := 0; l < draftnet.Layers; l++ {
		p := fmt.Sprintf("l%d.", l)
		for _, s := range []struct {
			n string
			d []int
		}{{"ln1.g", []int{D}}, {"ln1.b", []int{D}}, {"qkv.w", []int{D, 3 * D}}, {"qkv.b", []int{3 * D}},
			{"out.w", []int{D, D}}, {"out.b", []int{D}}, {"ln2.g", []int{D}}, {"ln2.b", []int{D}},
			{"ff1.w", []int{D, FF}}, {"ff1.b", []int{FF}}, {"ff2.w", []int{FF, D}}, {"ff2.b", []int{D}}} {
			shapes = append(shapes, struct {
				name string
				dims []int
			}{p + s.n, s.d})
		}
	}
	shapes = append(shapes, struct {
		name string
		dims []int
	}{"lnf.g", []int{D}}, struct {
		name string
		dims []int
	}{"lnf.b", []int{D}}, struct {
		name string
		dims []int
	}{"head.pol.w", []int{D}}, struct {
		name string
		dims []int
	}{"head.pol.b", []int{1}})
	r := rand.New(rand.NewPCG(1, 2))
	f := &draftnet.File{Meta: []byte(`{"` + draftnet.MetaTableHash + `":"` + t.Hash() + `"}`)}
	for _, s := range shapes {
		n := 1
		for _, d := range s.dims {
			n *= d
		}
		data := make([]float32, n)
		for i := range data {
			data[i] = float32(r.NormFloat64() * 0.1)
			if len(s.name) > 4 && s.name[len(s.name)-2:] == ".g" {
				data[i] = 1
			}
		}
		f.Tensors = append(f.Tensors, draftnet.Tensor{Name: s.name, Shape: s.dims, Data: data})
	}
	w := filepath.Join(dir, "net.dpkw")
	return w, os.WriteFile(w, f.Encode(), 0o644)
}
