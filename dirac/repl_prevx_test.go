package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// prevXScen is a scenario whose table carries a Chant of Vitu-Ghazi-shaped
// instant (a scoped prevention of all damage dealt by creatures, with the
// gain rider), a creature and a plain burn instant.
type prevXScen struct {
	*scen
	chant, bear, burn CardID
	eoc, scope, awe   CardID
	ria, mite, dbl    CardID
	emb               uint8
}

func newPrevXScen(t *testing.T) *prevXScen {
	t.Helper()
	return newPrevXTokScen(t, false)
}

// newPrevXTokScen is newPrevXScen with, when dbl, a token doubler in the
// table, and always with prevstate-04's token-rider cards: a vanilla 1/1
// Mite token and a creature whose ExtPreventNext carries every rider bit and
// names the Mite in B.
func newPrevXTokScen(t *testing.T, dbl bool) *prevXScen {
	t.Helper()
	var ids prevXScen
	sc, _ := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
		noPrevEmblemTweak(&ids.emb)(tbl, nil)
		in := kop.ExtInstr(kop.ExtPreventDamageScope)
		in.C = int16(kop.PrevScopeAll | kop.PrevScopeSrcCreature | kop.PrevScopeGain)
		add := func(d kop.CardDef) CardID {
			tbl.Cards = append(tbl.Cards, d)
			return CardID(len(tbl.Cards) - 1)
		}
		ids.chant = add(kop.CardDef{Name: "Test Chant", Types: kop.TInstant, Abilities: []kop.Ability{{Prog: []kop.Instr{in}}}})
		ids.bear = add(kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
		ids.burn = add(kop.CardDef{Name: "Test Burn", Types: kop.TInstant})
		aweIn := kop.ExtInstr(kop.ExtPreventNext)
		aweIn.C = int16(kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextGain)
		aweIn.Obj = kop.RefTarget0
		ids.awe = add(kop.CardDef{Name: "Test Awe", Types: kop.TInstant, Abilities: []kop.Ability{{Prog: []kop.Instr{aweIn}}}})
		eocIn := kop.ExtInstr(kop.ExtPreventDamageScope)
		eocIn.C = int16(kop.PrevScopePlayer | kop.PrevScopeCombat | kop.PrevScopeEndCombat)
		ids.eoc = add(kop.CardDef{Name: "Test EOC Scope", Types: kop.TInstant, Abilities: []kop.Ability{{Prog: []kop.Instr{eocIn}}}})
		scIn := kop.ExtInstr(kop.ExtPreventDamageScope)
		scIn.C = int16(kop.PrevScopePlayer | kop.PrevScopeCombat)
		ids.scope = add(kop.CardDef{Name: "Test Scope", Types: kop.TInstant, Abilities: []kop.Ability{{Prog: []kop.Instr{scIn}}}})
		ids.mite = add(kop.CardDef{Name: "Test Mite", Types: kop.TCreature, Power: 1, Toughness: 1})
		riaIn := kop.ExtInstr(kop.ExtPreventNext)
		riaIn.C = int16(kop.PrevNextAll | kop.PrevNextFrom | kop.PrevNextCombat | kop.PrevNextPlayers | kop.PrevNextTokens | kop.PrevNextEndCombat)
		riaIn.B.N = int16(ids.mite)
		riaIn.Obj = kop.RefTarget0
		ids.ria = add(kop.CardDef{Name: "Test Ria Rider", Types: kop.TCreature, Power: 2, Toughness: 2,
			Abilities: []kop.Ability{{Prog: []kop.Instr{riaIn}}}})
		if dbl {
			ids.dbl = add(kop.CardDef{Name: "Test Tok Doubler", Types: kop.TEnchantment,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDoubleTokens}}}})
		}
	})
	ids.scen = sc
	return &ids
}

// resolveChant registers the Chant of seat 0 as its resolution does.
func (f *prevXScen) resolveChant() {
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.chant, 0), 0, f.chant
	f.s.prevScopeAdd(f.k, &f.k.t.Cards[f.chant].Abilities[0].Prog[0])
}

// resolveScopeCard registers the scoped-prevention instant id of seat 0 as its
// resolution does.
func (f *prevXScen) resolveScopeCard(id CardID) {
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(id, 0), 0, id
	f.s.prevScopeAdd(f.k, &f.k.t.Cards[id].Abilities[0].Prog[0])
}

// creatureSrc makes a creature of seat 1 the resolving source of the damage.
func (f *prevXScen) creatureSrc() {
	bear := f.perm(f.bear, 1)
	e := &f.s.res.e
	e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, bear, f.s.objs[bear].ver, f.bear, 1
}

// burnSrc makes a burn spell of seat 1 the resolving source of the damage.
func (f *prevXScen) burnSrc() {
	e := &f.s.res.e
	e.kind, e.obj, e.card, e.ctrl = seSpell, f.obj(f.burn, 1), f.burn, 1
}

// resolveAwe registers the Awe-Strike-shaped instant of seat 0 as its
// resolution does, with bear as its remembered target.
func (f *prevXScen) resolveAwe(bear Obj) {
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seSpell, f.obj(f.awe, 0), 0, f.awe
	e.tgt[0] = TCode(bear)
	f.s.prevNextAdd(f.k, &f.k.t.Cards[f.awe].Abilities[0].Prog[0])
}

// resolveRiaTok registers the token-rider shield (the Test Ria Rider card's
// ExtPreventNext) as its resolution does, keyed on bear as the remembered
// source, resolving from a battlefield Ria of seat p so the record finds the
// instruction (and its Mite) again from its object's card.
func (f *prevXScen) resolveRiaTok(bear Obj, p uint8) {
	e := &f.s.res.e
	e.kind, e.obj, e.ctrl, e.card = seAbility, f.perm(f.ria, p), p, f.ria
	e.tgt[0] = TCode(bear)
	f.s.prevNextAdd(f.k, &f.k.t.Cards[f.ria].Abilities[0].Prog[0])
}

// mitesOf counts the Mites of card c on the battlefield under seat p.
func (f *prevXScen) mitesOf(c CardID, p uint8) int {
	n := 0
	for o := Obj(0); o < Obj(f.s.nobj); o++ {
		ob := &f.s.objs[o]
		if ob.zone == kop.ZoneBattlefield && ob.card == c && f.s.perms[ob.slot].ctrl == p {
			n++
		}
	}
	return n
}

// bearSrc makes an already-entered bear the resolving source of the damage.
func (f *prevXScen) bearSrc(bear Obj) {
	e := &f.s.res.e
	e.kind, e.obj, e.srcVer, e.card, e.ctrl = seAbility, bear, f.s.objs[bear].ver, f.bear, 1
}

// TestPrevXGainScope: Chant's scoped record prevents the damage creatures
// would deal and its controller gains that much; a non-creature source is
// dealt; under a can't-prevent static, or beside a second scoped record, the
// kernel stalls StallPrevRider; DebugNoPrevRider keeps the prevention and
// drops the gain.
func TestPrevXGainScope(t *testing.T) {
	t.Run("creature source", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant()
		if x := s.g.prevX[0]; x != pxRidGain {
			t.Fatalf("prevX[0] = %#x, want %#x (a gain rider for seat 0)", x, pxRidGain)
		}
		if s.g.prevCol[0].sc&(kop.PrevScopeGain) != 0 {
			t.Fatalf("the instruction-only bit reached the record's sc %#x", s.g.prevCol[0].sc)
		}
		f.creatureSrc()
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 0 {
			t.Fatalf("damageTo dealt %d, want 0 (prevented)", n)
		}
		if s.pl[1].life != life[1] || s.pl[0].life != life[0]+3 {
			t.Fatalf("life %d/%d from %d/%d: want the victim unharmed and the Chant's controller +3", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
		if s.StallReason() != 0 {
			t.Fatalf("stalled %s", StallName(s.StallReason()))
		}
	})
	t.Run("non-creature source", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant()
		f.burnSrc()
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 3 {
			t.Fatalf("damageTo dealt %d, want 3", n)
		}
		if s.pl[1].life != life[1]-3 || s.pl[0].life != life[0] {
			t.Fatalf("life %d/%d from %d/%d: want the victim -3 and no gain", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
	})
	t.Run("can't be prevented", func(t *testing.T) {
		f := newPrevXScen(t)
		f.resolveChant()
		f.creatureSrc()
		embDurPut(f.scen, 0, NoObj, f.emb)
		if !f.s.noPrev(f.k, false) {
			t.Fatal("precondition: the can't-prevent static is not in force")
		}
		f.s.damageTo(f.k, PlayerCode(1), 3)
		if got := f.s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
	})
	t.Run("second scoped record", func(t *testing.T) {
		f := newPrevXScen(t)
		f.resolveChant()
		f.resolveChant()
		if f.s.g.prevX[0] == 0 || f.s.g.prevX[1] == 0 {
			t.Fatal("precondition: two rider records")
		}
		f.creatureSrc()
		f.s.damageTo(f.k, PlayerCode(1), 3)
		if got := f.s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
	})
	t.Run("mutation", func(t *testing.T) {
		DebugNoPrevRider = true
		defer func() { DebugNoPrevRider = false }()
		f := newPrevXScen(t)
		s := f.s
		f.resolveChant()
		f.creatureSrc()
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 0 {
			t.Fatalf("damageTo dealt %d, want 0 (still prevented)", n)
		}
		if s.pl[0].life != life[0] || s.pl[1].life != life[1] {
			t.Fatalf("life %d/%d from %d/%d: want no gain", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
	})
}

// TestPrevXFromNext: the source-keyed PreventNext shield (kop.PrevNextFrom,
// Awe Strike) prevents the first damage event the remembered creature deals
// and its controller gains that much, the record is spent (the Effect exiles
// itself), a second event from the creature and damage TO the creature are
// dealt, the record is spent when the creature leaves the battlefield first
// (prevAllLeft, gorge's ExileOnMoved$ Battlefield), and under a
// can't-prevent static, or beside a second shield on the same event, the
// kernel stalls StallPrevRider.
func TestPrevXFromNext(t *testing.T) {
	t.Run("prevented and gained", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		bear := f.perm(f.bear, 1)
		f.resolveAwe(bear)
		if x := s.g.prevX[0]; x&(pxFrom|pxRidGain) != pxFrom|pxRidGain || x>>pxSeatShift != 0 {
			t.Fatalf("prevX[0] = %#x, want a seat-0 gain rider keyed on the source", x)
		}
		if r := s.g.prevCol[0]; r.ctl != TCode(bear) || r.col != 255 || r.sc&prevNextSc == 0 {
			t.Fatalf("record %+v, want ctl the bear, the unbounded pool and the shield bit", r)
		}
		f.bearSrc(bear)
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 0 {
			t.Fatalf("damageTo dealt %d, want 0 (prevented)", n)
		}
		if s.pl[1].life != life[1] || s.pl[0].life != life[0]+3 {
			t.Fatalf("life %d/%d from %d/%d: want the victim unharmed and the caster +3", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
		if s.g.prevCol[0].col != 0 {
			t.Fatalf("the record was not spent: col %d", s.g.prevCol[0].col)
		}
		if s.StallReason() != 0 {
			t.Fatalf("stalled %s", StallName(s.StallReason()))
		}
	})
	t.Run("second event dealt", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		bear := f.perm(f.bear, 1)
		f.resolveAwe(bear)
		f.bearSrc(bear)
		life := [2]int16{s.pl[0].life, s.pl[1].life}
		if n := s.damageTo(f.k, PlayerCode(1), 3); n != 0 {
			t.Fatalf("the first event dealt %d, want 0 (prevented)", n)
		}
		if n := s.damageTo(f.k, PlayerCode(1), 2); n != 2 {
			t.Fatalf("the second event dealt %d, want 2", n)
		}
		if s.pl[1].life != life[1]-2 || s.pl[0].life != life[0]+3 {
			t.Fatalf("life %d/%d from %d/%d: want the second event dealt and no second gain", s.pl[0].life, s.pl[1].life, life[0], life[1])
		}
	})
	t.Run("damage to the creature dealt", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		bear := f.perm(f.bear, 1)
		f.resolveAwe(bear)
		f.burnSrc()
		if n := s.damageTo(f.k, TCode(bear), 3); n != 3 {
			t.Fatalf("damage dealt %d, want 3 (the shield never matches its recipient)", n)
		}
		if s.g.prevCol[0].col != 255 {
			t.Fatalf("the record moved: col %d, want the unbounded pool", s.g.prevCol[0].col)
		}
	})
	t.Run("spent when the creature leaves", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		bear := f.perm(f.bear, 1)
		f.resolveAwe(bear)
		if !s.pnxLive() {
			t.Fatal("precondition: the shield is live")
		}
		s.removePerm(f.k, f.slot(bear), kop.ZoneGraveyard, false)
		if s.g.prevCol[0].col != 0 {
			t.Fatalf("the record survived the creature leaving: col %d, want spent (ExileOnMoved$)", s.g.prevCol[0].col)
		}
	})
	t.Run("can't be prevented", func(t *testing.T) {
		f := newPrevXScen(t)
		bear := f.perm(f.bear, 1)
		f.resolveAwe(bear)
		f.bearSrc(bear)
		embDurPut(f.scen, 0, NoObj, f.emb)
		if !f.s.noPrev(f.k, false) {
			t.Fatal("precondition: the can't-prevent static is not in force")
		}
		f.s.damageTo(f.k, PlayerCode(1), 3)
		if got := f.s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
	})
	t.Run("second shield", func(t *testing.T) {
		f := newPrevXScen(t)
		bear := f.perm(f.bear, 1)
		f.resolveAwe(bear)
		f.resolveAwe(bear)
		if f.s.g.prevCol[1].col == 0 || f.s.g.prevCol[1].ctl != TCode(bear) {
			t.Fatal("precondition: a second shield on the creature")
		}
		f.bearSrc(bear)
		f.s.damageTo(f.k, PlayerCode(1), 3)
		if got := f.s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
	})
}

// TestPrevXEndCombat: the scoped record whose instruction carries
// kop.PrevScopeEndCombat (Suppressor Skyguard's Duration$ UntilEndOfCombat
// prevention) is dropped on the transition out of the end-of-combat step,
// the record beside it survives, the freed slot is reused by the next
// resolution, and DebugNoPrevEndCombat keeps the record.
func TestPrevXEndCombat(t *testing.T) {
	t.Run("dropped at end of combat", func(t *testing.T) {
		f := newPrevXScen(t)
		s := f.s
		f.resolveScopeCard(f.eoc)
		f.resolveScopeCard(f.scope)
		if s.g.prevX[0]&pxEOC == 0 || s.g.prevX[1] != 0 {
			t.Fatalf("precondition: prevX %#x/%#x, want the end-of-combat rider in slot 0 alone", s.g.prevX[0], s.g.prevX[1])
		}
		keep := s.g.prevCol[1]
		s.step = kop.StepEndCombat
		s.advance(f.k)
		if s.g.prevCol[0] != (prevRec{}) || s.g.prevX[0] != 0 {
			t.Fatalf("the end-of-combat record survived: %+v prevX %#x", s.g.prevCol[0], s.g.prevX[0])
		}
		if s.g.prevCol[1] != keep {
			t.Fatalf("the plain record moved: %+v from %+v", s.g.prevCol[1], keep)
		}
		f.resolveScopeCard(f.eoc)
		if s.g.prevCol[0].sc == 0 || s.g.prevX[0]&pxEOC == 0 {
			t.Fatalf("the freed slot was not reused: %+v prevX %#x", s.g.prevCol[0], s.g.prevX[0])
		}
	})
	t.Run("mutation", func(t *testing.T) {
		DebugNoPrevEndCombat = true
		defer func() { DebugNoPrevEndCombat = false }()
		f := newPrevXScen(t)
		f.resolveScopeCard(f.eoc)
		f.s.step = kop.StepEndCombat
		f.s.advance(f.k)
		if f.s.g.prevCol[0].sc == 0 || f.s.g.prevX[0]&pxEOC == 0 {
			t.Fatalf("the record was dropped under DebugNoPrevEndCombat: %+v prevX %#x", f.s.g.prevCol[0], f.s.g.prevX[0])
		}
	})
}

// TestPrevXFromTokens: the token-rider shield (prevstate-04, Ria Ivor's
// shape: kop.PrevNextAll|From|Combat|Players|Tokens|EndCombat, B the Mite)
// prevents the first combat damage the remembered creature deals to a player
// and its controller mints that many Mites, damage to a blocker is dealt,
// the record ends with the combat (prevEndCombat), and a token doubler in
// the table stalls StallPrevRider.
func TestPrevXFromTokens(t *testing.T) {
	t.Run("prevented and minted", func(t *testing.T) {
		f := newPrevXTokScen(t, false)
		s := f.s
		bear := f.perm(f.bear, 0)
		f.resolveRiaTok(bear, 1)
		if x := s.g.prevX[0]; x != pxFrom|pxRidTok|pxEOC|pxPlayers|1<<pxSeatShift {
			t.Fatalf("prevX[0] = %#x, want the token rider, end-of-combat and player-recipient bits for seat 1", x)
		}
		if r := s.g.prevCol[0]; r.ctl != TCode(bear) || r.col != 255 || r.sc&prevNextSc == 0 {
			t.Fatalf("record %+v, want ctl the bear, the unbounded pool and the shield bit", r)
		}
		life1 := s.pl[1].life
		f.combat(t, []Obj{bear}, nil) // the bear attacks seat 1 unblocked
		if s.pl[1].life != life1 {
			t.Fatalf("life %d from %d: want the bear's 2 damage prevented", s.pl[1].life, life1)
		}
		if n := f.mitesOf(f.mite, 1); n != 2 {
			t.Fatalf("%d Mites under seat 1, want 2 (the damage prevented)", n)
		}
		if s.g.prevCol[0].col != 0 {
			t.Fatalf("the record was not spent: col %d", s.g.prevCol[0].col)
		}
		if s.StallReason() != 0 {
			t.Fatalf("stalled %s", StallName(s.StallReason()))
		}
	})
	t.Run("damage to a blocker dealt", func(t *testing.T) {
		f := newPrevXTokScen(t, false)
		s := f.s
		bear := f.perm(f.bear, 0)
		blocker := f.perm(f.bear, 1)
		f.resolveRiaTok(bear, 1)
		life1 := s.pl[1].life
		f.combat(t, []Obj{bear}, [][2]Obj{{blocker, bear}})
		if f.onBF(blocker) || f.onBF(bear) {
			t.Fatalf("blocker alive=%v bear alive=%v: want the bear's damage dealt to the blocker", f.onBF(blocker), f.onBF(bear))
		}
		if s.pl[1].life != life1 {
			t.Fatalf("life %d from %d: want no player damage", s.pl[1].life, life1)
		}
		if n := f.mitesOf(f.mite, 1); n != 0 {
			t.Fatalf("%d Mites under seat 1, want 0 (the record never matched a creature recipient)", n)
		}
	})
	t.Run("gone at end of combat", func(t *testing.T) {
		f := newPrevXTokScen(t, false)
		s := f.s
		bear := f.perm(f.bear, 0)
		f.resolveRiaTok(bear, 1)
		if s.g.prevCol[0].col == 0 || s.g.prevX[0]&pxEOC == 0 {
			t.Fatal("precondition: the token-rider record is live")
		}
		s.step = kop.StepEndCombat
		s.advance(f.k)
		if s.g.prevCol[0] != (prevRec{}) || s.g.prevX[0] != 0 {
			t.Fatalf("the record survived the end of combat: %+v prevX %#x", s.g.prevCol[0], s.g.prevX[0])
		}
		f.resolveRiaTok(bear, 1)
		if s.g.prevCol[0].col == 0 || s.g.prevX[0]&pxRidTok == 0 {
			t.Fatalf("the freed slot was not reused: %+v prevX %#x", s.g.prevCol[0], s.g.prevX[0])
		}
	})
	t.Run("token doubler stalls", func(t *testing.T) {
		f := newPrevXTokScen(t, true)
		s := f.s
		if f.k.gen.anyRepl&rbDblTok == 0 {
			t.Fatal("precondition: the token doubler is not in the gen bits")
		}
		bear := f.perm(f.bear, 0)
		f.resolveRiaTok(bear, 1)
		life1 := s.pl[1].life
		f.combat(t, []Obj{bear}, nil)
		if got := s.StallReason(); got != StallPrevRider {
			t.Fatalf("stall = %s, want %s", StallName(got), StallName(StallPrevRider))
		}
		if s.pl[1].life != life1-2 {
			t.Fatalf("life %d from %d: want the bear's 2 damage dealt (the shield never applied)", s.pl[1].life, life1)
		}
		if n := f.mitesOf(f.mite, 1); n != 0 {
			t.Fatalf("%d Mites under seat 1, want 0 (the mint never ran)", n)
		}
	})
}
