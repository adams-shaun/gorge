package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// tvBuild lowers and builds one probe card, returning its CardDef and every
// lowering and build refusal.
func tvBuild(t *testing.T, lines ...string) (kop.CardDef, string) {
	t.Helper()
	src := strings.Join(append([]string{"Name:Probe"}, lines...), "\n") + "\n"
	card, diags := cards.ParseBytes("tv_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{"Probe"}, byName: map[string]*cards.Card{"Probe": card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	var rs []string
	for _, r := range append(lw.cards[0].Refusals, bs[0].refusals...) {
		rs = append(rs, r.Reason)
	}
	return bs[0].def, strings.Join(rs, "\n")
}

// TestTrekValueShapesLower: the value and count shapes lower, each with the
// reading gorge gives it.
func TestTrekValueShapesLower(t *testing.T) {
	creature := []string{"ManaCost:2 U", "Types:Creature Human", "PT:2/2"}
	for _, c := range []struct {
		name  string
		lines []string
	}{
		{"MultiplyCounter Multiplier$ 2 is the default doubling", []string{
			"A:AB$ MultiplyCounter | Cost$ T | ValidTgts$ Creature.YouCtrl | Multiplier$ 2"}},
		{"the active player's hand on an opponent's turn", []string{
			"T:Mode$ Phase | Phase$ End of Turn | ValidPlayer$ Player | TriggerZones$ Battlefield | Execute$ Zap",
			"SVar:Zap:DB$ DealDamage | Defined$ TriggeredPlayer | NumDmg$ 1 | ConditionOpponentTurn$ True | ConditionCheckSVar$ HandNow | ConditionSVarCompare$ GE3",
			"SVar:HandNow:Count$ValidHand Card.ActivePlayerCtrl"}},
		{"a triggered charm counted by Hellbent", []string{
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ Pick",
			"SVar:Pick:DB$ Charm | MinCharmNum$ 1 | Choices$ OneA,OneB | CharmNum$ Count$Hellbent.2.1",
			"SVar:OneA:DB$ Draw",
			"SVar:OneB:DB$ GainLife | LifeAmount$ 2"}},
		{"the opponent's life lost this turn in a repeat over opponents", []string{
			"T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ Each",
			"SVar:Each:DB$ RepeatEach | RepeatPlayers$ Player.Opponent | RepeatSubAbility$ Drain",
			"SVar:Drain:DB$ LoseLife | Defined$ Remembered | LifeAmount$ LostSoFar",
			"SVar:LostSoFar:PlayerCountRemembered$LifeLostThisTurn"}},
		{"damage to each creature and each player", []string{
			"A:AB$ DamageAll | Cost$ T | ValidCards$ Creature | ValidPlayers$ Player | NumDmg$ 1"}},
		{"historic beside a mana value", []string{
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | TriggerZones$ Battlefield | ValidCard$ Permanent.Other+Historic+YouCtrl+cmcGE3 | Execute$ Pull",
			"SVar:Pull:DB$ Draw"}},
	} {
		d, rs := tvBuild(t, append(append([]string(nil), creature...), c.lines...)...)
		if rs != "" {
			t.Errorf("%s: refused:\n%s", c.name, rs)
			continue
		}
		switch {
		case strings.HasPrefix(c.name, "a triggered charm"):
			if d.ModesPickV.K == 0 {
				t.Errorf("%s: no ModesPickV", c.name)
			}
		case strings.HasPrefix(c.name, "damage to each"):
			found := false
			for _, in := range d.Abilities[0].Prog {
				if in.Op == kop.OpDamageAll {
					found = in.B.N == 1
				}
			}
			if !found {
				t.Errorf("%s: no OpDamageAll with each player (B.N 1)", c.name)
			}
		}
	}
}

// TestTrekValueShapesRefuse: the neighbouring shapes keep refusing.
func TestTrekValueShapesRefuse(t *testing.T) {
	creature := []string{"ManaCost:2 U", "Types:Creature Human", "PT:2/2"}
	for _, c := range []struct {
		name, want string
		lines      []string
	}{
		{"a tripling multiplier", "Multiplier$", []string{
			"A:AB$ MultiplyCounter | Cost$ T | ValidTgts$ Creature.YouCtrl | Multiplier$ 3"}},
		{"a fixed triggered charm beside a counted one", "charm count", []string{
			"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ Pick",
			"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ Fixed",
			"SVar:Pick:DB$ Charm | MinCharmNum$ 1 | Choices$ OneA,OneB | CharmNum$ Count$Hellbent.2.1",
			"SVar:Fixed:DB$ Charm | Choices$ OneA,OneB",
			"SVar:OneA:DB$ Draw",
			"SVar:OneB:DB$ GainLife | LifeAmount$ 2"}},
		{"life lost this turn in a repeat over every player", "LifeAmount$", []string{
			"T:Mode$ Phase | Phase$ End of Turn | TriggerZones$ Battlefield | Execute$ Each",
			"SVar:Each:DB$ RepeatEach | RepeatPlayers$ Player | RepeatSubAbility$ Drain",
			"SVar:Drain:DB$ LoseLife | Defined$ Remembered | LifeAmount$ LostSoFar",
			"SVar:LostSoFar:PlayerCountRemembered$LifeLostThisTurn"}},
		{"each player beside a planeswalker clause", "non-creature clause", []string{
			"A:AB$ DamageAll | Cost$ T | ValidCards$ Creature,Planeswalker | ValidPlayers$ Player | NumDmg$ 1"}},
	} {
		_, rs := tvBuild(t, append(append([]string(nil), creature...), c.lines...)...)
		if !strings.Contains(rs, c.want) {
			t.Errorf("%s: refusals %q, want one naming %q", c.name, rs, c.want)
		}
	}
}
