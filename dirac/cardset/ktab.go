package cardset

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// The binary card table: one PoolData on disk. Stdlib only and uncompressed, so
// the bytes stay stable across Go releases, and not encoding/gob, whose type ids
// depend on what the process encoded first.
//
//	"KTAB"  uvarint TableFormat  32-byte schema fingerprint
//	uvarint len + key  32-byte sha256(payload)  payload
//
// The payload is a reflect-driven walk of PoolData, so a new kop field needs no
// codec edit:
//
//	struct  for each field not IsZero, uvarint(fieldIndex+1) then the value; uvarint(0)
//	slice   uvarint(0) when nil, else uvarint(len+1) then the elements
//	array   uvarint(n), n the index past the last non-zero element, then n elements
//	string  uvarint(len), bytes
//	bool    one byte
//	int     zig-zag varint
//	uint    uvarint
//
// Any other kind panics at encode and at schema-fingerprint time, as does an
// unexported field, which the walk could not restore. No path, time or host data
// can reach a table: PoolData has no such field.

const tableMagic = "KTAB"

// TableFormat is the table layout's version.
const TableFormat = 3

// schemaFingerprint is the sha256 of a recursive walk of t writing each type's
// string, kind, field names and array length. It is redundant with the key on
// purpose: it catches a source file list that misses a type's file.
func schemaFingerprint(t reflect.Type) [32]byte {
	h := sha256.New()
	seen := map[reflect.Type]bool{}
	var walk func(t reflect.Type)
	walk = func(t reflect.Type) {
		if seen[t] {
			fmt.Fprintf(h, "@%s;", t)
			return
		}
		seen[t] = true
		fmt.Fprintf(h, "%s/%s[", t, t.Kind())
		switch t.Kind() {
		case reflect.Struct:
			for i := range t.NumField() {
				f := t.Field(i)
				if !f.IsExported() {
					panic(fmt.Sprintf("cardset: %s.%s is unexported, so a table cannot carry it", t, f.Name))
				}
				fmt.Fprintf(h, "%s ", f.Name)
				walk(f.Type)
			}
		case reflect.Slice:
			walk(t.Elem())
		case reflect.Array:
			fmt.Fprintf(h, "%d ", t.Len())
			walk(t.Elem())
		case reflect.Bool, reflect.String,
			reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
			reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		default:
			panic(fmt.Sprintf("cardset: kind %s (%s) cannot be in a table", t.Kind(), t))
		}
		fmt.Fprint(h, "]")
	}
	walk(t)
	var out [32]byte
	h.Sum(out[:0])
	return out
}

var poolSchema = sync.OnceValue(func() [32]byte { return schemaFingerprint(reflect.TypeFor[PoolData]()) })

func encodeValue(b []byte, v reflect.Value) []byte {
	switch v.Kind() {
	case reflect.Struct:
		for i := range v.NumField() {
			f := v.Field(i)
			if f.IsZero() {
				continue
			}
			b = binary.AppendUvarint(b, uint64(i+1))
			b = encodeValue(b, f)
		}
		return binary.AppendUvarint(b, 0)
	case reflect.Slice:
		if v.IsNil() {
			return binary.AppendUvarint(b, 0)
		}
		b = binary.AppendUvarint(b, uint64(v.Len())+1)
		for i := range v.Len() {
			b = encodeValue(b, v.Index(i))
		}
		return b
	case reflect.Array:
		n := v.Len()
		for n > 0 && v.Index(n-1).IsZero() {
			n--
		}
		b = binary.AppendUvarint(b, uint64(n))
		for i := range n {
			b = encodeValue(b, v.Index(i))
		}
		return b
	case reflect.String:
		b = binary.AppendUvarint(b, uint64(v.Len()))
		return append(b, v.String()...)
	case reflect.Bool:
		if v.Bool() {
			return append(b, 1)
		}
		return append(b, 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return binary.AppendVarint(b, v.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return binary.AppendUvarint(b, v.Uint())
	}
	panic(fmt.Sprintf("cardset: kind %s (%s) cannot be in a table", v.Kind(), v.Type()))
}

// tableReader decodes a payload. A malformed one panics with a tableError, which
// parseTableHeader and decodePayload turn into an ordinary error.
type tableReader struct{ b []byte }

type tableError string

func (r *tableReader) uvarint() uint64 {
	x, n := binary.Uvarint(r.b)
	if n <= 0 {
		panic(tableError("truncated uvarint"))
	}
	r.b = r.b[n:]
	return x
}

func (r *tableReader) varint() int64 {
	x, n := binary.Varint(r.b)
	if n <= 0 {
		panic(tableError("truncated varint"))
	}
	r.b = r.b[n:]
	return x
}

func (r *tableReader) take(n uint64) []byte {
	if n > uint64(len(r.b)) {
		panic(tableError(fmt.Sprintf("%d bytes wanted, %d left", n, len(r.b))))
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *tableReader) value(v reflect.Value) {
	switch v.Kind() {
	case reflect.Struct:
		for {
			i := r.uvarint()
			if i == 0 {
				return
			}
			if i > uint64(v.NumField()) {
				panic(tableError(fmt.Sprintf("%s has no field %d", v.Type(), i-1)))
			}
			r.value(v.Field(int(i - 1)))
		}
	case reflect.Slice:
		n := r.uvarint()
		if n == 0 {
			return
		}
		n--
		// Every element takes at least one byte, so this bounds the allocation.
		if n > uint64(len(r.b)) {
			panic(tableError(fmt.Sprintf("%s of %d elements in %d bytes", v.Type(), n, len(r.b))))
		}
		s := reflect.MakeSlice(v.Type(), int(n), int(n))
		for i := range int(n) {
			r.value(s.Index(i))
		}
		v.Set(s)
	case reflect.Array:
		n := r.uvarint()
		if n > uint64(v.Len()) {
			panic(tableError(fmt.Sprintf("%s with %d elements", v.Type(), n)))
		}
		for i := range int(n) {
			r.value(v.Index(i))
		}
	case reflect.String:
		v.SetString(string(r.take(r.uvarint())))
	case reflect.Bool:
		v.SetBool(r.take(1)[0] != 0)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		v.SetInt(r.varint())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		v.SetUint(r.uvarint())
	default:
		panic(fmt.Sprintf("cardset: kind %s (%s) cannot be in a table", v.Kind(), v.Type()))
	}
}

// EncodeTable renders pd as a table belonging to key. The same pd and key always
// give the same bytes.
func EncodeTable(pd *PoolData, key string) []byte {
	payload := encodeValue(nil, reflect.ValueOf(*pd))
	sum := sha256.Sum256(payload)
	fp := poolSchema()
	b := []byte(tableMagic)
	b = binary.AppendUvarint(b, TableFormat)
	b = append(b, fp[:]...)
	b = binary.AppendUvarint(b, uint64(len(key)))
	b = append(b, key...)
	b = append(b, sum[:]...)
	return append(b, payload...)
}

// parseTableHeader checks everything but the payload and returns the rest.
func parseTableHeader(b []byte) (key string, sum [32]byte, payload []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			te, ok := r.(tableError)
			if !ok {
				panic(r)
			}
			err = fmt.Errorf("truncated header: %s", string(te))
		}
	}()
	if !bytes.HasPrefix(b, []byte(tableMagic)) {
		return "", sum, nil, errors.New("bad magic")
	}
	r := tableReader{b[len(tableMagic):]}
	if v := r.uvarint(); v != TableFormat {
		return "", sum, nil, fmt.Errorf("table format %d, this binary reads %d", v, TableFormat)
	}
	fp := poolSchema()
	if !bytes.Equal(r.take(32), fp[:]) {
		return "", sum, nil, errors.New("schema fingerprint differs from this binary's PoolData")
	}
	key = string(r.take(r.uvarint()))
	copy(sum[:], r.take(32))
	return key, sum, r.b, nil
}

// DecodeTable parses a table and returns its data and the key it belongs to. Any
// mismatch of magic, format, schema or checksum is an error.
func DecodeTable(b []byte) (pd *PoolData, key string, err error) {
	key, sum, payload, err := parseTableHeader(b)
	if err != nil {
		return nil, "", err
	}
	if sha256.Sum256(payload) != sum {
		return nil, "", errors.New("payload checksum mismatch")
	}
	if pd, err = decodePayload(payload); err != nil {
		return nil, "", err
	}
	if len(pd.Status) != len(pd.Names) || len(pd.Reason) != len(pd.Names) {
		return nil, "", fmt.Errorf("%d statuses and %d reasons for %d entries", len(pd.Status), len(pd.Reason), len(pd.Names))
	}
	return pd, key, nil
}

// decodePayload decodes a payload whose checksum has already passed. It bounds every
// length, so a malformed one is an error whatever it holds.
func decodePayload(payload []byte) (pd *PoolData, err error) {
	defer func() {
		if r := recover(); r != nil {
			pd, err = nil, fmt.Errorf("malformed payload: %v", r)
		}
	}()
	r := tableReader{payload}
	pd = &PoolData{}
	r.value(reflect.ValueOf(pd).Elem())
	if len(r.b) != 0 {
		return nil, fmt.Errorf("malformed payload: %d bytes after the end", len(r.b))
	}
	return pd, nil
}

// TableSum returns a table's hex sha256(payload), the manifest's checksum column.
func TableSum(b []byte) (string, error) {
	_, sum, _, err := parseTableHeader(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(sum[:]), nil
}
