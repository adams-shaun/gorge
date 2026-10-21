// Package netsimd is a SIMD forward pass for the mlp value network
// (bots/gonet's network file): the same arithmetic as gonet.Net.eval, with the
// weights laid out transposed and padded to 64 bytes per row so every layer
// runs as whole-layer AVX-512 kernels (kern_amd64.s), and a scalar Go
// fallback (kern_generic.go) with identical semantics for every other CPU.
//
// It exists for the FDN microkernel plan's phase 4 microbenchmark: how much
// faster can one evaluation be without changing the network, its inputs or
// the search. The hot path allocates nothing after the first call per
// scratch; the package reads no clock (timing lives in the benchmark).
//
// A caller that knows which cards are unchanged between evaluations can
// precompute them (cards.go): CardIdentity is a card's per-identity half,
// CardVector its whole vector, CardStore a fixed, concurrent-safe table of
// either by a caller-owned slot, and EvalCards the network over a mix of
// precomputed and computed cards -- bit-identical to Eval.
package netsimd
