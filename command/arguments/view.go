package arguments

// View represents the global command-line arguments which configure the view.
type View struct {
	// NoColor is used to disable the use of terminal color codes in all
	// output.
	NoColor bool

	// CompactWarnings is used to coalesce duplicate warnings, to reduce the
	// level of noise when multiple instances of the same warning are raised
	// for a configuration.
	CompactWarnings bool
}

// ParseView processes CLI arguments, returning a View value and a
// possibly-modified slice of arguments. If any of the supported flags are
// found, they will be removed from the slice.
func ParseView(args []string) (*View, []string) {
	common := &View{}

	i := 0
	for _, v := range args {
		switch v {
		case "-no-color":
			common.NoColor = true
		case "-compact-warnings":
			common.CompactWarnings = true
		default:
			// copy and increment index
			args[i] = v
			i++
		}
	}
	return common, args[:i]
}
