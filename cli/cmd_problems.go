package cli

import (
	"github.com/spf13/cobra"
)

func (a *App) problemsCmd() *cobra.Command {
	var topic string
	cmd := &cobra.Command{
		Use:   "problems",
		Short: "List practice problems for a topic",
		Long: `List practice problems from a HackerEarth topic page.

The topic argument is the path used in the URL, e.g.:
  algorithms/sorting/bubble-sort
  algorithms/graphs/breadth-first-search
  data-structures/trees/binary-search-tree

Run 'he topics' to see the built-in list of topics.`,
		Example: `  he problems --topic algorithms/sorting/bubble-sort
  he problems --topic algorithms/graphs/breadth-first-search -n 10
  he problems --topic data-structures/stacks-and-queues/basics-of-stacks -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			a.progressf("fetching problems for %q...", topic)
			probs, err := a.client.Problems(cmd.Context(), topic, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(probs, len(probs))
		},
	}
	cmd.Flags().StringVarP(&topic, "topic", "t", "algorithms/sorting/bubble-sort",
		"practice topic path (e.g. algorithms/sorting/bubble-sort)")
	return cmd
}
