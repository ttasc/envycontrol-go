package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type CliOptions struct {
	Query            bool
	Switch           string
	Dm               string
	ForceComp        bool
	Coolbits         *int
	Rtd3             *int
	UseNvidiaCurrent bool
	ResetSddm        bool
	Reset            bool
	StateCreate      bool
	StateDelete      bool
	StateQuery       bool
}

func printHelp() {
	helpText := `usage: envycontrol [-h] [-v] [-q] [-s MODE] [--dm DISPLAY_MANAGER] [--force-comp] [--coolbits [VALUE]] [--rtd3 [VALUE]] [--use-nvidia-current] [--reset-sddm] [--reset] [--verbose]

options:
  -h, --help            show this help message and exit
  -v, --version         Output the current version
  -q, --query           Query the current graphics mode
  -s MODE, --switch MODE
                        Switch the graphics mode. Available choices: integrated, hybrid, nvidia
  --dm DISPLAY_MANAGER  Manually specify your Display Manager for Nvidia mode. Available choices: gdm, gdm3, sddm, lightdm
  --force-comp          Enable ForceCompositionPipeline on Nvidia mode
  --coolbits [VALUE]    Enable Coolbits on Nvidia mode. Default if specified: 28
  --rtd3 [VALUE]        Setup PCI-Express Runtime D3 (RTD3) Power Management on Hybrid mode. Available choices: 0, 1, 2, 3. Default if specified: 2
  --use-nvidia-current  Use nvidia-current instead of nvidia for kernel modules
  --reset-sddm          Restore default Xsetup file
  --reset               Revert changes made by EnvyControl
  --verbose             Enable verbose mode

Legacy options:
  --cache-create        Force create internal state file (hybrid mode only)
  --cache-delete        Delete internal state file
  --cache-query         Show internal state file
`
	fmt.Print(helpText)
}

func containsStr(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func containsInt(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

func ParseArgs(args []string) CliOptions {
	if len(args) == 1 {
		printHelp()
		os.Exit(1)
	}

	var opts CliOptions

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			printHelp()
			os.Exit(0)
		case "-v", "--version":
			fmt.Println(Version)
			os.Exit(0)
		case "-q", "--query":
			opts.Query = true
		case "-s", "--switch":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				opts.Switch = args[i+1]
				i++
			} else {
				LogError("argument -s/--switch: expected one argument")
				os.Exit(1)
			}
		case "--dm":
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				opts.Dm = args[i+1]
				i++
			}
		case "--force-comp":
			opts.ForceComp = true
		case "--coolbits":
			val := 28
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				if parsedVal, err := strconv.Atoi(args[i+1]); err == nil {
					val = parsedVal
					i++
				}
			}
			opts.Coolbits = &val
		case "--rtd3":
			val := 2
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				if parsedVal, err := strconv.Atoi(args[i+1]); err == nil {
					val = parsedVal
					i++
				}
			}
			opts.Rtd3 = &val
		case "--use-nvidia-current":
			opts.UseNvidiaCurrent = true
		case "--reset-sddm":
			opts.ResetSddm = true
		case "--reset":
			opts.Reset = true
		case "--cache-create":
			opts.StateCreate = true
		case "--cache-delete":
			opts.StateDelete = true
		case "--cache-query":
			opts.StateQuery = true
		case "--verbose":
			Verbose = true
		default:
			LogError("unrecognized arguments: %s", arg)
			os.Exit(1)
		}
	}

	if opts.Switch != "" && !containsStr(SupportedModes, opts.Switch) {
		LogError("argument -s/--switch: invalid choice: '%s'", opts.Switch)
		os.Exit(1)
	}
	if opts.Dm != "" && !containsStr(SupportedDisplayManagers, opts.Dm) {
		LogError("argument --dm: invalid choice: '%s'", opts.Dm)
		os.Exit(1)
	}
	if opts.Rtd3 != nil && !containsInt(Rtd3Modes, *opts.Rtd3) {
		LogError("argument --rtd3: invalid choice: %d", *opts.Rtd3)
		os.Exit(1)
	}

	return opts
}
