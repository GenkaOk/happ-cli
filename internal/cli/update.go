package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/update"
)

// updateTimeout bounds one full check/download/install sequence.
const updateTimeout = 5 * time.Minute

// autoUpdateTimeout bounds the best-effort update done by `connect
// --auto-update`, which must never delay the tunnel for long.
const autoUpdateTimeout = 15 * time.Second

// updateAPIBase overrides the GitHub API root. Tests point it at an httptest
// server; empty means https://api.github.com.
var updateAPIBase string

// updateOptions carries the `happ update` switches.
type updateOptions struct {
	repo  string
	check bool
	force bool
}

// defaultUpdateRepo resolves the repository to update from: HAPP_UPDATE_REPO
// overrides the compiled-in default.
func defaultUpdateRepo() string {
	if repo := strings.TrimSpace(os.Getenv("HAPP_UPDATE_REPO")); repo != "" {
		return repo
	}
	return update.DefaultRepo
}

func newUpdateCmd() *cobra.Command {
	var opts updateOptions
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update happ to the latest GitHub release",
		Long: "Check GitHub releases for a newer happ, verify the asset's SHA-256\n" +
			"checksum, and replace the running binary in place.\n\n" +
			"The previous binary is kept as <exe>.old. A running happ continues on the\n" +
			"old build until restarted, so restart the service after updating.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := context.WithTimeout(cmd.Context(), updateTimeout)
			defer cancel()
			return runUpdate(ctx, cmd.OutOrStdout(), appVersion, opts)
		},
	}
	cmd.Flags().BoolVar(&opts.check, "check", false, "only report current and latest versions; do not download or install")
	cmd.Flags().BoolVar(&opts.force, "force", false, "install even when the current version is equal, newer, or unknown (dev build)")
	cmd.Flags().StringVar(&opts.repo, "repo", defaultUpdateRepo(), "GitHub repository (owner/name) to update from (env HAPP_UPDATE_REPO)")
	return cmd
}

// runUpdate checks for a newer release and, unless opts.check is set, installs
// it. current is the version reported by the running binary.
func runUpdate(ctx context.Context, out io.Writer, current string, opts updateOptions) error {
	repo := strings.TrimSpace(opts.repo)
	if repo == "" {
		repo = defaultUpdateRepo()
	}
	client := &update.Client{Repo: repo, APIBase: updateAPIBase}

	rel, err := client.Latest(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "current: %s\n", current)
	fmt.Fprintf(out, "latest:  %s\n", rel.Tag)

	switch {
	case update.IsDevVersion(current):
		if opts.check {
			fmt.Fprintln(out, "note: development build; installing needs --force")
		} else if !opts.force {
			return fmt.Errorf("this build reports version %q, so happ cannot tell whether %s is newer; re-run 'happ update --force' to install it anyway", current, rel.Tag)
		}
	default:
		cmp, err := update.CompareVersions(current, rel.Tag)
		if err != nil {
			if !opts.force {
				return fmt.Errorf("%v; re-run with --force to install %s anyway", err, rel.Tag)
			}
		} else if cmp >= 0 {
			if cmp > 0 {
				fmt.Fprintf(out, "already up to date (current %s is newer than %s)\n", current, rel.Tag)
			} else {
				fmt.Fprintln(out, "already up to date")
			}
			return nil
		}
	}

	goos, goarch, goarm := update.CurrentPlatform()
	asset, err := client.FindAsset(rel, goos, goarch, goarm)
	if err != nil {
		return err
	}

	if opts.check {
		fmt.Fprintf(out, "update available: %s (%s)\n", rel.Tag, asset.Name)
		return nil
	}

	exe, err := update.CurrentExecutable()
	if err != nil {
		return err
	}
	replaced, err := update.Install(ctx, client, asset, exe)
	if err != nil {
		return err
	}
	if !replaced {
		fmt.Fprintln(out, "already up to date")
		return nil
	}
	fmt.Fprintf(out, "updated %s -> %s; restart to apply\n", current, rel.Tag)
	fmt.Fprintf(out, "backup: %s\n", exe+".old")
	return nil
}

// autoUpdate is the best-effort self-update run by `connect --auto-update`.
// It returns an error only so the caller can print it as a warning: the
// connection always proceeds. Dev builds are never auto-replaced.
func autoUpdate(ctx context.Context, current, repo string) error {
	if update.IsDevVersion(current) {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, autoUpdateTimeout)
	defer cancel()

	if strings.TrimSpace(repo) == "" {
		repo = defaultUpdateRepo()
	}
	client := &update.Client{Repo: repo, APIBase: updateAPIBase}

	rel, err := client.Latest(ctx)
	if err != nil {
		return err
	}
	cmp, err := update.CompareVersions(current, rel.Tag)
	if err != nil {
		return err
	}
	if cmp >= 0 {
		return nil
	}
	goos, goarch, goarm := update.CurrentPlatform()
	asset, err := client.FindAsset(rel, goos, goarch, goarm)
	if err != nil {
		return err
	}
	exe, err := update.CurrentExecutable()
	if err != nil {
		return err
	}
	if _, err := update.Install(ctx, client, asset, exe); err != nil {
		return err
	}
	fmt.Printf("updated to %s; this process keeps running the old build, restart to apply\n", rel.Tag)
	return nil
}
