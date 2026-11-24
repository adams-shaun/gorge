package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCastLedgerReadsTheCastFace: a push of an Adventure (CR 715.3b) or Omen
// (CR 720.3) half is counted as that half by the type-filtered ledger readers
// after the card has left the stack and shows its normal face again (CR
// 715.4, 720.4): gorge castsThisTurnMatching (ledgerCastFace), Forge's
// cast-time ThisTurnCast copies, XMage's SpellsCastWatcher. Aquatic
// Alchemist's ActivatorThisTurnCast$ EQ1 over Instant,Sorcery
// (castPushesMatching) must see Bubble Up, resolved into exile, as the
// turn's first sorcery; so must a Count$ThisTurnCast_<spec> head.
func TestCastLedgerReadsTheCastFace(t *testing.T) {
	for _, tc := range []struct {
		pool, front string
		mode        uint8
		bit         uint8
		rest        kop.Zone
	}{
		{"std-woe", "Aquatic Alchemist", kop.FaceAdventure, castAdventurePush, kop.ZoneExile},
		{"std-tdm", "Bloomvine Regent", kop.FaceOmen, castOmenPush, kop.ZoneLibrary},
	} {
		t.Run(tc.front, func(t *testing.T) {
			set := cardsettest.Synth(t, tc.pool)
			tbl, err := set.Pool()
			if err != nil {
				t.Fatal(err)
			}
			tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TInstant | kop.TSorcery})
			instOrSorc := kop.FilterID(len(tbl.Filters) - 1)
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
