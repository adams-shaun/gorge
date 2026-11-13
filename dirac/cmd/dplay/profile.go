package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// startProfile starts the -cpuprofile CPU profile and arms the -memprofile
// heap profile; the returned stop writes both and, with -memprofile, prints
// the run's allocation totals to stderr. Neither flag set costs nothing.
func startProfile(cpu, mem string) (stop func()) {
	var cf *os.File
	if cpu != "" {
		f, err := os.Create(cpu)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cpuprofile:", err)
		} else if err := pprof.StartCPUProfile(f); err != nil {
			fmt.Fprintln(os.Stderr, "cpuprofile:", err)
			f.Close()
		} else {
			cf = f
		}
	}
	return func() {
		if cf != nil {
			pprof.StopCPUProfile()
			cf.Close()
		}
		if mem == "" {
			return
		}
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(os.Stderr, "memstats: mallocs=%d total_alloc=%d num_gc=%d heap_sys=%d\n", ms.Mallocs, ms.TotalAlloc, ms.NumGC, ms.HeapSys)
		f, err := os.Create(mem)
		if err != nil {
			fmt.Fprintln(os.Stderr, "memprofile:", err)
			return
		}
		defer f.Close()
		if err := pprof.Lookup("allocs").WriteTo(f, 0); err != nil {
			fmt.Fprintln(os.Stderr, "memprofile:", err)
		}
	}
}
