package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// kddoCard lowers a synthetic creature whose one DamageDoneOnce trigger is
// body, with the given extra face SVars available.
func kddoCard(params map[string]string, svars map[string]string) []refusal {
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

func ddoBodyRead(rs []refusal) string {
	for _, r := range rs {
		if strings.Contains(r.Reason, "body reads") {
			return r.Reason
		}
	}
	return ""
}

// TestDdoReadsScopeToBodySVars: an SVar the body does not name is not the
// body's read. Yarus, Roar of the Old Gods' DamageDoneOnce body is only
// DB$ Draw; its TriggeredNewCardLKICopy spelling is its ChangesZone
// return's SVar. Reverting the scoped walk (scanning every face SVar)
// refuses the draw body below.
func TestDdoReadsScopeToBodySVars(t *testing.T) {
	const draw = "DB$ Draw | Defined$ You | NumCards$ 1"
	rs := kddoCard(map[string]string{"ValidTarget": "Creature"}, map[string]string{
		"Trig": draw,
		"Unrelated": "DB$ ChangeZone | Origin$ Graveyard | Destination$ Battlefield" +
			" | Defined$ TriggeredNewCardLKICopy",
	})
	if got := ddoBodyRead(rs); got != "" {
		t.Fatalf("unrelated SVar refused the draw body: %s", got)
	}
}

// TestDdoReadsFollowsBodySVars: a batch spelling in an SVar the body names
// directly (DigNum$ X, not SubAbility$) still refuses, and the carried
// batch total (X$ TriggerCount$DamageAmount) still lowers.
func TestDdoReadsFollowsBodySVars(t *testing.T) {
	const dig = "DB$ Dig | Defined$ You | DigNum$ X | ChangeNum$ All | DestinationZone$ Exile"
	rs := kddoCard(map[string]string{"ValidTarget": "Creature"}, map[string]string{
		"Trig": dig,
		"X":    "TriggerCount$DamageAmount",
	})
	if got := ddoBodyRead(rs); got != "" {
		t.Fatalf("the carried batch total refused: %s", got)
	}
	rs = kddoCard(map[string]string{"ValidTarget": "Creature"}, map[string]string{
		"Trig": dig,
		"X":    "TriggerCount$LifeAmount",
	})
	if got := ddoBodyRead(rs); !strings.Contains(got, "TriggerCount") {
		t.Fatalf("reachable SVar read not refused: %v", rs)
	}
}

// TestDdoTriggeredTargetControllerAdmitted: the spelling resolves to the
// damaged object's controller, which the kernel's per-referent queue binds
// as the trigger player only on an object referent.
func TestDdoTriggeredTargetControllerAdmitted(t *testing.T) {
	object := &lTrigger{Event: "DamageDone", PerRef: true, F2: &lFilter{Clauses: []lClause{{Types: tCreature}}}}
	player := &lTrigger{Event: "DamageDone", PerRef: true, F2: &lFilter{Players: plOpp}}
	if !ddoReadOK("TriggeredTargetController", object) {
		t.Error("TriggeredTargetController refused on an object referent")
	}
	if ddoReadOK("TriggeredTargetController", player) {
		t.Error("TriggeredTargetController admitted on a player referent")
	}
}

// TestDdoDigOptionalPrompt: PromptToSkipOptionalAbility$ and
// OptionalAbilityPrompt$ are the same optional take as Optional$ True
// (gorge effDig's promptToSkipOptional), read only beside it. Expedited
// Inheritance's Dig carries the first.
func TestDdoDigOptionalPrompt(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want string
	}{
		{"prompt", "DB$ Dig | Defined$ You | DigNum$ X | ChangeNum$ All | Optional$ True | PromptToSkipOptionalAbility$ True | DestinationZone$ Exile", ""},
		{"text", "DB$ Dig | Defined$ You | DigNum$ X | ChangeNum$ All | Optional$ True | OptionalAbilityPrompt$ Choose" + " | DestinationZone$ Exile", ""},
		{"no optional", "DB$ Dig | Defined$ You | DigNum$ X | ChangeNum$ All | PromptToSkipOptionalAbility$ True | DestinationZone$ Exile", "optional prompt without Optional$"},
	} {
		rs := kddoCard(map[string]string{"ValidTarget": "Creature"}, map[string]string{
			"Trig": c.body,
			"X":    "TriggerCount$DamageAmount",
		})
		found := false
		for _, r := range rs {
			found = found || strings.Contains(r.Reason, c.want)
		}
		if c.want == "" && len(rs) != 0 {
			t.Errorf("%s: refusals %v, want it to lower", c.name, rs)
		}
		if c.want != "" && !found {
			t.Errorf("%s: no refusal containing %q in %v", c.name, c.want, rs)
		}
	}
}
