package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestTraceBoardProjectionIsSortedAndRedacted(t *testing.T) {
	dmg := 3
	b := botpolicy.NewBoard(2)
	b.IsMain = true
	b.Pool = state.Mana{1, 2, 3, 4, 5, 6}
	b.Cards[9] = botpolicy.Card{Creature: true, Power: 4, CMC: 3, Basic: false, ManaCost: "2 R", Castable: true, Produces: cards.ManaProduction{Colour: [6]int32{0, 0, 0, 1, 0, 0}}}
	b.Cards[2] = botpolicy.Card{Basic: true, OnBattlefield: true}
	b.Life[1] = 17
	b.Life[0] = 20
	b.Creatures[8] = botpolicy.Creature{Power: 3, Toughness: 2, Damage: 1, Keywords: []string{"Haste"}, Tapped: true, Controller: 1}
	b.Creatures[3] = botpolicy.Creature{Power: 1, Toughness: 1, Controller: 0}
	b.Commanders[8] = botpolicy.Commander{Casts: 2, InCommandZone: false, Damage: map[state.PlayerID]int32{1: 7, 0: 2}}
	b.Stack = []botpolicy.StackEntry{{ID: 11, Controller: 1, IsSpell: true}}

	d := &decision.Decision{
		Seq: 12, Player: 0, Kind: decision.KTarget, Prompt: "SECRET PROMPT", Min: 1, Max: 1, Source: 99,
		TargetEffect: &decision.TargetEffect{API: "DealDamage", Damage: &decision.DamageEffect{Amount: &dmg}},
		Options: []decision.Option{{
			Index: 0, Kind: "target", Label: "SECRET LABEL", Obj: 8, Player: 1,
			Attacker: 3, Required: true, Group: "g", AltCostIndex: 2, Mode: "kicked",
			Amount: 4, Ability: 5, SVar: "SECRET_SVAR", Cost: "SECRET_COST",
			Grant: &decision.Grant{Keywords: []string{"Flying"}},
		}},
		ResumeKind: "SECRET_RESUME", Rolls: []int32{6}, ResumeMoved: []state.ObjID{7},
	}
	g := newGameTrace()
	if err := g.record(d, decision.Intent{Seq: 12, Player: 0, Choices: []int{0}}, &b, traceDecisionMeta{PairIndex: 4, Pair: "a:b", GameIndex: 6, Seed: 10, Policy: "bot"}); err != nil {
		t.Fatal(err)
	}
	got, err := json.Marshal(g.Decisions[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	for _, secret := range []string{"SECRET PROMPT", "SECRET LABEL", "SECRET_SVAR", "SECRET_COST", "SECRET_RESUME", `"source"`, `"rolls"`, `"grant"`} {
		if strings.Contains(s, secret) {
			t.Errorf("trace leaked %q: %s", secret, s)
		}
	}
	if !strings.Contains(s, `"card_id":2`) || strings.Index(s, `"card_id":2`) > strings.Index(s, `"card_id":9`) {
		t.Errorf("cards are not sorted by id: %s", s)
	}
	if !strings.Contains(s, `"player":0,"life":20`) || strings.Index(s, `"player":0,"life":20`) > strings.Index(s, `"player":1,"life":17`) {
		t.Errorf("life totals are not sorted by player: %s", s)
	}
	if !strings.Contains(s, `"target_effect":{"api":"DealDamage","damage":{"amount":3}}`) {
		t.Errorf("target effect missing: %s", s)
	}
}

func TestTraceSnapshotSurvivesBoardReuse(t *testing.T) {
	b := botpolicy.NewBoard(2)
	b.Cards[2] = botpolicy.Card{Power: 4, ManaCost: "2 G"}
	b.Creatures[3] = botpolicy.Creature{Power: 2, Keywords: []string{"Flying"}}
	b.Commanders[3] = botpolicy.Commander{Damage: map[state.PlayerID]int32{1: 5}}
	g := newGameTrace()
	d := &decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1, Options: []decision.Option{{Index: 0, Kind: "pass"}}}
	if err := g.record(d, decision.Intent{Seq: 1, Player: 0, Choices: []int{0}}, &b, traceDecisionMeta{}); err != nil {
		t.Fatal(err)
	}

	b.Cards[2] = botpolicy.Card{Power: 99, ManaCost: "SECRET"}
	b.Creatures[3] = botpolicy.Creature{Power: 99, Keywords: []string{"SECRET"}}
	b.Commanders[3].Damage[1] = 99
	delete(b.Cards, 2)

	got, err := json.Marshal(g.Decisions[0])
	if err != nil {
		t.Fatal(err)
	}
	s := string(got)
	if !strings.Contains(s, `"power":4`) || !strings.Contains(s, `"keywords":["Flying"]`) || !strings.Contains(s, `"damage":5`) {
		t.Fatalf("snapshot changed after board reuse: %s", s)
	}
	if strings.Contains(s, "SECRET") || strings.Contains(s, `"power":99`) {
		t.Fatalf("snapshot retained board aliases: %s", s)
	}
}

func TestTraceRejectsUnknownSchema(t *testing.T) {
	err := validateTraceRecord(traceDecisionV1{RecordType: "decision-v1", SchemaVersion: 2})
	if err == nil || !strings.Contains(err.Error(), "schema version") {
		t.Fatalf("validate unknown schema = %v, want schema version error", err)
	}
}
