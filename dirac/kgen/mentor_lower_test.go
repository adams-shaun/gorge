package kgen

import (
	"strings"
	"testing"
)

// mentorCarriers print K:Mentor (CR 702.134) on a creature; each lowers from
// gorge's kwMentor expansion, an Attacks self-trigger whose body is a targeted
// PutCounter carrying the Mentor$ lesser-power marker.
var mentorCarriers = []string{
	"Aurelia, Exemplar of Justice", "Barging Sergeant", "Blade Instructor", "Boros Challenger",
	"Goblin Banneret", "Hammer Dropper", "Light of the Legion",
	"Parhelion Patrol", "Sunhome Stalwart", "Tajic, Legion's Edge", "Truefire Captain",
	"Wojek Bodyguard", "Felisa, Fang of Silverquill", "Tributary Instructor",
}

func TestMentorLowers(t *testing.T) {
	for _, name := range mentorCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// Nyxborn Unicorn's bestowed "has mentor" lowers to a latent GrantedMentor
// trigger beside its printed K:Mentor line (CR 702.134b, 113.2c): the kernel
// lists one copy per granting static, so a printed Mentor wearing it and a
// vanilla creature wearing two each fire one trigger per instance, matching
// gorge after the granted-keyword fix.
func TestMentorGrantLowers(t *testing.T) {
	c := lowerNamedCard(t, "Nyxborn Unicorn")
	if len(c.Refusals) != 0 {
		t.Fatalf("Nyxborn Unicorn refusals = %v, want none", c.Refusals)
	}
	if lir := cardLIR(c); !strings.Contains(lir, "GrantedMentor") {
		t.Errorf("Nyxborn Unicorn LIR has no GrantedMentor trigger:\n%s", lir)
	}
}

// TestMentorGrantSplit: every bare Mentor term is its own instance; any other
// keyword stays in the list.
func TestMentorGrantSplit(t *testing.T) {
	for _, c := range []struct {
		kw, rest string
		n        int
	}{
		{"Mentor", "", 1},
		{"Mentor & Mentor", "", 2},
		{"Flying & Mentor", "Flying", 1},
		{"Flying", "Flying", 0},
		{"Mentor:1", "Mentor:1", 0},
	} {
		rest, n := splitMentorGrant(c.kw)
		if rest != c.rest || n != c.n {
			t.Errorf("splitMentorGrant(%q) = %q, %d; want %q, %d", c.kw, rest, n, c.rest, c.n)
		}
	}
}
