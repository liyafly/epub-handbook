package zipfs

import (
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
	maxEOCDSearch      = eocdFixedSize + 0xffff
	zip64LocatorSize   = 20
	zip64EOCDMinimum   = 56
)

// preflightEntryCount checks ZIP metadata before archive/zip allocates a
// *zip.File for every central-directory record. Malformed or unfamiliar ZIP
// layouts are left to archive/zip; a reliably decoded oversized count is not.
func preflightEntryCount(r io.ReaderAt, size int64, limits Limits, ctx context.Context) error {
	if err := contextErr(ctx); err != nil {
		return err
	}
	if size < eocdFixedSize {
		return nil
	}
	tailSize := min(int64(maxEOCDSearch), size)
	tail := make([]byte, int(tailSize))
	if _, err := r.ReadAt(tail, size-tailSize); err != nil && err != io.EOF {
		return nil
	}
	for i := len(tail) - eocdFixedSize; i >= 0; i-- {
		if binary.LittleEndian.Uint32(tail[i:i+4]) != eocdSignature {
			continue
		}
		commentLen := int(binary.LittleEndian.Uint16(tail[i+20 : i+22]))
		// archive/zip accepts trailing bytes after the comment, so keep the
		// preflight aligned with its EOCD search instead of requiring EOF here.
		if i+eocdFixedSize+commentLen > len(tail) {
			continue
		}
		eocdPos := size - tailSize + int64(i)
		entriesDisk := uint64(binary.LittleEndian.Uint16(tail[i+8 : i+10]))
		entriesTotal := uint64(binary.LittleEndian.Uint16(tail[i+10 : i+12]))
		cdSize := uint64(binary.LittleEndian.Uint32(tail[i+12 : i+16]))
		cdOffset := uint64(binary.LittleEndian.Uint32(tail[i+16 : i+20]))
		if entriesDisk != entriesTotal {
			return nil
		}
		if entriesTotal == 0xffff || cdSize == 0xffffffff || cdOffset == 0xffffffff {
			count, zsize, zoffset, ok := readZIP64Directory(r, eocdPos)
			if !ok {
				return nil
			}
			entriesTotal, cdSize, cdOffset = count, zsize, zoffset
		}
		if entriesTotal > uint64(limits.MaxEntries) {
			return fmt.Errorf("%d entries exceeds %d: %w", entriesTotal, limits.MaxEntries, ErrLimitExceeded)
		}
		return scanCentralDirectory(r, size, eocdPos, cdOffset, cdSize, limits, ctx)
	}
	return nil
}

func readZIP64Directory(r io.ReaderAt, eocdPos int64) (count, cdSize, cdOffset uint64, ok bool) {
	if eocdPos < zip64LocatorSize {
		return 0, 0, 0, false
	}
	var loc [zip64LocatorSize]byte
	if _, err := r.ReadAt(loc[:], eocdPos-zip64LocatorSize); err != nil {
		return 0, 0, 0, false
	}
	if binary.LittleEndian.Uint32(loc[:4]) != zip64LocatorSig || binary.LittleEndian.Uint32(loc[4:8]) != 0 || binary.LittleEndian.Uint32(loc[16:20]) != 1 {
		return 0, 0, 0, false
	}
	pos := binary.LittleEndian.Uint64(loc[8:16])
	var hdr [zip64EOCDMinimum]byte
	if pos > uint64(^uint64(0)>>1) {
		return 0, 0, 0, false
	}
	if _, err := r.ReadAt(hdr[:], int64(pos)); err != nil {
		return 0, 0, 0, false
	}
	if binary.LittleEndian.Uint32(hdr[:4]) != zip64EOCDSignature || binary.LittleEndian.Uint64(hdr[4:12]) < zip64EOCDMinimum-12 ||
		binary.LittleEndian.Uint32(hdr[16:20]) != 0 || binary.LittleEndian.Uint32(hdr[20:24]) != 0 ||
		binary.LittleEndian.Uint64(hdr[24:32]) != binary.LittleEndian.Uint64(hdr[32:40]) {
		return 0, 0, 0, false
	}
	return binary.LittleEndian.Uint64(hdr[32:40]), binary.LittleEndian.Uint64(hdr[40:48]), binary.LittleEndian.Uint64(hdr[48:56]), true
}

func scanCentralDirectory(r io.ReaderAt, size, eocdPos int64, offset, length uint64, limits Limits, ctx context.Context) error {
	if length == 0 || length > uint64(size) || offset > uint64(size) || eocdPos < 0 {
		return nil
	}
	// ZIP offsets in self-extracting archives are sometimes relative to the
	// ZIP payload and sometimes already absolute. Try the standard offset first,
	// then derive the payload base from the end record and directory size.
	starts := []uint64{offset}
	if offset <= uint64(eocdPos) && length <= uint64(eocdPos)-offset {
		fallback := uint64(eocdPos) - length
		if fallback >= offset && fallback != offset {
			starts = append(starts, fallback)
		}
	}
	for _, start := range starts {
		if start > uint64(size) || length > uint64(size)-start || start+length > uint64(eocdPos)+uint64(maxEOCDSearch) {
			continue
		}
		count, valid, err := scanCentralAt(r, int64(start), int64(length), limits, ctx)
		if err != nil {
			return err
		}
		if valid {
			if count > uint64(limits.MaxEntries) {
				return fmt.Errorf("%d central-directory entries exceeds %d: %w", count, limits.MaxEntries, ErrLimitExceeded)
			}
			return nil
		}
	}
	return nil
}

func scanCentralAt(r io.ReaderAt, start, length int64, limits Limits, ctx context.Context) (uint64, bool, error) {
	var hdr [centralFixedSize]byte
	var consumed int64
	var count uint64
	for consumed < length {
		if count&255 == 0 {
			if err := contextErr(ctx); err != nil {
				return count, false, err
			}
		}
		if length-consumed < centralFixedSize {
			return count, false, nil
		}
		if _, err := r.ReadAt(hdr[:], start+consumed); err != nil {
			return count, false, nil
		}
		if binary.LittleEndian.Uint32(hdr[:4]) != centralHeaderSig {
			return count, false, nil
		}
		nameLen := int64(binary.LittleEndian.Uint16(hdr[28:30]))
		extraLen := int64(binary.LittleEndian.Uint16(hdr[30:32]))
		commentLen := int64(binary.LittleEndian.Uint16(hdr[32:34]))
		recordLen := int64(centralFixedSize) + nameLen + extraLen + commentLen
		if recordLen > length-consumed {
			return count, false, nil
		}
		consumed += recordLen
		count++
		if count > uint64(limits.MaxEntries) {
			return count, true, nil
		}
	}
	return count, consumed == length, nil
}
