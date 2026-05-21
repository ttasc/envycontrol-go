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

	if opts.StateQuery {
		if content, err := os.ReadFile(StateFilePath); err == nil {
			fmt.Print(string(content))
		} else {
			fmt.Printf("ERROR: Could not read %s\n", StateFilePath)
		}
		return
	}

	// Exit early if no mutating flags were passed
	if opts.Switch == "" && !opts.Reset && !opts.ResetSddm && !opts.StateCreate && !opts.StateDelete {
		return
	}

	// --- Phase 2: Mutating Actions (Root Required) ---

	AssertRoot()

	// Handle Legacy State File overrides
	if opts.StateCreate {
		state := RebuildState()
		if state.CurrentMode != "hybrid" {
			LogError("--cache-create requires that the system be in the hybrid Optimus mode")
			os.Exit(1)
		}
		SaveState(state)
		LogInfo("State file forcefully created")
		return
	}

	if opts.StateDelete {
		os.Remove(StateFilePath)
		LogInfo("Removed state file")
		return
	}

	// Handle Primary Orchestrator Routing
	if opts.Switch != "" {
		switchOpts := SwitchOptions{
			DisplayManager:   opts.Dm,
			ForceComp:        opts.ForceComp,
			CoolbitsValue:    opts.Coolbits,
			Rtd3Value:        opts.Rtd3,
			UseNvidiaCurrent: opts.UseNvidiaCurrent,
		}
		SwitchMode(opts.Switch, switchOpts)
		return
	}

	if opts.ResetSddm {
		ResetSddm()
		return
	}

	if opts.Reset {
		ResetSystem()
		return
	}
}
