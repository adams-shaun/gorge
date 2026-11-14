package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Lwaxana Troi, Charming Guest's "{2}, {T}: Choose another target creature
// you control. For as long as Lwaxana remains tapped, if a triggered ability
// of that creature triggers, that ability triggers an additional time"
// (gorge Effect Mode$ Panharmonicon | ValidCard$ Card.IsRemembered, Duration$
// UntilUntaps, ExileOnMoved$ Battlefield; CR 603.2d, 611.2b). Each resolution
// adds one record (genState.pgT the granted object, pgH the tapped host,
// object+1, compacted so pgT[0] == 0 means none). panharmEchoes counts the
// records naming the trigger's source beside the printed statics, so two
// grants on one creature echo twice, as two registered effects do in gorge.
//
// A record ends at the host's first untap or departure (gorge
// expireUntilUntaps; the untap sites call netDrop, which reaches pgDropHost)
// and at the granted object's departure (effectMoveSweep; the leave site in
// zones.go calls pgDropTarget). A retap of the host never revives it.

// pgN is the grants live at once.
const pgN = 4

// DebugLwaxanaNoGrant switches the grant off (the Lwaxana mutation test: the
// lockstep must then diverge).
var DebugLwaxanaNoGrant bool

// prepGrantPanharm arms the echo walk (genPrep.anyPanharm) when some program
// holds an ExtGrantPanharm.
func prepGrantPanharm(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == kop.OpExt && in.A.K == kop.VConst && kop.ExtOp(in.A.N) == kop.ExtGrantPanharm {
					g.anyPanharm = true
				}
			}
		}
	}
}

// netAny reports whether a lock (Braided Net) or a Panharmonicon grant is
// live: the untap and leave sites then call netDrop.
func (g *genState) netAny() bool { return g.nlA[0] != 0 || g.pgT[0] != 0 }

// grantPanharm is ExtGrantPanharm: record the grant when the resolving
// ability's source is a tapped permanent and the target is a permanent.
//
//go:noinline
func (s *State) grantPanharm(k *prep, in *kop.Instr) {
	if DebugLwaxanaNoGrant {
		return
	}
	hs, ts := s.objRef(k, kop.RefSelf), s.objRef(k, in.Obj)
	if hs == NoSlot || ts == NoSlot || s.perms[hs].flags&pfTapped == 0 {
		return // gorge: a host that is not tapped as the effect is created starts nothing
	}
	for i := 0; i < pgN; i++ {
		if s.g.pgT[i] == 0 {
			s.g.pgT[i], s.g.pgH[i] = s.perms[ts].obj+1, s.perms[hs].obj+1
			s.touchSl(ts)
			return
		}
	}
	s.capFull(StallPanharmGrant)
}

// panharmGrants is the number of live grants on object o: the additional
// placements they demand for a trigger of o's.
func (s *State) panharmGrants(o Obj) int {
	n := 0
	for i := 0; i < pgN && s.g.pgT[i] != 0; i++ {
		if s.g.pgT[i] == o+1 {
			n++
		}
	}
	return n
}

// pgDropHost ends the grants hosted by o (its untap or departure).
func (s *State) pgDropHost(o Obj) { s.pgDrop(o, true) }

// pgDropTarget ends the grants given to o (its departure).
func (s *State) pgDropTarget(o Obj) { s.pgDrop(o, false) }

func (s *State) pgDrop(o Obj, host bool) {
	if s.g.pgT[0] == 0 {
		return
	}
	var t, h [pgN]Obj
	n := 0
	for i := 0; i < pgN && s.g.pgT[i] != 0; i++ {
		key := s.g.pgT[i]
		if host {
			key = s.g.pgH[i]
		}
		if key == o+1 {
			continue
		}
		t[n], h[n] = s.g.pgT[i], s.g.pgH[i]
		n++
	}
	s.g.pgT, s.g.pgH = t, h
}

func init() { stallNames[StallPanharmGrant] = "panharmgrant" }
