package rules

import "FIONA/internal/scanner"

type BySizeRule struct {
	IsPrimary bool
}

func (r *BySizeRule) GetDestination(fi *scanner.FileInfo) string {
	return assignSizeCategory(fi.Size)
}

func assignSizeCategory(size int64) string {
	switch {
	case size < 10_240:
		return "tiny"
	case size < 102_400:
		return "small"
	case size < 1_048_576:
		return "medium"
	case size < 10_485_760:
		return "large"
	case size < 104_857_600:
		return "huge"
	default:
		return "massive"
	}
}
