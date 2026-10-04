package cmd

import (
	"encoding/json"
	"fmt"

	"github.com/luuuc/council/internal/adapter"
	"github.com/luuuc/council/internal/config"
	"github.com/luuuc/council/internal/expert"
	"github.com/spf13/cobra"
)

var (
	doctorJSON  bool
	doctorQuiet bool
)

func init() {
	rootCmd.AddCommand(doctorCmd)
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output as JSON")
	doctorCmd.Flags().BoolVar(&doctorQuiet, "quiet", false, "Only output if unhealthy")
}

// DoctorResult holds the complete health check results
type DoctorResult struct {
	Healthy     bool              `json:"healthy"`
	Checks      []CheckResult     `json:"checks"`
	SyncTargets []SyncCheckResult `json:"sync_targets,omitempty"`
}

// CheckResult represents a single health check
type CheckResult struct {
	Name    string   `json:"name"`
	Status  string   `json:"status"` // "ok", "error", "info"
	Message string   `json:"message,omitempty"`
	Details []string `json:"details,omitempty"`
}

// SyncCheckResult represents a sync target check
type SyncCheckResult struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	Status   string `json:"status"` // "ok", "info", "error"
	Message  string `json:"message,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check council health and diagnose issues",
	Long:  `Verifies your council setup and reports any issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDoctor()
	},
	SilenceUsage:  true, // Don't show usage on health check failure
	SilenceErrors: true, // We handle error output ourselves
}

func runDoctor() error {
	result := collectDoctorResults()

	// Handle output based on flags
	if doctorJSON {
		return outputDoctorJSON(result)
	}

	// For --quiet mode, only output if unhealthy
	if doctorQuiet && result.Healthy {
		return nil
	}

	outputDoctorText(result)

	// Return error if unhealthy (Priority 1: proper exit code)
	if !result.Healthy {
		return fmt.Errorf("health check failed")
	}
	return nil
}

// collectDoctorResults gathers all health check data into a struct
func collectDoctorResults() *DoctorResult {
	result := &DoctorResult{
		Healthy: true,
		Checks:  []CheckResult{},
	}

	// Check 1: .council/ directory exists
	if config.Exists() {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "directory",
			Status: "ok",
		})
	} else {
		result.Checks = append(result.Checks, CheckResult{
			Name:    "directory",
			Status:  "error",
			Message: "Run 'council init' to create it",
		})
		result.Healthy = false
	}

	// Check 2: Config file
	cfg, err := config.Load()
	if err == nil {
		result.Checks = append(result.Checks, CheckResult{
			Name:   "config",
			Status: "ok",
		})
	} else {
		result.Checks = append(result.Checks, CheckResult{
			Name:    "config",
			Status:  "error",
			Message: err.Error(),
		})
		result.Healthy = false
	}

	// Check 3: Experts loaded
	expertResult, err := expert.ListWithWarnings()
	if err != nil {
		result.Checks = append(result.Checks, CheckResult{
			Name:    "experts",
			Status:  "error",
			Message: err.Error(),
		})
		result.Healthy = false
	} else {
		count := len(expertResult.Experts)
		if count > 0 {
			details := make([]string, len(expertResult.Experts))
			for i, e := range expertResult.Experts {
				details[i] = fmt.Sprintf("%s (%s)", e.Name, e.ID)
			}
			result.Checks = append(result.Checks, CheckResult{
				Name:    "experts",
				Status:  "ok",
				Message: fmt.Sprintf("%d expert(s) loaded", count),
				Details: details,
			})
		} else {
			result.Checks = append(result.Checks, CheckResult{
				Name:    "experts",
				Status:  "error",
				Message: "Run 'council add <name>' to add experts",
			})
			result.Healthy = false
		}

		// Add warnings for any files that couldn't be loaded
		for _, w := range expertResult.Warnings {
			result.Checks = append(result.Checks, CheckResult{
				Name:    "expert_file",
				Status:  "error",
				Message: w,
			})
			result.Healthy = false
		}
	}

	// Check 4: Sync targets (use tool or targets from config)
	if cfg != nil {
		// Determine targets to check
		var targetsToCheck []string
		if len(cfg.Targets) > 0 {
			targetsToCheck = cfg.Targets
		} else if cfg.Tool != "" {
			targetsToCheck = []string{cfg.Tool}
		}

		for _, targetName := range targetsToCheck {
			a, ok := adapter.Get(targetName)
			if !ok {
				result.SyncTargets = append(result.SyncTargets, SyncCheckResult{
					Name:    targetName,
					Status:  "error",
					Message: "unknown target",
				})
				result.Healthy = false
				continue
			}

			paths := a.Paths()
			location := paths.Agents
			if location == "." {
				location = "AGENTS.md"
			}

			exists := a.Detect()
			if exists {
				result.SyncTargets = append(result.SyncTargets, SyncCheckResult{
					Name:     a.DisplayName(),
					Location: location,
					Status:   "ok",
				})
			} else {
				result.SyncTargets = append(result.SyncTargets, SyncCheckResult{
					Name:     a.DisplayName(),
					Location: location,
					Status:   "info",
					Message:  "not synced yet",
				})
			}
		}
	}

	return result
}

// outputDoctorJSON outputs the result as JSON
func outputDoctorJSON(result *DoctorResult) error {
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(data))

	// Return error if unhealthy (Priority 1: proper exit code)
	if !result.Healthy {
		return fmt.Errorf("health check failed")
	}
	return nil
}

// outputDoctorText outputs the result in human-readable format
func outputDoctorText(result *DoctorResult) {
	fmt.Println("Council Doctor")
	fmt.Println("==============")
	fmt.Println()

	// Print checks
	for _, check := range result.Checks {
		switch check.Status {
		case "ok":
			if check.Message != "" {
				printCheck(true, check.Message)
			} else {
				printCheck(true, checkNameToText(check.Name))
			}
			for _, d := range check.Details {
				fmt.Printf("       - %s\n", d)
			}
		case "error":
			printCheck(false, checkNameToText(check.Name))
			if check.Message != "" {
				fmt.Printf("     %s\n", check.Message)
			}
		case "info":
			printOptional(check.Message)
		}
	}

	// Print sync targets
	if len(result.SyncTargets) > 0 {
		fmt.Println()
		fmt.Println("Sync targets:")
		for _, st := range result.SyncTargets {
			switch st.Status {
			case "ok":
				printCheck(true, fmt.Sprintf("%s (%s)", st.Name, st.Location))
			case "info":
				printOptional(fmt.Sprintf("%s (%s) - %s", st.Name, st.Location, st.Message))
			case "error":
				printCheck(false, fmt.Sprintf("%s (%s)", st.Name, st.Message))
			}
		}
	}

	// Summary
	fmt.Println()
	if result.Healthy {
		fmt.Println("Your council is healthy!")
		fmt.Println()
		fmt.Println("Next steps:")
		fmt.Println("  council sync    Sync to AI tool configs")
		fmt.Println("  /council        Use in Claude Code or OpenCode")
	} else {
		fmt.Println("Some issues found. See above for details.")
	}
}

// checkNameToText converts check names to human-readable text
func checkNameToText(name string) string {
	switch name {
	case "directory":
		return ".council/ directory exists"
	case "config":
		return "config.yaml is valid"
	case "experts":
		return "Experts loaded"
	case "expert_file":
		return "Expert file"
	default:
		return name
	}
}

func printCheck(ok bool, msg string) {
	if ok {
		fmt.Printf("  [ok] %s\n", msg)
	} else {
		fmt.Printf("  [!!] %s\n", msg)
	}
}

func printOptional(msg string) {
	fmt.Printf("  [--] %s\n", msg)
}
