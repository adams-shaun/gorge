package cards

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"unsafe"
)

// The image file.
//
// LoadRegistry used to gunzip and gob-decode the whole IR cache and build the
// pointer-free Image from the decoded tree on every process start: ~1.7 s of
// CPU and ~550 MB allocated, measured on the hunt's one-game-per-process
// mtgsim runs, where it was more than half of a Commander duel's cost. The
// Image is pointer-free and every row is a fixed-size struct of uint32s, so
// it is written once beside the cache as its raw rows (ir-<fp>.img) and read
// back with one read into one buffer, the rows used in place.
//
// The file is derived from the cache it sits beside, so it is served only
// when it is no older than that cache and records the cache's size, and only
// when its header names this build's schema, cache version, compiler
// fingerprint and byte order; a checksum covers every byte after the fixed
// header. Anything else falls back to the gob route, which rewrites the file.
// The Image a read returns is reflect.DeepEqual to the one buildImage made
// (TestImageFileRoundTrip), so the registry, and every game, is unchanged.
//
// Layout (native byte order, recorded as imgFileBOM):
//
//	fixed   magic "gorgeimf" | fileVersion u32 | bom u32 | cacheSize u64 |
//	        crc32c u32 | nSections u32                    (imgFileFixedLen)
//	table   nSections × (offset u64, count u64)
//	rows    ImageHeader, then each section's rows at its offset, 8-aligned
const (
	imgFileMagic    = "gorgeimf"
	imgFileVersion  = 1
	imgFileBOM      = 0x01020304
	imgFileFixedLen = 32
)

var imgCRC = crc32.MakeTable(crc32.Castagnoli)

// ImagePath is the image file that sits beside the gob cache at cache.
func ImagePath(cache string) string {
	seg := SegmentPath(cache)
	return seg[:len(seg)-len(".seg")] + ".img"
}

// imgSection is one Image slice as raw bytes: its address, element count and
// element size, so writer and reader walk the same list in the same order.
type imgSection struct {
	ptr  unsafe.Pointer
	n    int
	size uintptr
}

func sectionOf[T any](s []T) imgSection {
	var z T
	if len(s) == 0 {
		return imgSection{n: 0, size: unsafe.Sizeof(z)}
	}
	return imgSection{ptr: unsafe.Pointer(&s[0]), n: len(s), size: unsafe.Sizeof(z)}
}

func (im *Image) sections() []imgSection {
	return []imgSection{
		{ptr: unsafe.Pointer(&im.Hdr), n: 1, size: unsafe.Sizeof(im.Hdr)},
		sectionOf(im.Blob), sectionOf(im.Strs), sectionOf(im.Cards), sectionOf(im.Tokens),
		sectionOf(im.Faces), sectionOf(im.FaceSAs), sectionOf(im.Nodes), sectionOf(im.Trigs),
		sectionOf(im.Stats), sectionOf(im.Repls), sectionOf(im.Params), sectionOf(im.SVars),
		sectionOf(im.Words), sectionOf(im.NameKeys), sectionOf(im.NameOrds), sectionOf(im.FirstOrd),
	}
}

func align8(n uint64) uint64 { return (n + 7) &^ 7 }

// writeImageFile writes im to path atomically (a unique temp file renamed
// into place), recording cacheSize, the size of the cache it was built from.
func writeImageFile(path string, im *Image, cacheSize int64) error {
	secs := im.sections()
	tableLen := uint64(len(secs)) * 16
	off := align8(imgFileFixedLen + tableLen)
	table := make([]byte, tableLen)
	offs := make([]uint64, len(secs))
	for i, s := range secs {
		offs[i] = off
		binary.NativeEndian.PutUint64(table[i*16:], off)
		binary.NativeEndian.PutUint64(table[i*16+8:], uint64(s.n))
		off = align8(off + uint64(s.n)*uint64(s.size))
	}
	buf := make([]byte, off)
	copy(buf[imgFileFixedLen:], table)
	for i, s := range secs {
		if s.n > 0 {
			copy(buf[offs[i]:], unsafe.Slice((*byte)(s.ptr), uintptr(s.n)*s.size))
		}
	}
	copy(buf[0:8], imgFileMagic)
	binary.NativeEndian.PutUint32(buf[8:], imgFileVersion)
	binary.NativeEndian.PutUint32(buf[12:], imgFileBOM)
	binary.NativeEndian.PutUint64(buf[16:], uint64(cacheSize))
	binary.NativeEndian.PutUint32(buf[28:], uint32(len(secs)))
	binary.NativeEndian.PutUint32(buf[24:], crc32.Checksum(buf[imgFileFixedLen:], imgCRC))

	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	if err := f.Chmod(0o644); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if _, err := f.Write(buf); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

var errImageFile = errors.New("cards: image file unusable")

// readImageFile reads the image file at path, built from the cache at cache.
// Any mismatch (absent, older than the cache, another cache size, schema,
// version, fingerprint or byte order, a bad checksum or a short section) is
// an error and the caller takes the gob route.
func readImageFile(path, cache string) (*Image, error) {
	ci, err := os.Stat(cache)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if ci.ModTime().After(fi.ModTime()) || fi.Size() < imgFileFixedLen {
		return nil, errImageFile
	}
	// One 8-aligned buffer, so every row type (at most 4-byte aligned) can
	// be viewed in place.
	words := make([]uint64, (fi.Size()+7)/8)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(words)*8)[:fi.Size()]
	if _, err := io.ReadFull(f, buf); err != nil {
		return nil, err
	}
	if string(buf[0:8]) != imgFileMagic ||
		binary.NativeEndian.Uint32(buf[8:]) != imgFileVersion ||
		binary.NativeEndian.Uint32(buf[12:]) != imgFileBOM ||
		binary.NativeEndian.Uint64(buf[16:]) != uint64(ci.Size()) ||
		binary.NativeEndian.Uint32(buf[24:]) != crc32.Checksum(buf[imgFileFixedLen:], imgCRC) {
		return nil, errImageFile
	}
	im := &Image{}
	secs := im.sections()
	if binary.NativeEndian.Uint32(buf[28:]) != uint32(len(secs)) || uint64(len(buf)) < imgFileFixedLen+uint64(len(secs))*16 {
		return nil, errImageFile
	}
	view := func(i int, size uintptr) (unsafe.Pointer, int, error) {
		off := binary.NativeEndian.Uint64(buf[imgFileFixedLen+i*16:])
		n := binary.NativeEndian.Uint64(buf[imgFileFixedLen+i*16+8:])
		if off%8 != 0 || off > uint64(len(buf)) || n > (uint64(len(buf))-off)/uint64(size) {
			return nil, 0, errImageFile
		}
		if n == 0 {
			return nil, 0, nil
		}
		return unsafe.Pointer(&buf[off]), int(n), nil
	}
	// Hdr is copied out; every other section is a full-capacity-capped view
	// into buf, so an append (none happens: the Image is immutable) could
	// never write into the next section.
	p, n, err := view(0, unsafe.Sizeof(im.Hdr))
	if err != nil || n != 1 {
		return nil, errImageFile
	}
	im.Hdr = *(*ImageHeader)(p)
	fingerprint := hexFingerprint()
	if string(im.Hdr.Magic[:]) != "gorgeimg" || im.Hdr.Schema != imageSchema ||
		im.Hdr.CacheVersion != cacheVersion || im.Hdr.Fingerprint != fingerprint {
		return nil, errImageFile
	}
	fail := false
	set := func(i int, size uintptr, assign func(unsafe.Pointer, int)) {
		p, n, err := view(i, size)
		if err != nil {
			fail = true
			return
		}
		if n > 0 {
			assign(p, n)
		}
	}
	set(1, unsafe.Sizeof(byte(0)), func(p unsafe.Pointer, n int) { im.Blob = unsafe.Slice((*byte)(p), n) })
	set(2, unsafe.Sizeof(StringRef{}), func(p unsafe.Pointer, n int) { im.Strs = unsafe.Slice((*StringRef)(p), n) })
	set(3, unsafe.Sizeof(CardRec{}), func(p unsafe.Pointer, n int) { im.Cards = unsafe.Slice((*CardRec)(p), n) })
	set(4, unsafe.Sizeof(TokenRec{}), func(p unsafe.Pointer, n int) { im.Tokens = unsafe.Slice((*TokenRec)(p), n) })
	set(5, unsafe.Sizeof(FaceRec{}), func(p unsafe.Pointer, n int) { im.Faces = unsafe.Slice((*FaceRec)(p), n) })
	set(6, unsafe.Sizeof(NodeID(0)), func(p unsafe.Pointer, n int) { im.FaceSAs = unsafe.Slice((*NodeID)(p), n) })
	set(7, unsafe.Sizeof(SARec{}), func(p unsafe.Pointer, n int) { im.Nodes = unsafe.Slice((*SARec)(p), n) })
	set(8, unsafe.Sizeof(TrigRec{}), func(p unsafe.Pointer, n int) { im.Trigs = unsafe.Slice((*TrigRec)(p), n) })
	set(9, unsafe.Sizeof(StaticRec{}), func(p unsafe.Pointer, n int) { im.Stats = unsafe.Slice((*StaticRec)(p), n) })
	set(10, unsafe.Sizeof(ReplRec{}), func(p unsafe.Pointer, n int) { im.Repls = unsafe.Slice((*ReplRec)(p), n) })
	set(11, unsafe.Sizeof(ParamRow{}), func(p unsafe.Pointer, n int) { im.Params = unsafe.Slice((*ParamRow)(p), n) })
	set(12, unsafe.Sizeof(SVarRow{}), func(p unsafe.Pointer, n int) { im.SVars = unsafe.Slice((*SVarRow)(p), n) })
	set(13, unsafe.Sizeof(StringID(0)), func(p unsafe.Pointer, n int) { im.Words = unsafe.Slice((*StringID)(p), n) })
	set(14, unsafe.Sizeof(StringID(0)), func(p unsafe.Pointer, n int) { im.NameKeys = unsafe.Slice((*StringID)(p), n) })
	set(15, unsafe.Sizeof(uint32(0)), func(p unsafe.Pointer, n int) { im.NameOrds = unsafe.Slice((*uint32)(p), n) })
	set(16, unsafe.Sizeof(NameOrd{}), func(p unsafe.Pointer, n int) { im.FirstOrd = unsafe.Slice((*NameOrd)(p), n) })
	if fail {
		return nil, errImageFile
	}
	// buildImageHashed makes Cards and Tokens (empty, not nil, when there
	// are none); every other section is appended to and stays nil.
	if im.Cards == nil {
		im.Cards = []CardRec{}
	}
	if im.Tokens == nil {
		im.Tokens = []TokenRec{}
	}
	return im, nil
}

// hexFingerprint is CompilerFingerprint as buildImageHashed records it.
func hexFingerprint() [16]byte {
	var fp [16]byte
	b, _ := hex.DecodeString(CompilerFingerprint)
	copy(fp[:], b)
	return fp
}

// registryFromImage is LoadRegistry's result rebuilt from a read image: the
// tokens finished eagerly from the image's raw rows (which
// TestImageCorpusOracle holds equal to the cache's), the cards left lazy.
func registryFromImage(im *Image) (*Registry, error) {
	// The tokens go through the cache's own wire shape, so the map is built
	// exactly as a gob load builds it (nil for none: cacheFileWire.cacheFile).
	var w cacheFileWire
	for i, t := range im.Tokens {
		w.Tokens = append(w.Tokens, tokenEntry{Key: im.str(t.Key), Card: im.rawToken(i)})
	}
	r, err := finishDecoded(nil, w.cacheFile().Tokens)
	if err != nil {
		return nil, err
	}
	r.lazy = newLazyCards(im)
	return r, nil
}
