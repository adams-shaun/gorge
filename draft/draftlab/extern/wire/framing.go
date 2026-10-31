package wire

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Reader reads gorge-agent/1 messages, one JSON object per line.
type Reader struct {
	br      *bufio.Reader
	maxLine int
}

// NewReader wraps r; maxLine <= 0 means MaxLine.
func NewReader(r io.Reader, maxLine int) *Reader {
	if maxLine <= 0 {
		maxLine = MaxLine
	}
	return &Reader{br: bufio.NewReaderSize(r, 64<<10), maxLine: maxLine}
}

// Read decodes the next message. It does NOT validate; callers validate
// with their direction. It returns io.EOF only at a clean end: no bytes
// before the stream ended. A malformed line or a line over the cap is a
// protocol error; after ErrLineTooLong the over-long line has been consumed
// and the next line still reads.
func (r *Reader) Read() (Message, error) {
	line, tooLong, err := r.readLine()
	if err != nil {
		return Message{}, err
	}
	if tooLong {
		return Message{}, ErrLineTooLong
	}
	var m Message
	if err := json.Unmarshal(line, &m); err != nil {
		return Message{}, fmt.Errorf("%w: malformed JSON line: %v", ErrProtocol, err)
	}
	return m, nil
}

// readLine returns the next non-blank line (without its newline), whether
// it overflowed the cap, and the read result. Blank lines are skipped. A
// line longer than the bufio buffer accumulates across ReadSlice chunks
// (ErrBufferFull is not an error here, only the cap is).
func (r *Reader) readLine() (line []byte, tooLong bool, err error) {
	for {
		line = line[:0]
		tooLong = false
		var chunk []byte
		for {
			chunk, err = r.br.ReadSlice('\n')
			if errors.Is(err, bufio.ErrBufferFull) {
				// The buffer filled before the newline: keep the
				// chunk (unless the cap already tripped, in which
				// case discard it) and read on.
				if !tooLong {
					if len(line)+len(chunk) > r.maxLine+1 {
						tooLong = true
						line = line[:0]
					} else {
						line = append(line, chunk...)
					}
				}
				err = nil
				continue
			}
			if !tooLong {
				if len(line)+len(chunk) > r.maxLine+1 {
					// One byte of headroom keeps an exactly-maxLine
					// line readable and anything larger refused.
					tooLong = true
					line = line[:0]
				} else {
					line = append(line, chunk...)
				}
			}
			if err != nil {
				break
			}
			if len(chunk) > 0 && chunk[len(chunk)-1] == '\n' {
				err = nil
				break
			}
		}
		if err == io.EOF {
			if len(line) == 0 && !tooLong {
				return nil, false, io.EOF
			}
			err = nil // a final unterminated line still reads
		}
		if err != nil {
			return nil, false, err
		}
		if tooLong {
			return nil, true, nil
		}
		line = bytes.TrimRight(line, "\n")
		if len(bytes.TrimSpace(line)) == 0 {
			continue // blank line
		}
		return line, false, nil
	}
}

// Writer writes one JSON object per line. Write marshals m and appends
// '\n' in a single Write call. It is NOT goroutine-safe: one writer per
// session, driven by the one goroutine that owns it.
type Writer struct {
	w io.Writer
}

// NewWriter wraps w.
func NewWriter(w io.Writer) *Writer { return &Writer{w: w} }

// Write marshals m as compact JSON, appends '\n' and writes the whole line
// in one Write call, so a line-framed reader never sees a torn message.
func (w *Writer) Write(m Message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	line := make([]byte, 0, len(b)+1)
	line = append(line, b...)
	line = append(line, '\n')
	_, err = w.w.Write(line)
	return err
}
