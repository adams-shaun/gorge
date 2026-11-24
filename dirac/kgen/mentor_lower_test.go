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

// A granted Mentor (Nyxborn Unicorn's bestowed "has mentor") stays refused,
// by that grant alone: gorge fires one Mentor trigger per creature that has
// the keyword however many sources give it (checkGrantedMentorTriggers skips a
// printer), while a kernel granted trigger fires once per granting static
// beside any printed line, so the two would split on a Mentor creature wearing
// the Unicorn. Its printed K:Mentor and attack trigger lower.
func TestMentorGrantStaysRefused(t *testing.T) {
	c := lowerNamedCard(t, "Nyxborn Unicorn")
	if len(c.Refusals) != 1 {
		t.Fatalf("Nyxborn Unicorn refusals = %v, want exactly the granted keyword", c.Refusals)
	}
	if r := c.Refusals[0]; !strings.Contains(r.Node, "Mode=Continuous") || !strings.Contains(r.Reason, `keyword "Mentor"`) {
		t.Errorf("Nyxborn Unicorn refusal = %+v, want the Continuous grant of Mentor", r)
	}
}
