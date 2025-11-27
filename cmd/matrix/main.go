package main

import (
	"fmt"
	"os"
)

func main() {
	// Simple command routing without cobra for now
	if len(os.Args) < 2 {
		fmt.Println("matrix v0.0.1")
		fmt.Println("")
		fmt.Println("Intelligence tools for the Claude Code identity system.")
		fmt.Println("Analyzes and surfaces patterns across ~/.claude/ram/")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  garden-paths    Discover connections in the matrix garden")
		fmt.Println("  garden-seeds    Create well-structured RAM files from templates")
		fmt.Println("  tension-map     Surface conflicts and tensions across RAM")
		fmt.Println("  velocity        Track task completion velocity by identity")
		fmt.Println("  recon           Scan codebases and generate intelligence reports")
		fmt.Println("  incident-trace  Extract structured post-mortem data from debugging sessions")
		fmt.Println("  crossroads      Capture decision points and paths not taken")
		fmt.Println("  balance-checker Detect drift between design docs and implementation")
		fmt.Println("  breach-points   Audit for security vulnerabilities and exposures")
		fmt.Println("  vault-keys      Map authentication, authorization, and security boundaries")
		fmt.Println("  flight-check    Track deployment state across identity work")
		fmt.Println("  knowledge-gaps  Find unanswered questions and missing documentation")
		fmt.Println("  contract-ledger Track data flows and dependencies between identities")
		fmt.Println("  schema-catalog  Track database schemas across projects")
		fmt.Println("  phase-shift     Track cross-language compatibility and migration patterns")
		fmt.Println("  platform-map    Scan for cross-platform compatibility markers")
		fmt.Println("  verdict         Track test results and performance metrics")
		fmt.Println("  question        Surface hidden assumptions behind documented work")
		fmt.Println("  debt-ledger     Track technical debt markers and generate remediation tasks")
		fmt.Println("  friction-points Track UX review queue and feedback")
		fmt.Println("  spec-verify     Verify implementations against formal specifications")
		fmt.Println("  alt-routes      Accessibility audit and alternative output formats")
		fmt.Println("  data-harvest    Scan RAM for data patterns to build better fixtures")
		fmt.Println("  dependency-map  Map installed toolchains and package dependencies")
		fmt.Println("  diff-paths      Compare two implementations and extract architectural tradeoffs")
		return
	}

	cmd := os.Args[1]

	switch cmd {
	case "garden-paths":
		if err := runGardenPaths(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "garden-seeds":
		if err := runGardenSeeds(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "tension-map":
		if err := runTensionMap(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "velocity":
		if err := runVelocity(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "recon":
		if err := runRecon(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "incident-trace":
		if err := runIncidentTrace(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "crossroads":
		if err := runCrossroads(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "balance-checker":
		if err := runBalanceChecker(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "breach-points":
		if err := runBreachPoints(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "vault-keys":
		if err := runVaultKeys(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "flight-check":
		if err := runFlightCheck(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "knowledge-gaps":
		if err := runKnowledgeGaps(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "contract-ledger":
		if err := runContractLedger(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "schema-catalog":
		if err := runSchemaCatalog(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "phase-shift":
		if err := runPhaseShift(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "platform-map":
		if err := runPlatformMap(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "verdict":
		if err := runVerdict(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "question":
		if err := runQuestion(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "debt-ledger":
		if err := runDebtLedger(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "friction-points":
		if err := runFrictionPoints(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "spec-verify":
		if err := runSpecVerify(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "alt-routes":
		if err := runAltRoutes(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "data-harvest":
		if err := runDataHarvest(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "dependency-map":
		if err := runDependencyMap(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "diff-paths":
		if err := runDiffPaths(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	case "--help", "-h", "help":
		fmt.Println("matrix v0.0.1")
		fmt.Println("")
		fmt.Println("Intelligence tools for the Claude Code identity system.")
		fmt.Println("Analyzes and surfaces patterns across ~/.claude/ram/")
		fmt.Println("")
		fmt.Println("Commands:")
		fmt.Println("  garden-paths    Discover connections in the matrix garden")
		fmt.Println("  garden-seeds    Create well-structured RAM files from templates")
		fmt.Println("  tension-map     Surface conflicts and tensions across RAM")
		fmt.Println("  velocity        Track task completion velocity by identity")
		fmt.Println("  recon           Scan codebases and generate intelligence reports")
		fmt.Println("  incident-trace  Extract structured post-mortem data from debugging sessions")
		fmt.Println("  crossroads      Capture decision points and paths not taken")
		fmt.Println("  balance-checker Detect drift between design docs and implementation")
		fmt.Println("  breach-points   Audit for security vulnerabilities and exposures")
		fmt.Println("  vault-keys      Map authentication, authorization, and security boundaries")
		fmt.Println("  flight-check    Track deployment state across identity work")
		fmt.Println("  knowledge-gaps  Find unanswered questions and missing documentation")
		fmt.Println("  contract-ledger Track data flows and dependencies between identities")
		fmt.Println("  schema-catalog  Track database schemas across projects")
		fmt.Println("  phase-shift     Track cross-language compatibility and migration patterns")
		fmt.Println("  platform-map    Scan for cross-platform compatibility markers")
		fmt.Println("  verdict         Track test results and performance metrics")
		fmt.Println("  question        Surface hidden assumptions behind documented work")
		fmt.Println("  debt-ledger     Track technical debt markers and generate remediation tasks")
		fmt.Println("  friction-points Track UX review queue and feedback")
		fmt.Println("  spec-verify     Verify implementations against formal specifications")
		fmt.Println("  alt-routes      Accessibility audit and alternative output formats")
		fmt.Println("  data-harvest    Scan RAM for data patterns to build better fixtures")
		fmt.Println("  dependency-map  Map installed toolchains and package dependencies")
		fmt.Println("  diff-paths      Compare two implementations and extract architectural tradeoffs")
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		fmt.Println("Run 'matrix help' for usage")
		os.Exit(1)
	}
}
