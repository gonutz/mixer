// TODO package doc
// TODO make sound source releasable
// TODO right now the volume and pan only change in discrete chunks, whenever
// the sound buffer is written. To be accurate, individual samples would have
// to be modified depeding on when they are played, this can get hairy to
// implement so think about if it makes sense to do that or if the current
// solution is good enough
// TODO have SetPitch in Sound? Or In SoundSource?

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
	// sound output is computed
	sources []*soundSource

	// lock is for changes to the mixer state and changes to the sound, these
	// must not occur while mixing sound data
	lock sync.Mutex

	// volume is in the range from 0 (silent) to 1 (full volume)
	volume float32

	// stop is a signalling channel for the mixer to know when to stop updating
	// (which happens in a separate Go routine), e.g. after Close was called or
	// when an error happened from which it cannot recover
	stop chan bool

	// mixBuffer and writeAheadBuffer are the buffers for mixing the sound
	// sources; their size determines the time of the sound that will be output
	// for future playing in every mixer update
	mixBuffer               []float32
	leftBuffer, rightBuffer []float32
	writeAheadBuffer        []byte

	// lastError keeps the last error encountered by the mixer; it can be
	// queried by the client using the Error function
	lastError error
)

const bytesPerSample = 4 // 2 channels, 16 bit each
const bytesPerSecond = 44100 * bytesPerSample

func Init() error {
	if err := dsound.Init(44100); err != nil {
		return err
	}

	writeAheadByteCount := bytesPerSecond / 10 // buffer 100ms
	// make sure it is evenly dividable into samples
	writeAheadByteCount -= writeAheadByteCount % bytesPerSample
	writeAheadBuffer = make([]byte, writeAheadByteCount)
	mixBuffer = make([]float32, writeAheadByteCount/2) // 2 bytes form one value
	leftBuffer, rightBuffer = mixBuffer[:len(mixBuffer)/2], mixBuffer[len(mixBuffer)/2:]
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

// Close blocks until playing sound is stopped. It shuts down the DirectSound
// system.
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
		advanceSourcesByBytes(int(delta))

		// rewrite the whole look-ahead with newly mixed data
		lastError = dsound.WriteToSoundBuffer(mix(), write)
		if lastError != nil {
			return
		}
	}
	writeCursor = write
}

func (s *soundSource) addToMixBuffer() {
	if s.paused {
		return
	}

	writeTo := s.cursor + len(leftBuffer)
	if writeTo > len(s.left) {
		writeTo = len(s.left)
	}

	leftFactor := s.volume * s.leftPanFactor
	rightFactor := s.volume * s.rightPanFactor
	out := 0
	for i := s.cursor; i < writeTo; i++ {
		leftBuffer[out] += s.left[i] * leftFactor
		rightBuffer[out] += s.right[i] * rightFactor
		out++
	}
}

func mix() []byte {
	for i := range mixBuffer {
		mixBuffer[i] = 0.0
	}

	for _, source := range sources {
		source.addToMixBuffer()
	}

	out := 0
	for i := range leftBuffer {
		writeAheadBuffer[out], writeAheadBuffer[out+1] = floatToBytes(leftBuffer[i] * volume)
		writeAheadBuffer[out+2], writeAheadBuffer[out+3] = floatToBytes(rightBuffer[i] * volume)
		out += 4
	}

	return writeAheadBuffer
}

func floatToBytes(f float32) (lo, hi byte) {
	if f < 0 {
		if f < -1 {
			f = -1
		}
		value := int16(f * 32768)
		return byte(value & 0xFF), byte((value >> 8) & 0xFF)
	}

	if f > 1 {
		f = 1
	}
	value := int16(f * 32767)
	return byte(value & 0xFF), byte((value >> 8) & 0xFF)

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

func advanceSourcesByBytes(byteCount int) {
	for _, source := range sources {
		if !source.paused {
			source.advanceBySamples(byteCount / 4)
		}
	}
}

func SetVolume(v float32) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	lock.Lock()
	defer lock.Unlock()

	volume = v
}

// NewSound creates a new sound source from the given wave data and starts
// playing it right away. You can call SetPlaying(false) on the returned sound
// if you do not want to play the sound right away.
func NewSound(sound *wav.Wave) Sound {
	left, right := makeTwoChannelFloats(sound)
	source := newSoundSource(left, right)

	lock.Lock()
	defer lock.Unlock()

	sources = append(sources, source)
	return source
}

// TODO resample the right frequency
func makeTwoChannelFloats(w *wav.Wave) (left, right []float32) {
	if w.ChannelCount == 1 && w.BitsPerSample == 8 {
		result := make([]float32, len(w.Data))
		left = result
		right = result
		for i := range w.Data {
			result[i] = byteToFloat(w.Data[i])
		}
	} else if w.ChannelCount == 1 && w.BitsPerSample == 16 {
		result := make([]float32, len(w.Data)/2)
		left = result
		right = result
		in := 0
		for i := range result {
			result[i] = makeFloat(w.Data[in], w.Data[in+1])
			in += 2
		}
	} else if w.ChannelCount == 2 && w.BitsPerSample == 8 {
		result := make([]float32, len(w.Data))
		left, right = result[:len(result)/2], result[len(result)/2:]
		in := 0
		for i := range left {
			left[i] = byteToFloat(w.Data[in])
			right[i] = byteToFloat(w.Data[in+1])
			in += 2
		}
	} else if w.ChannelCount == 2 && w.BitsPerSample == 16 {
		data := w.Data
		if len(data)%4 != 0 {
			data = data[:len(data)-len(data)%4]
		}
		result := make([]float32, len(data)/2)
		left, right = result[:len(result)/2], result[len(result)/2:]
		in := 0
		for i := range left {
			left[i] = makeFloat(data[in], data[in+1])
			right[i] = makeFloat(data[in+2], data[in+3])
			in += 4
		}
	} else {
		panic("unsupported format, must be 1/2 channels and 8/16 bit samples")
		// TODO return an error here
	}
	return
}

func byteToFloat(b byte) float32 {
	// for 8 bit sound data, the value 128 is silence.
	if b >= 128 {
		return float32(b-128) / 127.0
	}
	return (float32(b) - 128) / 128.0
}

func makeFloat(b1, b2 byte) float32 {
	// 16 bit sound data is in little endian byte order signed int16s
	lo, hi := uint16(b1), uint16(b2)
	val := int16(lo | (hi << 8))
	if val >= 0 {
		return float32(val) / 32767
	}
	return float32(val) / 32768
}

// TODO make all float32 now float again? or remove the float altogether? or
// use ints instead (then again have a precision or use int?

func newSoundSource(left, right []float32) *soundSource {
	return &soundSource{
		left:           left,
		right:          right,
		volume:         1,
		leftPanFactor:  1,
		rightPanFactor: 1,
	}
}

type soundSource struct {
	left, right                   []float32
	cursor                        int
	paused                        bool
	volume                        float32
	pan                           float32
	leftPanFactor, rightPanFactor float32
}

func (s *soundSource) SetVolume(v float32) {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}

	lock.Lock()
	defer lock.Unlock()

	s.volume = v
}

func (s *soundSource) Volume() float32 {
	return s.volume
}

func (s *soundSource) SetPan(p float32) {
	if p < -1 {
		p = -1
	}
	if p > 1 {
		p = 1
	}

	left, right := float32(1), float32(1)
	if p < 0 {
		right = 1 + p
	}
	if p > 0 {
		left = 1 - p
	}

	lock.Lock()
	defer lock.Unlock()

	s.pan = p
	s.leftPanFactor, s.rightPanFactor = left, right
}

func (s *soundSource) Pan() float32 {
	return float32(s.pan)
}

func (s *soundSource) Playing() bool {
	return !s.paused && s.cursor < len(s.left)
}

func (s *soundSource) advanceBySamples(sampleCount int) {
	s.cursor += sampleCount
	if s.cursor > len(s.left) {
		s.cursor = len(s.left)
	}
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
	return time.Duration(float64(len(s.left))/bytesPerSecond*4000000000) * time.Nanosecond
}

func (s *soundSource) Position() time.Duration {
	return time.Duration(float64(s.cursor)/bytesPerSecond*4000000000) * time.Nanosecond
}

func (s *soundSource) SetPosition(pos time.Duration) {
	lock.Lock()
	defer lock.Unlock()

	s.cursor = int(bytesPerSecond*pos.Seconds()/4.0 + 0.5)
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.cursor > len(s.left) {
		s.cursor = len(s.left)
	}
}
