package cards

import (
	"strconv"
	"strings"
)

// manaBraceForm normalises a brace-form mana cost ("{2}{U}{U}") to the
// space-separated form cmcFromManaCost parses. Built once at package scope,
// not per call, because derive runs on every face of the corpus at load.
var manaBraceForm = strings.NewReplacer("{", " ", "}", " ")

func (f *Face) hasType(t string) bool {
	for _, x := range f.Types {
		if strings.EqualFold(x, t) {
			return true
		}
	}
	return false
}

func (f *Face) IsLand() bool         { return f.hasType("Land") }
func (f *Face) IsCreature() bool     { return f.hasType("Creature") }
func (f *Face) IsInstant() bool      { return f.hasType("Instant") }
func (f *Face) IsSorcery() bool      { return f.hasType("Sorcery") }
func (f *Face) IsArtifact() bool     { return f.hasType("Artifact") }
func (f *Face) IsEnchantment() bool  { return f.hasType("Enchantment") }
func (f *Face) IsPlaneswalker() bool { return f.hasType("Planeswalker") }

// IsPermanent reports whether resolving this face puts it onto the battlefield.
func (f *Face) IsPermanent() bool { return !f.IsInstant() && !f.IsSorcery() }

// KeywordHead strips a keyword's parameters: "Equip:2" is the Equip keyword.
// Keywords whose name contains spaces ("Protection from blue") keep them.
func KeywordHead(k string) string {
	if i := strings.IndexByte(k, ':'); i >= 0 {
		k = k[:i]
	}
	return strings.TrimSpace(k)
}

func (f *Face) HasKeyword(k string) bool {
	for _, x := range f.Keywords {
		if strings.EqualFold(KeywordHead(x), k) {
			return true
		}
	}
	return false
}

// KeywordParam returns the text after the colon of a parameterised keyword
// ("Kicker:B" -> "B"; "Equip:2" -> "2") and reports whether the keyword is
// printed at all ("Flash" -> "", true; absent -> "", false).
func (f *Face) KeywordParam(head string) (string, bool) {
	for _, k := range f.Keywords {
		if strings.EqualFold(KeywordHead(k), head) {
			if i := strings.IndexByte(k, ':'); i >= 0 {
				return strings.TrimSpace(k[i+1:]), true
			}
			return "", true
		}
	}
	return "", false
}

// SpellAbility is the SP$ ability a card casts with, if any.
func (f *Face) SpellAbility() *SA {
	for _, a := range f.Abilities {
		if a.Kind == "SP" {
			return a
		}
	}
	return nil
}

// ManaAbilities lists every activated ability that produces mana.
func (f *Face) ManaAbilities() []*SA {
	var out []*SA
	for _, a := range f.Abilities {
		if a.Kind == "AB" && a.API == "Mana" {
			out = append(out, a)
		}
	}
	return out
}

func (f *Face) Power() int     { return int(f.power) }
func (f *Face) Toughness() int { return int(f.toughness) }

// Cmc returns the face's converted mana cost, derived once at load from the
// printed ManaCost string. It mirrors botpolicy.CmcOf's arithmetic exactly
// (cards cannot import botpolicy or rules, so the few lines are duplicated
// here by design) so a face read the same way anywhere agrees. {X} counts as
// 0 off the stack, a hybrid/Phyrexian/colourless symbol as one generic.
func (f *Face) Cmc() int32 { return f.cmc }

// CharacteristicDefining reports whether the face's printed P/T is a
// characteristic-defining value ("*", "1+*"): Power()/Toughness() return 0
// for these and layer 7a (in rules) supplies the real value.
func (f *Face) CharacteristicDefining() bool { return f.characteristicDefining }

// derive computes the derived fields from the printed text fields. It must
// run after every path that constructs a Face values its printed fields from
// text — after ParseBytes and after the gob decode path — so the two
// construction routes produce identical faces. It is never run into the gob:
// the derived fields stay unexported (gob ignores them) and are recomputed on
// decode, so a stale cache whose gob zero-filled them is repaired with no
// error anywhere.
func (f *Face) derive() {
	f.power, f.toughness, f.characteristicDefining = parsePT(f.PT)
	f.cmc = cmcFromManaCost(f.ManaCost)
}

// parsePT splits a printed P/T ("2/2") into power and toughness. A face with
// no P/T yields 0,0 and flag false; a face whose P/T carries a
// characteristic-defining value ("*", "1+*") yields 0 for the affected side
// (exactly what pt used to return) and sets the flag, because layer 7a owns
// that value.
func parsePT(pt string) (pow, tgh int32, cd bool) {
	parts := strings.SplitN(pt, "/", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	for i, s := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(s))
		if err != nil {
			cd = true
			continue // 0 for this side; layer 7a owns characteristic-defining values
		}
		if i == 0 {
			pow = int32(n)
		} else {
			tgh = int32(n)
		}
	}
	return pow, tgh, cd
}

// cmcFromManaCost is cards' own conversion of a printed ManaCost string to a
// converted mana cost, an exact mirror of botpolicy.CmcOf. It deliberately
// re-derives rules/mana.go's ParseCost.CMC() by hand here because cards can
// import neither botpolicy nor rules.
func cmcFromManaCost(mc string) int32 {
	mc = manaBraceForm.Replace(mc)
	mc = strings.TrimSpace(mc)
	if mc == "" || strings.EqualFold(mc, "no cost") {
		return 0
	}
	var n int32
	for _, sym := range strings.Fields(mc) {
		if sym == "X" { // {X} is 0 off the stack
			continue
		}
		if len(sym) == 1 && strings.ContainsRune("WUBRGC", rune(sym[0])) { // a single coloured/colourless pip
			n++
			continue
		}
		if v, err := strconv.Atoi(sym); err == nil && v >= 0 {
			n += int32(v)
			continue
		}
		// Hybrid ("W/U"), Phyrexian ("UP"), and any other symbol: one generic.
		n++
	}
	return n
}
