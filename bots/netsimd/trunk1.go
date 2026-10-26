package netsimd

import (
	"unsafe"

	"github.com/adams-shaun/gorge/bots/policynet"
)

// trunkBag1 is trunkBagN for the production layout (Net.one): one trunk
// block, policy.0/2/4 and value.0/2. It is the pre-multi-block trunkBag,
// kept verbatim as straight-line code so the d=64 production path pays
// nothing for the general shapes: no block or head-layer loops (those cost
// ~94 user instructions per eval, +0.5%). trunkBagN computes the same bits
// on this layout (TestOneLayoutMatchesGeneral).
func (g *Net) trunkBag1(sc *Scratch, nc int, dense []float32, sparse []policynet.Feature, bagIn []float32, opts []policynet.Option, scores []float32) float32 {
	d, cd, e := g.d, g.cd, g.e
	sc.bag32 = growF(sc.bag32, e)

	// pools = [0.25·sum per group ‖ max per group] (an empty group's max is 0:
	// card vectors are post-relu, so max(0, c) = max over the group)
	sc.pool = growF(sc.pool, poolW*cd)
	clear(sc.pool)
	for k := 0; k < nc; k++ {
		if gr := int(sc.groups[k]); gr < groups {
			poolAccW(&sc.pool[gr*cd], &sc.pool[(groups+gr)*cd], &sc.ent[k*cd], cd)
		}
	}
	sc.s = growF(sc.s, d)
	s := &sc.s[0]
	gemvT(&g.poolProj.wt[0], &sc.pool[0], &g.poolProj.b[0], s, poolW*cd, d, false)

	// state trunk: s += dense_in(dense) + sparse_proj(bag(sparse))
	sc.dense = growF(sc.dense, denseW)
	n := copy(sc.dense, dense)
	clear(sc.dense[n:])
	gemvT(&g.denseIn.wt[0], &sc.dense[0], s, s, denseW, d, false)
	g.addv(s, &g.denseIn.b[0])
	if bagIn != nil {
		copy(sc.bag32, bagIn[:e])
	} else {
		clear(sc.bag32)
		if len(sparse) > 0 {
			bag(&sc.bag32[0], &g.table[0], (*feat)(unsafe.Pointer(&sparse[0])), len(sparse), tableRows, e)
		}
	}
	gemvT(&g.sparseProj.wt[0], &sc.bag32[0], s, s, e, d, false)
	// block: s += fc2(relu(fc1(LN(s))))
	sc.t = growF(sc.t, d)
	sc.h = growF(sc.h, g.ff)
	g.ln(&sc.t[0], s, &g.bnorm.w[0], &g.bnorm.b[0], g.bnorm.eps)
	gemvT(&g.fc1.wt[0], &sc.t[0], &g.fc1.b[0], &sc.h[0], d, g.ff, true)
	gemvT(&g.fc2.wt[0], &sc.h[0], s, s, g.ff, d, false)
	g.addv(s, &g.fc2.b[0])
	g.ln(s, s, &g.fnorm.w[0], &g.fnorm.b[0], g.fnorm.eps)

	// value head
	sc.vh = growF(sc.vh, g.hh)
	gemvT(&g.val0.wt[0], s, &g.val0.b[0], &sc.vh[0], d, g.hh, true)
	v := g.val2b + g.dot(&g.val2w[0], &sc.vh[0], g.hh)
	if len(opts) == 0 {
		return v
	}

	// options
	hh := g.hh
	sc.hs = growF(sc.hs, hh)
	gemvT(&g.pol0s.wt[0], s, &g.pol0s.b[0], &sc.hs[0], d, hh, false)
	sc.o = growF(sc.o, d)
	sc.oh = growF(sc.oh, hh)
	sc.oh2 = growF(sc.oh2, hh)
	sc.od = growF(sc.od, optDense)
	sc.pa = growF(sc.pa, max(nc, 1)*d)
	sc.pb = growF(sc.pb, max(nc, 1)*d)
	sc.paOK = growB(sc.paOK, nc)
	sc.pbOK = growB(sc.pbOK, nc)
	o := &sc.o[0]
	for j := range opts {
		op := &opts[j]
		// o = dense(opt dense) + bag(slots) + hproj(bag(hashed)) + bot·bot_vec + ea + eb
		n := copy(sc.od, op.Dense)
		clear(sc.od[n:])
		gemvT(&g.odense.wt[0], &sc.od[0], &g.odense.b[0], o, optDense, d, false)
		if len(op.Slots) > 0 {
			bag(o, &g.slots[0], (*feat)(unsafe.Pointer(&op.Slots[0])), len(op.Slots), slotRows, d)
		}
		if len(op.Hashed) > 0 {
			bag(o, &g.htab[0], (*feat)(unsafe.Pointer(&op.Hashed[0])), len(op.Hashed), tableRows, d)
		}
		if op.BotPick {
			g.addv(o, &g.bot[0])
		}
		if k := int(op.EntA) - 1; k >= 0 && k < nc {
			p := &sc.pa[k*d]
			if !sc.paOK[k] {
				gemvT(&g.eaProj.wt[0], &sc.ent[k*cd], &g.eaProj.b[0], p, cd, d, false)
				sc.paOK[k] = true
			}
			g.addv(o, p)
		} else {
			g.addv(o, &g.noneA[0])
		}
		if k := int(op.EntB) - 1; k >= 0 && k < nc {
			p := &sc.pb[k*d]
			if !sc.pbOK[k] {
				gemvT(&g.ebProj.wt[0], &sc.ent[k*cd], &g.ebProj.b[0], p, cd, d, false)
				sc.pbOK[k] = true
			}
			g.addv(o, p)
		} else {
			g.addv(o, &g.noneB[0])
		}
		g.ln(o, o, &g.onorm.w[0], &g.onorm.b[0], g.onorm.eps)
		// policy head: relu(hs + W_o·o), relu(policy.2), policy.4
		gemvT(&g.pol0o.wt[0], o, &sc.hs[0], &sc.oh[0], d, hh, true)
		gemvT(&g.pol2.wt[0], &sc.oh[0], &g.pol2.b[0], &sc.oh2[0], hh, hh, true)
		scores[j] = g.pol4b + g.dot(&g.pol4w[0], &sc.oh2[0], g.hh)
	}
	return v
}
