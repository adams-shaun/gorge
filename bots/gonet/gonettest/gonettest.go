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

// Config is the network's shape; New is the smallest the loader accepts.
const (
	d    = 8 // state and option width
	ff   = 8 // block feed-forward width
	e    = 4 // identity-row embedding width
	cd   = 4 // card vector width
	hh   = 8 // head hidden width
	grps = 4
)

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
	lin("dense_in", policynet.DenseWidth, d, true)
	lin("sparse_proj", e, d, false)
	lin("pool_proj", 2*grps*cd, d, true)
	norm("blocks.0.norm", d)
	lin("blocks.0.fc1", d, ff, true)
	lin("blocks.0.fc2", ff, d, true)
	norm("norm", d)
	rand("opt_in.slots.weight", policynet.OptionSlotWidth, d)
	lin("opt_in.hproj", e, d, false)
	lin("opt_in.dense", policynet.OptionDenseWidth, d, true)
	rand("opt_in.bot", d)
	lin("ea_proj", cd, d, true)
	lin("eb_proj", cd, d, true)
	rand("none_a", cd)
	rand("none_b", cd)
	norm("opt_norm", d)
	lin("policy.0", 2*d, hh, true)
	lin("policy.2", hh, 1, true)
	lin("value.0", d, hh, true)
	lin("value.2", hh, 1, true)

	var table []Entry
	var body []byte
	for _, t := range ts {
		table = append(table, Entry{t.name, t.shape, int64(len(body)), int64(4 * len(t.data))})
		for _, v := range t.data {
			body = binary.LittleEndian.AppendUint32(body, math.Float32bits(v))
		}
	}
	header, err := headerRaw(table, rawW, extra)
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
	h := map[string]any{
		"format": "dzgw-1",
		"arch":   "mlp",
		"config": map[string]float64{"d": d, "ff": ff, "d_emb": e, "card_d": cd, "head_hidden": hh,
			"layers": 1, "policy_layers": 1, "value_layers": 1},
		"consts": map[string]int{"DENSE_W": policynet.DenseWidth, "RAW_W": rawW,
			"OPT_DENSE_W": policynet.OptionDenseWidth, "TABLE_ROWS": policynet.TableRows,
			"SLOT_ROWS": policynet.OptionSlotWidth, "GROUPS": grps},
		"layernorm_eps": map[string]float64{"blocks.0.norm": 1e-5, "norm": 1e-5, "opt_norm": 1e-5},
		"tensors":       tensors,
	}
	for k, v := range extra {
		h[k] = v
	}
	return json.Marshal(h)
}
