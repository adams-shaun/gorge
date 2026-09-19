package host

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/view"
)

func TestNormalizeBotPolicy(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{name: "empty defaults to bot", want: "bot"},
		{name: "production bot", in: "bot", want: "bot"},
		{name: "experimental lethal pressure", in: "lethal-pressure", want: "lethal-pressure"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := NormalizeBotPolicy(tc.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("NormalizeBotPolicy(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	for _, name := range []string{"legacy", "random", "Bot"} {
		t.Run("rejects "+name, func(t *testing.T) {
			_, err := NormalizeBotPolicy(name)
			if err == nil {
				t.Fatalf("NormalizeBotPolicy(%q) accepted an unsupported hosted policy", name)
			}
			if !strings.Contains(err.Error(), "bot, lethal-pressure") {
				t.Fatalf("error %q does not name the deterministic hosted policy set", err)
			}
		})
	}
}

func TestNewBotPolicySeatIsDeterministic(t *testing.T) {
	d := decision.Decision{Seq: 1, Player: 0, Kind: decision.KPriority, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: "pass"}, {Index: 1, Kind: "cast"}}}
	for _, policy := range []string{"bot", "lethal-pressure"} {
		t.Run(policy, func(t *testing.T) {
			a, err := NewBotPolicySeat(policy, 19)
			if err != nil {
				t.Fatal(err)
			}
			b, err := NewBotPolicySeat(policy, 19)
			if err != nil {
				t.Fatal(err)
			}
			ia, err := a.Decide(context.Background(), view.View{}, d)
			if err != nil {
				t.Fatal(err)
			}
			ib, err := b.Decide(context.Background(), view.View{}, d)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(ia.Choices, ib.Choices) {
				t.Fatalf("same seed chose %v and %v", ia.Choices, ib.Choices)
			}
		})
	}
}
