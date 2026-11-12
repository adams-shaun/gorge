package oracle

import (
	"fmt"
	"path/filepath"
	"testing"
)

// TestCmd5IdentityManaFundsDarksteelJuggernaut pins the hunt games the
// commander-identity mana fix closed: two where Arcane Signet adds a colour of
// its controller's commander's identity (CR 903.4) and so funds Darksteel
// Juggernaut's {5}, and one where Commander's Sphere does the same for
// Unnatural Growth. The kernel used to lower such a source as colourless, so
// it offered four-mana casts but not the one the fifth mana pays for.
func TestCmd5IdentityManaFundsDarksteelJuggernaut(t *testing.T) {
	t.Parallel()
	games := []struct {
		name  string
		seat  int      // the player whose priority decision diverged
		cast  string   // the option only the reference offered there
		cards []string // the identity source and the cast it funds, in that deck
	}{
		{
			name:  "cmd5_identity_mana_funds_darksteel_juggernaut_s28p9",
			seat:  1,
			cast:  "cast(#111,-,0)",
			cards: []string{"Arcane Signet", "Darksteel Juggernaut"},
		},
		{
			name:  "cmd5_identity_mana_funds_darksteel_juggernaut_s34p10",
			seat:  1,
			cast:  "cast(#111,-,0)",
			cards: []string{"Arcane Signet", "Darksteel Juggernaut"},
		},
		{
			name:  "cmd5_identity_mana_funds_unnatural_growth_s15p15",
			seat:  0,
			cast:  "cast(#94,-,0)",
			cards: []string{"Commander's Sphere", "Unnatural Growth"},
		},
	}
	for _, g := range games {
		g := g
		t.Run(g.name, func(t *testing.T) {
			t.Parallel()
			r, err := ReadRepro(filepath.Join("testdata", "repro", g.name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}

			// The precondition: the recording is the divergence this test
			// pins, not merely a clean game. The seats differ by exactly the
			// cast the identity source pays for, and that seat's deck holds
			// both the source and the card.
			d := r.Divergence
			if d == nil || d.What != "options" {
				t.Fatalf("divergence = %+v, want a recorded options difference", d)
			}
			if want := fmt.Sprintf("priority decision for p%d", g.seat); d.Detail != want {
				t.Fatalf("divergence is %q, want %q", d.Detail, want)
			}
			for _, name := range g.cards {
				if !stringIn(r.Cards[g.seat], name) {
					t.Fatalf("p%d's deck has no %s; cards = %v", g.seat, name, r.Cards[g.seat])
				}
			}
			if diff := optionsOnlyIn(d.RefOptions, d.KerOptions); len(diff) != 1 || diff[0] != g.cast {
				t.Fatalf("reference-only options = %v, want exactly %s", diff, g.cast)
			}

			// The assertion: on the recorded surface the game now replays in
			// lockstep to its end without a kernel stall.
			stdReproNamed(t, g.name)
		})
	}
}

// optionsOnlyIn is the options a reference list offered that the kernel's did
// not, in reference order.
func optionsOnlyIn(ref, ker []string) []string {
	in := make(map[string]bool, len(ker))
	for _, o := range ker {
		in[o] = true
	}
	var out []string
	for _, o := range ref {
		if !in[o] {
			out = append(out, o)
		}
	}
	return out
}

// stringIn reports whether ss holds s.
func stringIn(ss []string, s string) bool {
	for _, x := range ss {
		if x == s {
			return true
		}
	}
	return false
}
