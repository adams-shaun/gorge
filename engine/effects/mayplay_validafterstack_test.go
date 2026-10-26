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

// TestEffectMayPlaySecondaryRegisters pins the display-only Secondary$ rider
// through the Effect route: Siphon Insight's STPlay line carries Secondary$
// True beside its implemented riders, and Forge reads Secondary only in the
// card text renderer (Card.java) -- so the grant builder must accept the
// line and register the real riders, never fail closed on the display bit.
func TestEffectMayPlaySecondaryRegisters(t *testing.T) {
	grant, ok := mayPlayGrantFromLine(map[string]string{
		"MayPlay":            "True",
		"MayPlayIgnoreColor": "True",
		"Affected":           "Card.IsRemembered",
		"AffectedZone":       "Exile",
		"Secondary":          "True",
		"Description":        "x",
	})
	if !ok {
		t.Fatal("Siphon Insight's STPlay line (Secondary$ True) refused by the Effect route")
	}
	if !grant.MayPlayIgnoreColor {
		t.Fatal("registered grant lost its MayPlayIgnoreColor rider")
	}
	if !grant.MayPlay {
		t.Fatal("registered grant is not a MayPlay grant")
	}
}

// TestEffectMayPlayGateKeysEffectRouteOnly pins the CheckSVar$/SVarCompare$
// whitelist direction: the Effect-delivered route accepts the gate pair and
// carries it on the registered effect (Possibility Technician, Flameshape),
// a bare SVarCompare$ with no CheckSVar$ is malformed and refused, and the
// printed route's MayPlayStaticParams still refuses the gate keys (its own
// gate is evaluated before the grant is built).
func TestEffectMayPlayGateKeysEffectRouteOnly(t *testing.T) {
	grant, ok := mayPlayGrantFromLine(map[string]string{
		"MayPlay": "True", "CheckSVar": "X", "SVarCompare": "GE1",
	})
	if !ok {
		t.Fatal("Effect route refused a CheckSVar$-gated may-play line")
	}
	if grant.MayPlayCheckSVar != "X" || grant.MayPlaySVarCompare != "GE1" {
		t.Fatalf("gate keys not carried: %+v", grant)
	}

	if _, ok := mayPlayGrantFromLine(map[string]string{
		"MayPlay": "True", "SVarCompare": "GE2",
	}); ok {
		t.Fatal("Effect route accepted a bare SVarCompare$ with no CheckSVar$")
	}

	if _, _, _, _, ok := MayPlayStaticParams(map[string]string{
		"MayPlay": "True", "CheckSVar": "X",
	}); ok {
		t.Fatal("printed route's MayPlayStaticParams accepted a CheckSVar$ line")
	}
	if _, _, _, _, ok := MayPlayStaticParams(map[string]string{
		"MayPlay": "True", "Secondary": "True",
	}); ok {
		t.Fatal("printed route's MayPlayStaticParams accepted a Secondary$ line")
	}
}

// TestEffectMayPlayBlankCheckSVarRefused pins the present-but-blank
// CheckSVar$ refusal: a blank gate key is unreadable, and accepting it would
// register the grant UNGATED (MayPlayCheckSVar trims to "" and the rules gate
// reads that as no gate) -- the printed route's convention is that
// present-but-empty fails closed.
func TestEffectMayPlayBlankCheckSVarRefused(t *testing.T) {
	for _, raw := range []string{"", "  \t"} {
		if _, ok := mayPlayGrantFromLine(map[string]string{
			"MayPlay": "True", "CheckSVar": raw,
		}); ok {
			t.Errorf("ordinary grant accepted present blank CheckSVar %q", raw)
		}
		if _, ok := mayPlayFreeGrantFromLine(map[string]string{
			"MayPlay": "True", "MayPlayWithoutManaCost": "True", "CheckSVar": raw,
		}); ok {
			t.Errorf("free grant accepted present blank CheckSVar %q", raw)
		}
	}
}

// TestAnimateMayPlayGrantRefusesGateKeys pins the Animate staticAbilities$
// route's gate-key refusal (Monk Class's StPlay): the shared grant builder
// now carries CheckSVar$ for the Effect route, but this route binds no
// resolving-face SVar table, so it must keep its loud unimplemented Note
// rather than silently register a grant whose gate can never evaluate.
func TestAnimateMayPlayGrantRefusesGateKeys(t *testing.T) {
	// Precondition: the line shape minus the gate key is accepted by the
	// same builder the Animate route calls, so the refusal below is
	// specifically the route's gate-key guard.
	if ce, ok := mayPlayGrantFromLine(map[string]string{
		"Affected": "Card.Self", "AffectedZone": "Exile", "MayPlay": "True",
	}); !ok || ce.MayPlayCheckSVar != "" {
		t.Fatalf("precondition: plain self/Exile line not accepted: %+v ok=%v", ce, ok)
	}
	for _, extra := range []map[string]string{
		{"CheckSVar": "YouCastThisTurn"},
		{"CheckSVar": "X", "SVarCompare": "GE1"},
	} {
		params := map[string]string{
			"Affected": "Card.Self", "AffectedZone": "Exile", "MayPlay": "True",
		}
		for k, v := range extra {
			params[k] = v
		}
		if _, ok := animateSelfMayPlayGrant(nil, NewCtxPtr(0, 0, CtxInit{}), 0, params); ok {
			t.Fatalf("Animate route registered a gate-keyed may-play grant without an SVar table: %v", extra)
		}
	}
}
