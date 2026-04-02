package main

import (
	"fmt"
	"os"

	"github.com/vmkteam/pcurl/internal/keyring"
	"github.com/vmkteam/pcurl/internal/pcurl"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var version = "dev"

// subcommands that should be routed through cobra.
var subcommands = map[string]bool{
	"add": true, "show": true, "list": true, "ls": true,
	"delete": true, "rm": true, "edit": true,
	"install": true, "uninstall": true, "help": true,
}

func main() {
	ex := &pcurl.Executer{
		CM:      pcurl.NewConfigManager(),
		Keyring: &keyring.OS{},
	}
	interactive := term.IsTerminal(int(os.Stdin.Fd()))

	// If the first arg is not a subcommand, bypass cobra entirely
	// so that curl flags (-H, -X, --data-raw, etc.) are not eaten.
	if len(os.Args) > 1 && !subcommands[os.Args[1]] && !isHelpOrVersion(os.Args[1]) {
		code, err := ex.Exec(os.Args[1:], os.Stderr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "pcurl: %v\n", err)
			os.Exit(1)
		}
		os.Exit(code)
	}

	root := &cobra.Command{
		Use:   "pcurl",
		Short: "Private curl — hide auth headers from AI agents",
		Long: fmt.Sprintf(`pcurl is a drop-in curl wrapper that keeps your secrets out of
AI agent context, shell history, and process lists.

Use pcurl @profile instead of curl for authenticated requests.
Without @profile, pcurl passes through to curl directly.

Config: %s`, ex.CM.Path()),
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newAddCmd(ex, interactive),
		newShowCmd(ex),
		newDeleteCmd(ex),
		newEditCmd(ex),
		newInstallCmd(ex),
		newUninstallCmd(ex),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "pcurl: %v\n", err)
		os.Exit(1)
	}
}

func isHelpOrVersion(arg string) bool {
	return arg == "--help" || arg == "-h" || arg == "--version"
}

func newAddCmd(ex *pcurl.Executer, interactive bool) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add [curl args...]",
		Short: "Add a profile from a curl command",
		Long: `Parses a curl command, extracts secrets, and creates a profile.

Browser noise headers (sec-ch-*, sec-fetch-*, sentry-trace, baggage) are removed.
Non-secret headers are shown in an interactive picker (optional headers deselected).
Secret headers are stored in the OS keychain by default.
Cookies are shown in an interactive picker with secret cookies pre-selected.

pcurl flags:
      --force         skip all prompts (keychain default, all headers/cookies selected)
      --name string   custom profile name (default: hostname)
      --raw           keep browser headers, select all headers/cookies, skip pickers
      --test          run test request after adding`,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, curlArgs, help := parseAddFlags(args)
			if help {
				return cmd.Help()
			}
			return ex.Add(curlArgs, opts, os.Stdout, os.Stdin, interactive)
		},
	}
	return cmd
}

func parseAddFlags(args []string) (pcurl.AddOptions, []string, bool) {
	var opts pcurl.AddOptions
	var curlArgs []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--force":
			opts.Force = true
		case "--test":
			opts.Test = true
		case "--raw":
			opts.Raw = true
		case "--name":
			if i+1 < len(args) {
				i++
				opts.Name = args[i]
			}
		case "-h", "--help":
			return opts, nil, true
		default:
			curlArgs = append(curlArgs, args[i])
		}
	}
	return opts, curlArgs, false
}

func newShowCmd(ex *pcurl.Executer) *cobra.Command {
	return &cobra.Command{
		Use:     "show [profile]",
		Aliases: []string{"list", "ls"},
		Short:   "List all profiles, or show details for one",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return ex.List(os.Stdout)
			}
			return ex.Show(args[0], os.Stdout)
		},
	}
}

func newDeleteCmd(ex *pcurl.Executer) *cobra.Command {
	return &cobra.Command{
		Use:     "delete <profile>",
		Aliases: []string{"rm"},
		Short:   "Delete a profile and its keychain entries",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return ex.Delete(args[0], os.Stdout)
		},
	}
}

func newEditCmd(ex *pcurl.Executer) *cobra.Command {
	return &cobra.Command{
		Use:   "edit",
		Short: "Open profiles.toml in $EDITOR",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return ex.Edit()
		},
	}
}

func newInstallCmd(ex *pcurl.Executer) *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Set up AI agent integration (CLAUDE.md, Cursor, Windsurf)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ex.Install()
		},
	}
}

func newUninstallCmd(ex *pcurl.Executer) *cobra.Command {
	var purge bool
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove AI agent integration",
		RunE: func(cmd *cobra.Command, args []string) error {
			return ex.Uninstall(purge)
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "also delete profiles and config directory")
	return cmd
}
