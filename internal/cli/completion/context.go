package completion

import (
	"context"
	"maps"
	"slices"
	"strings"

	"github.com/psviderski/uncloud/internal/cli"
	"github.com/spf13/cobra"
)

func Contexts(ctx context.Context, uncli *cli.CLI, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	// There are no contexts to complete when the CLI uses a direct machine connection (--connect) without a config.
	if uncli.Config == nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	contexts := slices.Sorted(maps.Keys(uncli.Config.Contexts))

	names := []cobra.Completion{}
	for _, context := range contexts {
		if slices.Contains(args, context) {
			continue
		}
		if strings.HasPrefix(context, toComplete) {
			names = append(names, context)
		}
	}

	return names, cobra.ShellCompDirectiveNoFileComp
}
