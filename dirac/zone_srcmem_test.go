package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// srcMemScen is a generated-path scenario whose table has one SrcMem card, so
// the prep has anySrcMem set and the cold object-set region is laid out.
func srcMemScen(t *testing.T) *scen {
	t.Helper()
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Rememberer", Types: kop.TCreature, Power: 1, Toughness: 1, SrcMem: true})
	return newScenTable(t, &tbl, Config{NoElide: true})
}

func srcMemRecs(s *State) []osRec { return s.cold.objSets[:s.osLen()] }

func TestSrcMemRecords(t *testing.T) {
	sc := srcMemScen(t)
	s := sc.s
	if len(s.cold.objSets) != maxObjSetsOne {
		t.Fatalf("region holds %d records, want %d", len(s.cold.objSets), maxObjSetsOne)
	}
	a, b := sc.perm(vanilla.GrizzlyBears, 0), sc.perm(vanilla.GrizzlyBears, 0)
	x, y := sc.perm(vanilla.GrizzlyBears, 1), sc.perm(vanilla.GrizzlyBears, 1)

	// add, dedupe, insertion order
	s.osAdd(osRem, a, x)
	s.osAdd(osRem, a, y)
	s.osAdd(osRem, a, x) // duplicate
	s.osAdd(osImp, a, x) // other kind
	s.osAdd(osRem, b, x) // other source
	want := []osRec{
		{x, a, s.objs[a].ver, osRem}, {y, a, s.objs[a].ver, osRem},
		{x, a, s.objs[a].ver, osImp}, {x, b, s.objs[b].ver, osRem},
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
	s.osAdd(osRem, a, x)
	if n := s.osLen(); n != 4 || srcMemRecs(s)[0].sver == s.objs[a].ver {
		t.Fatalf("duplicate rewrote a record: len %d, %v", n, srcMemRecs(s))
	}
	s.objs[a].ver--

	// liveness across a leave and re-enter of the source
	r := srcMemRecs(s)[0]
	if !s.osLiveNow(&r) {
		t.Fatalf("a record of a battlefield source is not live")
	}
	s.objs[a].ver++ // the source left and came back: a new object
	if s.osLiveNow(&r) {
		t.Fatalf("a record of an earlier incarnation is live now")
	}
	trig := &stackEntry{kind: seTrigger, obj: a, srcVer: r.sver}
	if !s.osLiveFor(&r, trig) {
		t.Fatalf("a trigger fired from the stamped incarnation does not read the record")
	}
	trig.srcVer = r.sver + 1 // a leaves trigger queued after the departure
	if !s.osLiveFor(&r, trig) {
		t.Fatalf("a leaves trigger (ver+1) does not read the record")
	}
	trig.srcVer = r.sver + 2
	if s.osLiveFor(&r, trig) {
		t.Fatalf("a trigger of a later incarnation reads the record")
	}
	// an activated ability of the stamped incarnation still on the stack reads
	// the kept list (gorge's object keeps it through the departure: Dino DNA's
	// {6} ability after Dino DNA was sacrificed); a later incarnation's does not
	if !s.osLiveFor(&r, &stackEntry{kind: seAbility, obj: a, srcVer: r.sver}) {
		t.Fatalf("an activated ability of the stamped incarnation does not read the record")
	}
	if s.osLiveFor(&r, &stackEntry{kind: seAbility, obj: a, srcVer: r.sver + 2}) || s.osLiveFor(&r, &stackEntry{kind: seSpell, obj: a, srcVer: r.sver}) || s.osLiveFor(&r, nil) {
		t.Fatalf("a later ability, a spell or no entry reads a dead record")
	}
	s.objs[a].ver--

	// osClear: a pending trigger's incarnation survives a clear by the new one
	s.osAdd(osImpAny, a, y)
	old := srcMemRecs(s)[0] // (x, a, osRem) stamped at ver v
	s.objs[a].ver++
	s.osAdd(osRem, a, x) // the new incarnation's own record (same key as the old one: deduped)
	if s.osLen() != 5 {
		t.Fatalf("an old-incarnation duplicate appended: %v", srcMemRecs(s))
	}
	s.osClear(a, osMRem, nil) // no entry: the records of a dead incarnation stay
	if s.osLen() != 5 {
		t.Fatalf("osClear dropped a dead incarnation's records: %v", srcMemRecs(s))
	}
	s.osClear(a, osMRem, &stackEntry{kind: seTrigger, obj: a, srcVer: old.sver + 3}) // a later incarnation's trigger
	if s.osLen() != 5 {
		t.Fatalf("osClear dropped records a trigger of another incarnation cannot read: %v", srcMemRecs(s))
	}
	s.osClear(a, osMRem, &stackEntry{kind: seTrigger, obj: a, srcVer: old.sver}) // its own trigger
	if st := srcMemRecs(s); len(st) != 3 || st[0].kind != osImp || st[1].src != b || st[2].kind != osImpAny {
		t.Fatalf("osClear of the trigger's own records left %v", st)
	}
	s.objs[a].ver--

	// osForget and osDropImp
	s.osForget(x, b, osMRem)
	if st := srcMemRecs(s); len(st) != 2 {
		t.Fatalf("osForget left %v", st)
	}
	s.osDropImp(a)
	if s.osLen() != 0 {
		t.Fatalf("osDropImp left %v", srcMemRecs(s))
	}

	// osGC frees a stale record unless a stack entry or pending trigger holds it
	s.osAdd(osRem, a, x)
	s.osAdd(osRem, b, y)
	s.objs[a].ver += 5
	s.objs[b].ver += 5
	s.pend[0] = pendingTrig{obj: b, srcVer: s.objs[b].ver - 5}
	s.npend = 1
	s.osGC()
	if st := srcMemRecs(s); len(st) != 1 || st[0].src != b {
		t.Fatalf("osGC kept %v, want only b's (a pending trigger holds it)", st)
	}
	s.npend = 0
	s.osGC()
	if s.osLen() != 0 {
		t.Fatalf("osGC kept a stale record: %v", srcMemRecs(s))
	}
	s.objs[a].ver -= 5
	s.objs[b].ver -= 5

	// allocation-free
	if n := testing.AllocsPerRun(100, func() {
		s.osAdd(osRem, a, x)
		s.osClear(a, osMRem, nil)
	}); n != 0 {
		t.Fatalf("osAdd/osClear allocate %v", n)
	}

	// the 65th live record stalls
	for i := range s.cold.objSets {
		s.cold.objSets[i] = osRec{}
	}
	for i := 0; i < maxObjSetsOne; i++ {
		s.osAdd(osRem, a, Obj(i))
	}
	if s.Stalled() || s.osLen() != maxObjSetsOne {
		t.Fatalf("filling the region: len %d stalled %v", s.osLen(), s.Stalled())
	}
	s.osAdd(osRem, a, Obj(maxObjSetsOne))
	if s.StallReason() != StallObjSets || StallName(StallObjSets) != "objsets" {
		t.Fatalf("the 65th record: stall %q, want objsets", StallName(s.StallReason()))
	}
}

func TestSrcMemEmptyRegionStalls(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	if len(coldObjSets.of(sc.s)) != 0 {
		t.Fatalf("a table without the flag has an object-set region")
	}
	sc.s.osAdd(osRem, 1, 0)
	if sc.s.StallReason() != StallObjSets {
		t.Fatalf("an empty region took a record: stall %q", StallName(sc.s.StallReason()))
	}
}
