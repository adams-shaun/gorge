package draftlab

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// pickSaltBase tags the per-seat pick-rng streams so they never collide
// with booster's own per-pack streams (booster salts seat<<32|pack). The
// same constant cmd/draftsim's event driver salts with.
const pickSaltBase = 0x5011C1

// DraftResult is one finished draft: the pools in pick order and the
// replayable event log through the end of the draft (the event stands in
// BUILD).
type DraftResult struct {
	// Set is the drafted set code, from the packs' CardRefs.
	Set string
	// Seed is the event seed the draft ran under.
	Seed uint64
	// Drafters is the effective ref per seat, in seat order.
	Drafters []AgentRef
	// Pools is each seat's picks in pick order, from the event's own pod.
	Pools [][]limited.CardRef
	// DraftIDs is the 17Lands draft_id per seat: "<seed>-s<seat>".
	DraftIDs []string
	// Log is the event log (JSON lines) through the end of the draft.
	Log []byte
}

// Draft runs packs ([seat][round] CardRefs, opened by one OpenPod call so
// the IDs are unique pod-wide) through an event in draft mode with no
// series: EventCreated, the packs seat-major, then draftsim's loop — every
// seat in order picks whenever its queue head has a pack, one persistent
// rng per seat from rand.NewPCG(seed, pickSaltBase<<32|seat) — until the
// pod is done and the event stands in BUILD.
//
// drafters has one entry (every seat plays it) or len(packs). The refusal
// list: a seat count that is not 4 or 8, a drafters slice that is neither
// 1 nor n, an empty or mixed set across the CardRefs, and any event.Apply
// error — duplicate CardRef IDs across packs surface there, which is why
// the error says to open a pod's packs with one OpenPod call.
func (l *Lab) Draft(packs [][][]limited.CardRef, drafters []NamedDrafter, seed uint64) (DraftResult, error) {
	n := len(packs)
	if n != 4 && n != 8 {
		return DraftResult{}, fmt.Errorf("draftlab: draft: got %d seats, want 4 or 8", n)
	}
	if len(drafters) != 1 && len(drafters) != n {
		return DraftResult{}, fmt.Errorf("draftlab: draft: got %d drafters for %d seats, want 1 or %d", len(drafters), n, n)
	}
	set, err := packSet(packs)
	if err != nil {
		return DraftResult{}, err
	}
	drafterOf := func(s int) NamedDrafter {
		if len(drafters) == 1 {
			return drafters[0]
		}
		return drafters[s]
	}

	seats := make([]event.SeatSpec, n)
	for s := range seats {
		seats[s] = event.SeatSpec{
			Kind:    "bot",
			Name:    fmt.Sprintf("p%d", s),
			Persona: fmt.Sprintf("%s/%s", set, drafterOf(s).Ref),
		}
	}
	e := event.New()
	if err := e.Apply(event.Entry{
		Kind: event.KindEventCreated, Seed: seed, Set: set,
		Seats: seats, Structure: event.StructureNone,
	}); err != nil {
		return DraftResult{}, fmt.Errorf("draftlab: draft: EventCreated: %w", err)
	}
	for s := 0; s < n; s++ {
		for rd := 0; rd < len(packs[s]); rd++ {
			if err := e.Apply(event.Entry{
				Kind: event.KindPackOpened, Round: rd + 1, Seat: s, Cards: packs[s][rd],
			}); err != nil {
				return DraftResult{}, fmt.Errorf("draftlab: draft: pack round %d seat %d: %w (open a pod's packs with one OpenPod call)", rd+1, s, err)
			}
		}
	}

	if err := runDraftLoop(e, n, drafterOf, seed); err != nil {
		return DraftResult{}, err
	}

	log, err := event.MarshalLog(e.Log())
	if err != nil {
		return DraftResult{}, fmt.Errorf("draftlab: draft: %w", err)
	}
	pools := make([][]limited.CardRef, n)
	drefs := make([]AgentRef, n)
	draftIDs := make([]string, n)
	for s := 0; s < n; s++ {
		pools[s] = e.Pool(s)
		drefs[s] = drafterOf(s).Ref
		draftIDs[s] = fmt.Sprintf("%d-s%d", seed, s)
	}
	return DraftResult{
		Set: set, Seed: seed, Drafters: drefs, Pools: pools, DraftIDs: draftIDs, Log: log,
	}, nil
}

// packSet derives the one set the packs' CardRefs belong to: the distinct
// Set value ignoring the rider codes. A rider code (SPG in FDN/BLB Play
// Boosters, SDS in TRK's) is a bonus sheet that rides inside another set's
// packs by design — an Arena FDN pod legitimately mixes FDN and SPG
// printings, a TRK pod mixes TRK and SDS — so a pod's set is the base set
// of its cards. A second base set (two pods concatenated) is an error, as
// is a CardRef without a set and a rider-only pod: an event names exactly
// one set, and its EventCreated Set field comes from here.
func packSet(packs [][][]limited.CardRef) (string, error) {
	set := ""
	riders := map[string]bool{}
	for s := range packs {
		for r := range packs[s] {
			for _, c := range packs[s][r] {
				if c.Set == "" {
					return "", fmt.Errorf("draftlab: draft: pack card %s (%s) carries no set", c.ID, c.Name)
				}
				if isRider(c.Set) {
					riders[c.Set] = true
					continue
				}
				if set == "" {
					set = c.Set
				}
				if c.Set != set {
					return "", fmt.Errorf("draftlab: draft: mixed sets in the packs: %q and %q", set, c.Set)
				}
			}
		}
	}
	if set == "" {
		if bonusOnly(packs) {
			return "", fmt.Errorf("draftlab: draft: packs carry only bonus-sheet cards (%s), no base set", riderJoin(riders))
		}
		return "", errors.New("draftlab: draft: packs carry no cards")
	}
	return set, nil
}

// riderSets derives the rider codes from the embedded booster specs: a
// code is a rider when some embedded collation lists it in
// source_set_codes and it is not itself one of setdata.Codes(). The next
// set with a rider sheet needs no edit here. A collation that fails to
// load is skipped (setdata's own tests validate the embedded files).
var riderSets = sync.OnceValue(func() map[string]bool {
	codes := setdata.Codes()
	own := map[string]bool{}
	for _, code := range codes {
		own[code] = true
	}
	riders := map[string]bool{}
	for _, code := range codes {
		for _, c := range setdata.Collations(code) {
			spec, err := setdata.LoadCollation(code, c)
			if err != nil {
				continue
			}
			for _, src := range spec.Config.SourceSetCodes {
				if !own[src] {
					riders[src] = true
				}
			}
		}
	}
	return riders
})

// isRider reports whether the code is a rider bonus-sheet code (SPG, SDS).
func isRider(code string) bool { return riderSets()[code] }

// riderJoin renders the distinct rider codes seen, sorted and
// comma-joined, for the rider-only error.
func riderJoin(riders map[string]bool) string {
	codes := make([]string, 0, len(riders))
	for c := range riders {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	return strings.Join(codes, ", ")
}

// bonusOnly reports whether the packs carry cards at all.
func bonusOnly(packs [][][]limited.CardRef) bool {
	for s := range packs {
		for r := range packs[s] {
			if len(packs[s][r]) > 0 {
				return true
			}
		}
	}
	return false
}

// runDraftLoop drives a draft-mode event from its pack openings to BUILD:
// every seat in order picks whenever its queue head has a pack, one
// persistent rng per seat from rand.NewPCG(seed, pickSaltBase<<32|seat) —
// the picks are a pure function of (seed, seat, pick history) — until the
// pod is done and the event stands in BUILD. Both Draft and the event
// driver funnel through it.
func runDraftLoop(e *event.Event, n int, drafterOf func(int) NamedDrafter, seed uint64) error {
	// One persistent rng per seat, used in pick order only: the picks are a
	// pure function of (seed, seat, pick history).
	rngs := make([]*rand.Rand, n)
	for s := range rngs {
		rngs[s] = rand.New(rand.NewPCG(seed, pickSaltBase<<32|uint64(s)))
	}
	for e.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < n; s++ {
			v, ok := e.Next(s)
			if !ok {
				continue
			}
			var id string
			if fd, ok := drafterOf(s).Drafter.(FallibleDrafter); ok {
				var err error
				if id, err = fd.PickErr(v, rngs[s]); err != nil {
					return fmt.Errorf("draftlab: draft: pick seat %d round %d pick %d: %w", s, v.Round, v.PickNo, err)
				}
			} else {
				id = drafterOf(s).Drafter.Pick(v, rngs[s])
			}
			if err := e.Apply(event.Entry{
				Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: id,
			}); err != nil {
				return fmt.Errorf("draftlab: draft: pick seat %d round %d pick %d: %w (open a pod's packs with one OpenPod call)", s, v.Round, v.PickNo, err)
			}
			progressed = true
		}
		if !progressed {
			return fmt.Errorf("draftlab: draft: stuck in phase %s with no legal pick", e.Phase())
		}
	}
	return nil
}
