// TODO there is a noise at the start of playing -> what is it?
package main

import (
	"fmt"
	"github.com/gonutz/mixer/wav"
	"time"
)

func main() {
	if err := initDirectSound(44100); err != nil {
		fmt.Println(err)
		return
	}
	defer closeDirectSound()

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

	mixer := newMixer(music.SoundChunks[0].Data)
	go mixer.start()

	fmt.Scanln()

	if mixer.err != nil {
		panic(mixer.err)
	}
}

func makeTestData(data []byte) []byte {
	sampleCount := len(data) / 4
	i := 0
	// shift the pitch by 2x
	for ; i < sampleCount/2; i++ {
		a, b := i*4, i*2*4
		data[a] = data[b]
		data[a+1] = data[b+1]
		data[a+2] = data[b+2]
		data[a+3] = data[b+3]
	}
	// silence the right half (simple pitch change by 2x halves the period)
	for ; i < sampleCount; i++ {
		a := i * 4
		data[a] = 0
		data[a+1] = 0
		data[a+2] = 0
		data[a+3] = 0
	}
	return data
}

func pan(data []byte) []byte {
	count := len(data) / 2
	for i := 0; i < count; i++ {
		if i%2 == 1 {
			lo, hi := data[i*2], data[i*2+1]
			sound := int16(uint16(lo) | (uint16(hi) << 8))
			sound /= 10
			lo, hi = byte(uint16(sound)&0xFF), byte((uint16(sound)&0xFF00)>>8)
			data[i*2], data[i*2+1] = lo, hi
		}
	}
	return data
}

func newMixer(soundData []byte) *mixer {
	return &mixer{
		soundData: soundData,
		delay:     500 * time.Millisecond, // TODO maybe make this 1/4 of the
		// sound buffer time (which right now is 2s)?
	}
}

type mixer struct {
	soundData []byte
	lastPlay  uint
	nextWrite uint
	playing   bool
	delay     time.Duration
	err       error
}

func (m *mixer) start() {
	const bufferSize = 2 * 4 * 44100 // TODO know this from lib or set as parameter

	for {
		if m.err != nil || len(m.soundData) == 0 {
			return
		}

		if !m.playing {
			// special start condition, write a chunk of data to position 0

			size := bufferSize
			if size > len(m.soundData) {
				size = len(m.soundData)
			}

			m.err = writeToSoundBuffer(m.soundData[:size], 0)
			if m.err != nil {
				return
			}
			m.soundData = m.soundData[size:]
			m.nextWrite = uint(size % bufferSize)

			m.err = startSound()
			if m.err != nil {
				return
			}
			m.playing = true
		} else {
			// while already playing, we need to find the play cursor and write
			// up to that the next sound data, starting at where the last write
			// ended

			play, _, err := getPlayAndWriteCursors()
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

					m.err = writeToSoundBuffer(m.soundData[:size], m.nextWrite)
					if m.err != nil {
						return
					}
					m.soundData = m.soundData[size:]
					m.nextWrite = (m.nextWrite + size) % bufferSize
				}
			}
		}

		time.Sleep(m.delay)
	}
}
