package main

import (
	"context"
	"fmt"
	"html"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
)

// sesSender is the minimum surface of the AWS SES v2 client we need. Tests
// inject a fake; production wires the real *sesv2.Client.
type sesSender interface {
	SendEmail(ctx context.Context, in *sesv2.SendEmailInput, optFns ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
}

// invitationEmailEnv captures the env-driven configuration for sending
// zero-touch invite emails. When either field is empty the helper short-circuits
// (graceful no-op for dev/test).
type invitationEmailEnv struct {
	Region    string
	FromEmail string
}

func loadInvitationEmailEnv() invitationEmailEnv {
	return invitationEmailEnv{
		Region:    strings.TrimSpace(os.Getenv("AWS_REGION")),
		FromEmail: strings.TrimSpace(os.Getenv("FLOWERSHOW_INVITE_FROM_EMAIL")),
	}
}

// invitationEmailParams carries everything sendInvitationEmail needs to render
// and dispatch a single zero-touch invite. The caller assembles this from the
// admin-add-form context.
type invitationEmailParams struct {
	Recipient     *Person
	Show          *Show
	RoleLabel     string // human-readable role description, e.g. "Show admin (helper)"
	RoleSummary   string // one-line plain-English permission summary
	InviterName   string
	InviterEmail  string // optional; used as Reply-To when present
	RedeemURL     string
	LoginURL      string
}

// invitationEmailEligible reports whether a freshly added person is a candidate
// for the zero-touch email invite. The current heuristic: they have an email
// address and were not previously in the store (i.e. this is the first time we
// have their record). Callers pass `wasExisting` from the resolve-or-create
// path so we don't email people whose accounts predate this admin's action.
func invitationEmailEligible(person *Person, wasExisting bool) bool {
	if person == nil {
		return false
	}
	if strings.TrimSpace(person.Email) == "" {
		return false
	}
	if wasExisting {
		return false
	}
	return true
}

// firstName best-effort grabs the first whitespace-separated token from a
// name, falling back to a friendly default.
func firstName(p *Person) string {
	if p == nil {
		return "there"
	}
	if fn := strings.TrimSpace(p.FirstName); fn != "" {
		return fn
	}
	if ln := strings.TrimSpace(p.LastName); ln != "" {
		return ln
	}
	return "there"
}

// renderInvitationEmailHTML returns a minimal inline-style HTML body. SES
// happily accepts a single Simple message with both HTML and text parts.
func renderInvitationEmailHTML(p invitationEmailParams) string {
	greeting := "Hi " + html.EscapeString(firstName(p.Recipient)) + ","
	inviter := html.EscapeString(strings.TrimSpace(p.InviterName))
	if inviter == "" {
		inviter = "A flower-show admin"
	}
	showName := html.EscapeString(strings.TrimSpace(p.Show.Name))
	role := html.EscapeString(strings.TrimSpace(p.RoleLabel))
	roleSummary := html.EscapeString(strings.TrimSpace(p.RoleSummary))
	redeem := html.EscapeString(p.RedeemURL)
	login := html.EscapeString(p.LoginURL)

	return `<!doctype html><html><body style="font-family:system-ui,-apple-system,sans-serif;color:#222;line-height:1.5">` +
		`<p>` + greeting + `</p>` +
		`<p>` + inviter + ` invited you to <strong>` + showName + `</strong> on Flowershow as <strong>` + role + `</strong>.</p>` +
		`<p>` + roleSummary + `</p>` +
		`<p style="margin:1.4em 0"><a href="` + redeem + `" style="display:inline-block;padding:.65em 1.1em;background:#26774b;color:#fff;text-decoration:none;border-radius:6px">Open ` + showName + ` on Flowershow</a></p>` +
		`<p style="font-size:.9em;color:#555">If you prefer to sign in with email + code, visit <a href="` + login + `">` + login + `</a> and enter your email.</p>` +
		`</body></html>`
}

// renderInvitationEmailText returns a plaintext alternative.
func renderInvitationEmailText(p invitationEmailParams) string {
	inviter := strings.TrimSpace(p.InviterName)
	if inviter == "" {
		inviter = "A flower-show admin"
	}
	var b strings.Builder
	b.WriteString("Hi ")
	b.WriteString(firstName(p.Recipient))
	b.WriteString(",\n\n")
	b.WriteString(inviter)
	b.WriteString(" invited you to ")
	b.WriteString(strings.TrimSpace(p.Show.Name))
	b.WriteString(" on Flowershow as ")
	b.WriteString(strings.TrimSpace(p.RoleLabel))
	b.WriteString(".\n\n")
	b.WriteString(strings.TrimSpace(p.RoleSummary))
	b.WriteString("\n\n")
	b.WriteString("Open ")
	b.WriteString(strings.TrimSpace(p.Show.Name))
	b.WriteString(": ")
	b.WriteString(p.RedeemURL)
	b.WriteString("\n\nIf you prefer to sign in with email + code, visit ")
	b.WriteString(p.LoginURL)
	b.WriteString(" and enter your email.\n")
	return b.String()
}

// sendInvitationEmail is the single entry point used from the people-roster
// add-form handlers. When SES env vars are unset it logs and returns nil
// (graceful no-op). The sender argument may be nil — the helper resolves it
// from env in that case. Tests can pass a mock.
func sendInvitationEmail(ctx context.Context, sender sesSender, env invitationEmailEnv, p invitationEmailParams) error {
	if env.Region == "" || env.FromEmail == "" {
		log.Printf("flowershow invite email: SES not configured (region=%q from=%q) — skipping send to %q", env.Region, env.FromEmail, p.Recipient.Email)
		return nil
	}
	if p.Recipient == nil || strings.TrimSpace(p.Recipient.Email) == "" || p.Show == nil {
		return fmt.Errorf("invitation email: recipient/show required")
	}

	if sender == nil {
		cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(env.Region))
		if err != nil {
			return fmt.Errorf("load aws config for SES: %w", err)
		}
		sender = sesv2.NewFromConfig(cfg)
	}

	subject := "You've been added to " + strings.TrimSpace(p.Show.Name) + " on Flowershow"
	htmlBody := renderInvitationEmailHTML(p)
	textBody := renderInvitationEmailText(p)

	from := env.FromEmail
	in := &sesv2.SendEmailInput{
		FromEmailAddress: &from,
		Destination: &sestypes.Destination{
			ToAddresses: []string{p.Recipient.Email},
		},
		Content: &sestypes.EmailContent{
			Simple: &sestypes.Message{
				Subject: &sestypes.Content{Data: &subject},
				Body: &sestypes.Body{
					Html: &sestypes.Content{Data: &htmlBody},
					Text: &sestypes.Content{Data: &textBody},
				},
			},
		},
	}
	if reply := strings.TrimSpace(p.InviterEmail); reply != "" {
		in.ReplyToAddresses = []string{reply}
	}

	if _, err := sender.SendEmail(ctx, in); err != nil {
		return fmt.Errorf("send SES invite to %s: %w", p.Recipient.Email, err)
	}
	return nil
}

// buildPeopleRosterRedeemURL constructs the absolute redeem URL for a freshly
// issued show-helper invite token. Falls back to the request base path when no
// FLOWERSHOW_PUBLIC_HOST is configured (dev mode).
func buildPeopleRosterRedeemURL(r *http.Request, show *Show, token string) string {
	host := strings.TrimSpace(os.Getenv("FLOWERSHOW_PUBLIC_HOST"))
	if host != "" {
		host = strings.TrimSuffix(host, "/")
		// allow either bare host ("example.com") or full origin ("https://example.com")
		if !strings.Contains(host, "://") {
			host = "https://" + host
		}
		return host + "/flowershow/shows/" + show.Slug + "/help-redeem?token=" + url.QueryEscape(token)
	}
	return requestBasePath(r) + "/shows/" + show.Slug + "/help-redeem?token=" + url.QueryEscape(token)
}

// buildPeopleRosterLoginURL constructs the absolute admin-login URL.
func buildPeopleRosterLoginURL(r *http.Request) string {
	host := strings.TrimSpace(os.Getenv("FLOWERSHOW_PUBLIC_HOST"))
	if host != "" {
		host = strings.TrimSuffix(host, "/")
		if !strings.Contains(host, "://") {
			host = "https://" + host
		}
		return host + "/flowershow/admin/login"
	}
	return requestBasePath(r) + "/admin/login"
}
