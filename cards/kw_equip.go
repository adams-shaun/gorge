// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwEquip(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	kwAttachCost(f, i, k, param, has, attachKWCfg{
		kw: "Equip", defaultTgts: "Creature.YouCtrl", prompt: "Select target creature you control",
	})
}

// attachKWCfg is the per-keyword config the shared Attach-cost expander
// kwAttachCost needs: kw is the keyword name (the Keyword$ tag and the
// SpellDescription$ head), defaultTgts the ValidTgts$ a line with no
// restriction field gets, prompt the TgtPrompt$.
type attachKWCfg struct{ kw, defaultTgts, prompt string }

// kwAttachCost is the Equip-style cost/attachment expansion kw:Equip and
// kw:Fortify share (CR 702.33 / CR 702.67 -- Fortify is Equip for lands, so
// cards/kw_fortify.go registers it against this parser with the land-facing
// defaults).
func kwAttachCost(f *Face, i int, k, param string, has func(kind, line string) bool, cfg attachKWCfg) {
	if has("A", k) {
		return
	}
	// param is "<cost>" followed by colon fields: a target
	// restriction ("3:Creature.YouCtrl+Legendary:legendary creature"),
	// rider parameters ("4:::ReduceCost$ Monarch:...",
	// "0:::ActivationLimit$ 1:...") and human-readable text. The first
	// field is exactly the cost: no corpus equip cost itself contains
	// a ":" (mana symbols, Sac<1/Creature>, PayLife<3> and so on are
	// all safe; measured over all 646 raw K:Equip lines at the corpus
	// pin -- the split-on-":" is a corpus invariant, not an
	// assumption to re-litigate per card).
	// The trailing fields are read, not dropped wholesale:
	//   - a "ReduceCost$ <v>" / "ActivationLimit$ <v>" / "AlternateCost$ <v>"
	//     field rides the minted SA verbatim; rules/legal.go's ownReduceCost
	//     and the offer loop's ActivationLimit gate already read the first
	//     two, and rules/activate.go's abilityAlternateCost reads the third
	//     (CR 702.6 / CR 601.2f: Transmogrant's Crown's "Equip {2} ... you
	//     may pay {B} instead" -- an alternative cost the activator may pay
	//     in place of the printed Equip cost).
	//   - the FIRST remaining field that is neither a rider nor a
	//     "Flavor " marker is the target restriction, a real filter
	//     spec passed through verbatim as ValidTgts$ (comma
	//     alternatives included); later fields are display text and
	//     stay dropped, as before. The space-free test separates spec
	//     from prose: every restriction spec in the corpus is
	//     space-free ("Creature.YouCtrl+Legendary",
	//     "Creature.YouCtrl+Shaman,..." — commas, never spaces), while
	//     every description field carries spaces ("legendary
	//     creature", "This ability costs {3} less to activate if
	//     you're the monarch"); the one-word descs ("Soldier",
	//     "commander") only ever trail a real restriction, so the
	//     first-real-field rule already claimed the slot.
	//   - any other "<Head>$ <value>" field is an unwired rider family --
	//     dropped, as today, but never mistaken for a restriction spec.
	fields := strings.Split(param, ":")
	cost := fields[0]
	restriction := ""
	var riders []string
	for _, fld := range fields[1:] {
		fld = strings.TrimSpace(fld)
		if fld == "" || strings.HasPrefix(fld, "Flavor ") {
			continue
		}
		head, _, isParam := strings.Cut(fld, " ")
		if isParam && strings.HasSuffix(head, "$") {
			if head == "ReduceCost$" || head == "ActivationLimit$" || head == "AlternateCost$" {
				riders = append(riders, fld)
			}
			continue
		}
		if restriction == "" && !strings.Contains(fld, " ") {
			restriction = fld
		}
	}
	tgts := cfg.defaultTgts
	if restriction != "" {
		tgts = restriction
	}
	saStr := "AB$ Attach | Cost$ " + cost + " | ValidTgts$ " + tgts + " | TgtPrompt$ " + cfg.prompt + " | SorcerySpeed$ True | Keyword$ " + cfg.kw + " | SpellDescription$ " + cfg.kw + " " + cost
	for _, r := range riders {
		saStr += " | " + r
	}
	sa, _ := parseSA("", saStr)
	if sa != nil {
		sa.Params["KeywordLine"] = k
		f.Abilities = append(f.Abilities, sa)
	}
}

func init() { registerKeyword(kwEquip, "Equip") }
