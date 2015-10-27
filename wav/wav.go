// Package wav provides functions to load audio files in the WAV (wave) format.
// Only uncompressed PCM formats are supported.
// Unknown chunks in the WAV file (like a LIST chunk which may contain
// additional information in the form of tags) are simply ignored when loading.
package wav

// TODO function comments

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"
)

type Wave struct {
	SoundChunks []SoundChunk
}

type SoundChunk struct {
	ChannelCount     int
	SamplesPerSecond int
	BitsPerSample    int
	Data             []byte
}

func LoadFromFile(path string) (*Wave, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	return Load(file)
}

func Load(r io.Reader) (*Wave, error) {
	var header waveHeader
	if err := binary.Read(r, endiannes, &header); err != nil {
		return nil, loadErr("reading 'RIFF' header", err)
	}
	if header.ChunkID != riffChunkID {
		return nil, errors.New("load WAV: expected 'RIFF' as the ID but got '" +
			header.ChunkID.String() + "'")
	}
	if header.WaveID != waveChunkID {
		return nil, errors.New("load WAV: expected 'WAVE' ID in header but got '" +
			header.WaveID.String() + "'")
	}

	data := make([]byte, header.ChunkSize-4)
	_, err := io.ReadFull(r, data[:])
	if err != nil {
		return nil, loadErr("illegal chunk size in header", err)
	}

	wav := &Wave{}
	if err := wav.parse(bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return wav, nil
}

func loadErr(msg string, err error) error {
	return errors.New("load WAV: " + msg + ": " + err.Error())
}

var endiannes = binary.LittleEndian

func (wav *Wave) parse(r *bytes.Reader) error {
	if r.Len() == 0 {
		return nil
	}

	var header chunkHeader
	if err := binary.Read(r, endiannes, &header); err != nil {
		return loadErr("unable to read chunk header", err)
	}

	if header.ChunkID == formatChunkID {
		var chunk formatChunkExtended
		if header.ChunkSize == 16 {
			if err := binary.Read(r, endiannes, &(chunk.formatChunkBase)); err != nil {
				return loadErr("reading format chunk", err)
			}
		} else if header.ChunkSize == 18 {
			err := binary.Read(r, endiannes, &(chunk.formatChunkWithExtension))
			if err != nil {
				return loadErr("reading format chunk", err)
			}
		} else if header.ChunkSize == 40 {
			if err := binary.Read(r, endiannes, &chunk); err != nil {
				return loadErr("reading format chunk", err)
			}
		} else {
			return fmt.Errorf("load WAV: illegal format chunk header size: %v",
				header.ChunkSize)
		}

		if chunk.FormatTag != pcmFormat {
			return fmt.Errorf(
				"load WAV: unsupported format: %v (only PCM is supported)",
				chunk.FormatTag)
		}

		soundChunk := SoundChunk{
			ChannelCount:     int(chunk.Channels),
			SamplesPerSecond: int(chunk.SamplesPerSec),
			BitsPerSample:    int(chunk.BitsPerSample),
		}
		wav.SoundChunks = append(wav.SoundChunks, soundChunk)
	} else if header.ChunkID == dataChunkID {
		data := make([]byte, header.ChunkSize)
		if _, err := io.ReadFull(r, data); err != nil {
			return err
		}

		last := len(wav.SoundChunks) - 1
		if last == -1 {
			return errors.New("load WAV: found data chunk before format chunk")
		}
		if wav.SoundChunks[last].Data == nil {
			wav.SoundChunks[last].Data = data
		} else {
			wav.SoundChunks[last].Data = append(wav.SoundChunks[last].Data, data...)
		}

		if header.ChunkSize%2 == 1 {
			// there is one byte padding if the chunk size is odd
			if _, err := r.ReadByte(); err != nil {
				return loadErr("reading data chunk padding", err)
			}
		}
	} else {
		// skip unknown chunks
		io.CopyN(ioutil.Discard, r, int64(header.ChunkSize))
	}

	if r.Len() == 0 {
		if len(wav.SoundChunks) == 0 {
			return errors.New("load WAV: file does not contain format information")
		}
		return nil
	}

	return wav.parse(r)
}

func (wav *Wave) String() string {
	if len(wav.SoundChunks) == 0 {
		return "Wave{}"
	}
	if len(wav.SoundChunks) == 1 {
		return "Wave{" + wav.SoundChunks[0].String() + "}"
	}
	chunkStrings := make([]string, len(wav.SoundChunks))
	for i := range wav.SoundChunks {
		chunkStrings[i] = "{" + wav.SoundChunks[i].String() + "}"
	}
	return "Wave{" + strings.Join(chunkStrings, ",") + "}"
}

func (s SoundChunk) String() string {
	return fmt.Sprintf(
		"%v channels, %v bits/sample, %v samples/sec, %v samples (%v bytes)",
		s.ChannelCount, s.BitsPerSample, s.SamplesPerSecond,
		len(s.Data)/(s.ChannelCount*s.BitsPerSample/8), len(s.Data),
	)
}

type chunkHeader struct {
	ChunkID   chunkID
	ChunkSize uint32
}

type waveHeader struct {
	// ChunkID should be "RIFF"
	// ChunkSize is 4 + length of the wave chunks that follow after the header
	chunkHeader
	WaveID chunkID // should be "WAVE"
}

type chunkID [4]byte

func (c chunkID) String() string { return string(c[:]) }

var (
	riffChunkID   = chunkID{'R', 'I', 'F', 'F'}
	waveChunkID   = chunkID{'W', 'A', 'V', 'E'}
	formatChunkID = chunkID{'f', 'm', 't', ' '}
	dataChunkID   = chunkID{'d', 'a', 't', 'a'}
	factChunkID   = chunkID{'f', 'a', 'c', 't'}
	listChunkID   = chunkID{'L', 'I', 'S', 'T'}
)

type waveChunk struct {
	id   uint32
	size uint32
}

type formatCode uint16

const (
	pcmFormat        formatCode = 0x0001
	ieeeFormat                  = 0x0003
	aLawFormat                  = 0x0006
	muLawFormat                 = 0x0007
	extensibleFormat            = 0xFFFE
)

func (f formatCode) String() string {
	switch f {
	case pcmFormat:
		return "PCM"
	case ieeeFormat:
		return "IEEE float"
	case aLawFormat:
		return "8-bit ITU-T G.711 A-law"
	case muLawFormat:
		return "8-bit ITU-T G.711 µ-law"
	case extensibleFormat:
		return "Extensible"
	default:
		return "Unknown"
	}
}

type formatChunkBase struct {
	FormatTag      formatCode
	Channels       uint16
	SamplesPerSec  uint32
	AvgBytesPerSec uint32
	BlockAlignment uint16
	BitsPerSample  uint16
}

type formatChunkWithExtension struct {
	formatChunkBase
	ExtensionSize uint16
}

type formatChunkExtended struct {
	formatChunkWithExtension
	ValidBitsPerSample uint16
	ChannelMask        uint32
	SubFormat          [16]byte
}

func (c formatChunkExtended) String() string {
	return fmt.Sprintf(`WAV Format Chunk {
	%s format
	%v channel(s)
	%v samples/s
	%v bytes/s
	%v byte block alignment
	%v bits/sample
}`,
		c.FormatTag, c.Channels, c.SamplesPerSec, c.AvgBytesPerSec,
		c.BlockAlignment, c.BitsPerSample)
}
