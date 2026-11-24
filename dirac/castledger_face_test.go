package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// castLedgerPool is the test's pool table: the named kernel pool (a set or a
// scenario pool such as adventurecard or omen) with the Instant,Sorcery
// filter these tests match and an ExThisTurnCastSpec expression that turns
// the cast ring on. Both are test fixtures; the pool itself is the table
// kgen built.
func castLedgerPool(t *testing.T, pool string) (cardset.Set, *kop.Table, kop.FilterID) {
	t.Helper()
	set := cardsettest.Set(t, pool)
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl.Exprs = append(append([]kop.Expr(nil), tbl.Exprs...), kop.Expr{Kind: kop.ExThisTurnCastSpec})
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TInstant | kop.TSorcery})
	return set, tbl, kop.FilterID(len(tbl.Filters) - 1)
}

// TestCastLedgerReadsTheCastFace: a push of an Adventure (CR 715.3b) or Omen
// (CR 720.3) half is counted as that half by the type-filtered ledger readers
// after the card has left the stack and shows its normal face again (CR
// 715.4, 720.4): gorge castsThisTurnMatching (ledgerCastFace), Forge's
// cast-time ThisTurnCast copies, XMage's SpellsCastWatcher. The
// ActivatorThisTurnCast$ EQ1 over Instant,Sorcery reader (castPushesMatching)
// must see the resolved half as the turn's first instant or sorcery; so must
// a Count$ThisTurnCast_<spec> head.
func TestCastLedgerReadsTheCastFace(t *testing.T) {
	for _, tc := range []struct {
		pool, front string
		mode        uint8
		bit         uint8
		rest        kop.Zone
	}{
		{"adventurecard", "Bramble Familiar", kop.FaceAdventure, castAdventurePush, kop.ZoneExile},
		{"omen", "Marang River Regent", kop.FaceOmen, castOmenPush, kop.ZoneLibrary},
	} {
		t.Run(tc.front, func(t *testing.T) {
			set, tbl, instOrSorc := castLedgerPool(t, tc.pool)
			i, ok := set.Lookup(tc.front)
			if !ok {
				t.Fatalf("%s not in %s", tc.front, tc.pool)
			}
			front := CardID(set.PoolID(i))
			sc := newScenTable(t, tbl, Config{})
			if !sc.k.gen.castRingOn {
				t.Fatalf("the %s table does not turn the cast ring on", tc.pool)
			}
			face := sc.k.t.Cards[front].Faces[1]
			if cd := sc.k.t.Cards[face]; cd.FaceMode != tc.mode || cd.Face != 1 {
				t.Fatalf("%s's face 1 is mode %v face %d, want mode %v face 1", tc.front, cd.FaceMode, cd.Face, tc.mode)
			}
			o := sc.hand(face, 0)
			sc.kcastBuild(o, face, 0)
			ring := sc.s.castPushes(0)
			if len(ring) != 1 || ring[0].prov&tc.bit == 0 {
				t.Fatalf("push not marked as the spell half: %+v", ring)
			}
			// The spell resolved: the card rests off the stack as its front.
			sc.s.ctl = ctlPriority
			sc.s.objs[o].card, sc.s.objs[o].zone = front, tc.rest
			ctx := evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}
			// Precondition: the card as it is now is not an instant or sorcery.
			if sc.s.castRingMatch(sc.k, instOrSorc, o, ctx) {
				t.Fatalf("%s's front matches Instant,Sorcery; the test cannot tell the faces apart", tc.front)
			}
			if n := sc.s.castPushesMatching(sc.k, instOrSorc, 0, trigListener{obj: NoObj, sl: NoSlot, ctrl: 0}); n != 1 {
				t.Fatalf("ActivatorThisTurnCast count = %d, want 1 (the spell half was cast)", n)
			}
			x := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou, F: instOrSorc}
			if n := sc.s.thisTurnCastSpec(sc.k, x, ctx); n != 1 {
				t.Fatalf("Count$ThisTurnCast = %d, want 1", n)
			}
		})
	}
}

// TestCastLedgerReadsTheCastFaceOnRecast: the recast-on-stack window. An
// Adventure half was pushed, then the card was cast again as its creature
// from exile and that creature spell is still on the stack. The push is
// still read as the half it was cast with (CR 715.3b), as gorge's
// ledgerCastFace reads it by the face the object shows, not by its zone; the
// creature recast reads as the front (CR 715.4) and does not count.
func TestCastLedgerReadsTheCastFaceOnRecast(t *testing.T) {
	set, tbl, instOrSorc := castLedgerPool(t, "adventurecard")
	i, ok := set.Lookup("Bramble Familiar")
	if !ok {
		t.Fatal("Bramble Familiar not in adventurecard")
	}
	front := CardID(set.PoolID(i))
	sc := newScenTable(t, tbl, Config{})
	if !sc.k.gen.castRingOn {
		t.Fatal("the adventurecard table does not turn the cast ring on")
	}
	face := sc.k.t.Cards[front].Faces[1]
	o := sc.hand(face, 0)
	sc.kcastBuild(o, face, 0) // the Adventure cast (CR 715.3a)
	ring := sc.s.castPushes(0)
	if len(ring) != 1 || ring[0].prov&castAdventurePush == 0 {
		t.Fatalf("push not marked as the Adventure half: %+v", ring)
	}
	// It resolved into exile and was recast as its creature: the card and its
	// stack entry show the front again (CR 715.4), the creature still on the
	// stack. The push above stays the Adventure cast it was.
	sc.s.objs[o].card = front
	for i := range sc.s.stack[:sc.s.sp] {
		if st := &sc.s.stack[i]; st.kind == seSpell && st.obj == o {
			st.card = front
		}
	}
	ctx := evalCtx{src: NoObj, srcSl: NoSlot, ctrl: 0}
	// Precondition: the recast creature on the stack is not an instant or
	// sorcery, so a current-face read cannot find the Adventure cast.
	if sc.s.objs[o].zone != kop.ZoneStack {
		t.Fatalf("recast object in zone %d, want the stack", sc.s.objs[o].zone)
	}
	if sc.s.castRingMatch(sc.k, instOrSorc, o, ctx) {
		t.Fatal("the creature spell on the stack matches Instant,Sorcery; the test cannot tell the faces apart")
	}
	if n := sc.s.castPushesMatching(sc.k, instOrSorc, 0, trigListener{obj: NoObj, sl: NoSlot, ctrl: 0}); n != 1 {
		t.Fatalf("ActivatorThisTurnCast count = %d, want 1 (the Adventure cast, not the creature recast)", n)
	}
	// The CastSpecAsCast arm (OnlyFirstSpell$) reads the same push by the same
	// face rule.
	x := &kop.Expr{Kind: kop.ExThisTurnCastSpec, N: kop.CastSpecYou | kop.CastSpecAsCast, F: instOrSorc}
	if n := sc.s.thisTurnCastSpec(sc.k, x, ctx); n != 1 {
		t.Fatalf("CastSpecAsCast count = %d, want 1", n)
	}
}
