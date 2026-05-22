package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CliOptions represents all possible parsed command-line arguments.
type CliOptions struct {
	Query  bool
	Switch string
	Rtd3   *int
	Reset  bool
}

// printHelp outputs the application usage text.
func printHelp() {
	helpText := `usage: envycontrol [-h] [-v] [-q] [-s MODE] [--rtd3 [VALUE]] [--reset] [--verbose]

A minimalist tool for GPU power management on Nvidia Optimus systems.

options:
  -h, --help        Show this help message and exit
  -v, --version     Output the current version
  -q, --query       Query the current graphics mode
  -s, --switch      Switch the graphics mode. Available choices: integrated, hybrid, nvidia
  --rtd3 [VALUE]    Setup PCI-Express Runtime D3 Power Management on Hybrid mode.
                    Available choices:
                      0 = Disabled: No power management.
                      1 = Coarse-grained: Turns off GPU when idle, but keeps memory powered.
                      2 = Fine-grained: Completely turns off GPU and memory when idle. (Default)
                      3 = Fine-grained (Ampere+): Specific to RTX 30-series architectures and newer.
  --reset           Revert all changes and restore system defaults
  --verbose         Enable debug logs and system command outputs

environment variables:
  NV_MODULE         Override the target Nvidia kernel module name.
                    Useful for distros using non-standard names.
                    Default: "nvidia" (e.g., export NV_MODULE="nvidia-current")
`
	fmt.Print(helpText)
}

// ParseArgs converts raw command-line arguments into a populated CliOptions struct.
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

		case "--rtd3":
			val := 2 // Default if flag is provided without a specific number
			parsed, consumed := parseOptionalInt(args, i)
			if consumed {
				val = parsed
				i++
			}
			opts.Rtd3 = &val

		case "--reset":
			opts.Reset = true

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
