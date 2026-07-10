package cli

import (
	"github.com/spf13/cobra"

	"github.com/aimuzov/happ-cli/internal/link"
)

// serverTags turns servers into shell completions, using the protocol as the
// description shown next to each tag.
func serverTags(servers []*link.Server) []cobra.Completion {
	out := make([]cobra.Completion, 0, len(servers))
	for _, s := range servers {
		out = append(out, cobra.CompletionWithDesc(s.Tag, s.Protocol))
	}
	return out
}

// subNames turns stored subscriptions into shell completions, annotating each
// with its title and marking the active one.
func subNames(st Store) []cobra.Completion {
	subs := st.Subscriptions()
	out := make([]cobra.Completion, 0, len(subs))
	for _, s := range subs {
		desc := s.Title
		if s.Name == st.Active() {
			if desc != "" {
				desc += " "
			}
			desc += "(active)"
		}
		if desc == "" {
			out = append(out, cobra.Completion(s.Name))
			continue
		}
		out = append(out, cobra.CompletionWithDesc(s.Name, desc))
	}
	return out
}

// completeSubNames returns a completion function for subscription-name arguments.
func completeSubNames(deps *Deps) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return subNames(deps.Store), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeSubFlag returns a completion function for the --sub flag.
func completeSubFlag(deps *Deps) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		return subNames(deps.Store), cobra.ShellCompDirectiveNoFileComp
	}
}

// completeServerSelector returns a completion function for connect server selectors.
func completeServerSelector(deps *Deps) func(*cobra.Command, []string, string) ([]cobra.Completion, cobra.ShellCompDirective) {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if len(args) != 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		name, _ := cmd.Flags().GetString("sub")
		sub, err := resolveSub(deps.Store, name)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return serverTags(sub.Servers()), cobra.ShellCompDirectiveNoFileComp
	}
}
