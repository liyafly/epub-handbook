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

func TestPreflightMatchesStdlibDirectoryParsing(t *testing.T) {
	limits := DefaultLimits()
	limits.MaxEntries = 10
	const entries = 65_541
	cdSize := uint32(entries * 47)
	cases := []struct {
		name string
		data []byte
	}{
		{
			name: "different disk counts",
			data: centralOnlyZip(entries, 0, func(size uint32) []byte { return eocd(1, 5, size, 0) }),
		},
		{
			name: "underreported directory size",
			data: centralOnlyZip(entries, 0, func(size uint32) []byte { return eocd(5, 5, 47*5, 0) }),
		},
		{
			name: "zero directory size",
			data: centralOnlyZip(entries, 0, func(size uint32) []byte { return eocd(5, 5, 0, 0) }),
		},
		{
			name: "EOCD outside legacy search window",
			data: centralOnlyZip(entries, 65_600, func(size uint32) []byte { return eocd(5, 5, size, 0) }),
		},
		{
			name: "ZIP64 disk number differs",
			data: centralOnlyZip(entries, 0, func(size uint32) []byte { return zip64Tail(5, 5, 1, size) }),
		},
		{
			name: "ZIP64 per-disk entry count differs",
			data: centralOnlyZip(entries, 0, func(size uint32) []byte { return zip64Tail(0, 5, 0, size) }),
		},
		{
			name: "ZIP64 sentinel without locator",
			data: centralOnlyZip(131_071, 0, func(size uint32) []byte { return eocd(0xffff, 0xffff, size, 0) }),
		},
	}
	if got := len(cases[0].data) - int(cdSize); got != eocdFixedSize {
		t.Fatalf("fixture tail size = %d, want %d", got, eocdFixedSize)
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := preflightEntryCount(bytes.NewReader(tc.data), int64(len(tc.data)), limits, t.Context())
			if !errors.Is(err, ErrLimitExceeded) {
				t.Fatalf("preflight error = %v, want ErrLimitExceeded", err)
			}
		})
	}

	valid := metadataZip(t, limits.MaxEntries)
	if err := preflightEntryCount(bytes.NewReader(valid), int64(len(valid)), limits, t.Context()); err != nil {
		t.Fatalf("entry count at limit error = %v, want nil", err)
	}
}

func centralOnlyZip(entries, trailing int, tail func(cdSize uint32) []byte) []byte {
	central := make([]byte, entries*47)
	for i := range entries {
		record := central[i*47 : (i+1)*47]
		binary.LittleEndian.PutUint32(record[0:4], centralHeaderSig)
		binary.LittleEndian.PutUint16(record[28:30], 1)
		record[46] = 'a'
	}
	data := append(central, tail(uint32(len(central)))...)
	return append(data, make([]byte, trailing)...)
}

func eocd(diskEntries, totalEntries uint16, cdSize, cdOffset uint32) []byte {
	data := make([]byte, eocdFixedSize)
	binary.LittleEndian.PutUint32(data[0:4], eocdSignature)
	binary.LittleEndian.PutUint16(data[8:10], diskEntries)
	binary.LittleEndian.PutUint16(data[10:12], totalEntries)
	binary.LittleEndian.PutUint32(data[12:16], cdSize)
	binary.LittleEndian.PutUint32(data[16:20], cdOffset)
	return data
}

func zip64Tail(diskEntries, totalEntries uint64, diskNumber uint32, cdSize uint32) []byte {
	data := make([]byte, zip64EOCDMinimum+zip64LocatorSize+eocdFixedSize)
	binary.LittleEndian.PutUint32(data[0:4], zip64EOCDSignature)
	binary.LittleEndian.PutUint64(data[4:12], zip64EOCDMinimum-12)
	binary.LittleEndian.PutUint32(data[16:20], diskNumber)
	binary.LittleEndian.PutUint64(data[24:32], diskEntries)
	binary.LittleEndian.PutUint64(data[32:40], totalEntries)
	binary.LittleEndian.PutUint64(data[40:48], uint64(cdSize))
	locator := data[zip64EOCDMinimum : zip64EOCDMinimum+zip64LocatorSize]
	binary.LittleEndian.PutUint32(locator[0:4], zip64LocatorSig)
	binary.LittleEndian.PutUint64(locator[8:16], uint64(cdSize))
	binary.LittleEndian.PutUint32(locator[16:20], 1)
	classic := data[zip64EOCDMinimum+zip64LocatorSize:]
	binary.LittleEndian.PutUint32(classic[0:4], eocdSignature)
	binary.LittleEndian.PutUint16(classic[8:10], 0xffff)
	binary.LittleEndian.PutUint16(classic[10:12], 0xffff)
	binary.LittleEndian.PutUint32(classic[12:16], 0xffffffff)
	binary.LittleEndian.PutUint32(classic[16:20], 0xffffffff)
	return data
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
