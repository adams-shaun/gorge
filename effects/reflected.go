package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func init() {
	Register("ManaReflected", effManaReflected)
}

// effManaReflected implements the reflected-mana family (AB$ ManaReflected,
// 24 raw corpus lines): the ability adds mana of a colour the CHOSEN objects
// carry rather than of a fixed symbol, so the colour set is computed at
// resolution from the game state.
//
// Colour sources, by ReflectProperty$:
//
//   - "Is" (or absent): the object's colours (ColorsOf -- mana cost and the
//     Colors$ marker, Devoid respected).
//   - "Produce": the colours the object's mana abilities could produce
//     (Face.ManaProduction). An Any production adds all five, mirroring
//     ManaProduction.DistinctColours' treatment of a choice.
//
// ColorOrType$ Type adds colourless to the set; the default "Color" keeps
// only the five WUBRG colours.
//
// The Valid$ selector resolves as:
//
//   - "Defined.Self" -- the resolving source.
//   - "Defined.Imprinted" / "Defined.ExiledWith" -- every card in exile whose
//     ExiledWith provenance names the source (Chrome Mox / the Imprint family).
//   - "Defined.ValidGraveyard <spec>" -- cards in every seat's graveyard
//     matching <spec> (Urborg's "a color among cards in your graveyard").
//   - "Defined.Sacrificed" -- the resolution's remembered list (the sacrifice
//     the cost paid).
//   - "Defined.Untapped" -- the controller's untapped battlefield permanents
//     (the one untapYType cost shape).
//   - anything else -- a card spec matched over every seat's battlefield in
//     deterministic AliveFrom(0) order (Land.YouCtrl, Land.OppCtrl,
//     Gate.YouCtrl, Basic.YouCtrl, Permanent.Legendary+YouCtrl, ...).
//
// A selector this resolver cannot name FAILS CLOSED: a Note, no mana.
//
// Stand-in (the same one effMana's Combo head keeps): a reflected set with
// MORE THAN ONE colour degenerates to the full Amount in EVERY reflected
// colour -- the colour-choice ask is the M4 mana-choice milestone -- recorded
// with a Note. A set with exactly one colour is the exact behaviour (a forced
// choice is no choice), which is what Chrome Mox and every single-colour
// script hits.
//
// Amount$ goes through the ordinary Num evaluator (literal, SVar body or
// inline Count$; default 1).
func effManaReflected(h Host, c *Ctx, sa *cards.SA) {
	colours := reflectedColours(h, c, sa)
	if len(colours) == 0 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "no reflected mana colours"})
		return
	}
	amt := Num(h, c, sa, "Amount", 1)
	if amt < 0 {
		amt = 0
	}
	if len(colours) > 1 {
		h.Emit(events.Event{Kind: events.Note, Obj: c.Source,
			Text: "reflects several colours; adds the full amount in each (the colour-choice stand-in)"})
	}
	for _, p := range ManaRecipients(h, c, sa) {
		for _, col := range colours {
			h.Emit(events.Event{Kind: events.ManaAdd, Player: p,
				Counter: string(col), Amount: amt})
		}
	}
}

// reflectedColours computes the reflected mana set in WUBRG-then-C order,
// deduplicated. Empty means the ability reflects nothing this build can name.
func reflectedColours(h Host, c *Ctx, sa *cards.SA) []byte {
	objs, ok := reflectedObjects(h, c, sa)
	if !ok {
		return nil
	}
	produce := strings.EqualFold(strings.TrimSpace(sa.Params["ReflectProperty"]), "Produce")
	set := map[byte]bool{}
	for _, o := range objs {
		if produce {
			mp := o.Face().ManaProduction()
			for i := 0; i < 5; i++ {
				if mp.Colour[i] > 0 {
					set["WUBRG"[i]] = true
				}
			}
			if mp.Any {
				for i := 0; i < 5; i++ {
					set["WUBRG"[i]] = true
				}
			}
			continue
		}
		for _, r := range ColorsOf(o) {
			set[byte(r)] = true
		}
	}
	if strings.EqualFold(strings.TrimSpace(sa.Params["ColorOrType"]), "Type") {
		set['C'] = true
	}
	out := make([]byte, 0, len(set))
	for _, r := range ManaSymbols {
		if set[byte(r)] {
			out = append(out, byte(r))
		}
	}
	return out
}

// reflectedObjects resolves the Valid$ selector to the objects whose colours
// are reflected. ok=false is an unresolvable selector (fail closed upstream).
func reflectedObjects(h Host, c *Ctx, sa *cards.SA) ([]*state.Object, bool) {
	g := h.Game()
	sc := c.SpecContext(c.Controller)
	valid := strings.TrimSpace(sa.Params["Valid"])
	if valid == "" {
		valid = "Defined.Self"
	}
	if head, rest, ok := strings.Cut(valid, "."); ok && head == "Defined" {
		// "Defined.<selector>" with an optional trailing card spec
		// ("Defined.ValidGraveyard Card.YouOwn").
		sel := strings.TrimSpace(rest)
		spec := ""
		if i := strings.IndexByte(sel, ' '); i >= 0 {
			spec = strings.TrimSpace(sel[i+1:])
			sel = strings.TrimSpace(sel[:i])
		}
		switch sel {
		case "Self":
			if c.Source == 0 {
				return nil, false
			}
			o := g.Obj(c.Source)
			if o == nil {
				return nil, false
			}
			return []*state.Object{o}, true
		case "Imprinted", "ExiledWith":
			var out []*state.Object
			for _, q := range g.AliveFrom(0) {
				for _, id := range g.Zone(state.ZExile, q) {
					if o := g.Obj(id); o != nil && o.ExiledWith == c.Source {
						out = append(out, o)
					}
				}
			}
			return out, true
		case "ValidGraveyard":
			var out []*state.Object
			for _, q := range g.AliveFrom(0) {
				for _, id := range g.Zone(state.ZGraveyard, q) {
					if spec != "" && !MatchesSpecCtx(g, spec, id, sc) {
						continue
					}
					if o := g.Obj(id); o != nil {
						out = append(out, o)
					}
				}
			}
			return out, true
		case "Sacrificed":
			var out []*state.Object
			for _, t := range c.Remembered {
				if o := g.Obj(t.Obj); o != nil {
					out = append(out, o)
				}
			}
			return out, true
		case "Untapped":
			var out []*state.Object
			for _, q := range g.AliveFrom(0) {
				for _, id := range g.Zone(state.ZBattlefield, q) {
					if o := g.Obj(id); o != nil && o.Controller == c.Controller && !o.Tapped {
						out = append(out, o)
					}
				}
			}
			return out, true
		}
		return nil, false
	}
	var out []*state.Object
	for _, q := range g.AliveFrom(0) {
		for _, id := range g.Zone(state.ZBattlefield, q) {
			if MatchesSpecCtx(g, valid, id, sc) {
				if o := g.Obj(id); o != nil {
					out = append(out, o)
				}
			}
		}
	}
	return out, true
}
