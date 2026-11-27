package feat

import (
	"fmt"
	"sort"
	"sync"

	"github.com/adams-shaun/gorge/dirac"
)

// kernelenc.go is the dirac-side half of the state-encoder contract: the
// kernel front end of an encoder, and a registry of them keyed by Name. The
// gorge-side half (a view front end and its own registry) lives in
// bots/encoder; dirac's core imports nothing from gorge, so the two do not
// share a type. They share the output Schema instead: Schema here has the
// same fields and the same Hash as bots/encoder.Schema, and a test in this
// package asserts the registries agree for every name in both.

// Schema is the output layout of one encoder; see bots/encoder.Schema, which
// it mirrors field for field.
type Schema struct {
	Name       string // registry key: "entity", "entity-pod"
	Version    uint32 // bump on any output change
	DenseW     int    // state dense width
	SparseRows int    // hashed row space of the sparse bags
	RawW       int    // entity raw width (0: no entity list)
	OptDenseW  int    // option dense width
	ExtW       int    // extension block width (0: none)
	Hidden     bool   // reads hidden input; never seatable

	// Pinned, when non-zero, is the Hash of the schema: the digest every
	// net trained under this encoder already carries.
	Pinned uint64
}

// Hash is the schema digest: Pinned when set, else FNV-1a over every field in
// a fixed order (strings length-prefixed, integers as 8 little-endian bytes).
// It is bots/encoder.Schema.Hash.
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

// KernelEncoder is the dirac front end of an encoder: it reads a kernel
// position and writes the policynet-layout state and options. One per
// goroutine; EncodeState's result is valid until the next call, and neither
// method allocates once the encoder is warm.
type KernelEncoder interface {
	Schema() Schema
	// EncodeState encodes s at decision d for d's player.
	EncodeState(s *dirac.State, d *dirac.Decision) *State
	// EncodeOptions encodes d's options, in d's option order, onto dst.
	EncodeOptions(d *dirac.Decision, dst []Option) []Option
}

// KernelSpec is a registered kernel encoder: its Schema and a constructor
// for one per-goroutine instance over a pool.
type KernelSpec struct {
	Schema Schema
	New    func(pool *Pool) KernelEncoder
}

var (
	kernelMu     sync.RWMutex
	kernelByName = map[string]KernelSpec{}
	kernelSorted []KernelSpec // by Name; rebuilt on RegisterKernel
)

// RegisterKernel adds sp under sp.Schema.Name. Call it from init. An empty
// name, a nil constructor or a duplicate name panics.
func RegisterKernel(sp KernelSpec) {
	name := sp.Schema.Name
	if name == "" || sp.New == nil {
		panic("feat: RegisterKernel needs a name and a constructor")
	}
	kernelMu.Lock()
	defer kernelMu.Unlock()
	if _, dup := kernelByName[name]; dup {
		panic(fmt.Sprintf("feat: kernel encoder %q registered twice", name))
	}
	kernelByName[name] = sp
	kernelSorted = append(kernelSorted, sp)
	sort.Slice(kernelSorted, func(i, j int) bool { return kernelSorted[i].Schema.Name < kernelSorted[j].Schema.Name })
}

// LookupKernel returns the kernel encoder registered under name.
func LookupKernel(name string) (KernelSpec, bool) {
	kernelMu.RLock()
	defer kernelMu.RUnlock()
	sp, ok := kernelByName[name]
	return sp, ok
}

// ListKernel returns every registered kernel encoder sorted by name. The
// slice is a copy.
func ListKernel() []KernelSpec {
	kernelMu.RLock()
	defer kernelMu.RUnlock()
	return append([]KernelSpec(nil), kernelSorted...)
}

// Pinned digests of the two sets feat implements: policynet.EncoderHashFor
// of FeaturesEntity and FeaturesPod, which every shipped net carries.
const (
	pinnedEntityHash = 0x9eff0a5fec167119
	pinnedPodHash    = 0x3ae74ef8f511ce19
)

// entityEncoder is *Encoder as a KernelEncoder; the methods are the
// Encoder's own, so the interface adds one dispatch per call and no work.
type entityEncoder struct {
	*Encoder
	schema Schema
}

func (e entityEncoder) Schema() Schema { return e.schema }

func entitySchema() Schema {
	return Schema{
		Name: "entity", Version: 1, DenseW: DenseWidth, SparseRows: TableRows,
		RawW: EntityRawWidth, OptDenseW: OptionDenseWidth, Pinned: pinnedEntityHash,
	}
}

func podSchema() Schema {
	return Schema{
		Name: "entity-pod", Version: 1, DenseW: DenseWidthPod, SparseRows: TableRows,
		RawW: EntityRawWidthPod, OptDenseW: OptionDenseWidth, Pinned: pinnedPodHash,
	}
}

func init() {
	RegisterKernel(KernelSpec{Schema: entitySchema(), New: func(pool *Pool) KernelEncoder {
		return entityEncoder{Encoder: NewEncoder(pool), schema: entitySchema()}
	}})
	RegisterKernel(KernelSpec{Schema: podSchema(), New: func(pool *Pool) KernelEncoder {
		e := NewEncoder(pool)
		e.SetPod(true)
		return entityEncoder{Encoder: e, schema: podSchema()}
	}})
}
