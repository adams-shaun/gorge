package oracle

import (
	"fmt"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestHostedCmdrManaShapesLockstep lowers the hosted-Commander mana shapes
// whose Forge lines kgen used to refuse. This first case is Virtue of
// Strength's ProduceMana replacement:
//
//	R:Event$ ProduceMana | ActiveZones$ Battlefield | ValidActivator$ You |
//	  ValidCard$ Land.Basic | ReplaceWith$ Triple
//	SVar:Triple:DB$ ReplaceMana | ReplaceAmount$ 3
//
// kgen lowers the exact shape to the kernel's tap-extra static in reflected
// mode (kop.StTapReflectDefer, TapAmt 2): the tapped basic land makes its own
// mana plus two more of the same type. The plan prices the source at its own
// production only (gorge does not fold the replacement) and the extra floats
// at the tap (Virtue Probe, the inline copy of the line, is cast first).
//
// The probe is {5}{G}{G}. Seven nonbasic Grove Probes (not Land.Basic, so the
// replacement does not reach them) pay for it; the deck's only Forest is kept
// in hand until the probe is on the battlefield, then played and tapped for a
// Grizzly Bears. That leaves exactly one green source untapped, so the payment
// plan has no source-choice tie-break to differ on: the lockstep compares the
// replacement's three mana itself. The case FAILS while kgen refuses the shape
// (printedDelveSetIn prints the table) and FAILS unless some seed casts the
// probe and then gets the Forest to produce three mana, so a passing lockstep
// is not vacuous. The real Virtue of Strength is checked by the lowering test
// in dirac/kgen (its split-card file name keeps it out of this corpus).
const virtueProbeScript = "Name:Virtue Probe\nManaCost:5 G G\nTypes:Enchantment\n" +
	"R:Event$ ProduceMana | ActiveZones$ Battlefield | ValidActivator$ You | ValidCard$ Land.Basic | ReplaceWith$ Triple | Description$ If you tap a basic land for mana, it produces three times as much of that mana instead.\n" +
	"SVar:Triple:DB$ ReplaceMana | ReplaceAmount$ 3\n" +
	"Oracle:If you tap a basic land for mana, it produces three times as much of that mana instead.\n"

const cubProbeScript = "Name:Cub Probe\nManaCost:G\nTypes:Creature Bear\nPT:2/2\n" +
	"Oracle:2/2\n"

const groveProbeScript = "Name:Grove Probe\nTypes:Land\n" +
	"A:AB$ Mana | Cost$ T Sac<1/CARDNAME/this land> | Produced$ G | SpellDescription$ Add {G}.\n" +
	"Oracle:{T}, Sacrifice this land: Add {G}.\n"

func TestHostedCmdrManaShapesLockstep(t *testing.T) {
	dir := filters11Corpus(t, map[string]string{"Virtue Probe": virtueProbeScript, "Grove Probe": groveProbeScript, "Cub Probe": cubProbeScript},
		"Forest")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	decks := [2][]string{
		append(deckOf(4, "Virtue Probe"), append(deckOf(28, "Cub Probe"),
			append(deckOf(7, "Grove Probe"), "Forest")...)...),
		deckOf(40, "Forest"),
	}
	if len(decks[0]) != 40 || len(decks[1]) != 40 {
		t.Fatalf("deck sizes = %d/%d, want 40/40", len(decks[0]), len(decks[1]))
	}
	var cardDecks [2][]*cards.Card
	for s, names := range decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], card)
		}
	}
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	set := printedDelveSetIn(t, dir, decks)
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	seen := false
	played := 0
	for seed := uint64(1); seed <= 80 && !(seen && played >= 3); seed++ {
		m, err := NewMatchup(reg, [2]string{"hms-probe-a", "hms-probe-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		rc := RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			if !seen && virtueProbeTripled(g) {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, hostedCmdrManaShapesPolicy(m, seed), rc)
		if res.Div != nil {
			d := res.Div
			n := len(res.Actions) - 8
			if n < 0 {
				n = 0
			}
			var names []string
			for _, a := range res.Actions[n:] {
				names = append(names, fmt.Sprintf("%v:%q", a, m.InstanceCard(int(a.Src()))))
			}
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v detail=%q last=%v\ngorge %v\ndirac %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, names, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if played < 3 {
		t.Fatalf("only %d seeds played a real game", played)
	}
	if !seen {
		t.Fatalf("the ProduceMana replacement was not reached on any seed played, so the lockstep is vacuous")
	}
	t.Run("Squandered Resources", hostedCmdrSquanderedLockstep)
	t.Run("Vivi Ornitier", hostedCmdrViviLockstep)
}

const squanderedProbeScript = "Name:Squandered Probe\nManaCost:B G\nTypes:Enchantment\n" +
	"A:AB$ ManaReflected | Cost$ Sac<1/Land> | ColorOrType$ Type | Valid$ Defined.Sacrificed | ReflectProperty$ Produce | SpellDescription$ Sacrifice a land: Add one mana of any type that land could produce.\n" +
	"Oracle:Sacrifice a land: Add one mana of any type that land could produce.\n"

const viviProbeScript = "Name:Vivi Probe\nManaCost:U R\nTypes:Creature Human Wizard\nPT:2/2\n" +
	"A:AB$ Mana | PlayerTurn$ True | ActivationLimit$ 1 | Cost$ 0 | Produced$ Combo U R | Amount$ X | SpellDescription$ During your turn, add X mana in any combination of {U} and/or {R}, where X is this creature's power. Activate only once each turn.\n" +
	"SVar:X:Count$CardPower\n" +
	"Oracle:During your turn, add X mana in any combination of {U} and/or {R}, where X is this creature's power. Activate only once each turn.\n"

// hostedCmdrSquanderedLockstep: seat 0 casts Squandered Probe ({B}{G}) off a
// Forest and a Swamp, then activates it sacrificing the Forest for {G} and
// casts Cub Probe with that mana. The payment is the sacrifice-reflected
// production (ManaSacrificed): both engines must agree on the activation, the
// victim, the colour the victim produces, and the cast.
func hostedCmdrSquanderedLockstep(t *testing.T) {
	dir := filters11Corpus(t, map[string]string{
		"Squandered Probe": squanderedProbeScript,
		"Cub Probe":        cubProbeScript,
	}, "Forest", "Swamp")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	deck0 := deckOf(18, "Squandered Probe")
	deck0 = append(deck0, deckOf(10, "Forest")...)
	deck0 = append(deck0, deckOf(8, "Swamp")...)
	deck0 = append(deck0, deckOf(4, "Cub Probe")...)
	decks := [2][]string{deck0, deckOf(40, "Forest")}
	if len(decks[0]) != 40 || len(decks[1]) != 40 {
		t.Fatalf("deck sizes = %d/%d, want 40/40", len(decks[0]), len(decks[1]))
	}
	var cardDecks [2][]*cards.Card
	for s, names := range decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], card)
		}
	}
	set := printedDelveSetIn(t, dir, decks)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	played, seen := 0, false
	for seed := uint64(1); seed <= 80 && !seen; seed++ {
		m, err := NewMatchup(reg, [2]string{"hms-squa-a", "hms-squa-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var cub bool
		pol := hostedCmdrSquanderedPolicy(m, seed, &cub)
		rc := RunConfig{Config: Config{}, MaxDecisions: 6000, MaxTurns: 40}
		rc.Finish = func(g *GorgeSide) {
			// the cast the line needs is the policy's own; the ability use is
			// gorge's event, never the cast
			if cub && hostedCmdrSquanderedSacrificed(g) && hostedGraveyardHas(g, 0, "Forest", "Swamp") {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, pol, rc)
		if res.Div != nil {
			d := res.Div
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v detail=%q\ngorge %v\ndirac %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if !seen {
		t.Fatalf("the sacrifice-reflected activation and its cast were not reached on any of %d seeds played", played)
	}
	t.Run("variants", hostedCmdrSquanderedVariants)
}

// hostedCmdrSquanderedPolicy plays the scripted line: tap a Forest and a Swamp,
// cast Squandered Probe, activate it (the first activation option), take the
// first sacrifice and colour answer, then cast Cub Probe with the {G}.
func hostedCmdrSquanderedPolicy(m *Matchup, seed uint64, cub *bool) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil || p.D.Player != 0 {
			return rnd.Pick(p)
		}
		if p.D.Kind != DPriority {
			return rnd.Pick(p)
		}
		hasCub := hostedInHand(p.G, 0, "Cub Probe")
		if hasCub {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Cub Probe" {
					*cub = true
					return a
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Squandered Probe" {
				return a
			}
		}
		if hostedInHand(p.G, 0, "Squandered Probe") {
			pool := p.G.E.G.Players[0].Pool
			for _, want := range []string{"Swamp", "Forest"} {
				if want == "Forest" && pool[state.MB] == 0 || want == "Swamp" && pool[state.MB] > 0 {
					continue
				}
				for _, a := range p.D.Options {
					if a.Kind() == ActActivate && m.InstanceCard(int(a.Src())) == want {
						return a
					}
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPlayLand && m.InstanceCard(int(a.Src())) == "Forest" {
				return a
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return rnd.Pick(p)
	})
}

const viviSpellScript = "Name:Vivi Spell\nManaCost:U R\nTypes:Instant\n" +
	"A:SP$ Draw | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\n" +
	"Oracle:Draw a card.\n"

// hostedCmdrViviLockstep: seat 0 casts Vivi Probe ({U}{R}) and then Vivi Spell
// ({U}{R}) with the only untapped source left: Vivi's {0} once-each-turn "add X
// mana in any combination of {U} and/or {R}" (X its power). The activation must
// take the finite {U}/{R} answers, record the once-each-turn bit (a second
// Vivi Spell the same turn must not be offered), and leave Vivi untapped.
func hostedCmdrViviLockstep(t *testing.T) {
	dir := filters11Corpus(t, map[string]string{
		"Vivi Probe": viviProbeScript,
		"Vivi Spell": viviSpellScript,
	}, "Island", "Mountain", "Forest")
	reg, err := cards.SharedCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	deck0 := deckOf(14, "Vivi Probe")
	deck0 = append(deck0, deckOf(10, "Island")...)
	deck0 = append(deck0, deckOf(10, "Mountain")...)
	deck0 = append(deck0, deckOf(6, "Vivi Spell")...)
	decks := [2][]string{deck0, deckOf(40, "Forest")}
	if len(decks[0]) != 40 || len(decks[1]) != 40 {
		t.Fatalf("deck sizes = %d/%d, want 40/40", len(decks[0]), len(decks[1]))
	}
	var cardDecks [2][]*cards.Card
	for s, names := range decks {
		for _, n := range names {
			card, ok := reg.Lookup(n)
			if !ok {
				t.Fatalf("%s not in the corpus", n)
			}
			cardDecks[s] = append(cardDecks[s], card)
		}
	}
	set := printedDelveSetIn(t, dir, decks)
	entries := func(names []string) []cardset.Entry {
		es := make([]cardset.Entry, len(names))
		for i, n := range names {
			es[i] = cardset.Entry{Name: n, Count: 1}
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(decks[0]), entries(decks[1]))
	if err != nil {
		t.Fatal(err)
	}
	played, seen := 0, false
	for seed := uint64(1); seed <= 80 && !seen; seed++ {
		m, err := NewMatchup(reg, [2]string{"hms-vivi-a", "hms-vivi-b"}, cardDecks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var reoffered bool
		var spellTurn int32
		pol := hostedCmdrViviPolicy(m, seed, &reoffered, &spellTurn)
		rc := RunConfig{Config: Config{}, MaxDecisions: 6000, MaxTurns: 40}
		var untapped bool
		rc.Finish = func(g *GorgeSide) {
			untapped = hostedViviUntapped(g)
			// the ability use is gorge's own ManaActivate marker (the
			// ActivationLimit$ scan), never the cast its mana paid for
			if hostedViviActivated(g) && untapped {
				seen = true
			}
		}
		res := Lockstep(m, seed, k, pol, rc)
		if res.Div != nil {
			d := res.Div
			CloseSide(k)
			t.Fatalf("seed %d: divergence at %d: %s %s %v detail=%q\ngorge %v\ndirac %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if reoffered {
			CloseSide(k)
			t.Fatalf("seed %d: Vivi's once-each-turn ability was offered twice in one turn", seed)
		}
		if res.Decisions > 100 {
			played++
		}
		CloseSide(k)
	}
	if !seen {
		t.Fatalf("Vivi's mana ability was not activated on any of %d seeds played", played)
	}
}

// hostedCmdrViviPolicy casts Vivi Probe, then casts Vivi Spell as soon as it
// is offered (the only untapped source is Vivi's ability), and records a
// second offer in the same turn.
func hostedCmdrViviPolicy(m *Matchup, seed uint64, reoffered *bool, spellTurn *int32) Policy {
	rnd := RandomPolicy(seed)
	viviCast := false
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.G == nil || p.D.Player != 0 {
			return rnd.Pick(p)
		}
		if p.D.Kind != DPriority {
			return rnd.Pick(p)
		}
		turn := p.G.E.G.Turn
		onBf := hostedOnBattlefield(p.G, 0, "Vivi Probe")
		if onBf {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Vivi Spell" {
					if *spellTurn == 0 {
						*spellTurn = turn
					} else if *spellTurn == turn {
						*reoffered = true
					}
					return a
				}
			}
		}
		if !viviCast {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Vivi Probe" {
					viviCast = true
					return a
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPlayLand && (m.InstanceCard(int(a.Src())) == "Island" || m.InstanceCard(int(a.Src())) == "Mountain") {
				return a
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return rnd.Pick(p)
	})
}

// hostedViviUntapped reports whether seat 0 controls an untapped Vivi Probe.
func hostedViviUntapped(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Vivi Probe" {
			return !o.Tapped
		}
	}
	return false
}

// hostedViviActivated reports whether seat 0 activated a Vivi Probe's mana
// ability: gorge's ManaActivate marker (the ActivationLimit$ scan), not a
// spell the activation paid for.
func hostedViviActivated(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.ManaActivate {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Vivi Probe" && e.Player == 0 {
			return true
		}
	}
	return false
}

// hostedGraveyardHas reports whether seat p's graveyard holds a card named one
// of names.
func hostedGraveyardHas(g *GorgeSide, p state.PlayerID, names ...string) bool {
	if g == nil || g.E == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZGraveyard, p) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil {
			for _, n := range names {
				if o.Face().Name == n {
					return true
				}
			}
		}
	}
	return false
}

// hostedOnBattlefield reports whether seat p controls a permanent named name.
func hostedOnBattlefield(g *GorgeSide, p state.PlayerID, name string) bool {
	if g == nil || g.E == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, p) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// hostedInHand reports whether seat p holds a card named name.
func hostedInHand(g *GorgeSide, p state.PlayerID, name string) bool {
	if g == nil || g.E == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZHand, p) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == name {
			return true
		}
	}
	return false
}

// virtueProbeTripled: the probe entered the battlefield under seat 0 and a
// later ManaAdd event carried three units (the replacement's rewritten
// production).
func virtueProbeTripled(g *GorgeSide) bool {
	probe := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Virtue Probe" && o.Controller == 0 {
				probe = true
			}
		}
		if probe && e.Kind == events.ManaAdd && e.Amount >= 3 {
			return true
		}
	}
	return false
}

// virtueOnBattlefield reports whether seat 0 controls a Virtue Probe.
func virtueOnBattlefield(g *GorgeSide) bool {
	if g == nil || g.E == nil {
		return false
	}
	for _, id := range g.E.G.Zone(state.ZBattlefield, 0) {
		if o := g.E.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Virtue Probe" {
			return true
		}
	}
	return false
}

// hostedCmdrManaShapesPolicy plays the scripted line: seat 0 casts the probe
// as soon as its seven Grove Probes can pay, then plays its one Forest and
// casts a single Cub Probe with the Forest's (tripled) mana. It never
// casts a second spell, so after the one bear the game only plays lands and
// passes; seat 1's all-Forest deck has nothing to cast. Keeping the Forest in
// hand until the probe is out makes it the only untapped green source for the
// bear, which keeps the payment out of the planners' source-choice tie-break.
func hostedCmdrManaShapesPolicy(m *Matchup, seed uint64) Policy {
	rnd := RandomPolicy(seed)
	cubs := 0
	return PolicyFunc(func(p *PickCtx) Canon {
		if p.D.Kind != DPriority || p.G == nil || p.D.Player != 0 {
			return rnd.Pick(p)
		}
		if !virtueOnBattlefield(p.G) {
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Virtue Probe" {
					return a
				}
			}
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand && m.InstanceCard(int(a.Src())) == "Grove Probe" {
					return a
				}
			}
		} else {
			for _, a := range p.D.Options {
				if a.Kind() == ActPlayLand && m.InstanceCard(int(a.Src())) == "Forest" {
					return a
				}
			}
			if cubs < 1 {
				for _, a := range p.D.Options {
					if a.Kind() == ActCast && m.InstanceCard(int(a.Src())) == "Cub Probe" {
						cubs++
						return a
					}
				}
			}
		}
		for _, a := range p.D.Options {
			if a.Kind() == ActPass {
				return a
			}
		}
		return rnd.Pick(p)
	})
}
