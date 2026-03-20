package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	agentTokenMinDays = 1
	agentTokenMaxDays = 60
)

type AgentToken struct {
	ID                string     `json:"id"`
	OwnerCognitoSub   string     `json:"owner_cognito_sub"`
	OwnerEmail        string     `json:"owner_email,omitempty"`
	OwnerName         string     `json:"owner_name,omitempty"`
	Label             string     `json:"label"`
	TokenPrefix       string     `json:"token_prefix"`
	PermissionProfile string     `json:"permission_profile"`
	Permissions       []string   `json:"permissions"`
	CreatedAt         time.Time  `json:"created_at"`
	ExpiresAt         time.Time  `json:"expires_at"`
	LastUsedAt        *time.Time `json:"last_used_at,omitempty"`
	RevokedAt         *time.Time `json:"revoked_at,omitempty"`
	RevokedReason     string     `json:"revoked_reason,omitempty"`
	TokenHash         string     `json:"-"`
}

type AgentTokenIssueInput struct {
	OwnerCognitoSub   string
	OwnerEmail        string
	OwnerName         string
	Label             string
	PermissionProfile string
	Permissions       []string
	ExpiresInDays     int
}

type IssuedAgentToken struct {
	Token    *AgentToken `json:"token"`
	Secret   string      `json:"secret"`
	SecretID string      `json:"secret_id"`
}

type agentPermissionDef struct {
	Key         string
	Label       string
	Description string
}

type agentPermissionProfileDef struct {
	ID          string
	Label       string
	Description string
	AdminOnly   bool
	Permissions []string
}

var agentPermissionCatalog = map[string]agentPermissionDef{
	"account.read": {
		Key:         "account.read",
		Label:       "Read account identity",
		Description: "Read the signed-in account identity, roles, active agent tokens, and capability summary.",
	},
	"admin.dashboard.read": {
		Key:         "admin.dashboard.read",
		Label:       "Read admin dashboard",
		Description: "Read admin overview counts and operator links.",
	},
	"organization.manage": {
		Key:         "organization.manage",
		Label:       "Manage clubs",
		Description: "Read club workspaces and manage organization-scoped administration surfaces.",
	},
	"organization.invites.manage": {
		Key:         "organization.invites.manage",
		Label:       "Manage club invites",
		Description: "Create and review club invitation records that can grant organization-scoped access after sign-in.",
	},
	"shows.workspace.read": {
		Key:         "shows.workspace.read",
		Label:       "Read private show workspaces",
		Description: "Read private show workspace projections, including schedule governance and operator-only context.",
	},
	"entries.private.read": {
		Key:         "entries.private.read",
		Label:       "Read private entry data",
		Description: "Read suppressed entries, private entrant identity, and scorecard detail.",
	},
	"ledger.read": {
		Key:         "ledger.read",
		Label:       "Read ledger history",
		Description: "Read accepted flower-show ledger history for tracked objects.",
	},
	"shows.manage": {
		Key:         "shows.manage",
		Label:       "Manage shows",
		Description: "Create and update shows.",
	},
	"schedule.manage": {
		Key:         "schedule.manage",
		Label:       "Manage schedules",
		Description: "Create or update schedules, divisions, and sections.",
	},
	"classes.manage": {
		Key:         "classes.manage",
		Label:       "Manage classes",
		Description: "Create, update, reorder classes, and recompute placements from scoring.",
	},
	"judges.manage": {
		Key:         "judges.manage",
		Label:       "Manage judges",
		Description: "Assign judges to shows.",
	},
	"entries.manage": {
		Key:         "entries.manage",
		Label:       "Manage entries",
		Description: "Create, update, move, delete, reassign, place, and adjust public visibility for entries.",
	},
	"persons.manage": {
		Key:         "persons.manage",
		Label:       "Manage persons",
		Description: "Create person records used by shows, judging, and entries.",
	},
	"awards.manage": {
		Key:         "awards.manage",
		Label:       "Manage awards",
		Description: "Create awards and compute award results.",
	},
	"taxonomy.manage": {
		Key:         "taxonomy.manage",
		Label:       "Manage taxonomy",
		Description: "Create taxonomy nodes for plants, design types, and rules.",
	},
	"media.manage": {
		Key:         "media.manage",
		Label:       "Manage media",
		Description: "Attach and delete entry media metadata.",
	},
	"rubrics.manage": {
		Key:         "rubrics.manage",
		Label:       "Manage judging rubrics",
		Description: "Create rubrics, criteria, and submit scorecards.",
	},
	"standards.manage": {
		Key:         "standards.manage",
		Label:       "Manage standards",
		Description: "Create standards, editions, rules, and class overrides.",
	},
	"sources.manage": {
		Key:         "sources.manage",
		Label:       "Manage sources and citations",
		Description: "Create source documents, citations, and pre-structured cited imports.",
	},
	"show_credits.manage": {
		Key:         "show_credits.manage",
		Label:       "Manage show credits",
		Description: "Create and delete free-form show credits such as host, scribe, or designer.",
	},
	"roles.manage": {
		Key:         "roles.manage",
		Label:       "Manage roles",
		Description: "Assign flower-show roles to Cognito subjects.",
	},
}

var agentPermissionProfiles = map[string]agentPermissionProfileDef{
	"account_agent": {
		ID:          "account_agent",
		Label:       "Account Assistant",
		Description: "Use this for a personal agent that only needs the signed-in account identity and current access summary.",
		Permissions: []string{"account.read"},
	},
	"show_operator": {
		ID:          "show_operator",
		Label:       "Show Operator",
		Description: "Use this for an agent that should help run flower-show operations without role or ledger control.",
		AdminOnly:   true,
		Permissions: []string{
			"account.read",
			"admin.dashboard.read",
			"organization.manage",
			"organization.invites.manage",
			"shows.workspace.read",
			"entries.private.read",
			"shows.manage",
			"schedule.manage",
			"classes.manage",
			"judges.manage",
			"entries.manage",
			"persons.manage",
			"awards.manage",
			"taxonomy.manage",
			"media.manage",
			"rubrics.manage",
			"standards.manage",
			"sources.manage",
			"show_credits.manage",
		},
	},
	"admin_delegate": {
		ID:          "admin_delegate",
		Label:       "Full Admin Delegate",
		Description: "Use this only when an agent needs the same operational reach as an admin session, including ledger and role control.",
		AdminOnly:   true,
		Permissions: []string{
			"account.read",
			"admin.dashboard.read",
			"organization.manage",
			"organization.invites.manage",
			"shows.workspace.read",
			"entries.private.read",
			"ledger.read",
			"shows.manage",
			"schedule.manage",
			"classes.manage",
			"judges.manage",
			"entries.manage",
			"persons.manage",
			"awards.manage",
			"taxonomy.manage",
			"media.manage",
			"rubrics.manage",
			"standards.manage",
			"sources.manage",
			"show_credits.manage",
			"roles.manage",
		},
	},
}

var commandCapabilityMap = map[string]string{
	"organization.create":        "organization.manage",
	"shows.create":               "shows.manage",
	"shows.update":               "shows.manage",
	"shows.reset_schedule":       "schedule.manage",
	"clubs.invites.create":       "organization.invites.manage",
	"schedules.upsert":           "schedule.manage",
	"divisions.create":           "schedule.manage",
	"sections.create":            "schedule.manage",
	"classes.create":             "classes.manage",
	"classes.update":             "classes.manage",
	"classes.reorder":            "classes.manage",
	"classes.compute_placements": "classes.manage",
	"judges.assign":              "judges.manage",
	"entries.create":             "entries.manage",
	"entries.update":             "entries.manage",
	"entries.move":               "entries.manage",
	"entries.delete":             "entries.manage",
	"entries.reassign_entrant":   "entries.manage",
	"entries.set_placement":      "entries.manage",
	"entries.set_visibility":     "entries.manage",
	"persons.create":             "persons.manage",
	"persons.update":             "persons.manage",
	"awards.create":              "awards.manage",
	"awards.compute":             "awards.manage",
	"taxons.create":              "taxonomy.manage",
	"media.upload":               "media.manage",
	"media.attach":               "media.manage",
	"media.delete":               "media.manage",
	"rubrics.create":             "rubrics.manage",
	"criteria.create":            "rubrics.manage",
	"scorecards.submit":          "rubrics.manage",
	"standards.create":           "standards.manage",
	"editions.create":            "standards.manage",
	"rules.create":               "standards.manage",
	"overrides.create":           "standards.manage",
	"sources.create":             "sources.manage",
	"citations.create":           "sources.manage",
	"ingestions.import":          "sources.manage",
	"show_credits.create":        "show_credits.manage",
	"show_credits.delete":        "show_credits.manage",
	"roles.assign":               "roles.manage",
}

func newAgentTokenSecret() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	secret := "fsa_" + base64.RawURLEncoding.EncodeToString(buf)
	prefix := secret
	if len(prefix) > 16 {
		prefix = prefix[:16]
	}
	return secret, prefix, nil
}

func hashAgentTokenSecret(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

func cloneAgentToken(in *AgentToken) *AgentToken {
	if in == nil {
		return nil
	}
	out := *in
	out.Permissions = append([]string(nil), in.Permissions...)
	if in.LastUsedAt != nil {
		t := *in.LastUsedAt
		out.LastUsedAt = &t
	}
	if in.RevokedAt != nil {
		t := *in.RevokedAt
		out.RevokedAt = &t
	}
	return &out
}

func normalizePermissions(perms []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(perms))
	for _, permission := range perms {
		permission = strings.TrimSpace(permission)
		if permission == "" {
			continue
		}
		if _, ok := agentPermissionCatalog[permission]; !ok {
			continue
		}
		if _, ok := seen[permission]; ok {
			continue
		}
		seen[permission] = struct{}{}
		out = append(out, permission)
	}
	sort.Strings(out)
	return out
}

func agentPermissionLookup(key string) agentPermissionDef {
	if item, ok := agentPermissionCatalog[key]; ok {
		return item
	}
	return agentPermissionDef{
		Key:         key,
		Label:       key,
		Description: key,
	}
}

func formatPermissionList(perms []string) string {
	if len(perms) == 0 {
		return "none"
	}
	labels := make([]string, 0, len(perms))
	for _, permission := range perms {
		labels = append(labels, agentPermissionLookup(permission).Label)
	}
	return strings.Join(labels, ", ")
}

func (a *app) availableAgentTokenProfiles(user UserIdentity) []agentPermissionProfileDef {
	isAdmin := a.userIsAdmin(user)
	profiles := make([]agentPermissionProfileDef, 0, len(agentPermissionProfiles))
	for _, profile := range agentPermissionProfiles {
		if profile.AdminOnly && !isAdmin {
			continue
		}
		profile.Permissions = normalizePermissions(profile.Permissions)
		profiles = append(profiles, profile)
	}
	sort.Slice(profiles, func(i, j int) bool {
		left := profiles[i]
		right := profiles[j]
		if left.AdminOnly != right.AdminOnly {
			return !left.AdminOnly
		}
		return left.Label < right.Label
	})
	return profiles
}

func (a *app) agentTokenProfileForUser(user UserIdentity, profileID string) (agentPermissionProfileDef, bool) {
	for _, profile := range a.availableAgentTokenProfiles(user) {
		if profile.ID == profileID {
			return profile, true
		}
	}
	return agentPermissionProfileDef{}, false
}

func (a *app) userIsAdmin(user UserIdentity) bool {
	for _, role := range a.rolesForUser(user) {
		if role == "admin" {
			return true
		}
	}
	return false
}

func bearerTokenFromRequest(r *http.Request) string {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if auth == "" {
		return ""
	}
	parts := strings.SplitN(auth, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func (a *app) currentAgentToken(r *http.Request) (*AgentToken, bool) {
	raw := bearerTokenFromRequest(r)
	if raw == "" {
		return nil, false
	}
	if a.serviceToken != "" && subtle.ConstantTimeCompare([]byte(raw), []byte(a.serviceToken)) == 1 {
		return nil, false
	}
	return a.store.authenticateAgentToken(raw)
}

func agentTokenHasPermission(token *AgentToken, permission string) bool {
	if token == nil {
		return false
	}
	for _, candidate := range token.Permissions {
		if candidate == permission {
			return true
		}
	}
	return false
}

func requiredCapabilityForCommand(command string) string {
	return commandCapabilityMap[command]
}

func (a *app) authorizeAPICapability(w http.ResponseWriter, r *http.Request, permission string, unauthenticatedHint string) bool {
	if permission == "" {
		return true
	}
	if a.isServiceToken(r) {
		return true
	}
	if user, ok := a.currentUser(r); ok {
		if a.userHasCapability(r.Context(), *user, permission, a.authorityScopesForRequest(r)...) {
			return true
		}
		a.writeAPIError(w, r, http.StatusForbidden, "permission_denied", "Your signed-in account does not currently grant that action.", fmt.Sprintf("Required permission: %s.", agentPermissionLookup(permission).Label), []apiFieldError{
			{Field: "required_permission", Message: permission},
		})
		return false
	}
	if token, ok := a.currentAgentToken(r); ok {
		if agentTokenHasPermission(token, permission) {
			return true
		}
		a.writeAPIError(w, r, http.StatusForbidden, "permission_denied", "This agent token does not grant that action.", fmt.Sprintf("This token currently grants: %s. Generate a new token from /account with %q if the agent needs it.", formatPermissionList(token.Permissions), agentPermissionLookup(permission).Label), []apiFieldError{
			{Field: "required_permission", Message: permission},
		})
		return false
	}
	a.writeAPIError(w, r, http.StatusUnauthorized, "unauthorized", "Authentication required.", unauthenticatedHint, nil)
	return false
}
