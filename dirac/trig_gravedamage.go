package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// the DamageDone trigger that works from the graveyard (Chandra's Phoenix:
// "whenever an opponent is dealt damage by a red instant or sorcery spell you
// control or by a red planeswalker you control, return this card from your
// graveyard to your hand"; CR 603.6, 113.6k). Called after the battlefield
// listeners of every damage event, from both damage paths: a permanent source
// (fireDamageEvGen) and a source that is no permanent (fireDamageSrc: a
// spell). Player 0's graveyard first, each in graveyard order, the order the
// other graveyard walks use (fireGraveDrawn). A line reads the source filter,
// the recipient half (TfToPlayer, Tf2ToOpponent, Tf2ToYou, Filter2) and the
// combat gate, as the battlefield walk does; kgen lowers no other parameter
// onto it (lower_gravedamage.go).

// fireGraveDamage queues the graveyard cards' DamageDone lines for one damage
// event of amt from the permanent at src (NoSlot: the spell or ability source
// o controlled by ctrl) to the players plM names, or the object rcp names.
//
//go:noinline
func (s *State) fireGraveDamage(k *prep, src Slot, o Obj, ctrl uint8, amt int16, plM uint8, combat bool, rcp TCode) {
	toPlayer := plM != 0
	for _, c := range k.gen.graveTrig {
		cd := &k.t.Cards[c]
		for pp := uint8(0); pp < s.nSeats(); pp++ {
			pl := &s.pl[pp]
			for i := 0; i < int(pl.graveN); i++ {
				ob := pl.grave[i]
				if s.objs[ob].card != c {
					continue
				}
				for j := range cd.Abilities {
					a := &cd.Abilities[j]
					if a.Kind != kop.AbTriggered || a.Trig.Zone != kop.ZoneGraveyard || a.Trig.Event != kop.TrigDamageDone {
						continue
					}
					t := &a.Trig
					if t.Flags&kop.TfSelf != 0 || t.Flags&kop.TfCombat != 0 && !combat || t.Flags2&kop.Tf2NonCombat != 0 && combat {
						continue // a graveyard card is never its own damage source
					}
					if src == NoSlot && t.Flags2&kop.Tf2SrcInPlay != 0 {
						continue // ValidSource$ ...inZoneBattlefield: this source is no permanent
					}
					if t.Flags&kop.TfToPlayer != 0 && !toPlayer && t.Filter2 == 0 {
						continue
					}
					if t.Flags2&kop.Tf2ToOpponent != 0 && !oppDamaged(plM, pp) || t.Flags2&kop.Tf2YourTurn != 0 && s.active != pp {
						continue
					}
					l := trigListener{obj: ob, sl: NoSlot, ctrl: pp, card: c, ab: uint8(j)}
					if t.Filter != 0 {
						if src != NoSlot {
							if !s.matchPermGen(k, t.Filter, src, pp, NoSlot, layerFull) {
								continue
							}
						} else if !s.matchCardGen(k, t.Filter, s.objs[o].card, ctrl, pp, o) {
							continue
						}
					}
					if !s.trigOK(k, l, a) {
						continue
					}
					tobj, tpl, n := o, s.dmgSeat(rcp, ctrl), amt
					if k.gen.anyDmgRef {
						var ok bool
						if n, tobj, tpl, ok = s.ddoLine(k, l, a, src, o, ctrl, amt, rcp, nil); !ok {
							continue
						}
					}
					s.queueTrig(k, l, a, tobj, tpl, n)
				}
			}
		}
	}
}
