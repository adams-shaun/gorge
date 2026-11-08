package oracle

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Group I (ExtImpact) on the constructed FDN positions (featscen.go).

// extImpactOn encodes sc with group I on and returns the Ext plus the encoder
// (whose DebugView carries the view's avail, which the test oracle has no
// other way to read).
func extImpactOn(sc *FeatScenario) ([]feat.Feature, *feat.Encoder) {
	e := feat.NewEncoder(sc.Pool)
	e.SetExt(feat.ExtImpact)
	return append([]feat.Feature(nil), e.EncodeStateFor(sc.State, &sc.Dec, sc.Seat).Ext...), e
}

// extCastableClass reports whether the seat's own hand holds a card of
// effect class class that passes castable's test (!land && mv <= me.avail).
// It reads the seat's view, not the decision's options: some scenarios pose
// the opponent's priority, where the seat has no cast options at all.
func extCastableClass(t *testing.T, sc *FeatScenario, class uint8) bool {
	t.Helper()
	e := feat.NewEncoder(sc.Pool)
	e.SetExt(feat.ExtImpact)
	e.EncodeStateFor(sc.State, &sc.Dec, sc.Seat)
	avail := extDebugAvail(t, e, int(sc.Seat))
	var p dirac.Projection
	sc.State.Project(&p)
	for i := 0; i < int(p.NObj); i++ {
		o := &p.Objs[i]
		if o.Zone != kop.ZoneHand || o.Owner != sc.Seat {
			continue
		}
		cd := &sc.State.Table().Cards[o.Card]
		if cd.Types&kop.TLand != 0 || cd.Cost.MV() > avail {
			continue
		}
		if sc.Pool.Effect(o.Card).Class == class {
			return true
		}
	}
	return false
}

// extDebugAvail reads player's avail from the encoder's last view.
func extDebugAvail(t *testing.T, e *feat.Encoder, player int) int {
	t.Helper()
	want := "p" + strconv.Itoa(player)
	for _, line := range strings.Split(e.DebugView(), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 || f[0] != want {
			continue
		}
		for i := 0; i+1 < len(f); i++ {
			if f[i] != "avail" {
				continue
			}
			n, err := strconv.Atoi(f[i+1])
			if err != nil {
				t.Fatalf("DebugView %q: avail %q: %v", line, f[i+1], err)
			}
			return n
		}
	}
	t.Fatalf("DebugView has no p%d avail:\n%s", player, e.DebugView())
	return 0
}

func TestExtImpactBaitVsBomb(t *testing.T) {
	by := stackScenarios(t)
	for _, c := range []struct {
		name string
		one  bool // a 1/1 bait, else a bomb
	}{{"cheapkill-on-oneone", true}, {"bigkill-on-bomb", false}} {
		sc := by[c.name]
		_, top := topEntry(t, sc)
		p := new(dirac.Projection)
		sc.State.Project(p)
		tgt := p.Objs[top.Targets[0]]
		ef := sc.Pool.Effect(top.Card)
		indestr := sc.State.Table().Cards[tgt.Card].Keywords&kop.KwIndestructible != 0
		if c.one {
			if tgt.Power != 1 || tgt.Tough != 1 {
				t.Fatalf("%s: target %d/%d, want the 1/1 bait", c.name, tgt.Power, tgt.Tough)
			}
			if b := feat.StrengthBucket(sc.Pool.PrintedStrength(tgt.Card)); b != 0 {
				t.Fatalf("%s: the 1/1 bait printed strength bucket is %d", c.name, b)
			}
		} else {
			if specKills(ef, int32(top.X), tgt, indestr) != 1 {
				t.Fatalf("%s: the kill does not resolve-kill its target", c.name)
			}
			if feat.StrengthBucket(sc.Pool.PrintedStrength(tgt.Card)) < 3 {
				t.Fatalf("%s: target printed strength %d is not a bomb", c.name, sc.Pool.PrintedStrength(tgt.Card))
			}
		}
		if !extCastableClass(t, sc, feat.EffCounter) {
			t.Fatalf("%s: no castable counter: the counter context precondition fails", c.name)
		}
	}
	cheap, _ := extImpactOn(by["cheapkill-on-oneone"])
	big, _ := extImpactOn(by["bigkill-on-bomb"])
	if got := extIn(cheap, 404, 407); !reflect.DeepEqual(got, []int{404}) {
		t.Errorf("cheapkill-on-oneone: top impact %v, want [404] (%s)", got, feat.ExtSlotName(404))
	}
	if got := extIn(big, 404, 407); !reflect.DeepEqual(got, []int{407}) {
		t.Errorf("bigkill-on-bomb: top impact %v, want [407] (%s)", got, feat.ExtSlotName(407))
	}
	cb, bb := extIn(cheap, 424, 427), extIn(big, 424, 427)
	if len(cb) != 1 || len(bb) != 1 {
		t.Fatalf("counter context slots %v / %v, want one each", cb, bb)
	}
	if cb[0] >= bb[0] {
		t.Errorf("counter context cheap %d not below big %d", cb[0]-424, bb[0]-424)
	}
	for _, sc := range []*FeatScenario{by["cheapkill-on-oneone"], by["bigkill-on-bomb"]} {
		if off := encStack(sc.Pool, 0, sc.State, &sc.Dec, sc.Seat); len(off) != 0 {
			t.Errorf("%s: off Ext has %d entries", sc.Name, len(off))
		}
	}
}

func TestExtRemovalWorthy(t *testing.T) {
	by := stackScenarios(t)
	strength := map[string]uint8{}
	for _, c := range []struct {
		name   string
		worthy bool
	}{{"removal-low-only", false}, {"removal-bomb-target", true}} {
		sc := by[c.name]
		p := new(dirac.Projection)
		sc.State.Project(p)
		removal := false
		for _, a := range sc.Dec.Options[:sc.Dec.N] {
			if a.Kind() != dirac.AkCast {
				continue
			}
			o := &p.Objs[a.Source()]
			ef := sc.Pool.Effect(o.Card)
			if o.Zone == kop.ZoneHand && (ef.Class == feat.EffKill || ef.Class == feat.EffDamage || ef.Class == feat.EffDebuff) && ef.TgtCreat {
				removal = true
			}
		}
		if !removal {
			t.Fatalf("%s: no castable removal in hand", c.name)
		}
		oppCreatures := 0
		for i := 0; i < int(p.NObj); i++ {
			o := &p.Objs[i]
			if o.Zone == kop.ZoneBattlefield && o.Ctrl != sc.Seat && sc.State.Table().Cards[o.Card].Types&kop.TCreature != 0 {
				oppCreatures++
				strength[c.name] = feat.StrengthBucket(sc.Pool.PrintedStrength(o.Card))
			}
		}
		if oppCreatures != 1 {
			t.Fatalf("%s: %d opponent creatures, want 1", c.name, oppCreatures)
		}
		on, _ := extImpactOn(sc)
		_, w := extSlot(on, 408)
		_, l := extSlot(on, 409)
		if w != c.worthy || l == c.worthy {
			t.Errorf("%s: worthy=%v low-only=%v, want worthy=%v low-only=%v: %v", c.name, w, l, c.worthy, !c.worthy, on)
		}
	}
	if strength["removal-low-only"] >= strength["removal-bomb-target"] {
		t.Errorf("target strength buckets %d (low-only) and %d (bomb-target) do not differ", strength["removal-low-only"], strength["removal-bomb-target"])
	}
}

func TestExtCounteredBombImpact(t *testing.T) {
	by := stackScenarios(t)
	sc := by["bomb-on-stack"]
	p, _ := topEntry(t, sc)
	if !extCastableClass(t, sc, feat.EffCounter) {
		t.Fatal("bomb-on-stack: no castable counter")
	}
	x, enc := extImpactOn(sc)
	if got := extIn(x, 404, 407); !reflect.DeepEqual(got, []int{407}) {
		t.Errorf("bomb-on-stack: top impact %v, want [407]", got)
	}
	if got := extIn(x, 424, 427); !reflect.DeepEqual(got, []int{427}) {
		t.Errorf("bomb-on-stack: counter strength %v, want [427]", got)
	}
	opp := 1 - int(sc.Seat)
	wantHand := 428 + int(min(int(p.HandN[opp]), 4))
	if got := extIn(x, 428, 432); !reflect.DeepEqual(got, []int{wantHand}) {
		t.Errorf("bomb-on-stack: opp-hand slots %v, want [%d]", got, wantHand)
	}
	avail := extDebugAvail(t, enc, opp)
	wantOpen := 433 + min(avail, 4)
	if got := extIn(x, 433, 437); !reflect.DeepEqual(got, []int{wantOpen}) {
		t.Errorf("bomb-on-stack: opp-open slots %v, want [%d] (avail %d)", got, wantOpen, avail)
	}

	ctr := by["bomb-countered"]
	var pc dirac.Projection
	ctr.State.Project(&pc)
	if pc.NStack != 0 {
		t.Fatalf("bomb-countered: stack %d, want empty", pc.NStack)
	}
	y, _ := extImpactOn(ctr)
	if got := extIn(y, 400, 437); len(got) != 0 {
		t.Errorf("bomb-countered: I slots set: %v", got)
	}

	pre := by["bomb-precast"]
	var pp dirac.Projection
	pre.State.Project(&pp)
	if pp.NStack != 0 || !extCastableClass(t, pre, feat.EffCounter) {
		t.Fatalf("bomb-precast: stack %d, castable counter %v", pp.NStack, extCastableClass(t, pre, feat.EffCounter))
	}
	z, _ := extImpactOn(pre)
	if got := extIn(z, 441, 441); !reflect.DeepEqual(got, []int{441}) {
		t.Errorf("bomb-precast: 441 slots %v, want [441]", got)
	}
	if got := extIn(z, 404, 437); len(got) != 0 {
		t.Errorf("bomb-precast: stack-context slots set: %v", got)
	}
}

func TestExtImpactEqualAcrossPaths(t *testing.T) {
	by := stackScenarios(t)
	hits := 0
	for _, n := range FeatScenarioNames {
		sc := by[n]
		mk := func() *feat.Encoder {
			e := feat.NewEncoder(sc.Pool)
			e.SetExt(feat.ExtAll)
			return e
		}
		x := append([]feat.Feature(nil), mk().EncodeStateFor(sc.State, &sc.Dec, sc.Seat).Ext...)
		y := append([]feat.Feature(nil), mk().EncodeStateIncFor(sc.State, &sc.Dec, sc.Seat).Ext...)
		z := mk().EncodeParts(sc.State, &sc.Dec, sc.Seat).Ext
		if !reflect.DeepEqual(x, y) || !reflect.DeepEqual(x, z) {
			t.Errorf("%s: Ext differs across paths:\nfull  %v\ninc   %v\nparts %v", n, x, y, z)
		}
		if extAny(x, 400, 441) {
			hits++
		}
	}
	if hits < 6 {
		t.Fatalf("only %d scenarios set an I slot", hits)
	}
}
