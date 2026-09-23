package zipfs

import (
	"bufio"
	"context"
	"encoding/binary"
	"fmt"
	"io"
)

const (
	eocdSignature      = 0x06054b50
	zip64EOCDSignature = 0x06064b50
	zip64LocatorSig    = 0x07064b50
	centralHeaderSig   = 0x02014b50
	eocdFixedSize      = 22
	centralFixedSize   = 46
	zip64LocatorSize   = 20
	zip64EOCDMinimum   = 56
)

// preflightEntryCount mirrors Go 1.27 archive/zip directory parsing so the
// entry limit is enforced before zip.NewReader materializes the directory.
// Re-check against archive/zip whenever the Go toolchain is upgraded.
func preflightEntryCount(r io.ReaderAt, size int64, limits Limits, ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	eocdPos, eocd, ok := findEOCD(r, size)
	if !ok {
		return nil // archive/zip fails too
	}
	records := uint64(binary.LittleEndian.Uint16(eocd[10:12])) // stdlib ignores bytes 8:10
	cdSize := uint64(binary.LittleEndian.Uint32(eocd[12:16]))
	cdOffset := uint64(binary.LittleEndian.Uint32(eocd[16:20]))
	dirEnd := eocdPos
	if records == 0xffff || cdSize == 0xffffffff || cdOffset == 0xffffffff {
		if p, found := findZIP64End(r, eocdPos); found {
			var rec [zip64EOCDMinimum]byte
			if p < 0 {
				return nil
			}
			if _, err := r.ReadAt(rec[:], p); err != nil {
				return nil
			}
			if binary.LittleEndian.Uint32(rec[0:4]) != zip64EOCDSignature {
				return nil
			}
			records = binary.LittleEndian.Uint64(rec[32:40])
			cdSize = binary.LittleEndian.Uint64(rec[40:48])
			cdOffset = binary.LittleEndian.Uint64(rec[48:56])
			dirEnd = p
		} // not found: stdlib keeps the 32-bit values, so do we
	}
	if records > uint64(limits.MaxEntries) {
		return fmt.Errorf("%d entries exceeds %d: %w", records, limits.MaxEntries, ErrLimitExceeded)
	}
	const maxInt64 = uint64(1<<63 - 1)
	if cdSize > maxInt64 || cdOffset > maxInt64 {
		return nil
	}
	base := dirEnd - int64(cdSize) - int64(cdOffset)
	if o := base + int64(cdOffset); o < 0 || o >= size {
		return nil
	}
	starts := []int64{base + int64(cdOffset)}
	if base > 0 {
		starts = append(starts, int64(cdOffset)) // stdlib may reset baseOffset to 0: check both
	}
	for _, start := range starts {
		if err := countCentralRecords(ctx, r, start, size, limits.MaxEntries); err != nil {
			return err
		}
	}
	return nil
}

func findEOCD(r io.ReaderAt, size int64) (int64, []byte, bool) { // mirrors readDirectoryEnd windows
	for i, blockLen := range []int64{1024, 65 * 1024} {
		if blockLen > size {
			blockLen = size
		}
		buf := make([]byte, int(blockLen))
		if _, err := r.ReadAt(buf, size-blockLen); err != nil && err != io.EOF {
			return 0, nil, false
		}
		if pos := findEOCDInBlock(buf); pos >= 0 {
			return size - blockLen + int64(pos), buf[pos : pos+eocdFixedSize], true
		}
		if i == 1 || blockLen == size {
			return 0, nil, false
		}
	}
	return 0, nil, false
}

func findEOCDInBlock(b []byte) int { // mirrors findSignatureInBlock: last signature wins; truncated comment => -1
	for i := len(b) - eocdFixedSize; i >= 0; i-- {
		if binary.LittleEndian.Uint32(b[i:i+4]) == eocdSignature {
			if int(binary.LittleEndian.Uint16(b[i+20:i+22]))+eocdFixedSize+i > len(b) {
				return -1
			}
			return i
		}
	}
	return -1
}

func findZIP64End(r io.ReaderAt, eocdPos int64) (int64, bool) { // mirrors findDirectory64End
	if eocdPos-zip64LocatorSize < 0 {
		return 0, false
	}
	var loc [zip64LocatorSize]byte
	if _, err := r.ReadAt(loc[:], eocdPos-zip64LocatorSize); err != nil {
		return 0, false
	}
	if binary.LittleEndian.Uint32(loc[0:4]) != zip64LocatorSig || binary.LittleEndian.Uint32(loc[4:8]) != 0 ||
		binary.LittleEndian.Uint32(loc[16:20]) != 1 {
		return 0, false
	}
	return int64(binary.LittleEndian.Uint64(loc[8:16])), true
}

// countCentralRecords counts like Reader.init: sequentially until a bad
// signature or a truncated record, ignoring the declared directory size.
func countCentralRecords(ctx context.Context, r io.ReaderAt, start, size int64, maxEntries int) error {
	br := bufio.NewReaderSize(io.NewSectionReader(r, start, size-start), 64<<10)
	var hdr [centralFixedSize]byte
	for count := 0; ; {
		if count&1023 == 0 {
			if err := contextErr(ctx); err != nil {
				return err
			}
		}
		if _, err := io.ReadFull(br, hdr[:]); err != nil {
			return nil
		}
		if binary.LittleEndian.Uint32(hdr[:4]) != centralHeaderSig {
			return nil
		}
		skip := int(binary.LittleEndian.Uint16(hdr[28:30])) + int(binary.LittleEndian.Uint16(hdr[30:32])) +
			int(binary.LittleEndian.Uint16(hdr[32:34]))
		if n, err := br.Discard(skip); err != nil || n != skip {
			return nil
		}
		count++
		if count > maxEntries {
			return fmt.Errorf("more than %d central-directory entries: %w", maxEntries, ErrLimitExceeded)
		}
	}
}
