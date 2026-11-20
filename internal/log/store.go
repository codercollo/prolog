// Store file we store records in.
package log

import (
	"bufio"
	"encoding/binary"
	"os"
	"sync"
)

// enc defines the byte order used to serialize record sizes and index entries.
var (
	enc = binary.BigEndian
)

// lenWidth defines the number of bytes used to store a record's length.
const (
	lenWidth = 8
)

// store wraps an OS file with buffered writes and thread-safe size tracking.
type store struct {
	*os.File
	mu   sync.Mutex
	buf  *bufio.Writer
	size uint64
}

// newStore create a store for the given file and records its current size.
func newStore(f *os.File) (*store, error) {
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	size := uint64(fi.Size())
	return &store{
		File: f,
		size: size,
		buf:  bufio.NewWriter(f),
	}, nil
}

// Append writes the length-prefixed record to the buffered store
// and returns bytes written and its starting position.
func (s *store) Append(p []byte) (n uint64, pos uint64, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Record the current file size as the starting position.
	pos = s.size

	// Write the record's byte length prefix
	if err := binary.Write(s.buf, enc, uint64(len(p))); err != nil {
		return 0, 0, err
	}

	// Write the actual record payload to the buffer.
	w, err := s.buf.Write(p)
	if err != nil {
		return 0, 0, err
	}

	// Update total store size and return total bytes written.
	w += lenWidth
	s.size += uint64(w)
	return uint64(w), pos, nil
}

// Read flushes buffered writes and returns the record stored at
// the given position.
//
// It returns the record stored at the given postion.
func (s *store) Read(pos uint64) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Flush the writer buffer to ensure all records are on disk before reading.
	if err := s.buf.Flush(); err != nil {
		return nil, err
	}

	// Read the record's byte length prefix from the given position.
	size := make([]byte, lenWidth)
	if _, err := s.File.ReadAt(size, int64(pos)); err != nil {
		return nil, err
	}

	// Read and return the record payload following the length prefix
	b := make([]byte, enc.Uint64(size))
	if _, err := s.File.ReadAt(b, int64(pos+lenWidth)); err != nil {
		return nil, err
	}

	return b, nil
}

// ReadAt reads byte length into p beginning at the offset in the
// store's file.
func (s *store) ReadAt(p []byte, off int64) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.buf.Flush(); err != nil {
		return 0, err
	}

	return s.File.ReadAt(p, off)
}

// Close persists any buffered data before closing the file
func (s *store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.buf.Flush()
	if err != nil {
		return err
	}

	return s.File.Close()
}
