// Package gonettest writes a tiny, deterministic network file for tests.
//
// The real networks come from python -m dzg.export and are megabytes; a test
// of the loader, the hosted bot that uses it, or the server that serves that
// bot needs a file the loader accepts, not one that plays well. Weights are a
// fixed pseudo-random sequence in [-0.25, 0.25], layer-norm gains are 1 and
// biases 0, so the same call always writes the same bytes and the network
// always answers the same way.
package gonettest

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/adams-shaun/gorge/bots/policynet"
)

// grps is the number of entity groups the pooled card vectors cover.
const grps = 4

// Shape is a network's widths.
type Shape struct {
	D  int // state and option width
	FF int // block feed-forward width
	E  int // identity-row embedding width
	CD int // card vector width
	HH int // head hidden width
}

// Small is the shape Write writes: the smallest the loader accepts.
var Small = Shape{D: 8, FF: 8, E: 4, CD: 4, HH: 8}

// Wide is the production layout's widths (d == head_hidden == 64, e == cd
// == 32), the shape bots/netsimd runs on its dedicated 64-lane kernels; its
// loader refuses Small (it needs d, ff, hh and cd multiples of 32).
var Wide = Shape{D: 64, FF: 64, E: 32, CD: 32, HH: 64}

type tensor struct {
	name  string
	shape []int
	data  []float32
}

// Write writes the network to dir/name and returns its path. Its header
// carries no encoder_hash, like every network exported before that field.
func Write(dir, name string) (string, error) { return WriteWith(dir, name, nil) }

// WriteWith is Write with extra header fields set (or overridden), for
// example {"encoder_hash": "..."}.
func WriteWith(dir, name string, extra map[string]any) (string, error) {
	return WriteRaw(dir, name, policynet.EntityRawWidth, extra)
}

// WriteRaw is WriteWith with an explicit entity raw width. card_raw is always
// DRAWN with EntityRawWidthV1 columns (so every later tensor's pseudo-random
// sequence is what Write produced before the relational slots), then written
// with rawW columns; the extra columns are zero.
func WriteRaw(dir, name string, rawW int, extra map[string]any) (string, error) {
	return WriteShape(dir, name, Small, rawW, extra)
}

// WriteShape is WriteRaw at shape sh. Its weights follow the same
// pseudo-random sequence, so the same arguments always write the same bytes.
func WriteShape(dir, name string, sh Shape, rawW int, extra map[string]any) (string, error) {
	return writeShape(dir, name, sh, rawW, extra, nil, DefaultLayout)
}

// Layout is a network's depth: its trunk blocks and the hidden layers of
// its policy and value heads (the trainer's layers, policy_layers and
// value_layers).
type Layout struct {
	Layers, PolicyLayers, ValueLayers int
	// ValueK, when non-zero, adds a K-way value head (the trainer's value_k):
	// value_k.0, value_k.2, ... with ValueLayers hidden layers and ValueK
	// outputs, written after every other tensor, so the rest of the file's
	// weights are the scalar layout's bit for bit, and the config key
	// value_k. Zero writes no head and no key.
	ValueK int
}

// DefaultLayout is every other Write*'s layout: one trunk block and one
// hidden layer per head (policy.0/2, value.0/2).
var DefaultLayout = Layout{Layers: 1, PolicyLayers: 1, ValueLayers: 1}

// ProductionLayout is dzg's trained layout: one trunk block, a two-layer
// policy head (policy.0/2/4) and a one-layer value head (value.0/2), which
// netsimd.Load runs on its straight-line trunk.
var ProductionLayout = Layout{Layers: 1, PolicyLayers: 2, ValueLayers: 1}

// WriteLayout is WriteShape at layout lay.
func WriteLayout(dir, name string, sh Shape, rawW int, extra map[string]any, lay Layout) (string, error) {
	return writeShape(dir, name, sh, rawW, extra, nil, lay)
}

// WriteExt writes a Small-or-sh network with the extension block: the const
// EXT_W = extW and the tensor ext_in.weight (d × extW, row-major, no bias)
// from extIn (len d·extW). The other tensors are WriteShape's, bit for bit.
func WriteExt(dir, name string, sh Shape, extW int, extIn []float32) (string, error) {
	return WriteExtLayout(dir, name, sh, extW, extIn, DefaultLayout)
}

// WriteExtLayout is WriteExt at layout lay.
func WriteExtLayout(dir, name string, sh Shape, extW int, extIn []float32, lay Layout) (string, error) {
	if len(extIn) != sh.D*extW {
		return "", fmt.Errorf("gonettest: ext_in has %d values, want %d·%d", len(extIn), sh.D, extW)
	}
	return writeShape(dir, name, sh, policynet.EntityRawWidth, map[string]any{"EXT_W": extW}, extIn, lay)
}

// denseWidthFor returns the state dense width a net with extra writes: the
// current width unless DENSE_W overrides it. A network written with the old
// 24-wide option row predates the Commander block too, so it defaults to the
// 68-wide vector as well.
func denseWidthFor(extra map[string]any) int {
	if w, ok := extra["DENSE_W"].(int); ok {
		return w
	}
	if w, ok := extra["OPT_DENSE_W"].(int); ok && w == policynet.OptionDenseWidthV1 {
		return policynet.DenseWidthV1
	}
	return policynet.DenseWidth
}

func writeShape(dir, name string, sh Shape, rawW int, extra map[string]any, extIn []float32, lay Layout) (string, error) {
	if lay.Layers < 1 || lay.PolicyLayers < 1 || lay.ValueLayers < 1 {
		return "", fmt.Errorf("gonettest: layout %+v: layers must be positive", lay)
	}
	denseW := denseWidthFor(extra)
	if denseW != policynet.DenseWidth && denseW != policynet.DenseWidthV1 {
		return "", fmt.Errorf("gonettest: DENSE_W %d is neither %d nor %d", denseW, policynet.DenseWidthV1, policynet.DenseWidth)
	}
	d, ff, e, cd, hh := sh.D, sh.FF, sh.E, sh.CD, sh.HH
	if rawW < policynet.EntityRawWidthV1 {
		return "", fmt.Errorf("gonettest: rawW %d is below the format-1 width %d", rawW, policynet.EntityRawWidthV1)
	}
	var ts []tensor
	rng := uint64(0x9E3779B97F4A7C15)
	next := func() float32 {
		rng ^= rng << 13
		rng ^= rng >> 7
		rng ^= rng << 17
		return float32(rng>>40)/float32(1<<24)*0.5 - 0.25
	}
	rand := func(name string, shape ...int) {
		n := 1
		for _, s := range shape {
			n *= s
		}
		f := make([]float32, n)
		for i := range f {
			f[i] = next()
		}
		ts = append(ts, tensor{name, shape, f})
	}
	fill := func(name string, v float32, shape ...int) {
		n := 1
		for _, s := range shape {
			n *= s
		}
		f := make([]float32, n)
		for i := range f {
			f[i] = v
		}
		ts = append(ts, tensor{name, shape, f})
	}
	lin := func(name string, in, out int, bias bool) {
		rand(name+".weight", out, in)
		if bias {
			rand(name+".bias", out)
		}
	}
	norm := func(name string, w int) {
		fill(name+".weight", 1, w)
		fill(name+".bias", 0, w)
	}
	rawRand := func(name string, out, in int) {
		f := make([]float32, out*in)
		for o := 0; o < out; o++ {
			for i := 0; i < policynet.EntityRawWidthV1; i++ {
				f[o*in+i] = next()
			}
		}
		ts = append(ts, tensor{name, []int{out, in}, f})
	}
	rand("table.weight", policynet.TableRows, e)
	rawRand("card_raw.weight", cd, rawW)
	rand("card_raw.bias", cd)
	lin("card_rows", e, cd, false)
	lin("card_phi", cd, cd, true)
	// dense_in is always DRAWN DenseWidth columns wide, so every later
	// tensor's pseudo-random sequence is the same at DENSE_W 68 and 92, then
	// written denseW wide; a pre-Commander net's extra columns are zero.
	{
		f := make([]float32, d*denseW)
		for o := 0; o < d; o++ {
			for i := 0; i < policynet.DenseWidth; i++ {
				x := next()
				if i < denseW {
					f[o*denseW+i] = x
				}
			}
		}
		ts = append(ts, tensor{"dense_in.weight", []int{d, denseW}, f})
		rand("dense_in.bias", d)
	}
	lin("sparse_proj", e, d, false)
	lin("pool_proj", 2*grps*cd, d, true)
	for i := 0; i < lay.Layers; i++ {
		p := fmt.Sprintf("blocks.%d", i)
		norm(p+".norm", d)
		lin(p+".fc1", d, ff, true)
		lin(p+".fc2", ff, d, true)
	}
	norm("norm", d)
	rand("opt_in.slots.weight", policynet.OptionSlotWidth, d)
	lin("opt_in.hproj", e, d, false)
	// opt_in.dense is always DRAWN 24 columns wide (OptionDenseWidthV1), so
	// every later tensor's pseudo-random sequence is what Write produced
	// before the relative-seat option columns, then written optW wide; the
	// extra columns are zero. extra {"OPT_DENSE_W": 24} writes a
	// pre-relative-seat network.
	optW := policynet.OptionDenseWidth
	if w, ok := extra["OPT_DENSE_W"].(int); ok {
		optW = w
	}
	if optW < policynet.OptionDenseWidthV1 {
		return "", fmt.Errorf("gonettest: OPT_DENSE_W %d is below %d", optW, policynet.OptionDenseWidthV1)
	}
	{
		f := make([]float32, d*optW)
		for o := 0; o < d; o++ {
			for i := 0; i < policynet.OptionDenseWidthV1; i++ {
				f[o*optW+i] = next()
			}
		}
		ts = append(ts, tensor{"opt_in.dense.weight", []int{d, optW}, f})
		rand("opt_in.dense.bias", d)
	}
	rand("opt_in.bot", d)
	lin("ea_proj", cd, d, true)
	lin("eb_proj", cd, d, true)
	rand("none_a", cd)
	rand("none_b", cd)
	norm("opt_norm", d)
	headN := func(name string, in, layers, last int) {
		for i := 0; i <= layers; i++ {
			out := hh
			if i == layers {
				out = last
			}
			lin(fmt.Sprintf("%s.%d", name, 2*i), in, out, true)
			in = out
		}
	}
	head := func(name string, in, layers int) { headN(name, in, layers, 1) }
	head("policy", 2*d, lay.PolicyLayers)
	head("value", d, lay.ValueLayers)
	if extIn != nil {
		ts = append(ts, tensor{"ext_in.weight", []int{sh.D, len(extIn) / sh.D}, extIn})
	}
	if lay.ValueK != 0 {
		headN("value_k", d, lay.ValueLayers, lay.ValueK)
	}

	var table []Entry
	var body []byte
	for _, t := range ts {
		table = append(table, Entry{t.name, t.shape, int64(len(body)), int64(4 * len(t.data))})
		for _, v := range t.data {
			body = binary.LittleEndian.AppendUint32(body, math.Float32bits(v))
		}
	}
	header, err := headerShape(table, sh, rawW, extra, lay)
	if err != nil {
		return "", err
	}
	out := append([]byte("DZGW0001"), binary.LittleEndian.AppendUint64(nil, uint64(len(header)))...)
	out = append(out, header...)
	for len(out)%64 != 0 {
		out = append(out, 0)
	}
	out = append(out, body...)
	path := filepath.Join(dir, name)
	return path, os.WriteFile(path, out, 0o644)
}

// Entry is one tensor's row in a .dzgw header's tensor table.
type Entry struct {
	Name   string `json:"name"`
	Shape  []int  `json:"shape"`
	Offset int64  `json:"offset"`
	NBytes int64  `json:"nbytes"`
}

// Header is the JSON header Write puts after the magic for tensor table
// tensors, with extra fields set or overridden (contract dzgw: the golden
// bots/gonet/testdata/contracts/dzgw_header.json is Header with a short
// table and an encoder_hash).
func Header(tensors []Entry, extra map[string]any) ([]byte, error) {
	return headerRaw(tensors, policynet.EntityRawWidth, extra)
}

// headerRaw is Header with an explicit RAW_W const.
func headerRaw(tensors []Entry, rawW int, extra map[string]any) ([]byte, error) {
	return headerShape(tensors, Small, rawW, extra, DefaultLayout)
}

// headerShape is headerRaw for a network of shape sh and layout lay.
func headerShape(tensors []Entry, sh Shape, rawW int, extra map[string]any, lay Layout) ([]byte, error) {
	d, ff, e, cd, hh := float64(sh.D), float64(sh.FF), float64(sh.E), float64(sh.CD), float64(sh.HH)
	eps := map[string]float64{"norm": 1e-5, "opt_norm": 1e-5}
	for i := 0; i < lay.Layers; i++ {
		eps[fmt.Sprintf("blocks.%d.norm", i)] = 1e-5
	}
	h := map[string]any{
		"format": "dzgw-1",
		"arch":   "mlp",
		"config": map[string]float64{"d": d, "ff": ff, "d_emb": e, "card_d": cd, "head_hidden": hh,
			"layers": float64(lay.Layers), "policy_layers": float64(lay.PolicyLayers), "value_layers": float64(lay.ValueLayers)},
		"consts": map[string]int{"DENSE_W": denseWidthFor(extra), "RAW_W": rawW,
			"OPT_DENSE_W": policynet.OptionDenseWidth, "TABLE_ROWS": policynet.TableRows,
			"SLOT_ROWS": policynet.OptionSlotWidth, "GROUPS": grps},
		"layernorm_eps": eps,
		"tensors":       tensors,
	}
	if lay.ValueK != 0 {
		h["config"].(map[string]float64)["value_k"] = float64(lay.ValueK)
	}
	for k, v := range extra {
		if k == "EXT_W" || k == "OPT_DENSE_W" || k == "DENSE_W" { // a const of the net, not a top-level header key
			h["consts"].(map[string]int)[k] = v.(int)
			continue
		}
		h[k] = v
	}
	return json.Marshal(h)
}
