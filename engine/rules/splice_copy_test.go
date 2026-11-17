package rules

import "testing"

// CR 707.10 + 702.47c: a copy of a spell copies its characteristics, and
// spliced text is one of them, so the copy draws too.
//
// Zada copies Into the Fray (Evermind spliced on) for the Ox: the original and
// the copy each resolve Evermind's "draw a card".
func TestCopyOfASplicedSpellKeepsTheSplicedText(t *testing.T) {
	t.Run("Zada copy per creature", func(t *testing.T) {
		playRuleScenario(t, `{"name":"zada-spliced","cr":["707.10","702.47c"],"why":"XMage CopySpellTest#ZadaHedronGrinderAndSplicedSpell",
		"setup":{"p0":{"hand":["Into the Fray","Evermind"],"battlefield":["Mountain","Island","Island","Zada, Hedron Grinder","Pillarfield Ox"]}},
		"steps":[{"op":"cast","seat":0,"card":"p0:Into the Fray","mana":"RUU","targets":["p0:Zada, Hedron Grinder"],"answers":[{"kind":"choose","pick":["Evermind"]}]},
		{"op":"resolve"},{"op":"resolve"},{"op":"resolve"}],
		"expect":[{"hand_size":{"p0":3}}]}`)
	})
	t.Run("Twincast copy", func(t *testing.T) {
		playRuleScenario(t, `{"name":"twincast-spliced","cr":["707.10","702.47c"],"why":"a plain copy effect keeps the spliced text",
		"setup":{"p0":{"hand":["Into the Fray","Evermind","Twincast"],"battlefield":["Mountain","Island","Island","Island","Island","Pillarfield Ox"]}},
		"steps":[{"op":"cast","seat":0,"card":"p0:Into the Fray","mana":"RUU","targets":["p0:Pillarfield Ox"],"answers":[{"kind":"choose","pick":["Evermind"]}]},
		{"op":"cast","seat":0,"card":"p0:Twincast","mana":"UU","targets":["p0:Into the Fray"]},
		{"op":"resolve"},{"op":"resolve"},{"op":"resolve"}],
		"expect":[{"hand_size":{"p0":3}}]}`)
	})
}
