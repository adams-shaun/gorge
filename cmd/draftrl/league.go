package main

import (
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/draftlab"
	"github.com/adams-shaun/gorge/limited/draftfeat"
	"github.com/adams-shaun/gorge/limited/pickbot"
)

// The league drafter hook (DR-7). A league spec token names one seat's
// drafter: "17l" and "ovr:PATH#FIELD" are built in (rating-colour with the
// embedded 17Lands ratings or a ratings override); any other "KIND:ARG"
// resolves through a DrafterKind registered under KIND. DR-5's network
// drafter plugs in as RegisterDrafterKind("net", ...) without draftrl
// depending on its package: the kind returns any pickbot.Drafter, and if
// that drafter also implements LogitSource the rollout records the logits
// it produced for each pick next to the pick's features.

// DrafterKind builds a fresh drafter for one seat of one pod from the text
// after "KIND:". It is called once per seat per pod, serially, so a kind
// may share loaded weights across calls. ref is the drafter id the records
// carry (e.g. "net@<weights hash>").
type DrafterKind func(lab *draftlab.Lab, arg, set string) (d pickbot.Drafter, ref draftlab.AgentRef, err error)

// LogitSource is the optional recorder side of a drafter: LastLogits
// returns the scores of the pick it just made, one per card of the pack in
// pack order (nil when it has none). The rollout copies the slice. DR-5's
// NetDrafter Recorder adapts to it with a few lines.
type LogitSource interface {
	LastLogits() []float32
}

// BehaviourSource is the optional sampling side of a LogitSource drafter:
// for a stochastic pick it returns the temperature and the chosen card's
// log-prob under softmax(logits/temp) (ok false for a deterministic pick).
// The rollout writes it into picks.bin's behaviour block (version 2) so
// PPO has the behaviour log-prob.
type BehaviourSource interface {
	LastBehaviour() (temp float32, logp float32, ok bool)
}

var drafterKinds = struct {
	sync.Mutex
	m map[string]DrafterKind
}{m: map[string]DrafterKind{}}

// RegisterDrafterKind registers a league drafter kind under prefix; "17l"
// and "ovr" are reserved. Registering a prefix twice replaces it.
func RegisterDrafterKind(prefix string, k DrafterKind) {
	if prefix == "17l" || prefix == "ovr" || prefix == "" || strings.Contains(prefix, ":") {
		panic(fmt.Sprintf("draftrl: drafter kind %q is reserved or malformed", prefix))
	}
	drafterKinds.Lock()
	defer drafterKinds.Unlock()
	drafterKinds.m[prefix] = k
}

func lookupDrafterKind(prefix string) (DrafterKind, bool) {
	drafterKinds.Lock()
	defer drafterKinds.Unlock()
	k, ok := drafterKinds.m[prefix]
	return k, ok
}

// leagueSpec is one parsed league entry.
type leagueSpec struct {
	text  string // the spec as given; the drafter id in records
	agent seatAgent
	kind  string // "" for a built-in (17l/ovr)
	arg   string
}

func parseLeague(s string) ([]leagueSpec, error) {
	var out []leagueSpec
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		if tok == "17l" || strings.HasPrefix(tok, "ovr:") {
			a, err := parseSeatAgents(tok, false)
			if err != nil {
				return nil, err
			}
			out = append(out, leagueSpec{text: tok, agent: a[0]})
			continue
		}
		kind, arg, ok := strings.Cut(tok, ":")
		if !ok {
			return nil, fmt.Errorf("league spec %q: want 17l, ovr:PATH#FIELD or KIND:ARG", tok)
		}
		if _, ok := lookupDrafterKind(kind); !ok {
			return nil, fmt.Errorf("league spec %q: no drafter kind %q is linked into this build", tok, kind)
		}
		out = append(out, leagueSpec{text: tok, kind: kind, arg: arg})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty league")
	}
	return out, nil
}

// leagueSalt tags a pod's seat-assignment stream.
const leagueSalt = 0xD4A7E6

// seatLeague draws a pod's four league entries, uniform with replacement,
// from rand.NewPCG(podSeed, leagueSalt). With a 4-entry league and
// -league-fixed the seats take the entries in order instead.
func seatLeague(league []leagueSpec, podSeed uint64, fixed bool) [4]int {
	return seatPod(len(league), podSeed, fixed, nil, -1, 0)
}

// seatPod is seatLeague with the S2 learner options (spec §3.3). n is the
// number of league entries. With learner >= 0 (the learner's index, which
// is never drawn by the league draw) the pod first places `learners` seats
// at random positions on the learner, then draws every other seat from the
// remaining entries. weights (one per entry, nil = uniform; the learner's
// own weight is ignored) make the league draw weighted. With learner < 0
// and nil weights it is exactly seatLeague's draw, so existing rollouts
// reproduce byte for byte.
func seatPod(n int, podSeed uint64, fixed bool, weights []float64, learner, learners int) [4]int {
	var out [4]int
	if fixed {
		for s := range out {
			out[s] = s % n
		}
		return out
	}
	r := rand.New(rand.NewPCG(podSeed, leagueSalt))
	if learner < 0 && weights == nil {
		for s := range out {
			out[s] = r.IntN(n)
		}
		return out
	}
	var isLearner [4]bool
	if learner >= 0 {
		for _, s := range r.Perm(4)[:learners] {
			isLearner[s] = true
		}
	}
	// The league draw: weighted over every entry but the learner.
	w := make([]float64, n)
	sum := 0.0
	for i := range w {
		if i == learner {
			continue
		}
		w[i] = 1
		if weights != nil {
			w[i] = weights[i]
		}
		sum += w[i]
	}
	for s := range out {
		if isLearner[s] {
			out[s] = learner
			continue
		}
		x := r.Float64() * sum
		out[s] = -1
		for i, wi := range w {
			if wi <= 0 {
				continue
			}
			out[s] = i
			if x < wi {
				break
			}
			x -= wi
		}
	}
	return out
}

// parseWeights parses -league-weights: n comma-separated finite weights >= 0
// with a positive sum, or nil for an empty string.
func parseWeights(s string, n int) ([]float64, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	toks := strings.Split(s, ",")
	if len(toks) != n {
		return nil, fmt.Errorf("%d weights for %d league entries", len(toks), n)
	}
	out := make([]float64, n)
	sum := 0.0
	for i, t := range toks {
		w, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil || w < 0 || math.IsNaN(w) || math.IsInf(w, 0) {
			return nil, fmt.Errorf("weight %q: want a finite number >= 0", t)
		}
		out[i] = w
		sum += w
	}
	if sum <= 0 {
		return nil, fmt.Errorf("weights sum to 0")
	}
	return out, nil
}

// ratingsCache loads each ratings override file once.
type ratingsCache struct {
	set string
	m   map[string]*pickbot.RatingsOverride
	log func(string)
}

func (c *ratingsCache) get(a seatAgent) (*pickbot.RatingsOverride, error) {
	if a.kind != "ovr" {
		return nil, nil
	}
	key := a.path + "#" + a.field
	if ov, ok := c.m[key]; ok {
		return ov, nil
	}
	b, err := os.ReadFile(a.path)
	if err != nil {
		return nil, err
	}
	ov, err := pickbot.ParseRatingsOverride(b, a.path, a.field, c.set)
	if err != nil {
		return nil, err
	}
	if c.log != nil {
		c.log(fmt.Sprintf("ratings override: %s field %s; %d 17Lands-rated cards missing (neutral)", a.path, a.field, len(ov.Missing)))
	}
	c.m[key] = ov
	return ov, nil
}

// newLeagueDrafter resolves one seat's drafter.
func newLeagueDrafter(lab *draftlab.Lab, sp leagueSpec, set string, rc *ratingsCache) (draftlab.NamedDrafter, error) {
	if sp.kind != "" {
		k, _ := lookupDrafterKind(sp.kind)
		d, ref, err := k(lab, sp.arg, set)
		if err != nil {
			return draftlab.NamedDrafter{}, fmt.Errorf("league %s: %w", sp.text, err)
		}
		return draftlab.NamedDrafter{Ref: ref, Drafter: d}, nil
	}
	d, err := lab.Drafter(draftlab.AgentRef{Name: "rating-colour"})
	if err != nil {
		return d, err
	}
	ov, err := rc.get(sp.agent)
	if err != nil {
		return d, err
	}
	if ov != nil {
		d.Drafter.(*pickbot.RatingColour).Ratings = ov
	}
	return d, nil
}

// pickRec is one recorded pick.
type pickRec struct {
	Pod       int       `json:"pod"`
	PodSeed   uint64    `json:"pod_seed"`
	Seat      int       `json:"seat"`
	Round     int       `json:"round"`
	PickNo    int       `json:"pick_no"`
	Pick      int       `json:"pick"` // the seat's 0-based pick ordinal over the draft
	Drafter   string    `json:"drafter"`
	Pack      []string  `json:"pack"` // card names in pack order
	Chosen    string    `json:"chosen"`
	ChosenIdx int       `json:"chosen_idx"`
	Logits    []float32 `json:"logits,omitempty"`
	// Temp and LogProb are the behaviour policy of a sampled pick
	// (BehaviourSource); HasBehaviour is false for a deterministic one.
	HasBehaviour bool               `json:"has_behaviour,omitempty"`
	Temp         float32            `json:"temp,omitempty"`
	LogProb      float32            `json:"logp,omitempty"`
	Features     draftfeat.Features `json:"features"`

	drafterIdx int
	packIdx    []int32 // table index per pack card, -1 when absent
	inBin      []byte  // net seats: the pick's learner.bin record body (InputsBinSource)
}

// InputsBinSource is a net drafter that keeps its last pick's encoded
// network inputs, logits, chosen index and behaviour log-prob
// (appendLearnerBin), for the PPO trainer.
type InputsBinSource interface {
	LastInputsBin() []byte
}

// recordingDrafter wraps one seat's drafter: before each pick it extracts
// the pick's features from the PickView alone (the seat's legal view), then
// delegates and records what was taken. It consumes no randomness of its
// own, so wrapping never moves a pick.
type recordingDrafter struct {
	inner      pickbot.Drafter
	table      *draftfeat.SetTable
	ext        draftfeat.Extractor
	pod, seat  int
	podSeed    uint64
	drafter    string
	drafterIdx int
	n          int
	out        *[]pickRec
}

func (d *recordingDrafter) Pick(v pickbot.PickView, r *rand.Rand) string {
	id, _ := d.pick(v, r, false)
	return id
}

// PickErr keeps a wrapped draftlab.FallibleDrafter fallible.
func (d *recordingDrafter) PickErr(v pickbot.PickView, r *rand.Rand) (string, error) {
	return d.pick(v, r, true)
}

func (d *recordingDrafter) pick(v pickbot.PickView, r *rand.Rand, fallible bool) (string, error) {
	f := d.ext.Extract(v, d.table)
	var id string
	if fd, ok := d.inner.(draftlab.FallibleDrafter); ok && fallible {
		var err error
		if id, err = fd.PickErr(v, r); err != nil {
			return "", err
		}
	} else {
		id = d.inner.Pick(v, r)
	}
	rec := pickRec{
		Pod: d.pod, PodSeed: d.podSeed, Seat: d.seat, Round: v.Round, PickNo: v.PickNo, Pick: d.n,
		Drafter: d.drafter, ChosenIdx: -1, Features: f, drafterIdx: d.drafterIdx,
	}
	d.n++
	rec.Pack = make([]string, len(v.Pack))
	rec.packIdx = make([]int32, len(v.Pack))
	for i, c := range v.Pack {
		rec.Pack[i] = c.Name
		rec.packIdx[i] = int32(d.table.Index(c.Name))
		if c.ID == id {
			rec.ChosenIdx, rec.Chosen = i, c.Name
		}
	}
	if ls, ok := d.inner.(LogitSource); ok {
		if l := ls.LastLogits(); l != nil {
			rec.Logits = append([]float32(nil), l...)
			if bs, ok := d.inner.(BehaviourSource); ok {
				rec.Temp, rec.LogProb, rec.HasBehaviour = bs.LastBehaviour()
			}
		}
	}
	if is, ok := d.inner.(InputsBinSource); ok {
		if b := is.LastInputsBin(); b != nil {
			rec.inBin = append([]byte(nil), b...)
		}
	}
	*d.out = append(*d.out, rec)
	return id, nil
}

// pairOf is a main deck's true colour pair: the two colours with the most
// non-land main cards by colour identity (ties to the lower WUBRG index),
// as a draftfeat.Pairs index; -1 for a deck with fewer than one coloured
// card. The label is training-only (the seat auxiliary loss).
func pairOf(lab *draftlab.Lab, main []string) (int, [draftfeat.NColours]int) {
	var counts [draftfeat.NColours]int
	for _, n := range main {
		c, ok := lab.Card(n)
		if !ok || len(c.Faces) == 0 || c.Faces[0].IsLand() {
			continue
		}
		id := c.ColourIdentity()
		for i := 0; i < draftfeat.NColours; i++ {
			if id&(1<<uint(i)) != 0 {
				counts[i]++
			}
		}
	}
	idx := []int{0, 1, 2, 3, 4}
	sort.SliceStable(idx, func(a, b int) bool { return counts[idx[a]] > counts[idx[b]] })
	if counts[idx[0]] == 0 {
		return -1, counts
	}
	a, b := idx[0], idx[1]
	if a > b {
		a, b = b, a
	}
	for p, pr := range draftfeat.Pairs {
		if pr[0] == a && pr[1] == b {
			return p, counts
		}
	}
	return -1, counts
}

func pairName(p int) string {
	if p < 0 {
		return ""
	}
	const wubrg = "WUBRG"
	return string(wubrg[draftfeat.Pairs[p][0]]) + string(wubrg[draftfeat.Pairs[p][1]])
}
