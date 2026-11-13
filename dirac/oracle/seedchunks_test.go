package oracle

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
)

// seedChunk is one parallel subtest's share of a seeded lockstep test: seeds
// lo..hi of matchup mi.
type seedChunk struct {
	mi     int
	lo, hi uint64
}

// seedChunks plays seeds first..last as parallel subtests "seeds/<lo>-<hi>" of
// chunk seeds each and returns when every one has finished, so a test's
// tallies and floors read the whole run after it. The seeds of a lockstep test
// are independent games (each builds its own matchup, side and policy), so the
// same games play and the same checks run as in one serial loop; the test's
// wall is its slowest chunk, and the budget's one-minute bound holds per
// subtest. A run of one chunk plays in the calling test, with no subtest.
//
// play gets its chunk's *testing.T and seed range, shares no state with the
// other chunks, and merges its results into the caller's tallies under mu.
func seedChunks(t *testing.T, first, last, chunk uint64, play func(t *testing.T, lo, hi uint64, mu *sync.Mutex)) {
	t.Helper()
	matchupChunks(t, 1, first, last, chunk, func(t *testing.T, c seedChunk, mu *sync.Mutex) { play(t, c.lo, c.hi, mu) })
}

// matchupChunks is seedChunks over matchups 0..matchups-1, each playing seeds
// first..last; with several matchups the subtests are "seeds/m<mi>_<lo>-<hi>".
func matchupChunks(t *testing.T, matchups int, first, last, chunk uint64, play func(t *testing.T, c seedChunk, mu *sync.Mutex)) {
	t.Helper()
	var cs []seedChunk
	for mi := 0; mi < matchups; mi++ {
		for lo := first; lo <= last; lo += chunk {
			cs = append(cs, seedChunk{mi: mi, lo: lo, hi: min(lo+chunk-1, last)})
		}
	}
	runChunks(t, cs, matchups > 1, play)
}

// runChunks plays the chunks cs as matchupChunks does; named prefixes each
// subtest's name with its matchup ("m<mi>_").
func runChunks(t *testing.T, cs []seedChunk, named bool, play func(t *testing.T, c seedChunk, mu *sync.Mutex)) {
	t.Helper()
	var mu sync.Mutex
	if len(cs) == 1 {
		play(t, cs[0], &mu)
		return
	}
	t.Run("seeds", func(t *testing.T) {
		for _, c := range cs {
			name := fmt.Sprintf("%d-%d", c.lo, c.hi)
			if named {
				name = fmt.Sprintf("m%d_%s", c.mi, name)
			}
			t.Run(name, func(t *testing.T) {
				t.Parallel()
				play(t, c, &mu)
			})
		}
	})
}

// addTally adds src's counts into dst (both pointers to the same struct
// type): every exported integer field, integer array and map[string]int, and
// nested structs of them. Chunked tests sum their chunks' tallies with it.
func addTally(dst, src any) {
	addValue(reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem())
}

func addValue(d, s reflect.Value) {
	switch d.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		d.SetInt(d.Int() + s.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		d.SetUint(d.Uint() + s.Uint())
	case reflect.Array:
		for i := 0; i < d.Len(); i++ {
			addValue(d.Index(i), s.Index(i))
		}
	case reflect.Struct:
		for i := 0; i < d.NumField(); i++ {
			if d.Type().Field(i).IsExported() {
				addValue(d.Field(i), s.Field(i))
			}
		}
	case reflect.Map:
		if s.Len() == 0 {
			return
		}
		if d.IsNil() {
			d.Set(reflect.MakeMap(d.Type()))
		}
		for _, k := range s.MapKeys() {
			v := reflect.New(d.Type().Elem()).Elem()
			if cur := d.MapIndex(k); cur.IsValid() {
				v.Set(cur)
			}
			addValue(v, s.MapIndex(k))
			d.SetMapIndex(k, v)
		}
	default:
		panic("addTally: unsupported field kind " + d.Kind().String())
	}
}
