package eventhost

import (
	"fmt"
	"math/rand/v2"

	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/draft"
)

// pickSaltBase tags the per-pick rng streams so they never collide with
// booster's own per-pack streams (booster salts seat<<32|pack).
const pickSaltBase = 0x5011C1

// pickRNG is one pick's decision rng: a fresh stream per pick, seeded from
// the event seed and salted by seat, round and pick number, so a resume
// needs no rng state — the same (seed, seat, round, pick) always yields
// the same stream.
func pickRNG(seed uint64, seat, round, pickNo int) *rand.Rand {
	return rand.New(rand.NewPCG(seed, pickSaltBase<<32|
		uint64(seat)<<16|uint64(round)<<8|uint64(pickNo)))
}

// advance drives the run forward: it loops its phase steps until a full
// pass makes no progress. It is called under run.mu after every mutation
// and at resume.
func (r *run) advance() {
	sawDraft := false
	for {
		progressed := false
		for _, step := range []func() bool{r.advanceLobby, r.advanceDraft, r.advanceBuild, r.advanceRounds} {
			if step() {
				progressed = true
			}
		}
		if r.ev.Phase() == event.PhaseDraft {
			sawDraft = true
		}
		if !progressed {
			break
		}
	}
	if sawDraft && r.ev.Phase() == event.PhaseBuild {
		r.justDraft = true
	}
	// E3-4: the log just moved — answer every parked series-hook call
	// whose plan changed, so a between-games table resumes the moment the
	// next GameStarted is on the log and an ended series' table stops.
	r.wakeWaiters()
	// E3-6: re-evaluate every human seat's pick timer against the new log.
	r.armPickTimers()
}

// advanceLobby creates the event once every human seat has a name, then
// opens every pack of the pod that the log does not yet hold. Regenerating
// the pod is deterministic, so a crash between the EventCreated and the
// last PackOpened resumes cleanly. It reports whether it applied anything.
func (r *run) advanceLobby() bool {
	if r.ev.Phase() != event.PhaseLobby {
		return false
	}
	for _, ms := range r.meta.Seats {
		if ms.Kind == "human" && ms.Name == "" {
			return false
		}
	}
	seats := [draft.Seats]event.SeatSpec{}
	for i, ms := range r.meta.Seats {
		spec := event.SeatSpec{Kind: ms.Kind, Name: ms.Name}
		if ms.Kind == "bot" {
			spec.Persona = ms.Drafter + "/" + ms.Policy
		}
		seats[i] = spec
	}
	if err := r.apply(event.Entry{
		Kind: event.KindEventCreated, Seed: r.meta.Seed, Set: r.meta.Set, Seats: seats[:],
	}); err != nil {
		return false
	}
	return r.openPacks()
}

// openPacks applies every PackOpened whose (round, seat) the log does not
// hold yet, seat-major and pack-minor, in DRAFT only. It reports whether
// it applied anything.
func (r *run) openPacks() bool {
	opened := r.openedInLog()
	packs, err := r.openPod()
	if err != nil {
		r.halted = fmt.Sprintf("event %s: pod generation failed: %v", r.meta.ID, err)
		return false
	}
	applied := false
	for s := 0; s < draft.Seats; s++ {
		for rd := 1; rd <= draft.Rounds; rd++ {
			if opened[s][rd-1] || r.ev.Phase() != event.PhaseDraft {
				continue
			}
			if err := r.apply(event.Entry{
				Kind: event.KindPackOpened, Round: rd, Seat: s, Cards: packs[s][rd-1],
			}); err != nil {
				return applied
			}
			applied = true
		}
	}
	return applied
}

// openedInLog records which (seat, round-1) PackOpened entries the log
// holds.
func (r *run) openedInLog() [draft.Seats][draft.Rounds]bool {
	var out [draft.Seats][draft.Rounds]bool
	for _, en := range r.ev.Log() {
		if en.Kind == event.KindPackOpened &&
			en.Seat >= 0 && en.Seat < draft.Seats &&
			en.Round >= 1 && en.Round <= draft.Rounds {
			out[en.Seat][en.Round-1] = true
		}
	}
	return out
}

// advanceDraft makes every bot seat take every pick it can: passes of
// seats 0..3, repeated until a pass picks nothing. A human pick (Pick)
// between passes is what keeps the pod moving in a mixed event, because
// the bots pick synchronously right after it. It reports whether it
// applied anything.
func (r *run) advanceDraft() bool {
	applied := false
	for r.ev.Phase() == event.PhaseDraft {
		progressed := false
		for s := 0; s < draft.Seats; s++ {
			if r.meta.Seats[s].Kind != "bot" || r.drafters[s] == nil {
				continue
			}
			v, ok := r.ev.Next(s)
			if !ok {
				continue
			}
			id := r.drafters[s].Pick(v, pickRNG(r.meta.Seed, s, v.Round, v.PickNo))
			if err := r.apply(event.Entry{
				Kind: event.KindPicked, Round: v.Round, PickNo: v.PickNo, Seat: s, ID: id,
			}); err != nil {
				r.halted = fmt.Sprintf("event %s: bot seat %d pick failed: %v", r.meta.ID, s, err)
				return applied
			}
			progressed, applied = true, true
		}
		if !progressed {
			return applied
		}
	}
	return applied
}
