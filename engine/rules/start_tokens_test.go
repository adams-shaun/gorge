package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	startBearToken     = "Name:Bear Token\nTypes:Creature Bear\nPT:1/1\nOracle:\n"
	startTreasureToken = "Name:Treasure Token\nTypes:Artifact Treasure\n" +
		"A:AB$ Mana | Cost$ T Sac<1/CARDNAME> | Produced$ Any | SpellDescription$ x\nOracle:\n"
	// startTokenWatch gains life whenever a token enters or you create one:
	// the triggers a scripted token must not fire.
	startTokenWatch = "Name:Token Watch\nManaCost:1\nTypes:Creature Elf\nPT:1/1\n" +
		"T:Mode$ ChangesZone | ValidCard$ Card.token | Destination$ Battlefield | Origin$ Any | TriggerZones$ Battlefield | Execute$ WatchGain | TriggerDescription$ x\n" +
		"T:Mode$ TokenCreated | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ WatchGain | TriggerDescription$ x\n" +
		"SVar:WatchGain:DB$ GainLife | LifeAmount$ 1 | Defined$ You\nOracle:x\n"
	// startTokenDoubler doubles the tokens and counters an effect makes: the
	// replacements a scripted placement must not go through.
	startTokenDoubler = "Name:Token Doubler\nManaCost:4 G\nTypes:Enchantment\n" +
		"R:Event$ CreateToken | ActiveZones$ Battlefield | ValidToken$ Card.YouCtrl | ReplaceWith$ DoubleToken | EffectOnly$ True | Description$ x\n" +
		"SVar:DoubleToken:DB$ ReplaceToken | Type$ Amount\n" +
		"R:Event$ AddCounter | ActiveZones$ Battlefield | ValidCard$ Permanent.YouCtrl+inZoneBattlefield | EffectOnly$ True | ReplaceWith$ DoubleCounters | Description$ x\n" +
		"SVar:DoubleCounters:DB$ ReplaceCounter | Amount$ Y\nSVar:Y:ReplaceCount$CounterNum/Twice\nOracle:x\n"
)

func startTokenConfig(t *testing.T, start *StartSpec) Config {
	t.Helper()
	cfg := startConfig(t, 11, start, [2]map[int]string{{2: startTokenWatch, 5: startTokenDoubler}, {}})
	cfg.Tokens = map[string]*cards.Card{
		"w_1_1_bear":       card(t, startBearToken),
		"c_a_treasure_sac": card(t, startTreasureToken),
	}
	return cfg
}

// TestScriptedStartTokens: StartSeat.Tokens mints each token on the seat's
// battlefield at genesis, tapped and with counters as asked, with no trigger,
// no tokens-created tally, and the same chain on every rebuild.
func TestScriptedStartTokens(t *testing.T) {
	start := &StartSpec{Seats: []StartSeat{
		{Battlefield: []int{2, 5}, Tokens: []StartToken{
			{Script: "w_1_1_bear", Counters: []StartTokenCounter{{Kind: "P1P1", N: 2}}},
			{Script: "c_a_treasure_sac", Tapped: true},
		}},
		{Tokens: []StartToken{{Script: "no_such_token"}}},
	}}
	cfg := startTokenConfig(t, start)
	e := New(cfg)
	var toks []*state.Object
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o.IsToken {
			toks = append(toks, o)
		}
	}
	if d := e.G.Obj(6); d.Zone != state.ZBattlefield || d.Face().Name != "Token Doubler" {
		t.Fatalf("precondition: the doubler is not on the battlefield (zone %v)", d.Zone)
	}
	if len(toks) != 2 {
		t.Fatalf("seat 0 tokens on the battlefield = %d, want 2", len(toks))
	}
	bear, treasure := toks[0], toks[1]
	if bear.Face().Name != "Bear Token" || treasure.Face().Name != "Treasure Token" {
		t.Fatalf("token order = %q, %q", bear.Face().Name, treasure.Face().Name)
	}
	if bear.Tapped || !treasure.Tapped {
		t.Errorf("tapped: bear %v treasure %v, want false/true", bear.Tapped, treasure.Tapped)
	}
	if n := bear.Counter("P1P1"); n != 2 {
		t.Errorf("bear P1P1 counters = %d, want 2", n)
	}
	if bear.Controller != 0 || treasure.Controller != 0 {
		t.Errorf("controllers %d/%d, want seat 0", bear.Controller, treasure.Controller)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 1) {
		if e.G.Obj(id).IsToken {
			t.Fatal("an unknown token script minted a token")
		}
	}
	for p := range e.G.Players {
		if n := e.G.Players[p].TokensCreatedThisTurn; n != 0 {
			t.Errorf("seat %d TokensCreatedThisTurn = %d after setup, want 0", p, n)
		}
	}
	life := e.G.Players[0].Life
	e.Advance()
	for _, ev := range e.L.Events {
		if ev.Kind == events.TriggerPush {
			t.Fatalf("a setup token fired a trigger: %+v", ev)
		}
	}
	if e.G.Players[0].Life != life {
		t.Errorf("life %d -> %d: a token trigger resolved", life, e.G.Players[0].Life)
	}
	// Control: a token minted in play does trigger the watcher, so the
	// silence above is the setup's.
	before := len(e.pendingTriggers)
	e.emit(events.Event{Kind: events.TokenCreate, Player: 0, Text: "w_1_1_bear"})
	if len(e.pendingTriggers) <= before {
		t.Fatal("control: an in-game token mint queued no watcher trigger")
	}
	// Replay: the same Config rebuilds the same chain.
	a, b := New(cfg), New(cfg)
	a.Advance()
	b.Advance()
	if a.L.Head() != b.L.Head() || len(a.L.Events) != len(b.L.Events) {
		t.Fatal("two builds of one scripted token start differ")
	}
}
