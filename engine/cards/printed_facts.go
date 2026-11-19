package cards

import (
	"strconv"
	"strings"
	"unsafe"
)

// Printed-face facts the seat view projects beside the text they are a pure
// function of (view.CardFacts): the type line's TypeMask bits and the mana
// cost's colour pips and mana value. An in-process CardView reader -- the
// policy-net feature encoder -- tests these bits instead of re-parsing the
// type line and the cost per card per encode. They are derived once at load
// (Face.derive) and guarded by the identity of the text they were computed
// over, like the word sets (words.go): a face struct copy whose Types slice
// or ManaCost string was replaced reads as unbound.

// PrintedFacts are a face's compiled type-line and mana-cost facts.
type PrintedFacts struct {
	// TypeFold and TypeExact are TypeLineMasks of the joined type line.
	TypeFold, TypeExact TypeMask
	// CostColours and ManaValue are ManaCostPips of ManaCost.
	CostColours ColourMask
	ManaValue   int
}

type printedFacts struct {
	facts PrintedFacts
	// line is strings.Join(Types, " "), the type line the view projects.
	line string
	// typesFirst/typesLen and costData/costLen are the identity of the
	// Types slice and ManaCost bytes the facts were derived over, held as
	// addresses so the guard adds nothing for the collector to scan (the
	// face's own fields keep that text alive, and the heap does not move).
	typesFirst uintptr
	typesLen   int
	costData   uintptr
	costLen    int
	bound      bool
}

// typeNames are the canonical TypeMask words: typeNames[i] is bit 1<<i.
var typeNames = [...]string{
	"Artifact", "Battle", "Conspiracy", "Creature", "Dungeon", "Enchantment", "Instant", "Kindred",
	"Land", "Phenomenon", "Plane", "Planeswalker", "Scheme", "Sorcery", "Tribal", "Vanguard",
	"Basic", "Legendary", "Ongoing", "Snow", "World", "Spacecraft", "Vehicle", "Room",
}

// TypeLineMasks folds a type line's whitespace-separated words into two
// TypeMasks: fold has a type's bit when some word equals its canonical name
// under strings.EqualFold, exact when some word equals it byte for byte.
func TypeLineMasks(line string) (fold, exact TypeMask) {
	for w := range strings.FieldsSeq(line) {
		for i, t := range typeNames {
			if w == t {
				exact |= 1 << i
				fold |= 1 << i
			} else if strings.EqualFold(w, t) {
				fold |= 1 << i
			}
		}
	}
	return fold, exact
}

// ManaCostPips parses a Forge-notation mana cost ("1 W", "X G", "G/W",
// "2/W", "UP", or brace form "{2}{U}") into its colour pips -- every WUBRG C
// letter a token names: a plain pip, each '/'-separated face of a hybrid, a
// Phyrexian's colour, a twobrid's colour half -- and its mana value. "No
// cost" and an empty cost are value 0 with no colours. It is the policy
// feature reading of a printed cost (bots/policynet manaCostBits), kept here
// so the projection can carry it per face: its value rule is
// cmcFromManaCost's, over tokens that keep each <...> group atomic.
func ManaCostPips(cost string) (ColourMask, int) {
	cost = strings.TrimSpace(manaBraceForm.Replace(cost))
	var colours ColourMask
	if IsNoManaCost(cost) {
		return 0, 0
	}
	mv := 0
	for i := 0; i < len(cost); {
		if isCostSpace(cost[i]) {
			i++
			continue
		}
		start, depth := i, 0
		for i < len(cost) {
			if cost[i] == '<' {
				depth++
			} else if cost[i] == '>' && depth > 0 {
				depth--
			} else if isCostSpace(cost[i]) && depth == 0 {
				break
			}
			i++
		}
		tok := cost[start:i]
		colours |= tokenColours(tok)
		if tok == "X" { // {X} is 0 off the stack
			continue
		}
		if len(tok) == 1 && strings.IndexByte("WUBRGC", tok[0]) >= 0 {
			mv++
			continue
		}
		if n, err := strconv.Atoi(tok); err == nil && n >= 0 {
			mv += n
			continue
		}
		if v, ok := twobridManaValue(tok); ok {
			// CR 202.4b: a monocolour hybrid's mana value is its generic face.
			mv += int(v)
			continue
		}
		// Hybrid ("W/U"), Phyrexian ("UP"), and any other symbolic token:
		// one generic pip.
		mv++
	}
	return colours, mv
}

func isCostSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\v' || b == '\f'
}

// tokenColours is the WUBRG C letters one cost token names: each one-letter
// face of a hybrid ("G/W" -> G,W; "2/W" -> W), else every letter of the token
// ("UP" -> U, "W" -> W). Generic digits and X/Y/Z name none.
func tokenColours(tok string) ColourMask {
	var m ColourMask
	if strings.IndexByte(tok, '/') >= 0 {
		for p := range strings.SplitSeq(tok, "/") {
			if len(p) == 1 {
				m |= pipColour(p[0])
			}
		}
		return m
	}
	for i := 0; i < len(tok); i++ {
		m |= pipColour(tok[i])
	}
	return m
}

func pipColour(c byte) ColourMask {
	switch c {
	case 'W':
		return ColourMaskWhite
	case 'U':
		return ColourMaskBlue
	case 'B':
		return ColourMaskBlack
	case 'R':
		return ColourMaskRed
	case 'G':
		return ColourMaskGreen
	case 'C':
		return ColourMaskColourless
	}
	return 0
}

// derivePrintedFacts binds the face's PrintedFacts to its current Types and
// ManaCost.
func (f *Face) derivePrintedFacts() {
	p := printedFacts{typesLen: len(f.Types), costData: stringAddr(f.ManaCost), costLen: len(f.ManaCost), bound: true}
	if len(f.Types) > 0 {
		p.typesFirst = uintptr(unsafe.Pointer(&f.Types[0]))
	}
	p.line = strings.Join(f.Types, " ")
	p.facts.TypeFold, p.facts.TypeExact = TypeLineMasks(p.line)
	p.facts.CostColours, p.facts.ManaValue = ManaCostPips(f.ManaCost)
	f.printed = p
}

// PrintedFacts returns the face's compiled type-line and mana-cost facts
// and its joined type line (strings.Join(Types, " ")), and false when they
// are not bound to its current Types and ManaCost (a face never derived, or
// a struct copy whose text was replaced): the caller then reads the text
// itself.
func (f *Face) PrintedFacts() (PrintedFacts, string, bool) {
	p := &f.printed
	if !p.bound || p.typesLen != len(f.Types) || p.costLen != len(f.ManaCost) ||
		p.costLen > 0 && p.costData != stringAddr(f.ManaCost) {
		return PrintedFacts{}, "", false
	}
	if len(f.Types) > 0 && p.typesFirst != uintptr(unsafe.Pointer(&f.Types[0])) {
		return PrintedFacts{}, "", false
	}
	return p.facts, p.line, true
}

func stringAddr(s string) uintptr { return uintptr(unsafe.Pointer(unsafe.StringData(s))) }
