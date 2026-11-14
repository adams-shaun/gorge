//go:build dirac_mp

package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// fourSeatHundred is a four-seat Commander game of four 100-card decks (the
// vanilla decks repeated), each with a Grizzly Bears commander at deck
// index 0: 400 objects before the first token, so seats 2 and 3 own only
// objects past 255.
func fourSeatHundred(seed uint64) *dirac.State {
	var decks [dirac.MaxSeats][]dirac.CardID
	var cmds [dirac.MaxSeats][2]int16
	for p := 0; p < 4; p++ {
		d := []dirac.CardID{vanilla.GrizzlyBears}
		for len(d) < 100 {
			d = append(d, vanilla.Decks[p][:min(40, 100-len(d))]...)
		}
		decks[p] = d
		cmds[p] = [2]int16{0, -1}
	}
	s := new(dirac.State)
	s.InitN(vanilla.Table, decks, 4, seed, dirac.Config{NoElide: true, Commander: true, Commanders: cmds})
	return s
}

// TestObjWidthFourSeatHundred: the encoders name objects at the kernel's
// object width. In 4 x 100 Commander games, at every 7th decision and for
// every viewer, each listed card is the kernel's object (battlefields per
// controller, the viewer's hand, every graveyard and exile), its attached
// and blocked links are the projection's, and an object past 255 (a token
// included) shows on its controller's battlefield list.
func TestObjWidthFourSeatHundred(t *testing.T) {
	e := NewEncoder(NewPool(vanilla.Table, make([]CardSpec, len(vanilla.Table.Cards))))
	e.Reserve()
	var po dirac.ProjObj
	checked, high, tokens := 0, 0, 0
	for seed := uint64(1); seed <= 3; seed++ {
		s := fourSeatHundred(seed)
		if s.NumSeats() != 4 {
			t.Fatalf("setup: %d seats", s.NumSeats())
		}
		pol := dirac.NewRandomPolicy(seed)
		var d dirac.Decision
		for i := 0; i < 300; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			if i%7 == 0 {
				for viewer := uint8(0); viewer < 4; viewer++ {
					e.EncodeStateFor(s, &d, viewer)
					v := &e.v
					for p := uint8(0); p < 4; p++ {
						pl := &v.players[p]
						lists := [...]struct {
							got  []int32
							want []dirac.Obj
							name string
						}{
							{pl.bf, s.FeatBattlefield(p, nil), "battlefield"},
							{pl.grave, s.FeatZoneList(p, kop.ZoneGraveyard, nil), "graveyard"},
							{pl.exile, s.FeatZoneList(p, kop.ZoneExile, nil), "exile"},
							{pl.hand, s.FeatZoneList(p, kop.ZoneHand, nil), "hand"},
						}
						for k, l := range lists {
							if k == 3 && p != viewer {
								continue
							}
							objs := l.want
							if len(l.got) != len(objs) {
								t.Fatalf("seed %d step %d viewer %d: seat %d %s lists %d cards, kernel %d", seed, i, viewer, p, l.name, len(l.got), len(objs))
							}
							for j, ci := range l.got {
								c := &v.cards[ci]
								if dirac.Obj(c.obj) != objs[j] {
									t.Fatalf("seed %d step %d viewer %d: seat %d %s card %d is object %d, kernel %d", seed, i, viewer, p, l.name, j, c.obj, objs[j])
								}
								if k != 0 {
									continue
								}
								s.FeatProjObj(objs[j], &po)
								if dirac.Obj(c.attached) != po.Attached || dirac.Obj(c.cblocks) != po.Blocks {
									t.Fatalf("seed %d step %d viewer %d: object %d attached %d blocks %d, kernel %d %d", seed, i, viewer, objs[j], c.attached, c.cblocks, po.Attached, po.Blocks)
								}
								if viewer == 0 && objs[j] > 255 {
									high++
									if po.Token {
										tokens++
									}
								}
							}
						}
					}
				}
				checked++
			}
			s.Apply(pol.Pick(&d))
		}
	}
	t.Logf("%d positions checked; %d battlefield sightings of objects past 255, %d of them tokens", checked, high, tokens)
	if high == 0 || tokens == 0 {
		t.Fatalf("precondition: objects past 255 seen %d times, tokens %d", high, tokens)
	}
}
