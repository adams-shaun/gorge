package mtgish

import "github.com/adams-shaun/gorge/engine/cardc/ir"

// charsKeywordNames are the keyword-only rules: variant to the keyword's
// rules name. All carry no args in the data.
var charsKeywordNames = map[string]string{
	"Flying": "Flying", "Deathtouch": "Deathtouch", "Defender": "Defender", "Flash": "Flash",
	"Haste": "Haste", "Hexproof": "Hexproof", "Indestructible": "Indestructible", "Lifelink": "Lifelink",
	"Menace": "Menace", "Reach": "Reach", "Trample": "Trample", "Vigilance": "Vigilance",
	"Prowess": "Prowess", "Exalted": "Exalted", "Storm": "Storm", "Undying": "Undying",
	"Delve": "Delve", "Evolve": "Evolve",
	"DoubleStrike": "Double Strike", "FirstStrike": "First Strike", "LivingWeapon": "Living Weapon",
}

func init() {
	for v, name := range charsKeywordNames {
		key := "Rule." + v
		name := name
		register(key, func(c *Ctx, n node) (any, error) {
			if _, has := n["args"]; has {
				return nil, &RefusalError{Key: key, Path: c.Src()}
			}
			src := c.Src()
			return &ir.Ability{Kind: ir.AbKeyword, Keyword: &ir.Keyword{Name: name, Src: src}, Src: src}, nil
		})
	}
}
