package register

import (
	_ "terminal-client/internal/plugins/info"
	_ "terminal-client/internal/plugins/termconfig"
)

// Register ensures all statically linked plugins execute their init logic.
func Register() {}
