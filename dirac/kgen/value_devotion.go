package kgen

import (
	"fmt"
	"strings"
)

// devotionValue lowers Count$Devotion.<colour>[.<colour>] (gorge
// count_body_heads evalCountBodyDottedDevotion/-Dual -> devotionCount: the
// mana symbols among the controller's permanents' mana costs whose colours
// meet the mask, a two-colour symbol counted once). Chosen and anything
// unreadable refuse.
func devotionValue(cols string) (lValue, error) {
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
