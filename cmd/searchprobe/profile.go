package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

// profiler captures a whole search-probe run without changing the default
// path. The heap profile contains both cumulative allocation and, after the
// forced collection, the live heap retained at the end of the run.
type profiler struct {
	cpuPath string
	memPath string
	cpuFile *os.File
}

func (p *profiler) start() error {
	if p.cpuPath == "" {
		return nil
	}
	f, err := os.Create(p.cpuPath)
	if err != nil {
		return fmt.Errorf("cpuprofile: %w", err)
	}
	p.cpuFile = f
	if err := pprof.StartCPUProfile(f); err != nil {
		_ = f.Close()
		p.cpuFile = nil
		return fmt.Errorf("cpuprofile: %w", err)
	}
	return nil
}

func (p *profiler) finish() error {
	var first error
	if p.cpuFile != nil {
		pprof.StopCPUProfile()
		if err := p.cpuFile.Close(); err != nil {
			first = fmt.Errorf("cpuprofile: %w", err)
		}
		p.cpuFile = nil
	}
	if p.memPath == "" {
		return first
	}
	f, err := os.Create(p.memPath)
	if err != nil {
		if first == nil {
			first = fmt.Errorf("memprofile: %w", err)
		}
		return first
	}
	runtime.GC()
	if err := pprof.Lookup("heap").WriteTo(f, 0); err != nil && first == nil {
		first = fmt.Errorf("memprofile: %w", err)
	}
	if err := f.Close(); err != nil && first == nil {
		first = fmt.Errorf("memprofile: %w", err)
	}
	return first
}
