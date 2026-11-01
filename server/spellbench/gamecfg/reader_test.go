package gamecfg_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

func TestReaderProbeMatchesGameProbe(t *testing.T) {
	reg := testcorpus.Registry(t)
	burn := deck(t, reg, "Burn")
	g, err := gamecfg.New(reg, secret(5), [2][]*cards.Card{burn, burn}, gamecfg.Rules{Mulligan: "london"})
	if err != nil {
		t.Fatal(err)
	}
	bots := [2]seat.Seat{heuristic.NewBot(1), heuristic.NewBot(2)}
	compared := 0
	for ; !g.E.G.Over && g.E.Pending() != nil && compared < 200; compared++ {
		d := g.E.Pending()
		in, _ := bots[d.Player].Decide(context.Background(), view.Project(g.E.G, g.E, d.Player, d), *d)
		head := g.E.L.Head()
		viaGame, err := g.Probe(in)
		if err != nil {
			t.Fatalf("decision %d: Game.Probe: %v", compared, err)
		}
		viaReader, err := g.R.Probe(in)
		if err != nil {
			t.Fatalf("decision %d: Reader.Probe: %v", compared, err)
		}
		if viaGame.G.Over != viaReader.Board().Over || !reflect.DeepEqual(viaGame.Pending(), viaReader.Pending()) {
			t.Fatalf("decision %d: the probes differ:\n game:   %+v\n reader: %+v", compared, viaGame.Pending(), viaReader.Pending())
		}
		if g.E.L.Head() != head {
			t.Fatalf("decision %d: probing moved the game", compared)
		}
		if err := g.Submit(in); err != nil {
			t.Fatal(err)
		}
	}
	if compared < 50 {
		t.Fatalf("compared %d decisions, want at least 50", compared)
	}
}
