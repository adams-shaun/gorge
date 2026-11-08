package feat

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// killClockP is group K's clock P from the projection: the seat's
// non-defender creatures' positive power, tapped and sick included.
func killClockP(p *dirac.Projection, seat uint8) int32 {
	var sum int32
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		if o.Zone != kop.ZoneBattlefield || o.Ctrl != seat {
			continue
		}
		if p.Chars[i].Types&kop.TCreature == 0 || p.Chars[i].Kw&kop.KwDefender != 0 {
			continue
		}
		if o.Power > 0 {
			sum += int32(o.Power)
		}
	}
	return sum
}

// extKillEncoder is extVanillaEncoder with the table's printed type lines
// and keywords: the clock rules read creature-ness and defender, which the
// empty specs the turn-row tests use do not carry.
func extKillEncoder() *Encoder {
	tbl := vanilla.Table
	specs := make([]CardSpec, len(tbl.Cards))
	for i := range specs {
		cd := &tbl.Cards[i]
		var ty []string
		for _, t := range [...]struct {
			m kop.TypeMask
			n string
		}{{kop.TCreature, "Creature"}, {kop.TLand, "Land"}, {kop.TInstant, "Instant"},
			{kop.TSorcery, "Sorcery"}, {kop.TArtifact, "Artifact"}, {kop.TEnchantment, "Enchantment"},
			{kop.TPlaneswalker, "Planeswalker"}, {kop.TBattle, "Battle"}} {
			if cd.Types&t.m != 0 {
				ty = append(ty, t.n)
			}
		}
		var kws []string
		for b := 0; b < 64; b++ {
			if cd.Keywords&(1<<b) != 0 && KeywordNames[b] != "" {
				kws = append(kws, KeywordNames[b])
			}
		}
		specs[i] = CardSpec{Name: cd.Name, Types: strings.Join(ty, " "), Keywords: kws,
			Power: int32(cd.Power), Toughness: int32(cd.Toughness)}
	}
	e := NewEncoder(NewPool(tbl, specs))
	e.Reserve()
	return e
}

// TestExtClockBuckets checks the K clock one-hots against the projection at
// every decision of a vanilla random game: slot 448+t for me (opp's life / my
// P) and 457+t for opp (my life / opp's P), t = clamp(ceil(life/P),1,8), 8 for
// P = 0.
func TestExtClockBuckets(t *testing.T) {
	e := extKillEncoder()
	e.SetExt(ExtKill)
	s, pol := extVanillaGame(3)
	var d dirac.Decision
	var n, zeroP, posP int
	for k := 0; k < 20000; k++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		var p dirac.Projection
		s.Project(&p)
		s.ProjectChars(&p)
		st := e.EncodeState(s, &d)
		seat := d.Player
		pm, po := killClockP(&p, seat), killClockP(&p, 1-seat)
		if pm == 0 {
			zeroP++
		} else {
			posP++
		}
		for _, c := range []struct {
			base int
			life int32
			p    int32
			side string
		}{
			{xKClockMe, int32(p.Life[1-seat]), pm, "me"},
			{xKClockOpp, int32(p.Life[seat]), po, "opp"},
		} {
			want := clockT(c.life, c.p)
			set := 0
			for j := 0; j <= 8; j++ {
				if _, ok := extHas(st.Ext, c.base+j); ok {
					set++
					if j != int(want) {
						t.Fatalf("decision %d turn %d: %s clock P=%d life=%d: slot %d set, want %d",
							k, p.Turn, c.side, c.p, c.life, c.base+j, c.base+int(want))
					}
				}
			}
			if set != 1 {
				t.Fatalf("decision %d: %s clock has %d one-hot slots, want 1", k, c.side, set)
			}
		}
		n++
		s.Apply(pol.Pick(&d))
	}
	if n == 0 {
		t.Fatal("no decisions visited")
	}
	if zeroP == 0 || posP == 0 {
		t.Fatalf("clock P was never both 0 and positive (%d zero, %d positive): the rule was not exercised", zeroP, posP)
	}
}
