package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// Task api:SetState.Optional: Optional$ True was unread -- every "you may
// transform" SetState ALWAYS transformed, unconditionally, with no election
// recorded anywhere in the log. effSetState (effects/misc.go) now poses a real
// yes/no KChoose through the shared Ask boundary with the "setstate_optional"
// resume arm (the put_optional/attach_optional precedent): the answer rides
// Ctx.SetStateOpt, the decline changes nothing while the chained SubAbility$
// still runs (the chain is owned by Resolve, never skipped by a decline), and
// the no-host fallback takes the deterministic decline (R-9) with option 0 =
// "yes" so the bot clamp keeps bot games byte-identical to the silent
// always-transform.
//
// These are the effects-package unit pins, against the REAL compiled corpus
// SAs (a synthetic map[string]string fixture has shipped bugs here before):
// the ask's shape, both answered re-entries, the no-ask gate, and the no-host
// deterministic decline stand-in (R-9). The engine-side end-to-end pins live
// in rules/setstate_optional_test.go.

// corpusSetStateSA returns the REAL compiled SetState sub-ability of a named
// corpus card, searching Abilities, every Trigger.Effect, every Repl.With and
// each of their Sub chains, and asserting it really carries Optional$ True --
// a caller can never be handed an SA that only looks like the shape under
// test. It also returns the owning face's SVar table, the same Ctx.SVars
// resolveTop's branches seed, so a chained sub can resolve its
// StaticAbilities$ SVar bodies.
func corpusSetStateSA(t *testing.T, name string) (*cards.SA, map[string]string) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	c, ok := reg.Lookup(name)
	if !ok {
		t.Fatalf("corpus has no %q", name)
	}
	var found *cards.SA
	var svars map[string]string
	walk := func(sa *cards.SA) {
		for ; sa != nil && found == nil; sa = sa.Sub {
			if sa.API == "SetState" && strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
				found = sa
				return
			}
		}
	}
	for _, f := range c.Faces {
		for _, a := range f.Abilities {
			walk(a)
		}
		for _, tr := range f.Triggers {
			walk(tr.Effect)
		}
		for _, r := range f.Repls {
			walk(r.With)
		}
		// A named SVar can be reached through a Branch/MakeCard rider
		// rather than a linked Sub chain (High Marshal Arguel). Resolve the
		// real corpus SVar instead of synthesising a replacement script.
		if found == nil {
			for _, name := range []string{"DBTransform"} {
				if f.SVars[name] == "" {
					continue
				}
				candidate := cards.ResolveSVar(f.SVars, name)
				if candidate != nil && candidate.API == "SetState" && strings.EqualFold(candidate.Params["Optional"], "True") {
					found = candidate
				}
			}
		}
		if found != nil {
			svars = f.SVars
			break
		}
	}
	if found == nil {
		t.Fatalf("corpus card %q has no compiled Optional$ SetState SA", name)
	}
	return found, svars
}

// setStateObject adds a two-faced permanent on h owned by seat owner, on the
// battlefield, and returns its id.
func setStateObject(t *testing.T, h *fakeHost, owner state.PlayerID) state.ObjID {
	t.Helper()
	o := h.g.AddObject(twoFacedCard(t), owner)
	o.Zone = state.ZBattlefield
	h.g.SetZone(state.ZBattlefield, owner, append(h.g.Zone(state.ZBattlefield, owner), o.ID))
	return o.ID
}

// flipFaceCount counts FlipFace events the host recorded.
func flipFaceCount(h *fakeHost) int {
	n := 0
	for _, ev := range h.log {
		if ev.Kind == events.FlipFace {
			n++
		}
	}
	return n
}

// TestSetStateOptionalAskShapeAndAnsweredReEntries pins the election's wire
// shape and both re-entries on Dowsing Dagger's real DBTransform (DB$ SetState
// | Defined$ Self | Mode$ Transform | Optional$ True): the first pass SUSPENDS
// on the ask with option 0 = "yes" and flips nothing yet; the "no" re-entry
// flips nothing; the "yes" re-entry flips exactly once.
func TestSetStateOptionalAskShapeAndAnsweredReEntries(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	src := setStateObject(t, &h.fakeHost, 0)
	sa, sv := corpusSetStateSA(t, "Dowsing Dagger")
	if !strings.EqualFold(strings.TrimSpace(sa.Params["Optional"]), "True") {
		t.Fatalf("SA is not Optional$ True: %+v", sa.Params)
	}
	if !strings.EqualFold(strings.TrimSpace(sa.Params["Mode"]), "Transform") {
		t.Fatalf("SA is not Mode$ Transform: %+v", sa.Params)
	}
	if !strings.EqualFold(strings.TrimSpace(sa.Params["Defined"]), "Self") || sa.Params["Choices"] != "" {
		t.Fatalf("SA Defined = %q / Choices = %q, want Self without Choices", sa.Params["Defined"], sa.Params["Choices"])
	}

	if o := h.g.Obj(src); o.Zone != state.ZBattlefield || o.FaceIdx != 0 || len(o.Card.Faces) != 2 {
		t.Fatalf("precondition: want front face of a two-faced battlefield permanent, got %+v", o)
	}
	Resolve(h, &Ctx{Source: src, Controller: 0, SVars: sv}, sa)
	if h.asked == nil {
		t.Fatal("no election posed for an Optional$ True SetState")
	}
	d := h.asked
	if d.Kind != decision.KChoose || d.Min != 1 || d.Max != 1 {
		t.Fatalf("election = %+v, want a Min==Max==1 KChoose", d)
	}
	if d.Player != 0 {
		t.Fatalf("ask player = %d, want the controller (0)", d.Player)
	}
	if d.ResumeKind != "setstate_optional" {
		t.Fatalf("ResumeKind = %q, want setstate_optional", d.ResumeKind)
	}
	if len(d.Options) != 2 || d.Options[0].Kind != "yes" || d.Options[1].Kind != "no" {
		t.Fatalf("options = %+v, want yes then no (option 0 = yes, so the bot clamp keeps the always-flip)", d.Options)
	}
	if got := flipFaceCount(&h.fakeHost); got != 0 {
		t.Fatalf("first pass flipped %d time(s) before the election was answered", got)
	}

	Resolve(h, &Ctx{Source: src, Controller: 0, SVars: sv, SetStateOpt: "no"}, sa)
	if got := flipFaceCount(&h.fakeHost); got != 0 {
		t.Fatalf("decline flipped %d time(s), want none", got)
	}
	if o := h.g.Obj(src); o.FaceIdx != 0 {
		t.Fatalf("decline moved FaceIdx to %d, want 0", o.FaceIdx)
	}

	Resolve(h, &Ctx{Source: src, Controller: 0, SVars: sv, SetStateOpt: "yes"}, sa)
	if got := flipFaceCount(&h.fakeHost); got != 1 {
		t.Fatalf("accept flipped %d time(s), want exactly 1", got)
	}
	if o := h.g.Obj(src); o.FaceIdx != 1 {
		t.Fatalf("accept left FaceIdx at %d, want 1", o.FaceIdx)
	}
}

// TestSetStateOptionalNothingToChangeNeverAsks pins the no-ask gate: a
// single-faced source has nothing the change would do, so decline and accept
// are the same and no decision is posed -- the existing
// TestSetStateNoOpsOnASingleFaceCard stays event-free.
func TestSetStateOptionalNothingToChangeNeverAsks(t *testing.T) {
	h := &askHost{}
	h.g = state.NewGame(names(2))
	card := mkCard(t, "Name:Single\nTypes:Creature\nPT:1/1\nOracle:x\n")
	o := h.g.AddObject(card, 0)
	o.Zone = state.ZBattlefield
	sa, _ := corpusSetStateSA(t, "Dowsing Dagger")
	if len(o.Card.Faces) != 1 || sa.Params["Defined"] != "Self" {
		t.Fatalf("precondition: want single-faced Self recipient, got %d faces / %q", len(o.Card.Faces), sa.Params["Defined"])
	}

	Resolve(h, &Ctx{Source: o.ID, Controller: 0}, sa)
	if h.asked != nil {
		t.Fatalf("election posed with nothing to change: %+v", h.asked)
	}
	if h.askCount != 0 || len(h.log) != 0 {
		t.Fatalf("no-op asked %d time(s), log = %+v", h.askCount, h.log)
	}
	// A single-face no-op must not pass just because SetState was unregistered.
	if registry.load().byName["SetState"] == nil {
		t.Fatal("SetState handler not registered")
	}
	// Positive control: this same real SA must ask when given a two-face
	// battlefield recipient; the no-op-only assertion would pass unfixed.
	live := setStateObject(t, &h.fakeHost, 0)
	Resolve(h, &Ctx{Source: live, Controller: 0}, sa)
	if h.asked == nil {
		t.Fatal("positive control: Optional$ SetState did not ask for live recipient")
	}
}

// TestSetStateOptionalNoAskHostDeclinesAndRunsTheChain pins the R-9 stand-in
// AND the decline-runs-the-chain semantics on High Marshal Arguel's real
// DBTransform (Defined$ Remembered, Mode$ Transform, Optional$ True,
// SubAbility$ DBCleanup): a host that cannot ask takes the deterministic
// DECLINE (no flip), and the chained DBCleanup still runs -- observable as a
// completed, non-suspended resolution carrying the same SetState source with
// FaceIdx unchanged.
func TestSetStateOptionalNoAskHostDeclinesAndRunsTheChain(t *testing.T) {
	h := newHost(t, 2)
	rem := setStateObject(t, h, 0)
	sa, sv := corpusSetStateSA(t, "High Marshal Arguel")
	if !strings.Contains(sa.Params["Defined"], "Remembered") {
		t.Fatalf("High Marshal Arguel SA Defined = %q, want Remembered", sa.Params["Defined"])
	}
	if strings.TrimSpace(sa.Params["SubAbility"]) == "" {
		t.Fatalf("High Marshal Arguel SA has no SubAbility chain: %+v", sa.Params)
	}

	if o := h.g.Obj(rem); o.Zone != state.ZBattlefield || len(o.Card.Faces) != 2 || o.FaceIdx != 0 {
		t.Fatalf("precondition: want front face of a two-faced battlefield permanent, got %+v", o)
	}
	Resolve(h, &Ctx{Source: rem, Controller: 0, SVars: sv,
		Remembered: []state.Target{{Obj: rem}}}, sa)
	if h.askCount != 1 {
		t.Fatalf("no-host path asked %d times, want one attempted election", h.askCount)
	}
	if got := flipFaceCount(h); got != 0 {
		t.Fatalf("no-ask host flipped %d time(s), want the deterministic decline (0)", got)
	}
	if o := h.g.Obj(rem); o.FaceIdx != 0 {
		t.Fatalf("no-ask host left FaceIdx at %d, want 0", o.FaceIdx)
	}
	if !setStateCleanupRan(h) {
		t.Fatal("no-host decline skipped High Marshal Arguel's DBCleanup")
	}
}

// TestSetStateOptionalChainedSubAbilityRunsOnBothAnswers pins the
// chain-owns-Resolve semantics: an answered SetState with a SubAbility$ still
// runs the sub on BOTH the accept and the decline paths (the decline returns
// from effSetState, never from Resolve). The sub here is High Marshal Arguel's
// real DBCleanup; the observable that the chain ran is that Resolve completed
// each pass without stalling and produced exactly the face change the answer
// asked for.
func setStateCleanupRan(h *fakeHost) bool {
	for _, ev := range h.log {
		if ev.Kind == events.Note && ev.Text == "clears remembered/imprinted objects" {
			return true
		}
	}
	return false
}

func TestSetStateOptionalChainedSubAbilityRunsOnBothAnswers(t *testing.T) {
	sa, sv := corpusSetStateSA(t, "High Marshal Arguel")
	for _, answer := range []string{"no", "yes"} {
		t.Run(answer, func(t *testing.T) {
			h := newHost(t, 2)
			rem := setStateObject(t, h, 0)
			if o := h.g.Obj(rem); o.Zone != state.ZBattlefield || len(o.Card.Faces) != 2 || o.FaceIdx != 0 {
				t.Fatalf("precondition: want front face of a two-faced battlefield permanent, got %+v", o)
			}
			Resolve(h, &Ctx{Source: rem, Controller: 0, SVars: sv,
				Remembered: []state.Target{{Obj: rem}}, SetStateOpt: answer}, sa)
			if !setStateCleanupRan(h) {
				t.Fatalf("%s skipped High Marshal Arguel's DBCleanup: %+v", answer, h.log)
			}
			want := 0
			if answer == "yes" {
				want = 1
			}
			if got := flipFaceCount(h); got != want {
				t.Fatalf("%s flipped %d time(s), want %d", answer, got, want)
			}
		})
	}
}
