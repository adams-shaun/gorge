// Package encoder is the state-encoder contract: a bot is (policy, encoder,
// net), and a net's header names the encoder it was trained under. An encoder
// is described by its output Schema; the Schema's Hash is what a net file
// carries and what loading checks.
//
// The built-in encoders are the policynet feature sets (builtin.go), each
// registered under its FeatureSet spelling with a Hash equal to
// policynet.EncoderHashFor, so every shipped net keeps loading.
package encoder

import "github.com/adams-shaun/gorge/bots/policynet"

// Schema is the output layout of one encoder.
type Schema struct {
	Name       string // registry key: "entity", "entity-pod", ...
	Version    uint32 // bump on any output change
	DenseW     int    // state dense width
	SparseRows int    // hashed row space of the sparse bags
	RawW       int    // entity raw width (0: no entity list)
	OptDenseW  int    // option dense width
	ExtW       int    // extension block width (0: none)
	Hidden     bool   // reads Diag/omniscient input; never seatable

	// Pinned, when non-zero, is the Hash of the schema. A built-in encoder
	// sets it to its policynet.EncoderHashFor so the digest every existing
	// net carries is unchanged; a new encoder leaves it zero.
	Pinned uint64
}

// Hash is the schema digest: Pinned when set, else FNV-1a over every field in
// a fixed order (strings length-prefixed, integers as 8 little-endian bytes).
func (s Schema) Hash() uint64 {
	if s.Pinned != 0 {
		return s.Pinned
	}
	h := uint64(14695981039346656037)
	word := func(v uint64) {
		for i := 0; i < 8; i++ {
			h ^= v & 0xff
			h *= 1099511628211
			v >>= 8
		}
	}
	word(uint64(len(s.Name)))
	for i := 0; i < len(s.Name); i++ {
		h ^= uint64(s.Name[i])
		h *= 1099511628211
	}
	word(uint64(s.Version))
	word(uint64(s.DenseW))
	word(uint64(s.SparseRows))
	word(uint64(s.RawW))
	word(uint64(s.OptDenseW))
	word(uint64(s.ExtW))
	if s.Hidden {
		word(1)
	} else {
		word(0)
	}
	return h
}

// Diag is the hidden information a Hidden encoder reads (policynet.Diag).
type Diag = policynet.Diag
