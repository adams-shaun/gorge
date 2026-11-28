package adapter

import (
	"runtime"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/engine/game"
)

// collectTables collects until the registry holds want tables, and returns what
// it holds when it does or when it gives up. A release follows a collection on
// the runtime's own goroutine, so one pass is not enough.
func collectTables(want int) int {
	n := dirac.RegisteredTables()
	for i := 0; i < 500 && n != want; i++ {
		runtime.GC()
		time.Sleep(time.Millisecond)
		n = dirac.RegisteredTables()
	}
	return n
}

// heldTables is the registry's count after collections that had every chance to
// release a table a live game still plays on.
func heldTables() int {
	for range 10 {
		runtime.GC()
		time.Sleep(time.Millisecond)
	}
	return dirac.RegisteredTables()
}

// settledTables is the count once the games earlier tests discarded are released.
func settledTables() int {
	n := heldTables()
	for m := heldTables(); m != n; m = heldTables() {
		n = m
	}
	return n
}

func TestGameReleasesTable(t *testing.T) {
	e, c := fourSeatPod(t)
	start := settledTables()
	const games = 50
	live := make([]game.Game, games)
	for i := range live {
		c.Seed = uint64(i + 1)
		g, err := e.New(c)
		if err != nil {
			t.Fatal(err)
		}
		live[i] = g
	}
	// precondition: every game registered its own table, and keeps it
	if n := heldTables(); n != start+games {
		t.Fatalf("%d live games: %d tables registered, want %d", games, n, start+games)
	}
	runtime.KeepAlive(live)
	clear(live)
	if n := collectTables(start); n != start {
		t.Fatalf("%d games discarded: %d tables registered, want %d", games, n, start)
	}
}

// A fork shares its game's table, so the entry lives as long as either does.
func TestForkKeepsTable(t *testing.T) {
	e, c := fourSeatPod(t)
	start := settledTables()
	g, err := e.New(c)
	if err != nil {
		t.Fatal(err)
	}
	fork := g.Fork()
	g = nil
	if n := heldTables(); n != start+1 {
		t.Fatalf("the original discarded, its fork live: %d tables registered, want %d", n, start+1)
	}
	// the fork still plays: its next table lookup resolves
	for i := range 20 {
		if fork.Pending() == nil {
			t.Fatalf("the fork has no decision after %d answers", i)
		}
		answerFirst(t, fork.(*Game))
	}
	fork = nil
	if n := collectTables(start); n != start {
		t.Fatalf("the fork discarded too: %d tables registered, want %d", n, start)
	}
}
