// TODO there is a noise at the start of playing -> what is it?
package main

import (
	"fmt"
	"github.com/gonutz/mixer/dsound"
	"github.com/gonutz/mixer/wav"
	"sync"
	"time"
)

func main() {
	if err := dsound.InitDirectSound(44100); err != nil {
		fmt.Println(err)
		return
	}
	defer dsound.CloseDirectSound()

	// load test WAV
	music, err := wav.LoadFromFile("./music.wav")
	if err != nil {
		panic(err)
	}
	fmt.Println(music)
	drop, err := wav.LoadFromFile("./oil_drop.wav")
	if err != nil {
		panic(err)
	}
	fmt.Println(drop)

	mixer := newMixer()
	mixer.Play(music.SoundChunks[0].Data)
	go func() {
		time.Sleep(3 * time.Second)
		fmt.Println("Slience!")
		mixer.SetVolume(0)
	}()

	fmt.Scanln()

	if mixer.err != nil {
		panic(mixer.err)
	}
}

func newMixer() *mixer {
	return &mixer{
		// TODO what should this time be? It has to be responsive so settings
		// the volume, stopping sounds and other such actions do not cause a
		// great delay
		delay:   10 * time.Millisecond,
		volume:  1.0,
		changed: &syncBool{},
	}
}

type mixer struct {
	soundData []byte
	lastPlay  uint
	nextWrite uint
	playing   bool
	delay     time.Duration
	volume    float
	err       error
	changed   *syncBool
}

type syncBool struct {
	sync.Mutex
	value bool
}

func (b *syncBool) Set() {
	b.Lock()
	defer b.Unlock()
	b.value = true
}

func (b *syncBool) GetAndReset() bool {
	b.Lock()
	defer b.Unlock()
	value := b.value
	b.value = false
	return value
}

func (m *mixer) Play(data []byte) {
	m.soundData = data
	go m.start()
}

func (m *mixer) start() {
	const bufferSize = 2 * 4 * 44100 // TODO know this from lib or set as parameter

	pulse := time.Tick(m.delay)
	for {
		select {
		case <-pulse:
			if m.err != nil || len(m.soundData) == 0 {
				return
			}

			if !m.playing {
				// special start condition, write a chunk of data to position 0

				size := bufferSize
				if size > len(m.soundData) {
					size = len(m.soundData)
				}

				m.err = dsound.WriteToSoundBuffer(m.adjust(m.soundData[:size]), 0)
				if m.err != nil {
					return
				}
				m.soundData = m.soundData[size:]
				m.nextWrite = uint(size % bufferSize)

				m.err = dsound.StartSound()
				if m.err != nil {
					return
				}
				m.playing = true
			} else {
				// while already playing, we need to find the play cursor and
				// write up to that the next sound data, starting at where the
				// last write ended

				play, _, err := dsound.GetPlayAndWriteCursors()
				if err != nil {
					m.err = err
					return
				}

				if play != m.lastPlay {
					m.lastPlay = play

					var size uint
					if play > m.nextWrite {
						size = play - m.nextWrite
					} else if play < m.nextWrite {
						size = (bufferSize - m.nextWrite) + play
					}

					// NOTE no else: if play == m.nextWrite we cannot yet write there
					if size > 0 {
						if size > uint(len(m.soundData)) {
							size = uint(len(m.soundData))
						}

						m.err = dsound.WriteToSoundBuffer(m.adjust(m.soundData[:size]),
							m.nextWrite)
						if m.err != nil {
							return
						}
						m.soundData = m.soundData[size:]
						m.nextWrite = (m.nextWrite + size) % bufferSize
					}
				}
			}
		default:
			time.Sleep(1 * time.Millisecond)
		}
	}
}

func (m *mixer) SetVolume(scale float64) {
	m.volume = float(scale)
	m.changed.Set()
}

func (m *mixer) adjust(data []byte) []byte {
	// TODO check what happens when rounding to max or min, make sure
	// values do not clip at these bounds
	const min = -32768
	const max = 32767
	count := len(data) / 2
	for i := 0; i < count; i++ {
		lo, hi := int16(uint16(data[i*2])), int16(data[i*2+1])
		f := float(lo + 256*hi)
		f *= m.volume
		if f < min {
			f = min + 0.1
		}
		if f > max {
			f = max - 0.1
		}
		back := roundFloatToInt16(f)
		data[i*2], data[i*2+1] = byte(back&0xFF), byte((back>>8)&0xFF)
	}
	return data
}

func roundFloatToInt16(f float) int16 {
	if f > 0 {
		return int16(f + 0.5)
	}
	return int16(f - 0.5)
}
