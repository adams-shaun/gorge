package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// an until-end-of-turn Effect's Triggers$ on combat
// events (CR 603.7, CR 114: the kernel's emblem stands in for the Effect
// object). gorge effEffect (engine/effects/effect.go) registers the body as
// a recurring delayed trigger dropped at the turn change (dropEOTEmblems);
// trigger_match.go checks the delayed triggers of a Damage or
// DeclareAttackers event after its face triggers, with "You" the effect's
// controller (trigger_delayed.go effectMatchController) and the printed
// matchers (delayedEventMatches). Mistway Spy, Dalkovan Encampment.

// DebugEffEOTTrigOppOwner is a mutation switch (TestEffeottrigMutation):
// the emblem's source filter is matched for the emblem owner's opponent
// instead of the owner (gorge effectMatchController: "You" is the effect's
// controller). Matching for the source's controller would be inert: an EOT
// emblem lives only on its owner's turn, when every creature dealing combat
// damage to a player is the owner's (CR 506.2, 510.1).
var DebugEffEOTTrigOppOwner bool

// EffEOTTrigStats counts the combat emblem firings (the lockstep's reach).
type EffEOTTrigStats struct {
	DmgFires        int // DamageDone emblem triggers queued
	AtkFires        int // AttackersDeclared emblem triggers queued
	DmgWithListener int // DamageDone emblem triggers queued by an event with a battlefield DamageDone listener (the hook after the listener walk)
	DmgBeside       int // DamageDone emblem triggers queued beside another pending trigger of the owner's (the trigger_order ask)
}

// DebugEffEOTTrigStats is the test hook; nil in normal play.
var DebugEffEOTTrigStats *EffEOTTrigStats

// fireEmblemsDmg fires the emblems' DamageDone triggers for one damage
// event from the permanent at src (to a player when toPlayer, combat
// damage when combat; rcp the recipient code), after the battlefield
// listeners: each emblem trigger whose flags and source filter match for
// the emblem's owner, queued as the creating card's latent command-zone
// copy with the source as its trigger object (amount 0: kgen refuses a
// body reading it).
//
//go:noinline
func (s *State) fireEmblemsDmg(k *prep, src Slot, toPlayer, combat bool, rcp uint8, listeners bool) {
	srcObj := s.perms[src].obj
	srcCtrl := s.perms[src].ctrl
	dpl := 1 - srcCtrl
	if pl, ok := recipientPlayer(rcp); ok {
		dpl = pl
	}
	for pp := uint8(0); pp < 2; pp++ {
		for i := 0; i < int(s.g.nemb[pp]); i++ {
			e := &k.t.Emblems[s.g.emb[pp][i]-1]
			if !e.UntilEOT {
				continue // a permanent emblem's combat damage trigger is not this code's (unverified against gorge)
			}
			for j := range e.Abilities {
				ea := &e.Abilities[j]
				if ea.Kind != kop.AbTriggered || ea.Trig.Event != kop.TrigDamageDone {
					continue
				}
				t := &ea.Trig
				if t.Flags&kop.TfCombat != 0 && !combat || t.Flags2&kop.Tf2NonCombat != 0 && combat {
					continue
				}
				if t.Flags&kop.TfToPlayer != 0 && !toPlayer {
					continue
				}
				if t.Flags2&kop.Tf2ToOpponent != 0 && dpl == pp {
					continue
				}
				if t.Flags2&kop.Tf2YourTurn != 0 && s.active != pp {
					continue
				}
				who := pp
				if DebugEffEOTTrigOppOwner {
					who = 1 - pp
				}
				if t.Filter != 0 && !s.matchPermGen(k, t.Filter, src, who, NoSlot, layerFull) {
					continue
				}
				c := s.g.embC[pp][i]
				cd := &k.t.Cards[c]
				for ab := range cd.Abilities {
					la := &cd.Abilities[ab]
					if la.Kind == kop.AbTriggered && la.AFlags&kop.AfLatent != 0 && la.Trig.Event == kop.TrigDamageDone && la.Trig.Zone == kop.ZoneCommand {
						o := s.g.embO[pp][i]
						if st := DebugEffEOTTrigStats; st != nil {
							st.DmgFires++
							if listeners {
								st.DmgWithListener++
							}
							for x := 0; x < int(s.npend); x++ {
								if s.pend[x].ctrl == pp {
									st.DmgBeside++
									break
								}
							}
						}
						s.enqueueLKI(o, s.objs[o].ver+1, c, uint8(ab), pp, srcObj, 1-srcCtrl, 0)
						break
					}
				}
			}
		}
	}
}

// fireEmblemsAttack fires the emblems' AttackersDeclared triggers for the
// active player's declaration (after the battlefield listeners, before the
// attackers' tap triggers), gated on the attacker count as playerTrigGen
// gates a printed line.
//
//go:noinline
func (s *State) fireEmblemsAttack(k *prep) {
	if s.nAttackers == 0 {
		return
	}
	n, before := int(s.nAttackers), s.npend
	s.fireEmblems(k, kop.TrigAttackersDeclared, s.active, NoObj, func(a *kop.Ability) bool {
		return a.Trig.N == 0 || trigCmpOK(n, a.Trig.N, true)
	})
	if st := DebugEffEOTTrigStats; st != nil {
		st.AtkFires += int(s.npend) - int(before)
	}
}
