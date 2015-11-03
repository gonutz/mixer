package mixer

import "time"

// TODO maybe be able to set the volume per channel instead of only changing
// the pan, this way a higher level lib can provide pan change functionality
// and additional operations; on the other hand this lib could provide other
// functions as well when needed or simplye give the user the per-channel volume
// so she can do it herself.
type Sound interface {
	//Play()
	//Stop()

	SetPaused(bool)

	Paused() bool

	Playing() bool

	// SetVolume sets the volume factor for all channels. Its range is [0..1]
	// and it will be clamped to that range.
	// Note that the audible difference in loudness between 100% and 50% is the
	// same as between 50% and 25% and so on. Changing the sound on a
	// logarithmic scale will sound to the human ear as if you decrease the
	// sound by equal steps.
	SetVolume(float64)

	// Volume returns a value in the range of 0 (silent) to 1 (full volume).
	Volume() float64

	// SetPan changes the volume ratio between left and right output channel.
	// Setting it to -1 will make channel 1 (left speaker) output at 100% volume
	// while channel 2 (right speaker) has a volume of 0%.
	// A pan of 0 means both speakers' volumes are at 100%, +1 means the left
	// speaker is silenced.
	// This value is clamped to [-1..1]
	SetPan(float64)

	// Pan returns the current pan as a value in the range of -1 (only left
	// speaker) to 1 (only right speaker). A value of 0 means both speakers play
	// at full volume.
	Pan() float64

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
