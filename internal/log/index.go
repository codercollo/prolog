// index - file we store index entries in.
package log

import (
	"io"
	"os"

	"github.com/tysonmote/gommap"
)

// Help define the number of bytes that make up each index entry.
// enWidth access the postion of an entry given offset .
var (
	offWidth uint64 = 4
	posWidth uint64 = 8
	entWidth        = offWidth + posWidth
)

// Index entry contains: the record's offset and its position
// in the store file
type index struct {
	file *os.File
	mmap gommap.MMap
	size uint64
}

// newIndex creates an index for the given file, grows it to max size and
// memory-maps it.
func newIndex(f *os.File, c Config) (*index, error) {
	idx := &index{
		file: f,
	}

	// Record the current file size before growing it
	fi, err := os.Stat(f.Name())
	if err != nil {
		return nil, err
	}
	idx.size = uint64(fi.Size())

	// Grow the file to the max index size  before memory-mapping.
	if err = os.Truncate(f.Name(), int64(c.Segment.MaxIndexBytes)); err != nil {
		return nil, err
	}

	// Memory-map the index file for fast read/write access
	if idx.mmap, err = gommap.Map(idx.file.Fd(), gommap.PROT_READ|gommap.PROT_WRITE, gommap.MAP_SHARED); err != nil {
		return nil, err
	}
	return idx, nil
}

// Read takes in an offset and returns the associated record's position
// in the store. The  given offset is relative to the segment's base
// offset.
// We use relative offset to reduce the size of the indexes by sorting offsets
// as uint32s
func (i *index) Read(in int64) (out uint32, pos uint64, err error) {
	// Return EOF if the index is empty
	if i.size == 0 {
		return 0, 0, io.EOF
	}

	// Select the last entry if -1 is passed, otherwise use the requested offset.
	if in == -1 {
		out = uint32((i.size / entWidth) - 1)
	} else {
		out = uint32(in)
	}

	// Calculate the entry's byte position and ensure it exists within the index.
	pos = uint64(out) * entWidth
	if i.size < pos+entWidth {
		return 0, 0, io.EOF
	}

	// Decode and return the record's offset and store position from the memory map.
	out = enc.Uint32(i.mmap[pos : pos+offWidth])
	pos = enc.Uint64(i.mmap[pos+entWidth : pos+entWidth])
	return out, pos, nil
}

// Write appends the given offset and postion to the index.
// We validate that we have space to write the entry. If there's
// sapce, we then encode the offset and postion and write them to the
// memory-mapped file. Then we increment the postion where the next write will go.

func (i *index) Write(off uint32, pos uint64) error {
	if uint64(len(i.mmap)) < i.size+entWidth {
		return io.EOF
	}

	enc.PutUint32(i.mmap[i.size:i.size+offWidth], off)
	enc.PutUint64(i.mmap[i.size+offWidth:i.size+entWidth], pos)
	i.size += uint64(entWidth)
	return nil
}

// Close ensures the memory-mapped file has synced its data to the
// persisted file and that persisted file has flushed its contents to
// stable storage . Then it truncates the persisted file to the amount
// of data that's actually in it and closes the file.
func (i *index) Close() error {
	if err := i.mmap.Sync(gommap.MS_ASYNC); err != nil {
		return err
	}

	if err := i.file.Sync(); err != nil {
		return err
	}

	if err := i.file.Truncate(int64(i.size)); err != nil {
		return err
	}
	return i.file.Close()
}

// Name returns the index's file path
func (i *index) Name() string {
	return i.file.Name()
}
