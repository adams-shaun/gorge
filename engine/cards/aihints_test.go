package cards

import "testing"

func TestParseAIHints(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want string
	}{
		{"RemoveDeck All", "AI:RemoveDeck:All\n", "All"},
		{"RemoveDeck Random", "AI:RemoveDeck:Random\n", "Random"},
		{"RemoveDeck NonCommander", "AI:RemoveDeck:NonCommander\n", "NonCommander"},
		{"RemoveDeckAll", "AI:RemoveDeckAll\n", "All"},
		{"RemoveDeckCard", "AI:RemoveDeckCard\n", "Card"},
		{"unrelated AI", "AI:RemoveDeckCardName:Foo\n", ""},
	}
	for _, tc := range cases {
		c, diags := ParseBytes(tc.name+".txt", []byte("Name:Test\n"+tc.src+"Types:Creature\n"))
		if len(diags) != 0 {
			t.Fatalf("%s: diags = %v", tc.name, diags)
		}
		if got := c.Faces[0].RemoveDeck; got != tc.want {
			t.Errorf("%s: RemoveDeck = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestPlayMain1Hint(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{PlayMain1Always, PlayMain1Always},
		{PlayMain1OwnCreatures, PlayMain1OwnCreatures},
		{PlayMain1OpponentCreatures, PlayMain1OpponentCreatures},
		{"FALSE", ""},
		{"", ""},
	}
	for _, tc := range cases {
		f := &Face{SVars: map[string]string{"PlayMain1": tc.value}}
		if got := f.PlayMain1Hint(); got != tc.want {
			t.Errorf("PlayMain1Hint(%q) = %q, want %q", tc.value, got, tc.want)
		}
	}
	if got := (*Face)(nil).PlayMain1Hint(); got != "" {
		t.Errorf("nil face PlayMain1Hint = %q, want empty", got)
	}
}

// TestParsePlayMain1Canonicalises pins the script-boundary conversion: every
// spelling lands in the closed set once, so the runtime reader never scans
// the raw value.
func TestParsePlayMain1Canonicalises(t *testing.T) {
	cases := []struct {
		value string
		want  string
	}{
		{"ALWAYS", PlayMain1Always},
		{"Always", PlayMain1Always},
		{"TRUE", PlayMain1OwnCreatures},
		{"True", PlayMain1OwnCreatures},
		{"Random", PlayMain1OwnCreatures},
		{"OPPONENTCREATURES", PlayMain1OpponentCreatures},
		{"OpponentCreatures", PlayMain1OpponentCreatures},
		{"FALSE", ""},
		{"", ""},
	}
	for _, tc := range cases {
		c, diags := ParseBytes("PlayMain1.txt", []byte("Name:Test\nSVar:PlayMain1:"+tc.value+"\nTypes:Creature\n"))
		if len(diags) != 0 {
			t.Fatalf("PlayMain1:%s: diags = %v", tc.value, diags)
		}
		if got := c.Faces[0].PlayMain1Hint(); got != tc.want {
			t.Errorf("PlayMain1:%s parses to hint %q, want %q", tc.value, got, tc.want)
		}
	}
}
