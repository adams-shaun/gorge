package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// srcMemScen is a generated-path scenario whose table has one SrcMem card, so
// the prep has anySrcMem set and the cold srcMem region is laid out.
func srcMemScen(t *testing.T) *scen {
	t.Helper()
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rememberer", Types: kop.TCreature, Power: 1, Toughness: 1, SrcMem: true})
	return newScenTable(t, &tbl, Config{NoElide: true})
}

func srcMemRecs(s *State) []smRec { return coldSrcMem.of(s)[:s.smLen()] }

func TestSrcMemRecords(t *testing.T) {
	sc := srcMemScen(t)
	s := sc.s
	if len(coldSrcMem.of(s)) != maxSrcMem {
		t.Fatalf("region holds %d records, want %d", len(coldSrcMem.of(s)), maxSrcMem)
	}
	a, b := sc.perm(vanilla.GrizzlyBears, 0), sc.perm(vanilla.GrizzlyBears, 0)
	x, y := sc.perm(vanilla.GrizzlyBears, 1), sc.perm(vanilla.GrizzlyBears, 1)

	// add, dedupe, insertion order
	s.smAdd(x, a, smRem)
	s.smAdd(y, a, smRem)
	s.smAdd(x, a, smRem) // duplicate
	s.smAdd(x, a, smImp) // other kind
	s.smAdd(x, b, smRem) // other source
	want := []smRec{
		{x, a, s.objs[a].ver, smRem}, {y, a, s.objs[a].ver, smRem},
		{x, a, s.objs[a].ver, smImp}, {x, b, s.objs[b].ver, smRem},
	}
	got := srcMemRecs(s)
	if len(got) != len(want) {
		t.Fatalf("records %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("record %d = %v, want %v", i, got[i], want[i])
		}
	}
	// a duplicate keeps the old stamp whatever the source's ver
	s.objs[a].ver++
	s.smAdd(x, a, smRem)
	if n := s.smLen(); n != 4 || srcMemRecs(s)[0].sver == s.objs[a].ver {
		t.Fatalf("duplicate rewrote a record: len %d, %v", n, srcMemRecs(s))
	}
	s.objs[a].ver--

	// liveness across a leave and re-enter of the source
	r := srcMemRecs(s)[0]
	if !s.smLiveNow(&r) {
		t.Fatalf("a record of a battlefield source is not live")
	}
	s.objs[a].ver++ // the source left and came back: a new object
	if s.smLiveNow(&r) {
		t.Fatalf("a record of an earlier incarnation is live now")
	}
	trig := &stackEntry{kind: seTrigger, obj: a, srcVer: r.sver}
	if !s.smLiveFor(&r, trig) {
		t.Fatalf("a trigger fired from the stamped incarnation does not read the record")
	}
	trig.srcVer = r.sver + 1 // a leaves trigger queued after the departure
	if !s.smLiveFor(&r, trig) {
		t.Fatalf("a leaves trigger (ver+1) does not read the record")
	}
	trig.srcVer = r.sver + 2
	if s.smLiveFor(&r, trig) {
		t.Fatalf("a trigger of a later incarnation reads the record")
	}
	// an activated ability of the stamped incarnation still on the stack reads
	// the kept list (gorge's object keeps it through the departure: Dino DNA's
	// {6} ability after Dino DNA was sacrificed); a later incarnation's does not
	if !s.smLiveFor(&r, &stackEntry{kind: seAbility, obj: a, srcVer: r.sver}) {
		t.Fatalf("an activated ability of the stamped incarnation does not read the record")
	}
	if s.smLiveFor(&r, &stackEntry{kind: seAbility, obj: a, srcVer: r.sver + 2}) || s.smLiveFor(&r, &stackEntry{kind: seSpell, obj: a, srcVer: r.sver}) || s.smLiveFor(&r, nil) {
		t.Fatalf("a later ability, a spell or no entry reads a dead record")
	}
	s.objs[a].ver--

	// smClear: a pending trigger's incarnation survives a clear by the new one
	s.smAdd(y, a, smImpAny)
	old := srcMemRecs(s)[0] // (x, a, smRem) stamped at ver v
	s.objs[a].ver++
	s.smAdd(x, a, smRem) // the new incarnation's own record (same key as the old one: deduped)
	if s.smLen() != 5 {
		t.Fatalf("an old-incarnation duplicate appended: %v", srcMemRecs(s))
	}
	s.smClear(a, smRem, nil) // no entry: the records of a dead incarnation stay
	if s.smLen() != 5 {
		t.Fatalf("smClear dropped a dead incarnation's records: %v", srcMemRecs(s))
	}
	s.smClear(a, smRem, &stackEntry{kind: seTrigger, obj: a, srcVer: old.sver + 3}) // a later incarnation's trigger
	if s.smLen() != 5 {
		t.Fatalf("smClear dropped records a trigger of another incarnation cannot read: %v", srcMemRecs(s))
	}
	s.smClear(a, smRem, &stackEntry{kind: seTrigger, obj: a, srcVer: old.sver}) // its own trigger
	if st := srcMemRecs(s); len(st) != 3 || st[0].fl != smImp || st[1].src != b || st[2].fl != smImpAny {
		t.Fatalf("smClear of the trigger's own records left %v", st)
	}
	s.objs[a].ver--

	// smForget and smDropImp
	s.smForget(x, b, smRem)
	if st := srcMemRecs(s); len(st) != 2 {
		t.Fatalf("smForget left %v", st)
	}
	s.smDropImp(a)
	if s.smLen() != 0 {
		t.Fatalf("smDropImp left %v", srcMemRecs(s))
	}

	// smGC frees a stale record unless a stack entry or pending trigger holds it
	s.smAdd(x, a, smRem)
	s.smAdd(y, b, smRem)
	s.objs[a].ver += 5
	s.objs[b].ver += 5
	s.pend[0] = pendingTrig{obj: b, srcVer: s.objs[b].ver - 5}
	s.npend = 1
	s.smGC()
	if st := srcMemRecs(s); len(st) != 1 || st[0].src != b {
		t.Fatalf("smGC kept %v, want only b's (a pending trigger holds it)", st)
	}
	s.npend = 0
	s.smGC()
	if s.smLen() != 0 {
		t.Fatalf("smGC kept a stale record: %v", srcMemRecs(s))
	}
	s.objs[a].ver -= 5
	s.objs[b].ver -= 5

	// allocation-free
	if n := testing.AllocsPerRun(100, func() {
		s.smAdd(x, a, smRem)
		s.smClear(a, smRem, nil)
	}); n != 0 {
		t.Fatalf("smAdd/smClear allocate %v", n)
	}

	// the 65th live record stalls
	for i := range coldSrcMem.of(s) {
		coldSrcMem.of(s)[i] = smRec{}
	}
	for i := 0; i < maxSrcMem; i++ {
		s.smAdd(Obj(i), a, smRem)
	}
	if s.Stalled() || s.smLen() != maxSrcMem {
		t.Fatalf("filling the region: len %d stalled %v", s.smLen(), s.Stalled())
	}
	s.smAdd(Obj(maxSrcMem), a, smRem)
	if s.StallReason() != StallSrcMem || StallName(StallSrcMem) != "srcmem" {
		t.Fatalf("the 65th record: stall %q, want srcmem", StallName(s.StallReason()))
	}
}

func TestSrcMemEmptyRegionStalls(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	if len(coldSrcMem.of(sc.s)) != 0 {
		t.Fatalf("a table without the flag has a srcMem region")
	}
	sc.s.smAdd(0, 1, smRem)
	if sc.s.StallReason() != StallSrcMem {
		t.Fatalf("an empty region took a record: stall %q", StallName(sc.s.StallReason()))
	}
}
