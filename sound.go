package mixer

import "time"

// TODO maybe be able to set the volume per channel instead of only changing
// the pan, this way a higher level lib can provide pan change functionality
// and additional operations; on the other hand this lib could provide other
// functions as well when needed or simply give the user the per-channel volume
// so she can do it herself.
type Sound interface {
	// SetPaused starts or stops the sound. Note that the sound position is not
	// changed with this function, meaning that if the sound is not playing
	// because it was played all the way to the end, calling SetPaused(false)
	// will not restart it from the beginning. You have to call SetPosition(0)
	// to reset the sound to the start. If the sound is not paused, it will then
	// play right away.
	SetPaused(bool)

	// Paused returns the last value set in SetPaused. It does not consider
	// whether the sound is being played right now. It may not be paused but
	// could have reached the end and thus is not audible although not paused.
	Paused() bool

	// Playing returns true if the sound is not paused and has not reached the
	// end.
	Playing() bool

	// SetVolume sets the volume factor for all channels. Its range is [0..1]
	// and it will be clamped to that range.
	// Note that the audible difference in loudness between 100% and 50% is the
	// same as between 50% and 25% and so on. Changing the sound on a
	// logarithmic scale will sound to the human ear as if you decrease the
	// sound by equal steps.
	SetVolume(float32)

	// Volume returns a value in the range of 0 (silent) to 1 (full volume).
	Volume() float32

	// SetPan changes the volume ratio between left and right output channel.
	// Setting it to -1 will make channel 1 (left speaker) output at 100% volume
	// while channel 2 (right speaker) has a volume of 0%.
	// A pan of 0 means both speakers' volumes are at 100%, +1 means the left
	// speaker is silenced.
	// This value is clamped to [-1..1]
	SetPan(float32)

	// Pan returns the current pan as a value in the range of -1 (only left
	// speaker) to 1 (only right speaker). A value of 0 means both speakers play
	// at full volume.
	Pan() float32

	// Length is the length of the whole sound, it does not consider how far it
	// is already played or if it loops or not.
	Length() time.Duration

	// SetPosition sets the time offset into the sound at which it will continue
	// to play.
	SetPosition(time.Duration)

	// Position is the current offset from the start of the sound. It changes
	// while the sound is played.
	Position() time.Duration
}
