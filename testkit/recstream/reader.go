package recstream

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
)

// A Scanner reads records from plain JSON Lines, gzipped JSON Lines or a
// condensed stream, telling them apart by content.
type Scanner struct {
	br        *bufio.Reader
	closers   []io.Closer
	condensed bool
	prev      *node
	line      []byte
	out       []byte
	n         int
}

// Open opens path for reading records, whatever its form.
func Open(path string) (*Scanner, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	s, err := NewScanner(f)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	s.closers = append(s.closers, f)
	return s, nil
}

// NewScanner returns a Scanner on r.
func NewScanner(r io.Reader) (*Scanner, error) {
	s := &Scanner{}
	br := bufio.NewReaderSize(r, 1<<16)
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, err
		}
		s.closers = append(s.closers, gz)
		br = bufio.NewReaderSize(gz, 1<<16)
	}
	s.br = br
	if head, _ := br.Peek(len(Header)); string(head) == Header {
		s.condensed = true
	}
	return s, nil
}

// Condensed reports whether the stream is in the condensed form.
func (s *Scanner) Condensed() bool { return s.condensed }

// Close closes what Open opened.
func (s *Scanner) Close() error {
	var first error
	for i := len(s.closers) - 1; i >= 0; i-- {
		if err := s.closers[i].Close(); err != nil && first == nil {
			first = err
		}
	}
	s.closers = nil
	return first
}

// readLine returns the next line without its '\n'. A last line with no
// '\n' is returned too; io.EOF follows.
func (s *Scanner) readLine() ([]byte, error) {
	s.line = s.line[:0]
	for {
		chunk, err := s.br.ReadSlice('\n')
		s.line = append(s.line, chunk...)
		switch {
		case err == nil:
			return s.line[:len(s.line)-1], nil
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF) && len(s.line) > 0:
			return s.line, nil
		default:
			return nil, err
		}
	}
}

// Next returns the next record, without its newline, or io.EOF. The slice
// is valid until the next call.
func (s *Scanner) Next() ([]byte, error) {
	for {
		line, err := s.readLine()
		if err != nil {
			return nil, err
		}
		if !s.condensed {
			return line, nil
		}
		s.n++
		if len(line) == 0 {
			return nil, fmt.Errorf("recstream: frame %d: empty frame", s.n)
		}
		switch line[0] {
		case '#':
			continue
		case '=':
			txt := append([]byte(nil), line[1:]...)
			s.prev = nil
			if n, err := parse(txt); err == nil && n.kind == 'o' {
				s.prev = n
			}
			return txt, nil
		case '+', '~', ':', ';':
			p, err := parse(append([]byte(nil), line[1:]...))
			if err != nil {
				return nil, fmt.Errorf("recstream: frame %d: %w", s.n, err)
			}
			base, st := s.prev, style(line[0])
			if line[0] == ':' || line[0] == ';' {
				base, st = &node{kind: 'o'}, compact
				if line[0] == ';' {
					st = spaced
				}
			}
			cur, err := apply(base, p)
			if err != nil {
				return nil, fmt.Errorf("recstream: frame %d: %w", s.n, err)
			}
			s.prev = cur
			s.out = cur.write(s.out[:0], st)
			return s.out, nil
		default:
			return nil, fmt.Errorf("recstream: frame %d: unknown frame %q", s.n, line[0])
		}
	}
}

// NewReader returns the JSON Lines text of r's records, '\n' after each,
// whatever r's form: a drop-in for code that reads .jsonl through a
// bufio.Scanner or json.Decoder.
func NewReader(r io.Reader) (io.Reader, error) {
	s, err := NewScanner(r)
	if err != nil {
		return nil, err
	}
	if !s.condensed {
		return s.br, nil // plain or gunzipped text passes straight through
	}
	return &textReader{s: s}, nil
}

// OpenReader opens path as NewReader does; Close closes the file.
func OpenReader(path string) (io.ReadCloser, error) {
	s, err := Open(path)
	if err != nil {
		return nil, err
	}
	if !s.condensed {
		return passThrough{s}, nil
	}
	return &textReader{s: s}, nil
}

type textReader struct {
	s   *Scanner
	buf bytes.Buffer
	err error
}

func (t *textReader) Read(p []byte) (int, error) {
	for t.buf.Len() == 0 {
		if t.err != nil {
			return 0, t.err
		}
		rec, err := t.s.Next()
		if err != nil {
			t.err = err
			continue
		}
		t.buf.Write(rec)
		t.buf.WriteByte('\n')
	}
	return t.buf.Read(p)
}

func (t *textReader) Close() error { return t.s.Close() }

type passThrough struct{ s *Scanner }

func (p passThrough) Read(b []byte) (int, error) { return p.s.br.Read(b) }
func (p passThrough) Close() error               { return p.s.Close() }
