package main

import (
	"fmt"
	"github.com/gonutz/mixer"
	"github.com/gonutz/mixer/dsound"
	"github.com/gonutz/mixer/wav"
	"time"
)

func main() {
	if err := dsound.Init(44100); err != nil {
		fmt.Println(err)
		return
	}
	defer dsound.Close()

	// load test WAV
	soundNames := []string{
		"music",
		"test",
		"switch",
		"oil_drop",
		"next_track",
	}
	sounds := make(map[string]*wav.Wave)
	for _, name := range soundNames {
		s, err := wav.LoadFromFile("./" + name + ".wav")
		if err != nil {
			panic(err)
		}
		sounds[name] = s
	}

	m := mixer.New()
	music := m.Play(sounds["music"])
	m.Start()
	defer m.Stop()

	for i := 0; i < 10; i++ {
		time.Sleep(1500 * time.Millisecond)
		fmt.Println("blop")
		m.Play(sounds["oil_drop"])

		if i == 3 {
			fmt.Println("sshhhhh")
			music.SetVolume(0.1)
		}
		if i == 6 {
			fmt.Println("huh?")
			music.SetVolume(1.0)
		}
	}

	time.Sleep(1 * time.Second)

	for music.Playing() {
		time.Sleep(1 * time.Second)
	}
}
