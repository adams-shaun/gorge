package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// tacsLower lowers and builds one synthetic card; it returns the refusal
// reasons joined (empty: the card lowers whole).
func tacsLower(t *testing.T, src string) string {
	t.Helper()
	card, diags := cards.ParseBytes("probe.txt", []byte(src+"\n"))
	if card == nil || len(diags) != 0 {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{Tokens: map[string]*cards.Card{
		"w_1_1_rabbit":              {Faces: []*cards.Face{{Name: "Rabbit", Types: []string{"Creature", "Rabbit"}}}},
		"w_1_1_ninja_turtle_spirit": {Faces: []*cards.Face{{Name: "Spirit", Types: []string{"Creature", "Spirit"}}}},
	}}}
	lw := lowerPool(p)
	if _, _, _, err := buildPool(lw); err != nil {
		t.Fatal(err)
	}
	var rs []string
	for _, r := range lw.cards[0].Refusals {
		rs = append(rs, r.Reason)
	}
	return strings.Join(rs, "\n")
}

// TestTokenAttackingCharmSpell: TokenAttacking$ True lowers in a charm mode
// of the controller's own AttackersDeclared trigger (Warren Warleader) and in
// a spell with no trigger (The Last Ronin's Technique, a Branch on a sneaked
// count); a selector other than True and Adeline's, and a True in a trigger
// that is not the controller's own attack, still refuse.
func TestTokenAttackingCharmSpell(t *testing.T) {
	warren := `Name:Warren Warleader
ManaCost:2 W W
Types:Creature Rabbit Knight
PT:4/4
T:Mode$ AttackersDeclared | AttackingPlayer$ You | Execute$ TrigCharm | TriggerZones$ Battlefield | TriggerDescription$ Whenever you attack, ABILITY
SVar:TrigCharm:DB$ Charm | Choices$ DBToken,DBPumpAll
SVar:DBToken:DB$ Token | TokenScript$ w_1_1_rabbit | TokenOwner$ You | TokenTapped$ True | TokenAttacking$ True | SpellDescription$ Create a 1/1 white Rabbit creature token that's tapped and attacking.
SVar:DBPumpAll:DB$ PumpAll | ValidCards$ Creature.attacking+YouCtrl | NumAtt$ +1 | NumDef$ +1 | SpellDescription$ Attacking creatures you control get +1/+1 until end of turn.
Oracle:x`
	ronin := `Name:The Last Ronin's Technique
ManaCost:3 W
Types:Instant
K:Sneak:1 W
A:SP$ Branch | BranchConditionSVar$ X | BranchConditionSVarCompare$ GE1 | TrueSubAbility$ AttackingTokens | FalseSubAbility$ NormalTokens | SpellDescription$ Create three 1/1 tokens.
SVar:NormalTokens:DB$ Token | TokenAmount$ 3 | TokenScript$ w_1_1_ninja_turtle_spirit | TokenOwner$ You
SVar:AttackingTokens:DB$ Token | TokenAmount$ 3 | TokenScript$ w_1_1_ninja_turtle_spirit | TokenOwner$ You | TokenTapped$ True | TokenAttacking$ True
SVar:X:Count$ValidStack Card.Self+sneaked
Oracle:x`
	for _, c := range []struct{ name, script, want string }{
		{"warren", warren, ""},
		{"ronin", ronin, ""},
		{"selector", strings.Replace(ronin, "TokenAttacking$ True", "TokenAttacking$ Opponent", 1), "TokenAttacking$"},
		{"other trigger", strings.Replace(warren, "Mode$ AttackersDeclared | AttackingPlayer$ You", "Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self", 1), "TokenAttacking$ True"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got := tacsLower(t, c.script)
			switch {
			case c.want == "" && got != "":
				t.Fatalf("refused: %s", got)
			case c.want != "" && !strings.Contains(got, c.want):
				t.Fatalf("want a refusal naming %q, got %q", c.want, got)
			}
		})
	}
}
