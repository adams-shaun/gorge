package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// permcopyTable is vanilla's table with a Copier (a {1} creature whose own
// cast trigger copies it, OpCopySpell C&2), a Face Copier (the same with a
// transforming layout), an Omen Copier (the same with an omen layout, face
// 1 an unaffordable instant) and a Watcher (gains 1 life whenever a token
// enters under its controller's control).
func permcopyTable() (t *kop.Table, copier, faced, watcher, omen CardID) {
	tb := *v.Table
	tb.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tb.Exprs = make([]kop.Expr, 1) // a generated table: genPrep is on
	tb.Filters = append(tb.Filters, kop.Filter{Types: kop.TCreature, Owner: kop.CtrlYou})
	yours := kop.FilterID(len(tb.Filters) - 1)
	cp := func(name string, mode uint8) kop.CardDef {
		return kop.CardDef{Name: name, Cost: kop.ManaCost{Generic: 1}, Types: kop.TCreature, Power: 1, Toughness: 1, FaceMode: mode,
			Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigSpellCast, Flags: kop.TfSelf},
				Prog: []kop.Instr{{Op: kop.OpCopySpell, Obj: kop.RefTriggerObj, A: kop.Const(1), C: 2}}}}}
	}
	copier, faced = CardID(len(tb.Cards)), CardID(len(tb.Cards)+1)
	omen, omenFace := CardID(len(tb.Cards)+2), CardID(len(tb.Cards)+3)
	watcher = CardID(len(tb.Cards) + 4)
	tb.Cards = append(tb.Cards, cp("Test Copier", kop.FaceNone), cp("Test Face Copier", kop.FaceTransform),
		cp("Test Omen Copier", kop.FaceOmen),
		kop.CardDef{Name: "Test Omen Copier's Omen", Cost: kop.ManaCost{Generic: 99}, Types: kop.TInstant,
			Face: 1, FaceMode: kop.FaceOmen})
	for i, f := range []CardID{omen, omenFace} {
		tb.Cards[f].Faces = []CardID{omen, omenFace}
		tb.Cards[f].Face = uint8(i)
	}
	tb.Cards = append(tb.Cards, kop.CardDef{Name: "Test Token Watcher", Types: kop.TArtifact, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
		Trig: kop.Trigger{Event: kop.TrigTokenCreated, Filter: yours},
		Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}})
	return &tb, copier, faced, watcher, omen
}

func permcopyCast(t *testing.T, faced bool) (sc *scen, orig Obj, copier CardID, life int) {
	t.Helper()
	tb, c, f, w, _ := permcopyTable()
	if faced {
		c = f
	}
	return permcopyCastCard(t, tb, c, w)
}

// permcopyOmenCast casts the omen-layout copier (face 0, the creature).
func permcopyOmenCast(t *testing.T) (sc *scen, orig Obj, copier CardID, life int) {
	t.Helper()
	tb, _, _, w, c := permcopyTable()
	return permcopyCastCard(t, tb, c, w)
}

func permcopyCastCard(t *testing.T, tb *kop.Table, c, w CardID) (sc *scen, orig Obj, copier CardID, life int) {
	t.Helper()
	sc = newScenTable(t, tb, Config{NoElide: true})
	if !sc.k.gen.anyPermCopy {
		t.Fatal("genPrep.anyPermCopy not set")
	}
	sc.lands(0, v.Mountain, 1)
	sc.perm(w, 0)
	life = int(sc.s.pl[0].life)
	orig = sc.hand(c, 0)
	sc.do(t, AkCast, TCode(orig), scAny)
	return sc, orig, c, life
}

// TestPermcopyEntersAsToken: the copy of a permanent spell resolves onto the
// battlefield as a token, no longer a spell copy (CR 111.13, 608.3f).
func TestPermcopyEntersAsToken(t *testing.T) {
	sc, orig, c, _ := permcopyCast(t, false)
	sc.resolveAll(t)
	sl := sc.copiesOf(c)
	if len(sl) != 2 {
		t.Fatalf("%d permanents of the card, want the original and its copy", len(sl))
	}
	for _, s := range sl {
		o := sc.s.perms[s].obj
		of := sc.s.objs[o].of
		if o == orig {
			if of&(ofToken|ofCopy) != 0 {
				t.Fatalf("the original has of %#x", of)
			}
			if !sc.s.g.cast.has(s) {
				t.Fatal("the cast original reads as not cast")
			}
			continue
		}
		if of&ofToken == 0 || of&ofCopy != 0 {
			t.Fatalf("the resolved copy has of %#x, want ofToken without ofCopy", of)
		}
		if sc.s.isSpellCopy(o) {
			t.Fatal("the resolved copy is still a spell copy")
		}
		if sc.s.g.cast.has(s) {
			t.Fatal("the resolved copy reads as cast: a copy is never cast (CR 707.10)")
		}
	}
}

// TestPermcopyCeases: the token copy ceases off the battlefield (CR 704.5d).
func TestPermcopyCeases(t *testing.T) {
	sc, orig, c, _ := permcopyCast(t, false)
	sc.resolveAll(t)
	var cp Obj = NoObj
	for _, s := range sc.copiesOf(c) {
		if o := sc.s.perms[s].obj; o != orig {
			cp = o
		}
	}
	if cp == NoObj {
		t.Fatal("no copy on the battlefield")
	}
	sc.s.destroy(sc.k, sc.slot(cp))
	sc.s.ceaseTokensGen()
	if z := sc.s.objs[cp].zone; z == kop.ZoneGraveyard {
		t.Fatal("the token copy stayed in the graveyard")
	}
	if sc.s.pl[0].graveN != 0 {
		t.Fatalf("graveyard holds %d cards", sc.s.pl[0].graveN)
	}
}

// TestPermcopyNoTokenCreated: the flip is not a token creation (gorge emits
// no TokenCreate), so a token-created trigger does not fire (CR 707.10f).
func TestPermcopyNoTokenCreated(t *testing.T) {
	sc, _, c, life := permcopyCast(t, false)
	sc.resolveAll(t)
	if n := len(sc.copiesOf(c)); n != 2 {
		t.Fatalf("%d permanents, want 2", n)
	}
	if got := int(sc.s.pl[0].life); got != life {
		t.Fatalf("life %d -> %d: a token-created trigger fired for the copy", life, got)
	}
}

// TestPermcopyOmenCopyResolves: an omen card is one card (CR 720.2c), so the
// copy of its permanent spell is an ordinary copy (CR 707.10f), its token the
// front face and not a double-faced token (CR 707.10g): the game does not
// stall and no token-created trigger fires.
func TestPermcopyOmenCopyResolves(t *testing.T) {
	sc, orig, c, life := permcopyOmenCast(t)
	sc.resolveAll(t)
	if r := sc.s.StallReason(); r != 0 {
		t.Fatalf("stalled at %q", StallName(r))
	}
	sl := sc.copiesOf(c)
	if len(sl) != 2 {
		t.Fatalf("%d permanents of the card, want the original and its copy", len(sl))
	}
	for _, s := range sl {
		o := sc.s.perms[s].obj
		of := sc.s.objs[o].of
		if o == orig {
			if of&(ofToken|ofCopy) != 0 {
				t.Fatalf("the original has of %#x", of)
			}
			continue
		}
		if of&ofToken == 0 || of&ofCopy != 0 {
			t.Fatalf("the resolved copy has of %#x, want ofToken without ofCopy", of)
		}
	}
	if got := int(sc.s.pl[0].life); got != life {
		t.Fatalf("life %d -> %d: a token-created trigger fired for the copy", life, got)
	}
}

// TestPermcopyOmenCopyCeases: the omen card's copy-token ceases off the
// battlefield like any token copy (CR 704.5d, 707.10f).
func TestPermcopyOmenCopyCeases(t *testing.T) {
	sc, orig, c, _ := permcopyOmenCast(t)
	sc.resolveAll(t)
	if r := sc.s.StallReason(); r != 0 {
		t.Fatalf("stalled at %q", StallName(r))
	}
	var cp Obj = NoObj
	for _, s := range sc.copiesOf(c) {
		if o := sc.s.perms[s].obj; o != orig {
			cp = o
		}
	}
	if cp == NoObj {
		t.Fatal("no copy on the battlefield")
	}
	sc.s.destroy(sc.k, sc.slot(cp))
	sc.s.ceaseTokensGen()
	if z := sc.s.objs[cp].zone; z == kop.ZoneGraveyard {
		t.Fatal("the token copy stayed in the graveyard")
	}
	if sc.s.pl[0].graveN != 0 {
		t.Fatalf("graveyard holds %d cards", sc.s.pl[0].graveN)
	}
}

// TestPermcopyFaceStall: a permanent-spell copy of a two-faced card stalls
// loud (CR 707.10g is not modelled).
func TestPermcopyFaceStall(t *testing.T) {
	sc, _, _, _ := permcopyCast(t, true)
	for i := 0; i < 20 && sc.s.StallReason() == 0; i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			break
		}
		sc.s.Apply(d.Options[0])
	}
	if got := sc.s.StallReason(); got != StallCopyFace {
		t.Fatalf("stall reason %q, want copyface", StallName(got))
	}
}
