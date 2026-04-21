package slack

import (
	"context"
	"fmt"

	"github.com/skip-the-line/internal/notifier"
	"github.com/slack-go/slack"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const pkgName = "github.com/skip-the-line"

// Client wraps the slack-go/slack client and implements notification.Notifier.
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

// SendNotification opens a direct message channel with the given Slack user ID
// and posts a Block Kit message rendered from msg.
func (c *Client) SendNotification(ctx context.Context, slackUserID string, msg notifier.Message) error {
	ctx, span := otel.Tracer(pkgName).Start(ctx, "slack.SendNotification")
	span.SetAttributes(attribute.String("slack_user_id", slackUserID))
	defer span.End()

	blocks := buildBlocks(msg)

	channel, _, _, err := c.api.OpenConversationContext(ctx, &slack.OpenConversationParameters{
		Users: []string{slackUserID},
	})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("slack: open DM channel for user %q: %w", slackUserID, err)
	}

	_, _, err = c.api.PostMessageContext(ctx, channel.ID, slack.MsgOptionBlocks(blocks...))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("slack: post message to channel %q: %w", channel.ID, err)
	}

	return nil
}

// buildBlocks converts a platform-agnostic Message into Slack Block Kit blocks.
func buildBlocks(msg notifier.Message) []slack.Block {
	switch msg.EventType {
	case notifier.EventReviewRequested:
		return buildReviewRequestedBlocks(msg)
	case notifier.EventReviewSubmitted:
		return buildReviewSubmittedBlocks(msg)
	case notifier.EventReviewComment:
		return buildReviewCommentBlocks(msg)
	default:
		return []slack.Block{}
	}
}

func buildReviewRequestedBlocks(msg notifier.Message) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*Your review was requested by <@%s>*", msg.AuthorID),
			},
			nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*PR*: #%d | %s", msg.PRNumber, msg.PRTitle),
			},
			nil, nil,
		),
		func() slack.Block {
			btnTxt := slack.NewTextBlockObject("plain_text", "Review now!", false, false)
			btn := slack.NewButtonBlockElement("", "review_button", btnTxt)
			btn.URL = msg.PRURL
			return slack.NewActionBlock("", btn)
		}(),
	}
}

func buildReviewSubmittedBlocks(msg notifier.Message) []slack.Block {
	approved := msg.ReviewState == "approved"
	headerText := fmt.Sprintf("*<@%s> submitted a review on your pull request*", msg.AuthorID)
	buttonText := "View review"
	if approved {
		headerText = fmt.Sprintf("*<@%s> approved your pull request — it's ready to merge! :rocket:*", msg.AuthorID)
		buttonText = "Merge now!"
	}

	return []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: headerText,
			},
			nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*PR*: #%d | %s", msg.PRNumber, msg.PRTitle),
			},
			nil, nil,
		),
		func() slack.Block {
			btnTxt := slack.NewTextBlockObject("plain_text", buttonText, false, false)
			btn := slack.NewButtonBlockElement("", "review_button", btnTxt)
			btn.URL = msg.PRURL
			return slack.NewActionBlock("", btn)
		}(),
	}
}

func buildReviewCommentBlocks(msg notifier.Message) []slack.Block {
	return []slack.Block{
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*<@%s> commented on your pull request*", msg.AuthorID),
			},
			nil, nil,
		),
		slack.NewDividerBlock(),
		slack.NewSectionBlock(
			&slack.TextBlockObject{
				Type: slack.MarkdownType,
				Text: fmt.Sprintf("*PR*: #%d | %s", msg.PRNumber, msg.PRTitle),
			},
			nil, nil,
		),
		func() slack.Block {
			btnTxt := slack.NewTextBlockObject("plain_text", "View comment", false, false)
			btn := slack.NewButtonBlockElement("", "review_button", btnTxt)
			btn.URL = msg.PRURL
			return slack.NewActionBlock("", btn)
		}(),
	}
}
