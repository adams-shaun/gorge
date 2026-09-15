package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() { Register("ManaReflected", effManaReflected) }

// manaReflectedOrder is the fixed candidate order. Never a map, so the
// offered option list and deterministic single-colour pick are stable.
const manaReflectedOrder = "WUBRGC"

// ManaReflectedCandidates resolves which mana symbols an "AB$ ManaReflected"
// ability can add, in the fixed manaReflectedOrder order. Two params drive it:
//
//   - Valid$ names the objects the ability reflects FROM. A "Defined.<name>"
//     value resolves that name through the ordinary Defined machinery (the
//     resolving context's Remembered/Targets/Source), so Chrome Mox's
//     "Defined.Imprinted" resolves to whatever imprint record the engine
//     keeps; every other value is an object spec matched against the
//     battlefield in AliveFrom(0) x zone order via MatchesSpecFrom, with the
//     resolving source as the filter's "Self" and its controller as "You".
//
//   - ReflectProperty$ decides what is read: "Produce" unions what the
//     matching objects' mana abilities could produce; "Is" reads the matching
//     objects' colours; "Produced" reads the mana types captured from the
//     mana event that caused a TapsForMana trigger. Produced intentionally
//     does not scan Defined$ objects: those values name the player receiving
//     the additional mana, not an object (Mana Flare/Kinnan).
//
// ColorOrType$ "Color" keeps coloured symbols. "Type" additionally admits
// colourless, but only when a reflected mana ability can actually produce
// {C}; it never manufactures colourless merely because the parameter appears.
//
// An unknown element in either parameter degrades to an empty set, which the
// executor reports as a Note rather than inventing mana.
func ManaReflectedCandidates(h Host, c *Ctx, sa *cards.SA) []string {
	property := strings.TrimSpace(sa.Params["ReflectProperty"])
	widenType := strings.TrimSpace(sa.Params["ColorOrType"]) == "Type"
	if property == "Produced" {
		set := map[byte]bool{}
		for _, r := range c.TriggerMana {
			if strings.ContainsRune("WUBRGC", r) && (widenType || r != 'C') {
				set[byte(r)] = true
			}
		}
		var out []string
		for _, symbol := range manaReflectedOrder {
			if set[byte(symbol)] {
				out = append(out, string(symbol))
			}
		}
		return out
	}
	if property != "Produce" && property != "Is" {
		return nil
	}
	spec := strings.TrimSpace(sa.Params["Valid"])
	if spec == "" {
		return nil
	}
	var objs []state.ObjID
	if rest, ok := strings.CutPrefix(spec, "Defined."); ok {
		// A "Defined.<name>" spec: resolve <name> through the ordinary
		// Defined resolver against this resolution's own context. The scan
		// copy carries only the Defined$ param, so the resolver cannot read
		// anything else off the script line.
		scan := *sa
		scan.Params = map[string]string{"Defined": strings.TrimSpace(rest)}
		for _, t := range Defined(h, c, &scan) {
			if !t.IsPlayer {
				objs = append(objs, t.Obj)
			}
		}
	} else {
		g := h.Game()
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				if MatchesSpecFrom(g, spec, id, c.Controller, c.Source) {
					objs = append(objs, id)
				}
			}
		}
	}
	produce := property == "Produce"
	set := map[string]bool{}
	for _, id := range objs {
		o := h.Game().Obj(id)
		if o == nil {
			continue
		}
		var syms string
		if produce {
			syms = producibleSymbols(o)
		} else {
			syms = ColorsOf(o)
		}
		for _, r := range syms {
			set[string(r)] = true
		}
	}
	if !widenType {
		delete(set, "C")
	}
	var out []string
	for _, s := range manaReflectedOrder {
		if set[string(s)] {
			out = append(out, string(s))
		}
	}
	return out
}

// producibleSymbols unions the mana symbols one permanent's mana abilities
// could produce. It mirrors effMana's own classifier exactly: a blank or
// "Any"/"Combo Any" Produced$ is every colour; a "Combo <colours>" shape is
// exactly those colours (effects.ComboColours rejects every other combo
// word); a plain brace/space-stripped rune list contributes its coloured and
// colourless runes. Amount$ and RestrictValid$ belong to the mana the
// reflected ability itself produces, not the source ability it examines, so
// they intentionally do not affect this candidate census.
func producibleSymbols(o *state.Object) string {
	f := o.Face()
	if f == nil {
		return ""
	}
	set := map[byte]bool{}
	for _, ma := range f.ManaAbilities() {
		p := strings.TrimSpace(ma.Params["Produced"])
		switch {
		case p == "" || p == "Any" || p == "Combo Any":
			for _, r := range "WUBRG" {
				set[byte(r)] = true
			}
			continue
		}
		if cols, ok := ComboColours(p); ok {
			for _, col := range cols {
				set[col[0]] = true
			}
			continue
		}
		runes := strings.NewReplacer("{", "", "}", "", " ", "").Replace(p)
		for _, r := range runes {
			if strings.ContainsRune("WUBRGC", r) {
				set[byte(r)] = true
			}
		}
	}
	var b strings.Builder
	for _, c := range "WUBRGC" {
		if set[byte(c)] {
			b.WriteByte(byte(c))
		}
	}
	return b.String()
}

// effManaReflected implements "AB$ ManaReflected": add Amount$ (default one)
// mana of a colour the reflected set offers. The rules engine
// (rules/mana_activation.go) asks the colour whenever more than one is
// available and re-enters this effect with Produced$ overridden to the chosen
// letter, so the override is the authoritative single-colour path.
// RestrictValid$ is retained on each ManaAdd event, rather than discarded when
// the pool coalesces equal-colour mana; rules payment consumes that provenance
// only for a matching spell or activated ability.
//
// An empty set adds nothing and records a Note: Exotic Orchard tapped with no
// opponent land in play, or Chrome Mox with no imprinted card recorded, must
// not invent a colour.
func effManaReflected(h Host, c *Ctx, sa *cards.SA) {
	produced := strings.TrimSpace(sa.Params["Produced"])
	amount := Num(h, c, sa, "Amount", 1)
	if amount < 0 {
		amount = 0
	}
	restriction := strings.TrimSpace(sa.Params["RestrictValid"])
	manaAdd := func(player state.PlayerID, color string) {
		if amount == 0 {
			return
		}
		ev := events.Event{Kind: events.ManaAdd, Player: player, Counter: color, Amount: amount}
		if restriction != "" {
			ev.Text = events.ManaRestrictionText(restriction)
		}
		h.Emit(ev)
	}
	recipient := c.Controller
	if strings.TrimSpace(sa.Params["ReflectProperty"]) == "Produced" {
		// On this corpus shape Defined$ names who receives the additional
		// mana (TriggeredActivator, TriggeredCardController, or You), unlike
		// Produce/Is where Valid$ names reflected objects. Resolve those three
		// roles locally so the ordinary Defined grammar is not widened for
		// unrelated effects.
		switch strings.TrimSpace(sa.Params["Defined"]) {
		case "TriggeredActivator":
			if c.TriggerPlayer.IsPlayer {
				recipient = c.TriggerPlayer.Player
			}
		case "TriggeredCardController":
			if o := h.Game().Obj(c.TriggerCard); o != nil {
				recipient = o.Controller
			}
		}
	}
	// The rules-engine colour-ask path re-enters with Produced$ set to one
	// plain letter.
	if len(produced) == 1 && strings.ContainsRune(ManaSymbols, rune(produced[0])) {
		manaAdd(recipient, produced)
		return
	}
	cols := ManaReflectedCandidates(h, c, sa)
	switch len(cols) {
	case 0:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "ManaReflected found no mana to reflect"})
	case 1:
		manaAdd(recipient, cols[0])
	default:
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "chose first reflected colour " + cols[0] + " (no ask possible)"})
		manaAdd(recipient, cols[0])
	}
}
