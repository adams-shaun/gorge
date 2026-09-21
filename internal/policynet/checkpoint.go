package policynet

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
)

// The checkpoint is the versioned binary serialization of a trained Model
// (R1 §3.6): magic, schema version, the ENCODER's golden hash id, the
// geometry, then the float32 weight blocks little-endian. The encoder hash
// is the load-bearing field: a checkpoint trained against one encoder
// contract must refuse to load against a drifted one, because a silent
// feature-space change turns a trained scorer into noise.
//
// Layout (little-endian throughout):
//
//	"GPOL"                    4 bytes magic
//	uint32                    schema version (CheckpointVersion)
//	uint64                    EncoderHash() at training time
//	uint32 × 6                rows, H, hidden, stateDenseW, optSlotW, optDenseW
//	float32 × rows·H          Table
//	float32 × DenseWidth·H    StateW
//	float32 × H               StateB
//	float32 × hidden·inW      HidW
//	float32 × hidden          HidB
//	float32 × hidden          OutW
//	float32 × 1               OutB
//	float32 × 1               ResidualW (schema version 2+)
//
// The loader rejects a wrong magic, an unknown version, an encoder-hash
// mismatch, a geometry that does not match the encoder contract (rows must
// be TableRows; the dense/slot widths are encoder constants) and a truncated
// or overlong body.
//
// Schema version 2 appends ResidualW (the bot-prior residual weight).
// Version 1 checkpoints are refused rather than silently loaded with
// ResidualW 0: the version bump is the tripwire that says the body grew.
const (
	CheckpointMagic   = "GPOL"
	CheckpointVersion = 2
)

// EncoderHash is the encoder contract's golden hash id: a FNV-1a 64 over
// every vocabulary and layout constant the encoders read — the table and
// dense widths, the step/phase/zone/pool/option-kind/type/mode/keyword
// vocabularies in their fixed orders, and four pinned hash ids off the
// canonical feature strings. Any drift in the hash function, a string
// format, a vocabulary or a layout moves this value, so a checkpoint
// trained before the drift refuses to load (TestEncoderHashPinned holds the
// literal). The full encoding goldens (policynet_test.go) pin the encodings
// themselves; this hash is the wire-contract digest a checkpoint carries.
func EncoderHash() uint64 {
	var b strings.Builder
	b.WriteString("gorge-policynet-encoder-v1\x1f")
	fmt.Fprintf(&b, "rows=%d\ndense=%d\nslots=%d\noptdense=%d\n",
		TableRows, DenseWidth, OptionSlotWidth, OptionDenseWidth)
	writeVocab := func(name string, vs []string) {
		b.WriteString(name)
		b.WriteByte('=')
		b.WriteString(strings.Join(vs, ","))
		b.WriteByte('\n')
	}
	writeVocab("steps", stepNames[:])
	writeVocab("phases", phaseNames[:])
	writeVocab("zones", zoneTagList)
	writeVocab("pool", poolKeys[:])
	writeVocab("optkinds", optionKinds[:])
	writeVocab("types", cardTypeBits[:])
	writeVocab("modes", modeValues[:])
	writeVocab("keywords", keywordBits[:])
	fmt.Fprintf(&b, "pins=%d,%d,%d,%d\n",
		hashID("card|player"),
		hashID("optkind|pass"),
		hashID("Swords to Plowshares|hand"),
		hashID("Swords to Plowshares|t0|a0|s0|c-"))
	s := b.String()
	h := uint64(14695981039346656037)
	for i := 0; i < len(s); i++ {
		h ^= uint64(s[i])
		h *= 1099511628211
	}
	return h
}

// zoneTagList mirrors the zoneTag* constants in policynet.go (a var so the
// hash can read them in one fixed order).
var zoneTagList = []string{
	zoneTagHand, zoneTagBattleMe, zoneTagBattleOpp, zoneTagGraveMe,
	zoneTagGraveOpp, zoneTagExileMe, zoneTagExileOpp, zoneTagStack,
	zoneTagCommand, zoneTagCommanders,
}

// WriteCheckpoint serialises m to w.
func WriteCheckpoint(m *Model, w io.Writer) error {
	bw := bufio.NewWriter(w)
	var hdr [24]byte
	copy(hdr[0:4], CheckpointMagic)
	binary.LittleEndian.PutUint32(hdr[4:8], CheckpointVersion)
	binary.LittleEndian.PutUint64(hdr[8:16], EncoderHash())
	binary.LittleEndian.PutUint32(hdr[16:20], uint32(m.Rows))
	binary.LittleEndian.PutUint32(hdr[20:24], uint32(m.H))
	if _, err := bw.Write(hdr[:]); err != nil {
		return err
	}
	var b4 [4]byte
	put32 := func(v uint32) error {
		binary.LittleEndian.PutUint32(b4[:], v)
		_, err := bw.Write(b4[:])
		return err
	}
	for _, v := range []uint32{uint32(m.Hidden), uint32(DenseWidth), uint32(OptionSlotWidth), uint32(OptionDenseWidth)} {
		if err := put32(v); err != nil {
			return err
		}
	}
	for _, block := range [][]float32{m.Table, m.StateW, m.StateB, m.HidW, m.HidB, m.OutW, {m.OutB}, {m.ResidualW}} {
		if err := writeFloats(bw, block); err != nil {
			return err
		}
	}
	return bw.Flush()
}

func writeFloats(w io.Writer, fs []float32) error {
	// Batched through a local buffer to keep the write count low without
	// changing a single byte of the format.
	const chunk = 1024
	var big [chunk * 4]byte
	for i := 0; i < len(fs); i += chunk {
		n := len(fs) - i
		if n > chunk {
			n = chunk
		}
		for k := 0; k < n; k++ {
			binary.LittleEndian.PutUint32(big[k*4:], math.Float32bits(fs[i+k]))
		}
		if _, err := w.Write(big[:n*4]); err != nil {
			return err
		}
	}
	return nil
}

// LoadCheckpoint reads a checkpoint. Every mismatch is a hard error naming
// what mismatched; a checkpoint that passes is a Model ready to Score.
func LoadCheckpoint(r io.Reader) (*Model, error) {
	br := bufio.NewReader(r)
	var hdr [24]byte
	if _, err := io.ReadFull(br, hdr[:]); err != nil {
		return nil, fmt.Errorf("checkpoint header: %w", err)
	}
	if string(hdr[0:4]) != CheckpointMagic {
		return nil, fmt.Errorf("checkpoint: bad magic %q, want %q", hdr[0:4], CheckpointMagic)
	}
	if v := binary.LittleEndian.Uint32(hdr[4:8]); v != CheckpointVersion {
		return nil, fmt.Errorf("checkpoint: unsupported schema version %d, want %d", v, CheckpointVersion)
	}
	if h := binary.LittleEndian.Uint64(hdr[8:16]); h != EncoderHash() {
		return nil, fmt.Errorf("checkpoint: encoder hash %#x does not match this build's encoder %#x — the encoder drifted since this checkpoint was trained", h, EncoderHash())
	}
	rows := int(binary.LittleEndian.Uint32(hdr[16:20]))
	hh := int(binary.LittleEndian.Uint32(hdr[20:24]))
	get32 := func(what string) (int, error) {
		var b4 [4]byte
		if _, err := io.ReadFull(br, b4[:]); err != nil {
			return 0, fmt.Errorf("checkpoint geometry (%s): %w", what, err)
		}
		return int(binary.LittleEndian.Uint32(b4[:])), nil
	}
	hidden, err := get32("hidden")
	if err != nil {
		return nil, err
	}
	stateW, err := get32("state dense width")
	if err != nil {
		return nil, err
	}
	slotW, err := get32("option slot width")
	if err != nil {
		return nil, err
	}
	optDenseW, err := get32("option dense width")
	if err != nil {
		return nil, err
	}
	if rows != TableRows {
		return nil, fmt.Errorf("checkpoint geometry: table rows %d, want %d", rows, TableRows)
	}
	if stateW != DenseWidth {
		return nil, fmt.Errorf("checkpoint geometry: state dense width %d, want %d", stateW, DenseWidth)
	}
	if slotW != OptionSlotWidth {
		return nil, fmt.Errorf("checkpoint geometry: option slot width %d, want %d", slotW, OptionSlotWidth)
	}
	if optDenseW != OptionDenseWidth {
		return nil, fmt.Errorf("checkpoint geometry: option dense width %d, want %d", optDenseW, OptionDenseWidth)
	}
	if hh <= 0 || hidden <= 0 {
		return nil, fmt.Errorf("checkpoint geometry: h %d hidden %d", hh, hidden)
	}
	m := &Model{
		Rows:   rows,
		H:      hh,
		Hidden: hidden,
		InW:    2*hh + slotW + optDenseW,
	}
	blocks := []struct {
		name string
		n    int
		set  func(fs []float32)
	}{
		{"table", rows * hh, func(fs []float32) { m.Table = fs }},
		{"state projection", DenseWidth * hh, func(fs []float32) { m.StateW = fs }},
		{"state bias", hh, func(fs []float32) { m.StateB = fs }},
		{"hidden", hidden * m.InW, func(fs []float32) { m.HidW = fs }},
		{"hidden bias", hidden, func(fs []float32) { m.HidB = fs }},
		{"output", hidden, func(fs []float32) { m.OutW = fs }},
		{"output bias", 1, func(fs []float32) { m.OutB = fs[0] }},
		{"residual weight", 1, func(fs []float32) { m.ResidualW = fs[0] }},
	}
	for _, blk := range blocks {
		fs, err := readFloats(br, blk.n)
		if err != nil {
			return nil, fmt.Errorf("checkpoint %s block: %w", blk.name, err)
		}
		blk.set(fs)
	}
	// The body must end exactly at the last weight: a trailing byte means
	// the writer and this reader disagree about the layout.
	if n, err := br.ReadByte(); err != io.EOF {
		if err != nil {
			return nil, fmt.Errorf("checkpoint trailing: %w", err)
		}
		return nil, fmt.Errorf("checkpoint: %d trailing byte(s) after the weights — layout mismatch", n+1)
	}
	return m, nil
}

func readFloats(r io.Reader, n int) ([]float32, error) {
	buf := make([]byte, n*4)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	fs := make([]float32, n)
	for i := range fs {
		fs[i] = math.Float32frombits(binary.LittleEndian.Uint32(buf[i*4:]))
	}
	return fs, nil
}

// SaveCheckpoint writes m to path atomically: encode into a sibling
// temporary file, sync, close, rename over the destination.
func (m *Model) SaveCheckpoint(path string) (err error) {
	tmp, err := os.CreateTemp(dirOf(path), "."+baseOf(path)+"-*.tmp")
	if err != nil {
		return fmt.Errorf("creating checkpoint temporary file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpName)
		}
	}()
	if err = WriteCheckpoint(m, tmp); err != nil {
		return fmt.Errorf("writing checkpoint: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("syncing checkpoint: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("closing checkpoint: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publishing checkpoint: %w", err)
	}
	return nil
}

// LoadCheckpointFile loads a checkpoint from a path.
func LoadCheckpointFile(path string) (*Model, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening checkpoint: %w", err)
	}
	defer f.Close()
	return LoadCheckpoint(f)
}

func dirOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return "."
}

func baseOf(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
