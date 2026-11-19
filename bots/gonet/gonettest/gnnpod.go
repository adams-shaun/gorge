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

// WritePodGNN writes a tiny FeaturesPod graph network (arch gnn, config
// player_nodes 4): d=8, two heads, one local pass and one global block,
// every weight a fixed pseudo-random value, layer norms at gain 1 and bias
// 0, so the same call always writes the same bytes. valueK, when non-zero,
// adds a valueK-way value head. The tensors are gonet's pod graph's (nine
// node types, nine edge types, seat_emb and seat_leaf beside GraphNet's).
func WritePodGNN(dir, name string, valueK int) (string, error) {
	const d, heads, ff, e, passes, globals, hh = 8, 2, 8, 4, 1, 1, 8
	var ts []tensor
	rng := uint64(0x2545F4914F6CDD1D)
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
	fill := func(name string, v float32, n int) {
		f := make([]float32, n)
		for i := range f {
			f[i] = v
		}
		ts = append(ts, tensor{name, []int{n}, f})
	}
	eps := map[string]float64{}
	lin := func(name string, in, out int, bias bool) {
		rand(name+".weight", out, in)
		if bias {
			rand(name+".bias", out)
		}
	}
	norm := func(name string) {
		fill(name+".weight", 1, d)
		fill(name+".bias", 0, d)
		eps[name] = 1e-5
	}
	head := func(name string, in, layers, last int) {
		for i := 0; i <= layers; i++ {
			out := hh
			if i == layers {
				out = last
			}
			lin(fmt.Sprintf("%s.%d", name, 2*i), in, out, true)
			in = out
		}
	}
	rand("node_type.weight", 9, d)
	rand("edge_type.weight", 9, d)
	lin("raw_leaf", policynet.EntityRawWidthPod, d, true)
	lin("dense_leaf", policynet.DenseWidthPod, d, true)
	norm("norm")
	rand("opt_in.slots.weight", policynet.OptionSlotWidth, d)
	lin("opt_in.dense", policynet.OptionDenseWidth, d, true)
	rand("opt_in.bot", d)
	lin("ea_proj", d, d, true)
	lin("eb_proj", d, d, true)
	rand("none_a", d)
	rand("none_b", d)
	head("policy", 4*d, 1, 1)
	head("value", d, 1, 1)
	rand("table.weight", policynet.TableRows, e)
	lin("leaf_proj", e, d, false)
	lin("opt_in.hproj", e, d, false)
	for p := 0; p < passes; p++ {
		for _, n := range []string{"card", "zone", "player", "root"} {
			pre := fmt.Sprintf("local.%d.%s", p, n)
			lin(pre+".qkv", d, 3*d, true)
			norm(pre + ".n1")
			norm(pre + ".n2")
			lin(pre+".out", d, d, true)
			lin(pre+".ff1", d, ff, true)
			lin(pre+".ff2", ff, d, true)
		}
	}
	for i := 0; i < globals; i++ {
		pre := fmt.Sprintf("blocks.%d", i)
		norm(pre + ".n1")
		lin(pre+".qkv", d, 3*d, true)
		lin(pre+".out", d, d, true)
		norm(pre + ".n2")
		lin(pre+".ff1", d, ff, true)
		lin(pre+".ff2", ff, d, true)
	}
	norm("cross.nq")
	norm("cross.nkv")
	norm("cross.n2")
	lin("cross.q", d, d, true)
	lin("cross.kv", d, 2*d, true)
	lin("cross.out", d, d, true)
	lin("cross.ff1", d, ff, true)
	lin("cross.ff2", ff, d, true)
	rand("seat_emb.weight", 4, d)
	lin("seat_leaf", policynet.PodSeatLeafWidth, d, true)
	if valueK != 0 {
		head("value_k", d, 1, valueK)
	}
	var table []Entry
	var body []byte
	for _, t := range ts {
		table = append(table, Entry{t.name, t.shape, int64(len(body)), int64(4 * len(t.data))})
		for _, v := range t.data {
			body = binary.LittleEndian.AppendUint32(body, math.Float32bits(v))
		}
	}
	cfg := map[string]float64{"d": d, "d_emb": e, "heads": heads, "ff": ff, "passes": passes,
		"global_layers": globals, "head_hidden": hh, "policy_layers": 1, "value_layers": 1, "player_nodes": 4}
	if valueK != 0 {
		cfg["value_k"] = float64(valueK)
	}
	h := map[string]any{
		"format": "dzgw-1", "arch": "gnn", "config": cfg,
		"encoder_hash": fmt.Sprintf("%016x", policynet.EncoderHashFor(policynet.FeaturesPod)),
		"consts": map[string]int{"DENSE_W": policynet.DenseWidthPod, "RAW_W": policynet.EntityRawWidthPod,
			"OPT_DENSE_W": policynet.OptionDenseWidth, "TABLE_ROWS": policynet.TableRows,
			"SLOT_ROWS": policynet.OptionSlotWidth, "GROUPS": policynet.EntityGroupsPod},
		"layernorm_eps": eps,
		"tensors":       table,
	}
	header, err := json.Marshal(h)
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
