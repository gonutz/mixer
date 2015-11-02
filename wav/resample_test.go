package wav

import "testing"

func Test8BitTo16BitMapsRangesCorrectly(t *testing.T) {
	input := []byte{0, 255}
	output := convert8bitTo16bitSamples(input)
	checkBytes(t, output, []byte{0, 128, 255, 127})
}

func TestMonoToStereo16bitCopiesEachSample(t *testing.T) {
	input := []byte{1, 2, 3, 4, 5, 6}
	output := convert16bitMonoTo16bitStereo(input)
	checkBytes(t, output, []byte{1, 2, 1, 2, 3, 4, 3, 4, 5, 6, 5, 6})
}

func checkBytes(t *testing.T, actual, expected []byte) {
	if len(actual) != len(expected) {
		t.Fatal("different lengths, got", len(actual), "expected", len(expected))
	}

	for i := range actual {
		if actual[i] != expected[i] {
			t.Error("at index", i, "got", actual[i], "expected", expected[i])
		}
	}
}

func TestSplittingByteInputIntoFloatChannels(t *testing.T) {
	left, right := split2Stereo16bitChannelsToFloats([]byte{
		0, 0, // left 0
		255, 255, // right 0
		255, 127, // left 1
		0, 128, // right 1
	},
	)
	checkFloats(t, left, []float32{0, 32767})
	checkFloats(t, right, []float32{-1, -32768})
}

func checkFloats(t *testing.T, floats, expected []float32) {
	if len(floats) != len(expected) {
		t.Fatal("length", len(expected), "expected but was", len(floats))
	}
	for i := range expected {
		if floats[i] != expected[i] {
			t.Error("at index", i, "expected", expected[i], "but got", floats[i])
		}
	}
}

func TestResampleFrequency(t *testing.T) {
	// 2 samples at 2 Hz => 1 second
	input := []int16{1, 1, 3, 3}
	// resample the second at 3 Hz => 2*(3/2) = 3 samples
	output := convert16bitStereoFrequencies(toBytes(input), 2, 3)
	checkBytes(t, output, toBytes([]int16{1, 1, 2, 2, 3, 3}))

	input = []int16{1, 1, 3, 3, 5, 5}
	output = convert16bitStereoFrequencies(toBytes(input), 2, 3)
	checkBytes(t, output, toBytes([]int16{1, 1, 2, 2, 3, 3, 4, 4, 5, 5}))
}

func toBytes(input []int16) []byte {
	result := make([]byte, 0, len(input)*2)
	for _, i := range input {
		lo, hi := makeBytes(i)
		result = append(result, lo, hi)
	}
	return result
}

func makeBytes(sample int16) (lo, hi byte) {
	return byte(sample & 0xFF), byte((sample >> 8) & 0xFF)
}
