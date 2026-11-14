package kgen

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// damageAmountGate lowers DamageAmount$ <cmp><literal> on a plain Mode$
// DamageDone trigger (Sahir, Visitor in Darkness: "a source you control deals
// exactly 1 damage to you", DamageAmount$ EQ1). gorge compares each damage
// event's own amount (trigmatch.DamageAmountMatches, the per-event modes), and
// the kernel reads the gate where it reads the recipient of the line: ddoLine
// (dirac/trig_damagedonce.go) tests kop.DmgGatePass over the event's amount
// for every line that reads its recipient, so only a line with a recipient
// read (ValidTarget$ You, or an object / player-and-planeswalker filter)
// carries it. A batch mode's DamageAmount$ is a batch total (DamageDoneOnce
// lowers it in damageDoneOnce, DamageDealtOnce is refused), an SVar or
// engine-defined operand (TargetToughness) has no kernel gate, and a line
// with no recipient read would drop the gate silently, so each of those stays
// refused.
func (lc *lowerCtx) damageAmountGate(tp *params, tr *lTrigger, t cards.Trigger) error {
	ga, ok := tp.get("DamageAmount")
	if !ok {
		return nil
	}
	ga = strings.TrimSpace(ga)
	if t.Mode != "DamageDone" {
		return refuse("%s DamageAmount$ %q (a batch total)", t.Mode, ga)
	}
	cmp, lit, ok := damageAmountLiteral(ga)
	if !ok {
		return refuse("DamageDone DamageAmount$ %q", ga)
	}
	f2 := tr.F2
	if f2 == nil || len(f2.Clauses) == 0 && f2.Players != plYou {
		return refuse("DamageDone DamageAmount$ %q on a line that reads no recipient", ga)
	}
	tr.DmgGate = kop.DmgGate(cmp, lit)
	return nil
}

// damageAmountLiteral parses a <cmp><literal> DamageAmount$ whose literal fits
// kop.DmgGate's low byte.
func damageAmountLiteral(ga string) (kop.DmgCmp, int, bool) {
	if len(ga) < 3 {
		return 0, 0, false
	}
	cmp, ok := map[string]kop.DmgCmp{"GE": kop.DmgGE, "LE": kop.DmgLE, "EQ": kop.DmgEQ, "NE": kop.DmgNE, "GT": kop.DmgGT, "LT": kop.DmgLT}[ga[:2]]
	lit, err := strconv.Atoi(ga[2:])
	if !ok || err != nil || lit < 0 || lit > 255 {
		return 0, 0, false
	}
	return cmp, lit, true
}
