package feat

import (
	"strconv"
	"strings"
)

// The mana-cost parser below is policynet's (bots/policynet/option.go:
// manaCostBits and its helpers), copied verbatim: pool-load time only.

var manaBraceForm = strings.NewReplacer("{", " ", "}", " ")

// manaCostBits parses a Forge-notation mana cost ("1 W", "X G", "G/W",
// "2/W", "UP") into six colour booleans (WUBRG; index 5 = colourless {C})
// and the mana value. It is an exact mirror of the engine's canonical
// conversion, cards/cmcFromManaCost (engine/cards/face.go), because policynet must
// not import rules and there is no exported entry point: the value rule is
// repeated here, character for character, so the two cannot drift silently
// -- and TestManaCostBitsMirrorsEngine pins the parity on the shapes the
// corpus carries. A view's "no cost" (every land) or empty ManaCost is mana
// value 0 with no colours, exactly as cards.Face.ManaValue reports it.
func manaCostBits(cost string) ([6]bool, int) {
	cost = strings.TrimSpace(manaBraceForm.Replace(cost))
	var colours [6]bool
	if cost == "" || strings.EqualFold(cost, "no cost") {
		return colours, 0
	}
	mv := 0
	for _, tok := range costTokens(cost) {
		// Colour bits come from the token's own colour letters: a plain pip,
		// each '/'-separated face of a hybrid ("G/W"), or a twobrid's colour
		// half ("2/W" -> W). X/Y/Z and pure-generic tokens carry none.
		addColour(costTokenColours(tok), &colours)

		if tok == "X" { // {X} is 0 off the stack
			continue
		}
		if len(tok) == 1 && strings.ContainsRune("WUBRGC", rune(tok[0])) {
			mv++
			continue
		}
		if n, err := strconv.Atoi(tok); err == nil && n >= 0 {
			mv += n
			continue
		}
		if v, ok := twobridManaValue(tok); ok {
			// CR 202.4b: a monocolour hybrid's mana value is its generic
			// face. {2/W} is mana value 2 whether it is paid with two mana
			// or one white mana.
			mv += v
			continue
		}
		// Hybrid ("W/U"), Phyrexian ("UP"), and any other symbolic token:
		// one generic pip.
		mv++
	}
	return colours, mv
}

// costTokenColours returns the WUBRG C colour letters a mana-cost token
// names: a plain pip ("W"), each face of a hybrid ("G/W" -> G,W), a
// Phyrexian single colour ("UP" -> U), or a twobrid's colour half
// ("2/W" -> W, "2W" -> W). Generic digits and X/Y/Z yield nothing.
func costTokenColours(tok string) string {
	var b strings.Builder
	add := func(r rune) {
		if strings.ContainsRune("WUBRGC", r) {
			b.WriteRune(r)
		}
	}
	if strings.ContainsRune(tok, '/') {
		for _, p := range strings.Split(tok, "/") {
			if len(p) == 1 {
				add(rune(p[0]))
			}
		}
		return b.String()
	}
	// Phyrexian: a GC/WUBRG-first, P-second spelling ("UP"); or a bare pip.
	for i := 0; i < len(tok); i++ {
		add(rune(tok[i]))
	}
	return b.String()
}

// addColour ORs a token's colour letters into the WUBRG C mask.
func addColour(letters string, colours *[6]bool) {
	for i := 0; i < len(letters); i++ {
		switch letters[i] {
		case 'W':
			colours[0] = true
		case 'U':
			colours[1] = true
		case 'B':
			colours[2] = true
		case 'R':
			colours[3] = true
		case 'G':
			colours[4] = true
		case 'C':
			colours[5] = true
		}
	}
}

// twobridManaValue recognises Forge's concatenated ("2W") and slash
// ("2/W") monocolour-hybrid spellings, returning the generic face — the
// symbol's mana value. This is cards/twobridManaValue copied verbatim
// (engine/cards/face.go) so the two stay in lockstep; that one in turn mirrors
// rules.ParseCost's twobrid parser without importing rules.
func twobridManaValue(sym string) (int, bool) {
	generic, col := "", ""
	if left, right, ok := strings.Cut(sym, "/"); ok {
		generic, col = left, right
	} else {
		i := 0
		for i < len(sym) && sym[i] >= '0' && sym[i] <= '9' {
			i++
		}
		if i == 0 {
			return 0, false
		}
		generic, col = sym[:i], sym[i:]
	}
	if len(col) != 1 || !strings.ContainsRune("WUBRGC", rune(col[0])) {
		return 0, false
	}
	v, err := strconv.Atoi(generic)
	if err != nil || v < 0 {
		return 0, false
	}
	return v, true
}

// mvOf is manaCostBits' mana-value half alone.
func mvOf(cost string) int {
	_, mv := manaCostBits(cost)
	return mv
}

// costTokens splits a cost string on whitespace but keeps each <...> group
// atomic, so a Forge non-mana token whose trailing "/description" contains
// spaces (e.g. "Sac<1/Creature.powerGE4/creature with power 4 or greater>")
// is one token rather than several. This mirrors rules.splitCostTokens; a
// plain strings.Fields would tear the description apart and mis-read its
// stray digits as generic mana. A cost with no <...> group tokenises exactly
// as Fields would.
func costTokens(s string) []string {
	var out []string
	i := 0
	for i < len(s) {
		if isSpaceByte(s[i]) {
			i++
			continue
		}
		start, depth := i, 0
		for i < len(s) {
			if s[i] == '<' {
				depth++
			} else if s[i] == '>' && depth > 0 {
				depth--
			} else if isSpaceByte(s[i]) && depth == 0 {
				break
			}
			i++
		}
		out = append(out, s[start:i])
	}
	return out
}

// isSpaceByte reports whether b is an ASCII whitespace byte. Forge costs are
// ASCII; rules's iterator uses unicode.IsSpace, which agrees on every byte
// that can appear in a cost string.
func isSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// manaOnlyPips prices the MANA value of a formatCost rendering
// (decision.Option.Cost), ignoring every non-mana component. It mirrors the
// positive mana vocabulary of rules.ParseCost token by token: a token is
// mana only when it is X, S, a single WUBRGC letter, a non-negative integer,
// a hybrid, a Phyrexian, a twobrid or a hybrid-Phyrexian symbol. Anything
// else -- T, Forage, and every <head><N/Spec> component (Sac, Discard,
// Draw, SubCounter, AddCounter, Exile/ExileFromHand/ExileFromGrave, Reveal,
// Behold, tapXType, Blight, Return, PayLife, LifeX, DamageYou, PayEnergy) --
// is non-mana and contributes 0. Classifying by POSITIVE mana membership
// (rather than enumerating non-mana heads) is deliberate: a future cost
// component formatCost learns to render is priced 0 here unless it is also
// real mana, so no new component can silently become a phantom pip.
//
// A malformed or unrecognised token is non-mana here. That cannot diverge
// from the engine on real wire data: formatCost only ever emits recognised
// mana symbols and the known non-mana heads (an unparseable raw token is
// folded to generic mana before formatCost ever sees it), so every token in
// a real marker classifies.
func manaOnlyPips(cost string) int {
	cost = strings.TrimSpace(manaBraceForm.Replace(cost))
	if cost == "" || strings.EqualFold(cost, "no cost") {
		return 0
	}
	mv := 0
	for _, tok := range costTokens(cost) {
		switch {
		case tok == "X": // {X} is 0 off the stack
		case tok == "S": // snow mana: one pip
			mv++
		case len(tok) == 1 && strings.ContainsRune("WUBRGC", rune(tok[0])):
			mv++
		case isPlainDigits(tok):
			if n, err := strconv.Atoi(tok); err == nil {
				mv += n
			}
		case isHybridSym(tok), isPhyrexianSym(tok), isHybridPhyrexianSym(tok):
			mv++
		default:
			// Twobrid is the one mana symbol with a non-unit value; every
			// other token here is a non-mana component and contributes 0.
			if v, ok := twobridManaValue(tok); ok {
				mv += v
			}
		}
	}
	return mv
}

// isPlainDigits reports whether s is a non-empty run of ASCII digits with no
// sign or other character.
func isPlainDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// isHybridSym mirrors rules.isHybrid: a two-colour hybrid pip, slash
// ("W/U") or concatenated ("WU"), the two colours distinct and WUBRGC.
func isHybridSym(sym string) bool {
	var a, b byte
	if len(sym) == 3 && sym[1] == '/' {
		a, b = sym[0], sym[2]
	} else if len(sym) == 2 && sym[1] != 'P' {
		a, b = sym[0], sym[1]
	} else {
		return false
	}
	return a != b && strings.ContainsRune("WUBRGC", rune(a)) && strings.ContainsRune("WUBRGC", rune(b))
}

// isPhyrexianSym mirrors rules.isPhyrexian: a WUBRG colour then P, slash
// ("W/P") or concatenated ("UP").
func isPhyrexianSym(sym string) bool {
	if len(sym) == 3 && sym[1] == '/' {
		return strings.ContainsRune("WUBRG", rune(sym[0])) && sym[2] == 'P'
	}
	if len(sym) != 2 {
		return false
	}
	return sym[1] == 'P' && strings.ContainsRune("WUBRG", rune(sym[0]))
}

// isHybridPhyrexianSym mirrors rules.isHybridPhyrexian: two distinct WUBRG
// colours and P, in either spelling ("G/W/P", "GWP", the P-first "PRG").
func isHybridPhyrexianSym(sym string) bool {
	if a, b, ok := stringsCutSlash(sym); ok {
		if len(a) != 1 || len(b) != 3 || b[1] != '/' {
			return false
		}
		if a[0] == 'P' {
			return b[0] != b[2] && strings.ContainsRune("WUBRG", rune(b[0])) &&
				strings.ContainsRune("WUBRG", rune(b[2]))
		}
		return b[2] == 'P' && a[0] != b[0] && strings.ContainsRune("WUBRG", rune(a[0])) &&
			strings.ContainsRune("WUBRG", rune(b[0]))
	}
	if len(sym) != 3 {
		return false
	}
	if sym[0] == 'P' {
		return sym[1] != sym[2] && strings.ContainsRune("WUBRG", rune(sym[1])) &&
			strings.ContainsRune("WUBRG", rune(sym[2]))
	}
	return sym[2] == 'P' && sym[0] != sym[1] && strings.ContainsRune("WUBRG", rune(sym[0])) &&
		strings.ContainsRune("WUBRG", rune(sym[1]))
}

// stringsCutSlash splits "a/b" at the first slash, returning ok=false when
// the slash is absent or at either end -- rules.splitHybridSlash's contract.
func stringsCutSlash(sym string) (a, b string, ok bool) {
	i := strings.IndexByte(sym, '/')
	if i <= 0 || i == len(sym)-1 {
		return "", "", false
	}
	return sym[:i], sym[i+1:], true
}

// ---- the v2 block's printed half (policynet entity.go's v2 helpers) -------

// pipMax caps one pip count, as policynet's does: more pips of one colour
// than this are saturated (the distinction is not meaningful).
const pipMax = 5

// v2ColourPips is the v2 block's colour and pip lanes for a mana cost:
// manaCostBits' colours, then pipCounts / pipMax. Pool-load time only.
func v2ColourPips(cost string) [12]float32 {
	var out [12]float32
	colours, _ := manaCostBits(cost)
	for k := 0; k < 6; k++ {
		if colours[k] {
			out[k] = 1
		}
	}
	pips := pipCounts(cost)
	for k := 0; k < 6; k++ {
		if pips[k] != 0 {
			out[6+k] = float32(pips[k]) / float32(pipMax)
		}
	}
	return out
}

// pipCounts counts a mana cost's pips per colour (WUBRG then generic) over
// the token walk manaCostBits takes, so the two cannot disagree: policynet's
// pipCounts, which mirrors this one (the oracle compares the slots).
func pipCounts(cost string) [6]int {
	var n [6]int
	cost = strings.TrimSpace(manaBraceForm.Replace(cost))
	if cost == "" || strings.EqualFold(cost, "no cost") {
		return n
	}
	for _, tok := range costTokens(cost) {
		addPips(&n, tok)
	}
	for i := range n {
		if n[i] > pipMax {
			n[i] = pipMax
		}
	}
	return n
}

// addPips adds one mana-cost token's pips to n: a numeric token is generic,
// a slash token (hybrid, twobrid, Phyrexian) counts each one-letter face and
// nothing else, and any other token counts every WUBRG C letter it names.
func addPips(n *[6]int, tok string) {
	if strings.EqualFold(tok, "X") {
		return
	}
	if isPlainDigits(tok) {
		v := 0
		for i := 0; i < len(tok); i++ {
			v = v*10 + int(tok[i]-'0')
		}
		n[5] += v
		return
	}
	if strings.IndexByte(tok, '/') >= 0 {
		for _, p := range strings.Split(tok, "/") {
			if len(p) == 1 {
				if k := pipIndexByte(p[0]); k >= 0 {
					n[k]++
				}
			}
		}
		return
	}
	for i := 0; i < len(tok); i++ {
		if k := pipIndexByte(tok[i]); k >= 0 {
			n[k]++
		}
	}
}

// pipIndexByte maps a colour letter to its v2 pip slot (WUBRG C), or -1.
func pipIndexByte(b byte) int {
	switch b {
	case 'W':
		return 0
	case 'U':
		return 1
	case 'B':
		return 2
	case 'R':
		return 3
	case 'G':
		return 4
	case 'C':
		return 5
	}
	return -1
}

// The v2 category's printed markers (policynet's removalAPIMarkers and
// rampNameMarkers): coarse on purpose, evidence of a role.
var (
	removalAPIMarkers = []string{"destroy", "exile", "damage", "deals", "sacrifices", "counter target"}
	rampNameMarkers   = []string{"signet", "talisman", "lantern", "monument", "orb", "prism", "keyrune"}
)

// containsAnyFold reports whether s contains any marker, ASCII
// case-insensitively (the markers are lower case): policynet's
// containsFoldASCII, so a non-ASCII name folds the same on both sides.
func containsAnyFold(s string, markers []string) bool {
	for _, m := range markers {
		for i := 0; i+len(m) <= len(s); i++ {
			ok := true
			for j := 0; j < len(m); j++ {
				b := s[i+j]
				if b >= 'A' && b <= 'Z' {
					b += 'a' - 'A'
				}
				if b != m[j] {
					ok = false
					break
				}
			}
			if ok {
				return true
			}
		}
	}
	return false
}
