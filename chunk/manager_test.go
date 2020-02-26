package chunk

import "testing"

func TestSplitChunkRanges(t *testing.T) {
	testcases := []struct {
		offset, size, chunkSize int64
		result                  []byteRange
	}{
		{0, 0, 4096, []byteRange{}},
		{0, 4096, 4096, []byteRange{
			{0, 4096},
		}},
		{4095, 4096, 4096, []byteRange{
			{4095, 1},
			{4096, 4095},
		}},
		{0, 8192, 4096, []byteRange{
			{0, 4096},
			{4096, 4096},
		}},
		{2048, 8192, 4096, []byteRange{
			{2048, 2048},
			{4096, 4096},
			{8192, 2048},
		}},
		{2048, 8192, 4096, []byteRange{
			{2048, 2048},
			{4096, 4096},
			{8192, 2048},
		}},
		{17960960, 16777216, 10485760, []byteRange{
			{17960960, 3010560},
			{20971520, 10485760},
			{31457280, 3280896},
		}},
	}
	for i, tc := range testcases {
		ranges := splitChunkRanges(tc.offset, tc.size, tc.chunkSize)
		actualSize := len(ranges)
		expectedSize := len(tc.result)
		if actualSize != expectedSize {
			t.Fatalf("ByteRange %v length mismatch: %v != %v", i, actualSize, expectedSize)
		}
		for j, r := range ranges {
			actual := r
			expected := tc.result[j]
			if actual != expected {
				t.Fatalf("ByteRange %v mismatch: %v != %v", i, actual, expected)
			}
		}
	}
}
