package zipfs

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"testing"
)

func TestPreflightEntryCount(t *testing.T) {
	data := metadataZip(t, 3)
	limits := DefaultLimits()
	limits.MaxEntries = 2
	if err := preflightEntryCount(bytes.NewReader(data), int64(len(data)), limits, t.Context()); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ordinary ZIP preflight error = %v, want ErrLimitExceeded", err)
	}
	withTrailer := append(bytes.Clone(data), []byte("trailing data")...)
	if err := preflightEntryCount(bytes.NewReader(withTrailer), int64(len(withTrailer)), limits, t.Context()); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("trailing-data ZIP preflight error = %v, want ErrLimitExceeded", err)
	}
	if _, err := OpenWithLimits(t.Context(), writeTempZip(t, withTrailer), limits); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("trailing-data OpenWithLimits error = %v, want ErrLimitExceeded", err)
	}

	zip64 := convertToZIP64(t, data)
	if err := preflightEntryCount(bytes.NewReader(zip64), int64(len(zip64)), limits, t.Context()); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ZIP64 preflight error = %v, want ErrLimitExceeded", err)
	}
	zip64Path := writeTempZip(t, zip64)
	if _, err := OpenWithLimits(t.Context(), zip64Path, limits); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("ZIP64 OpenWithLimits error = %v, want ErrLimitExceeded", err)
	}
}

func TestPreflightScansActualCentralDirectoryCount(t *testing.T) {
	data := metadataZip(t, 4)
	eocd := bytes.LastIndex(data, []byte("PK\x05\x06"))
	if eocd < 0 {
		t.Fatal("EOCD not found")
	}
	binary.LittleEndian.PutUint16(data[eocd+8:eocd+10], 1)
	binary.LittleEndian.PutUint16(data[eocd+10:eocd+12], 1)
	limits := DefaultLimits()
	limits.MaxEntries = 2
	if err := preflightEntryCount(bytes.NewReader(data), int64(len(data)), limits, t.Context()); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("underreported central-directory count error = %v, want ErrLimitExceeded", err)
	}
	if _, err := OpenWithLimits(t.Context(), writeTempZip(t, data), limits); !errors.Is(err, ErrLimitExceeded) {
		t.Fatalf("underreported OpenWithLimits error = %v, want ErrLimitExceeded", err)
	}
}

func TestPreflightEntryCountCanceledAndMalformedInputs(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	data := metadataZip(t, 1)
	if err := preflightEntryCount(bytes.NewReader(data), int64(len(data)), DefaultLimits(), ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("pre-canceled preflight error = %v, want context.Canceled", err)
	}
	for _, raw := range [][]byte{nil, []byte("not a zip"), data[:len(data)-10]} {
		if err := preflightEntryCount(bytes.NewReader(raw), int64(len(raw)), DefaultLimits(), t.Context()); err != nil {
			t.Fatalf("malformed input preflight error = %v, want nil fallback", err)
		}
	}
}

func metadataZip(t *testing.T, entries int) []byte {
	t.Helper()
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for i := range entries {
		name := []byte{'a' + byte(i), '.', 't', 'x', 't'}
		f, err := w.Create(string(name))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte("x")); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return bytes.Clone(b.Bytes())
}

func convertToZIP64(t *testing.T, ordinary []byte) []byte {
	t.Helper()
	data := bytes.Clone(ordinary)
	eocd := bytes.LastIndex(data, []byte("PK\x05\x06"))
	if eocd < 0 {
		t.Fatal("EOCD not found")
	}
	count := binary.LittleEndian.Uint16(data[eocd+10 : eocd+12])
	cdSize := binary.LittleEndian.Uint32(data[eocd+12 : eocd+16])
	cdOffset := binary.LittleEndian.Uint32(data[eocd+16 : eocd+20])
	zip64Pos := uint64(eocd)
	record := make([]byte, zip64EOCDMinimum+zip64LocatorSize)
	binary.LittleEndian.PutUint32(record[0:4], zip64EOCDSignature)
	binary.LittleEndian.PutUint64(record[4:12], zip64EOCDMinimum-12)
	binary.LittleEndian.PutUint16(record[12:14], 45)
	binary.LittleEndian.PutUint16(record[14:16], 45)
	binary.LittleEndian.PutUint64(record[24:32], uint64(count))
	binary.LittleEndian.PutUint64(record[32:40], uint64(count))
	binary.LittleEndian.PutUint64(record[40:48], uint64(cdSize))
	binary.LittleEndian.PutUint64(record[48:56], uint64(cdOffset))
	loc := record[zip64EOCDMinimum:]
	binary.LittleEndian.PutUint32(loc[0:4], zip64LocatorSig)
	binary.LittleEndian.PutUint32(loc[4:8], 0)
	binary.LittleEndian.PutUint64(loc[8:16], zip64Pos)
	binary.LittleEndian.PutUint32(loc[16:20], 1)
	out := append(data[:eocd], record...)
	out = append(out, data[eocd:]...)
	newEOCD := eocd + len(record)
	binary.LittleEndian.PutUint16(out[newEOCD+8:newEOCD+10], 0xffff)
	binary.LittleEndian.PutUint16(out[newEOCD+10:newEOCD+12], 0xffff)
	binary.LittleEndian.PutUint32(out[newEOCD+12:newEOCD+16], 0xffffffff)
	binary.LittleEndian.PutUint32(out[newEOCD+16:newEOCD+20], 0xffffffff)
	return out
}
