package draftlab

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestParseAgent(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    AgentRef
		wantErr bool
	}{
		{in: "bot", want: AgentRef{Name: "bot"}},
		{in: "bot@1", want: AgentRef{Name: "bot", Version: "1"}},
		{in: "", wantErr: true},
		{in: "@1", wantErr: true},
		{in: "bot@", wantErr: true},
		{in: "a@b@c", wantErr: true},
	} {
		got, err := ParseAgent(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseAgent(%q) accepted", tc.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAgent(%q): %v", tc.in, err)
			continue
		}
		if got != tc.want {
			t.Errorf("ParseAgent(%q) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
	// String round-trips.
	if s := (AgentRef{Name: "bot", Version: "1"}).String(); s != "bot@1" {
		t.Errorf("String = %q, want bot@1", s)
	}
	if s := (AgentRef{Name: "bot"}).String(); s != "bot" {
		t.Errorf("String = %q, want bot", s)
	}
}

func TestAgentResolution(t *testing.T) {
	l := testLab(t)

	// A bare ref resolves to the table's version and records it.
	d := testDrafter(t, l, "rating-colour")
	if d.Ref != (AgentRef{Name: "rating-colour", Version: "1"}) {
		t.Errorf("drafter ref = %+v, want rating-colour@1", d.Ref)
	}
	if d.Drafter == nil {
		t.Error("drafter instance is nil")
	}
	// An explicit matching version passes through.
	d2, err := l.Drafter(AgentRef{Name: "rating-colour", Version: "1"})
	if err != nil || d2.Ref.Version != "1" {
		t.Errorf("explicit version: %+v %v", d2.Ref, err)
	}
	// A mismatched version is refused.
	if _, err := l.Drafter(AgentRef{Name: "rating-colour", Version: "2"}); !errors.Is(err, ErrVersion) {
		t.Errorf("mismatched version err = %v, want ErrVersion", err)
	}
	// A name neither registry nor table knows is ErrUnknownAgent.
	if _, err := l.Drafter(AgentRef{Name: "no-such-agent"}); !errors.Is(err, ErrUnknownAgent) {
		t.Errorf("unknown drafter err = %v, want ErrUnknownAgent", err)
	}
	// A test-registered pickbot name resolves with Version "" to
	// "unversioned" (the registry alone knows the version; nothing to
	// fingerprint).
	const probe = "lab-probe-agent"
	pickbotRegisterForTest(t, probe)
	pd, err := l.Drafter(AgentRef{Name: probe})
	if err != nil {
		t.Fatalf("probe drafter: %v", err)
	}
	if pd.Ref.Version != unversioned {
		t.Errorf("probe drafter version = %q, want %q", pd.Ref.Version, unversioned)
	}
	if _, err := l.Drafter(AgentRef{Name: probe, Version: "1"}); !errors.Is(err, ErrVersion) {
		t.Errorf("unlisted name with explicit version err = %v, want ErrVersion", err)
	}

	// The builder role mirrors the drafter role.
	b := testBuilder(t, l, "best23")
	if b.Ref != (AgentRef{Name: "best23", Version: "1"}) {
		t.Errorf("builder ref = %+v, want best23@1", b.Ref)
	}
	if _, err := l.Builder(AgentRef{Name: "best23", Version: "9"}); !errors.Is(err, ErrVersion) {
		t.Errorf("mismatched builder version err = %v, want ErrVersion", err)
	}

	// An unknown player policy is ErrUnknownAgent.
	if _, err := l.Player(AgentRef{Name: "no-such-policy"}); !errors.Is(err, ErrUnknownAgent) {
		t.Errorf("unknown player err = %v, want ErrUnknownAgent", err)
	}

	// Players: the bot policy resolves; an Env policy is refused without the
	// opt-in and admitted (ReadsDecklists true) with it.
	bot, err := l.Player(AgentRef{Name: "bot"})
	if err != nil {
		t.Fatalf("Player(bot): %v", err)
	}
	if bot.Ref != (AgentRef{Name: "bot", Version: "1"}) {
		t.Errorf("player ref = %+v, want bot@1", bot.Ref)
	}
	if st, err := bot.New(1); err != nil || st == nil {
		t.Errorf("player New: seat %v err %v", st, err)
	}
	if _, err := l.Player(AgentRef{Name: "sb-tactical"}); !errors.Is(err, ErrDecklistBot) {
		t.Errorf("Env policy without opt-in: err = %v, want ErrDecklistBot", err)
	}
	l2, err := Open(testutil.CorpusDirOrSkip(t), Options{AllowDecklistBots: true})
	if err != nil {
		t.Fatal(err)
	}
	env, err := l2.Player(AgentRef{Name: "sb-tactical"})
	if err != nil {
		t.Fatalf("Env policy with opt-in: %v", err)
	}
	if !env.ReadsDecklists {
		t.Error("Env policy with opt-in: ReadsDecklists = false, want true")
	}
}

// pickbotRegisterForTest registers a throwaway drafter name under the
// pickbot registry so the lab's unversioned path can be exercised.
// Re-registering replaces the entry (registry.go), so repeats are safe;
// the name is test-only and no other test resolves it.
func pickbotRegisterForTest(t *testing.T, name string) {
	t.Helper()
	pickbot.Register(name, func(lookup pickbot.CardLookup) pickbot.Drafter {
		return &pickbot.RatingColour{Lookup: lookup}
	}, func(lookup pickbot.CardLookup) pickbot.Builder {
		return &pickbot.Best23{Lookup: lookup}
	})
}
