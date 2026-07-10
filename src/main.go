package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	if err := runCLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func runCLI(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}
	command := args[0]
	switch command {
	case "--help", "-h", "help":
		printUsage(os.Stdout)
		return nil
	case "--list", "list":
		for _, name := range ScenarioNames() {
			fmt.Fprintln(os.Stdout, name)
		}
		return nil
	case "scenario":
		if len(args) < 2 {
			return fmt.Errorf("scenario name is required")
		}
		name := args[1]
		options := parseRunOptions(args[2:])
		options.ScenarioName = name
		options.Source = "scenario:" + name
		fixture, err := LoadScenario(name)
		if err != nil {
			return err
		}
		return executeFixture(fixture, options)
	case "run":
		if len(args) < 2 {
			return fmt.Errorf("fixture path is required")
		}
		path := args[1]
		options := parseRunOptions(args[2:])
		options.Source = path
		fixture, err := LoadFixtureFile(path)
		if err != nil {
			return err
		}
		return executeFixture(fixture, options)
	case "validate":
		if len(args) < 2 {
			return fmt.Errorf("fixture path is required")
		}
		fixture, err := LoadFixtureFile(args[1])
		if err != nil {
			return err
		}
		if _, err := NormalizeFixture(fixture); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, "ok")
		return nil
	case "dump-scenario":
		if len(args) < 2 {
			return fmt.Errorf("scenario name is required")
		}
		fixture, err := LoadScenario(args[1])
		if err != nil {
			return err
		}
		return EncodeFixture(os.Stdout, fixture)
	default:
		if strings.HasPrefix(command, "-") {
			return fmt.Errorf("unknown option %s", command)
		}
		return fmt.Errorf("unknown command %s", command)
	}
}

func parseRunOptions(args []string) RunOptions {
	options := RunOptions{}
	for _, arg := range args {
		switch arg {
		case "--events":
			options.IncludeEvents = true
		case "--strict":
			options.Strict = true
		}
	}
	return options
}

func executeFixture(fixture Fixture, options RunOptions) error {
	report, err := NewEngine().Run(fixture, options)
	if err != nil {
		return err
	}
	if err := ValidateReport(report); err != nil {
		return err
	}
	return WriteReportJSON(os.Stdout, report)
}

func printUsage(out *os.File) {
	fmt.Fprintln(out, "DeltaForgeDTL")
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Usage:")
	fmt.Fprintln(out, "  deltaforgedtl --list")
	fmt.Fprintln(out, "  deltaforgedtl scenario <name> [--events]")
	fmt.Fprintln(out, "  deltaforgedtl run <fixture.json> [--events]")
	fmt.Fprintln(out, "  deltaforgedtl validate <fixture.json>")
	fmt.Fprintln(out, "  deltaforgedtl dump-scenario <name>")
}
