package cards

import (
	"bytes"
	"regexp"
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

// byteRewrite is one literal rewrite: every occurrence of from becomes to.
// The tables below are applied in order; no rule's output holds another rule's
// input, so the order cannot chain two rules.
type byteRewrite struct{ from, to []byte }

// rewrites builds a rewrite table from alternating from/to literals.
func rewrites(pairs ...string) []byteRewrite {
	out := make([]byteRewrite, 0, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, byteRewrite{[]byte(pairs[i]), []byte(pairs[i+1])})
	}
	return out
}

// applyRewrites applies every rule of table to b.
func applyRewrites(b []byte, table []byteRewrite) []byte {
	for _, r := range table {
		if bytes.Contains(b, r.from) {
			b = bytes.ReplaceAll(b, r.from, r.to)
		}
	}
	return b
}

// attackConditionRewrites are context-free: Forge's Condition$ shorthands that
// upstream spelled out as a check on the triggered players.
var attackConditionRewrites = rewrites(
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
var phaseRewrites = rewrites(
	"ControlledBy TriggeredPlayer", "ActivePlayerCtrl",
	"TriggeredPlayer$CardsInHand", "Count$ValidHand Card.ActivePlayerCtrl",
	"TriggeredPlayer$DomainPlayer", "Count$DomainActivePlayer",
	"OpponentOf TriggeredPlayer", "OpponentToActive",
	"ThanTriggeredPlayer", "ThanActive",
	"ControlledBy DelayTriggerRemembered", "ActivePlayerCtrl",
)

// The script words the structural rewrites below look for and write.
var (
	wTriggeredPlayer     = []byte("TriggeredPlayer")
	wImmediateTrigger    = []byte("AB$ ImmediateTrigger")
	wRememberTrigPlayer  = []byte(" | RememberObjects$ TriggeredPlayer")
	wModePhase           = []byte("Mode$ Phase")
	wTriggerPrefix       = []byte("T:")
	wModePrefix          = []byte("Mode$ ")
	wChangeZoneDB        = []byte("DB$ ChangeZone")
	wChangeZone          = []byte("ChangeZone")
	wDestExile           = []byte("Destination$ Exile")
	wRemember            = []byte("Remember")
	wEffectDB            = []byte("DB$ Effect")
	wCopyPermanentDB     = []byte("DB$ CopyPermanent")
	wSubAbility          = []byte("SubAbility$")
	wSubAbilityEq        = []byte("SubAbility$ ")
	wValidTgts           = []byte("ValidTgts$")
	wDefinedTargeted     = []byte("Defined$ Targeted |")
	wDefinedRemembered   = []byte("Defined$ Remembered |")
	wRememberedOwnerSub  = []byte("DefinedPlayer$ RememberedOwner | SubAbility$ ")
	wRememberLKIOwner    = []byte("RememberObjects$ RememberedLKI & RememberedOwner")
	wNonRememberedCtrl   = []byte("Controller$ NonRememberedController")
	wRememberChangedTail = []byte(" | RememberChanged$ True")
	wRememberLKITail     = []byte(" | RememberLKI$ True")
	wSubAbilityTail      = []byte(" | SubAbility$ ")
	wCleanupBody         = []byte(":DB$ Cleanup | ClearRemembered$ True\n")
	wSVarPrefix          = []byte("\nSVar:")
	newline              = []byte("\n")
	trimTail             = " \r"
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
	text := bytes.Clone(src)
	if bytes.Contains(text, exiledOwnerPlayer) {
		text = exiledOwnerRewrite(text)
	}
	if bytes.Contains(text, targetedAndOwner) {
		text = targetedOwnerRewrite(text)
	}
	if bytes.Contains(text, nonTargetedCopy) {
		text = nonTargetedCopyRewrite(text)
	}
	// A life-total check on the triggered player: CheckSVar$ X | SVarCompare$ LE5
	// with X = TriggeredPlayer$LifeTotal is LifeTotal$ ActivePlayer | LifeAmount$ LE5.
	for _, m := range lifeTotalSVarRE.FindAllSubmatch(text, -1) {
		re := regexp.MustCompile(`CheckSVar\$ ` + regexp.QuoteMeta(string(m[1])) + ` \| SVarCompare\$ (\w+)`)
		text = re.ReplaceAll(text, []byte("LifeTotal$$ ActivePlayer | LifeAmount$$ $1"))
	}
	text = applyRewrites(text, attackConditionRewrites)
	if !bytes.Contains(text, wTriggeredPlayer) {
		return text
	}
	lines := bytes.Split(text, newline)
	scope := phaseScope(lines)
	for i := range scope {
		lines[i] = applyRewrites(lines[i], phaseRewrites)
		if bytes.Contains(lines[i], wImmediateTrigger) {
			// the reflexive trigger's remembered triggered player is only read by
			// the ControlledBy DelayTriggerRemembered form rewritten above
			lines[i] = bytes.Replace(lines[i], wRememberTrigPlayer, nil, 1)
		}
	}
	// the Razor Pendulum shape needs the SVar left in place; nothing reads it now.
	return bytes.Join(lines, newline)
}

// svarAt is one SVar line: its name and its index.
type svarAt struct {
	name []byte
	line int
}

// phaseScope returns the indices of the lines reachable from a Mode$ Phase
// trigger and from no other trigger.
func phaseScope(lines [][]byte) map[int]struct{} {
	var svars []svarAt
	for i, l := range lines {
		if m := svarLineRE.FindSubmatch(l); m != nil {
			svars = append(svars, svarAt{m[1], i})
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
			if m := svarLineRE.FindSubmatch(body); m != nil {
				body = m[2]
			}
			for _, sv := range svars {
				if sv.line != i && containsWord(body, sv.name) {
					stack = append(stack, sv.line)
				}
			}
		}
	}
	phase, other := map[int]struct{}{}, map[int]struct{}{}
	for i, l := range lines {
		if !isTriggerLine(l) {
			continue
		}
		if bytes.Contains(l, wModePhase) {
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
func containsWord(s, name []byte) bool {
	for off := 0; ; {
		k := bytes.Index(s[off:], name)
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
func isTriggerLine(l []byte) bool {
	if bytes.HasPrefix(l, wTriggerPrefix) {
		return true
	}
	if m := svarLineRE.FindSubmatch(l); m != nil {
		return bytes.HasPrefix(m[2], wModePrefix)
	}
	return false
}

// appendTo returns line, its trailing blanks trimmed, with tail appended.
func appendTo(line, tail []byte) []byte {
	return append(bytes.TrimRight(line, trimTail), tail...)
}

// withCleanup joins lines and appends a Cleanup SVar named cleanup that clears
// the remembered set.
func withCleanup(lines [][]byte, cleanup string) []byte {
	out := bytes.TrimRight(bytes.Join(lines, newline), "\n")
	out = append(out, wSVarPrefix...)
	out = append(out, cleanup...)
	return append(out, wCleanupBody...)
}

// exiledOwnerRewrite maps the exiledOwnerPlayer selector back to the
// remembered-owner form: every ChangeZone into exile that remembers nothing
// remembers what it moved, the selector names the remembered card's owner,
// and a Cleanup clears the set after the body. A card whose exile already
// remembers (or that exiles in more than one place) is returned as is.
func exiledOwnerRewrite(text []byte) []byte {
	lines := bytes.Split(text, newline)
	exile := -1
	for i, l := range lines {
		if !bytes.Contains(l, wChangeZoneDB) || !bytes.Contains(l, wDestExile) {
			continue
		}
		if exile >= 0 || bytes.Contains(l, wRemember) {
			return text
		}
		exile = i
	}
	if exile < 0 {
		return text
	}
	const cleanup = "DBCanonExiledOwnerCleanup"
	lines[exile] = appendTo(lines[exile], wRememberChangedTail)
	for i, l := range lines {
		if bytes.Contains(l, exiledOwnerPlayer) {
			lines[i] = bytes.Replace(l, exiledOwnerPlayer, append(bytes.Clone(wRememberedOwnerSub), cleanup...), 1)
			break
		}
	}
	return withCleanup(lines, cleanup)
}

// targetedOwnerRewrite maps targetedAndOwner back to RememberedLKI &
// RememberedOwner: the link whose SubAbility$ names the Effect and which
// moves a target (a ChangeZone with ValidTgts$) gains RememberLKI$ True, and
// the Effect clears the remembered set after itself. A script where the
// Effect's parent is not exactly one such link, or where the Effect already
// has a SubAbility$, is returned as is.
func targetedOwnerRewrite(text []byte) []byte {
	lines := bytes.Split(text, newline)
	effect := -1
	for i, l := range lines {
		if !bytes.Contains(l, targetedAndOwner) {
			continue
		}
		if effect >= 0 || !bytes.Contains(l, wEffectDB) || bytes.Contains(l, wSubAbility) {
			return text
		}
		effect = i
	}
	if effect < 0 {
		return text
	}
	m := svarLineRE.FindSubmatch(lines[effect])
	if m == nil {
		return text
	}
	parent := movingParent(lines, m[1])
	if parent < 0 {
		return text
	}
	const cleanup = "DBCanonTargetedOwnerCleanup"
	lines[parent] = appendTo(lines[parent], wRememberLKITail)
	e := bytes.Replace(bytes.TrimRight(lines[effect], trimTail), targetedAndOwner, wRememberLKIOwner, 1)
	lines[effect] = append(append(e, wSubAbilityTail...), cleanup...)
	return withCleanup(lines, cleanup)
}

// subAbilityNames reports whether line's SubAbility$ parameter is exactly name.
func subAbilityNames(line, name []byte) bool {
	for _, part := range bytes.Split(line, []byte("|")) {
		if v, ok := bytes.CutPrefix(bytes.TrimSpace(part), wSubAbilityEq); ok && bytes.Equal(v, name) {
			return true
		}
	}
	return false
}

// movingParent returns the index of the one line whose SubAbility$ is name and
// that moves a target (a ChangeZone with ValidTgts$) without remembering
// anything, or -1 when there is not exactly one such line.
func movingParent(lines [][]byte, name []byte) int {
	parent := -1
	for i, l := range lines {
		if !subAbilityNames(l, name) {
			continue
		}
		if parent >= 0 || !bytes.Contains(l, wChangeZone) || !bytes.Contains(l, wValidTgts) || bytes.Contains(l, wRemember) {
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
func nonTargetedCopyRewrite(text []byte) []byte {
	lines := bytes.Split(text, newline)
	copyLine := -1
	for i, l := range lines {
		if !bytes.Contains(l, nonTargetedCopy) {
			continue
		}
		if copyLine >= 0 || !bytes.Contains(l, wCopyPermanentDB) || !bytes.Contains(l, wDefinedTargeted) ||
			bytes.Contains(l, wSubAbility) {
			return text
		}
		copyLine = i
	}
	if copyLine < 0 {
		return text
	}
	m := svarLineRE.FindSubmatch(lines[copyLine])
	if m == nil {
		return text
	}
	parent := movingParent(lines, m[1])
	if parent < 0 {
		return text
	}
	const cleanup = "DBCanonNonTargetedCopyCleanup"
	lines[parent] = appendTo(lines[parent], wRememberLKITail)
	l := bytes.Replace(bytes.TrimRight(lines[copyLine], trimTail), wDefinedTargeted, wDefinedRemembered, 1)
	l = bytes.Replace(l, nonTargetedCopy, wNonRememberedCtrl, 1)
	lines[copyLine] = append(append(l, wSubAbilityTail...), cleanup...)
	return withCleanup(lines, cleanup)
}
