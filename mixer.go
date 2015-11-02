// TODO package doc
// TODO make sure sample length are % 4 so there is no []byte index panic

package mixer

import (
	"github.com/gonutz/mixer/dsound"
	"github.com/gonutz/mixer/wav"
	"sync"
	"time"
)

type Mixer interface {
	Start()
	Stop() error
	Play(*wav.Wave) Sound
	SetVolume(float64)
	Error() error
}

// TODO maybe be able to set the volume per channel instead of only changing
// the pan, this way a higher level lib can provide pan change functionality
// and additional operations; on the other hand this lib could provide other
// functions as well when needed or simplye give the user the per-channel volume
// so she can do it herself.
type Sound interface {
	//Play()
	//Pause()
	//SetPositionOffset(time.Duration)
	Playing() bool
	// SetVolume sets the volume factor for all channels. Its range is [0..1]
	// and it will be clamped to that range.
	// Note that the audible difference in loudness between 100% and 50% is the
	// same as between 50% and 25% and so on. Changing the sound on a
	// logarithmic scale will sound to the human ear as if you decrease the
	// sound by equal steps.
	SetVolume(float64)
	Volume() float64
	// SetPan changes the volume ratio between left and right output channel.
	// Setting it to -1 will make channel 1 (left speaker) output at 100% volume
	// while channel 2 (right speaker) has a volume of 0%.
	// A pan of 0 means both speakers' volumes are at 100%, +1 means the left
	// speaker is silenced.
	// This value is clamped to [-1..1]
	SetPan(float64)
	Pan() float64
}

func New() Mixer {
	const bytesPerSecond = 44100 * 2      // TODO init DS here and set this
	writeAhead := bytesPerSecond / 10     // buffer 100ms
	const sampleSize = 4                  // 2 channels, 16 bit each
	writeAhead -= writeAhead % sampleSize // should be dividable into samples
	buffer := make([]byte, writeAhead)

	return &mixer{
		writeAheadByteCount: writeAhead, // TODO what should this be? 1/10 buffer size?
		volume:              1,
		updateDelay:         10 * time.Millisecond,
		changed:             &syncBool{},
		stop:                make(chan bool),
		mixBuffer:           buffer,
	}
}

type mixer struct {
	playCursor, writeCursor uint
	sources                 []*soundSource
	// writeAheadByteCount is the number of bytes that will be computed and
	// written in front of the write cursor
	writeAheadByteCount int
	// changed indicates that the pre-written sound data needs to be updated
	// because the state since pre-computing it has changed
	changed     *syncBool
	volume      float
	updateDelay time.Duration
	// playing indicates that the sound card is currently outputting sound
	playing   bool
	stop      chan bool
	mixBuffer []byte
	err       error
}

func (m *mixer) Error() error {
	return m.err // TODO assign this when an error occurs
}

func (m *mixer) Start() {
	go func() {
		pulse := time.Tick(m.updateDelay) // make the delay const
		for {
			select {
			case <-pulse:
				m.update()
			case <-m.stop:
				return
			default:
				time.Sleep(1 * time.Millisecond)
			}
		}
	}()
}

// Stop blocks until playing sound is stopped.
func (m *mixer) Stop() error {
	m.stop <- true
	m.playing = false
	return dsound.StopSound()
}

// TODO handle all errors (dsound calls)
func (m *mixer) update() {
	m.changed.Lock()
	defer m.changed.Unlock()

	if !m.playing { // TODO put this condition in Start(), it is only done once
		if len(m.sources) == 0 {
			// started without any sounds -> initialize the whole buffer to 0
			silence := make([]byte, dsound.BufferSize())
			dsound.WriteToSoundBuffer(silence, 0) // TODO error handling
		} else {
			// start playing the given sound sources, write cursor is at 0 (as
			// is the play cursor)
			dsound.WriteToSoundBuffer(m.remix(), 0)
		}
		dsound.StartSound()
		m.playing = true
	} else {
		m.changed.value = true // TODO remove this or maybe make this the default?
		if m.changed.value {
			play, write, _ := dsound.GetPlayAndWriteCursors()
			if write != m.writeCursor {
				var delta uint
				if write > m.writeCursor {
					delta = write - m.writeCursor
				} else {
					// wrap-around happened
					delta = write + dsound.BufferSize() - m.writeCursor
				}
				m.makeSourcesForget(int(delta))

				// rewrite the whole look-ahead with newly mixed data
				dsound.WriteToSoundBuffer(m.remix(), write)
			}
			m.playCursor, m.writeCursor = play, write
			m.changed.value = false
		} else {
			// TODO instead of re-writing the whole look-ahead, only compute the
			// sound data that is not yet written and append it at the end of
			// what is already written, e.g.
			//dsound.WriteToSoundBuffer(m.mix(), write+writeAheadByteCount-m.writeCursor)?
		}
	}
}

func (m *mixer) remix() []byte {
	for i := 0; i < len(m.mixBuffer); i += 2 {
		var f float
		for _, source := range m.sources {
			factor := source.volume
			if i%4 == 0 && source.pan > 0 {
				factor *= 1 - source.pan
			}
			if i%4 == 2 && source.pan < 0 {
				factor *= 1 + source.pan
			}
			f += source.floatSampleAt(i) * factor
		}
		f *= m.volume
		m.mixBuffer[i], m.mixBuffer[i+1] = floatSampleToBytes(f)
	}
	return m.mixBuffer
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

func (m *mixer) makeSourcesForget(byteCount int) {
	atLeastOneSourceIsDone := false
	for _, source := range m.sources {
		source.forget(byteCount)
		if source.isDone() {
			atLeastOneSourceIsDone = true
		}
	}

	if atLeastOneSourceIsDone {
		for i := 0; i < len(m.sources); i++ {
			if m.sources[i].isDone() {
				m.sources[i].mixer = nil
				m.sources = append(m.sources[:i], m.sources[i+1:]...)
				i--
			}
		}
	}
}

func (m *mixer) Play(sound *wav.Wave) Sound {
	m.changed.Lock()
	defer m.changed.Unlock()
	m.changed.value = true

	source := newSoundSource(sound, m)
	m.sources = append(m.sources, source)
	return source
}

func (m *mixer) SetVolume(v float64) {
	m.changed.Lock()
	defer m.changed.Unlock()
	m.changed.value = true

	if v < 0 {
		v = 0
	}
	m.volume = float(v)
}

// soundSource

func newSoundSource(w *wav.Wave, m *mixer) *soundSource {
	return &soundSource{w.Data, m, 1, 0}
}

// TODO handle frequency modulation in here (fitting source to destination
// samples/sec)?
type soundSource struct {
	data   []byte
	mixer  *mixer
	volume float
	pan    float
}

func (s *soundSource) SetVolume(v float64) {
	if s.mixer != nil {
		s.mixer.changed.Lock()
		defer s.mixer.changed.Unlock()
		s.mixer.changed.value = true
	}

	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	s.volume = float(v)
}

func (s *soundSource) Volume() float64 {
	return float64(s.volume)
}

func (s *soundSource) SetPan(p float64) {
	if s.mixer != nil {
		s.mixer.changed.Lock()
		defer s.mixer.changed.Unlock()
		s.mixer.changed.value = true
	}

	if p < -1 {
		p = -1
	}
	if p > 1 {
		p = 1
	}
	s.pan = float(p)
}

func (s *soundSource) Pan() float64 {
	return float64(s.pan)
}

func (s *soundSource) Playing() bool {
	return len(s.data) != 0
}

func (s *soundSource) isDone() bool {
	return len(s.data) == 0
}

func (s *soundSource) forget(byteCount int) {
	if len(s.data) <= byteCount {
		s.data = nil
	} else {
		s.data = s.data[byteCount:]
	}
}

func (s *soundSource) floatSampleAt(index int) float {
	if index+1 > len(s.data) {
		return 0
	}
	return bytesToFloatSample(s.data[index], s.data[index+1])
}

type syncBool struct {
	sync.Mutex
	value bool
}
