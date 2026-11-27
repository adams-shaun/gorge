// Package encodertest is the determinism kit for state encoders: Check runs
// one encoder over fixture positions repeatedly, each run on a fresh copy of
// the view whose maps were rebuilt in a shuffled insertion order, and reports
// any run whose output differs from the first. An encoder that ranges a map
// where the order reaches its output (the one nondeterminism Go itself
// randomises) fails here instead of in a training run.
//
// An encoder in another module gets the same coverage by calling CheckAll
// from its own test after registering.
package encodertest

import (
	"fmt"
	"math/rand/v2"
	"reflect"

	"github.com/adams-shaun/gorge/bots/encoder"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// T is the part of *testing.T the kit uses; *testing.T and *testing.B satisfy it.
type T interface {
	Helper()
	Errorf(format string, args ...any)
}

// Runs is how many encodings of each input Check compares. A map with n keys
// reorders on ~1/n of runs at worst, so 16 runs miss a planted range of 6+
// keys with probability under 1e-11.
const Runs = 16

// Position is one fixture: a view, the seat it is encoded for and the Diag a
// Hidden encoder reads (nil for none).
type Position struct {
	Name string
	View view.View
	Seat state.PlayerID
	Diag *encoder.Diag
}

// Positions are the fixtures every encoder is checked on: a four-seat Commander
// table whose per-player and per-card maps each hold several keys, from seat 0
// with a priority decision over a mix of option kinds.
func Positions() []Position {
	v := view.View{Turn: 9, Step: "declare-blockers", Phase: "combat", Active: 0, Priority: 0, DayNight: "night"}
	for p := 0; p < 4; p++ {
		id := state.ObjID(100 + p)
		pv := view.PlayerView{
			ID: state.PlayerID(p), Life: 40 - int32(p),
			Commanders:     []view.CardView{{ID: id, Name: "Commander", Owner: state.PlayerID(p), Controller: state.PlayerID(p)}},
			CommanderCasts: []int32{0},
			Command:        []view.CardView{{ID: id, Name: "Commander", Owner: state.PlayerID(p), Controller: state.PlayerID(p), Types: "Legendary Creature"}},
			Pool:           map[string]int32{"W": 1, "U": 2, "B": 3, "R": 4, "G": 5, "C": 6},
			Available:      map[string]int32{"W": 6, "U": 5, "B": 4, "R": 3, "G": 2, "C": 1},
			Counters:       map[string]int32{"POISON": 1 + int32(p), "ENERGY": 2, "EXPERIENCE": 3, "RAD": 4, "STUN": 5, "TIME": 6},
		}
		v.Players = append(v.Players, pv)
	}
	atk := state.PlayerID(3)
	v.Players[0].Battlefield = []view.CardView{
		{ID: 10, Name: "Bear", Types: "Creature", Power: 2, Toughness: 2, Attacking: true, AttackingPlayer: &atk,
			Counters:   map[string]int32{"P1P1": 2, "M1M1": 1, "SHIELD": 3, "STUN": 4, "TIME": 5, "LOYALTY": 6},
			Controller: 0, Owner: 0},
		{ID: 11, Name: "Forest", Types: "Basic Land", Controller: 0, Owner: 0},
	}
	v.Players[0].Hand = []view.CardView{{ID: 12, Name: "Bear", Types: "Creature", ManaCost: "1 G", Controller: 0, Owner: 0}}
	v.Players[0].HandSize = 1
	v.Players[0].CmdDamage = map[state.ObjID]int32{101: 1, 102: 2, 103: 3, 111: 4, 112: 5, 113: 6}
	v.Players[1].Battlefield = []view.CardView{{ID: 20, Name: "Wall", Types: "Creature", Power: 0, Toughness: 4, Controller: 1, Owner: 1}}
	v.Players[3].IsMonarch = true
	v.Decision = &decision.Decision{
		Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{
			{Index: 0, Kind: "pass"},
			{Index: 1, Kind: "cast", Obj: 12, Label: "Bear"},
			{Index: 2, Kind: "play_land", Obj: 11, Label: "Forest"},
		},
	}
	diag := &encoder.Diag{
		OppHand:       []view.CardView{{ID: 30, Name: "Bear", Types: "Creature", Owner: 1, Controller: 1}},
		OppLibraryTop: []string{"Forest", "Island"},
		OwnLibraryTop: []string{"Swamp", "Mountain"},
	}
	return []Position{
		{Name: "seat0", View: v, Seat: 0, Diag: diag},
		{Name: "seat2", View: v, Seat: 2, Diag: diag},
	}
}

// CheckAll runs Check for every registered encoder.
func CheckAll(t T) {
	t.Helper()
	for _, e := range encoder.List() {
		Check(t, e, Positions())
	}
}

// Check encodes every position's state and options Runs times and reports
// each run whose output differs from the first. Run 0 uses the view as given;
// every later run uses ShuffledCopy of it, and the state buffer alternates
// between a fresh allocation and a reused pooled one.
func Check(t T, e encoder.ViewEncoder, ps []Position) {
	t.Helper()
	name := e.Schema().Name
	if len(ps) == 0 {
		t.Errorf("encoder %q: no positions to check", name)
		return
	}
	rng := rand.New(rand.NewPCG(1, 2))
	for _, p := range ps {
		var want snapshot
		pooled := policynet.AcquireStateBuf()
		for run := 0; run < Runs; run++ {
			v := p.View
			if run > 0 {
				v = ShuffledCopy(p.View, rng)
			}
			buf := pooled
			if run%2 == 0 {
				buf = nil
			}
			got := encodeAll(e, buf, &v, p)
			if run == 0 {
				want = got
				continue
			}
			if what := want.diff(got); what != "" {
				t.Errorf("encoder %q position %q run %d: %s differs from run 0 (nondeterministic: a map range or unseeded value reaches the output)", name, p.Name, run, what)
				break
			}
		}
		policynet.ReleaseStateBuf(pooled)
	}
}

// snapshot is one encoding's output, deep-copied out of any pooled storage.
type snapshot struct {
	state   policynet.State
	options []policynet.Option
}

func encodeAll(e encoder.ViewEncoder, buf *policynet.StateBuf, v *view.View, p Position) snapshot {
	st := e.EncodeState(buf, v, p.Seat, p.Diag)
	s := snapshot{state: policynet.State{
		Dense:  append([]float32(nil), st.Dense...),
		Sparse: append([]policynet.Feature(nil), st.Sparse...),
		Cards:  append([]policynet.EntityCard(nil), st.Cards...),
	}}
	if v.Decision != nil {
		n := len(v.Decision.Options)
		for i, o := range v.Decision.Options {
			s.options = append(s.options, e.EncodeOption(v, p.Seat, v.Decision.Kind, o, i, n))
		}
	}
	return s
}

// diff names the first part of got that differs from s, "" when equal.
func (s snapshot) diff(got snapshot) string {
	switch {
	case !reflect.DeepEqual(s.state.Dense, got.state.Dense):
		return "state Dense"
	case !reflect.DeepEqual(s.state.Sparse, got.state.Sparse):
		return "state Sparse"
	case !reflect.DeepEqual(s.state.Cards, got.state.Cards):
		return "state Cards"
	}
	for i := range s.options {
		if !reflect.DeepEqual(s.options[i], got.options[i]) {
			return fmt.Sprintf("option %d", i)
		}
	}
	return ""
}

// ShuffledCopy returns a deep copy of v in which every map is rebuilt by
// inserting its keys in an order drawn from rng, so a copy shares no storage
// with v and its maps do not repeat v's insertion order. Equal maps compare
// equal whatever the order; only an encoder that lets iteration order reach
// its output can tell the copies apart.
func ShuffledCopy(v view.View, rng *rand.Rand) view.View {
	src := reflect.ValueOf(&v).Elem()
	dst := reflect.New(src.Type()).Elem()
	copyShuffled(dst, src, rng)
	return dst.Interface().(view.View)
}

func copyShuffled(dst, src reflect.Value, rng *rand.Rand) {
	switch src.Kind() {
	case reflect.Pointer:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.New(src.Type().Elem()))
		copyShuffled(dst.Elem(), src.Elem(), rng)
	case reflect.Slice:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeSlice(src.Type(), src.Len(), src.Len()))
		for i := 0; i < src.Len(); i++ {
			copyShuffled(dst.Index(i), src.Index(i), rng)
		}
	case reflect.Array:
		for i := 0; i < src.Len(); i++ {
			copyShuffled(dst.Index(i), src.Index(i), rng)
		}
	case reflect.Map:
		if src.IsNil() {
			return
		}
		dst.Set(reflect.MakeMapWithSize(src.Type(), src.Len()))
		// MapKeys order is unspecified, so the draw below is the only order
		// that is ours: sort-free, it is a Fisher-Yates over whatever came out.
		keys := src.MapKeys()
		rng.Shuffle(len(keys), func(i, j int) { keys[i], keys[j] = keys[j], keys[i] })
		for _, k := range keys {
			val := reflect.New(src.Type().Elem()).Elem()
			copyShuffled(val, src.MapIndex(k), rng)
			dst.SetMapIndex(k, val)
		}
	case reflect.Struct:
		dst.Set(src) // unexported fields are shared; exported ones are replaced below
		for i := 0; i < src.NumField(); i++ {
			if src.Type().Field(i).IsExported() {
				copyShuffled(dst.Field(i), src.Field(i), rng)
			}
		}
	default:
		dst.Set(src)
	}
}
