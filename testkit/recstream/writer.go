package recstream

import (
	"bytes"
	"compress/gzip"
	"errors"
	"io"
)

// Header is the first line of every condensed stream.
const Header = "#recstream/1"

// Ext is the file extension of a condensed stream.
const Ext = ".cjsonl.gz"

// Options tune a Writer. The zero value takes the defaults.
type Options struct {
	Keyframe   int  // records per block, each block opening with a keyframe (default 8192)
	BlockBytes int  // record text per block before an early keyframe (default 2 MiB)
	Level      int  // gzip level (default 6)
	NoAdaptive bool // always write frames, never fall back to a verbatim block
}

func (o Options) norm() Options {
	if o.Keyframe <= 0 {
		o.Keyframe = 8192
	}
	if o.BlockBytes <= 0 {
		o.BlockBytes = 2 << 20
	}
	if o.Level == 0 {
		o.Level = 6
	}
	return o
}

// A Writer writes records to a condensed stream. It is also an io.Writer
// that takes JSON Lines text (so a json.Encoder can write to it directly):
// every '\n' ends a record. A block of records is buffered and written as
// one gzip member when it fills, and by Close, which does not close the
// underlying writer.
type Writer struct {
	dst     io.Writer
	opt     Options
	gz      *gzip.Writer
	zbuf    bytes.Buffer
	head    bool   // the header is still to be written
	n       int    // records in the current block
	bytes   int    // record text in the current block
	blk     []byte // the block's frames
	vblk    []byte // the block's records as verbatim keyframes
	prev    *node
	partial []byte
	err     error
	stats   Stats
}

// Stats counts what a Writer wrote.
type Stats struct {
	Records, Keyframes, Deltas int
	Blocks, VerbatimBlocks     int
	RecordBytes, FrameBytes    int64
}

// NewWriter returns a Writer on dst.
func NewWriter(dst io.Writer, opt Options) (*Writer, error) {
	opt = opt.norm()
	gz, err := gzip.NewWriterLevel(io.Discard, opt.Level)
	if err != nil {
		return nil, err
	}
	return &Writer{dst: dst, opt: opt, gz: gz, head: true}, nil
}

// Reset discards w's state and starts a new stream on dst, header first,
// reusing w's buffers and gzip state. A tool that writes one gzip member per
// game resets onto each game's buffer: concatenated, such
// streams are one valid condensed stream, the later headers reading as
// comments. Stats keep counting across a Reset.
func (w *Writer) Reset(dst io.Writer) {
	w.dst, w.head, w.err = dst, true, nil
	w.n, w.bytes = 0, 0
	w.blk, w.vblk, w.partial = w.blk[:0], w.vblk[:0], w.partial[:0]
	w.prev = nil
}

// Stats returns the counts so far.
func (w *Writer) Stats() Stats { return w.stats }

// Write takes JSON Lines text; each '\n'-terminated line is one record.
func (w *Writer) Write(p []byte) (int, error) {
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			w.partial = append(w.partial, p...)
			break
		}
		line := p[:i]
		if len(w.partial) > 0 {
			line = append(w.partial, line...)
			w.partial = w.partial[:0]
		}
		if err := w.WriteRecord(line); err != nil {
			return 0, err
		}
		p = p[i+1:]
	}
	return n, nil
}

// keyStyle is the frame byte of a keyframe patch (against {}) per style.
var keyStyle = map[style]byte{compact: ':', spaced: ';'}

// WriteRecord writes one record. rec must not contain '\n'. The Writer
// keeps no reference to rec.
func (w *Writer) WriteRecord(rec []byte) error {
	if w.err != nil {
		return w.err
	}
	if bytes.IndexByte(rec, '\n') >= 0 {
		return errors.New("recstream: record contains a newline")
	}
	if w.n >= w.opt.Keyframe || w.bytes >= w.opt.BlockBytes {
		if w.err = w.flushBlock(); w.err != nil {
			return w.err
		}
	}
	txt := append([]byte(nil), rec...)
	cur, err := parse(txt)
	var st style
	ok := err == nil
	if ok {
		st, ok = deltaable(cur, txt)
	}
	mark := len(w.blk)
	if ok {
		base := w.prev
		if w.n == 0 {
			base = nil
		}
		if p, dok := diff(base, cur); dok && len(p)+1 < len(txt) {
			c := byte(st)
			if base == nil {
				c = keyStyle[st]
				w.stats.Keyframes++
			} else {
				w.stats.Deltas++
			}
			w.blk = append(append(w.blk, c), p...)
		}
	}
	if len(w.blk) == mark {
		w.blk = append(append(w.blk, '='), txt...)
		w.stats.Keyframes++
	}
	w.blk = append(w.blk, '\n')
	if !w.opt.NoAdaptive {
		w.vblk = append(append(append(w.vblk, '='), txt...), '\n')
	}
	w.stats.FrameBytes += int64(len(w.blk) - mark)
	w.prev = nil
	if ok {
		w.prev = cur
	}
	w.n++
	w.bytes += len(rec)
	w.stats.Records++
	w.stats.RecordBytes += int64(len(rec)) + 1
	return nil
}

// flushBlock writes the block as one gzip member: its frames or, when
// smaller, the same records as verbatim keyframes (records that share
// little with their neighbours compress better whole).
func (w *Writer) flushBlock() error {
	if w.n == 0 && !w.head {
		return nil
	}
	pick := w.blk
	if !w.opt.NoAdaptive && w.n > 1 {
		a, err := w.compress(w.blk)
		if err != nil {
			return err
		}
		b, err := w.compress(w.vblk)
		if err != nil {
			return err
		}
		if b < a {
			pick = w.vblk
			w.stats.VerbatimBlocks++
		}
	}
	w.zbuf.Reset()
	w.gz.Reset(&w.zbuf)
	if w.head {
		io.WriteString(w.gz, Header+"\n")
		w.head = false
	}
	if _, err := w.gz.Write(pick); err != nil {
		return err
	}
	if err := w.gz.Close(); err != nil {
		return err
	}
	w.stats.Blocks++
	if _, err := w.dst.Write(w.zbuf.Bytes()); err != nil {
		return err
	}
	w.blk, w.vblk = w.blk[:0], w.vblk[:0]
	w.n, w.bytes = 0, 0
	return nil
}

// compress returns the gzip size of frames.
func (w *Writer) compress(frames []byte) (int, error) {
	w.zbuf.Reset()
	w.gz.Reset(&w.zbuf)
	if _, err := w.gz.Write(frames); err != nil {
		return 0, err
	}
	if err := w.gz.Close(); err != nil {
		return 0, err
	}
	return w.zbuf.Len(), nil
}

// Close writes any unterminated last record and the last block. A Writer
// can be Reset after Close.
func (w *Writer) Close() error {
	if len(w.partial) > 0 && w.err == nil {
		w.err = w.WriteRecord(w.partial)
		w.partial = w.partial[:0]
	}
	if w.err == nil {
		w.err = w.flushBlock()
	}
	return w.err
}
