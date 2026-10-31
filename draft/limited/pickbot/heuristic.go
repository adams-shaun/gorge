package pickbot

import (
	"strings"

	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/engine/cards"
)

// rarityBase is the heuristic's starting score by printing rarity (C 0, U 2,
// R 5, M 6); anything else — basic lands, special
// printings — starts at 0.
func rarityBase(r limited.Rarity) float64 {
	switch r {
	case limited.Uncommon:
		return 2
	case limited.Rare:
		return 5
	case limited.Mythic:
		return 6
	default:
		return 0
	}
}

// evasionKeywords are the evasion keywords the heuristic pays a small bonus
// for, each +1 up to a +2 cap.
var evasionKeywords = []string{"Flying", "Menace", "Shadow", "Intimidate", "Fear", "Skulk"}

// Heuristic rates one card from its printed facts alone: the rarity base,
// plus a creature term from P/T against mana value, plus small bonuses for
// evasion keywords and for removal-like spell types. It is the fallback
// for every set no rating file covers, on the same rough scale as the
// rated sets' metric points (see Tuning.HeuristicCentre/HeuristicScale). A
// nil card scores its rarity base only.
func Heuristic(c *cards.Card, rarity limited.Rarity) float64 {
	score := rarityBase(rarity)
	if c == nil || len(c.Faces) == 0 {
		return score
	}
	f := c.Faces[0]
	if f.IsCreature() {
		stat := float64(f.Power()+f.Toughness()) - 2*float64(f.Cmc())
		term := 0.5 * stat
		if term > 3 {
			term = 3
		}
		if term < -3 {
			term = -3
		}
		score += term
	}
	evasion := 0
	for _, k := range evasionKeywords {
		if evasion == 2 {
			break
		}
		if f.HasKeyword(k) {
			evasion++
		}
	}
	score += float64(evasion)
	if f.IsInstant() || f.IsSorcery() {
		o := strings.ToLower(f.Oracle)
		if strings.Contains(o, "destroy target") ||
			strings.Contains(o, "exile target") ||
			strings.Contains(o, "damage to target") {
			score += 2
		}
	}
	return score
}
