package neutral

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/protocol"
	"github.com/adams-shaun/gorge/server/spellbench/xview"
)

// TestNeutralGrantMatchesNativeAtPendingGrant pins decision.Option.Grant, the
// server-side no-op fact the bot's A1 rule reads: an activated ability whose
// whole effect is a pure keyword grant is redundant while an identical grant
// from the same source is on the stack. The fixture is the recorded choose
// request at the first Fledgling Imp divergence (seed 6063..., index 426):
// the native bot passed because the engine filled Duplicate, while the
// neutral world activated because it built no Grant at all. The session is
// told it answered the activation decision that put the ability on the stack,
// exactly the state a self-played neutral game reaches.
func TestNeutralGrantMatchesNativeAtPendingGrant(t *testing.T) {
	reg := testcorpus.Registry(t)
	raw, err := os.ReadFile("testdata/pending_grant_fledgling_imp.json")
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	sd := q.Decision
	var p xview.Payload
	if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &p); err != nil {
		t.Fatal(err)
	}
	// Precondition: the native decision carries the redundant grant.
	var nativeGrant *decision.Grant
	for _, o := range p.Decision.Options {
		if o.Kind == "ability" && o.Index < len(p.Facts.Options) {
			nativeGrant = p.Facts.Options[o.Index].Grant
		}
	}
	if nativeGrant == nil || !nativeGrant.Duplicate || len(nativeGrant.Keywords) != 1 || nativeGrant.Keywords[0] != "Flying" {
		t.Fatalf("precondition: native grant = %+v, want a pending Flying grant", nativeGrant)
	}
	// Precondition: the observation shows the activated ability on the stack,
	// which is what the session's Duplicate read keys on.
	var srcID string
	for _, e := range sd.Observation.Stack {
		if e.StackKind == "activated_ability" && e.Source != nil && e.Source.CardName != nil &&
			*e.Source.CardName == "Fledgling Imp" {
			srcID = e.Source.ObjectID
		}
	}
	if srcID == "" {
		t.Fatal("precondition: no Fledgling Imp activated ability on the observed stack")
	}

	s := NewSession(reg)
	s.SetAutoPay(true)
	// The previous choose request was that activation; Picked records the
	// source's latest activation and its pure-grant keywords.
	cand := -1
	for i, c := range sd.Candidates {
		if c.Semantic.Kind == "activate_ability" {
			cand = i
		}
	}
	if cand < 0 {
		t.Fatal("precondition: no activate_ability candidate")
	}
	s.last = &sd
	s.Picked(cand)
	pl, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	var got *decision.Grant
	for _, o := range pl.Decision.Options {
		if o.Kind == "ability" && o.Grant != nil {
			got = o.Grant
		}
	}
	if got == nil {
		t.Fatalf("rebuilt ability option carries no Grant; native %+v", nativeGrant)
	}
	if !got.Duplicate || got.Already != nativeGrant.Already || len(got.Keywords) != len(nativeGrant.Keywords) ||
		got.Keywords[0] != nativeGrant.Keywords[0] {
		t.Fatalf("rebuilt grant = %+v, native %+v", got, nativeGrant)
	}
}

// TestNeutralGrantSurvivesCostTargetDecision is the MAJOR finding from review:
// the session pruned its recorded grant on every Payload, so the non-priority
// choose_cost_target decision an activation poses before its ability is on the
// stack (Fledgling Imp's "Discard a card") deleted the grant, and the later
// priority decision rebuilt Duplicate=false. Build that intermediate decision
// from the same fixture's hand (three cards, so the discard cost is really
// posed), assert it translated as a cost ask with the grant still recorded,
// then assert the stack-bearing priority decision still reports Duplicate.
func TestNeutralGrantSurvivesCostTargetDecision(t *testing.T) {
	reg := testcorpus.Registry(t)
	raw, err := os.ReadFile("testdata/pending_grant_fledgling_imp.json")
	if err != nil {
		t.Fatal(err)
	}
	var q struct {
		Decision protocol.SeatDecision `json:"decision"`
	}
	if err := json.Unmarshal(raw, &q); err != nil {
		t.Fatal(err)
	}
	sd := q.Decision
	var p xview.Payload
	if err := json.Unmarshal(sd.Extensions["x_gorge_view_v1"], &p); err != nil {
		t.Fatal(err)
	}
	var nativeGrant *decision.Grant
	for _, o := range p.Decision.Options {
		if o.Kind == "ability" && o.Index < len(p.Facts.Options) {
			nativeGrant = p.Facts.Options[o.Index].Grant
		}
	}
	if nativeGrant == nil || !nativeGrant.Duplicate {
		t.Fatalf("precondition: native grant = %+v, want Duplicate", nativeGrant)
	}

	// The activation candidate names the source and ability; the hand must
	// hold more than one card or gorge would not pose the discard cost.
	var src protocol.ObjectRef
	cand := -1
	for i, c := range sd.Candidates {
		if c.Semantic.Kind == "activate_ability" {
			src, err = ref(c.Semantic, "source")
			if err != nil {
				t.Fatal(err)
			}
			cand = i
		}
	}
	if cand < 0 {
		t.Fatal("precondition: no activate_ability candidate")
	}
	var hand []protocol.ObjectRecord
	for _, pl := range sd.Observation.Players {
		if pl.Seat == "p1" {
			hand = pl.Hand
		}
	}
	if len(hand) < 2 {
		t.Fatalf("precondition: hand has %d cards, want >1 for a discard cost", len(hand))
	}

	// The intermediate decision: choose which card to discard, before the
	// ability is on the stack. It is a choose_cost_target, not a priority ask.
	inter := sd
	inter.Context.Kind = "choice"
	inter.Observation.Stack = nil
	inter.Candidates = nil
	for _, c := range hand[:2] {
		b, err := json.Marshal(map[string]any{"semantic": map[string]any{
			"kind": "choose_cost_target", "cost_kind": "discard", "source": src,
			"minimum": 1, "maximum": 1, "candidate": protocol.ObjectTarget(c.ObjectRef),
		}})
		if err != nil {
			t.Fatal(err)
		}
		var cost protocol.Candidate
		if err := json.Unmarshal(b, &cost); err != nil {
			t.Fatal(err)
		}
		inter.Candidates = append(inter.Candidates, cost)
	}

	s := NewSession(reg)
	s.SetAutoPay(true)
	s.last = &sd
	s.Picked(cand)
	mid, err := s.Payload(&inter)
	if err != nil {
		t.Fatal(err)
	}
	if mid.Decision.Kind != decision.KChoose {
		t.Fatalf("precondition: intermediate decision = %v, want a cost choose", mid.Decision.Kind)
	}
	if _, ok := s.grants[src.ObjectID]; !ok {
		t.Fatal("intermediate non-priority Payload dropped the recorded grant")
	}

	pl, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	var got *decision.Grant
	for _, o := range pl.Decision.Options {
		if o.Kind == "ability" && o.Grant != nil {
			got = o.Grant
		}
	}
	if got == nil {
		t.Fatalf("rebuilt ability option carries no Grant; native %+v", nativeGrant)
	}
	if !got.Duplicate || got.Already != nativeGrant.Already || len(got.Keywords) != len(nativeGrant.Keywords) ||
		got.Keywords[0] != nativeGrant.Keywords[0] {
		t.Fatalf("rebuilt grant after cost decision = %+v, native %+v", got, nativeGrant)
	}
}

// TestNeutralGrantTracksEveryPendingGrantFromOneSource pins the defect where a
// source whose latest activation was a pure grant overwrote the one before it:
// Glorifier of Dusk put a Flying grant then a different Vigilance grant on the
// stack, and the neutral rebuild reported only the Vigilance one as duplicate,
// so the bot paid 2 life for a redundant Flying pump. The native engine reads
// every object on the stack (rules.abilityGrant); the neutral session must
// remember every answered activation, oldest first, and trim by count.
func TestNeutralGrantTracksEveryPendingGrantFromOneSource(t *testing.T) {
	reg := testcorpus.Registry(t)
	card, ok := reg.Lookup("Glorifier of Dusk")
	if !ok {
		t.Fatal(`precondition: corpus has no "Glorifier of Dusk"`)
	}
	if len(card.Faces) == 0 {
		t.Fatal("precondition: Glorifier of Dusk has no faces")
	}
	var grants [][]string
	for _, ab := range card.Faces[0].Abilities {
		if kw := rules.PureGrantKeywords(ab); len(kw) > 0 {
			grants = append(grants, kw)
		}
	}
	if len(grants) != 2 {
		t.Fatalf("precondition: pure-grant abilities = %v, want exactly 2", grants)
	}
	if len(grants[0]) != 1 || !strings.EqualFold(grants[0][0], "Flying") ||
		len(grants[1]) != 1 || !strings.EqualFold(grants[1][0], "Vigilance") {
		t.Fatalf("precondition: pure grants = %v, want Flying then Vigilance", grants)
	}
	// The two abilities are the face's first two entries, which is the
	// ability_index the observation's candidates name.
	if len(card.Faces[0].Abilities) < 2 || card.Faces[0].Abilities[0].Kind != "AB" || card.Faces[0].Abilities[1].Kind != "AB" {
		t.Fatalf("precondition: first two abilities are not ABs")
	}

	name, seat0 := "Glorifier of Dusk", "p0"
	perm := protocol.ObjectRef{ObjectID: "o-glo", CardName: &name, OwnerSeat: "p0", ControllerSeat: "p0", Zone: "battlefield"}
	power, toughness := int32(4), int32(4)
	sd := protocol.SeatDecision{
		ActingSeat: "p0",
		SeatStep:   97,
		Group:      protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context:    protocol.Context{Kind: "priority"},
		Observation: protocol.Observation{
			Viewer:       "p0",
			Turn:         7,
			PhaseStep:    "precombat_main",
			ActiveSeat:   &seat0,
			PrioritySeat: &seat0,
			Players: [2]protocol.PlayerObs{
				{
					Seat: "p0",
					Life: 20,
					Battlefield: []protocol.ObjectRecord{{
						ObjectRef: perm,
						Characteristics: &protocol.Characteristics{
							Types: []string{"creature"}, Subtypes: []string{"vampire", "soldier"},
							Colors: []string{"white"}, ManaValue: 5, Power: &power, Toughness: &toughness,
							Keywords: []string{},
						},
						Permanent: &protocol.Permanent{Tapped: false},
					}},
				},
				{Seat: "p1", Life: 20},
			},
			// Both answered activations are still on the stack, from the
			// same source: the observation's only Duplicate evidence.
			Stack: []protocol.StackEntry{
				{ObjectRef: protocol.ObjectRef{ObjectID: "o-glo-stack-1", CardName: &name, OwnerSeat: "p0",
					ControllerSeat: "p0", Zone: "stack"}, StackKind: "activated_ability", Source: &perm},
				{ObjectRef: protocol.ObjectRef{ObjectID: "o-glo-stack-2", CardName: &name, OwnerSeat: "p0",
					ControllerSeat: "p0", Zone: "stack"}, StackKind: "activated_ability", Source: &perm},
			},
		},
		Candidates: []protocol.Candidate{
			{CandidateID: 0, Semantic: protocol.ActivateAbility(perm, 0)},
			{CandidateID: 1, Semantic: protocol.ActivateAbility(perm, 1)},
			{CandidateID: 2, Semantic: protocol.Pass()},
		},
	}
	// The wire turns semantic fields into json.RawMessage/json.Number; the
	// field readers (ref, num) require that round trip.
	wire, err := json.Marshal(sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wire, &sd); err != nil {
		t.Fatal(err)
	}

	s := NewSession(reg)
	s.last = &sd
	s.Picked(0) // Flying
	s.Picked(1) // Vigilance
	pl, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	var abilities []decision.Option
	for _, o := range pl.Decision.Options {
		if o.Kind == "ability" {
			abilities = append(abilities, o)
		}
	}
	if len(abilities) != 2 {
		t.Fatalf("precondition: rebuilt ability options = %d, want 2", len(abilities))
	}
	var stack int
	for _, e := range sd.Observation.Stack {
		if e.StackKind == "activated_ability" && e.Source != nil && e.Source.ObjectID == perm.ObjectID {
			stack++
		}
	}
	if stack != 2 {
		t.Fatalf("precondition: observed stack entries from the source = %d, want 2", stack)
	}
	for i, o := range abilities {
		if o.Grant == nil {
			t.Errorf("option %d (%s) carries no Grant", i, o.Label)
			continue
		}
		if o.Grant.Already {
			t.Errorf("option %d (%s) Already = true, want false", i, o.Label)
		}
		if !o.Grant.Duplicate {
			t.Errorf("option %d (%s) Duplicate = false; rebuilt grants: %s", i, o.Label, grantSummary(abilities))
		}
		want := grants[i][0]
		if len(o.Grant.Keywords) != 1 || !strings.EqualFold(o.Grant.Keywords[0], want) {
			t.Errorf("option %d keywords = %v, want [%s]", i, o.Grant.Keywords, want)
		}
	}

	in, err := heuristic.NewBot(16462000697783136304).Decide(context.Background(), pl.View, pl.Decision)
	if err != nil {
		t.Fatal(err)
	}
	for _, ch := range in.Choices {
		if ch >= 0 && ch < len(pl.Decision.Options) && pl.Decision.Options[ch].Kind == "ability" {
			t.Errorf("bot picked ability option %d (%s) for a redundant grant: %v", ch,
				pl.Decision.Options[ch].Label, in.Choices)
		}
	}
}

// grantSummary renders the rebuilt ability options' grant facts for a failure.
func grantSummary(opts []decision.Option) string {
	var b strings.Builder
	for _, o := range opts {
		g := o.Grant
		if g == nil {
			fmt.Fprintf(&b, "[%s:<nil>]", o.Label)
			continue
		}
		fmt.Fprintf(&b, "[%s:dup=%t]", strings.Join(g.Keywords, ","), g.Duplicate)
	}
	return b.String()
}

// TestNeutralRebuildsGrantedAbility pins the observation-backed ability list:
// a granted body the recipient's printed face does not carry, named by its
// position in ObjectRecord.Abilities, must translate rather than go opaque,
// and the follow-up target ask must resolve its effect against the GRANTOR's
// SVar table (the Longbow's Damage). The second Payload is the regression
// guard for targetEffect's missing lower-bound check: a granted activation's
// face index is -1, and f.Abilities[-1] panicked before the stored body and
// the i >= 0 guard.
func TestNeutralRebuildsGrantedAbility(t *testing.T) {
	reg := testcorpus.Registry(t)
	sorc, ok := reg.Lookup("Prodigal Sorcerer")
	if !ok {
		t.Fatal(`precondition: corpus has no "Prodigal Sorcerer"`)
	}
	lb, ok := reg.Lookup("Viridian Longbow")
	if !ok {
		t.Fatal(`precondition: corpus has no "Viridian Longbow"`)
	}
	// Precondition: the Sorcerer prints exactly one non-mana activated
	// ability, and the Longbow's Damage SVar is the granted targeting body.
	printed := 0
	for _, a := range sorc.Faces[0].Abilities {
		if a.Kind == "AB" && !cards.IsManaAbilitySA(a) {
			printed++
		}
	}
	if printed != 1 {
		t.Fatalf("precondition: Prodigal Sorcerer printed non-mana ABs = %d, want 1", printed)
	}
	damage := "Damage"
	sa := cards.ResolveSVar(lb.Faces[0].SVars, damage)
	if sa == nil || sa.API != "DealDamage" || sa.Params["ValidTgts"] == "" || sa.Params["NumDmg"] != "1" {
		t.Fatalf("precondition: Longbow Damage SVar = %+v, want DealDamage with a target for 1", sa)
	}

	seat0, p1 := "p0", "p1"
	sorcName, lbName := "Prodigal Sorcerer", "Viridian Longbow"
	sorcRef := protocol.ObjectRef{ObjectID: "o-sorc", CardName: &sorcName, OwnerSeat: "p0",
		ControllerSeat: "p0", Zone: "battlefield"}
	lbRef := protocol.ObjectRef{ObjectID: "o-longbow", CardName: &lbName, OwnerSeat: "p0",
		ControllerSeat: "p0", Zone: "battlefield"}
	printedLabel := "Prodigal Sorcerer: {T}: This creature deals 1 damage to any target."
	grantedLabel := "Viridian Longbow: {T}: This creature deals 1 damage to any target."
	power, toughness := int32(0), int32(1)
	abs := []protocol.AbilityRecord{
		{Label: printedLabel},
		{Label: grantedLabel, SVar: &damage, GrantSource: &lbRef},
	}
	// Precondition: the two entries differ and index 1 is the anchored one.
	if abs[0].Label == abs[1].Label {
		t.Fatalf("precondition: entries share the label %q", abs[0].Label)
	}
	if abs[0].SVar != nil || abs[0].GrantSource != nil {
		t.Fatalf("precondition: entry 0 is anchored: %+v", abs[0])
	}
	if abs[1].SVar == nil || abs[1].GrantSource == nil {
		t.Fatalf("precondition: entry 1 is not anchored: %+v", abs[1])
	}

	base := protocol.Observation{
		Viewer:       "p0",
		Turn:         7,
		PhaseStep:    "precombat_main",
		ActiveSeat:   &seat0,
		PrioritySeat: &seat0,
		Players: [2]protocol.PlayerObs{
			{
				Seat: "p0",
				Life: 20,
				Battlefield: []protocol.ObjectRecord{{
					ObjectRef: sorcRef,
					Characteristics: &protocol.Characteristics{
						Types: []string{"creature"}, Subtypes: []string{"human", "wizard"},
						Colors: []string{"blue"}, ManaValue: 3, Power: &power, Toughness: &toughness,
						Keywords: []string{},
					},
					Permanent: &protocol.Permanent{Tapped: false},
					Abilities: abs,
				}},
			},
			{Seat: "p1", Life: 20},
		},
	}
	sd := protocol.SeatDecision{
		ActingSeat:  "p0",
		SeatStep:    97,
		Group:       protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context:     protocol.Context{Kind: "priority"},
		Observation: base,
		Candidates: []protocol.Candidate{
			{CandidateID: 0, Semantic: protocol.ActivateAbility(sorcRef, 1)},
			{CandidateID: 1, Semantic: protocol.Pass()},
		},
	}
	wire, err := json.Marshal(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(wire, &sd); err != nil {
		t.Fatal(err)
	}

	s := NewSession(reg)
	pl, err := s.Payload(&sd)
	if err != nil {
		t.Fatal(err)
	}
	if len(pl.Ops) != 2 {
		t.Fatalf("precondition: ops = %d, want 2", len(pl.Ops))
	}
	if pl.Ops[0].Op != "choose" {
		t.Fatalf("granted activation is untranslated: %+v (%s)", pl.Ops[0], Untranslated)
	}
	if pl.Ops[0].Option < 0 || pl.Ops[0].Option >= len(pl.Decision.Options) {
		t.Fatalf("granted op option %d outside %d options", pl.Ops[0].Option, len(pl.Decision.Options))
	}
	got := pl.Decision.Options[pl.Ops[0].Option]
	if got.Kind != "ability" || got.SVar != damage || got.Label != grantedLabel || got.Ability != -1 {
		t.Fatalf("rebuilt granted ability = %+v, want kind ability, SVar %q, label %q, Ability -1",
			got, damage, grantedLabel)
	}
	if pass := pl.Decision.Options[pl.Ops[1].Option]; pass.Kind != "pass" {
		t.Fatalf("pass candidate rebuilt as %+v", pass)
	}

	// The follow-up target ask: Picked records the granted body, so the ask
	// resolves against the Longbow's Damage SA instead of indexing the
	// recipient's face at -1.
	s.Picked(0)
	var targetObs protocol.Observation
	stackName := "Prodigal Sorcerer"
	stackRef := protocol.ObjectRef{ObjectID: "o-sorc-stack", CardName: &stackName, OwnerSeat: "p0",
		ControllerSeat: "p0", Zone: "stack"}
	targetObs = base
	targetObs.Stack = []protocol.StackEntry{{ObjectRef: stackRef, StackKind: "activated_ability", Source: &sorcRef}}
	tgt := protocol.SeatDecision{
		ActingSeat:  "p0",
		SeatStep:    98,
		Group:       protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context:     protocol.Context{Kind: "choice", Source: &stackRef},
		Observation: targetObs,
		Candidates: []protocol.Candidate{
			{CandidateID: 0, Semantic: protocol.ChooseTarget(stackRef, 0, protocol.TargetRef{Player: &p1}, 0, 1, 1)},
		},
	}
	twire, err := json.Marshal(&tgt)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(twire, &tgt); err != nil {
		t.Fatal(err)
	}
	tp, err := s.Payload(&tgt)
	if err != nil {
		t.Fatal(err)
	}
	te := tp.Decision.TargetEffect
	if te == nil || te.API != "DealDamage" || te.Damage == nil || te.Damage.Amount == nil || *te.Damage.Amount != 1 {
		t.Fatalf("target effect = %+v, want the Longbow's DealDamage 1", te)
	}

	// The announcement-time ask (the ability is not on the stack yet): the
	// same body must resolve through targetEffect's battlefield case. Its
	// face index is -1, so the i >= 0 guard is what keeps f.Abilities[-1]
	// from panicking.
	announce := protocol.SeatDecision{
		ActingSeat:  "p0",
		SeatStep:    99,
		Group:       protocol.Group{SubstepIndex: 0, SubstepCount: 1},
		Context:     protocol.Context{Kind: "choice", Source: &sorcRef},
		Observation: base,
		Candidates: []protocol.Candidate{
			{CandidateID: 0, Semantic: protocol.ChooseTarget(sorcRef, 0, protocol.TargetRef{Player: &p1}, 0, 1, 1)},
		},
	}
	awire, err := json.Marshal(&announce)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(awire, &announce); err != nil {
		t.Fatal(err)
	}
	ap, err := s.Payload(&announce)
	if err != nil {
		t.Fatal(err)
	}
	ae := ap.Decision.TargetEffect
	if ae == nil || ae.API != "DealDamage" || ae.Damage == nil || ae.Damage.Amount == nil || *ae.Damage.Amount != 1 {
		t.Fatalf("announcement target effect = %+v, want the Longbow's DealDamage 1", ae)
	}
}
