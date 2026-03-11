package version

// Set via -ldflags at build time.
var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

// String returns a formatted version string.
func String() string {
	return Version + " (commit: " + Commit + ", built: " + Date + ")"
}
