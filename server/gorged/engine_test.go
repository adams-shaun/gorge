package gorged

import (
	"context"
	"net"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/view"
)

// otherEngine stands in for an engine a binary links beside gorge.
type otherEngine struct{ game.Engine }

func (otherEngine) Name() string { return "other" }

func TestEngineFlagDefaultsToGorge(t *testing.T) {
	f := ServeFlagSet().Lookup("engine")
	if f == nil || f.DefValue != gorgegame.Name {
		t.Fatalf("-engine flag: %+v", f)
	}
}

// An engine the binary does not link is refused before the corpus is opened,
// naming what it does link.
func TestEngineNotLinkedFailsAtStartup(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	ext := Extensions{Engines: map[string]game.Engine{"other": otherEngine{}}}
	err = Serve(context.Background(), []string{"-engine", "missing", "-cards", t.TempDir()}, ln, ext)
	if err == nil || !strings.Contains(err.Error(), `engine "missing"`) || !strings.Contains(err.Error(), "available: gorge, other") {
		t.Fatalf("Serve: %v", err)
	}
}

func TestEngineFlagReachesTables(t *testing.T) {
	c := config{tables: 2, seats: 2, engine: "other"}
	for _, cfg := range c.tableConfigs(nil, []string{"a"}, view.Public) {
		if cfg.Engine != "other" {
			t.Fatalf("table %s engine %q", cfg.ID, cfg.Engine)
		}
	}
	if got := c.capabilityEngine(); got != "other" {
		t.Fatalf("capability engine %q", got)
	}
	if got := (config{engine: gorgegame.Name}).capabilityEngine(); got != "" {
		t.Fatalf("gorge's capability engine %q", got)
	}
}
