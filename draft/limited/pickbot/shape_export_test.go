package pickbot_test

import (
	"testing"

	pb "github.com/adams-shaun/gorge/draft/limited/pickbot"
)

func TestShapeExports(t *testing.T) {
	reg := corpus(t)
	lions := lookupCard(t, reg, "Savannah Lions").Faces[0]
	if got := pb.ClassName(lions); got != "creature" {
		t.Errorf("ClassName(Savannah Lions) = %q, want creature", got)
	}
	growth := lookupCard(t, reg, "Giant Growth").Faces[0]
	if got := pb.ClassName(growth); got != "instant" {
		t.Errorf("ClassName(Giant Growth) = %q, want instant", got)
	}

	for _, tc := range []struct{ mv, want int }{
		{0, 0}, {1, 0}, {2, 1}, {5, 4}, {6, 5}, {9, 5},
	} {
		if got := pb.CurveBucket(tc.mv); got != tc.want {
			t.Errorf("CurveBucket(%d) = %d, want %d", tc.mv, got, tc.want)
		}
	}

	cove := lookupCard(t, reg, "Tranquil Cove").Faces[0]
	if got, want := pb.LandColours(cove), uint8(0b00011); got != want {
		t.Errorf("LandColours(Tranquil Cove) = %05b, want %05b", got, want)
	}
	plains := lookupCard(t, reg, "Plains").Faces[0]
	if got, want := pb.LandColours(plains), uint8(0b00001); got != want {
		t.Errorf("LandColours(Plains) = %05b, want %05b", got, want)
	}
}
