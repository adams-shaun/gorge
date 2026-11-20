package cards

import (
	"bytes"
	"regexp"
	"strings"
)

// Upstream Forge (master after Nov 2026) rewrote a family of script spellings
// without changing what a card does. The rewrite below maps each back to the
// spelling the engine and dirac's kgen already read, so one normalisation
// serves both. Every rule is exact for the context it is applied in; a form
// whose meaning moved (Mirkwood Trapper's attacking-player scope, the Curse
// cycle's Defined$) is not touched and stays unsupported.

// upstreamMarkers gates the pass: a script with none of them is returned as is.
var upstreamMarkers = [...][]byte{
	[]byte("TriggeredPlayer"), []byte("TriggeredAttackingPlayer"),
	[]byte("TriggeredDefendingPlayer"), []byte("TriggeredDefender"),
	[]byte("hasMoreLifeThanYou"), []byte("!wasCastFromHand"), []byte("controlsCreature.attacking Player.EnchantedBy"),
}

// attackConditionRewrites are context-free: Forge's Condition$ shorthands that
// upstream spelled out as a check on the triggered players.
var attackConditionRewrites = strings.NewReplacer(
	"CheckSVar$ TriggeredDefendingPlayer$LifeTotal | SVarCompare$ GECount$OppGreatestLifeTotal",
	"Condition$ NoOpponentHasMoreLifeThanAttacked",
	"CheckDefinedPlayer$ Player.OpponentOf TriggeredAttackingPlayer+!Defending",
	"Condition$ AttackerHasUnattackedOpp",
	"CheckDefinedPlayer$ TriggeredDefender.withMostLife",
	"Condition$ AttackedPlayerWithMostLife",
	"Attacked$ Player.hasMoreLifeThanYou",
	"Attacked$ Player.hasMoreLifeThanActive",
	// Wash Away: upstream spells the owner's hand as the generic hand zone; a
	// spell is cast from its owner's hand except under a cast-from-another-hand
	// effect.
	"ValidTgts$ Card.!wasCastFromHand", "ValidTgts$ Card.!wasCastFromTheirHand",
	// the Curse cycle: only the attacking player controls attackers, so the
	// triggered attacking player is the one opponent the new form names.
	"$ Player.Opponent+controlsCreature.attacking Player.EnchantedBy",
	"$ TriggeredAttackingPlayer.Opponent+controlsCreature.attacking Player.EnchantedBy",
)

// phaseRewrites hold only inside a Mode$ Phase trigger's own chain, where the
// triggered player is the active player (CR 502-514: the player whose step or
// phase begins).
var phaseRewrites = strings.NewReplacer(
	"ControlledBy TriggeredPlayer", "ActivePlayerCtrl",
	"TriggeredPlayer$CardsInHand", "Count$ValidHand Card.ActivePlayerCtrl",
	"TriggeredPlayer$DomainPlayer", "Count$DomainActivePlayer",
	"OpponentOf TriggeredPlayer", "OpponentToActive",
	"ThanTriggeredPlayer", "ThanActive",
	"ControlledBy DelayTriggerRemembered", "ActivePlayerCtrl",
)

var (
	lifeTotalSVarRE = regexp.MustCompile(`(?m)^SVar:(\w+):TriggeredPlayer\$LifeTotal\s*$`)
	svarLineRE      = regexp.MustCompile(`^SVar:(\w+):(.*)$`)
)

// canonicalizeUpstream returns src with upstream's respelled forms mapped back.
func canonicalizeUpstream(src []byte) []byte {
	hit := false
	for _, m := range upstreamMarkers {
		if bytes.Contains(src, m) {
			hit = true
			break
		}
	}
	if !hit {
		return src
	}
	text := string(src)
	// A life-total check on the triggered player: CheckSVar$ X | SVarCompare$ LE5
	// with X = TriggeredPlayer$LifeTotal is LifeTotal$ ActivePlayer | LifeAmount$ LE5.
	for _, m := range lifeTotalSVarRE.FindAllStringSubmatch(text, -1) {
		re := regexp.MustCompile(`CheckSVar\$ ` + regexp.QuoteMeta(m[1]) + ` \| SVarCompare\$ (\w+)`)
		text = re.ReplaceAllString(text, "LifeTotal$$ ActivePlayer | LifeAmount$$ $1")
	}
	text = attackConditionRewrites.Replace(text)
	if !strings.Contains(text, "TriggeredPlayer") {
		return []byte(text)
	}
	lines := strings.Split(text, "\n")
	scope := phaseScope(lines)
	for i := range scope {
		lines[i] = phaseRewrites.Replace(lines[i])
		if strings.Contains(lines[i], "AB$ ImmediateTrigger") {
			// the reflexive trigger's remembered triggered player is only read by
			// the ControlledBy DelayTriggerRemembered form rewritten above
			lines[i] = strings.Replace(lines[i], " | RememberObjects$ TriggeredPlayer", "", 1)
		}
	}
	// the Razor Pendulum shape needs the SVar left in place; nothing reads it now.
	return []byte(strings.Join(lines, "\n"))
}

// phaseScope returns the indices of the lines reachable from a Mode$ Phase
// trigger and from no other trigger.
func phaseScope(lines []string) map[int]struct{} {
	svars := map[string]int{}
	for i, l := range lines {
		if m := svarLineRE.FindStringSubmatch(l); m != nil {
			svars[m[1]] = i
		}
	}
	reach := func(root int, into map[int]struct{}) {
		stack := []int{root}
		for len(stack) > 0 {
			i := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, seen := into[i]; seen {
				continue
			}
			into[i] = struct{}{}
			body := lines[i]
			if m := svarLineRE.FindStringSubmatch(body); m != nil {
				body = m[2]
			}
			for name, j := range svars {
				if j != i && containsWord(body, name) {
					stack = append(stack, j)
				}
			}
		}
	}
	phase, other := map[int]struct{}{}, map[int]struct{}{}
	for i, l := range lines {
		if !isTriggerLine(l) {
			continue
		}
		if strings.Contains(l, "Mode$ Phase") {
			reach(i, phase)
		} else {
			reach(i, other)
		}
	}
	for i := range other {
		delete(phase, i)
	}
	return phase
}

// containsWord reports whether name occurs in s as a whole identifier.
func containsWord(s, name string) bool {
	for off := 0; ; {
		k := strings.Index(s[off:], name)
		if k < 0 {
			return false
		}
		a, b := off+k, off+k+len(name)
		if (a == 0 || !isWordByte(s[a-1])) && (b == len(s) || !isWordByte(s[b])) {
			return true
		}
		off = a + 1
	}
}

func isWordByte(c byte) bool {
	return c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// isTriggerLine reports a printed T: line or an SVar that carries a trigger
// body (an AddTrigger$ target such as Citadel Siege's KhansTrigger).
func isTriggerLine(l string) bool {
	if strings.HasPrefix(l, "T:") {
		return true
	}
	if m := svarLineRE.FindStringSubmatch(l); m != nil {
		return strings.HasPrefix(m[2], "Mode$ ")
	}
	return false
}
