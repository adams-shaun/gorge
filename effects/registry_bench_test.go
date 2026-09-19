package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
)

var benchmarkEffectCalls int

func benchmarkEffect(Host, *Ctx, *cards.SA) { benchmarkEffectCalls++ }

func benchmarkResolve(b *testing.B, sa *cards.SA) {
	b.Helper()
	h := &fakeHost{}
	c := &Ctx{}
	benchmarkEffectCalls = 0
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		Resolve(h, c, sa)
	}
	b.StopTimer()
	if benchmarkEffectCalls != b.N {
		b.Fatalf("effect ran %d times, want %d", benchmarkEffectCalls, b.N)
	}
}

func BenchmarkResolveKnownCompiledAPI(b *testing.B) {
	Register("Draw", benchmarkEffect)
	b.Cleanup(func() { Register("Draw", effDraw) })
	sa := &cards.SA{Kind: "SP", API: "Draw"}
	r := cards.NewRegistry()
	r.Add(&cards.Card{Faces: []*cards.Face{{Abilities: []*cards.SA{sa}}}})
	if err := r.CompileMetadata(); err != nil {
		b.Fatal(err)
	}
	benchmarkResolve(b, sa)
}

func BenchmarkResolveKnownTextAPI(b *testing.B) {
	Register("Draw", benchmarkEffect)
	b.Cleanup(func() { Register("Draw", effDraw) })
	benchmarkResolve(b, &cards.SA{Kind: "SP", API: "Draw"})
}

func BenchmarkResolveUnknownAPI(b *testing.B) {
	const name = "BenchmarkExtensionAPI"
	Register(name, benchmarkEffect)
	b.Cleanup(func() { unregister(name) })
	benchmarkResolve(b, &cards.SA{Kind: "SP", API: name})
}
