package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// CliOptions represents the parsed configuration derived from command-line arguments.
type CliOptions struct {
	Query  bool
	Switch string
	Rtd3   *int
	Reset  bool
	Update bool // Indicates a request to auto-update the tool
}

// printHelp prints the application's usage instructions, available options,
// and environment variables to standard output.
func printHelp() {
	helpText := `usage: envycontrol [-h] [-v] [-u] [-q] [-s MODE] [--rtd3 [VALUE]] [--reset] [--verbose]

A minimalist tool for GPU power management on Nvidia Optimus systems.

options:
  -h, --help        Show this help message and exit
  -v, --version     Output the current version
  -u, --update      Update the tool to the latest version
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

// ParseArgs parses the raw command-line arguments and returns a populated CliOptions.
// It acts as the primary entry point for CLI evaluation. If an error occurs, or if
// help/version is requested, it prints the corresponding output and terminates
// the program via os.Exit.
func ParseArgs(args []string) CliOptions {
	if len(args) == 1 {
		printHelp()
		os.Exit(1)
	}

	opts, err := parseArgsInternal(args)
	if err != nil {
		if err.Error() == "help requested" {
			printHelp()
			os.Exit(0)
		} else if err.Error() == "version requested" {
			fmt.Println(Version) // Version is defined in constants.go
			os.Exit(0)
		}
		LogError(err.Error())
		os.Exit(1)
	}
	return opts
}

// --- Helper Functions ---

// parseArgsInternal encapsulates the core logic for parsing arguments.
// It is separated from ParseArgs to allow for unit testing without triggering os.Exit.
func parseArgsInternal(args []string) (CliOptions, error) {
	var opts CliOptions

	// Return a help request if no arguments are provided (only the binary name)
	if len(args) <= 1 {
		return opts, fmt.Errorf("help requested")
	}

	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "-h", "--help":
			return opts, fmt.Errorf("help requested")

		case "-v", "--version":
			return opts, fmt.Errorf("version requested")

		case "-u", "--update":
			opts.Update = true

		case "-q", "--query":
			opts.Query = true

		case "-s", "--switch":
			val, consumed := parseOptionalString(args, i)
			if !consumed {
				return opts, fmt.Errorf("argument -s/--switch: expected one argument")
			}
			opts.Switch = val
			i++

		case "--rtd3":
			val := 2 // Default value if the --rtd3 flag is provided without a specific number
			parsed, consumed := parseOptionalInt(args, i)
			if consumed {
				val = parsed
				i++
			}
			opts.Rtd3 = &val

		case "--reset":
			opts.Reset = true

		case "--verbose":
			Verbose = true // Verbose is a global variable declared in sysutil.go

		default:
			return opts, fmt.Errorf("unrecognized arguments: %s", arg)
		}
	}

	// Validate the constraints on the user input
	err := validateOptionsInternal(&opts)
	return opts, err
}

// parseOptionalString extracts the next string argument from the list if it exists
// and does not start with a flag indicator ("-"). It returns the extracted string
// and a boolean indicating whether the argument was successfully consumed.
func parseOptionalString(args []string, currentIndex int) (string, bool) {
	if currentIndex+1 < len(args) && !strings.HasPrefix(args[currentIndex+1], "-") {
		return args[currentIndex+1], true
	}
	return "", false
}

// parseOptionalInt extracts the next argument from the list, attempting to parse it
// as an integer. It returns the parsed integer and a boolean indicating whether
// the argument was successfully consumed.
func parseOptionalInt(args []string, currentIndex int) (int, bool) {
	if strVal, ok := parseOptionalString(args, currentIndex); ok {
		if parsedVal, err := strconv.Atoi(strVal); err == nil {
			return parsedVal, true
		}
	}
	return 0, false
}

// validateOptionsInternal verifies that the parsed CLI options contain valid values.
// It returns an error if unsupported modes or invalid RTD3 values are provided.
func validateOptionsInternal(opts *CliOptions) error {
	if opts.Switch != "" && !containsStr(SupportedModes, opts.Switch) {
		return fmt.Errorf("argument -s/--switch: invalid choice: '%s'", opts.Switch)
	}
	if opts.Rtd3 != nil && !containsInt(Rtd3Modes, *opts.Rtd3) {
		return fmt.Errorf("argument --rtd3: invalid choice: %d", *opts.Rtd3)
	}
	return nil
}

// containsStr reports whether the slice contains the specified string.
func containsStr(slice []string, val string) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}

// containsInt reports whether the slice contains the specified integer.
func containsInt(slice []int, val int) bool {
	for _, item := range slice {
		if item == val {
			return true
		}
	}
	return false
}
