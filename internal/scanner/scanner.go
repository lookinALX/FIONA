package scanner

type Scanner struct {
	FollowSymlinks bool
	SkipHidden     bool
	MaxDepth       int

	filesScanned int
	dirsScanned  int
	errorsCount  int
	errors       []error
}
