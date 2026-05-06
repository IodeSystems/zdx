package streams

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/iodesystems/zdx-go/internal/cli"
	"github.com/iodesystems/zdx-go/internal/dxclient"
)

// proposalHandler is the StreamHandler for target_type="proposal" — the
// flow that lived in internal/cli/project/proposal_evaluate.go before
// IS-619-T2. FetchBody returns the canonical body the classifier
// evaluates against; ApplyAddressed posts a revision event tagged with
// the addressing event ID.
type proposalHandler struct{}

func (proposalHandler) TargetType() string { return "proposal" }

func (proposalHandler) FetchBody(ctx context.Context, c *cli.Client, slug, targetID string) (string, error) {
	pid, err := parseProposalTargetID(targetID)
	if err != nil {
		return "", err
	}
	resp, err := c.ShowProposalWithResponse(ctx, pid, &dxclient.ShowProposalParams{Slug: slug})
	if err != nil {
		return "", fmt.Errorf("show proposal: %w", err)
	}
	if err := c.CheckStatus(resp.StatusCode(), resp.Body); err != nil {
		return "", err
	}
	if resp.JSON200 == nil {
		return "", fmt.Errorf("empty proposal response")
	}
	return resp.JSON200.Proposal.Body, nil
}

func (proposalHandler) ApplyAddressed(ctx context.Context, c *cli.Client, slug, targetID string, addressingEventID int64, revisedBody string) error {
	detail, _ := json.Marshal(map[string]any{
		"body":                revisedBody,
		"addressing_event_id": addressingEventID,
	})
	eventType := "revision"
	authorKind := "agent"
	resp, err := c.AddEventCommentWithResponse(ctx, dxclient.AddEventCommentRequest{
		Slug:       slug,
		TargetType: "proposal",
		TargetId:   targetID,
		EventType:  &eventType,
		AuthorKind: &authorKind,
		DetailJson: json.RawMessage(detail),
	})
	if err != nil {
		return fmt.Errorf("post revision: %w", err)
	}
	return c.CheckStatus(resp.StatusCode(), resp.Body)
}

// parseProposalTargetID accepts both "PR-12" and bare "12". Duplicated
// from internal/cli/project/proposal.go because that package will import
// streams once the CLI is refactored, so streams cannot import it back.
func parseProposalTargetID(s string) (int32, error) {
	s = strings.TrimPrefix(s, "PR-")
	id, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("invalid proposal ID: %s", s)
	}
	return int32(id), nil
}

func init() { Register(proposalHandler{}) }
