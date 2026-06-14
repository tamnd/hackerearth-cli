package cli

import (
	"github.com/spf13/cobra"
	"github.com/tamnd/hackerearth-cli/hackerearth"
)

func (a *App) challengesCmd() *cobra.Command {
	var kind string
	cmd := &cobra.Command{
		Use:   "challenges",
		Short: "List active challenges and hackathons",
		Long: `List active competitive challenges and hackathons from HackerEarth.

The --kind flag selects which listing page to fetch:
  competitive  Hiring challenges and competitive programming contests
  hackathon    Open hackathons
  all          Both pages combined`,
		Example: `  he challenges
  he challenges --kind hackathon
  he challenges --kind all -n 20
  he challenges -o json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			n := a.effectiveLimit(20)
			k := hackerearth.ChallengeKind(kind)
			a.progressf("fetching %s challenges...", kind)
			chal, err := a.client.Challenges(cmd.Context(), k, n)
			if err != nil {
				return mapFetchErr(err)
			}
			return a.renderOrEmpty(chal, len(chal))
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "competitive", "challenge kind: competitive|hackathon|all")
	return cmd
}
