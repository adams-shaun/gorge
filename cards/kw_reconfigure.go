// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

// kwReconfigure expands K:Reconfigure (CR 702.150) into the activated
// abilities the keyword prints, riding the same AB$ Attach primitive
// kw:Equip mints. For each printed cost the expansion mints a PAIR:
//
//   - the attach half: "AB$ Attach | Cost$ <cost> | ValidTgts$
//     Creature.YouCtrl | SorcerySpeed$ True" -- CR 702.150a fixes the
//     target to "another target creature you control" (Equip's line
//     carries its own restriction field; Reconfigure's does not, so the
//     CR wording IS the restriction), and effAttach's own self-exclusion
//     makes "another" real. The target ask's ValidTgts$ Creature spec
//     also cannot offer an already-attached reconfigurer once the
//     not-a-creature switch is live (state.Object.ReconfiguredAttached
//     through effects/filter.go's hasType gate).
//   - the unattach half: the same shape with `Unattach$ True` and no
//     target, resolving to the no-IDs Attach event -- the detach encoding
//     rules/attach.go's CR 704.5n SBA already folds. rules/legal.go's
//     printed-ability offer loop withholds an Unattach$ ability while the
//     source is unattached ("unattach from a creature" has no legal
//     action there, and a payable no-op the bot can answer forever is the
//     livelock shape the offer gates exist to withhold).
//
// Both halves carry `SorcerySpeed$ True` (CR 702.150a: "Reconfigure only
// as a sorcery"), the same rider the Equip expansion mints and the offer
// loop's sorcery gate reads.
//
// param is "<cost>" optionally followed by a second colon field that is an
// ALTERNATIVE cost: Razorfield Ripper's `K:Reconfigure:2:PayEnergy<3>` is
// "Pay {2} or {E}{E}{E}", so CR 702.150a's "[Cost]" is one cost printed as
// alternatives and each alternative gets its own ability pair. The second
// field is read as an alternative cost only when it is space-free and
// carries a Forge "<...>" token; any other field (prose, or an
// Equip-style target restriction the corpus never carries on this
// keyword) is dropped. Measured over the corpus pin: the 21
// K:Reconfigure lines' only second field anywhere is that one
// PayEnergy<3>.
func kwReconfigure(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("A", k) {
		return
	}
	fields := strings.Split(param, ":")
	costs := []string{strings.TrimSpace(fields[0])}
	if len(fields) > 1 {
		alt := strings.TrimSpace(fields[1])
		if alt != "" && !strings.Contains(alt, " ") && strings.Contains(alt, "<") {
			costs = append(costs, alt)
		}
	}
	for _, cost := range costs {
		if sa, _ := parseSA("", "AB$ Attach | Cost$ "+cost+
			" | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | SorcerySpeed$ True | Keyword$ Reconfigure | SpellDescription$ Reconfigure -- pay "+cost+": attach to target creature you control"); sa != nil {
			sa.Params["KeywordLine"] = k
			f.Abilities = append(f.Abilities, sa)
		}
		if sa, _ := parseSA("", "AB$ Attach | Cost$ "+cost+
			" | Unattach$ True | SorcerySpeed$ True | Keyword$ Reconfigure | SpellDescription$ Reconfigure -- pay "+cost+": unattach"); sa != nil {
			sa.Params["KeywordLine"] = k
			f.Abilities = append(f.Abilities, sa)
		}
	}
}

func init() { registerKeyword(kwReconfigure, "Reconfigure") }
