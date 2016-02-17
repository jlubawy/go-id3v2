// Implements ID3v2.3.0 described at http://id3.org/id3v2.3.0

package id3v230

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/jlubawy/go-id3v2"
)

const VersionString = "id3v2.3.0"

// a - Unsynchronisation
// Bit 7 in the 'ID3v2 flags' indicates whether or not unsynchronisation is used (see section 5 for details); a set bit indicates usage.
// b - Extended header
// The second bit (bit 6) indicates whether or not the header is followed by an extended header. The extended header is described in section 3.2.
// c - Experimental indicator
// The third bit (bit 5) should be used as an 'experimental indicator'. This flag should always be set when the tag is in an experimental stage.
const (
	HeaderFlagExperimentalIndicator = uint8(1 << 5)
	HeaderFlagExtendedHeader        = uint8(1 << 6)
	HeaderFlagUnsynchronisation     = uint8(1 << 7)
)

// ID3v2/file identifier   "ID3"
// ID3v2 version           $03 00
// ID3v2 flags             %abc00000
// ID3v2 size              4 * %0xxxxxxx
type header struct {
	ID        [3]byte
	Version   [2]byte
	Flags     byte
	SynchSafe uint32
}

const (
	ExtendedHeaderFlagCRC32DataPresent = uint16(1 << 15)
)

// Extended header size   $xx xx xx xx
// Extended Flags         $xx xx
// Size of padding        $xx xx xx xx
type extendedHeader struct {
	Size        uint32
	Flags       uint16
	PaddingSize uint32
}

// Frame ID       $xx xx xx xx (four characters)
// Size           $xx xx xx xx
// Flags          $xx xx
type frame struct {
	ID    [4]byte
	Size  uint32
	Flags uint16
}

type tag struct {
	header
	extendedHeader

	frames     map[string][]byte
	frameOrder []string
}

func (t *tag) Frames() map[string][]byte {
	return t.frames
}

func (t *tag) FrameOrder() []string {
	return t.frameOrder
}

func (t *tag) SetFrames(f map[string][]byte) {
	t.frames = f

	// Update the size
	hdrSize := uint32(binary.Size(frame{}))
	framesSize := uint32(0)
	for _, data := range f {
		framesSize = framesSize + hdrSize + 1 + uint32(binary.Size(data))
	}

	t.header.SynchSafe = id3v2.SizeToSynchSafe(framesSize)
}

func (t *tag) Size() uint32 {
	return id3v2.SynchSafeToSize(t.SynchSafe) + uint32(binary.Size(t.header))
}

func Decode(r io.Reader) (id3v2.Tag, error) {
	t := &tag{}

	if err := binary.Read(r, binary.BigEndian, &t.header); err != nil {
		return nil, err
	}

	bytesLeft := id3v2.SynchSafeToSize(t.header.SynchSafe)

	// Read the extended header if one exists
	if t.header.Flags&HeaderFlagExtendedHeader != 0 {
		if err := binary.Read(r, binary.BigEndian, &t.extendedHeader); err != nil {
			return nil, err
		}

		bytesLeft = bytesLeft - uint32(binary.Size(t.extendedHeader))

		// Read the CRC-32 data if any exists
		if t.extendedHeader.Flags&ExtendedHeaderFlagCRC32DataPresent != 0 {
			var crc32 uint32

			bytesLeft = bytesLeft - uint32(binary.Size(crc32))

			_ = crc32
		}
	}

	t.frames = make(map[string][]byte)

	for bytesLeft > 0 {
		f := frame{}

		if err := binary.Read(r, binary.BigEndian, &f); err != nil {
			return nil, err
		}

		bytesLeft = bytesLeft - uint32(binary.Size(f))

		if f.ID[0] == 0 {
			break
		}

		buf := &bytes.Buffer{}
		n, err := io.CopyN(buf, r, int64(f.Size))
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("blah")
			}
			return nil, err
		}
		if uint32(n) != f.Size {
			return nil, fmt.Errorf("id3v230: expected frame size %d but got %d", f.Size, n)
		}

		bytesLeft = bytesLeft - f.Size

		t.frameOrder = append(t.frameOrder, string(f.ID[:]))
		t.frames[string(f.ID[:])] = buf.Bytes()
	}

	return id3v2.Tag(t), nil
}

func Encode(w io.Writer, tag id3v2.Tag) error {
	fBuf := &bytes.Buffer{}

	for _, id := range tag.FrameOrder() {
		// Check that the frame still exists
		data, ok := tag.Frames()[id]
		if !ok {
			continue
		}

		if len(id) != 4 {
			return fmt.Errorf("id3v230: expected frame ID of length 4 but got %d", len(id))
		}
		if _, ok := SupportedFrames[id]; !ok {
			return fmt.Errorf("id3v230: unsupported frame ID '%s'", id)
		}

		f := frame{
			Size:  uint32(len(data)),
			Flags: 0,
		}
		copy(f.ID[:], []byte(id))

		if err := binary.Write(fBuf, binary.BigEndian, f); err != nil {
			return err
		}

		if err := binary.Write(fBuf, binary.BigEndian, data); err != nil {
			return err
		}
	}

	h := header{
		Version:   [2]byte{3, 0},
		Flags:     0,
		SynchSafe: id3v2.SizeToSynchSafe(uint32(fBuf.Len())),
	}
	copy(h.ID[:], id3v2.FileIdentifier)

	if err := binary.Write(w, binary.BigEndian, h); err != nil {
		return err
	}

	if _, err := io.Copy(w, fBuf); err != nil && err != io.EOF {
		return err
	}

	return nil
}

func init() {
	id3v2.RegisterVersion(3, 0, Decode)
}

// SupportedFlags is a map of frames supported by ID3v2.3.0 and their descriptions.
var SupportedFrames = map[string]string{
	"AENC": "[[#sec4.20|Audio encryption]]",
	"APIC": "[#sec4.15 Attached picture]",
	"COMM": "[#sec4.11 Comments]",
	"COMR": "[#sec4.25 Commercial frame]",
	"ENCR": "[#sec4.26 Encryption method registration]",
	"EQUA": "[#sec4.13 Equalization]",
	"ETCO": "[#sec4.6 Event timing codes]",
	"GEOB": "[#sec4.16 General encapsulated object]",
	"GRID": "[#sec4.27 Group identification registration]",
	"IPLS": "[#sec4.4 Involved people list]",
	"LINK": "[#sec4.21 Linked information]",
	"MCDI": "[#sec4.5 Music CD identifier]",
	"MLLT": "[#sec4.7 MPEG location lookup table]",
	"OWNE": "[#sec4.24 Ownership frame]",
	"PRIV": "[#sec4.28 Private frame]",
	"PCNT": "[#sec4.17 Play counter]",
	"POPM": "[#sec4.18 Popularimeter]",
	"POSS": "[#sec4.22 Position synchronisation frame]",
	"RBUF": "[#sec4.19 Recommended buffer size]",
	"RVAD": "[#sec4.12 Relative volume adjustment]",
	"RVRB": "[#sec4.14 Reverb]",
	"SYLT": "[#sec4.10 Synchronized lyric/text]",
	"SYTC": "[#sec4.8 Synchronized tempo codes]",
	"TALB": "[#TALB Album/Movie/Show title]",
	"TBPM": "[#TBPM BPM (beats per minute)]",
	"TCOM": "[#TCOM Composer]",
	"TCON": "[#TCON Content type]",
	"TCOP": "[#TCOP Copyright message]",
	"TDAT": "[#TDAT Date]",
	"TDLY": "[#TDLY Playlist delay]",
	"TENC": "[#TENC Encoded by]",
	"TEXT": "[#TEXT Lyricist/Text writer]",
	"TFLT": "[#TFLT File type]",
	"TIME": "[#TIME Time]",
	"TIT1": "[#TIT1 Content group description]",
	"TIT2": "[#TIT2 Title/songname/content description]",
	"TIT3": "[#TIT3 Subtitle/Description refinement]",
	"TKEY": "[#TKEY Initial key]",
	"TLAN": "[#TLAN Language(s)]",
	"TLEN": "[#TLEN Length]",
	"TMED": "[#TMED Media type]",
	"TOAL": "[#TOAL Original album/movie/show title]",
	"TOFN": "[#TOFN Original filename]",
	"TOLY": "[#TOLY Original lyricist(s)/text writer(s)]",
	"TOPE": "[#TOPE Original artist(s)/performer(s)]",
	"TORY": "[#TORY Original release year]",
	"TOWN": "[#TOWN File owner/licensee]",
	"TPE1": "[#TPE1 Lead performer(s)/Soloist(s)]",
	"TPE2": "[#TPE2 Band/orchestra/accompaniment]",
	"TPE3": "[#TPE3 Conductor/performer refinement]",
	"TPE4": "[#TPE4 Interpreted, remixed, or otherwise modified by]",
	"TPOS": "[#TPOS Part of a set]",
	"TPUB": "[#TPUB Publisher]",
	"TRCK": "[#TRCK Track number/Position in set]",
	"TRDA": "[#TRDA Recording dates]",
	"TRSN": "[#TRSN Internet radio station name]",
	"TRSO": "[#TRSO Internet radio station owner]",
	"TSIZ": "[#TSIZ Size]",
	"TSRC": "[#TSRC ISRC (international standard recording code)]",
	"TSSE": "[#TSEE Software/Hardware and settings used for encoding]",
	"TYER": "[#TYER Year]",
	"TXXX": "[#TXXX User defined text information frame]",
	"UFID": "[#sec4.1 Unique file identifier]",
	"USER": "[#sec4.23 Terms of use]",
	"USLT": "[#sec4.9 Unsychronized lyric/text transcription]",
	"WCOM": "[#WCOM Commercial information]",
	"WCOP": "[#WCOP Copyright/Legal information]",
	"WOAF": "[#WOAF Official audio file webpage]",
	"WOAR": "[#WOAR Official artist/performer webpage]",
	"WOAS": "[#WOAS Official audio source webpage]",
	"WORS": "[#WORS Official internet radio station homepage]",
	"WPAY": "[#WPAY Payment]",
	"WPUB": "[#WPUB Publishers official webpage]",
	"WXXX": "[#WXXX User defined URL link frame]",
}
