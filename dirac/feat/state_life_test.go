package feat

import (
	"testing"

	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/engine/view"
)

// lifeView is the minimal two-seat view the kernel's life rows are checked
// against: seat 0 (the viewer) at meLife, seat 1 at oppLife.
func lifeView(meLife, oppLife int32) view.View {
	return view.View{
		Viewer: 0,
		Players: []view.PlayerView{
			{ID: 0, Name: "me", Life: meLife},
			{ID: 1, Name: "opp", Life: oppLife},
		},
	}
}

func sparseHasRow(st policynet.State, r uint16) bool {
	for _, f := range st.Sparse {
		if f.Row == r {
			return true
		}
	}
	return false
}

// TestLifeBucketRowsMatchPolicynet: the kernel's precomputed life rows must
// equal policynet's hashed mz|me|life= / mz|opp|life= tokens for every bucket,
// including the widened 24..61 range (40 is Commander's starting life), and
// the rows table must actually grow with the clamp.
func TestLifeBucketRowsMatchPolicynet(t *testing.T) {
	if got := int32(len(rowsMeLife)); got != lifeBucketHi+1 {
		t.Fatalf("rowsMeLife has %d entries, want lifeBucketHi+1 = %d", got, lifeBucketHi+1)
	}
	if got := int32(len(rowsOppLife)); got != lifeBucketHi+1 {
		t.Fatalf("rowsOppLife has %d entries, want lifeBucketHi+1 = %d", got, lifeBucketHi+1)
	}
	if rowsMeLife[clampI(40/2, lifeBucketHi)] == rowsMeLife[clampI(24/2, lifeBucketHi)] {
		t.Fatal("rowsMeLife buckets 20 and 12 are equal: 40 and 24 life are not distinguished")
	}

	for _, life := range []int32{0, 23, 24, 25, 26, 40, 60, 61, 80} {
		// Seat 0 at life, opponent at 20.
		v := lifeView(life, 20)
		if v.Players[0].Life != life || v.Players[1].Life != 20 {
			t.Fatalf("setup: lives %d/%d, want %d/20", v.Players[0].Life, v.Players[1].Life, life)
		}
		wantMe := rowsMeLife[clampI(life/2, lifeBucketHi)]
		if !sparseHasRow(policynet.EncodeStateWith(policynet.FeaturesEntity, v, 0, nil), wantMe) {
			t.Fatalf("life %d: kernel me row %d not in policynet's sparse bag", life, wantMe)
		}

		// The opponent at life, seat 0 at 20.
		v = lifeView(20, life)
		wantOpp := rowsOppLife[clampI(life/2, lifeBucketHi)]
		if !sparseHasRow(policynet.EncodeStateWith(policynet.FeaturesEntity, v, 0, nil), wantOpp) {
			t.Fatalf("life %d: kernel opp row %d not in policynet's sparse bag", life, wantOpp)
		}
	}
}
