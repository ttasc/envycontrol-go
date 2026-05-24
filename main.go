package main

import (
	"fmt"
	"os"
)

// main initializes the CLI processing and routes the execution
// to either read-only data querying functions or system-mutating actions.
func main() {
	opts := ParseArgs(os.Args)

	// --- Phase 1: Read-Only Actions (No Root Required) ---
	if opts.Query {
		liveMode := GuessCurrentMode()
		fmt.Println(liveMode)
		return
	}

	// Exit early if no mutating flags were passed
	if opts.Switch == "" && !opts.Reset && !opts.Update {
		return
	}

	// --- Phase 2: Mutating Actions (Root Required) ---
	AssertRoot()

	if opts.Update {
		UpdateEnvyControl()
		return
	}

	// Capture the environment variable for custom kernel module names
	nvModule := os.Getenv("NV_MODULE")
	if nvModule == "" {
		nvModule = "nvidia" // Default to standard module name
	}

	// Handle Primary Orchestrator Routing
	if opts.Switch != "" {
		switchOpts := SwitchOptions{
			NvidiaModule: nvModule,
			Rtd3Value:    opts.Rtd3,
		}
		SwitchMode(opts.Switch, switchOpts)
		return
	}

	if opts.Reset {
		ResetSystem()
		return
	}
}
