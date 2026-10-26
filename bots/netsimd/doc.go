// Package netsimd is a SIMD forward pass for the mlp value network
// (bots/gonet's network file): the same arithmetic as gonet.Net.eval, with the
// weights laid out transposed and padded to 64 bytes per row so every layer
// runs as whole-layer AVX-512 kernels (kern_amd64.s), and a scalar Go
// fallback (kern_generic.go) with identical semantics for every other CPU.
//
// Shapes: any number of trunk blocks (the trainer's `layers`), any policy
// and value head depth, d/ff/card_d/head_hidden any multiples of 32 (d_emb
// any width). d == head_hidden == 64 is the production shape and runs the
// dedicated 64-lane kernels (ln64/dot64/addv64); every other width runs the
// width-generic kernels (lnN/dotN/addvN/poolAccN, and the bag at any width),
// AVX-512 at every multiple of 16 lanes. Every assembly kernel is
// bit-identical to its scalar twin, so ForceScalar changes no bit at any
// shape; against gonet (float64 layernorm and dot) the logits agree to fp32
// rounding (TestShapesMatchGonet, TestParityWithGonet).
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
