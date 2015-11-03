// TODO package doc
// TODO make sound source releasable

package mixer

import (
	"github.com/gonutz/mixer/dsound"
	"github.com/gonutz/mixer/wav"
	"sync"
	"time"
)

var (
	// writeCursor keeps the offset into DirectSound's ring buffer at which data
	// was written last
	writeCursor uint

	// sources are all currently active sound sources from which the mixed
	// sound output is mixed
	sources []*soundSource

	// lock is for changes to the mixer state and changes to the sound, these
	// must not occur while mixing sound data
	lock sync.Mutex

	// volume is in the range from 0 (silent) to 1 (full volume)
	volume float

	// stop is a signalling so the mixer knows when to stop updating (which
	// happens in a separate Go routine)
	stop chan bool

	// mixBuffer is the buffer for mixing the sound sources; its size is the
	// number of bytes that are output to the sound card for being played in the
	// future
	mixBuffer []byte

	// lastError keeps the last error encountered by the mixer; it can be
	// queried by the client with the Error function
	lastError error
)

const sampleSize = 4 // 2 channels, 16 bit each
const bytesPerSecond = 44100 * sampleSize

func Init() error {
	if err := dsound.Init(44100); err != nil {
		return err
	}

	writeAhead := bytesPerSecond / 10     // buffer 100ms
	writeAhead -= writeAhead % sampleSize // should be dividable into samples
	mixBuffer = make([]byte, writeAhead)
	volume = 1

	if err := dsound.WriteToSoundBuffer(mix(), 0); err != nil {
		return err
	}
	if err := dsound.StartSound(); err != nil {
		return err
	}

	stop = make(chan bool, 1)
	go func() {
		pulse := time.Tick(10 * time.Millisecond)
		for {
			select {
			case <-pulse:
				update()
				if lastError != nil {
					return
				}
			case <-stop:
				return
			default:
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()

	return nil
}

// Stop blocks until playing sound is stopped.
func Close() {
	stop <- true
	dsound.StopSound()
	dsound.Close()
}

func Error() error {
	return lastError
}

func update() {
	lock.Lock()
	defer lock.Unlock()

	_, write, err := dsound.GetPlayAndWriteCursors()
	if err != nil {
		lastError = err
		return
	}
	if write != writeCursor {
		var delta uint
		if write > writeCursor {
			delta = write - writeCursor
		} else {
			// wrap-around happened in DirectSound's ring buffer
			delta = write + dsound.BufferSize() - writeCursor
		}
		advanceSourcesBy(int(delta))

		// rewrite the whole look-ahead with newly mixed data
		lastError = dsound.WriteToSoundBuffer(mix(), write)
		if lastError != nil {
			return
		}
	}
	writeCursor = write
}

func mix() []byte {
	for i := 0; i < len(mixBuffer); i += 2 {
		var f float
		for _, source := range sources {
			if source.paused {
				continue
			}
			factor := source.volume
			if i%4 == 0 && source.pan > 0 {
				factor *= 1 - source.pan
			}
			if i%4 == 2 && source.pan < 0 {
				factor *= 1 + source.pan
			}
			f += source.floatSampleAt(i) * factor
		}
		f *= volume
		mixBuffer[i], mixBuffer[i+1] = floatSampleToBytes(f)
	}
	return mixBuffer
}

func bytesToFloatSample(b1, b2 byte) float {
	lo, hi := int16(uint16(b1)), int16(b2)
	return float(lo + 256*hi)
}

func floatSampleToBytes(f float) (b1, b2 byte) {
	// TODO check what happens when rounding to max or min, make sure
	// values do not clip at these bounds
	const min = -32768
	const max = 32767
	if f < min {
		f = min + 0.1
	}
	if f > max {
		f = max - 0.1
	}
	asInt16 := roundFloatToInt16(f)
	return byte(asInt16 & 0xFF), byte((asInt16 >> 8) & 0xFF)
}

func roundFloatToInt16(f float) int16 {
	if f > 0 {
		return int16(f + 0.5)
	}
	return int16(f - 0.5)
}

func advanceSourcesBy(byteCount int) {
	for _, source := range sources {
		if !source.paused {
			source.advanceBy(byteCount)
		}
	}
}

func SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	lock.Lock()
	defer lock.Unlock()

	volume = float(v)
}

// NewSound creates a new sound source from the given wave data and starts
// playing it right away. You can call SetPlaying(false) on the returned sound
// if you do not want to play the sound right away.
func NewSound(sound *wav.Wave) Sound {
	sound = wav.ConvertTo44100Hz2Channels16BitSamples(sound)
	source := newSoundSource(sound)

	lock.Lock()
	defer lock.Unlock()

	sources = append(sources, source)
	return source
}

func newSoundSource(w *wav.Wave) *soundSource {
	return &soundSource{data: w.Data, volume: 1}
}

type soundSource struct {
	data   []byte
	cursor int
	paused bool
	volume float
	pan    float
}

func (s *soundSource) SetVolume(v float64) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	lock.Lock()
	defer lock.Unlock()

	s.volume = float(v)
}

func (s *soundSource) Volume() float64 {
	return float64(s.volume)
}

func (s *soundSource) SetPan(p float64) {
	if p < -1 {
		p = -1
	}
	if p > 1 {
		p = 1
	}

	lock.Lock()
	defer lock.Unlock()

	s.pan = float(p)
}

func (s *soundSource) Pan() float64 {
	return float64(s.pan)
}

func (s *soundSource) Playing() bool {
	return !s.paused && s.cursor < len(s.data)
}

func (s *soundSource) isDone() bool {
	return s.cursor >= len(s.data)
}

func (s *soundSource) advanceBy(byteCount int) {
	s.cursor += byteCount
	if s.cursor > len(s.data) {
		s.cursor = len(s.data)
	}
}

func (s *soundSource) floatSampleAt(index int) float {
	index += s.cursor
	if index+1 > len(s.data) {
		return 0
	}
	return bytesToFloatSample(s.data[index], s.data[index+1])
}

func (s *soundSource) SetPaused(paused bool) {
	lock.Lock()
	defer lock.Unlock()

	s.paused = paused
}

func (s *soundSource) Paused() bool {
	return s.paused
}

func (s *soundSource) Length() time.Duration {
	return time.Duration(float64(len(s.data))/bytesPerSecond*1000000000) * time.Nanosecond
}

func (s *soundSource) Position() time.Duration {
	return time.Duration(float64(s.cursor)/bytesPerSecond*1000000000) * time.Nanosecond
}

func (s *soundSource) SetPosition(pos time.Duration) {
	lock.Lock()
	defer lock.Unlock()

	s.cursor = int(bytesPerSecond*pos.Seconds() + 0.5)
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor > len(s.data) {
		s.cursor = len(s.data)
	}
}
