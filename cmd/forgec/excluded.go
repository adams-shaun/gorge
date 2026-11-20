package main

import (
	"fmt"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The exclusion register: cards the paper rules ban in every format, which
// gorge therefore never certifies and never implements. One entry per card,
// with the cause, the paper-rule citation and the missing primitives that
// keep it unplayable in this build, so every census (forgec report's gap
// table, the coverage breakdown) diagnoses them as exclusions instead of
// counting them as unplayable gaps waiting on engine work.
//
// The register can only shrink: when gorge ever supports every primitive an
// entry names, that entry has become a lie about what is missing and
// TestExclusionRegister fails until the entry is removed. Adding an entry
// for a card gorge can already play fails the same way.
//
// Causes:
//
//   - "ante": cards that ante (CR 407). Removed from the deck before play
//     unless the game is played for ante, and banned in every sanctioned
//     format, so they can never be certified.
//   - "dexterity": physical-dexterity cards (Chaos Orb, Falling Star) whose
//     resolution is a physical act the engine cannot adjudicate; banned in
//     every sanctioned format.
//   - "subgame": Shahrazad, the only subgame card; banned in every
//     sanctioned format.
type excludedCard struct {
	// Name is the card's corpus name, exactly as Forge's script prints it.
	Name string
	// Cause is one of "ante", "dexterity", "subgame".
	Cause string
	// Citation is the paper rule that bans the card.
	Citation string
	// Prims are the missing primitives that keep this card unplayable in
	// this build (cards.Registry.Unsupported against effects.Supported).
	Prims []string
}

// antePrim is the corpus keyword every ante card's script carries
// ("K:Remove CARDNAME from your deck before playing if you're not playing
// for ante."), unimplemented and therefore unsupported, and itself the
// paper rule's marker on the script.
const antePrim = "kw:Remove CARDNAME from your deck before playing if you're not playing for ante."

// excludedRegister is the register, one entry per card, sorted by name. The
// paper-rule citation per cause follows the brief: CR 407 for the ante
// cards, the every-format ban for the rest.
var excludedRegister = []excludedCard{
	{Name: "Amulet of Quoz", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim}},
	{Name: "Bronze Tablet", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim, "api:GainOwnership"}},
	{Name: "Chaos Orb", Cause: "dexterity", Citation: "banned in every sanctioned format", Prims: []string{"api:FlipOntoBattlefield"}},
	{Name: "Contract from Below", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim}},
	{Name: "Darkpact", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim, "api:ExchangeZone", "api:GainOwnership"}},
	{Name: "Demonic Attorney", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim}},
	{Name: "Falling Star", Cause: "dexterity", Citation: "banned in every sanctioned format", Prims: []string{"api:FlipOntoBattlefield"}},
	{Name: "Jeweled Bird", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim}},
	{Name: "Rebirth", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim}},
	{Name: "Shahrazad", Cause: "subgame", Citation: "banned in every sanctioned format", Prims: []string{"api:Subgame"}},
	{Name: "Tempest Efreet", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim, "api:GainOwnership"}},
	{Name: "Timmerian Fiends", Cause: "ante", Citation: "CR 407 (ante); banned in every sanctioned format", Prims: []string{antePrim, "api:GainOwnership"}},
}

// excludedCauses lists the register's causes in print order.
var excludedCauses = []string{"ante", "dexterity", "subgame"}

// excludedByName is the register keyed by cards.NormalizeName, so the
// coverage hook answers by key and never ranges the register.
var excludedByName = func() map[string]bool {
	m := make(map[string]bool, len(excludedRegister))
	for _, e := range excludedRegister {
		m[cards.NormalizeName(e.Name)] = true
	}
	return m
}()

// paperExcluded is cards.Registry.Excluded for the coverage report: a card
// the register names (by front-face name, like deck's ban lists) is
// excluded from the coverage universe rather than counted as an unplayable
// gap. See excludedRegister.
func paperExcluded(c *cards.Card) bool {
	if c == nil || len(c.Faces) == 0 {
		return false
	}
	return excludedByName[cards.NormalizeName(c.Faces[0].Name)]
}

// excludedCauseCounts counts the register's entries per cause, in
// excludedCauses order, for the report's excluded line.
func excludedCauseCounts() (int, string) {
	byCause := map[string]int{}
	for _, e := range excludedRegister {
		byCause[e.Cause]++
	}
	total := 0
	var parts []string
	for _, cause := range excludedCauses {
		if n := byCause[cause]; n > 0 {
			parts = append(parts, fmt.Sprintf("%s: %d", cause, n))
			total += n
		}
	}
	detail := ""
	if len(parts) > 0 {
		detail = " (" + strings.Join(parts, ", ") + ")"
	}
	return total, detail
}
