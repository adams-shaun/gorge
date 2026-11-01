package adapter

import (
	"math/bits"
	"slices"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// objFacts fills the characteristics the view derives for kernel object i. Nothing
// reads the kernel later, so they are answered now.
func (g *Game) objFacts(i uint8, id state.ObjID) {
	po := &g.kp.Objs[i]
	f := &g.board.facts[id-1]
	f.retyped, f.types = false, f.types[:0]
	f.keywords = f.keywords[:0]
	ob := dirac.Obj(i)
	if face := g.face(po.Card); face != nil {
		f.power, f.tough = int32(face.Power()), int32(face.Toughness())
		f.keywords = append(f.keywords, face.Keywords...)
	}
	if po.Zone != kop.ZoneBattlefield {
		if p, t, ok := g.s.FeatCDAPT(ob); ok {
			f.power, f.tough = int32(p), int32(t)
		}
		dp, dt, _, _ := g.s.FeatStaticPT(ob)
		f.power += int32(dp)
		f.tough += int32(dt)
		return
	}
	f.power, f.tough = int32(po.Power), int32(po.Tough)
	granted, rep, printed := g.s.FeatGrantedRep(ob)
	if !printed {
		f.keywords = f.keywords[:0]
	}
	for m := uint32(granted); m != 0; m &= m - 1 {
		b := bits.TrailingZeros32(m)
		if n := feat.KeywordNames[b]; n != "" {
			for k := 0; k <= int(rep[b]); k++ {
				f.keywords = append(f.keywords, n)
			}
		}
	}
	g.retype(i, f)
}

// retype records a type-changing effect's derived type line: the printed words still
// true, then what the effect added.
func (g *Game) retype(i uint8, f *objFacts) {
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
	for b, n := range g.tbl.Subtypes {
		if subs.Has(b) && !isCard(n) && !contains(f.types, n) {
			f.types = append(f.types, n)
		}
	}
	if all {
		for _, w := range effects.CreatureTypeWordList() {
			if !contains(f.types, w) {
				f.types = append(f.types, w)
			}
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

// ViewCharacteristics answers the view's four derived facts at once.
func (b *board) ViewCharacteristics(id state.ObjID) (string, bool, []string, int32, int32) {
	f := b.fact(id)
	return "", false, f.keywords, f.power, f.tough
}

// LandDropOpen reports whether a seat has a land drop left this turn.
func (b *board) LandDropOpen(p state.PlayerID) bool { return b.landOpen[p&1] }

func (b *board) PendingTriggers() []state.PendingTrigger { return nil }

func (b *board) StackOptional(state.ObjID) (bool, state.PlayerID) { return false, 0 }

// AvailableMana is left at zero: no client reads it.
func (b *board) AvailableMana(state.PlayerID) state.Mana { return state.Mana{} }

// AbilityCosts is left empty: no client reads it.
func (b *board) AbilityCosts(state.PlayerID, state.ObjID) []string { return nil }

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
			out = append(out, decision.PotentialAction{Kind: o.Kind, Obj: o.Obj, Ability: o.Ability, Mode: o.Mode, Label: o.Label})
		}
	}
	for i := range d.PaymentActions {
		a := &d.PaymentActions[i]
		out = append(out, decision.PotentialAction{Kind: "cast", Obj: a.Cast.Object, Label: a.Label})
	}
	return out
}

// OwnDeck is the seat's deck as it entered the game.
func (b *board) OwnDeck(p state.PlayerID) *deck.Manifest {
	if int(p) >= len(b.game.manifests) {
		return nil
	}
	m := b.game.manifests[p].Clone()
	return &m
}
