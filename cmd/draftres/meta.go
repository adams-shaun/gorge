package main

import (
	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/engine/cards"
)

// metaTypes is a deck's spell-type counts. A struct, not a map, so the key
// order is fixed.
type metaTypes struct {
	Creature     int `json:"creature"`
	Instant      int `json:"instant"`
	Sorcery      int `json:"sorcery"`
	Enchantment  int `json:"enchantment"`
	Artifact     int `json:"artifact"`
	Planeswalker int `json:"planeswalker"`
	Battle       int `json:"battle"`
	Other        int `json:"other"`
}

// meta is one deck's shape: its colours and splash, its spell, land and type
// counts, and its curve.
type meta struct {
	Colours       string    `json:"colours"`
	Splash        string    `json:"splash"`
	Archetype     string    `json:"archetype"`
	ColourCount   int       `json:"colour_count"`
	SplashCards   int       `json:"splash_cards"`
	Unsourced     int       `json:"unsourced"`
	Spells        int       `json:"spells"`
	LandsBasic    int       `json:"lands_basic"`
	LandsNonbasic int       `json:"lands_nonbasic"`
	PoolSize      int       `json:"pool_size"`
	Curve         [6]int    `json:"curve"`
	Types         metaTypes `json:"types"`
}

const wubrg = "WUBRG"

// describe reads one built deck's shape through the lab's corpus. Spells are
// the maindeck cards whose front face is not a land; pips are summed over
// every face's mana cost.
func describe(lab *draftlab.Lab, pool []limited.CardRef, d draftlab.Deck) meta {
	m := meta{PoolSize: len(pool)}
	if d.Build == nil {
		return m
	}
	refs := make(map[string]limited.CardRef, len(pool))
	for _, ref := range pool {
		refs[ref.ID] = ref
	}

	var sourced uint8
	for i, n := range d.Build.Basics {
		m.LandsBasic += n
		if n > 0 {
			sourced |= 1 << i
		}
	}

	var pips [5]int
	var spells []*cards.Card
	for _, id := range d.Build.Main {
		ref, ok := refs[id]
		if !ok {
			continue
		}
		c, ok := lab.Card(ref.Name)
		if !ok || c == nil || len(c.Faces) == 0 {
			continue
		}
		front := c.Faces[0]
		if front.IsLand() {
			if !front.IsBasic() {
				m.LandsNonbasic++
				sourced |= pickbot.LandColours(front)
			}
			continue
		}
		m.Spells++
		m.Curve[pickbot.CurveBucket(int(front.Cmc()))]++
		switch pickbot.ClassName(front) {
		case "creature":
			m.Types.Creature++
		case "instant":
			m.Types.Instant++
		case "sorcery":
			m.Types.Sorcery++
		case "enchantment":
			m.Types.Enchantment++
		case "artifact":
			m.Types.Artifact++
		case "planeswalker":
			m.Types.Planeswalker++
		case "battle":
			m.Types.Battle++
		default:
			m.Types.Other++
		}
		for _, face := range c.Faces {
			for i := 0; i < 5; i++ {
				pips[i] += int(cards.ManaCostChroma(face.ManaCost, 1<<i))
			}
		}
		spells = append(spells, c)
	}

	var chosen [5]bool
	for k := 0; k < 2; k++ {
		best := -1
		for i := 0; i < 5; i++ {
			if chosen[i] || pips[i] <= 0 {
				continue
			}
			if best < 0 || pips[i] > pips[best] {
				best = i
			}
		}
		if best < 0 {
			break
		}
		chosen[best] = true
	}
	var splashMask uint8
	var colourStr, splashStr []byte
	for i := 0; i < 5; i++ {
		switch {
		case pips[i] <= 0:
			continue
		case chosen[i]:
			colourStr = append(colourStr, wubrg[i])
		default:
			splashStr = append(splashStr, wubrg[i])
			splashMask |= 1 << i
		}
		m.ColourCount++
	}
	if len(colourStr) == 0 {
		colourStr = []byte("C")
	}
	m.Colours = string(colourStr)
	m.Splash = string(splashStr)
	m.Archetype = m.Colours
	if m.Splash != "" {
		m.Archetype += "+" + m.Splash
	}

	for _, c := range spells {
		id := c.ColourIdentity()
		if id&splashMask != 0 {
			m.SplashCards++
		}
		if id&^sourced != 0 {
			m.Unsourced++
		}
	}
	return m
}
