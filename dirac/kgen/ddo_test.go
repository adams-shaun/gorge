package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kddoRefusals lowers a synthetic creature whose one trigger is
// Mode$ DamageDoneOnce with the given extra parameters and body.
func kddoRefusals(params map[string]string, body string) []refusal {
	svars := map[string]string{"Trig": body}
	p2 := map[string]string{"Execute": "Trig", "TriggerZones": "Battlefield"}
	for k, v := range params {
		p2[k] = v
	}
	face := &cards.Face{
		Name:     "Synthetic",
		ManaCost: "1 R",
		Types:    []string{"Creature"},
		SVars:    svars,
		Triggers: []cards.Trigger{{Mode: "DamageDoneOnce", Params: p2, Effect: cards.ResolveSVar(svars, "Trig")}},
	}
	card := &cards.Card{Faces: []*cards.Face{face}}
	p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	return lowerCard(p, "Synthetic", card, false).Refusals
}

// TestDdoRefusals: every DamageDoneOnce shape the per-referent trigger does
// not carry exactly is a named refusal.
func TestDdoRefusals(t *testing.T) {
	const draw = "DB$ Draw | Defined$ You | NumCards$ 1"
	for _, c := range []struct {
		name   string
		params map[string]string
		body   string
		want   string
	}{
		{"mixed players and battle", map[string]string{"ValidTarget": "Player,Battle"}, draw, "mixes players and objects"},
		{"mixed you and planeswalker", map[string]string{"ValidTarget": "You,Planeswalker.YouCtrl"}, draw, "mixes players and objects"},
		{"first time", map[string]string{"ValidTarget": "Card.Self", "FirstTime": "True"}, draw, "FirstTime$"},
		{"non-literal amount", map[string]string{"ValidTarget": "Card.Self", "DamageAmount": "GEX"}, draw, "DamageAmount"},
		{"unknown comparator", map[string]string{"ValidTarget": "Card.Self", "DamageAmount": "XX3"}, draw, "DamageAmount"},
		{"triggered source", map[string]string{"ValidTarget": "Card.Self"}, "DB$ Destroy | Defined$ TriggeredSource", "body reads TriggeredSource"},
		// no ValidTarget$: the referent is an object or a player, so the
		// body's TriggeredTarget names neither exactly (a players-only
		// ValidTarget$ is the damaged player, trigger_referents10_test.go)
		{"triggered target", map[string]string{}, "DB$ Destroy | Defined$ TriggeredTarget", "body reads TriggeredTarget"},
		{"other trigger count", map[string]string{"ValidTarget": "Card.Self"}, "DB$ Draw | Defined$ You | NumCards$ X | X$ TriggerCount$LifeAmount", "body reads TriggerCount"},
	} {
		found := false
		for _, r := range kddoRefusals(c.params, c.body) {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if !found {
			t.Errorf("%s: no refusal containing %q", c.name, c.want)
		}
	}
}

// TestDdoSrcInPlay: a DamageDone ValidSource$ with inZoneBattlefield on every
// alternative sets the source-in-play flag, one without it does not, and a
// comma list carrying the word on only some alternatives is refused.
func TestDdoSrcInPlay(t *testing.T) {
	for _, c := range []struct {
		vs      string
		want    bool
		refused bool
	}{
		{"Creature.inZoneBattlefield", true, false},
		{"Creature", false, false},
		{"Creature.inZoneBattlefield,Planeswalker.inZoneBattlefield", true, false},
		{"Creature.inZoneBattlefield,Planeswalker", false, true},
	} {
		got, err := srcInPlay(c.vs)
		if got != c.want || (err != nil) != c.refused {
			t.Errorf("%q: got %v, %v; want %v, refused %v", c.vs, got, err, c.want, c.refused)
		}
	}
	// end to end through the lowering: the flag reaches the kernel trigger
	svars := map[string]string{"Trig": "DB$ Destroy | Defined$ TriggeredSource"}
	for vs, want := range map[string]bool{"Creature.inZoneBattlefield": true, "Creature": false} {
		p2 := map[string]string{"Execute": "Trig", "TriggerZones": "Battlefield", "ValidSource": vs, "ValidTarget": "You"}
		face := &cards.Face{Name: "Synthetic", ManaCost: "1 R", Types: []string{"Enchantment"}, SVars: svars,
			Triggers: []cards.Trigger{{Mode: "DamageDone", Params: p2, Effect: cards.ResolveSVar(svars, "Trig")}}}
		card := &cards.Card{Faces: []*cards.Face{face}}
		p := &pool{byName: map[string]*cards.Card{"Synthetic": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
		lc := lowerCard(p, "Synthetic", card, false)
		found := false
		for _, a := range lc.Abilities {
			if a.Trigger != nil {
				found = true
				if a.Trigger.SrcInPlay != want {
					t.Errorf("%q: SrcInPlay %v, want %v", vs, a.Trigger.SrcInPlay, want)
				}
			}
		}
		if !found {
			t.Errorf("%q: no trigger lowered (refusals %v)", vs, lc.Refusals)
		}
	}
}
