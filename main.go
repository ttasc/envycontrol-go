package main

import (
	"fmt"
	"os"
)

func main() {
	opts := ParseArgs(os.Args)

	// --- 1. Nhóm hành động Chỉ Đọc (Query) ---
	if opts.Query {
		state := LoadState()
		fmt.Println(state.CurrentMode)
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

	// --- 2. Nhóm hành động Thay đổi Hệ thống (Yêu cầu Root) ---
	if opts.Switch != "" || opts.Reset || opts.ResetSddm || opts.StateCreate || opts.StateDelete {
		AssertRoot()

		// Các hành động tương tác với State File (Legacy)
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

		// Nhóm hành động Orchestrator
		if opts.Switch != "" {
			switchOpts := SwitchOptions{
				DisplayManager:   opts.Dm,
				ForceComp:        opts.ForceComp,
				CoolbitsValue:    opts.Coolbits,
				Rtd3Value:        opts.Rtd3,
				UseNvidiaCurrent: opts.UseNvidiaCurrent,
			}
			SwitchMode(opts.Switch, switchOpts)

		} else if opts.ResetSddm {
			ResetSddm()
		} else if opts.Reset {
			ResetSystem()
		}
	}
}
