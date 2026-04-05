package slack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const pkgName = "github.com/skip-the-line"

// Client wraps the slack-go/slack client and implements notification.SlackNotifier.
type Client struct {
	api *slack.Client
}

// NewClient creates a new Slack Client authenticated with the given bot token.
// If apiURL is non-empty the client points at that URL instead of slack.com
// (useful for local development against a mock server).
func NewClient(token, apiURL string) *Client {
	opts := []slack.Option{}
	if apiURL != "" {
		opts = append(opts, slack.OptionAPIURL(apiURL))
	}
	return &Client{api: slack.New(token, opts...)}
}

// LookupUserByEmail returns the Slack user ID for the given email address.
func (c *Client) LookupUserByEmail(ctx context.Context, email string) (string, error) {
	ctx, span := otel.Tracer(pkgName).Start(ctx, "slack.LookupUserByEmail")
	span.SetAttributes(attribute.String("email", email))
	defer span.End()

	user, err := c.api.GetUserByEmailContext(ctx, email)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("slack: lookup user by email %q: %w", email, err)
	}
	return user.ID, nil
}

// SendDM opens a direct message channel with the given Slack user ID and posts
// a Block Kit message. The message parameter must be a JSON array of Slack block objects.
func (c *Client) SendDM(ctx context.Context, slackUserID, message string) error {
	ctx, span := otel.Tracer(pkgName).Start(ctx, "slack.SendDM")
	span.SetAttributes(attribute.String("slack_user_id", slackUserID))
	defer span.End()

	channel, _, _, err := c.api.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{slackUserID},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("slack: open DM channel for user %q: %w", slackUserID, err)
	}

	// Wrap the blocks array in the envelope expected by slack.Blocks.UnmarshalJSON.
	envelope := fmt.Sprintf(`{"blocks":%s}`, message)
	var blocks slack.Blocks
	if err := json.Unmarshal([]byte(envelope), &blocks); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("slack: invalid block kit JSON: %w", err)
	}

	_, _, err = c.api.PostMessageContext(ctx, channel.ID, slack.MsgOptionBlocks(blocks.BlockSet...))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("slack: post message to channel %q: %w", channel.ID, err)
	}

	return nil
}
