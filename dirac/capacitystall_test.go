package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// The silent-capacity fixes: each bounded pool in the
// kernel now stalls loud at capacity instead of clamping, dropping,
// evicting or recycling. The tests are synthetic-table scenarios in the
// pattern of TestLayerWalkCollectsTwentyFour (capacity_test.go)
// and TestDigWindowStallsLoud (trig_selfcast_test.go).

func TestSurveilNineStalls(t *testing.T) {
	for _, tc := range []struct {
		name string
		op   kop.Op
		ask  uint8
	}{
		{"surveil", kop.OpSurveil, raSurveil},
		{"scry", kop.OpScry, raScryTop},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// N=8 poses the ask; res.peek holds exactly eight.
			sc := newScen(t, Config{NoElide: true})
			sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
			if ok := sc.s.exec(sc.k, &kop.Instr{Op: tc.op, Obj: kop.RefController, A: kop.Const(8)}); ok {
				t.Fatalf("N=8: exec completed without posing an ask")
			}
			if sc.s.Stalled() {
				t.Fatalf("N=8: stalled %q", StallName(sc.s.StallReason()))
			}
			if sc.s.res.ask != tc.ask || sc.s.res.peekN != 8 {
				t.Fatalf("N=8: ask %d peekN %d, want %d and 8", sc.s.res.ask, sc.s.res.peekN, tc.ask)
			}

			// N=9 is past the peek window: stall, never pose a truncated ask.
			sc = newScen(t, Config{NoElide: true})
			sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
			sc.s.exec(sc.k, &kop.Instr{Op: tc.op, Obj: kop.RefController, A: kop.Const(9)})
			if got := sc.s.StallReason(); got != StallTables {
				t.Fatalf("N=9: stall %q, want tables", StallName(got))
			}
			if sc.s.res.ask == tc.ask {
				t.Fatalf("N=9: posed a truncated %d-card ask", sc.s.res.peekN)
			}
		})
	}
}

// TestLayerWalkSubGrantOverflowStalls: 24 collected effects plus one
// subtype-grant StatContinuous (or an emblem subtype grant) stalls
// StallBoard rather than dropping the grant.
func TestLayerWalkSubGrantOverflowStalls(t *testing.T) {
	type build struct {
		sc     *scen
		target Obj
	}
	mk := func(t *testing.T, lords int, emblem bool) build {
		t.Helper()
		tbl := *vanilla.Table
		tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
		tbl.Overrides = append([]kop.Override(nil), vanilla.Table.Overrides...)
		tbl.Exprs = make([]kop.Expr, 1) // force the generated path
		tbl.Overrides = append(tbl.Overrides, kop.Override{HasPT: true, SetP: 3, SetT: 3})
		ov := uint16(len(tbl.Overrides))
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Lord", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatOverride, Override: ov}}}})
		tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grant", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatContinuous, AddSub: (kop.SubMask{}).With(0)}}}})
		lord := CardID(len(tbl.Cards) - 2)
		if emblem {
			tbl.Emblems = append([]kop.Emblem(nil), vanilla.Table.Emblems...)
			tbl.Emblems = append(tbl.Emblems, kop.Emblem{Abilities: []kop.Ability{{Kind: kop.AbStatic,
				Static: kop.Static{Kind: kop.StatContinuous, AddSub: (kop.SubMask{}).With(1)}}}})
		}
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		target := sc.perm(vanilla.GrizzlyBears, 0)
		for i := 0; i < lords; i++ {
			sc.perm(lord, 0)
		}
		if !emblem {
			sc.perm(CardID(len(tbl.Cards)-1), 0) // the subtype grant source
		}
		if emblem {
			sc.s.g.nemb[0] = 1
			sc.s.g.emb[0][0] = 1
		}
		sc.s.g.stalled = 0
		return build{sc, target}
	}

	t.Run("subtype grant", func(t *testing.T) {
		b := mk(t, 23, false)
		var fx [maxLfx]lfx
		if got := b.sc.s.lfxCollect(b.sc.k, b.sc.slot(b.target), &fx); got != 24 {
			t.Fatalf("23 lords + grant: collected %d, want 24", got)
		}
		if b.sc.s.Stalled() {
			t.Fatalf("23 lords + grant: stalled %q", StallName(b.sc.s.StallReason()))
		}
		b = mk(t, 24, false)
		if got := b.sc.s.lfxCollect(b.sc.k, b.sc.slot(b.target), &fx); got != 24 {
			t.Fatalf("24 lords + grant: collected %d, want 24", got)
		}
		if got := b.sc.s.StallReason(); got != StallBoard {
			t.Fatalf("24 lords + grant: stall %q, want board", StallName(got))
		}
	})

	t.Run("emblem subtype grant", func(t *testing.T) {
		b := mk(t, 23, true)
		var fx [maxLfx]lfx
		if got := b.sc.s.lfxCollect(b.sc.k, b.sc.slot(b.target), &fx); got != 24 {
			t.Fatalf("23 lords + emblem: collected %d, want 24", got)
		}
		if b.sc.s.Stalled() {
			t.Fatalf("23 lords + emblem: stalled %q", StallName(b.sc.s.StallReason()))
		}
		b = mk(t, 24, true)
		if got := b.sc.s.lfxCollect(b.sc.k, b.sc.slot(b.target), &fx); got != 24 {
			t.Fatalf("24 lords + emblem: collected %d, want 24", got)
		}
		if got := b.sc.s.StallReason(); got != StallBoard {
			t.Fatalf("24 lords + emblem: stall %q, want board", StallName(got))
		}
	})
}

// TestDmgLatchNinthListenerStalls: a ninth DamageDealtOnce listener on one
// source in one combat batch stalls StallDmgLatch; eight latch and queue
// once each.
func TestDmgLatchNinthListenerStalls(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Watcher", Types: kop.TCreature, Power: 1, Toughness: 1,
		Abilities: []kop.Ability{{Kind: kop.AbTriggered, Trig: kop.Trigger{Event: kop.TrigDamageDealtOnce}}}})
	watcher := CardID(len(tbl.Cards) - 1)

	build := func(t *testing.T, n int) (*scen, *dmgLatch) {
		t.Helper()
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		src := sc.perm(vanilla.GrizzlyBears, 0)
		for i := 0; i < n; i++ {
			sc.perm(watcher, 0)
		}
		sc.s.g.stalled = 0
		sc.s.npend = 0
		latch := &dmgLatch{total: 1}
		sc.s.fireDamageEvGen(sc.k, sc.slot(src), 1, false, true, latch, 0, nil)
		return sc, latch
	}

	sc, latch := build(t, 8)
	if sc.s.Stalled() {
		t.Fatalf("8 listeners: stalled %q", StallName(sc.s.StallReason()))
	}
	if latch.n != 8 || sc.s.npend != 8 {
		t.Fatalf("8 listeners: latched %d, pending %d, want 8 and 8", latch.n, sc.s.npend)
	}

	sc, latch = build(t, 9)
	if got := sc.s.StallReason(); got != StallDmgLatch {
		t.Fatalf("9 listeners: stall %q, want dmglatch", StallName(got))
	}
	if latch.n != 8 {
		t.Fatalf("9 listeners: latched %d, want 8 (the ninth never drops into a full latch)", latch.n)
	}
}

// TestOverrideRecordsFitEight: eight distinct override records live on one
// slot in application order; the ninth stalls StallTables and no record is
// evicted. ObjHash and ViewHash stay bit-identical to the base [4]ovRec
// layout for three records (the hash reads the first four records as
// before, and only mixes the second four when record four exists).
func TestOverrideRecordsFitEight(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Overrides = append([]kop.Override(nil), vanilla.Table.Overrides...)
	tbl.Exprs = make([]kop.Expr, 1)
	base := len(tbl.Overrides)
	for i := 0; i < 9; i++ {
		ov := kop.Override{}
		if i == 0 {
			ov.HasPT, ov.SetP, ov.SetT = true, 10, 10
		}
		tbl.Overrides = append(tbl.Overrides, ov)
	}
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	target := sc.perm(vanilla.GrizzlyBears, 0)
	sl := sc.slot(target)
	var firstTS uint16
	for i := 0; i < 8; i++ {
		sc.s.applyOverridePT(sc.k, sl, base+i, vanilla.GrizzlyBears, 0, 10, 10)
		if i == 0 {
			firstTS = sc.s.ovRecs(sl)[0].ts
		}
	}
	if sc.s.Stalled() {
		t.Fatalf("8 records: stalled %q", StallName(sc.s.StallReason()))
	}
	recs := sc.s.ovRecs(sl)
	for i := 0; i < 8; i++ {
		if recs[i].idx == 0 {
			t.Fatalf("record %d missing", i)
		}
		if i > 0 && recs[i].ts <= recs[i-1].ts {
			t.Fatalf("records out of timestamp order at %d: %d then %d", i, recs[i-1].ts, recs[i].ts)
		}
	}
	if got := recs[0].ts; got != firstTS {
		t.Fatalf("oldest record timestamp %d, want %d (never evicted)", got, firstTS)
	}
	if c := sc.s.chars(sc.k, sl, upTo7b); c.bp != 10 || c.bt != 10 {
		t.Fatalf("chars P/T %d/%d, want 10/10 from the oldest record", c.bp, c.bt)
	}

	sc.s.applyOverridePT(sc.k, sl, base+8, vanilla.GrizzlyBears, 0, 0, 0)
	if got := sc.s.StallReason(); got != StallTables {
		t.Fatalf("9th record: stall %q, want tables", StallName(got))
	}
	if recs := sc.s.ovRecs(sl); recs[0].ts != firstTS {
		t.Fatalf("9th record evicted the oldest (ts %d, want %d)", recs[0].ts, firstTS)
	}

	// Hash compatibility: three records, computed with the base [4]ovRec
	// layout at 38babcfc (the first four records hash exactly as before).
	// The ViewHash recipe gained genState.embN with (the
	// generated state's emblems mix their frozen chosen number too), then
	// genState.embT (an emblem's layer timestamp orders its static against a
	// lose-all), so the view pin is the recipe after those changes; ObjHash
	// is unchanged.
	hsc := newScenTable(t, &tbl, Config{NoElide: true})
	ht := hsc.perm(vanilla.GrizzlyBears, 0)
	hsl := hsc.slot(ht)
	for i := 0; i < 3; i++ {
		hsc.s.applyOverridePT(hsc.k, hsl, base+i, vanilla.GrizzlyBears, 0, 10, 10)
	}
	// Computed at 38babcfc with the base [4]ovRec layout.
	const (
		wantObjHash  uint64 = 0xdc072663b2b01bd1
		wantViewHash uint64 = 0x68736735437a8890
	)
	if got := hsc.s.ObjHash(ht); got != wantObjHash {
		t.Fatalf("ObjHash %#x, want %#x", got, wantObjHash)
	}
	if got := hsc.s.ViewHash(0, nil); got != wantViewHash {
		t.Fatalf("ViewHash %#x, want %#x", got, wantViewHash)
	}
}

// TestReplCandsOverflowStalls: the three CR 616.1 candidate writers stall
// StallReplCands on a fifth candidate instead of silently leaving it out.
func TestReplCandsOverflowStalls(t *testing.T) {
	type cards struct{ stay, freeze, tyrant, barricade CardID }
	mk := func(t *testing.T) (cards, *scen) {
		t.Helper()
		tbl := *vanilla.Table
		tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
		tbl.Filters = append([]kop.Filter(nil), vanilla.Table.Filters...)
		tbl.Exprs = make([]kop.Expr, 1)
		tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FOther})
		other := kop.FilterID(len(tbl.Filters) - 1)
		tbl.Cards = append(tbl.Cards,
			kop.CardDef{Name: "Test Stay", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDoesntUntap}}}},
			kop.CardDef{Name: "Test Freeze", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplOthersDoesntUntap, Filter: other}}}},
			kop.CardDef{Name: "Test Tyrant", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDoubleDamageToOpps}}}},
			kop.CardDef{Name: "Test Barricade", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplPreventNoncombatToOthers}}}},
		)
		n := len(tbl.Cards)
		c := cards{CardID(n - 4), CardID(n - 3), CardID(n - 2), CardID(n - 1)}
		return c, newScenTable(t, &tbl, Config{NoElide: true})
	}

	t.Run("doesn't untap", func(t *testing.T) {
		c, sc := mk(t)
		stay := sc.perm(c.stay, 0)
		for i := 0; i < 3; i++ {
			sc.perm(c.freeze, 0)
		}
		sc.s.g.stalled = 0
		if got := sc.s.untapCands(sc.k, sc.slot(stay)); got != 4 || sc.s.Stalled() {
			t.Fatalf("4 candidates: got %d stall %q, want 4 and none", got, StallName(sc.s.StallReason()))
		}
		sc.perm(c.freeze, 0) // the fifth candidate
		sc.s.g.stalled = 0
		if got := sc.s.untapCands(sc.k, sc.slot(stay)); got != 0 {
			t.Fatalf("5 candidates: got %d, want 0 (no ask)", got)
		}
		if got := sc.s.StallReason(); got != StallReplCands {
			t.Fatalf("5 candidates: stall %q, want replcands", StallName(got))
		}
	})

	t.Run("damage doubler", func(t *testing.T) {
		c, sc := mk(t)
		src := sc.perm(vanilla.GrizzlyBears, 0)
		for i := 0; i < 4; i++ {
			sc.perm(c.tyrant, 0)
		}
		sc.s.g.stalled = 0
		if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), vanilla.GrizzlyBears, PlayerTarget1, 3, 0); !ok || sc.s.Stalled() {
			t.Fatalf("4 doublers: park %v stall %q, want true and none", ok, StallName(sc.s.StallReason()))
		}
		if sc.s.g.ro.n != 4 {
			t.Fatalf("4 doublers: candidates %d, want 4", sc.s.g.ro.n)
		}
		sc.perm(c.tyrant, 0) // the fifth candidate
		sc.s.g.ro = replOrd{}
		sc.s.g.nc = ncDmg{}
		sc.s.g.stalled = 0
		if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), vanilla.GrizzlyBears, PlayerTarget1, 3, 0); ok {
			t.Fatalf("5 doublers: park succeeded, want no ask")
		}
		if got := sc.s.StallReason(); got != StallReplCands {
			t.Fatalf("5 doublers: stall %q, want replcands", StallName(got))
		}
	})

	t.Run("noncombat prevention", func(t *testing.T) {
		c, sc := mk(t)
		src := sc.perm(vanilla.GrizzlyBears, 0)
		dst := sc.perm(vanilla.GrizzlyBears, 1)
		for i := 0; i < 4; i++ {
			sc.perm(c.barricade, 1)
		}
		sc.s.g.stalled = 0
		code := uint8(dst)
		if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), vanilla.GrizzlyBears, code, 3, 0); !ok || sc.s.Stalled() {
			t.Fatalf("4 preventers: park %v stall %q, want true and none", ok, StallName(sc.s.StallReason()))
		}
		if sc.s.g.ro.n != 4 {
			t.Fatalf("4 preventers: candidates %d, want 4", sc.s.g.ro.n)
		}
		sc.perm(c.barricade, 1) // the fifth candidate
		sc.s.g.ro = replOrd{}
		sc.s.g.nc = ncDmg{}
		sc.s.g.stalled = 0
		if ok := sc.s.parkNC(sc.k, 0, sc.slot(src), vanilla.GrizzlyBears, code, 3, 0); ok {
			t.Fatalf("5 preventers: park succeeded, want no ask")
		}
		if got := sc.s.StallReason(); got != StallReplCands {
			t.Fatalf("5 preventers: stall %q, want replcands", StallName(got))
		}
	})
}

// TestTokenMintAtMaxObjStalls: at MaxObj the mint stalls StallBoard and
// never reuses a ceased token's object index; a direct newToken is an
// invariant panic no guarded caller reaches.
func TestTokenMintAtMaxObjStalls(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	sc.s.nobj = MaxObj
	// A ceased token in the last object slot: the base recycler would take it.
	ceased := Obj(MaxObj - 1)
	sc.s.objs[ceased] = object{card: vanilla.GrizzlyBears, chosen: 255, of: ofToken, slot: NoSlot, acts: 0}
	before := sc.s.objs[ceased]
	sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
	sc.s.exec(sc.k, &kop.Instr{Op: kop.OpCreateToken, Obj: kop.RefController, A: kop.Const(1), C: int16(vanilla.GrizzlyBears)})
	if got := sc.s.StallReason(); got != StallBoard {
		t.Fatalf("mint at MaxObj: stall %q, want board", StallName(got))
	}
	if sc.s.nobj != MaxObj {
		t.Fatalf("mint at MaxObj: nobj %d, want %d (no reuse)", sc.s.nobj, MaxObj)
	}
	if sc.s.objs[ceased] != before {
		t.Fatalf("mint at MaxObj: reused ceased object %d", ceased)
	}

	sc = newScen(t, Config{NoElide: true})
	sc.s.nobj = MaxObj
	defer func() {
		if recover() == nil {
			t.Fatalf("newToken at MaxObj did not panic")
		}
	}()
	sc.s.newToken(vanilla.GrizzlyBears, 0)
}

// TestStallBuildingOptionsPosesNothing: a counterspell's feasibility probe finds
// a triggered ability past the stack range target codes can name. The game stalls
// while the priority options are built, and Step returns no decision to answer.
func TestStallBuildingOptionsPosesNothing(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	counter := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Counter", Types: kop.TInstant, Abilities: []kop.Ability{{Kind: kop.AbSpell,
		Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtStackObj}}, NTargets: 1,
		Prog: []kop.Instr{{Op: kop.OpNop}}}}})
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	sc.hand(counter, 0)
	for i := 0; i <= maxStackTgt; i++ {
		sc.s.stack[i].kind = seTrigger
	}
	sc.s.sp = uint8(maxStackTgt + 1)
	d := sc.s.Step()
	if got := sc.s.StallReason(); got != StallTables {
		t.Fatalf("stall %q, want tables", StallName(got))
	}
	if d.Kind != DkNone || d.N != 0 {
		t.Fatalf("a stalled game posed decision kind %d with %d options", d.Kind, d.N)
	}
}

// TestPoolAddPast255Stalls: a pool colour is a byte, so poolAdd saturates at
// 255 and stalls StallDynMana instead of wrapping (200 + 100 once left 44).
func TestPoolAddPast255Stalls(t *testing.T) {
	const col = 0
	fresh := func(have uint8) *scen {
		sc := newScen(t, Config{NoElide: true})
		sc.s.pl[0].pool[col] = have
		if sc.s.pl[0].pool[col] != have || sc.s.Stalled() {
			t.Fatalf("precondition: pool %d stalled %v", sc.s.pl[0].pool[col], sc.s.Stalled())
		}
		return sc
	}
	sc := fresh(200)
	sc.s.poolAdd(0, col, 100)
	if got := sc.s.pl[0].pool[col]; got != 255 || sc.s.g.stalled != StallDynMana {
		t.Fatalf("200+100: pool %d stalled %d, want 255 and StallDynMana", got, sc.s.g.stalled)
	}
	sc = fresh(200)
	sc.s.poolAdd(0, col, 55)
	if got := sc.s.pl[0].pool[col]; got != 255 || sc.s.Stalled() {
		t.Fatalf("200+55: pool %d stalled %d, want 255 and no stall", got, sc.s.g.stalled)
	}
	for _, n := range []int{0, -3} {
		sc = fresh(200)
		sc.s.poolAdd(0, col, n)
		if got := sc.s.pl[0].pool[col]; got != 200 || sc.s.Stalled() {
			t.Fatalf("add %d: pool %d stalled %d, want 200 and no stall", n, got, sc.s.g.stalled)
		}
	}
}

// TestDynManaPast255Stalls: a dynamic mana amount above 255 stalls
// StallDynMana at the amount (dynManaN) and at the pool add (OpAddMana), where
// uint8(300) used to leave 44 in the pool with no stall.
func TestDynManaPast255Stalls(t *testing.T) {
	t.Run("dynManaN", func(t *testing.T) {
		sc, c := actProliferateScen(t)
		sl := sc.slot(sc.perm(c.bear, 0))
		ab := kop.Ability{Kind: kop.AbMana, ManaV: kop.Const(255)}
		if got := sc.s.dynManaN(sc.k, &ab, sl); got != 255 || sc.s.Stalled() {
			t.Fatalf("amount 255: got %d stalled %d, want 255 and no stall", got, sc.s.g.stalled)
		}
		ab.ManaV = kop.Const(300)
		if got := sc.s.dynManaN(sc.k, &ab, sl); got != 300 || sc.s.g.stalled != StallDynMana {
			t.Fatalf("amount 300: got %d stalled %d, want 300 and StallDynMana", got, sc.s.g.stalled)
		}
	})
	t.Run("addMana", func(t *testing.T) {
		sc := newScen(t, Config{NoElide: true})
		sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
		in := &kop.Instr{Op: kop.OpAddMana, Obj: kop.RefController, A: kop.Const(255), C: 1}
		sc.s.exec(sc.k, in)
		if got := sc.s.pl[0].pool[0]; got != 255 || sc.s.Stalled() {
			t.Fatalf("255: pool %d stalled %d, want 255 and no stall", got, sc.s.g.stalled)
		}
		sc = newScen(t, Config{NoElide: true})
		sc.s.res = resCtx{e: stackEntry{ctrl: 0}, active: true}
		in.A = kop.Const(300)
		sc.s.exec(sc.k, in)
		if got := sc.s.pl[0].pool[0]; got != 255 || sc.s.g.stalled != StallDynMana {
			t.Fatalf("300: pool %d stalled %d, want 255 and StallDynMana", got, sc.s.g.stalled)
		}
	})
}
