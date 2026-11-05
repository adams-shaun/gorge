package main

import (
	"fmt"
	"strings"
)

// summarise renders the pool's shape as the fixed eight-line summary.
func summarise(metas []meta) string {
	n := len(metas)
	if n == 0 {
		return ""
	}
	var b strings.Builder

	minCreatures, maxCreatures, sumCreatures := metas[0].Types.Creature, metas[0].Types.Creature, 0
	var curve [6]int
	for _, m := range metas {
		c := m.Types.Creature
		if c < minCreatures {
			minCreatures = c
		}
		if c > maxCreatures {
			maxCreatures = c
		}
		sumCreatures += c
		for i := range curve {
			curve[i] += m.Curve[i]
		}
	}
	fmt.Fprintf(&b, "creatures min %d mean %.2f max %d\n", minCreatures, float64(sumCreatures)/float64(n), maxCreatures)
	fmt.Fprintf(&b, "curve mean <=1 %.2f 2 %.2f 3 %.2f 4 %.2f 5 %.2f 6+ %.2f\n",
		float64(curve[0])/float64(n), float64(curve[1])/float64(n), float64(curve[2])/float64(n),
		float64(curve[3])/float64(n), float64(curve[4])/float64(n), float64(curve[5])/float64(n))

	classCounts := func(get func(meta) int) (zero, one, many int) {
		for _, m := range metas {
			switch v := get(m); {
			case v == 0:
				zero++
			case v == 1:
				one++
			default:
				many++
			}
		}
		return
	}
	for _, row := range []struct {
		name string
		get  func(meta) int
	}{
		{"instant", func(m meta) int { return m.Types.Instant }},
		{"sorcery", func(m meta) int { return m.Types.Sorcery }},
		{"enchantment", func(m meta) int { return m.Types.Enchantment }},
		{"artifact", func(m meta) int { return m.Types.Artifact }},
	} {
		zero, one, many := classCounts(row.get)
		fmt.Fprintf(&b, "%s 0:%d 1:%d 2+:%d\n", row.name, zero, one, many)
	}

	var one, two, three, fourPlus int
	var splash, lands17, lands18, unsourced int
	for _, m := range metas {
		switch m.ColourCount {
		case 1:
			one++
		case 2:
			two++
		case 3:
			three++
		default:
			if m.ColourCount >= 4 {
				fourPlus++
			}
		}
		if m.Splash != "" {
			splash++
		}
		switch m.LandsBasic + m.LandsNonbasic {
		case 17:
			lands17++
		case 18:
			lands18++
		}
		if m.Unsourced > 0 {
			unsourced++
		}
	}
	fmt.Fprintf(&b, "colours 1:%d 2:%d 3:%d 4+:%d\n", one, two, three, fourPlus)
	fmt.Fprintf(&b, "splash %d lands17 %d lands18 %d unsourced %d\n", splash, lands17, lands18, unsourced)
	return b.String()
}
