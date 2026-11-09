package policynet

import (
	"bytes"
	"encoding/binary"
	"math/rand/v2"
	"strings"
	"testing"
)

// TestOracleLoadMessagesA24WideCheckpoint: a 24-wide checkpoint is still an
// ordinary (non-oracle) checkpoint, so LoadOracleCheckpoint must point the
// reader at LoadCheckpoint rather than report a drifted encoder.
func TestOracleLoadMessagesA24WideCheckpoint(t *testing.T) {
	m := NewModelOptDense(TableRows, 4, 6, OptionDenseWidthV1, rand.New(rand.NewPCG(7, 8)))
	m.Features = FeaturesMZ
	var buf bytes.Buffer
	if err := WriteCheckpoint(m, &buf); err != nil {
		t.Fatal(err)
	}
	// Precondition: the file really carries the 24-wide hash, not the
	// current-width one.
	if h := binary.LittleEndian.Uint64(buf.Bytes()[8:16]); h != EncoderHashForV1Options(FeaturesMZ) || h == EncoderHashFor(FeaturesMZ) {
		t.Fatalf("checkpoint hash %#x, want the 24-wide %#x (not %#x)", h, EncoderHashForV1Options(FeaturesMZ), EncoderHashFor(FeaturesMZ))
	}
	if _, err := LoadOracleCheckpoint(bytes.NewReader(buf.Bytes())); err == nil ||
		!strings.Contains(err.Error(), "LoadCheckpoint") || strings.Contains(err.Error(), "drifted") {
		t.Fatalf("LoadOracleCheckpoint on a 24-wide checkpoint: %v", err)
	}
	// The ordinary loader still reads the same file.
	if _, err := LoadCheckpoint(bytes.NewReader(buf.Bytes())); err != nil {
		t.Fatalf("LoadCheckpoint on a 24-wide checkpoint: %v", err)
	}
}
