package kgen

import (
	"fmt"
	"strings"
)

// devotionValue lowers Count$Devotion.<colour>[.<colour>] (gorge
// count_body_heads evalCountBodyDottedDevotion/-Dual -> devotionCount: the
// mana symbols among the controller's permanents' mana costs whose colours
// meet the mask, a two-colour symbol counted once). Chosen is the source's
// recorded colour (vDevotionChosen, kop.ExDevotionChosen); anything
// unreadable refuses.
func devotionValue(cols string) (lValue, error) {
	if strings.EqualFold(strings.TrimSpace(cols), "Chosen") {
		return lValue{Kind: vDevotionChosen}, nil
	}
	letters := map[string]int{"W": 1, "WHITE": 1, "U": 2, "BLUE": 2, "B": 4, "BLACK": 4, "R": 8, "RED": 8, "G": 16, "GREEN": 16}
	parts := strings.Split(cols, ".")
	if len(parts) > 2 {
		return lValue{}, fmt.Errorf("Count$Devotion.%s", cols)
	}
	mask := 0
	for _, p := range parts {
		b, ok := letters[strings.ToUpper(strings.TrimSpace(p))]
		if !ok {
			return lValue{}, fmt.Errorf("Count$Devotion.%s", cols)
		}
		mask |= b
	}
	return lValue{Kind: vDevotion, N: mask}, nil
}

// devotionHead lowers the Devotion.<colour>[.<colour>] head and its
// two-colour spelling DevotionDual.<colour>.<colour> (gorge
// evalCountBodyDottedDevotionDual: one scan over the union mask, the
// Devotion head's read). ok is false for any other head.
func devotionHead(rest string) (v lValue, ok bool, err error) {
	for _, pre := range [...]string{"Devotion.", "DevotionDual."} {
		if cols, found := strings.CutPrefix(rest, pre); found {
			if pre == "DevotionDual." && len(strings.Split(cols, ".")) != 2 {
				// gorge fails a Dual head without exactly two colours closed
				return lValue{}, true, fmt.Errorf("Count$%s%s", pre, cols)
			}
			v, err = devotionValue(cols)
			return v, true, err
		}
	}
	return lValue{}, false, nil
}
