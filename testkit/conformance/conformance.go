// Package conformance plays one game on two engines through a client wire and
// reports the first thing the wire says differently. It compares only what a
// client can read, after a normaliser named per wire removes what that wire
// documents as representation; there is no allow-list to extend per run.
package conformance

import (
	"encoding/json"
	"fmt"
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/game"
)

// Wire is a client protocol a game can be played through.
type Wire interface {
	Name() string
	Open(t testing.TB, e game.Engine, c game.Config) Session
}

// Session is one client's connection to every seat and a spectator of a game.
type Session interface {
	// Next waits for what the wire poses next. At the end of the game it
	// returns the final state, with no answers, and false.
	Next() (Frame, bool)
	// Answer submits one of the last frame's Answers. An error means the game
	// refused it and nothing changed, so another answer may be tried.
	Answer(key string) error
}

// Frame is what a wire poses at one point of a game, normalised.
type Frame struct {
	Key     string         // which decision this is
	Answers []string       // sorted ids of the answers on offer
	State   map[string]any // the wire's state, by viewer
}

// Picker chooses the answer both sessions are sent. It is called with the
// frame's Answers, less any the game has refused.
type Picker func(Frame) string

// First always picks the first answer.
func First() Picker {
	return func(f Frame) string { return f.Answers[0] }
}

// Random picks uniformly, as a pure function of seed and the frames it is shown.
func Random(seed uint64) Picker {
	rng := rand.New(rand.NewPCG(seed, 0))
	return func(f Frame) string { return f.Answers[rng.IntN(len(f.Answers))] }
}

// Diff is the first difference between two sessions of one game.
type Diff struct {
	Wire  string
	Seed  uint64 // Config.Seed, which reproduces the game
	Frame int    // frames compared before this one
	Key   string // the first session's frame key
	Path  string // JSON path of the difference
	A, B  any    // the two sessions' values at Path
}

func (d *Diff) String() string {
	return fmt.Sprintf("%s wire, seed %d, frame %d (%s): %s: a=%s b=%s",
		d.Wire, d.Seed, d.Frame, d.Key, d.Path, jsonText(d.A), jsonText(d.B))
}

func jsonText(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// absent stands in for a value one side does not have.
const absent = "<absent>"

// Run plays c on a and b through w, sending both the answers pick chooses from
// a's frames, and returns the first difference, or nil if the whole game agrees.
func Run(t testing.TB, w Wire, a, b game.Engine, c game.Config, pick Picker) *Diff {
	t.Helper()
	sa, sb := w.Open(t, a, c), w.Open(t, b, c)
	for n := 0; ; n++ {
		fa, moreA := sa.Next()
		fb, moreB := sb.Next()
		diff := func(path string, va, vb any) *Diff {
			return &Diff{Wire: w.Name(), Seed: c.Seed, Frame: n, Key: fa.Key, Path: path, A: va, B: vb}
		}
		if path, va, vb, found := firstDifference(frameTree(fa), frameTree(fb), "$"); found {
			return diff(path, va, vb)
		}
		if moreA != moreB {
			return diff("$.more", moreA, moreB)
		}
		if !moreA {
			return nil
		}
		for offered := fa; ; {
			if len(offered.Answers) == 0 {
				t.Fatalf("%s wire, seed %d, frame %d (%s): the game refused every answer on offer", w.Name(), c.Seed, n, fa.Key)
			}
			key := pick(offered)
			errA, errB := sa.Answer(key), sb.Answer(key)
			if (errA == nil) != (errB == nil) {
				return diff("$.answer["+key+"]", errText(errA), errText(errB))
			}
			if errA == nil {
				break
			}
			offered.Answers = slices.DeleteFunc(slices.Clone(offered.Answers), func(s string) bool { return s == key })
		}
	}
}

func errText(err error) string {
	if err == nil {
		return "accepted"
	}
	return err.Error()
}

func frameTree(f Frame) map[string]any {
	answers := make([]any, len(f.Answers))
	for i, s := range f.Answers {
		answers[i] = s
	}
	return map[string]any{"key": f.Key, "answers": answers, "state": f.State}
}

// firstDifference walks two decoded JSON trees in a fixed order: sorted keys,
// ascending indices.
func firstDifference(a, b any, path string) (string, any, any, bool) {
	switch x := a.(type) {
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok {
			return path, a, b, true
		}
		keys := slices.Sorted(maps.Keys(x))
		for k := range y {
			if _, both := x[k]; !both {
				keys = append(keys, k)
			}
		}
		slices.Sort(keys)
		for _, k := range keys {
			va, inA := x[k]
			vb, inB := y[k]
			switch {
			case !inA:
				return path + "." + k, absent, vb, true
			case !inB:
				return path + "." + k, va, absent, true
			}
			if p, da, db, found := firstDifference(va, vb, path+"."+k); found {
				return p, da, db, true
			}
		}
		return "", nil, nil, false
	case []any:
		y, ok := b.([]any)
		if !ok {
			return path, a, b, true
		}
		for i := range min(len(x), len(y)) {
			if p, da, db, found := firstDifference(x[i], y[i], fmt.Sprintf("%s[%d]", path, i)); found {
				return p, da, db, true
			}
		}
		switch {
		case len(x) > len(y):
			return fmt.Sprintf("%s[%d]", path, len(y)), x[len(y)], absent, true
		case len(y) > len(x):
			return fmt.Sprintf("%s[%d]", path, len(x)), absent, y[len(x)], true
		}
		return "", nil, nil, false
	}
	if !reflect.DeepEqual(a, b) {
		return path, a, b, true
	}
	return "", nil, nil, false
}
