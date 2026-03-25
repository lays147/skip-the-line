package slack

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slack-go/slack"
)

// Client wraps the slack-go/slack client and implements notification.SlackNotifier.
type Client struct {
	api *slack.Client
}

// NewClient creates a new Slack Client authenticated with the given bot token.
func NewClient(token string) *Client {
	return &Client{api: slack.New(token)}
}

// LookupUserByEmail returns the Slack user ID for the given email address.
func (c *Client) LookupUserByEmail(ctx context.Context, email string) (string, error) {
	user, err := c.api.GetUserByEmailContext(ctx, email)
	if err != nil {
		return "", fmt.Errorf("slack: lookup user by email %q: %w", email, err)
	}
	return user.ID, nil
}

// SendDM opens a direct message channel with the given Slack user ID and posts
// a Block Kit message. The message parameter must be a JSON array of Slack block objects.
func (c *Client) SendDM(ctx context.Context, slackUserID, message string) error {
	channel, _, _, err := c.api.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{slackUserID},
	})
	if err != nil {
		return fmt.Errorf("slack: open DM channel for user %q: %w", slackUserID, err)
	}

	var rawBlocks []json.RawMessage
	if err := json.Unmarshal([]byte(message), &rawBlocks); err != nil {
		return fmt.Errorf("slack: invalid block kit JSON: %w", err)
	}

	blocks := make([]slack.Block, 0, len(rawBlocks))
	for _, raw := range rawBlocks {
		block, err := slack.UnmarshalBlock(raw)
		if err != nil {
			return fmt.Errorf("slack: unmarshal block: %w", err)
		}
		blocks = append(blocks, block)
	}

	_, _, err = c.api.PostMessageContext(ctx, channel.ID, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		return fmt.Errorf("slack: post message to channel %q: %w", channel.ID, err)
	}

	return nil
}
