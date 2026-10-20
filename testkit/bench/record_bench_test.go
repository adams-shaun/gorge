package bench_test

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/bench"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// benchSeeds is the fixed game cycle both benchmarks play, so a fixed
// -benchtime=Nx (N a multiple of len) plays the same games on any build.
var benchSeeds = [...]uint64{424242, 4100, 4101, 4102, 4103, 4104, 4105, 7}

func benchConfig(b *testing.B) rules.Config {
	reg := testutil.CorpusRegistry(b)
	names := []string{"mono-black-aggro", "mono-red-prowess"}
	decks := make([][]*cards.Card, len(names))
	for i, n := range names {
		d, err := testutil.LoadRepoDeck(reg, n)
		if err != nil {
			b.Fatal(err)
		}
		decks[i] = d
	}
	return rules.Config{Names: names, Decks: decks, Tokens: reg.Tokens}
}

// BenchmarkPlayGameRecordOff is the Hooks.Record off path: the ordinary
// bench game (SyncAnswer on, as enginebench's bot row plays it). Its
// allocs/op and B/op must not move when a recording feature lands; compare
// it against the same file at the base revision, e.g.
// `go test -run x -bench PlayGameRecordOff -benchtime 64x -count 6 ./testkit/bench`.
func BenchmarkPlayGameRecordOff(b *testing.B) {
	cfg := benchConfig(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Seed = benchSeeds[i%len(benchSeeds)]
		if _, _, err := bench.PlayGame(cfg, heuristicSeats(cfg.Seed, 2), 60, 4000, bench.Hooks{SyncAnswer: true}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPlayGameRecordOn prices recording: the same games with
// Hooks.Record set and a sink that builds and gzips the record to a temp file.
func BenchmarkPlayGameRecordOn(b *testing.B) {
	cfg := benchConfig(b)
	dir := b.TempDir()
	sink := &matchrecord.Sink{Put: func(r *matchrecord.Record) error {
		return matchrecord.Write(dir+"/g"+matchrecord.Ext, r)
	}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cfg.Seed = benchSeeds[i%len(benchSeeds)]
		if _, _, err := bench.PlayGame(cfg, heuristicSeats(cfg.Seed, 2), 60, 4000, bench.Hooks{SyncAnswer: true, Record: sink}); err != nil {
			b.Fatal(err)
		}
	}
}
