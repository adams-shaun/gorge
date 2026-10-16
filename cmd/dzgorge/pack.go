package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/contracts"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// runPack converts visit corpora (play -corpus, recorded under -record-features entity)
// into dzg's training shards (gorge/dzg/SPEC.md §2): one directory of .npy arrays per
// shard of -shard records. The layout is contract dzg-pack (docs/contracts.md):
// -format 2, the default, writes dzg-pack-2; -format 1 writes the deprecated
// dzg-pack-1 layout byte for byte (first-seen kind indices, no encoder hash in
// meta.json) and is kept for one release.
//
// Every record of every input must agree on record_type, schema_version and
// encoder_hash: corpora recorded under different feature sets are refused, never
// mixed. Under -format 2 a record that does not decode is an error naming its
// file and line (-allow-truncated keeps what came before a truncated gzip tail,
// the mark of a run killed mid-write).
func runPack(args []string) int {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	in := fs.String("in", "", "comma-separated visit corpora (.jsonl.gz)")
	out := fs.String("out", "", "output directory; shards go to OUT/00000, OUT/00001, ...")
	shard := fs.Int("shard", 200000, "records per shard")
	format := fs.Int("format", contracts.DZGPackVersion, "dzg-pack major to write: 2, or 1 (deprecated: first-seen kind indices, no encoder hash)")
	allowTrunc := fs.Bool("allow-truncated", false, "-format 2: keep the records before a truncated gzip tail instead of failing")
	fs.Parse(args)
	if *in == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "pack: -in and -out are required")
		return 2
	}
	if *format != 1 && *format != 2 {
		fmt.Fprintf(os.Stderr, "pack: -format %d: want 2 (dzg-pack-2) or 1 (deprecated dzg-pack-1)\n", *format)
		return 2
	}
	if *format == 1 {
		fmt.Fprintln(os.Stderr, "pack: -format 1 (dzg-pack-1) is deprecated and will be removed after one release; readers should move to dzg-pack-2")
	}
	w := &shardWriter{dir: *out, per: *shard, source: *in, format: *format, allowTrunc: *allowTrunc}
	w.reset()
	for _, path := range strings.Split(*in, ",") {
		if err := packCorpus(path, w); err != nil {
			fmt.Fprintf(os.Stderr, "pack: %v\n", err)
			return 1
		}
	}
	if err := w.flush(); err != nil {
		fmt.Fprintln(os.Stderr, "pack:", err)
		return 1
	}
	fmt.Printf("packed %d records (%d without a known outcome, %d not entity-encoded) into %d shards under %s\n",
		w.total, w.unknown, w.noEntity, w.shards, *out)
	return 0
}

// packCorpus adds every record of one corpus file, one JSON record per line.
func packCorpus(path string, w *shardWriter) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	zr, err := gzip.NewReader(bufio.NewReaderSize(f, 1<<20))
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	br := bufio.NewReaderSize(zr, 1<<20)
	for line := 1; ; line++ {
		raw, rerr := br.ReadBytes('\n')
		if rerr != nil && rerr != io.EOF {
			// The stream itself broke (a gzip member cut short by a killed run).
			if w.format == 1 || w.allowTrunc {
				fmt.Fprintf(os.Stderr, "pack: %s:%d: stopped at a truncated stream (%v)\n", path, line, rerr)
				return nil
			}
			return fmt.Errorf("%s:%d: %w (a run killed mid-write leaves a truncated tail; -allow-truncated keeps what came before)", path, line, rerr)
		}
		if len(bytes.TrimSpace(raw)) > 0 {
			var rec policynet.VisitRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				if w.format == 1 {
					// dzg-pack-1 behaviour: keep what came before.
					fmt.Fprintf(os.Stderr, "pack: %s: stopped at a bad record (%v)\n", path, err)
					return nil
				}
				return fmt.Errorf("%s:%d: bad record: %w", path, line, err)
			}
			if err := w.check(&rec, fmt.Sprintf("%s:%d", path, line)); err != nil {
				return err
			}
			if err := w.add(&rec); err != nil {
				return fmt.Errorf("%s:%d: %w", path, line, err)
			}
		}
		if rerr == io.EOF {
			return nil
		}
	}
}

// check holds rec to the corpus identity the first record set: one record type,
// schema version and encoder hash across every input.
func (w *shardWriter) check(rec *policynet.VisitRecord, at string) error {
	if rec.RecordType != policynet.VisitRecordType || rec.SchemaVersion != policynet.VisitSchemaVersion {
		return fmt.Errorf("%s: record_type %q schema_version %d: this build reads %s schema_version %d only",
			at, rec.RecordType, rec.SchemaVersion, policynet.VisitRecordType, policynet.VisitSchemaVersion)
	}
	if w.firstAt == "" {
		w.recordType, w.schemaVersion, w.encoderHash, w.firstAt = rec.RecordType, rec.SchemaVersion, rec.EncoderHash, at
		return nil
	}
	if rec.EncoderHash != w.encoderHash {
		return fmt.Errorf("%s: encoder_hash %s, but %s has encoder_hash %s: corpora recorded under different feature sets cannot share a pack",
			at, rec.EncoderHash, w.firstAt, w.encoderHash)
	}
	return nil
}

type shardWriter struct {
	dir, source string
	per         int
	format      int  // the dzg-pack major written: 2, or the deprecated 1
	allowTrunc  bool // -allow-truncated
	// the corpus identity every record must share (check)
	recordType, encoderHash, firstAt string
	schemaVersion                    int
	p                                packer
	// per-record training columns (SPEC §2's second table)
	outcome, rootValue, cVisits, cPrior, cQ []float32
	turn, sims, choice, game, candOff       []int32
	coOff, coOpt                            []int32
	seat, subset, kind                      []uint8
	games                                   []string
	gameIdx                                 map[string]int32
	kinds                                   map[string]uint8
	kindNames                               []string
	total, unknown, noEntity, shards        int
}

func (w *shardWriter) reset() {
	w.p.reset()
	w.outcome, w.rootValue, w.cVisits, w.cPrior, w.cQ = nil, nil, nil, nil, nil
	w.turn, w.sims, w.choice, w.game = nil, nil, nil, nil
	w.candOff, w.coOff, w.coOpt = []int32{0}, []int32{0}, nil
	w.seat, w.subset, w.kind = nil, nil, nil
	w.games, w.gameIdx = nil, map[string]int32{}
	if w.kinds == nil {
		w.kinds = map[string]uint8{}
	}
}

func (w *shardWriter) add(rec *policynet.VisitRecord) error {
	if len(rec.State.Cards) == 0 {
		w.noEntity++ // an mz corpus has no entity cards; an entity one always shows a hand or a board
	}
	st := policynet.State{Dense: rec.State.Dense}
	for i := range rec.State.Rows {
		st.Sparse = append(st.Sparse, policynet.Feature{Row: rec.State.Rows[i], Value: rec.State.Vals[i]})
	}
	for _, c := range rec.State.Cards {
		ec := policynet.EntityCard{Group: c.Group, Raw: c.Raw}
		for i := range c.Rows {
			ec.Rows = append(ec.Rows, policynet.Feature{Row: c.Rows[i], Value: c.Vals[i]})
		}
		st.Cards = append(st.Cards, ec)
	}
	opts := make([]policynet.Option, len(rec.Options))
	for i, o := range rec.Options {
		op := policynet.Option{Dense: o.Dense, BotPick: o.BotPick, EntA: o.EntA, EntB: o.EntB}
		for j := range o.SlotsR {
			op.Slots = append(op.Slots, policynet.Feature{Row: o.SlotsR[j], Value: o.SlotsV[j]})
		}
		for j := range o.HashedR {
			op.Hashed = append(op.Hashed, policynet.Feature{Row: o.HashedR[j], Value: o.HashedV[j]})
		}
		opts[i] = op
	}
	w.p.add(&st, opts)

	oc := float32(-1)
	if rec.OutcomeKnown {
		oc = float32(rec.Outcome)
	} else {
		w.unknown++
	}
	w.outcome = append(w.outcome, oc)
	w.rootValue = append(w.rootValue, float32(rec.RootValue))
	w.turn = append(w.turn, rec.Turn)
	w.sims = append(w.sims, int32(rec.Sims))
	w.choice = append(w.choice, int32(rec.Choice))
	w.seat = append(w.seat, uint8(rec.Seat))
	sub := uint8(0)
	if rec.Subset {
		sub = 1
	}
	w.subset = append(w.subset, sub)
	k, err := w.kindIndex(string(rec.Kind))
	if err != nil {
		return err
	}
	w.kind = append(w.kind, k)
	g, ok := w.gameIdx[rec.GameID]
	if !ok {
		g = int32(len(w.games))
		w.gameIdx[rec.GameID] = g
		w.games = append(w.games, rec.GameID)
	}
	w.game = append(w.game, g)
	for c, members := range rec.Cands {
		at := func(x []float64) float32 {
			if c < len(x) {
				return float32(x[c])
			}
			return 0
		}
		v := float32(0)
		if c < len(rec.Visits) {
			v = float32(rec.Visits[c])
		}
		w.cVisits = append(w.cVisits, v)
		w.cPrior = append(w.cPrior, at(rec.Prior))
		w.cQ = append(w.cQ, at(rec.Q))
		for _, m := range members {
			w.coOpt = append(w.coOpt, int32(m))
		}
		w.coOff = append(w.coOff, int32(len(w.coOpt)))
	}
	w.candOff = append(w.candOff, int32(len(w.cVisits)))
	w.total++
	if w.p.states() >= w.per {
		return w.flush()
	}
	return nil
}

// kindIndex is kind's kind-column value: its contracts.PackKinds index under
// dzg-pack-2, its first-seen index in this run under the deprecated dzg-pack-1.
func (w *shardWriter) kindIndex(kind string) (uint8, error) {
	if w.format != 1 {
		i := contracts.PackKindIndex(kind)
		if i < 0 {
			return 0, fmt.Errorf("decision kind %q has no index in the dzg-pack kind table (contracts.PackKinds): append it there", kind)
		}
		return uint8(i), nil
	}
	k, ok := w.kinds[kind]
	if !ok {
		k = uint8(len(w.kindNames))
		w.kinds[kind] = k
		w.kindNames = append(w.kindNames, kind)
	}
	return k, nil
}

func (w *shardWriter) flush() error {
	n := w.p.states()
	if n == 0 {
		return nil
	}
	dir := filepath.Join(w.dir, fmt.Sprintf("%05d", w.shards))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	p := &w.p
	arrays := []struct {
		name string
		data any
		cols int
	}{
		{"dense", p.dense, policynet.DenseWidth}, {"sp_off", p.spOff, 0}, {"sp_row", p.spRow, 0}, {"sp_val", p.spVal, 0},
		{"card_off", p.cardOff, 0}, {"card_group", p.cardGroup, 0}, {"card_raw", p.cardRaw, policynet.EntityRawWidth},
		{"cr_off", p.crOff, 0}, {"cr_row", p.crRow, 0}, {"cr_val", p.crVal, 0},
		{"opt_off", p.optOff, 0}, {"opt_dense", p.optDense, policynet.OptionDenseWidth}, {"opt_bot", p.optBot, 0},
		{"opt_ea", p.optEA, 0}, {"opt_eb", p.optEB, 0},
		{"os_off", p.osOff, 0}, {"os_row", p.osRow, 0}, {"os_val", p.osVal, 0},
		{"oh_off", p.ohOff, 0}, {"oh_row", p.ohRow, 0}, {"oh_val", p.ohVal, 0},
		{"outcome", w.outcome, 0}, {"root_value", w.rootValue, 0}, {"turn", w.turn, 0}, {"seat", w.seat, 0},
		{"subset", w.subset, 0}, {"kind", w.kind, 0}, {"sims", w.sims, 0}, {"choice", w.choice, 0}, {"game", w.game, 0},
		{"cand_off", w.candOff, 0}, {"cand_visits", w.cVisits, 0}, {"cand_prior", w.cPrior, 0}, {"cand_q", w.cQ, 0},
		{"co_off", w.coOff, 0}, {"co_opt", w.coOpt, 0},
	}
	for _, a := range arrays {
		if err := writeNpy(filepath.Join(dir, a.name+".npy"), a.data, a.cols); err != nil {
			return err
		}
	}
	m := map[string]any{"format": "dzg-pack-1", "n": n, "games": w.games, "kinds": w.kindNames, "source": w.source}
	if w.format != 1 {
		m["format"] = contracts.Format(contracts.DZGPack, contracts.DZGPackVersion)
		m["kinds"] = contracts.PackKinds
		m["encoder_hash"], m["record_type"], m["schema_version"] = w.encoderHash, w.recordType, w.schemaVersion
		m["gorge"] = buildGorge
	}
	meta, _ := json.MarshalIndent(m, "", " ")
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), meta, 0o644); err != nil {
		return err
	}
	w.shards++
	w.reset()
	return nil
}

// writeNpy writes a 1-D array, or a 2-D one of cols columns, as a version 1.0 .npy file.
func writeNpy(path string, data any, cols int) error {
	var descr string
	var n int
	var raw []byte
	switch x := data.(type) {
	case []float32:
		descr, n = "<f4", len(x)
		raw = make([]byte, 4*n)
		for i, v := range x {
			binary.LittleEndian.PutUint32(raw[4*i:], math.Float32bits(v))
		}
	case []int32:
		descr, n = "<i4", len(x)
		raw = make([]byte, 4*n)
		for i, v := range x {
			binary.LittleEndian.PutUint32(raw[4*i:], uint32(v))
		}
	case []uint16:
		descr, n = "<u2", len(x)
		raw = make([]byte, 2*n)
		for i, v := range x {
			binary.LittleEndian.PutUint16(raw[2*i:], v)
		}
	case []uint8:
		descr, n = "|u1", len(x)
		raw = append([]byte(nil), x...)
	default:
		return fmt.Errorf("writeNpy: unsupported %T", data)
	}
	shape := fmt.Sprintf("(%d,)", n)
	if cols > 0 {
		if n%cols != 0 {
			return fmt.Errorf("writeNpy %s: %d values not a multiple of %d", path, n, cols)
		}
		shape = fmt.Sprintf("(%d, %d)", n/cols, cols)
	}
	hdr := fmt.Sprintf("{'descr': '%s', 'fortran_order': False, 'shape': %s, }", descr, shape)
	total := 10 + len(hdr) + 1
	hdr += strings.Repeat(" ", (64-total%64)%64) + "\n"
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	bw.WriteString("\x93NUMPY\x01\x00")
	var l [2]byte
	binary.LittleEndian.PutUint16(l[:], uint16(len(hdr)))
	bw.Write(l[:])
	bw.WriteString(hdr)
	bw.Write(raw)
	if err := bw.Flush(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
