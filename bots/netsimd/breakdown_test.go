package netsimd

import (
	"math/rand/v2"
	"os"
	"testing"
	"unsafe"
)

// Where an evaluation's time goes, on synthetic inputs shaped like the
// recorded corpus (TestCorpusStats: 14.7 cards with 1.07 identity rows, 140
// sparse rows, 3.2 options with 6.2 slot and 1.8 hashed rows). Each
// benchmark times one stage of Eval with the same kernels.

func loadBreakdownNet(b *testing.B) *Net {
	b.Helper()
	p := os.Getenv("NETSIMD_DZGW")
	if p == "" {
		b.Skip("no network: NETSIMD_DZGW is unset")
	}
	if _, err := os.Stat(p); err != nil {
		b.Skipf("no network at %s (NETSIMD_DZGW)", p)
	}
	g, err := Load(p, 0)
	if err != nil {
		b.Fatal(err)
	}
	return g
}

func randFeats(r *rand.Rand, n, limit int) []feat {
	fs := make([]feat, n)
	for i := range fs {
		fs[i] = feat{Row: uint16(r.IntN(limit)), Value: 1}
	}
	return fs
}

// One 64→64 layer: the per-call floor of a whole-layer kernel.
func BenchmarkKernelGemv64x64(b *testing.B) {
	g := loadBreakdownNet(b)
	x, y := aligned(64), aligned(64)
	for i := 0; i < b.N; i++ {
		gemvT(&g.polHid[0].wt[0], &x[0], &g.polHid[0].b[0], &y[0], 64, 64, true)
	}
}

func BenchmarkKernelGemv256x64(b *testing.B) {
	g := loadBreakdownNet(b)
	x, y := aligned(256), aligned(64)
	for i := 0; i < b.N; i++ {
		gemvT(&g.poolProj.wt[0], &x[0], &g.poolProj.b[0], &y[0], 256, 64, false)
	}
}

func BenchmarkKernelLN64(b *testing.B) {
	g := loadBreakdownNet(b)
	x := aligned(64)
	for i := 0; i < b.N; i++ {
		ln64(&x[0], &x[0], &g.fnorm.w[0], &g.fnorm.b[0], g.fnorm.eps)
	}
}

// The sparse bag: 140 rows gathered from the 2 MB table (32 wide).
func BenchmarkStageSparseBag(b *testing.B) {
	g := loadBreakdownNet(b)
	r := rand.New(rand.NewPCG(3, 4))
	// 200 distinct states' worth of rows so the gathers are not all hot
	sets := make([][]feat, 200)
	for i := range sets {
		sets[i] = randFeats(r, 140, tableRows)
	}
	y := aligned(32)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fs := sets[i%len(sets)]
		clear(y)
		bag(&y[0], &g.table[0], &fs[0], len(fs), tableRows, 32)
	}
}

// One card vector from scratch: card_raw, card_rows(bag), card_phi.
func BenchmarkStageCard(b *testing.B) {
	g := loadBreakdownNet(b)
	r := rand.New(rand.NewPCG(5, 6))
	raw, u, bg, ent := aligned(g.rawW), aligned(32), aligned(32), aligned(32)
	for i := range raw {
		raw[i] = float32(r.NormFloat64())
	}
	fs := randFeats(r, 1, tableRows)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gemvT(&g.cardRaw.wt[0], &raw[0], &g.cardRaw.b[0], &u[0], g.rawW, 32, false)
		clear(bg)
		bag(&bg[0], &g.table[0], &fs[0], 1, tableRows, 32)
		gemvT(&g.cardRows.wt[0], &bg[0], &u[0], &u[0], 32, 32, true)
		gemvT(&g.cardPhi.wt[0], &u[0], &g.cardPhi.b[0], &ent[0], 32, 32, true)
	}
}

// The state trunk after the cards and the sparse bag: pool_proj, dense_in,
// sparse_proj, the block, the final norm and the value head.
func BenchmarkStageTrunk(b *testing.B) {
	g := loadBreakdownNet(b)
	pool, dense, sp := aligned(256), aligned(g.rawW), aligned(32)
	s, t, h, vh := aligned(64), aligned(64), aligned(g.ff), aligned(64)
	var v float32
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gemvT(&g.poolProj.wt[0], &pool[0], &g.poolProj.b[0], &s[0], 256, 64, false)
		gemvT(&g.denseIn.wt[0], &dense[0], &s[0], &s[0], denseW, 64, false)
		addv64(&s[0], &g.denseIn.b[0])
		gemvT(&g.sparseProj.wt[0], &sp[0], &s[0], &s[0], 32, 64, false)
		ln64(&t[0], &s[0], &g.blocks[0].norm.w[0], &g.blocks[0].norm.b[0], g.blocks[0].norm.eps)
		gemvT(&g.blocks[0].fc1.wt[0], &t[0], &g.blocks[0].fc1.b[0], &h[0], 64, g.ff, true)
		gemvT(&g.blocks[0].fc2.wt[0], &h[0], &s[0], &s[0], g.ff, 64, false)
		addv64(&s[0], &g.blocks[0].fc2.b[0])
		ln64(&s[0], &s[0], &g.fnorm.w[0], &g.fnorm.b[0], g.fnorm.eps)
		gemvT(&g.val0.wt[0], &s[0], &g.val0.b[0], &vh[0], 64, 64, true)
		v += g.valOutB + dot64(&g.valOutW[0], &vh[0])
	}
	_ = v
}

// One option: dense, slot and hashed bags, none projections, norm, the
// policy head.
func BenchmarkStageOption(b *testing.B) {
	g := loadBreakdownNet(b)
	r := rand.New(rand.NewPCG(7, 8))
	od, o, hs, oh, oh2 := aligned(optDense), aligned(64), aligned(64), aligned(64), aligned(64)
	slots := randFeats(r, 6, slotRows)
	hashed := randFeats(r, 2, tableRows)
	var v float32
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		gemvT(&g.odense.wt[0], &od[0], &g.odense.b[0], &o[0], optDense, 64, false)
		bag(&o[0], &g.slots[0], &slots[0], len(slots), slotRows, 64)
		bag(&o[0], &g.htab[0], &hashed[0], len(hashed), tableRows, 64)
		addv64(&o[0], &g.noneA[0])
		addv64(&o[0], &g.noneB[0])
		ln64(&o[0], &o[0], &g.onorm.w[0], &g.onorm.b[0], g.onorm.eps)
		gemvT(&g.pol0o.wt[0], &o[0], &hs[0], &oh[0], 64, 64, true)
		gemvT(&g.polHid[0].wt[0], &oh[0], &g.polHid[0].b[0], &oh2[0], 64, 64, true)
		v += g.polOutB + dot64(&g.polOutW[0], &oh2[0])
	}
	_ = v
}

var _ = unsafe.Sizeof(feat{})
