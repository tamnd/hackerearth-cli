package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) topicsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "topics",
		Short: "List known practice topics",
		Long: `List the built-in set of HackerEarth practice topics.

Each topic slug can be passed to 'he problems --topic <slug>' to fetch
the problems for that topic.`,
		Example: `  he topics
  he topics -o json
  he topics --fields name,slug`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			topics := a.client.Topics()
			return a.renderOrEmpty(topics, len(topics))
		},
	}
}
