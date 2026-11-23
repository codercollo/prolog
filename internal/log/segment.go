// Segement the abstraction that ties a store and an index together
package log

import (
	"fmt"
	"os"
	"path"

	api "github.com/codercollo/prolog/api/v1"
	"google.golang.org/protobuf/proto"
)

type segment struct {
	store                  *store
	index                  *index
	baseOffset, nextOffset uint64
	config                 Config
}

// newSegment creates or opens a segment's store and index files and sets its next offset.
func newSegment(dir string, baseOffset uint64, c Config) (*segment, error) {
	s := &segment{
		baseOffset: baseOffset,
		config:     c,
	}

	var err error

	// Open or create the append-only store file named after the base offset.
	storeFile, err := os.OpenFile(path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".store")),
		os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	if s.store, err = newStore(storeFile); err != nil {
		return nil, err
	}

	// Open or create the index file named after the base offset
	indexFile, err := os.OpenFile(path.Join(dir, fmt.Sprintf("%d%s", baseOffset, ".index")),
		os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return nil, err
	}

	if s.index, err = newIndex(indexFile, c); err != nil {
		return nil, err
	}

	// Set the next offset to baseOffset if empty, or one past the last entry if resuming.
	if off, _, err := s.index.Read(-1); err != nil {
		s.nextOffset = baseOffset
	} else {
		s.nextOffset = baseOffset + uint64(off) + 1
	}

	return s, nil
}

// Append writes the record to the segment and returns the newly appended
// record's offset. The log returns the offset to the API response. The segment
// appends a record in a two step process: it appends the data to the store
// then adds an index entry.
func (s *segment) Append(record *api.Record) (offset uint64, err error) {
	cur := s.nextOffset
	record.Offset = cur
	p, err := proto.Marshal(record)
	if err != nil {
		return 0, err
	}

	_, pos, err := s.store.Append(p)
	if err != nil {
		return 0, err
	}

	if err = s.index.Write(
		// index offsets are relative to bas offset
		uint32(s.nextOffset-uint64(s.baseOffset)),
		pos,
	); err != nil {
		return 0, err
	}
	s.nextOffset++
	return cur, nil
}

// Read  returns the record for the given offset by looking
// in the index and unmarshalling it from the store.
func (s *segment) Read(off uint64) (*api.Record, error) {
	// Convert the absolute offset to a relative offset and
	// look up its position in the index.
	_, pos, err := s.index.Read(int64(off - s.baseOffset))
	if err != nil {
		return nil, err
	}

	// Read the row record bytes from the store at that position
	p, err := s.store.Read(pos)
	if err != nil {
		return nil, err
	}

	// Unmarshal the raw bytes into a protobuf Record and return it.
	record := &api.Record{}
	err = proto.Unmarshal(p, record)
	return record, err
}

// isMaxed returns whether the segment has reached its max size,
// either by writting too much to the store or the index.
// The log use it to know if it needs to create a new segment.
func (s *segment) IsMaxed() bool {
	return s.store.size >= s.config.Segment.MaxStoreBytes ||
		s.index.size >= s.config.Segment.MaxIndexBytes
}

// Remove closes the segment and removes the index and store files.
func (s *segment) Remove() error {
	if err := s.Close(); err != nil {
		return err
	}

	if err := os.Remove(s.index.Name()); err != nil {
		return err
	}

	if err := os.Remove(s.store.Name()); err != nil {
		return err
	}
	return nil
}

// Close closes both the segment's index and store files
func (s *segment) Close() error {
	if err := s.index.Close(); err != nil {
		return err
	}
	if err := s.store.Close(); err != nil {
		return err
	}
	return nil
}

// nearestMultiple returns the nearest and lesser multiple
// of k in j. We take the lesser multiple to make sure
// we stay under the user's disk capacity
func nearestMultiple(j, k uint64) uint64 {
	if j >= 0 {
		return (j / k) * k

	}
	return ((j - k + 1) / k) * k
}
