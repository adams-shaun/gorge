package seat

import (
	"context"
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/server/matchrecord"
	sbenv "github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
)

func TestGameStartNamesTheCardPool(t *testing.T) {
	argv, record := recordingMinimal(t)
	reg := testcorpus.Registry(t)
	sbenv.SetCorpusRef(matchrecord.ForgeRef(testcorpus.Dir(t)))
	e, _, _ := testEngine(t)
	raw, err := New("test-agent", argv, bots.Options{})
	if err != nil {
		t.Fatal(err)
	}
	agent := raw.(*Agent)
	if err := agent.AttachMatch(&bots.MatchContext{
		Context: context.Background(), Game: gorgegame.Game{E: e}, Registry: reg,
		Seats: 2, Format: "constructed", Seed: []byte("identity"), OwnDeck: e.OwnDeck(0),
	}); err != nil {
		t.Fatal(err)
	}
	agent.EndMatch(bots.MatchResult{Classification: "halted"})
	f, err := os.Open(record)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var start struct {
		Engine struct {
			CardPoolIdentity string `json:"card_pool_identity"`
		} `json:"engine"`
	}
	lines := json.NewDecoder(f)
	for i := 0; i < 2; i++ { // hello, then game_start
		if err := lines.Decode(&start); err != nil {
			t.Fatal(err)
		}
	}
	if !regexp.MustCompile(`^forge-[0-9a-f]{12}/corpus-[0-9a-f]{16}/catalog-[0-9a-f]{16}$`).MatchString(start.Engine.CardPoolIdentity) {
		t.Fatalf("card_pool_identity %q", start.Engine.CardPoolIdentity)
	}
}
