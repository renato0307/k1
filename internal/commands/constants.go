package commands

import "time"

// Command execution constants
const (
	// DefaultKubectlTimeout is the default timeout for kubectl subprocess
	// commands. Set to 30 seconds to handle slow clusters or large resource
	// operations while preventing indefinite hangs.
	DefaultKubectlTimeout = 30 * time.Second
)
