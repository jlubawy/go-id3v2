package id3v2

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
)

var ErrFormat = errors.New("id3v2: unknown format")
var ErrVersion = errors.New("id3v2: unknown version")

var FileIdentifier = []byte("ID3")

// A version defines an ID3v2 version and how to decode it.
type version struct {
	major, revision byte
	decode          func(io.Reader) (Tag, error)
}

// Versions is the list of registered versions.
var versions []version

func RegisterVersion(major, revision byte, decode func(io.Reader) (Tag, error)) {
	versions = append(versions, version{major, revision, decode})
}

type Tag interface {
	//Flags() byte
	//Size() uint32
	Frames() map[string][]byte
	FrameOrder() []string
	SetFrames(map[string][]byte)
	Size() uint32
}

func Decode(r io.Reader) (Tag, string, error) {
	var id [3]byte
	var version [2]byte

	br := bufio.NewReader(r)
	b, err := br.Peek(len(id) + len(version))
	if err != nil {
		return nil, "", ErrFormat
	}

	copy(id[:], b[0:])
	copy(version[:], b[3:])

	if !bytes.Equal(id[:], FileIdentifier) {
		return nil, fmt.Sprintf("id3v2.%d.%d", version[0], version[1]), ErrFormat
	}

	for _, ver := range versions {
		if bytes.Equal(version[:], []byte{ver.major, ver.revision}) {
			tag, err := ver.decode(br)
			return tag, fmt.Sprintf("id3v2.%d.%d", ver.major, ver.revision), err
		}
	}

	return nil, fmt.Sprintf("id3v2.%d.%d", version[0], version[1]), ErrVersion
}

// SizeToSynchSafe converts a normal 28-bit size to a synchsafe format.
func SizeToSynchSafe(s uint32) uint32 {
	if s > 0x0FFFFFFF {
		panic("id3v2: size must be less than 28-bits")
	}

	return ((s & 0xFE00000) << 3) | ((s & 0x1FC000) << 2) | ((s & 0x3F80) << 1) | (s & 0x7F)
}

// SynchSafeToSize converts a synchsafe format to a normal 28-bit size.
func SynchSafeToSize(s uint32) uint32 {
	return ((s & 0x7F000000) >> 3) | ((s & 0x7F0000) >> 2) | ((s & 0x7F00) >> 1) | (s & 0x7F)
}
