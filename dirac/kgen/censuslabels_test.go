package kgen

import "testing"

// Pure string-to-label tests, kept apart from the census tests that
// drive real cards.

func TestRefineLabel(t *testing.T) {
	for _, tc := range []struct{ class, reason, want string }{
		{"kw:Ward", `DB$ Play: keyword "Ward"`, "kw:Ward"},
		{`other:Defined$ "_"`, `DB$ Play: Defined$ "ExiledWith"`, `Defined$ "ExiledWith"`},
		{`other:Defined$ "_"`, `DB$ Play: Defined$ "Valid Card.YouCtrl"`, `Defined$ "Valid <filter>"`},
		{`other:Defined$ "_"`, `DB$ Play: Defined$ "TriggeredCardLKICopyX"`, `Defined$ "TriggeredCardLKICopy<...>"`},
		{`other:filter "_": unknown filter head "_"`, `SP$ Effect: filter "Foo": unknown filter head "EACH A;B"`, `filter head "<A;B type list>"`},
		{`other:filter "_": unknown filter head "_"`, `SP$ Effect: filter "Foo": unknown filter head "EACH Foo"`, `filter head "EACH <...>"`},
		{`other:AffectedZone "_" without MayPlay`, `DB$ Effect: AffectedZone "Graveyard" without MayPlay`, `AffectedZone "Graveyard" without MayPlay`},
		{`other:phase "_"`, `DB$ Play: phase "12"`, `phase "N"`},
		{`other:Defined$ "_"`, `DB$ ChangeZone: Defined$ "ExiledWith"`, `Defined$ "ExiledWith"`},
		{`other:Defined$ "_"`, `DB$ ChangeZone: Defined$ "Valid Creature.YouCtrl"`, `Defined$ "Valid <filter>"`},
		{`other:filter "_": unknown filter head "_"`, `SP$ Effect: filter "x": unknown filter head "A;B"`, `filter head "<A;B type list>"`},
		{`other:AffectedZone "_" without MayPlay`, `DB$ Effect: AffectedZone "Stack" without MayPlay`, `AffectedZone "Stack" without MayPlay`},
		{`other:counter kind "_"`, `DB$ Effect: counter kind "P1P1" 12`, `counter kind "PNPN" N`},
	} {
		if got := refineLabel(tc.class, tc.reason); got != tc.want {
			t.Errorf("refineLabel(%q, %q) = %q, want %q", tc.class, tc.reason, got, tc.want)
		}
	}
}

func TestStdTier(t *testing.T) {
	for _, tc := range []struct{ class, want string }{
		{`other:Defined$ "_"`, "G"},
		{`other:filter "_": unknown filter head "_"`, "G"},
		{`other:static Condition "_"`, "G"},
		{`other:phase "_"`, "G"},
		{"other:counter cost", "G"},
		{"other:Effect triggers without a permanent duration", "O"},
		{"other:AlterAttribute Attributes$ \"_\"", "O"},
		{"other:mana foo", "G"},
		{"other:mana ability with a rider program", "O"},
		{`other:mana "_"`, "G"},
		{"other:Dig foo", "O"},
		{"other:MinMaxBlocker foo", "G"},
		{"other:Hidden$ foo", "G"},
		{"other:Remembered$ foo", "G"},
		{"api:ChangeTargets", "O"},
		{"api:Scry", "O"},
		{"trig:DamageDoneOnce", "M5"},
		{"cost:tapXType", "M13"},
		{"kw:Ward", "K"},
		{"other:Whatever", "?"},
	} {
		if got := stdTier(tc.class); got != tc.want {
			t.Errorf("stdTier(%q) = %q, want %q", tc.class, got, tc.want)
		}
	}
	if got := classTier("other:optional ChooseCard"); got != "?" {
		t.Errorf("classTier(other:optional ChooseCard) = %q, want ?", got)
	}
}
