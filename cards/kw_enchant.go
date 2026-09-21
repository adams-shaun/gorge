// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwEnchant(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) || f.SpellAbility() != nil {
		return
	}
	// param is "<spec>", occasionally followed by the human-
	// readable prompt Forge itself shows ("Creature.YouCtrl:
	// creature you control"). When present, that third field is
	// used verbatim as the prompt; when absent, one is generated
	// from the spec the same way the brief's table describes.
	spec, prompt, hasPrompt := strings.Cut(param, ":")
	if !hasPrompt || prompt == "" {
		prompt = strings.ToLower(spec)
	}
	sa, _ := parseSA("", "SP$ Attach | ValidTgts$ "+spec+" | TgtPrompt$ Select target "+prompt+" | Object$ Self | Keyword$ Enchant")
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwEnchant, "Enchant") }
