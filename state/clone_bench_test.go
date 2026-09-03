package state

import "testing"

func BenchmarkClone(b *testing.B) {
	g := NewGame([]string{"a", "b", "c", "d"})
	c, _ := cardsFixture()
	for p := PlayerID(0); p < 4; p++ {
		var lib []ObjID
		for i := 0; i < 60; i++ {
			lib = append(lib, g.AddObject(c, p).ID)
		}
		g.SetZone(ZLibrary, p, lib)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = g.Clone()
	}
}
