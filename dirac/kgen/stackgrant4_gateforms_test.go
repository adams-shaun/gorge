package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// stackGrantProbeLowered lowers one synthetic card and builds the result.
func stackGrantProbeLowered(t *testing.T, src string) (*lowered, error) {
	t.Helper()
	card, diags := cards.ParseBytes("grant_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Grant Probe"}, byName: map[string]*cards.Card{"Grant Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, _, _, err := buildPool(lw)
	return lw, err
}

// stackGrantProbe lowers one synthetic stack-grant static and builds the
// result. svarName/svarBody add one SVar the gate reads.
func stackGrantProbe(t *testing.T, static, svarName, svarBody string) (*lCard, error) {
	t.Helper()
	src := "Name:Grant Probe\nManaCost:3 R G\nTypes:Enchantment\n" + static + "\n"
	if svarName != "" {
		src += "SVar:" + svarName + ":" + svarBody + "\n"
	}
	lw, err := stackGrantProbeLowered(t, src)
	return lw.cards[0], err
}

// TestStackGrantGateForms is the shape census of the four stack-grant forms
// gap4-06 lowers: a Condition$/IsPresent$/CheckSVar$ gate on a cast-time head,
// YouCtrl with no cast word, the wasCastFromHand / wasCastFromExile
// provenance words, and "A & A" (two instances of the one head). It also
// pins the named refusals that stay: an unsupported head, a mixed keyword
// grant, a Treasure-provenance word, a gate body the condition lowering
// cannot represent, and a gate on Convoke, Improvise, Lifelink or Split
// second (gorge reads those at announcement/damage/priority time, not with
// the in-flight cast excluded).
func TestStackGrantGateForms(t *testing.T) {
	const (
		cascade = "S:Mode$ Continuous | Affected$ %s | AffectedZone$ Stack | AddKeyword$ %s | EffectZone$ Battlefield"
	)
	cases := []struct {
		name     string
		aff      string
		kw       string
		extra    string
		svarName string
		svarBody string
		want     string // a refusal substring; "" = lowers
		statics  int    // when it lowers
		flags    string
		cond     bool
	}{
		{name: "PlayerTurn gate", aff: "Card.YouCtrl+YouOwn+wasCastFromHand+cmcLEX", kw: "Cascade",
			extra: " | Condition$ PlayerTurn", svarName: "X", svarBody: "Count$LifeOppsLostThisTurn", statics: 1, cond: true},
		{name: "CheckSVar gate", aff: "Instant.YouCtrl,Sorcery.YouCtrl", kw: "Casualty:2",
			extra: " | CheckSVar$ X | SVarCompare$ EQ0", svarName: "X", svarBody: "Count$ThisTurnCast_Instant.YouCtrl,Sorcery.YouCtrl", statics: 1, cond: true},
		{name: "IsPresent gate", aff: "Card.Artifact+nonLegendary+YouCtrl", kw: "Casualty:2",
			extra: " | IsPresent$ Artifact.YouCtrl", statics: 1, cond: true},
		{name: "YouCtrl no cast word", aff: "Card.nonCreature+YouCtrl", kw: "Cascade", statics: 1},
		{name: "wasCastFromHand", aff: "Enchantment.YouCtrl+YouOwn+wasCastFromHand", kw: "Cascade", statics: 1, flags: "CastHand"},
		{name: "wasCastFromExile", aff: "Card.YouCtrl+wasCastFromExile", kw: "Convoke", statics: 1, flags: "CastExile"},
		{name: "not from hand", aff: "Card.YouCtrl+!wasCastFromYourHand", kw: "Demonstrate",
			extra: " | CheckSVar$ X | SVarCompare$ EQ0", svarName: "X", svarBody: "Count$ThisTurnCast_Card.YouCtrl+!wasCastFromYourHand", statics: 1, flags: "CastNotHand", cond: true},
		{name: "A & A", aff: "Card.Colorless+YouCtrl+YouOwn+wasCastFromHand+cmcGE7", kw: "Cascade & Cascade", statics: 2, flags: "CastHand"},
		{name: "gated Convoke", aff: "Card.Historic+YouCtrl", kw: "Convoke",
			extra: " | CheckSVar$ X | SVarCompare$ EQ0", svarName: "X", svarBody: "Count$ThisTurnCast_Card.Historic+YouCtrl",
			want: "AffectedZone Stack Convoke grant with a condition"},
		{name: "gated Improvise", aff: "Card.nonArtifact+YouCtrl", kw: "Improvise",
			extra: " | CheckSVar$ X | SVarCompare$ EQ0", svarName: "X", svarBody: "Count$ThisTurnCast_Card.nonArtifact+YouCtrl",
			want: "AffectedZone Stack Improvise grant with a condition"},
		{name: "gated Lifelink", aff: "Card.YouCtrl", kw: "Lifelink",
			extra: " | IsPresent$ Creature.YouCtrl", want: "AffectedZone Stack Lifelink grant with a condition"},
		{name: "gated Split second", aff: "Card.YouCtrl", kw: "Split second",
			extra: " | IsPresent$ Creature.YouCtrl", want: "AffectedZone Stack Split second grant with a condition"},
		{name: "unsupported head", aff: "Card.YouCtrl", kw: "Rebound", want: `AffectedZone Stack grant of "Rebound"`},
		{name: "mixed keywords", aff: "Card.YouCtrl", kw: "Cascade & Convoke", want: `AffectedZone Stack grant of "Cascade & Convoke"`},
		{name: "treasure provenance", aff: "Card.YouCtrl+CastSa Spell.ManaFromTreasure", kw: "Cascade", want: `Affected$ word "CastSa Spell.ManaFromTreasure"`},
		{name: "gate body not lowered", aff: "Card.YouCtrl", kw: "Cascade", extra: " | Condition$ SomeUnreadGate", want: `static Condition "SomeUnreadGate"`},
	}
	for _, c := range cases {
		body := strings.Replace(cascade, "%s", c.aff, 1)
		body = strings.Replace(body, "%s", c.kw, 1) + c.extra
		lc, err := stackGrantProbe(t, body, c.svarName, c.svarBody)
		var refusals []string
		for _, r := range lc.Refusals {
			refusals = append(refusals, r.Reason)
		}
		all := strings.Join(refusals, " | ")
		if c.want != "" {
			if !strings.Contains(all, c.want) {
				t.Errorf("%s: refusals %q, want one containing %q", c.name, all, c.want)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: build: %v (refusals %q)", c.name, err, all)
			continue
		}
		if len(refusals) != 0 {
			t.Errorf("%s: refused: %s", c.name, all)
			continue
		}
		if len(lc.Statics) != c.statics {
			t.Errorf("%s: %d statics, want %d: %+v", c.name, len(lc.Statics), c.statics, lc.Statics)
			continue
		}
		wantKw := c.kw
		if c.statics > 1 {
			wantKw = strings.Split(c.kw, " & ")[0]
		}
		for _, st := range lc.Statics {
			if st.Kind != "StackKw" && st.Kind != "ReduceCost" {
				t.Errorf("%s: static kind %q", c.name, st.Kind)
			}
			if strings.Join(st.Kw, ",") != wantKw {
				t.Errorf("%s: kw %v, want %q", c.name, st.Kw, wantKw)
			}
			if c.flags != "" && (len(st.Flags) != 1 || st.Flags[0] != c.flags) {
				t.Errorf("%s: flags %v, want [%s]", c.name, st.Flags, c.flags)
			}
			if got := st.Cond.Kind != cNone; got != c.cond {
				t.Errorf("%s: condition present = %v, want %v", c.name, got, c.cond)
			}
		}
	}
	// EffectZone$ Stack self grant (Bloodbraid Marauder): the static applies
	// from the spell's own stack entry under its Delirium gate.
	self, err := stackGrantProbe(t, "S:Mode$ Continuous | Affected$ Card.Self | EffectZone$ Stack | AffectedZone$ Stack | AddKeyword$ Cascade | Condition$ Delirium", "", "")
	if err != nil {
		t.Fatalf("self stack grant: build: %v (refusals %v)", err, self.Refusals)
	}
	if len(self.Refusals) != 0 {
		t.Fatalf("self stack grant: refused: %v", self.Refusals)
	}
	if len(self.Statics) != 1 || self.Statics[0].Cond.Kind == cNone ||
		len(self.Statics[0].Flags) != 1 || self.Statics[0].Flags[0] != "SelfStack" {
		t.Fatalf("self stack grant: statics %+v, want one gated SelfStack static", self.Statics)
	}
	// an Effect-issued grant (Archway of Innovation, Dark Apostle): the
	// Effect's static lowers to StackKw and its ForgetOnCast$ ends the emblem
	// on the matching cast (kop.Emblem.Forget)
	eff, err := stackGrantProbeLowered(t, "Name:Grant Probe\nManaCost:3 R\nTypes:Creature\nPT:3/3\n"+
		"A:AB$ Effect | Cost$ 3 T | StaticAbilities$ GrantCascade | ForgetOnCast$ Card.nonCreature+YouCtrl\n"+
		"SVar:GrantCascade:Mode$ Continuous | Affected$ Card.nonCreature+YouCtrl | AffectedZone$ Stack | AddKeyword$ Cascade\n")
	if err != nil {
		t.Fatalf("effect grant: build: %v (refusals %v)", err, eff.cards[0].Refusals)
	}
	if len(eff.cards[0].Refusals) != 0 {
		t.Fatalf("effect grant: refused: %v", eff.cards[0].Refusals)
	}
	if len(eff.cards[0].Emblems) != 1 || eff.cards[0].Emblems[0].Forget == nil || len(eff.cards[0].Emblems[0].Statics) != 1 ||
		eff.cards[0].Emblems[0].Statics[0].Kind != "StackKw" {
		t.Fatalf("effect grant: emblems %+v, want one StackKw emblem with a Forget filter", eff.cards[0].Emblems)
	}
}
