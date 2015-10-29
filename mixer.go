// TODO there is a noise at the start of playing -> what is it?
// TODO error handling
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
}

type Sound interface {
	SetVolume(float)
	Playing() bool
}

func New() Mixer {
	return &mixer{
		writeAheadByteCount: 44100 / 5, // TODO what should this be? 1/10 buffer size?
		volume:              1,
		updateDelay:         10 * time.Millisecond,
		changed:             &syncBool{},
		stop:                make(chan bool),
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
	playing bool
	stop    chan bool
}

func (m *mixer) Start() {
	go func() {
		pulse := time.Tick(m.updateDelay)
		for {
			select {
			case <-pulse:
				m.update()
			case <-m.stop:
				return
			default:
				time.Sleep(1 * time.Millisecond) // TODO really?
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
	buf := make([]byte, m.writeAheadByteCount) // TODO only allocate this once
	for i := 0; i < len(buf); i += 2 {
		var f float
		for _, source := range m.sources {
			f += source.floatSampleAt(i) * source.volume
		}
		f *= m.volume
		buf[i], buf[i+1] = floatSampleToBytes(f)
	}
	return buf
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

	m.volume = float(v)
}

// TODO handle frequency modulation in here (1. fitting source to destination
// samples/sec and 2. simple pitch shift)
type soundSource struct {
	data   []byte
	mixer  *mixer
	volume float
}

func (s *soundSource) SetVolume(v float) {
	if s.mixer == nil {
		return
	}

	s.mixer.changed.Lock()
	defer s.mixer.changed.Unlock()
	s.mixer.changed.value = true

	s.volume = v
}

func (s *soundSource) Playing() bool {
	if s.mixer == nil {
		return false
	}

	s.mixer.changed.Lock()
	defer s.mixer.changed.Unlock()

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

func newSoundSource(w *wav.Wave, m *mixer) *soundSource {
	return &soundSource{w.SoundChunks[0].Data, m, 1}
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
