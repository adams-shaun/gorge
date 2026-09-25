package effects

import "testing"

func TestEffectMayPlayValidAfterStackRejectsBlankQualifier(t *testing.T) {
	for _, raw := range []string{"", "   \t"} {
		params := map[string]string{
			"MayPlay":         "True",
			"ValidAfterStack": raw,
		}
		if _, ok := mayPlayGrantFromLine(params); ok {
			t.Errorf("ordinary grant accepted present blank ValidAfterStack %q", raw)
		}

		freeParams := map[string]string{
			"MayPlay":                "True",
			"MayPlayWithoutManaCost": "True",
			"ValidAfterStack":        raw,
		}
		if _, ok := mayPlayFreeGrantFromLine(freeParams); ok {
			t.Errorf("free grant accepted present blank ValidAfterStack %q", raw)
		}
	}

	if _, ok := mayPlayGrantFromLine(map[string]string{
		"MayPlay": "True", "ValidAfterStack": "Spell.Equipment",
	}); !ok {
		t.Fatal("ordinary grant rejected valid ValidAfterStack")
	}
	if _, ok := mayPlayFreeGrantFromLine(map[string]string{
		"MayPlay": "True", "MayPlayWithoutManaCost": "True", "ValidAfterStack": "Spell.Equipment",
	}); !ok {
		t.Fatal("free grant rejected valid ValidAfterStack")
	}
}
