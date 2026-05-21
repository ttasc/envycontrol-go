package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CliOptions represents all possible parsed command-line arguments.
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

// printHelp outputs the application usage text, preserving the exact wording
// of the original Python application.
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

// ParseArgs converts raw command-line arguments into a populated CliOptions struct.
// It implements a bespoke parser to handle optional values exactly like Python's argparse (nargs='?').
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
			val, consumed := parseOptionalString(args, i)
			if !consumed {
				LogError("argument -s/--switch: expected one argument")
				os.Exit(1)
			}
			opts.Switch = val
			i++

		case "--dm":
			val, consumed := parseOptionalString(args, i)
			if consumed {
				opts.Dm = val
				i++
			}

		case "--force-comp":
			opts.ForceComp = true

		case "--coolbits":
			val := 28 // Default if specified but no value provided
			parsed, consumed := parseOptionalInt(args, i)
			if consumed {
				val = parsed
				i++
			}
			opts.Coolbits = &val

		case "--rtd3":
			val := 2 // Default if specified but no value provided
			parsed, consumed := parseOptionalInt(args, i)
			if consumed {
				val = parsed
				i++
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

	validateOptions(&opts)
	return opts
}

// --- Helper Functions ---

// parseOptionalString checks if the next argument exists and does not start with a flag indicator ("-").
func parseOptionalString(args []string, currentIndex int) (string, bool) {
	if currentIndex+1 < len(args) && !strings.HasPrefix(args[currentIndex+1], "-") {
		return args[currentIndex+1], true
	}
	return "", false
}

// parseOptionalInt acts like parseOptionalString but parses the result into an integer.
func parseOptionalInt(args []string, currentIndex int) (int, bool) {
	if strVal, ok := parseOptionalString(args, currentIndex); ok {
		if parsedVal, err := strconv.Atoi(strVal); err == nil {
			return parsedVal, true
		}
	}
	return 0, false
}

// validateOptions enforces constraints on the user input before returning.
func validateOptions(opts *CliOptions) {
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
