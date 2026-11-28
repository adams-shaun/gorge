package mtgish

import (
	"fmt"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

// Mana symbols, layouts and loyalty register here; compile.go reads the
// characteristics through them.

var charsColorBits = map[byte]ir.ColorSet{'W': ir.ColorW, 'U': ir.ColorU, 'B': ir.ColorB, 'R': ir.ColorR, 'G': ir.ColorG}

var charsColorNames = map[string]ir.ColorSet{"White": ir.ColorW, "Blue": ir.ColorU, "Black": ir.ColorB, "Red": ir.ColorR, "Green": ir.ColorG}

// charsLayouts maps the mtgish layouts cardc plays; every other layout is
// refused whole.
var charsLayouts = map[string]ir.Layout{
	"Card":         ir.LayoutNormal,
	"Transforming": ir.LayoutTransform,
	"ModalDFC":     ir.LayoutModalDFC,
	"Split":        ir.LayoutSplit,
	"Adventurer":   ir.LayoutAdventure,
	"Flip":         ir.LayoutFlip,
	"Melded":       ir.LayoutMeld,
	"MeldPiece":    ir.LayoutMeld,
	"Preparer":     ir.LayoutPrepare,
	"Room":         ir.LayoutRoom,
}

func charsColors(s string) ir.ColorSet {
	var c ir.ColorSet
	for i := 0; i < len(s); i++ {
		c |= charsColorBits[s[i]]
	}
	return c
}

func charsSym(sym ir.ManaSym) lowerFunc {
	return func(c *Ctx, n node) (any, error) {
		s := sym
		return &s, nil
	}
}

func init() {
	register("ManaSymbol.ManaCostGeneric", func(c *Ctx, n node) (any, error) {
		v, ok := intOf(n["args"])
		if !ok {
			return nil, fmt.Errorf("cardc: %s: generic mana needs a number", c.Src())
		}
		return &ir.ManaSym{Kind: ir.ManaGeneric, N: int32(v)}, nil
	})
	for _, l := range "WUBRG" {
		s := string(l)
		register("ManaSymbol.ManaCost"+s, charsSym(ir.ManaSym{Kind: ir.ManaColored, Colors: charsColors(s)}))
		register("ManaSymbol.ManaCost"+s+"P", charsSym(ir.ManaSym{Kind: ir.ManaPhyrexian, Colors: charsColors(s)}))
		register("ManaSymbol.ManaCost2"+s, charsSym(ir.ManaSym{Kind: ir.ManaTwoHybrid, Colors: charsColors(s), N: 2}))
	}
	for _, h := range []string{"WU", "UB", "BR", "RG", "GW", "WB", "UR", "BG", "RW", "GU"} {
		register("ManaSymbol.ManaCost"+h, charsSym(ir.ManaSym{Kind: ir.ManaHybrid, Colors: charsColors(h)}))
	}
	for _, h := range []string{"RW", "RG", "GW", "GU"} {
		register("ManaSymbol.ManaCost"+h+"P", charsSym(ir.ManaSym{Kind: ir.ManaHybridPhyrexian, Colors: charsColors(h)}))
	}
	register("ManaSymbol.ManaCostC", charsSym(ir.ManaSym{Kind: ir.ManaColorless}))
	register("ManaSymbol.ManaCostX", charsSym(ir.ManaSym{Kind: ir.ManaX}))
	register("ManaSymbol.ManaCostS", charsSym(ir.ManaSym{Kind: ir.ManaSnow}))

	for name, l := range charsLayouts {
		l := l
		register("OracleCard."+name, func(c *Ctx, n node) (any, error) { return &l, nil })
	}

	register("LoyaltyNumber.Integer", func(c *Ctx, n node) (any, error) {
		v, ok := intOf(n["args"])
		if !ok {
			return nil, fmt.Errorf("cardc: %s: loyalty needs a number", c.Src())
		}
		x := int32(v)
		return &x, nil
	})
}
