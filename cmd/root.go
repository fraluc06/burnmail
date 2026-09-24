package cmd

import (
	"fmt"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"burnmail/internal/config"
)

var (
	cyan   = color.New(color.FgCyan).SprintFunc()
	green  = color.New(color.FgGreen).SprintFunc()
	yellow = color.New(color.FgYellow).SprintFunc()
	red    = color.New(color.FgRed).SprintFunc()
)

var (
	rootCmd = &cobra.Command{
		Use:   "burnmail",
		Short: "🔥 Burn through temporary emails straight from your terminal",
		Long:  `Burnmail is a CLI tool to quickly generate and manage disposable email addresses using mail.tm API.`,
		// Cobra registers the --version flag from this field at execution
		// time; config.Version is already stamped by the linker by then.
		// Handlers print user-facing messages and return errors; Execute prints
		// the returned error once, without the usage wall.
		Version:       config.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
)

var generateCmd = &cobra.Command{
	Use:     "g",
	Aliases: []string{"generate"},
	Short:   "Generate a new disposable email address",
	Args:    cobra.NoArgs,
	RunE:    generateEmail,
}

var messagesCmd = &cobra.Command{
	Use:     "m",
	Aliases: []string{"messages", "inbox"},
	Short:   "View inbox messages (interactive TUI)",
	Args:    cobra.NoArgs,
	RunE:    viewMessagesTUI,
}

var messagesListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List messages (classic view)",
	Args:    cobra.NoArgs,
	RunE:    viewMessages,
}

var deleteCmd = &cobra.Command{
	Use:     "d",
	Aliases: []string{"delete"},
	Short:   "Delete the current account",
	Args:    cobra.NoArgs,
	RunE:    deleteAccount,
}

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show account details",
	Args:  cobra.NoArgs,
	RunE:  showAccount,
}

var versionCmd = &cobra.Command{
	Use:     "v",
	Aliases: []string{"version"},
	Short:   "Show version information",
	Args:    cobra.NoArgs,
	Run: func(cmd *cobra.Command, _ []string) {
		// cmd.Printf writes to OutOrStdout() (capturable in tests) and
		// swallows write errors internally, satisfying errcheck.
		cmd.Printf("burnmail v%s\n", config.Version)
	},
}

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for burnmail.

To load completions:

Bash:
  $ source <(burnmail completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ burnmail completion bash > /etc/bash_completion.d/burnmail
  # macOS:
  $ burnmail completion bash > $(brew --prefix)/etc/bash_completion.d/burnmail

Zsh:
  $ source <(burnmail completion zsh)
  # To load completions for each session, execute once:
  $ burnmail completion zsh > "${fpath[1]}/_burnmail"

Fish:
  $ burnmail completion fish | source
  # To load completions for each session, execute once:
  $ burnmail completion fish > ~/.config/fish/completions/burnmail.fish
`,
	ValidArgs:             []string{"bash", "zsh", "fish"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	DisableFlagsInUseLine: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		out := cmd.OutOrStdout()
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(out)
		case "zsh":
			return cmd.Root().GenZshCompletion(out)
		case "fish":
			return cmd.Root().GenFishCompletion(out, true)
		}
		return nil
	},
}

var exportCmd = &cobra.Command{
	Use:     "export",
	Aliases: []string{"exp"},
	Short:   "Export all messages and account info to JSON file",
	Args:    cobra.NoArgs,
	RunE:    exportData,
}

func init() {
	rootCmd.AddCommand(generateCmd)
	rootCmd.AddCommand(messagesCmd)
	messagesCmd.AddCommand(messagesListCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(meCmd)
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(completionCmd)
	rootCmd.AddCommand(exportCmd)
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%s %v\n", red("✗"), err)
		os.Exit(1)
	}
}
