package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/state"
)

type ColorMask uint8

var colorMaskStrings = [32]string{
	"", "W", "U", "WU", "B", "WB", "UB", "WUB",
	"R", "WR", "UR", "WUR", "BR", "WBR", "UBR", "WUBR",
	"G", "WG", "UG", "WUG", "BG", "WBG", "UBG", "WUBG",
	"RG", "WRG", "URG", "WURG", "BRG", "WBRG", "UBRG", "WUBRG",
}

func colorBit(c byte) ColorMask {
	switch c {
	case 'W':
		return 1 << 0
	case 'U':
		return 1 << 1
	case 'B':
		return 1 << 2
	case 'R':
		return 1 << 3
	case 'G':
		return 1 << 4
	default:
		return 0
	}
}

func (m ColorMask) String() string {
	return colorMaskStrings[m&31]
}

// ColorsOf is an object's colours as WUBRG letters in that fixed order:
// the colours of its mana cost, or an explicit Colors: line for a card
// whose cost does not show them (a token, an artifact "that is green").
// Devoid makes a card colourless regardless (CR 702.114). A Face-less
// object (an ability, a copy of nothing) is colourless. Protection (rules)
// and the colour predicates read this rather than the face directly.
func ColorsOf(o *state.Object) string {
	return ColorMaskOf(o).String()
}

// ColorMaskOf is ColorsOf's compact representation for rules hot paths that
// combine printed colours with continuous effects before rendering WUBRG.
func ColorMaskOf(o *state.Object) ColorMask {
	if o == nil {
		return 0
	}
	f := o.Face()
	if f == nil || f.HasKeyword("Devoid") {
		return 0
	}
	var mask ColorMask
	for i := 0; i < len(f.ManaCost); i++ {
		mask |= colorBit(f.ManaCost[i])
	}
	if mask == 0 && f.Colors != "" {
		for rest := f.Colors; ; {
			word := rest
			if comma := strings.IndexByte(rest, ','); comma >= 0 {
				word, rest = rest[:comma], rest[comma+1:]
			} else {
				rest = ""
			}
			word = strings.TrimSpace(word)
			switch {
			case strings.EqualFold(word, "white"):
				mask |= 1 << 0
			case strings.EqualFold(word, "blue"):
				mask |= 1 << 1
			case strings.EqualFold(word, "black"):
				mask |= 1 << 2
			case strings.EqualFold(word, "red"):
				mask |= 1 << 3
			case strings.EqualFold(word, "green"):
				mask |= 1 << 4
			}
			if rest == "" {
				break
			}
		}
	}
	return mask
}

// colorLetters turns a Forge colour-list parameter value (Animate's Colors$,
// the "White,Blue" / "White" / "All" / "Colorless" vocabulary measured over
// the corpus's AB$ Animate lines, and the "Green & White" static spelling)
// into the WUBRG letters of the colour set it names, in WUBRG order and
// deduplicated. A list may separate its entries with commas OR the " & "
// Forge uses on its static list parameters (Witness Protection's
// `SetColor$ Green & White`, Ludevic, Necrogenius's `AddColors$ Blue & Black`)
// -- the SAME grammar rules' statList applies to AddType$, so the two list
// parameters on one static line cannot disagree. "All" is every colour,
// "Colorless" is the empty set. The second return reports whether EVERY
// non-blank word was recognised: a value this parser does not know (the
// corpus's "ChosenColor" family, which asks its controller for a colour) is
// a fail closed "ok=false" with whatever prefix parsed, never a guess -- the
// caller gates the effect registration on it so an unparseable value keeps
// the object's printed colours rather than silently overwriting them away.
// Blank entries (an absent parameter splits to one) neither contribute nor
// spoil ok. The letters are what state.ContinuousEffect's colour fields
// carry, so a caller never re-parses the words.
func colorLetters(list string) ([]string, bool) {
	var set [5]bool
	ok := true
	for _, entry := range strings.Split(list, ",") {
		for _, word := range strings.Split(strings.TrimSpace(entry), " & ") {
			switch strings.ToLower(strings.TrimSpace(word)) {
			case "":
			case "all":
				set = [5]bool{true, true, true, true, true}
			case "colorless":
				// The empty set; ok stays true. Whether that grant does anything
				// (an overwrite to colourless) or nothing (an add of the empty
				// set) is the CALLER's decision -- effAnimate notes the no-op arm.
			case "white":
				set[0] = true
			case "blue":
				set[1] = true
			case "black":
				set[2] = true
			case "red":
				set[3] = true
			case "green":
				set[4] = true
			default:
				ok = false
			}
		}
	}
	var out []string
	for i, c := range "WUBRG" {
		if set[i] {
			out = append(out, string(c))
		}
	}
	return out, ok
}

// ColorLetters is colorLetters for callers outside this package (rules'
// static scan reading a card's SetColor$). It is the same colour-word
// vocabulary -- commas or " & " between entries -- and the same fail-closed
// contract: ok=false means at least one word was not recognised, so the
// caller must not overwrite the target's colours with the partial prefix.
// "Colorless" parses to an empty set with ok=true, which is a real
// overwrite-to-colourless for a SetColor$ grant (Imprisoned in the Moon),
// distinct from the fail-closed arm.
func ColorLetters(list string) ([]string, bool) {
	return colorLetters(list)
}
