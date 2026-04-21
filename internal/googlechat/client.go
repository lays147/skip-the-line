// Package googlechat provides a Google Chat notifier that implements
// notification.Notifier. It sends direct messages to Google Chat users
// identified by their email address.
//
// # Prerequisites
//
//  1. A Google Cloud service account with a JSON key.
//  2. Domain-wide delegation enabled on the service account with the scopes:
//     - https://www.googleapis.com/auth/chat.bot
//     - https://www.googleapis.com/auth/admin.directory.user.readonly
//  3. A Google Workspace admin email for impersonating the Admin SDK calls.
//
// Set GCHAT_CREDENTIALS_JSON and GCHAT_ADMIN_EMAIL in the environment.
package googlechat

import (
	"context"
	"fmt"

	"github.com/skip-the-line/internal/notifier"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"golang.org/x/oauth2/google"
	admin "google.golang.org/api/admin/directory/v1"
	chat "google.golang.org/api/chat/v1"
	"google.golang.org/api/option"
)

const pkgName = "github.com/skip-the-line/googlechat"

// Client implements notification.Notifier using the Google Chat API.
type Client struct {
	chatSvc    *chat.Service
	adminSvc   *admin.Service
	adminEmail string
}

// NewClient creates a Google Chat Client authenticated with the given service
// account credentials JSON. adminEmail is a Google Workspace admin account
// used for domain-wide delegation when resolving users via the Admin SDK.
func NewClient(credentialsJSON []byte, adminEmail string) (*Client, error) {
	ctx := context.Background()

	// Build JWT config with the required scopes.
	jwtCfg, err := google.JWTConfigFromJSON(credentialsJSON,
		chat.ChatBotScope,
		admin.AdminDirectoryUserReadonlyScope,
	)
	if err != nil {
		return nil, fmt.Errorf("googlechat: parse service account credentials: %w", err)
	}

	// The Chat API is called as the bot (no subject / impersonation needed).
	chatTokenSrc := jwtCfg.TokenSource(ctx)
	chatSvc, err := chat.NewService(ctx, option.WithTokenSource(chatTokenSrc))
	if err != nil {
		return nil, fmt.Errorf("googlechat: create chat service: %w", err)
	}

	// The Admin SDK requires impersonating a Workspace admin to look up users.
	jwtCfg.Subject = adminEmail
	adminTokenSrc := jwtCfg.TokenSource(ctx)
	adminSvc, err := admin.NewService(ctx, option.WithTokenSource(adminTokenSrc))
	if err != nil {
		return nil, fmt.Errorf("googlechat: create admin service: %w", err)
	}

	return &Client{
		chatSvc:    chatSvc,
		adminSvc:   adminSvc,
		adminEmail: adminEmail,
	}, nil
}

// LookupUserByEmail returns the Google Chat user resource name ("users/{id}")
// for the given email address by querying the Admin SDK Directory API.
func (c *Client) LookupUserByEmail(ctx context.Context, email string) (string, error) {
	_, span := otel.Tracer(pkgName).Start(ctx, "googlechat.LookupUserByEmail")
	span.SetAttributes(attribute.String("email", email))
	defer span.End()

	user, err := c.adminSvc.Users.Get(email).Context(ctx).Do()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return "", fmt.Errorf("googlechat: lookup user by email %q: %w", email, err)
	}
	return "users/" + user.Id, nil
}

// SendNotification finds (or creates) a direct message space with the user
// identified by recipientID ("users/{id}") and posts a formatted message.
func (c *Client) SendNotification(ctx context.Context, recipientID string, msg notifier.Message) error {
	_, span := otel.Tracer(pkgName).Start(ctx, "googlechat.SendNotification")
	span.SetAttributes(attribute.String("recipient_id", recipientID))
	defer span.End()

	// Find or create the DM space between the bot and the recipient.
	space, err := c.chatSvc.Spaces.FindDirectMessage().Name(recipientID).Context(ctx).Do()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("googlechat: find direct message space for %q: %w", recipientID, err)
	}

	chatMsg := buildMessage(msg)
	_, err = c.chatSvc.Spaces.Messages.Create(space.Name, chatMsg).Context(ctx).Do()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("googlechat: send message to space %q: %w", space.Name, err)
	}

	return nil
}

// buildMessage converts a platform-agnostic Message into a Google Chat message.
// Uses Card v2 format with a header and action button.
func buildMessage(msg notifier.Message) *chat.Message {
	header, body, buttonLabel := messageContent(msg)

	return &chat.Message{
		CardsV2: []*chat.CardWithId{
			{
				CardId: "notification",
				Card: &chat.GoogleAppsCardV1Card{
					Header: &chat.GoogleAppsCardV1CardHeader{
						Title: header,
					},
					Sections: []*chat.GoogleAppsCardV1Section{
						{
							Widgets: []*chat.GoogleAppsCardV1Widget{
								{
									TextParagraph: &chat.GoogleAppsCardV1TextParagraph{
										Text: body,
									},
								},
								{
									ButtonList: &chat.GoogleAppsCardV1ButtonList{
										Buttons: []*chat.GoogleAppsCardV1Button{
											{
												Text: buttonLabel,
												OnClick: &chat.GoogleAppsCardV1OnClick{
													OpenLink: &chat.GoogleAppsCardV1OpenLink{
														Url: msg.PRURL,
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// messageContent returns the header, body text, and button label for a message.
// authorMention uses Google Chat's <users/ID> mention syntax.
func messageContent(msg notifier.Message) (header, body, buttonLabel string) {
	authorMention := fmt.Sprintf("<users/%s>", msg.AuthorID)
	// If AuthorID already starts with "users/", use it directly in the mention format.
	if len(msg.AuthorID) > 6 && msg.AuthorID[:6] == "users/" {
		authorMention = fmt.Sprintf("<%s>", msg.AuthorID)
	}

	prRef := fmt.Sprintf("PR #%d: %s", msg.PRNumber, msg.PRTitle)

	switch msg.EventType {
	case notifier.EventReviewRequested:
		return "Review requested",
			fmt.Sprintf("%s requested your review on %s", authorMention, prRef),
			"Review now"
	case notifier.EventReviewSubmitted:
		if msg.ReviewState == "approved" {
			return "Pull request approved",
				fmt.Sprintf("%s approved your %s — it's ready to merge!", authorMention, prRef),
				"Merge now"
		}
		return "Review submitted",
			fmt.Sprintf("%s submitted a review on your %s", authorMention, prRef),
			"View review"
	case notifier.EventReviewComment:
		return "New comment on your PR",
			fmt.Sprintf("%s commented on your %s", authorMention, prRef),
			"View comment"
	default:
		return "GitHub notification",
			prRef,
			"View on GitHub"
	}
}
