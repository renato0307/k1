package keyboard

// Keys holds all keyboard shortcut configurations for k1
type Keys struct {
	// Command Bar Activation
	FilterActivate  string // Activate filter mode
	PaletteActivate string // Activate command palette
	ResourceNav     string // Navigate to resources

	// Context Switching
	PrevContext string // Previous Kubernetes context
	NextContext string // Next Kubernetes context

	// Resource Operations
	Describe string // Describe resource
	Edit     string // Edit resource
	Logs     string // View logs
	YAML     string // View YAML
	Delete   string // Delete resource

	// Navigation
	Up              string // Move selection up
	Down            string // Move selection down
	JumpTop         string // Jump to top
	JumpBottom      string // Jump to bottom
	PageUp          string // Page up
	PageDown        string // Page down
	NamespaceFilter string // Namespace filter

	// Global
	Quit    string // Quit application
	Refresh string // Refresh data
	Back    string // Back/clear filter
	Help    string // Show help
}

// Default returns the default k9s-aligned keyboard configuration
func Default() *Keys {
	return &Keys{
		// Command Bar Activation
		FilterActivate:  "/",
		PaletteActivate: "ctrl+p",
		ResourceNav:     ":",

		// Context Switching
		PrevContext: "[",
		NextContext: "]",

		// Resource Operations
		Describe: "d",
		Edit:     "e",
		Logs:     "l",
		YAML:     "y",
		Delete:   "ctrl+x",

		// Navigation
		Up:              "k",
		Down:            "j",
		JumpTop:         "g",
		JumpBottom:      "G",
		PageUp:          "ctrl+b",
		PageDown:        "ctrl+f",
		NamespaceFilter: "n",

		// Global
		Quit:    "ctrl+c", // Note: :q command also works
		Refresh: "ctrl+r",
		Back:    "esc",
		Help:    "?",
	}
}

// GetKeys returns the current keyboard configuration
// Future: This will load from config file
func GetKeys() *Keys {
	return Default()
}
