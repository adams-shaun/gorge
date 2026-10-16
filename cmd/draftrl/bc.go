package main

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/draftlab"
	"github.com/adams-shaun/gorge/limited/draftfeat"
	"github.com/adams-shaun/gorge/limited/draftnet"
	"github.com/adams-shaun/gorge/limited/pickbot"
	"github.com/adams-shaun/gorge/limited/setdata"
)

// The bc subcommand (DR-8, spec §3.2): draft-only pods for behaviour
// cloning. Every seat is scored by the teacher (rating-colour with the
// teacher's ratings): at each pick the teacher's score for every pack card
// is recorded as that pick's logits in picks.bin (hasLogits = 1), whatever
// the seat actually picks. Each pod seats -teachers argmax teachers and
// fills the rest from -variants, so the states the teacher labels are
// diverse. Every seat's pool is built with best23 on the teacher's ratings
// and seats.jsonl carries its true colour pair, the seat-head label. No
// games are played.

// kindMu serialises DrafterKind constructors, which the registry contract
// calls serially.
var kindMu sync.Mutex

// bcSalt tags a pod's seat-assignment stream.
const bcSalt = 0xD4A7E8

type bcOpts struct {
	corpus, set, collation string
	seed                   uint64
	pods, teachers         int
	teacher                seatAgent
	variants               []bcVariant
	table                  string
	workers                int
	jsonl                  bool
	dumpInputs             int
	inputsBin              bool
	out                    string
}

// bcVariant is one non-teacher seat kind.
type bcVariant struct {
	text string
	kind string  // "teacher", "temp", "eps", "cfm", "17l", "random", or a registered DrafterKind
	x    float64 // temperature, epsilon or ColourFitMax
	arg  string  // a DrafterKind's argument
}

func parseBCVariants(s string) ([]bcVariant, error) {
	var out []bcVariant
	for _, tok := range strings.Split(s, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		v := bcVariant{text: tok}
		kind, arg, hasArg := strings.Cut(tok, ":")
		v.kind = kind
		switch kind {
		case "temp", "eps", "cfm":
			if !hasArg {
				return nil, fmt.Errorf("variant %q: want %s:X", tok, kind)
			}
			x, err := strconv.ParseFloat(arg, 64)
			if err != nil || x < 0 || (kind == "eps" && x > 1) || (kind == "temp" && x == 0) {
				return nil, fmt.Errorf("variant %q: bad value", tok)
			}
			v.x = x
		case "17l", "random", "teacher":
			if hasArg {
				return nil, fmt.Errorf("variant %q takes no argument", tok)
			}
		default:
			if _, ok := lookupDrafterKind(kind); ok && hasArg {
				v.arg = arg
				break
			}
			return nil, fmt.Errorf("unknown variant %q (want teacher | temp:T | eps:E | cfm:K | 17l | random | KIND:ARG, a registered drafter kind such as net:WEIGHTS)", tok)
		}
		out = append(out, v)
	}
	return out, nil
}

func parseBC(args []string) (bcOpts, error) {
	var o bcOpts
	var teacher, variants string
	fs := flag.NewFlagSet("draftrl bc", flag.ContinueOnError)
	fs.StringVar(&o.corpus, "corpus", ".cards", "card corpus directory")
	fs.StringVar(&o.set, "set", "FDN", "set code")
	fs.StringVar(&o.collation, "collation", string(setdata.DefaultCollation), "booster collation")
	fs.Uint64Var(&o.seed, "seed", 3_000_000, "first pod seed; pod n uses seed+n (training seeds, disjoint from the 201+ gate seeds)")
	fs.IntVar(&o.pods, "pods", 1, "draft-only 4-seat pods")
	fs.IntVar(&o.teachers, "teachers", 2, "argmax teacher seats per pod (the rest draw from -variants)")
	fs.StringVar(&teacher, "teacher", "", "the teacher's ratings: 17l | ovr:PATH#FIELD (required); also every seat's best23 builder")
	fs.StringVar(&variants, "variants", "temp:3,eps:0.15,cfm:4,cfm:12,17l,random", "non-teacher seat kinds, drawn uniformly: teacher | temp:T (sample softmax(score/T)) | eps:E (uniform random with prob E) | cfm:K (rating-colour, ColourFitMax K) | 17l | random")
	fs.StringVar(&o.table, "table", "", "draftfeat SetTable JSON; empty = rollout's stand-in (facts + z-scored teacher ratings)")
	fs.IntVar(&o.workers, "workers", 4, "pods drafted at once (records are written in pod order)")
	fs.BoolVar(&o.jsonl, "jsonl", false, "also write picks.jsonl (large)")
	fs.BoolVar(&o.inputsBin, "inputs-bin", false, "write every pick's draftnet.Inputs (Go's Fill; card facts by table index) and the teacher's scores to inputs.bin, the trainer's compact input (see writeBCInputsBin)")
	fs.IntVar(&o.dumpInputs, "dump-inputs", 0, "write the first N picks' draftnet.Inputs (Go's Fill) to inputs.jsonl, for the trainer's parity check")
	fs.StringVar(&o.out, "out", "", "output directory (required)")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	if teacher == "" {
		return o, fmt.Errorf("-teacher is required")
	}
	t, err := parseSeatAgents(teacher, false)
	if err != nil || len(t) != 1 || (t[0].kind != "17l" && t[0].kind != "ovr") {
		return o, fmt.Errorf("-teacher %q: want one 17l or ovr:PATH#FIELD (%v)", teacher, err)
	}
	o.teacher = t[0]
	if o.variants, err = parseBCVariants(variants); err != nil {
		return o, fmt.Errorf("-variants: %w", err)
	}
	switch {
	case o.pods < 1:
		return o, fmt.Errorf("-pods must be at least 1")
	case o.teachers < 0 || o.teachers > 4:
		return o, fmt.Errorf("-teachers must be 0-4")
	case o.teachers < 4 && len(o.variants) == 0:
		return o, fmt.Errorf("-variants is empty but %d seats need one", 4-o.teachers)
	case o.workers < 1:
		return o, fmt.Errorf("-workers must be at least 1")
	case o.out == "":
		return o, fmt.Errorf("-out is required")
	}
	return o, nil
}

// bcDrafterNames is the picks.bin drafter list: index 0 is the argmax
// teacher, then each variant in -variants order.
func (o bcOpts) drafterNames() []string {
	out := []string{"teacher"}
	for _, v := range o.variants {
		out = append(out, v.text)
	}
	return out
}

// bcSeats draws a pod's seat kinds: -teachers argmax teachers and uniform
// variants, placed by a shuffle, all from rand.NewPCG(podSeed, bcSalt).
// The result is the drafter index per seat (0 = teacher).
func bcSeats(o bcOpts, podSeed uint64) [4]int {
	r := rand.New(rand.NewPCG(podSeed, bcSalt))
	var out [4]int
	for s := o.teachers; s < 4; s++ {
		out[s] = 1 + r.IntN(len(o.variants))
	}
	r.Shuffle(4, func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out
}

// teacherScored wraps a seat's drafter: before each pick it records the
// teacher's scores for every pack card (LogitSource), then delegates.
// With dump set it also keeps Go's draftnet encoding of the pick.
type teacherScored struct {
	teacher *pickbot.RatingColour
	inner   pickbot.Drafter
	scores  []float64
	last    []float32

	dump  *[]bcInputs
	limit int
	bin   *[]byte
	table *draftfeat.SetTable
	ex    draftfeat.Extractor
	in    draftnet.Inputs
	seat  int
	pod   int
	n     int
}

func (d *teacherScored) Pick(v pickbot.PickView, r *rand.Rand) string {
	d.scores = d.teacher.Scores(v, d.scores)
	d.last = d.last[:0]
	for _, x := range d.scores {
		d.last = append(d.last, float32(x))
	}
	wantDump := d.dump != nil && len(*d.dump) < d.limit
	if wantDump || d.bin != nil {
		f := d.ex.Extract(v, d.table)
		d.in.Fill(&f, v, d.table)
		if wantDump {
			*d.dump = append(*d.dump, newBCInputs(d.pod, d.seat, d.n, &d.in))
		}
		if d.bin != nil {
			*d.bin = appendBCInputsBin(*d.bin, &d.in, d.last)
		}
	}
	d.n++
	return d.inner.Pick(v, r)
}

func (d *teacherScored) LastLogits() []float32 { return d.last }

// bcVariantDrafter is a teacher-derived variant that samples from or
// perturbs the teacher's own scores.
type bcVariantDrafter struct {
	teacher *pickbot.RatingColour
	v       bcVariant
	scores  []float64
}

func (d *bcVariantDrafter) Pick(v pickbot.PickView, r *rand.Rand) string {
	if len(v.Pack) == 0 {
		return ""
	}
	switch d.v.kind {
	case "random":
		return v.Pack[r.IntN(len(v.Pack))].ID
	case "eps":
		if r.Float64() < d.v.x {
			return v.Pack[r.IntN(len(v.Pack))].ID
		}
		d.scores = d.teacher.Scores(v, d.scores)
		return v.Pack[pickbot.BestScore(d.scores, r)].ID
	case "temp":
		d.scores = d.teacher.Scores(v, d.scores)
		mx := math.Inf(-1)
		for _, x := range d.scores {
			mx = math.Max(mx, x)
		}
		sum := 0.0
		for _, x := range d.scores {
			sum += math.Exp((x - mx) / d.v.x)
		}
		u := r.Float64() * sum
		for i, x := range d.scores {
			u -= math.Exp((x - mx) / d.v.x)
			if u < 0 {
				return v.Pack[i].ID
			}
		}
		return v.Pack[len(v.Pack)-1].ID
	}
	panic("bc: variant " + d.v.kind + " is not score-derived")
}

// bcInputs is one inputs.jsonl line: the parity fixture's shape (see
// draftnet's parityCase) plus the pick's address.
type bcInputs struct {
	Pod   int         `json:"pod"`
	Seat  int         `json:"seat"`
	Pick  int         `json:"pick"`
	Ctx   []float32   `json:"ctx"`
	Seats []bcSeatTok `json:"seats"`
	Pool  []bcPoolTok `json:"pool"`
	Cand  []bcCandTok `json:"cand"`
}

type bcSeatTok struct {
	Off  int       `json:"off"`
	Feat []float32 `json:"feat"`
}

type bcPoolTok struct {
	Emb   int32     `json:"emb"`
	Pick  int32     `json:"pick"`
	Facts []float32 `json:"facts"`
}

type bcCandTok struct {
	Emb   int32     `json:"emb"`
	Facts []float32 `json:"facts"`
	Feat  []float32 `json:"feat"`
}

func newBCInputs(pod, seat, pick int, in *draftnet.Inputs) bcInputs {
	b := bcInputs{Pod: pod, Seat: seat, Pick: pick, Ctx: append([]float32(nil), in.Ctx[:]...)}
	for j := 0; j < in.NSeat; j++ {
		b.Seats = append(b.Seats, bcSeatTok{Off: in.SeatOff[j], Feat: append([]float32(nil), in.Seat[j][:]...)})
	}
	for _, p := range in.Pool {
		b.Pool = append(b.Pool, bcPoolTok{Emb: p.Emb, Pick: p.Pick, Facts: append([]float32(nil), p.Facts[:]...)})
	}
	for _, c := range in.Cand {
		b.Cand = append(b.Cand, bcCandTok{Emb: c.Emb, Facts: append([]float32(nil), c.Facts[:]...), Feat: append([]float32(nil), c.Feat[:]...)})
	}
	return b
}

// appendBCInputsBin appends one inputs.bin record (little-endian):
//
//	i32 nSeat, i32 nPool, i32 nCand,
//	CtxDim × f32 ctx,
//	nSeat × (i32 offset, SeatDim × f32),
//	nPool × (i32 table index, i32 pick ordinal),
//	nCand × (i32 table index, CandDim × f32),
//	nCand × f32 teacher score (v.Pack order; the BC target's logits)
//
// Card facts are not repeated: they are a pure function of the table row
// (draftnet fillCard), which the trainer rebuilds from table.json.
func appendBCInputsBin(b []byte, in *draftnet.Inputs, scores []float32) []byte {
	i32 := func(x int32) { b = binary.LittleEndian.AppendUint32(b, uint32(x)) }
	f32 := func(x float32) { b = binary.LittleEndian.AppendUint32(b, math.Float32bits(x)) }
	nc := len(in.Cand)
	i32(int32(in.NSeat))
	i32(int32(len(in.Pool)))
	i32(int32(nc))
	for _, x := range in.Ctx {
		f32(x)
	}
	for j := 0; j < in.NSeat; j++ {
		i32(int32(in.SeatOff[j]))
		for _, x := range in.Seat[j] {
			f32(x)
		}
	}
	for _, p := range in.Pool {
		i32(p.Emb)
		i32(p.Pick)
	}
	for _, c := range in.Cand {
		i32(c.Emb)
		for _, x := range c.Feat {
			f32(x)
		}
	}
	for j := 0; j < nc; j++ {
		var x float32
		if j < len(scores) {
			x = scores[j]
		}
		f32(x)
	}
	return b
}

// bcPod is one drafted pod's records.
type bcPod struct {
	picks  []pickRec
	seats  []seatRec
	inputs []bcInputs
	bin    []byte
	err    error
}

type bcManifest struct {
	draftlab.Manifest
	Pods      int            `json:"pods"`
	Teachers  int            `json:"teachers"`
	Teacher   string         `json:"teacher"`
	Variants  []string       `json:"variants"`
	Builder   string         `json:"builder"`
	Table     string         `json:"table"`
	TableHash string         `json:"table_hash"`
	Picks     int            `json:"picks"`
	Files     []string       `json:"files"`
	PicksBin  map[string]any `json:"picks_bin"`
	WallSec   float64        `json:"wall_sec"`
}

// bcResult is what a bc run wrote, for the tests.
type bcResult struct {
	Picks int
	Seats []seatRec
}

func bc(args []string, out io.Writer) (*bcResult, error) {
	o, err := parseBC(args)
	if err != nil {
		return nil, err
	}
	start := time.Now()
	lab, err := draftlab.Open(o.corpus, draftlab.Options{Workers: 1})
	if err != nil {
		return nil, err
	}
	coll := setdata.Collation(o.collation)
	rc := &ratingsCache{set: o.set, m: map[string]*pickbot.RatingsOverride{}, log: func(s string) { fmt.Fprintln(out, s) }}
	tov, err := rc.get(o.teacher)
	if err != nil {
		return nil, err
	}
	var table *draftfeat.SetTable
	tableSrc := "stand-in: facts + z-scored teacher ratings (rollout defaultSetTable)"
	if o.table != "" {
		b, err := os.ReadFile(o.table)
		if err != nil {
			return nil, err
		}
		if table, err = draftfeat.DecodeSetTable(b); err != nil {
			return nil, err
		}
		tableSrc = o.table
	} else if table, err = defaultSetTable(lab, o.set, coll, tov); err != nil {
		return nil, err
	}
	names := o.drafterNames()
	fmt.Fprintf(out, "bc: set %s (%s), pods %d (seeds %d-%d), teacher rating-colour/%s x%d, variants %s, builder best23/%s, table %s (%s), workers %d\n",
		o.set, coll, o.pods, o.seed, o.seed+uint64(o.pods)-1, o.teacher, o.teachers, strings.Join(names[1:], ","), o.teacher, table.Hash(), tableSrc, o.workers)

	newRC := func(cfm float64, ratings *pickbot.RatingsOverride) (*pickbot.RatingColour, error) {
		d, err := lab.Drafter(draftlab.AgentRef{Name: "rating-colour"})
		if err != nil {
			return nil, err
		}
		rcd := d.Drafter.(*pickbot.RatingColour)
		rcd.Ratings = ratings
		if cfm >= 0 {
			t := pickbot.DefaultTuning
			t.ColourFitMax = cfm
			rcd.Tuning = t
		}
		return rcd, nil
	}

	runPod := func(p int) bcPod {
		podSeed := o.seed + uint64(p)
		packs, err := lab.OpenPod(o.set, coll, podSeed, 4, 3)
		if err != nil {
			return bcPod{err: fmt.Errorf("pod seed %d: %w", podSeed, err)}
		}
		kinds := bcSeats(o, podSeed)
		var podPicks [4][]pickRec
		var inputs []bcInputs
		var bin []byte
		drafters := make([]draftlab.NamedDrafter, 4)
		for s := range drafters {
			teacher, err := newRC(-1, tov)
			if err != nil {
				return bcPod{err: err}
			}
			var inner pickbot.Drafter = teacher
			if k := kinds[s]; k > 0 {
				v := o.variants[k-1]
				switch v.kind {
				case "teacher":
				case "cfm":
					if inner, err = newRC(v.x, tov); err != nil {
						return bcPod{err: err}
					}
				case "17l":
					if inner, err = newRC(-1, nil); err != nil {
						return bcPod{err: err}
					}
				case "temp", "eps", "random":
					inner = &bcVariantDrafter{teacher: teacher, v: v}
				default:
					k, _ := lookupDrafterKind(v.kind)
					kindMu.Lock()
					inner, _, err = k(lab, v.arg, o.set)
					kindMu.Unlock()
					if err != nil {
						return bcPod{err: fmt.Errorf("variant %s: %w", v.text, err)}
					}
				}
			}
			ts := &teacherScored{teacher: teacher, inner: inner, table: table, seat: s, pod: p}
			if p == 0 && o.dumpInputs > 0 {
				ts.dump, ts.limit = &inputs, o.dumpInputs
			}
			if o.inputsBin {
				ts.bin = &bin
			}
			drafters[s] = draftlab.NamedDrafter{
				Ref: draftlab.AgentRef{Name: "bc-" + strings.ReplaceAll(names[kinds[s]], ":", "-")},
				Drafter: &recordingDrafter{inner: ts, table: table, pod: p, seat: s, podSeed: podSeed,
					drafter: names[kinds[s]], drafterIdx: kinds[s], out: &podPicks[s]},
			}
		}
		dr, err := lab.Draft(packs, drafters, podSeed)
		if err != nil {
			return bcPod{err: fmt.Errorf("pod seed %d: %w", podSeed, err)}
		}
		res := bcPod{inputs: inputs, bin: bin}
		for s := 0; s < 4; s++ {
			res.picks = append(res.picks, podPicks[s]...)
		}
		for s, pool := range dr.Pools {
			b, err := lab.Builder(draftlab.AgentRef{Name: "best23"})
			if err != nil {
				return bcPod{err: err}
			}
			if tov != nil {
				b.Builder.(*pickbot.Best23).Ratings = tov
			}
			deck, err := lab.Build(pool, b)
			if err != nil {
				return bcPod{err: fmt.Errorf("pod seed %d seat %d: %w", podSeed, s, err)}
			}
			var main []string
			for _, e := range deck.File.Cards {
				for c := 0; c < e.Count; c++ {
					main = append(main, e.Name)
				}
			}
			sort.Strings(main)
			poolNames := make([]string, len(pool))
			for i, c := range pool {
				poolNames[i] = c.Name
			}
			pair, counts := pairOf(lab, main)
			res.seats = append(res.seats, seatRec{
				Pod: p, PodSeed: podSeed, Seat: s, Drafter: names[kinds[s]], DrafterRef: dr.Drafters[s].String(),
				Builder: "best23/" + o.teacher.String(), Deck: fmt.Sprintf("p%d-s%d", podSeed, s), DeckHash: deckHash(deck),
				Main: main, Pool: poolNames, Pair: pair, PairName: pairName(pair), Colours: counts,
			})
		}
		return res
	}

	if err := os.MkdirAll(o.out, 0o755); err != nil {
		return nil, err
	}
	type sink struct {
		f *os.File
		w *bufio.Writer
	}
	open := func(name string) (*sink, error) {
		f, err := os.Create(filepath.Join(o.out, name))
		if err != nil {
			return nil, err
		}
		return &sink{f: f, w: bufio.NewWriterSize(f, 1<<20)}, nil
	}
	closeAll := func(ss ...*sink) error {
		var first error
		for _, s := range ss {
			if s == nil {
				continue
			}
			if err := s.w.Flush(); err != nil && first == nil {
				first = err
			}
			if err := s.f.Close(); err != nil && first == nil {
				first = err
			}
		}
		return first
	}
	binS, err := open("picks.bin")
	if err != nil {
		return nil, err
	}
	seatS, err := open("seats.jsonl")
	if err != nil {
		return nil, err
	}
	var jsonS, inS, ibS *sink
	files := []string{"picks.bin", "seats.jsonl"}
	if o.jsonl {
		if jsonS, err = open("picks.jsonl"); err != nil {
			return nil, err
		}
		files = append(files, "picks.jsonl")
	}
	if o.dumpInputs > 0 {
		if inS, err = open("inputs.jsonl"); err != nil {
			return nil, err
		}
		files = append(files, "inputs.jsonl")
	}
	if o.inputsBin {
		if ibS, err = open("inputs.bin"); err != nil {
			return nil, err
		}
		files = append(files, "inputs.bin")
	}
	// picks.bin: the header once, then each pod's records in pod order.
	if err := writePicksBin(binS.w, table.Hash(), names, nil); err != nil {
		return nil, err
	}

	// Workers draft pods; the writer emits them strictly in pod order, so
	// the files do not depend on -workers.
	results := make([]chan bcPod, o.pods)
	for i := range results {
		results[i] = make(chan bcPod, 1)
	}
	next := make(chan int)
	var wg sync.WaitGroup
	for w := 0; w < o.workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range next {
				results[p] <- runPod(p)
			}
		}()
	}
	// The dispatcher stays at most 4*workers pods ahead of the writer, so
	// memory is bounded however many pods the run has.
	tokens := make(chan struct{}, 4*o.workers)
	go func() {
		for p := 0; p < o.pods; p++ {
			waitPause()
			tokens <- struct{}{}
			next <- p
		}
		close(next)
	}()
	res := &bcResult{}
	var firstErr error
	lastLog := time.Now()
	for p := 0; p < o.pods; p++ {
		r := <-results[p]
		results[p] = nil
		<-tokens
		if r.err != nil && firstErr == nil {
			firstErr = r.err
		}
		if firstErr != nil {
			continue
		}
		res.Picks += len(r.picks)
		if err := writePickRecords(binS.w, r.picks); err != nil {
			firstErr = err
			continue
		}
		enc := json.NewEncoder(seatS.w)
		for i := range r.seats {
			if err := enc.Encode(&r.seats[i]); err != nil {
				firstErr = err
			}
		}
		if len(res.Seats) < 64 {
			res.Seats = append(res.Seats, r.seats...)
		}
		if jsonS != nil {
			je := json.NewEncoder(jsonS.w)
			for i := range r.picks {
				if err := je.Encode(&r.picks[i]); err != nil {
					firstErr = err
				}
			}
		}
		if inS != nil {
			ie := json.NewEncoder(inS.w)
			for i := range r.inputs {
				if err := ie.Encode(&r.inputs[i]); err != nil {
					firstErr = err
				}
			}
		}
		if ibS != nil {
			if _, err := ibS.w.Write(r.bin); err != nil {
				firstErr = err
			}
		}
		if time.Since(lastLog) > 30*time.Second {
			lastLog = time.Now()
			fmt.Fprintf(out, "bc: %d/%d pods, %.1f pods/s\n", p+1, o.pods, float64(p+1)/time.Since(start).Seconds())
		}
	}
	wg.Wait()
	if err := closeAll(binS, seatS, jsonS, inS, ibS); err != nil && firstErr == nil {
		firstErr = err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	tb, err := table.Encode()
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(o.out, "table.json"), tb, 0o644); err != nil {
		return nil, err
	}
	wall := time.Since(start).Seconds()
	m := bcManifest{
		Manifest: lab.NewManifest("bc", o.seed, []draftlab.RoleRef{
			{Role: draftlab.RoleDrafter, Ref: draftlab.AgentRef{Name: "rating-colour"}},
			{Role: draftlab.RoleBuilder, Ref: draftlab.AgentRef{Name: "best23"}},
		}, nil),
		Pods: o.pods, Teachers: o.teachers, Teacher: o.teacher.String(), Variants: names[1:],
		Builder: "best23/" + o.teacher.String(), Table: tableSrc, TableHash: table.Hash(), Picks: res.Picks,
		Files:    append(files, "table.json", "manifest.json"),
		PicksBin: map[string]any{"version": picksBinVersion, "global": globalLayout, "cand": candLayout, "drafters": names, "logits": "teacher scores (rating-colour Scores)"},
		WallSec:  math.Round(wall*10) / 10,
	}
	m.Set, m.Collation, m.Mode, m.Seats = o.set, o.collation, "draft", 4
	b, err := json.MarshalIndent(m, "", " ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(o.out, "manifest.json"), append(b, '\n'), 0o644); err != nil {
		return nil, err
	}
	fmt.Fprintf(out, "bc: %d pods, %d picks, wall %.1fs (%.1f pods/s); wrote %s\n", o.pods, res.Picks, wall, float64(o.pods)/max(wall, 1e-9), o.out)
	return res, nil
}
