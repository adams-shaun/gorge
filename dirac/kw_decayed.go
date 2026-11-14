package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Decayed's end-of-combat sacrifice (CR 702.147a, Tainted Adversary's
// token). gorge registers it at the attack trigger's resolution as a one-shot
// __kwDecayed delayed registration that is incarnation-tracked
// (events/apply_delayed.go): a creature that left the battlefield before the
// end of combat and came back as a new object is not acted on, and neither
// is one that is simply gone. Cold and out of line.

// dpTrack marks a dprog record tracked to its source's object version
// (OpDelayedProgram B = kop.DpTrack): objs[0]/vers[0] hold the source.
const dpTrack uint8 = 1 << 2

// dpTrackAmt marks a pending trigger as a tracked promise (trigAmt): the
// ordinary delayed programs carry amt 0..2, the speed trigger -1.
const dpTrackAmt int16 = -3

// delayTracked registers OpDelayedProgram with kop.DpTrack: the program runs
// only while the resolving source is still the object that registered it.
// The caller checked maxDProg.
//
//go:noinline
func (s *State) delayTracked(e *stackEntry, in *kop.Instr) {
	if s.objs[e.obj].ver != e.srcVer {
		// the creature left the battlefield since its attack trigger was put on the stack
		// (Condemn on a decayed token): "sacrifice it" cannot reach the new object (CR 400.7),
		// and gorge's OncePerSource registration (effDelayedTrigger) is never made, so no
		// end-of-combat promise goes on the stack
		return
	}
	d := dprog{card: e.card, ab: uint8(in.C), ctrl: e.ctrl, src: e.obj, sv: s.objs[e.obj].ver, n: 1 | dpTrack | dpWhen(in), ext: dpExt(in)}
	d.objs[0], d.vers[0] = e.obj, e.srcVer
	s.g.dp[s.g.ndp] = d
	s.g.dpLKI[s.g.ndp] = e.li
	s.g.ndp++
}
