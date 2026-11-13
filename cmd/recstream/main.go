// Command recstream converts JSON Lines record history to and from the
// condensed form of package testkit/recstream (delta frames, keyframes,
// gzip), and reads any of the forms.
//
//	recstream pack [-k 8192] [-level 6] [-rm] FILE...
//	    write FILE's records to STEM.cjsonl.gz beside it (STEM is FILE less
//	    .jsonl or .jsonl.gz), read both back and compare them record by
//	    record, and only then keep the output. -rm deletes FILE after that
//	    check passes; without it FILE is never touched.
//	recstream unpack [-o OUT] FILE      records as JSON Lines (default stdout)
//	recstream cat FILE...               the same, for several files of any form
//	recstream verify A B                A and B hold the same records
//	recstream measure FILE...           raw, gzip, frame and condensed sizes
//
// The exit status is 1 on any error or mismatch, 2 on a usage error.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/adams-shaun/gorge/testkit/recstream"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch cmd, args := os.Args[1], os.Args[2:]; cmd {
	case "pack":
		err = pack(args)
	case "unpack":
		err = unpack(args)
	case "cat":
		err = cat(args)
	case "verify":
		err = verifyCmd(args)
	case "measure":
		err = measure(args)
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "recstream:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: recstream pack|unpack|cat|verify|measure [flags] FILE...")
	os.Exit(2)
}

// Stem strips a JSON Lines extension.
func stem(path string) string {
	for _, ext := range []string{recstream.Ext, ".jsonl.gz", ".jsonl", ".gz"} {
		if strings.HasSuffix(path, ext) {
			return strings.TrimSuffix(path, ext)
		}
	}
	return path
}

func pack(args []string) error {
	fs := flag.NewFlagSet("pack", flag.ExitOnError)
	k := fs.Int("k", 8192, "records per keyframe block (a block also ends at 2 MiB of record text)")
	level := fs.Int("level", 6, "gzip level")
	rm := fs.Bool("rm", false, "delete each input once its output verifies")
	fs.Parse(args)
	var in, out int64
	for _, path := range fs.Args() {
		dst := stem(path) + recstream.Ext
		if dst == path {
			return fmt.Errorf("%s is already condensed", path)
		}
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("%s exists", dst)
		}
		st, err := packFile(path, dst, recstream.Options{Keyframe: *k, Level: *level})
		if err != nil {
			return err
		}
		n, err := verify(path, dst)
		if err != nil {
			os.Remove(dst)
			return fmt.Errorf("verify %s: %w (output removed)", dst, err)
		}
		if n != st.Records {
			os.Remove(dst)
			return fmt.Errorf("verify %s: %d records read back, %d written (output removed)", dst, n, st.Records)
		}
		a, b := size(path), size(dst)
		in, out = in+a, out+b
		fmt.Printf("%s\t%d records\t%d keyframes\t%d -> %d bytes\t%.2fx\tverified\n", dst, st.Records, st.Keyframes, a, b, ratio(a, b))
		if *rm {
			if err := os.Remove(path); err != nil {
				return err
			}
		}
	}
	if len(fs.Args()) > 1 {
		fmt.Printf("total\t%d -> %d bytes\t%.2fx\n", in, out, ratio(in, out))
	}
	return nil
}

func packFile(src, dst string, opt recstream.Options) (recstream.Stats, error) {
	s, err := recstream.Open(src)
	if err != nil {
		return recstream.Stats{}, err
	}
	defer s.Close()
	tmp := dst + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return recstream.Stats{}, err
	}
	bw := bufio.NewWriterSize(f, 1<<20)
	w, err := recstream.NewWriter(bw, opt)
	if err == nil {
		err = copyRecords(w, s)
	}
	if err == nil {
		err = w.Close()
	}
	if err == nil {
		err = bw.Flush()
	}
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, dst)
	}
	if err != nil {
		os.Remove(tmp)
		return recstream.Stats{}, err
	}
	return w.Stats(), nil
}

func copyRecords(w *recstream.Writer, s *recstream.Scanner) error {
	for {
		rec, err := s.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := w.WriteRecord(rec); err != nil {
			return err
		}
	}
}

// verify compares two files record by record and returns the count.
func verify(a, b string) (int, error) {
	sa, err := recstream.Open(a)
	if err != nil {
		return 0, err
	}
	defer sa.Close()
	sb, err := recstream.Open(b)
	if err != nil {
		return 0, err
	}
	defer sb.Close()
	for n := 0; ; n++ {
		ra, ea := sa.Next()
		ra = append([]byte(nil), ra...)
		rb, eb := sb.Next()
		if errors.Is(ea, io.EOF) && errors.Is(eb, io.EOF) {
			return n, nil
		}
		if ea != nil || eb != nil {
			return n, fmt.Errorf("record %d: %v / %v", n+1, ea, eb)
		}
		if !bytes.Equal(ra, rb) {
			return n, fmt.Errorf("record %d differs", n+1)
		}
	}
}

func verifyCmd(args []string) error {
	if len(args) != 2 {
		usage()
	}
	n, err := verify(args[0], args[1])
	if err != nil {
		return err
	}
	fmt.Printf("%d records equal\n", n)
	return nil
}

func unpack(args []string) error {
	fs := flag.NewFlagSet("unpack", flag.ExitOnError)
	out := fs.String("o", "", "output file (default stdout)")
	fs.Parse(args)
	if fs.NArg() != 1 {
		usage()
	}
	if *out == "" {
		return catTo(os.Stdout, fs.Args())
	}
	f, err := os.Create(*out)
	if err != nil {
		return err
	}
	if err := catTo(f, fs.Args()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func cat(args []string) error { return catTo(os.Stdout, args) }

func catTo(dst io.Writer, paths []string) error {
	bw := bufio.NewWriterSize(dst, 1<<20)
	for _, path := range paths {
		r, err := recstream.OpenReader(path)
		if err != nil {
			return err
		}
		_, err = io.Copy(bw, r)
		r.Close()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return bw.Flush()
}

type counter struct{ n int64 }

func (c *counter) Write(p []byte) (int, error) { c.n += int64(len(p)); return len(p), nil }

func measure(args []string) error {
	fs := flag.NewFlagSet("measure", flag.ExitOnError)
	k := fs.Int("k", 8192, "records per keyframe block (a block also ends at 2 MiB of record text)")
	level := fs.Int("level", 6, "gzip level")
	fs.Parse(args)
	fmt.Println("file\trecords\traw\tgzip\tframes\tcondensed\tgzip_x\tcondensed_x\tverbatim_blocks")
	var tr, tg, tf, tc int64
	for _, path := range fs.Args() {
		s, err := recstream.Open(path)
		if err != nil {
			return err
		}
		var raw, gzc, cond counter
		gz, _ := gzip.NewWriterLevel(&gzc, *level)
		w, err := recstream.NewWriter(&cond, recstream.Options{Keyframe: *k, Level: *level})
		if err != nil {
			return err
		}
		for {
			rec, err := s.Next()
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			raw.n += int64(len(rec)) + 1
			gz.Write(rec)
			gz.Write([]byte{'\n'})
			if err := w.WriteRecord(rec); err != nil {
				return err
			}
		}
		s.Close()
		gz.Close()
		w.Close()
		st := w.Stats()
		fmt.Printf("%s\t%d\t%d\t%d\t%d\t%d\t%.2f\t%.2f\t%d/%d\n", filepath.Base(path), st.Records, raw.n, gzc.n, st.FrameBytes, cond.n, ratio(raw.n, gzc.n), ratio(raw.n, cond.n), st.VerbatimBlocks, st.Blocks)
		tr, tg, tf, tc = tr+raw.n, tg+gzc.n, tf+st.FrameBytes, tc+cond.n
	}
	fmt.Printf("TOTAL\t-\t%d\t%d\t%d\t%d\t%.2f\t%.2f\n", tr, tg, tf, tc, ratio(tr, tg), ratio(tr, tc))
	return nil
}

func size(path string) int64 {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return fi.Size()
}

func ratio(a, b int64) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}
