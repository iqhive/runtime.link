package arm64

import "errors"

// encodeBitPattern computes n, immr, and imms for a given BitPattern.
func (bp BitPattern) encode() (n, immr, imms uint32, err error) {
	imm := uint64(bp)
	if imm == 0 || imm == 0xFFFFFFFFFFFFFFFF {
		return 0, 0, 0, errors.New("invalid bit pattern: all zeros or all ones")
	}

	elementSizes := []uint32{2, 4, 8, 16, 32, 64}
	for _, E := range elementSizes {
		if E == 64 {
			// n = 1 case: 64-bit element
			for S := uint32(0); S < 64; S++ {
				for R := uint32(0); R < 64; R++ {
					w := (uint64(1) << (S + 1)) - 1 // Base block of S+1 ones
					// Right rotation within 64 bits
					rotated := (w >> R) | ((w << (64 - R)) & 0xFFFFFFFFFFFFFFFF)
					if rotated == imm {
						return 1, R & 0x3F, S & 0x3F, nil // Mask to 6 bits
					}
				}
			}
		} else {
			// n = 0 case: smaller elements
			pattern := imm & ((1 << E) - 1) // Extract first E bits
			// Check if pattern repeats across 64 bits
			expected := uint64(0)
			for i := uint32(0); i < 64/E; i++ {
				expected |= pattern << (i * E)
			}
			if expected == imm {
				// Find rotation that makes pattern a contiguous block of ones
				for R := uint32(0); R < E; R++ {
					rotated := (pattern >> R) | ((pattern << (E - R)) & ((1 << E) - 1))
					if isConsecutiveOnes(rotated) {
						S := countTrailingOnes(rotated) - 1
						if S < E {
							return 0, R & 0x3F, S & 0x3F, nil
						}
					}
				}
			}
		}
	}
	return 0, 0, 0, errors.New("invalid bit pattern: not an ARM64 logical immediate")
}

// isConsecutiveOnes checks if a value is a contiguous block of ones (e.g., 00001111).
func isConsecutiveOnes(val uint64) bool {
	return val != 0 && ((val+1)&val) == 0
}

// countTrailingOnes counts consecutive ones from the right.
func countTrailingOnes(val uint64) uint32 {
	if val == 0 {
		return 0
	}
	count := uint32(0)
	temp := val
	for temp&1 == 1 {
		count++
		temp >>= 1
	}
	return count
}
