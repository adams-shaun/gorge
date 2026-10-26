package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// cr-g122: a battlefield ChangeZone SubAbility$ link's target is announced as
// the spell is cast or the ability activated (CR 601.2c, 602.2b), not asked
// mid-resolution after the earlier instructions have already happened. The
// cards are the corpus scripts; the permanents are inline fixtures.

const g122Land = "Name:Tin Plains\nTypes:Land\nOracle:x\n"

// g122Put places a fixture permanent under any seat (newFixtureDeck seeds
// only seat 0's deck).
func g122Put(t *testing.T, e *Engine, seat state.PlayerID, src string) state.ObjID {
	t.Helper()
	id := e.G.AddObject(card(t, src), seat).ID
	e.emit(events.Event{Kind: events.MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield || o.Controller != seat {
		t.Fatalf("fixture %d is not a battlefield permanent of seat %d: %+v", id, seat, o)
	}
	return id
}

// g122PickRoot answers the pending root KTarget ask with the object.
func g122PickRoot(t *testing.T, e *Engine, id state.ObjID) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind == "cast_sub" {
		t.Fatalf("pending = %+v, want the ROOT target ask", d)
	}
	i := targetOptionIndex(d, id)
	if i < 0 {
		t.Fatalf("object %d is not offered by the root ask: %+v", id, d.Options)
	}
	submitChoices(t, e, i)
}

// g122SubOffer returns the cast_sub ask's offered objects.
func g122SubOffer(d *decision.Decision) map[state.ObjID]bool {
	out := map[state.ObjID]bool{}
	for _, o := range d.Options {
		out[o.Obj] = true
	}
	return out
}

func TestFieryAnnihilationEquipmentTargetedOnCast(t *testing.T) {
	for _, tc := range []struct {
		name       string
		announce   bool
		wantBlade  state.Zone
		wantHelmAt state.Zone
	}{
		{"one_equipment", true, state.ZExile, state.ZBattlefield},
		{"zero_equipment_is_legal", false, state.ZBattlefield, state.ZBattlefield},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e, _, spell := newFixtureDeck(t, 9122, corpusCardText(t, "f/fiery_annihilation.txt"),
				subTgtBlade, subTgtLoose)
			ox := g122Put(t, e, 1, subTgtOx)
			blade := moveSeeded(t, e, 0, subTgtBlade, state.ZBattlefield)
			loose := moveSeeded(t, e, 0, subTgtLoose, state.ZBattlefield)
			e.emit(events.Event{Kind: events.Attach, Obj: blade, IDs: []state.ObjID{ox}})
			if o := e.G.Obj(blade); o.AttachedTo != ox {
				t.Fatalf("precondition: Tin Blade attached to %d, want the Ox %d", o.AttachedTo, ox)
			}
			addMana(t, e, 0, "CCR")
			submitChoices(t, e, castOptionFor(t, e, spell).Index)
			g122PickRoot(t, e, ox)

			d := castSubAsk(t, e)
			if d.Min != 0 || d.Max != 1 {
				t.Fatalf("Equipment ask bounds %d..%d, want 0..1 (up to one)", d.Min, d.Max)
			}
			if off := g122SubOffer(d); len(off) != 1 || !off[blade] || off[loose] {
				t.Fatalf("Equipment offer %+v, want exactly the Equipment attached to the Ox", d.Options)
			}
			if tc.announce {
				answerCastSubObj(t, e, blade)
			} else {
				submitChoices(t, e)
			}
			if so := e.G.Obj(spell); so == nil || so.Zone != state.ZStack {
				t.Fatalf("spell %+v should be on the stack once its targets are announced", so)
			}
			passUntilStackEmpty(t, e, 20)

			if z := e.G.Obj(ox).Zone; z != state.ZExile {
				t.Fatalf("damaged Ox zone = %v, want exile (dies-instead replacement)", z)
			}
			if z := e.G.Obj(blade).Zone; z != tc.wantBlade {
				t.Fatalf("Equipment zone = %v, want %v", z, tc.wantBlade)
			}
			if z := e.G.Obj(loose).Zone; z != state.ZBattlefield {
				t.Fatalf("unrelated Equipment zone = %v, want battlefield", z)
			}
		})
	}
}

func TestIntoTheFloodMawGiftTargetOnCast(t *testing.T) {
	for _, promised := range []bool{true, false} {
		name := "declined"
		if promised {
			name = "promised"
		}
		t.Run(name, func(t *testing.T) {
			e, _, spell := newFixtureDeck(t, 9123, corpusCardText(t, "i/into_the_flood_maw.txt"))
			theirBear := bearPermanent(t, e, 1)
			theirBlade := g122Put(t, e, 1, subTgtBlade)
			ownBear := bearPermanent(t, e, 0)
			addMana(t, e, 0, "U")
			if !castOffered(e, spell) {
				t.Fatal("precondition: Into the Flood Maw not offered")
			}
			submitChoices(t, e, castCardOption(t, e, spell).Index)
			answerGift(t, e, promised)

			if !promised {
				// The root's creature is the only target: no second ask.
				g122PickRoot(t, e, theirBear)
				if d := e.Pending(); d == nil || d.Kind != decision.KPriority {
					t.Fatalf("declined gift: pending = %+v, want priority (no second target)", d)
				}
				passUntilStackEmpty(t, e, 20)
				if z := e.G.Obj(theirBear).Zone; z != state.ZHand {
					t.Fatalf("declined gift: creature zone = %v, want hand", z)
				}
				if z := e.G.Obj(theirBlade).Zone; z != state.ZBattlefield {
					t.Fatalf("declined gift: the nonland permanent zone = %v, want untouched", z)
				}
				return
			}
			d := castSubAsk(t, e)
			if off := g122SubOffer(d); len(off) != 2 || !off[theirBear] || !off[theirBlade] || off[ownBear] {
				t.Fatalf("promised gift: offer %+v, want the opponent's two nonland permanents", d.Options)
			}
			if d.Min != 1 || d.Max != 1 {
				t.Fatalf("promised gift: bounds %d..%d, want 1..1", d.Min, d.Max)
			}
			answerCastSubObj(t, e, theirBlade)
			passUntilStackEmpty(t, e, 20)
			if z := e.G.Obj(theirBlade).Zone; z != state.ZHand {
				t.Fatalf("promised gift: nonland permanent zone = %v, want hand", z)
			}
			if z := e.G.Obj(theirBear).Zone; z != state.ZBattlefield {
				t.Fatalf("promised gift: unannounced creature zone = %v, want battlefield", z)
			}
		})
	}
}

func TestGripOfDesolationLandTargetOnCast(t *testing.T) {
	t.Run("both_announced_on_cast", func(t *testing.T) {
		e, _, spell := newFixtureDeck(t, 9124, corpusCardText(t, "g/grip_of_desolation.txt"))
		ox := g122Put(t, e, 1, subTgtOx)
		land1 := g122Put(t, e, 1, g122Land)
		land2 := g122Put(t, e, 1, g122Land)
		if land1 == land2 {
			t.Fatal("precondition: two distinct lands")
		}
		addMana(t, e, 0, "CCCCBB")
		submitChoices(t, e, castOptionFor(t, e, spell).Index)
		g122PickRoot(t, e, ox)
		d := castSubAsk(t, e)
		if off := g122SubOffer(d); !off[land1] || !off[land2] || off[ox] {
			t.Fatalf("land ask offers %+v, want the lands only", d.Options)
		}
		answerCastSubObj(t, e, land2)
		passUntilStackEmpty(t, e, 20)
		if z := e.G.Obj(ox).Zone; z != state.ZExile {
			t.Fatalf("creature zone = %v, want exile", z)
		}
		if z := e.G.Obj(land2).Zone; z != state.ZExile {
			t.Fatalf("announced land zone = %v, want exile", z)
		}
		if z := e.G.Obj(land1).Zone; z != state.ZBattlefield {
			t.Fatalf("other land zone = %v, want battlefield", z)
		}
	})
	t.Run("no_land_is_not_offered", func(t *testing.T) {
		// Mountains in hand/library only: the battlefield holds a creature
		// and no land, so the mandatory land link has no legal pool.
		e, _, spell := newFixtureDeck(t, 9125, corpusCardText(t, "g/grip_of_desolation.txt"))
		g122Put(t, e, 1, subTgtOx)
		for _, side := range []state.PlayerID{0, 1} {
			for _, id := range e.G.Zone(state.ZBattlefield, side) {
				if e.G.Obj(id).Face().IsLand() {
					t.Fatalf("precondition: land %d on the battlefield", id)
				}
			}
		}
		addMana(t, e, 0, "CCCCBB")
		if castOffered(e, spell) {
			t.Fatal("Grip of Desolation offered with no land to target (CR 601.2c, 115.1)")
		}
		if o := e.G.Obj(spell); o.Zone != state.ZHand {
			t.Fatalf("precondition: spell zone %v, want hand", o.Zone)
		}
		// Control: the same board plus one land makes the spell castable, so
		// the withheld offer above is the land link's doing and not the pool.
		g122Put(t, e, 1, g122Land)
		e.priorityRound()
		if !castOffered(e, spell) {
			t.Fatal("control: Grip of Desolation not offered with a creature and a land to target")
		}
	})
}

func TestKayaSpiritsJusticePerPlayerTargetsOnActivation(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	kaya := mustCorpusCard(t, reg, "Kaya, Spirits' Justice")
	bears := mustCorpusCard(t, reg, "Grizzly Bears")
	elves := mustCorpusCard(t, reg, "Llanowar Elves")
	cfg := seatZeroStart(Config{Seed: 9126, Names: []string{"a", "b", "c"}, Decks: [][]*cards.Card{
		append(mountainDeck(t, 40), kaya, bears),
		append(mountainDeck(t, 40), bears, elves),
		append(mountainDeck(t, 40), bears),
	}})
	e := New(cfg)
	e.Advance()
	kayaID := moveSeededCard(t, e, 0, kaya, state.ZBattlefield)
	mine := moveSeededCard(t, e, 0, bears, state.ZBattlefield)
	elf1 := moveSeededCard(t, e, 1, elves, state.ZBattlefield)
	bear1 := moveSeededCard(t, e, 1, bears, state.ZBattlefield)
	bear2 := moveSeededCard(t, e, 2, bears, state.ZBattlefield)
	if o := e.G.Obj(kayaID); o == nil || o.Zone != state.ZBattlefield || o.Counter("LOYALTY") < 2 {
		t.Fatalf("precondition: Kaya needs loyalty for -2: %+v", o)
	}
	ab := kaya.Faces[0].Abilities
	if len(ab) < 3 || ab[2].Sub == nil || ab[2].Sub.Params["TargetsForEachPlayer"] != "True" {
		t.Fatalf("precondition: Kaya's -2 shape changed: %+v", ab)
	}
	e.priorityRound()
	submitChoices(t, e, abilityOption(t, e, kayaID, 2).Index)
	g122PickRoot(t, e, mine)

	d := castSubAsk(t, e)
	if d.Min != 0 || d.Max != 2 {
		t.Fatalf("per-player ask bounds %d..%d, want 0..2 (one per other player)", d.Min, d.Max)
	}
	off := g122SubOffer(d)
	if len(off) != 3 || !off[elf1] || !off[bear1] || !off[bear2] || off[mine] {
		t.Fatalf("per-player offer %+v, want the three creatures the other players control", d.Options)
	}
	idx := map[state.ObjID]int{}
	grp := map[state.ObjID]string{}
	for _, o := range d.Options {
		idx[o.Obj], grp[o.Obj] = o.Index, o.Group
	}
	if grp[elf1] == "" || grp[elf1] != grp[bear1] || grp[elf1] == grp[bear2] {
		t.Fatalf("groups %v, want one group per other player", grp)
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx[elf1], idx[bear1]}}); err == nil {
		t.Fatal("two creatures of one player were accepted")
	}
	submitChoices(t, e, idx[elf1], idx[bear2])
	passUntilStackEmpty(t, e, 20)
	for id, want := range map[state.ObjID]state.Zone{mine: state.ZExile, elf1: state.ZExile, bear2: state.ZExile, bear1: state.ZBattlefield} {
		if z := e.G.Obj(id).Zone; z != want {
			t.Fatalf("object %d zone = %v, want %v", id, z, want)
		}
	}
	replayCheck(t, e, cfg)
}
