package main

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/oracle"
)

// kstats (-stats): the peak occupancy of each of the kernel's bounded pools
// over the run, sampled at every decision the lockstep poses
// (dirac.State.Occupancy through oracle.OccupancyHook), with the first
// pair that reached it. A game stops at its first divergence, so a
// diverging pair is sampled only up to it.
var statsOn bool

type kstats struct {
	pair int
	peak [dirac.NumPools]int
	at   [dirac.NumPools]int
	occ  dirac.Occupancy
}

func (k *kstats) install() {
	for i := range k.at {
		k.at[i] = -1
	}
	oracle.OccupancyHook = func(s *dirac.State) {
		s.Occupancy(&k.occ)
		for i, n := range k.occ.N {
			if n > k.peak[i] {
				k.peak[i], k.at[i] = n, k.pair
			}
		}
	}
}

func (k *kstats) print(set string) {
	fmt.Printf("%s-kstats: pool peak/cap (first pair at peak)\n", set)
	for i := range k.peak {
		c := fmt.Sprint(dirac.PoolCaps[i])
		if dirac.PoolCaps[i] == 0 {
			c = "-"
		}
		fmt.Printf("%s-kstats: %-9s %4d / %-4s (pair %d)\n", set, dirac.PoolNames[i], k.peak[i], c, k.at[i])
	}
}
