package rules

// cast_cda_flash_test.go pins CR 604.3: a characteristic-defining
// `AddKeyword$ Flash` static works in hand, so a conditional "this spell has
// flash" card is offered at instant speed while its condition holds, and not
// when it fails (the IsPresent$ recheck still gates it).

import "testing"

func TestCastCDAFlash(t *testing.T) {
	cases := []struct {
		name, setup, mana, card string
		want                    bool
	}{
		{"crashing-tide-merfolk", `{"p0":{"battlefield":["Merfolk Looter"],"hand":["Crashing Tide"]}}`, "UUUUUUU", "p0:Crashing Tide", true},
		{"crashing-tide-no-merfolk", `{"p0":{"hand":["Crashing Tide"]}}`, "UUUUUUU", "p0:Crashing Tide", false},
		{"rattlewurm-desert", `{"p0":{"battlefield":["Desert"],"hand":["Colossal Rattlewurm"]}}`, "GGGGGGG", "p0:Colossal Rattlewurm", true},
		{"rattlewurm-no-desert", `{"p0":{"hand":["Colossal Rattlewurm"]}}`, "GGGGGGG", "p0:Colossal Rattlewurm", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			runFlashZoneScenario(t, `{"name":"`+c.name+`","cr":["604.3"],"why":"inline","setup":`+c.setup+`,"steps":[`+
				flashZoneToP0Priority(c.mana, flashZoneOffered(c.card, c.want))+`]}`)
		})
	}
}
