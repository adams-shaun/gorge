package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Omen lockstep (CR 720): constructed matchups over the generated omen
// pool. The decks carry the instant and sorcery Omens, the {X} spells whose
// smaller X leaves the mana a face cast needs floating, counterspells and
// non-flying creatures for the faces that target them.
//
// The counters come from gorge's event log and decisions. The mutations
// break the rest or the shuffle and must diverge.
var komenDecks = [2][]cardset.Entry{
	{
		{Name: "Twinmaw Stormbrood", Count: 2}, {Name: "Riling Dawnbreaker", Count: 2},
		{Name: "Stormshriek Feral", Count: 2}, {Name: "Feral Deathgorger", Count: 2},
		{Name: "Purging Stormbrood", Count: 2}, {Name: "Blaze", Count: 4},
		{Name: "Exsanguinate", Count: 4}, {Name: "Stream of Life", Count: 3},
		{Name: "Savannah Lions", Count: 2}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 5}, {Name: "Mountain", Count: 6}, {Name: "Swamp", Count: 4},
	},
	{
		{Name: "Dirgur Island Dragon", Count: 2}, {Name: "Marang River Regent", Count: 2},
		{Name: "Runescale Stormbrood", Count: 2}, {Name: "Sagu Wildling", Count: 2},
		{Name: "Disruptive Stormbrood", Count: 2}, {Name: "Whirlwing Stormbrood", Count: 2},
		{Name: "Mind Spring", Count: 4}, {Name: "Stream of Life", Count: 3},
		{Name: "Negate", Count: 2}, {Name: "Counterspell", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Island", Count: 6},
		{Name: "Forest", Count: 5}, {Name: "Swamp", Count: 4},
	},
}

// omenPolicy prefers an Omen face cast, then a Negate/Counterspell answering
// an opponent's Omen spell, else preparePolicy.
func omenPolicy(seed uint64) Policy {
	base := preparePolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActCastFace && a.Mode()&15 == faceCodeOmen {
				return a
			}
		}
		if c.G != nil {
			g := c.G.E.G
			p := state.PlayerID(c.D.Player)
			if omenOnTopByOpponent(g, p) {
				for _, a := range c.D.Options {
					if a.Kind() != ActCast {
						continue
					}
					if o := canonObj(c, a.Src()); o != nil && o.Face() != nil &&
						(o.Face().Name == "Negate" || o.Face().Name == "Counterspell") {
						return a
					}
				}
			}
		}
		return base.Pick(c)
	})
}

// canonObj maps a canonical source ref back to the gorge object.
func canonObj(c *PickCtx, r Ref) *state.Object {
	if c.G == nil || int(r) >= len(c.G.id.obj) {
		return nil
	}
	return c.G.E.G.Obj(c.G.id.obj[r])
}

// hasOmenSorceryInHand reports whether p holds an Omen card whose face is a
// sorcery.
func hasOmenSorceryInHand(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZHand, p) {
		o := g.Obj(id)
		if o == nil || o.Card == nil || o.Card.AlternateMode != "Omen" || len(o.Card.Faces) != 2 {
			continue
		}
		if o.Card.Faces[1].IsSorcery() {
			return true
		}
	}
	return false
}

// controlsWhirlwing reports whether p controls Whirlwing Stormbrood (its
// flash lets the sorcery face be cast at instant speed).
func controlsWhirlwing(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZBattlefield, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Whirlwing Stormbrood" {
			return true
		}
	}
	return false
}

// omenOnTopByOpponent reports whether the top stack object is an Omen face
// cast by someone other than p.
func omenOnTopByOpponent(g *state.Game, p state.PlayerID) bool {
	if len(g.Stack) == 0 {
		return false
	}
	o := g.Obj(g.Stack[len(g.Stack)-1])
	return o != nil && o.Card != nil && o.Card.AlternateMode == "Omen" &&
		o.FaceIdx == 1 && o.Controller != p
}

// omenTally counts the Omen routes. The cumulative counters survive across
// games; the frame/ref state is per game.
type omenTally struct {
	autopay bool
	ref     *GorgeSide
	frames  map[uint64]bool

	casts, instCasts, whirlwingFlash   int
	toLib, roostDouble, libNoShuffle   int
	libBadFace                         int
	toGrave, counterMoves, fizzleMoves int
	graveShuffle, graveBadFace         int
	halfCasts, halfToBF                int
	withheld, offeredWithheld          int
	autopayOffers                      int
	emptyOffer                         map[state.ObjID]bool
	emptyOffers, emptyTaken            int
	targetedOmens, runescaleTriggers   int
	reversals, redrawn                 int
	perCard                            map[string]int
}

// start resets the per-game tracking.
func (o *omenTally) start(cfg Config) {
	o.autopay = cfg.GorgeAutopaySurface
	o.ref = nil
	o.frames = map[uint64]bool{}
	o.emptyOffer = map[state.ObjID]bool{}
	if o.perCard == nil {
		o.perCard = map[string]int{}
	}
}

// check counts priority frames: a sorcery Omen face is withheld when it is
// not sorcery timing, the actor holds such an Omen and controls no
// Whirlwing.
func (o *omenTally) check(ref *GorgeSide, k Side) *Divergence {
	o.ref = ref
	fr := ref.fr
	if fr == nil || fr.d == nil || fr.d.Kind != "priority" || o.frames[fr.seq] {
		return nil
	}
	o.frames[fr.seq] = true
	g := ref.E.G
	actor := state.PlayerID(fr.d.Player)
	offeredSorc := false
	for _, r := range kernelFaceRefs(k, faceCodeOmen) {
		if o.autopay {
			o.autopayOffers++
		}
		id := ref.id.obj[r]
		if g.Players[actor].Pool.Total() == 0 {
			o.emptyOffers++
			o.emptyOffer[id] = true
		}
		if co := g.Obj(id); co != nil && co.Card != nil && len(co.Card.Faces) == 2 &&
			co.Card.Faces[1].IsSorcery() {
			offeredSorc = true
		}
	}
	sorc := g.Active == actor && g.Step.IsMain() && len(g.Stack) == 0
	if sorc || !hasOmenSorceryInHand(g, actor) || controlsWhirlwing(g, actor) {
		return nil
	}
	o.withheld++
	if offeredSorc {
		o.offeredWithheld++
	}
	return nil
}

// scan reads the finished game's event log. It runs once per game, after the
// last decision, so adjacency checks have the whole log.
func (o *omenTally) scan() {
	if o.ref == nil {
		return
	}
	evs := o.ref.E.L.Events
	g := o.ref.E.G
	omenCast := map[state.ObjID]bool{}
	halfCast := map[state.ObjID]bool{}
	resolving := map[state.ObjID]int{}
	shuffled := map[state.ObjID]bool{}
	active := state.PlayerID(0)
	step := state.Step(0)
	stack, shuffles := 0, 0
	for i := range evs {
		e := &evs[i]
		switch e.Kind {
		case events.TurnChange:
			active = e.Player
		case events.StepChange:
			step = e.Step
		case events.Shuffle:
			shuffles++
		case events.PutOnStack:
			obj := g.Obj(e.Obj)
			if obj != nil && obj.Card != nil && obj.Card.AlternateMode == "Omen" && len(obj.Card.Faces) == 2 {
				f1 := obj.Card.Faces[1]
				if e.Text == f1.Name {
					delete(halfCast, e.Obj)
					omenCast[e.Obj] = true
					o.casts++
					if o.emptyOffer[e.Obj] {
						o.emptyTaken++
					}
					o.perCard[obj.Card.Faces[0].Name]++
					sorc := active == obj.Controller && step.IsMain() && stack == 0
					if f1.IsInstant() && !sorc {
						o.instCasts++
					}
					if f1.IsSorcery() && obj.Card.Faces[0].Name == "Whirlwing Stormbrood" && !sorc {
						o.whirlwingFlash++
					}
				} else {
					delete(omenCast, e.Obj)
					halfCast[e.Obj] = true
					o.halfCasts++
				}
			}
			stack++
		case events.Resolve:
			if omenCast[e.Obj] {
				resolving[e.Obj] = shuffles
			}
		case events.MoveZone:
			if e.From == state.ZStack {
				if stack > 0 {
					stack--
				}
				obj := g.Obj(e.Obj)
				switch {
				case e.To == state.ZLibrary && omenCast[e.Obj]:
					o.toLib++
					owner := state.PlayerID(0)
					if obj != nil {
						owner = obj.Owner
					}
					if i+1 < len(evs) {
						n := &evs[i+1]
						if n.Kind != events.Shuffle || n.Player != owner {
							o.libNoShuffle++
						}
					} else {
						o.libNoShuffle++
					}
					if base, ok := resolving[e.Obj]; ok && shuffles > base {
						o.roostDouble++
					}
					if obj != nil && obj.FaceIdx != 0 {
						o.libBadFace++
					}
					shuffled[e.Obj] = true
				case e.To == state.ZGraveyard && omenCast[e.Obj]:
					o.toGrave++
					switch {
					case strings.HasPrefix(e.Text, "countered"):
						o.counterMoves++
					case strings.HasPrefix(e.Text, "fizzled"):
						o.fizzleMoves++
					}
					if i+1 < len(evs) && evs[i+1].Kind == events.Shuffle {
						o.graveShuffle++
					}
					if obj != nil && obj.FaceIdx != 0 {
						o.graveBadFace++
					}
				}
			}
			if e.To == state.ZBattlefield && halfCast[e.Obj] {
				o.halfToBF++
			}
		case events.SpellCastReversed:
			o.reversals++
		case events.Draw:
			if shuffled[e.Obj] {
				o.redrawn++
			}
		case events.TargetsChosen:
			for _, id := range e.IDs {
				if omenCast[id] {
					o.targetedOmens++
					break
				}
			}
		case events.TriggerPush:
			if src := g.Obj(e.Obj); src != nil && src.Face() != nil &&
				src.Face().Name == "Runescale Stormbrood" {
				o.runescaleTriggers++
			}
		}
	}
}

func komenSetup(t *testing.T) (*cards.Registry, cardset.Set, [2][]*cards.Card, *kop.Table) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("omen")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range komenDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(komenDecks[0], komenDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, decks, tbl
}

func komenRun(t *testing.T, reg *cards.Registry, set cardset.Set, decks [2][]*cards.Card, tbl *kop.Table, seed uint64, cfg Config, tally *omenTally) Result {
	t.Helper()
	m, err := NewMatchup(reg, [2]string{"omen-a", "omen-b"}, decks, 1)
	if err != nil {
		t.Fatal(err)
	}
	k, err := SetSide(set, tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	pol := omenPolicy(seed)
	if seed%4 == 1 {
		pol = RandomPolicy(seed)
	}
	var check func(*GorgeSide, Side) *Divergence
	if tally != nil {
		tally.start(cfg)
		check = tally.check
	}
	res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 5000, MaxTurns: 80, Check: check})
	if tally != nil {
		tally.scan()
	}
	CloseSide(k)
	return res
}

// TestOmenLockstep: the kernel matches gorge over the Omen pool, and every
// rest route is reached.
func TestOmenLockstep(t *testing.T) {
	t.Parallel()

	reg, set, decks, tbl := komenSetup(t)
	var tally omenTally
	casts := 0
	const seeds = 256
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			// the autopay surface has no floating mana to pay a face with
			cfg = Config{GorgeAutopaySurface: true}
		}
		res := komenRun(t, reg, set, decks, tbl, seed, cfg, &tally)
		if res.Stall != "" {
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		for _, a := range res.Actions {
			if a.Kind() == ActCastFace && a.Mode()&15 == faceCodeOmen {
				casts++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds: %d Omen face casts; Omen casts %d (%d instant at non-sorcery timing, %d Whirlwing sorcery flash); resolved to library %d (%d Roost double shuffle, %d not front, %d without owner shuffle); countered %d, fizzled %d (%d shuffle after grave move); creature half cast %d (%d reached battlefield); sorcery face withheld %d frames (%d offered omen_alt, %d autopay offers); empty-pool omen_alt offers %d, taken %d; H4 targeted Omens %d, Runescale triggers %d; H3 Exude Toxin 0 (not in pool); reversals %d; Omens drawn again %d",
		seeds, casts, tally.casts, tally.instCasts, tally.whirlwingFlash,
		tally.toLib, tally.roostDouble, tally.libBadFace, tally.libNoShuffle,
		tally.counterMoves, tally.fizzleMoves, tally.graveShuffle,
		tally.halfCasts, tally.halfToBF, tally.withheld, tally.offeredWithheld, tally.autopayOffers,
		tally.emptyOffers, tally.emptyTaken,
		tally.targetedOmens, tally.runescaleTriggers, tally.reversals, tally.redrawn)
	for _, c := range []struct {
		name string
		got  int
	}{
		{"Omen casts", tally.casts},
		{"resolved to library", tally.toLib},
		{"countered or fizzled to graveyard", tally.counterMoves + tally.fizzleMoves},
		{"creature half cast", tally.halfCasts},
		{"sorcery face withheld", tally.withheld},
		{"instant-face Omen casts at non-sorcery timing", tally.instCasts},
		{"empty-pool omen_alt offers", tally.emptyOffers},
		{"empty-pool omen_alt offers taken", tally.emptyTaken},
	} {
		want := 10
		if c.name == "instant-face Omen casts at non-sorcery timing" {
			want = 5
		}
		if c.got < want {
			t.Errorf("%s: %d, want at least %d", c.name, c.got, want)
		}
	}
	// Every carrier in the pool must actually fire: a face cast whose Omen
	// face never reaches the stack proves nothing. Derive the list from the
	// table so a carrier added to the pool is covered without editing here.
	var carriers []string
	for _, c := range tbl.Cards {
		if c.FaceMode == kop.FaceOmen && c.Face == 0 {
			carriers = append(carriers, c.Name)
		}
	}
	t.Logf("per-carrier Omen face casts: %v", tally.perCard)
	for _, name := range carriers {
		if got := tally.perCard[name]; got < 1 {
			t.Errorf("carrier %s: %d Omen face casts, want at least 1", name, got)
		}
	}
	for _, c := range []struct {
		name string
		got  int
	}{
		{"omen_alt offered in a withheld frame", tally.offeredWithheld},
		{"library Omen move not followed by its owner's shuffle", tally.libNoShuffle},
		{"shuffle right after a countered or fizzled Omen", tally.graveShuffle},
		{"library Omen move not at the front face", tally.libBadFace},
		{"graveyard Omen move not at the front face", tally.graveBadFace},
	} {
		if c.got != 0 {
			t.Errorf("%s: %d, want 0", c.name, c.got)
		}
	}
}

// TestOmenMutation: each mutation must diverge on some seed.
func TestOmenMutation(t *testing.T) {
	reg, set, decks, tbl := komenSetup(t)
	muts := []struct {
		name string
		on   *bool
	}{
		{"resolved Omen to the graveyard", &dirac.DebugOmenGraveyard},
		{"resolved Omen without a shuffle", &dirac.DebugOmenNoShuffle},
		{"resolved Omen on top", &dirac.DebugOmenTop},
		{"face cast offered only when the pool pays", &dirac.DebugFacePoolOnly},
	}
	for _, mu := range muts {
		*mu.on = true
		diverged := false
		for seed := uint64(1); seed <= 256 && !diverged; seed++ {
			cfg := Config{}
			if seed%4 == 0 {
				cfg = Config{GorgeAutopaySurface: true}
			}
			res := komenRun(t, reg, set, decks, tbl, seed, cfg, nil)
			if diverged = res.Div != nil; diverged {
				t.Logf("mutation %q: seed %d diverged at decision %d: %s %s", mu.name, seed, res.Div.At, res.Div.What, res.Div.Field)
			}
		}
		*mu.on = false
		if !diverged {
			t.Errorf("mutation %q never diverged", mu.name)
		}
	}
}
