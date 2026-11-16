package pickbot

import "testing"

// TestCommanderBuilderSources: on-identity mana rocks and dorks count as
// sources of the colours they produce (an any-colour one for each colour of
// the identity); a producer's colours outside the identity count nothing.
func TestCommanderBuilderSources(t *testing.T) {
	const w, u, g = 1 << 0, 1 << 1, 1 << 4
	cs := []cmdrCard{
		{src: w, rock: true},         // 0: a white rock
		{src: w | u, rock: true},     // 1: a white-blue dork
		{src: 0x1f, rock: true},      // 2: an any-colour rock
		{src: g, rock: true},         // 3: a green dork, off the identity
		{src: 0, rock: true},         // 4: a colourless rock
		{src: u, land: true},         // 5: an island
		{src: 0, rock: false},        // 6: a plain spell
		{src: 0, rock: false, mv: 5}, // 7: a producer too dear to count has no src
	}
	all := []int{0, 1, 2, 3, 4, 5, 6, 7}
	got := identitySources(cs, w|u, all)
	if want := [5]int{3, 3, 0, 0, 0}; got != want {
		t.Fatalf("W/U identity sources = %v, want %v", got, want)
	}
	if got := identitySources(cs, w|u, []int{3}); got != [5]int{} {
		t.Fatalf("an off-identity dork counted: %v", got)
	}
	if got := identitySources(cs, g, []int{0, 3}, []int{2}); got != [5]int{0, 0, 0, 0, 2} {
		t.Fatalf("mono-green sources = %v, want only the dork and the any-colour rock", got)
	}
}

// TestCommanderBuilderLandsByWidth: a 60-card deck plays 23 lands for one or
// two colours and one more per colour beyond two, capped at 24; the count
// scales with the deck size.
func TestCommanderBuilderLandsByWidth(t *testing.T) {
	for _, c := range []struct {
		name string
		mask uint8
		size int
		want int
	}{
		{"colourless", 0, 60, 23},
		{"1 colour", 0b00001, 60, 23},
		{"2 colours", 0b00011, 60, 23},
		{"3 colours", 0b00111, 60, 24},
		{"4 colours", 0b01111, 60, 24},
		{"5 colours", 0b11111, 60, 24},
		{"2 colours at 50", 0b00011, 50, cmdrLandsFor(50)},
		{"3 colours at 50", 0b00111, 50, 20},
	} {
		if got := cmdrLandsForWidth(c.size, c.mask); got != c.want {
			t.Errorf("%s: lands = %d, want %d", c.name, got, c.want)
		}
	}
	if cmdrLandsFor(50) != 19 {
		t.Errorf("cmdrLandsFor(50) = %d, want 19", cmdrLandsFor(50))
	}
}
