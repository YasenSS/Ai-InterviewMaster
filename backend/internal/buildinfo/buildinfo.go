package buildinfo

// Values are overridden with -ldflags in release builds.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)
