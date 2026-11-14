package adapter

import (
	"math/bits"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// objFacts fills the characteristics the view derives for kernel object i. Nothing
// reads the kernel later, so they are answered now.
func (g *Game) objFacts(i canon.Ref, id state.ObjID) {
	po := &g.kp.Objs[i]
	f := &g.board.facts[id-1]
	f.retyped, f.types = false, f.types[:0]
	f.keywords = f.keywords[:0]
	ob := dirac.Obj(i)
	f.name, f.effective = "", ""
	if face := g.face(po.Card); face != nil {
		f.name = face.Name
		f.power, f.tough = int32(face.Power()), int32(face.Toughness())
		f.keywords = append(f.keywords, face.Keywords...)
	}
	if po.Zone != kop.ZoneBattlefield {
		if p, t, ok := g.s.FeatCDAPT(ob); ok {
			f.power, f.tough = int32(p), int32(t)
		}
		granted, rep := g.s.FeatGrantedOffRep(ob)
		f.keywords = appendGranted(f.keywords, granted, &rep)
		if po.Zone == kop.ZoneHand {
			f.effective = g.handCost(ob, po)
		}
		return
	}
	f.power, f.tough = int32(po.Power), int32(po.Tough)
	granted, rep, printed := g.s.FeatGrantedRep(ob)
	if !printed {
		f.keywords = f.keywords[:0]
	}
	f.keywords = appendGranted(f.keywords, granted, &rep)
	g.retype(i, f)
}

// appendGranted lists granted keywords, each once per grant.
func appendGranted(out []string, granted kop.KwMask, rep *dirac.FeatKwReps) []string {
	for m := uint32(granted); m != 0; m &= m - 1 {
		b := bits.TrailingZeros32(m)
		if n := feat.KeywordNames[b]; n != "" {
			for k := 0; k <= int(rep[b]); k++ {
				out = append(out, n)
			}
		}
	}
	return out
}

// handCost is a hand card's cost when a static changed it: no X and no alternative
// cost, the only cast the view prices.
func (g *Game) handCost(o dirac.Obj, po *dirac.ProjObj) string {
	c, changed := g.s.FeatHandCost(po.Owner, o)
	if !changed || c.X != 0 {
		return ""
	}
	if f := g.face(po.Card); f != nil {
		if sa := f.SpellAbility(); sa != nil && (sa.ParamStr(cards.PKAlternativeCost) != "" || sa.ParamStr(cards.PKCost) != "") {
			return ""
		}
	}
	out := cost.Cost{Generic: int32(c.Generic)}
	for k, n := range c.Pips {
		out.Colored[k] = int32(n)
	}
	for k, m := range c.Hyb {
		if m == 0 {
			break
		}
		a, b := hybridFaces(m, c.HybOrd&(1<<k) != 0)
		out.Hybrid = append(out.Hybrid, cost.ManaPair{A: a, B: b})
	}
	return cost.FormatCost(out)
}

// hybridFaces spells a hybrid pip's two colours in printed order.
func hybridFaces(m kop.ColorMask, swapped bool) (a, b byte) {
	const faces = "WUBRG"
	lo := bits.TrailingZeros8(uint8(m))
	hi := 7 - bits.LeadingZeros8(uint8(m))
	if swapped {
		lo, hi = hi, lo
	}
	return faces[lo], faces[hi]
}

// retype records a type-changing effect's derived type line: the printed words still
// true, then what the effect added.
func (g *Game) retype(i canon.Ref, f *objFacts) {
	types, subs, all, setSub, ok := g.s.FeatViewTypes(dirac.Obj(i))
	if !ok {
		return
	}
	face := g.face(g.kp.Objs[i].Card)
	if face == nil {
		return
	}
	cardTypes := [...]struct {
		mask kop.TypeMask
		word string
	}{{kop.TArtifact, "Artifact"}, {kop.TBattle, "Battle"}, {kop.TCreature, "Creature"}, {kop.TEnchantment, "Enchantment"},
		{kop.TInstant, "Instant"}, {kop.TLand, "Land"}, {kop.TPlaneswalker, "Planeswalker"}, {kop.TSorcery, "Sorcery"}}
	isCard := func(w string) bool {
		for _, c := range cardTypes {
			if c.word == w {
				return true
			}
		}
		return false
	}
	has := func(w string) bool {
		switch w {
		case "Legendary", "Basic", "Snow", "World":
			return true
		}
		for _, c := range cardTypes {
			if c.word == w {
				return types&c.mask != 0
			}
		}
		names := g.tbl.Subtypes
		for b, n := range names {
			if n == w {
				return all || subs.Has(b)
			}
		}
		return !setSub
	}
	for _, w := range face.Types {
		if has(w) {
			f.types = append(f.types, w)
		}
	}
	for _, c := range cardTypes {
		if types&c.mask != 0 && !contains(f.types, c.word) {
			f.types = append(f.types, c.word)
		}
	}
	if all {
		for _, w := range effects.CreatureTypeWordList() {
			if !contains(f.types, w) {
				f.types = append(f.types, w)
			}
		}
	}
	for b, n := range g.tbl.Subtypes {
		if !all && subs.Has(b) && !isCard(n) && !contains(f.types, n) {
			f.types = append(f.types, n)
		}
	}
	f.retyped = !sameWords(f.types, face.Types)
}

func contains(xs []string, w string) bool { return slices.Contains(xs, w) }

func sameWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for _, w := range a {
		if !slices.Contains(b, w) {
			return false
		}
	}
	return true
}

// ViewTypes is the derived type line, when an effect changed it.
func (b *board) ViewTypes(id state.ObjID) ([]string, bool) {
	f := b.fact(id)
	return f.types, f.retyped
}

func (b *board) fact(id state.ObjID) *objFacts {
	if id == 0 || int(id) > len(b.facts) {
		return &objFacts{}
	}
	return &b.facts[id-1]
}

func (b *board) Power(id state.ObjID) int32 { return b.fact(id).power }

func (b *board) Toughness(id state.ObjID) int32 { return b.fact(id).tough }

func (b *board) Keywords(id state.ObjID) []string { return b.fact(id).keywords }

// HasKeyword reports whether id has keyword kw now.
func (g *Game) HasKeyword(id state.ObjID, kw string) bool {
	return slices.ContainsFunc(g.board.fact(id).keywords, func(k string) bool { return strings.EqualFold(k, kw) })
}

// ObservedAbilities lists id's printed non-mana activated abilities. The kernel keeps
// no granted activated ability apart from its printed ones.
func (g *Game) ObservedAbilities(_ state.PlayerID, id state.ObjID) []decision.Option {
	o := g.board.g.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	var out []decision.Option
	for i, ab := range o.Face().Abilities {
		if ab != nil && ab.Kind == "AB" && !cards.IsManaAbilitySA(ab) {
			out = append(out, decision.Option{Index: len(out), Kind: "ability", Obj: id, Ability: i, Label: o.Face().Name + ": " + ab.ParamStr(cards.PKSpellDescription)})
		}
	}
	return out
}

// Facts is id's characteristics after continuous effects.
func (g *Game) Facts(id state.ObjID) game.Facts {
	f := g.board.fact(id)
	types := f.types
	if !f.retyped {
		if o := g.board.g.Obj(id); o != nil && o.Face() != nil {
			types = o.Face().Types
		}
	}
	return game.Facts{Name: f.name, Types: types, Colors: g.colors(id), Keywords: f.keywords, Power: f.power, Toughness: f.tough}
}

// colors is id's colours as WUBRG letters.
func (g *Game) colors(id state.ObjID) string {
	r := g.RefOf(id)
	if int(r) >= int(g.kp.NObj) {
		return ""
	}
	mask := uint8(g.tbl.Cards[g.kp.Objs[r].Card].Colors)
	if g.kp.Objs[r].Zone == kop.ZoneBattlefield {
		mask = g.s.FeatColorsOf(dirac.Obj(r))
	}
	var out []byte
	for i := range 5 {
		if mask&(uint8(kop.ColW)<<i) != 0 {
			out = append(out, canon.ManaSyms[i])
		}
	}
	return string(out)
}

// HostTriggerAmount has no kernel read: a served observation leaves the amount out.
func (g *Game) HostTriggerAmount(state.ObjID) (int32, bool) { return 0, false }

// ZoneMoves counts id's zone changes (CR 400.7).
func (g *Game) ZoneMoves(id state.ObjID) uint32 {
	r := g.RefOf(id)
	if int(r) >= int(g.kp.NObj) {
		return 0
	}
	return uint32(g.s.ObjVer(dirac.Obj(r)))
}

// ViewCharacteristics answers the view's four derived facts at once.
func (b *board) ViewCharacteristics(id state.ObjID) (string, bool, []string, int32, int32) {
	f := b.fact(id)
	return f.name, false, f.keywords, f.power, f.tough
}

// SpellEffectiveCost is a hand card's cost as its statics leave it, "" when unchanged.
func (b *board) SpellEffectiveCost(_ state.PlayerID, id state.ObjID) string {
	return b.fact(id).effective
}

// LandDropOpen reports whether a seat has a land drop left this turn.
func (b *board) LandDropOpen(p state.PlayerID) bool { return b.landOpen[p&1] }

func (b *board) PendingTriggers() []state.PendingTrigger { return b.pending }

// StackOptional: the kernel asks a "you may" trigger's controller.
func (b *board) StackOptional(id state.ObjID) (bool, state.PlayerID) {
	f := b.fact(id)
	return f.optional, f.decider
}

// AvailableMana is left at zero: no client reads it.
func (b *board) AvailableMana(state.PlayerID) state.Mana { return state.Mana{} }

// AbilityCosts is each non-mana activated ability's printed cost.
func (b *board) AbilityCosts(_ state.PlayerID, id state.ObjID) []string {
	o := b.g.Obj(id)
	if o == nil || o.Face() == nil {
		return nil
	}
	var out []string
	for _, ab := range o.Face().Abilities {
		if ab.Kind == "AB" && !cards.IsManaAbilitySA(ab) {
			out = append(out, cost.FormatCost(cost.ParseCost(ab.ParamStr(cards.PKCost))))
		}
	}
	return out
}

func (b *board) MayLookAtLibraryTop(state.PlayerID) bool { return false }

func (b *board) LibraryTopRevealed(state.PlayerID) bool { return false }

// PotentialActions is every play offered at the seat's own priority decision.
func (b *board) PotentialActions(p state.PlayerID) []decision.PotentialAction {
	d := b.game.pend
	if d == nil || d.Kind != decision.KPriority || d.Player != p {
		return nil
	}
	var out []decision.PotentialAction
	for i := range d.Options {
		o := &d.Options[i]
		if o.Kind != "pass" && o.Kind != "concede" {
			out = append(out, decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode, Label: b.playLabel(o)})
		}
	}
	for i := range d.PaymentActions {
		a := &d.PaymentActions[i]
		mode := a.Cast.CastMode()
		out = append(out, decision.PotentialAction{Kind: "cast", Obj: a.Cast.Object, Label: b.castLabel(a.Cast.Object, a.Label, mode, 0), Mode: mode})
	}
	return out
}

// castLabel words a cast as gorge's priority walk does: the face cast, and what it
// is cast for unless that is the card's own cost or the only permission to cast it.
func (b *board) castLabel(obj state.ObjID, name, mode string, alt int) string {
	switch mode {
	case "", "mayplay":
		switch alt {
		case 0:
			if text := b.mayPlayText(obj, mode); text != "" {
				return "Cast " + name + " (" + text + ")"
			}
			return "Cast " + name
		case 1:
			return "Cast " + name + " (alternative cost)"
		}
		return "Cast " + name + " (alternative cost " + strconv.Itoa(alt) + ")"
	case "modal_spell", "adventure_alt", "split_alt", "room_alt", "omen_alt":
		return "Cast " + b.otherFace(obj, name)
	case "aftermath":
		return "Cast " + b.otherFace(obj, name) + " (aftermath)"
	case "fuse":
		return "Cast " + name + " // " + b.otherFace(obj, name) + " (fused)"
	case "foretell":
		return "Foretell " + name
	case "plot":
		return "Plot " + name
	}
	return "Cast " + name + " (" + castWord(mode) + ")"
}

// mayPlayText is the name a typed permission gives a may-play cast (MayPlayText$).
func (b *board) mayPlayText(obj state.ObjID, mode string) string {
	g := b.game
	if mode != "mayplay" {
		return ""
	}
	c, nth, ok := g.s.FeatMayPlayTyped(g.d.Player, dirac.Obj(g.RefOf(obj)))
	if !ok {
		return ""
	}
	f := g.face(c)
	if f == nil {
		return ""
	}
	for i := range f.Statics {
		if f.Statics[i].ParamStr(cards.PKMayPlay) == "" {
			continue
		}
		if nth == 0 {
			return f.Statics[i].ParamStr(cards.PKMayPlayText)
		}
		nth--
	}
	return ""
}

// castWord is the word gorge's priority walk has for a cast mode.
func castWord(mode string) string {
	switch mode {
	case "adventure_recast":
		return "from adventure zone"
	case "warp_recast":
		return "from warp exile"
	case "morphed", "megamorphed", "disguised":
		return "face down"
	case "plot_cast":
		return "plotted"
	case "prepared_copy":
		return "prepared"
	case "airbend_cast":
		return "airbent"
	}
	return mode
}

// otherFace names the second face of obj's card.
func (b *board) otherFace(obj state.ObjID, name string) string {
	o := b.g.Obj(obj)
	if o == nil || o.Card == nil || len(o.Card.Faces) < 2 {
		return name
	}
	return o.Card.Faces[1].Name
}

// playLabel words a play the way a client lists what it could do.
func (b *board) playLabel(o *decision.Option) string {
	switch o.Kind {
	case "cast":
		return b.castLabel(o.Obj, o.Label, o.Mode, o.AltCostIndex)
	case "play_land":
		return "Play " + o.Label
	case "ability":
		if o.GrantSource != 0 {
			if sa := cards.ResolveSVar(b.g.Obj(o.GrantSource).Face().SVars, o.SVar); sa != nil {
				return o.Label + ": " + sa.ParamStr(cards.PKSpellDescription)
			}
		}
		if obj := b.g.Obj(o.Obj); obj != nil && obj.Face() != nil && o.Ability < len(obj.Face().Abilities) {
			if d := obj.Face().Abilities[o.Ability].ParamStr(cards.PKSpellDescription); d != "" {
				return o.Label + ": " + d
			}
		}
	}
	return o.Label
}

// OwnDeck is the seat's deck as it entered the game.
func (b *board) OwnDeck(p state.PlayerID) *deck.Manifest {
	if int(p) >= len(b.game.manifests) {
		return nil
	}
	m := b.game.manifests[p].Clone()
	return &m
}
