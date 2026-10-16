package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/draftlab"
	"github.com/adams-shaun/gorge/limited/draftfeat"
	"github.com/adams-shaun/gorge/limited/draftnet"
	"github.com/adams-shaun/gorge/limited/pickbot"
)

// The "net" drafter kind (DR-8/DR-9): net:WEIGHTS.dpkw[@TABLE.json][~T] is
// a pickbot.NetDrafter. With no ~T it runs at temperature 0 (hosted play:
// argmax, seeded tie-break); ~T (T a decimal >= 0, e.g. ~1) samples
// softmax(logits/T) with the seat's pick rng, which draftlab.Draft derives
// from the pod seed and seat alone, so picks are byte-identical at any
// -workers count. The temperature is the text after the LAST '~' only
// when that text parses as a number; a path whose final '~' is followed by
// a bare number must therefore carry an explicit ~0. The table defaults to table.json beside the weights, and the
// weights' meta must name that table's hash (draftnet.CheckTable), so a
// net is never run against a table it was not trained on. It serves both
// the rollout league and the gate's -drafters.
func init() { RegisterDrafterKind("net", netKind) }

type loadedNet struct {
	net   *draftnet.Net
	table *draftfeat.SetTable
	ref   draftlab.AgentRef
}

var netCache = struct {
	sync.Mutex
	m map[string]*loadedNet
}{m: map[string]*loadedNet{}}

func loadNet(arg string) (*loadedNet, error) {
	netCache.Lock()
	defer netCache.Unlock()
	if l, ok := netCache.m[arg]; ok {
		return l, nil
	}
	wpath, tpath, ok := strings.Cut(arg, "@")
	if !ok {
		tpath = filepath.Join(filepath.Dir(wpath), "table.json")
	}
	wb, err := os.ReadFile(wpath)
	if err != nil {
		return nil, err
	}
	n, err := draftnet.Load(wb)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", wpath, err)
	}
	tb, err := os.ReadFile(tpath)
	if err != nil {
		return nil, err
	}
	t, err := draftfeat.DecodeSetTable(tb)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", tpath, err)
	}
	if err := n.CheckTable(t); err != nil {
		return nil, fmt.Errorf("%s with %s: %w", wpath, tpath, err)
	}
	sum := sha256.Sum256(wb)
	l := &loadedNet{net: n, table: t, ref: draftlab.AgentRef{Name: "net", Version: hex.EncodeToString(sum[:6])}}
	netCache.m[arg] = l
	return l, nil
}

// netLogitDrafter is a NetDrafter whose recorder keeps the last pick's
// logits for the rollout's LogitSource hook and, when it samples (T > 0),
// the behaviour log-prob of the chosen card for BehaviourSource.
type netLogitDrafter struct {
	pickbot.NetDrafter
	last []float32
	logp float32
	bin  []byte
}

// LastInputsBin is the last pick's learner.bin record body: an inputs.bin
// record whose score slots carry the logits, then i32 chosen index and f32
// behaviour log-prob (0 for an argmax seat).
func (d *netLogitDrafter) LastInputsBin() []byte { return d.bin }

func (d *netLogitDrafter) Record(v pickbot.PickView, f *draftfeat.Features, in *draftnet.Inputs, logits []float32, chosen int) {
	d.last = append(d.last[:0], logits...)
	d.logp = 0
	if d.Temperature > 0 {
		d.logp = float32(pickbot.SoftmaxLogProb(logits, d.Temperature, chosen))
	}
	d.bin = appendBCInputsBin(d.bin[:0], in, logits)
	d.bin = binary.LittleEndian.AppendUint32(d.bin, uint32(int32(chosen)))
	d.bin = binary.LittleEndian.AppendUint32(d.bin, math.Float32bits(d.logp))
}

func (d *netLogitDrafter) LastLogits() []float32 { return d.last }

// LastBehaviour reports the sampling temperature and the chosen card's
// log-prob under softmax(logits/T); ok is false for an argmax (T = 0) seat.
func (d *netLogitDrafter) LastBehaviour() (temp float32, logp float32, ok bool) {
	if d.Temperature <= 0 || d.last == nil {
		return 0, 0, false
	}
	return float32(d.Temperature), d.logp, true
}

// splitNetTemp splits a net kind argument into the weights[@table] part
// and the temperature (0 when absent).
func splitNetTemp(arg string) (string, float64, error) {
	i := strings.LastIndexByte(arg, '~')
	if i < 0 {
		return arg, 0, nil
	}
	tail := arg[i+1:]
	if tail == "" || strings.ContainsAny(tail, "/@") {
		return arg, 0, nil
	}
	t, err := strconv.ParseFloat(tail, 64)
	if err != nil {
		return arg, 0, nil // a '~' inside a path, not a temperature
	}
	if t < 0 || math.IsNaN(t) || math.IsInf(t, 0) {
		return "", 0, fmt.Errorf("net temperature %q: want a finite T >= 0", tail)
	}
	if i == 0 {
		return "", 0, fmt.Errorf("net spec %q: no weights path", arg)
	}
	return arg[:i], t, nil
}

func netKind(lab *draftlab.Lab, arg, set string) (pickbot.Drafter, draftlab.AgentRef, error) {
	path, temp, err := splitNetTemp(arg)
	if err != nil {
		return nil, draftlab.AgentRef{}, err
	}
	l, err := loadNet(path)
	if err != nil {
		return nil, draftlab.AgentRef{}, err
	}
	if l.table.Set != set {
		return nil, draftlab.AgentRef{}, fmt.Errorf("net table is set %s, pod is %s", l.table.Set, set)
	}
	d := &netLogitDrafter{NetDrafter: pickbot.NetDrafter{Net: l.net, Table: l.table, Temperature: temp}}
	d.Recorder = d
	ref := l.ref
	if temp > 0 {
		ref.Version += "~" + strconv.FormatFloat(temp, 'g', -1, 64)
	}
	return d, ref, nil
}
