package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The T17c replacement-mode ports: each mode's census example cards lower.
// These are the sole-blocker carriers of their classes — a card listed here
// unlocks exactly when the named mode lowers beside everything else it
// prints, so the precondition (every listed card refuses only at its mode
// today) is what the RED run pins.
//
// TestReplModePortsLower: the BeginPhase phase-skip (ReplBeginPhase), the
// LifeReduced damage floor (ReplLifeReduced), the widened Counter CantHappen
// ValidSA grammar and the ActiveZones-empty printed prevention lower AND
// BUILD. gorge registers all four (engine/rules/replacement_phase.go,
// replacement.go's LifeReduced arm, replacement_counter.go, and the printed
// prevention path whose ActiveZones gate every carrier leaves empty).
func TestReplModePortsLower(t *testing.T) {
	for _, c := range []struct {
		mode  string
		names []string
	}{
		{"BeginPhase", []string{
			"Damia, Sage of Stone", "Dragon Appeasement", "Eon Hub", "Infernal Sovereign",
			"Null Profusion", "Recycle", "Stasis", "Symbiotic Deployment", "Wild Wasteland"}},
		{"LifeReduced", []string{
			"Ali from Cairo", "Elderscale Wurm", "Fortune Thief", "Sustaining Spirit", "Worship"}},
		{"Counter", []string{
			"Allosaurus Shepherd", "Cunning Nightbonder", "Leyline of Lifeforce", "Root Sliver", "Spellbreaker Behemoth"}},
		{"PreventionZones", []string{
			"Armament of Nyx", "Candletrap", "Demonic Torment", "Gaseous Form"}},
		{"DamageRewrite", []string{
			"Benevolent Unicorn", "Lashknife Barrier", "Divine Presence"}},
	} {
		t.Run(c.mode, func(t *testing.T) {
			for _, c := range lowerPoolBuilt(t, c.names) {
				for _, r := range c.refusals {
					t.Errorf("%s: refused: %s", c.name, r.Reason)
				}
			}
		})
	}
}

// TestReplModePortsRefuses: the shapes the new modes' bodies do not model
// refuse by name, fail-closed the way gorge reads them — an Optional$
// BeginPhase with a body parks gorge's phase ask, an unknown Phase$ value
// fails closed in phaseStep, a Hellbent gate is a hand read the kernel does
// not model on a replacement, a floor without IsDamage$ True would clamp a
// life cost, an unknown ValidSA qualifier fails closed in counterValidSA and
// an empty ActiveZones prevention with a player recipient would match from
// the hand gorge scans.
func TestReplModePortsRefuses(t *testing.T) {
	for _, c := range []struct {
		name   string
		params map[string]string
		want   string
	}{
		{"BeginPhase optional body", map[string]string{
			"Event": "BeginPhase", "ActiveZones": "Battlefield", "ValidPlayer": "You",
			"Phase": "Draw", "Layer": "Other", "Optional": "True",
			"ReplaceWith": "DBGainLife"}, "BeginPhase Optional"},
		{"BeginPhase unknown phase", map[string]string{
			"Event": "BeginPhase", "ActiveZones": "Battlefield", "Phase": "Combat",
			"Skip": "True"}, "BeginPhase Phase"},
		{"BeginPhase Hellbent", map[string]string{
			"Event": "BeginPhase", "ActiveZones": "Battlefield", "ValidPlayer": "You",
			"Phase": "Upkeep", "Skip": "True", "Hellbent": "True"}, "BeginPhase Hellbent"},
		{"BeginPhase command zone", map[string]string{
			"Event": "BeginPhase", "ActiveZones": "Command", "ValidPlayer": "You",
			"Phase": "Draw", "Skip": "True"}, "BeginPhase ActiveZones"},
		{"LifeReduced floor without IsDamage", map[string]string{
			"Event": "LifeReduced", "ActiveZones": "Battlefield", "ValidPlayer": "You.lifeGE1",
			"Result": "LT1", "ReplaceWith": "ReduceLoss"}, "LifeReduced IsDamage"},
		{"LifeReduced floor not damage-only Result", map[string]string{
			"Event": "LifeReduced", "ActiveZones": "Battlefield", "ValidPlayer": "You.lifeGE1",
			"IsDamage": "True", "Result": "LT2", "ReplaceWith": "ReduceLoss"}, "LifeReduced floor life"},
		{"Counter unknown qualifier", map[string]string{
			"Event": "Counter", "Layer": "CantHappen", "ActiveZones": "Battlefield",
			"ValidSA": "Spell.Green+YouCtrl+foo"}, `unknown property "foo"`},
		{"Counter SpellAbility kind", map[string]string{
			"Event": "Counter", "Layer": "CantHappen", "ActiveZones": "Battlefield",
			"ValidSA": "SpellAbility"}, "Counter replacement shape"},
		{"Prevention empty ActiveZones player recipient", map[string]string{
			"Event": "DamageDone", "Prevent": "True", "ValidTarget": "You",
			"IsCombat": "True", "ValidSource": "Creature"}, "DamageDone static prevention ActiveZones"},
	} {
		t.Run(c.name, func(t *testing.T) {
			sv := map[string]string{}
			if c.name == "LifeReduced floor not damage-only Result" {
				// the printed body the floor refuses on (its floor life 1, the
				// printed Result 2)
				sv = map[string]string{"ReduceLoss": "DB$ ReplaceEffect | VarName$ Amount | VarValue$ X",
					"X": "ReplaceCount$Amount/LimitMax.Xd", "Xd": "Count$YourLifeTotal/Minus.1"}
			}
			lc := replLowerSVars(t, c.params, sv)
			if len(lc.Refusals) == 0 || !strings.Contains(lc.Refusals[0].Reason, c.want) {
				t.Errorf("refusals %v, want one naming %q", lc.Refusals, c.want)
			}
		})
	}
}

// replLower lowers one synthesized R: line with the given params.
func replLower(t *testing.T, params map[string]string) *lCard {
	return replLowerSVars(t, params, nil)
}

func replLowerSVars(t *testing.T, params map[string]string, sv map[string]string) *lCard {
	t.Helper()
	rl := cards.Repl{Event: params["Event"], Params: params}
	if params["Event"] == "LifeReduced" && params["IsDamage"] == "True" {
		// the printed DB$ ReplaceEffect | VarName$ Amount body, With pre-bound
		// the way the card loader's SVar resolution does
		rl.With = &cards.SA{API: "ReplaceEffect", Params: map[string]string{"VarName": "Amount", "VarValue": "X"}}
	}
	face := &cards.Face{Name: "Synthetic", ManaCost: "1 W", Types: []string{"Enchantment"},
		Repls: []cards.Repl{rl}, SVars: sv}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false)
}
