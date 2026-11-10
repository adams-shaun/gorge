package dirac

// Occupancy is a read-only census of the kernel's bounded pools at one
// moment: each pool's live entries against its capacity (Occupancy.Caps).
// It is a developer aid (capacity sizing): nothing in the
// kernel calls it, so it costs the hot path nothing.
type Occupancy struct {
	N [NumPools]int
}

// The pools Occupancy counts, in PoolNames order.
const (
	PoolObj        = iota // card instances (MaxObj)
	PoolPerm              // battlefield slots (MaxPerm)
	PoolStack             // stack entries (MaxStack)
	PoolPending           // waiting triggers (MaxPending)
	PoolDelayed           // delayed triggers (MaxDelayed)
	PoolRem               // remembered objects (maxRem)
	PoolChosen            // chosen objects (maxRem)
	PoolMay               // play-from-zone permissions (maxMay)
	PoolDProg             // delayed programs (maxDProg)
	PoolEmb               // emblems of the player with most (maxEmb)
	PoolDmgMap            // damage-map entries (maxDmgMap)
	PoolXTgt              // extra target objects of the fullest multi-target entry (maxXTgt)
	PoolXTPool            // multi-target pool entries in use (maxXTPool)
	PoolUsed              // once-per-turn records (maxUsed)
	PoolKwRepX            // repeated keyword grant records (maxKwRepX)
	PoolOvPool            // effect pool records, all permanents (maxFx)
	PoolOvr               // override records on the permanent with most (maxOvr)
	PoolTrigTurn          // triggers resolved this turn (maxTrigTurn)
	PoolLinks             // exile-until-leaves links (gen.links)
	PoolExiledWith        // exiled-with source links (gen.exw)
	PoolFree              // cast-without-paying candidates (gen.free)
	PoolFreeQ             // queued free casts (gen.freeQ)
	PoolAsk               // queued as-enters choices (gen.ask)
	PoolCtrKinds          // distinct counter kinds on the permanent with most (no cap)
	PoolPtX               // pumps past the int8 range (maxPtX)
	PoolTs                // the layer clock State.ts (uint16; this counts its peak)
	NumPools
)

// PoolNames names the pools; PoolCaps is each one's capacity (0 = none).
var (
	PoolNames = [NumPools]string{"obj", "perm", "stack", "pending", "delayed", "rem", "chosen", "may", "dprog",
		"emblem", "dmgmap", "xtgt", "xtpool", "used", "kwrepx", "ovpool", "ovr", "trigturn", "links", "exiledwith", "free", "freeq", "ask", "ctrkinds", "ptx", "ts"}
	PoolCaps = [NumPools]int{MaxObj, MaxPerm, MaxStack, MaxPending, MaxDelayed, maxRem, maxRem, maxMay + maxMayCold, maxDProg,
		maxEmb, maxDmgMap, maxXTgt, maxXTPool, maxUsed, maxKwRepX, maxFx, maxOvr, maxTrigTurn,
		len(State{}.g.links), maxExw, len(State{}.g.free), len(State{}.g.freeQ), len(State{}.g.ask), 0, maxPtX, 1<<16 - 1}
)

// Occupancy fills o with s's current pool occupancy.
func (s *State) Occupancy(o *Occupancy) {
	*o = Occupancy{}
	o.N[PoolObj] = int(s.nobj)
	o.N[PoolPerm] = s.occ.count()
	o.N[PoolStack] = int(s.sp)
	o.N[PoolPending] = int(s.npend)
	o.N[PoolDelayed] = int(s.ndly)
	o.N[PoolRem] = int(s.g.nrem)
	o.N[PoolChosen] = int(s.g.nchosen)
	o.N[PoolMay] = int(s.g.nmay)
	o.N[PoolDProg] = int(s.g.ndp)
	o.N[PoolEmb] = int(max(s.g.nemb[0], s.g.nemb[1]))
	o.N[PoolDmgMap] = int(s.g.ndmg)
	for i := uint8(1); i <= maxXTPool; i++ {
		if s.xtUsed(i) {
			o.N[PoolXTPool]++
			o.N[PoolXTgt] = max(o.N[PoolXTgt], int(s.g.xtp[i-1].n))
		}
	}
	o.N[PoolUsed] = int(s.g.nused)
	o.N[PoolKwRepX] = int(s.nkwx)
	o.N[PoolOvPool] = int(s.g.nfx)
	o.N[PoolTrigTurn] = int(s.g.trigTurn)
	o.N[PoolLinks] = int(s.g.nlinks)
	o.N[PoolExiledWith] = int(s.g.nexw)
	o.N[PoolFree] = int(s.g.nfree)
	o.N[PoolFreeQ] = int(s.g.nfq)
	o.N[PoolAsk] = int(s.g.nask)
	o.N[PoolPtX] = int(s.nptx)
	o.N[PoolTs] = int(s.ts)
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		n := 0
		for _, r := range s.ovRecs(sl) {
			if r.idx != 0 {
				n++
			}
		}
		o.N[PoolOvr] = max(o.N[PoolOvr], n)
		k := 0
		if s.perms[sl].p1 != 0 {
			k++
		}
		for _, c := range s.perms[sl].cn {
			if c != 0 {
				k++
			}
		}
		for _, e := range s.ctrOverOf(s.perms[sl].obj, 0) {
			if e.n != 0 {
				k++
			}
		}
		o.N[PoolCtrKinds] = max(o.N[PoolCtrKinds], k)
	}
}
