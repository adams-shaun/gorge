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
	[]byte("hasMoreLifeThanYou"), []byte("controlsCreature.attacking Player.EnchantedBy"),
	exiledOwnerPlayer, targetedAndOwner, nonTargetedCopy,
}

// exiledOwnerPlayer is upstream's player selector for "the exiled card's
// owner" of a leaves-the-battlefield body (Unidentified Hovership): the
// players owning a card exiled with this source. The engines read the older
// form, the exile remembering what it moved and the body naming the
// remembered card's owner, then clearing the set; exiledOwnerRewrite maps
// it back.
var exiledOwnerPlayer = []byte("DefinedPlayer$ Player.HasCardsInExile_Card.ExiledWithSource_GE1")

// targetedAndOwner is upstream's spelling of an Effect that remembers the
// card its parent link just moved and that card's owner (Reflector Mage: "its
// owner can't cast spells with the same name"). The engines read the older
// form, the moving link remembering the card as it was (RememberLKI$) and the
// Effect naming the LKI card and its owner, then clearing the set;
// targetedOwnerRewrite maps it back. A bare Targeted member would drop a
// target the very chain moved (a new object, CR 400.7), which is right for an
// Effect about the object and wrong for one about the name it had.
var targetedAndOwner = []byte("RememberObjects$ Targeted & TargetedOwner")

// nonTargetedCopy is upstream's spelling of Fractured Identity's copy leg: the
// copy of the card the parent link exiled, for every player but its controller,
// naming the spell's target. The engines read the older form (the exile
// remembering what it moved, Defined$ Remembered, NonRememberedController);
// nonTargetedCopyRewrite maps it back.
var nonTargetedCopy = []byte("Controller$ NonTargetedController")

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
	if bytes.Contains(src, exiledOwnerPlayer) {
		text = exiledOwnerRewrite(text)
	}
	if bytes.Contains(src, targetedAndOwner) {
		text = targetedOwnerRewrite(text)
	}
	if bytes.Contains(src, nonTargetedCopy) {
		text = nonTargetedCopyRewrite(text)
	}
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

// exiledOwnerRewrite maps the exiledOwnerPlayer selector back to the
// remembered-owner form: every ChangeZone into exile that remembers nothing
// remembers what it moved, the selector names the remembered card's owner,
// and a Cleanup clears the set after the body. A card whose exile already
// remembers (or that exiles in more than one place) is returned as is.
func exiledOwnerRewrite(text string) string {
	lines := strings.Split(text, "\n")
	exile := -1
	for i, l := range lines {
		if !strings.Contains(l, "DB$ ChangeZone") || !strings.Contains(l, "Destination$ Exile") {
			continue
		}
		if exile >= 0 || strings.Contains(l, "Remember") {
			return text
		}
		exile = i
	}
	if exile < 0 {
		return text
	}
	lines[exile] = strings.TrimRight(lines[exile], " \r") + " | RememberChanged$ True"
	const cleanup = "DBCanonExiledOwnerCleanup"
	out := strings.Join(lines, "\n")
	out = strings.Replace(out, string(exiledOwnerPlayer), "DefinedPlayer$ RememberedOwner | SubAbility$ "+cleanup, 1)
	return strings.TrimRight(out, "\n") + "\nSVar:" + cleanup + ":DB$ Cleanup | ClearRemembered$ True\n"
}

// targetedOwnerRewrite maps targetedAndOwner back to RememberedLKI &
// RememberedOwner: the link whose SubAbility$ names the Effect and which
// moves a target (a ChangeZone with ValidTgts$) gains RememberLKI$ True, and
// the Effect clears the remembered set after itself. A script where the
// Effect's parent is not exactly one such link, or where the Effect already
// has a SubAbility$, is returned as is.
func targetedOwnerRewrite(text string) string {
	lines := strings.Split(text, "\n")
	effect := -1
	for i, l := range lines {
		if !strings.Contains(l, string(targetedAndOwner)) {
			continue
		}
		if effect >= 0 || !strings.Contains(l, "DB$ Effect") || strings.Contains(l, "SubAbility$") {
			return text
		}
		effect = i
	}
	if effect < 0 {
		return text
	}
	m := svarLineRE.FindStringSubmatch(lines[effect])
	if m == nil {
		return text
	}
	parent := movingParent(lines, m[1])
	if parent < 0 {
		return text
	}
	const cleanup = "DBCanonTargetedOwnerCleanup"
	lines[parent] = strings.TrimRight(lines[parent], " \r") + " | RememberLKI$ True"
	lines[effect] = strings.Replace(strings.TrimRight(lines[effect], " \r"), string(targetedAndOwner),
		"RememberObjects$ RememberedLKI & RememberedOwner", 1) + " | SubAbility$ " + cleanup
	out := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	return out + "\nSVar:" + cleanup + ":DB$ Cleanup | ClearRemembered$ True\n"
}

// subAbilityNames reports whether line's SubAbility$ parameter is exactly name.
func subAbilityNames(line, name string) bool {
	for _, part := range strings.Split(line, "|") {
		part = strings.TrimSpace(part)
		if v, ok := strings.CutPrefix(part, "SubAbility$ "); ok && v == name {
			return true
		}
	}
	return false
}

// movingParent returns the index of the one line whose SubAbility$ is name and
// that moves a target (a ChangeZone with ValidTgts$) without remembering
// anything, or -1 when there is not exactly one such line.
func movingParent(lines []string, name string) int {
	parent := -1
	for i, l := range lines {
		if !subAbilityNames(l, name) {
			continue
		}
		if parent >= 0 || !strings.Contains(l, "ChangeZone") || !strings.Contains(l, "ValidTgts$") || strings.Contains(l, "Remember") {
			return -1
		}
		parent = i
	}
	return parent
}

// nonTargetedCopyRewrite maps nonTargetedCopy back: the moving parent gains
// RememberLKI$ True, the copy leg reads Defined$ Remembered with
// NonRememberedController and clears the remembered set after itself. A copy
// leg that is not exactly Defined$ Targeted, or that already has a
// SubAbility$, is returned as is.
func nonTargetedCopyRewrite(text string) string {
	lines := strings.Split(text, "\n")
	copyLine := -1
	for i, l := range lines {
		if !strings.Contains(l, string(nonTargetedCopy)) {
			continue
		}
		if copyLine >= 0 || !strings.Contains(l, "DB$ CopyPermanent") || !strings.Contains(l, "Defined$ Targeted |") ||
			strings.Contains(l, "SubAbility$") {
			return text
		}
		copyLine = i
	}
	if copyLine < 0 {
		return text
	}
	m := svarLineRE.FindStringSubmatch(lines[copyLine])
	if m == nil {
		return text
	}
	parent := movingParent(lines, m[1])
	if parent < 0 {
		return text
	}
	const cleanup = "DBCanonNonTargetedCopyCleanup"
	lines[parent] = strings.TrimRight(lines[parent], " \r") + " | RememberLKI$ True"
	l := strings.Replace(strings.TrimRight(lines[copyLine], " \r"), "Defined$ Targeted |", "Defined$ Remembered |", 1)
	lines[copyLine] = strings.Replace(l, string(nonTargetedCopy), "Controller$ NonRememberedController", 1) + " | SubAbility$ " + cleanup
	out := strings.TrimRight(strings.Join(lines, "\n"), "\n")
	return out + "\nSVar:" + cleanup + ":DB$ Cleanup | ClearRemembered$ True\n"
}
