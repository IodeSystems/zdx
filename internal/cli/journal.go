package cli

import (
	"fmt"
	"net/url"
	"time"

	"github.com/spf13/cobra"
)

type journalEntry struct {
	Date          string `json:"date"`
	Baseline      bool   `json:"baseline"`
	Tldr          string `json:"tldr"`
	Assessment    string `json:"assessment"`
	Concerns      string `json:"concerns"`
	Next          string `json:"next"`
	ChangelogJSON string `json:"changelog_json"`
	StateJSON     string `json:"state_json"`
}

func JournalCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "journal", Short: "Owner/tech journal check-ins"}
	cmd.AddCommand(
		journalCheckinCmd(),
		journalShowCmd(),
		journalStateCmd(),
	)
	return cmd
}

func journalCheckinCmd() *cobra.Command {
	var role, tldr, assessment, concerns, next, date string
	cmd := &cobra.Command{
		Use:   "checkin",
		Short: "Record a journal check-in",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			var resp struct{ OK bool }
			if err := c.post("/api/dx/journal/checkin", map[string]any{
				"slug":       c.SlugOrDie(),
				"role":       role,
				"date":       date,
				"tldr":       tldr,
				"assessment": assessment,
				"concerns":   concerns,
				"next":       next,
			}, &resp); err != nil {
				return err
			}
			fmt.Println("ok")
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "journal role (owner/tech)")
	cmd.Flags().StringVar(&tldr, "tldr", "", "one-line summary")
	cmd.Flags().StringVar(&assessment, "assessment", "", "assessment text")
	cmd.Flags().StringVar(&concerns, "concerns", "", "concerns text")
	cmd.Flags().StringVar(&next, "next", "", "next steps text")
	cmd.Flags().StringVar(&date, "date", "", "entry date (default: today)")
	return cmd
}

func journalShowCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "List journal entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"role": {role},
			}
			var resp struct {
				Entries []journalEntry `json:"entries"`
			}
			if err := c.get("/api/dx/journal/show", params, &resp); err != nil {
				return err
			}
			if len(resp.Entries) == 0 {
				fmt.Println("no entries")
				return nil
			}
			for _, e := range resp.Entries {
				tag := ""
				if e.Baseline {
					tag = " [baseline]"
				}
				fmt.Printf("%s%s  %s\n", e.Date, tag, e.Tldr)
				if e.Assessment != "" {
					fmt.Printf("  assessment: %s\n", e.Assessment)
				}
				if e.Concerns != "" {
					fmt.Printf("  concerns:   %s\n", e.Concerns)
				}
				if e.Next != "" {
					fmt.Printf("  next:       %s\n", e.Next)
				}
				fmt.Println()
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "journal role (owner/tech)")
	return cmd
}

func journalStateCmd() *cobra.Command {
	var role string
	cmd := &cobra.Command{
		Use:   "state",
		Short: "Show latest journal state snapshot",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := mustClient()
			params := url.Values{
				"slug": {c.SlugOrDie()},
				"role": {role},
			}
			var resp struct {
				StateJSON string `json:"state_json"`
			}
			if err := c.get("/api/dx/journal/state", params, &resp); err != nil {
				return err
			}
			if resp.StateJSON == "" {
				fmt.Println("no state")
				return nil
			}
			fmt.Println(resp.StateJSON)
			return nil
		},
	}
	cmd.Flags().StringVar(&role, "role", "owner", "journal role (owner/tech)")
	return cmd
}
