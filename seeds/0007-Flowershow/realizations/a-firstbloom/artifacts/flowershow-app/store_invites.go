package main

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"
)

func normalizeInvitePermissionRoles(items []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := flowershowAuthorityBundles[item]; !ok {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	sort.Strings(out)
	return out
}

func normalizeOrganizationRole(role string) string {
	role = strings.TrimSpace(role)
	if role == "" {
		return "member"
	}
	return role
}

func validateOrganizationInviteInput(input OrganizationInviteInput) OrganizationInviteInput {
	input.OrganizationID = strings.TrimSpace(input.OrganizationID)
	input.FirstName = strings.TrimSpace(input.FirstName)
	input.LastName = strings.TrimSpace(input.LastName)
	input.Email = strings.TrimSpace(input.Email)
	input.OrganizationRole = normalizeOrganizationRole(input.OrganizationRole)
	input.PermissionRoles = normalizeInvitePermissionRoles(input.PermissionRoles)
	input.InvitedBySubject = strings.TrimSpace(input.InvitedBySubject)
	input.InvitedByName = strings.TrimSpace(input.InvitedByName)
	return input
}

func cloneOrganizationInvite(in *OrganizationInvite) *OrganizationInvite {
	if in == nil {
		return nil
	}
	out := *in
	out.PermissionRoles = append([]string(nil), in.PermissionRoles...)
	return &out
}

func scanOrganizationInvite(row interface {
	Scan(dest ...any) error
}) (*OrganizationInvite, error) {
	var item OrganizationInvite
	var claimedAt *time.Time
	err := row.Scan(
		&item.ID,
		&item.OrganizationID,
		&item.FirstName,
		&item.LastName,
		&item.Email,
		&item.OrganizationRole,
		&item.PermissionRoles,
		&item.Status,
		&item.InvitedBySubject,
		&item.InvitedByName,
		&item.InvitedAt,
		&item.ClaimedSubjectID,
		&item.ClaimedCognitoSub,
		&claimedAt,
	)
	if err != nil {
		return nil, err
	}
	if claimedAt != nil {
		item.ClaimedAt = claimedAt.UTC()
	}
	return &item, nil
}

func (s *memoryStore) createOrganizationInvite(input OrganizationInviteInput) (*OrganizationInvite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	input = validateOrganizationInviteInput(input)
	if input.OrganizationID == "" {
		return nil, errors.New("organization_id is required")
	}
	if input.Email == "" {
		return nil, errors.New("email is required")
	}
	if _, ok := s.organizations[input.OrganizationID]; !ok {
		return nil, errors.New("organization not found")
	}

	item := &OrganizationInvite{
		ID:               newID("orginvite"),
		OrganizationID:   input.OrganizationID,
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		Email:            input.Email,
		OrganizationRole: input.OrganizationRole,
		PermissionRoles:  append([]string(nil), input.PermissionRoles...),
		Status:           "pending",
		InvitedBySubject: input.InvitedBySubject,
		InvitedByName:    input.InvitedByName,
		InvitedAt:        time.Now().UTC(),
	}
	s.orgInvites[item.ID] = item
	s.appendClaim(item.ID, "organization_invite", "organization.invite_created", item)
	return cloneOrganizationInvite(item), nil
}

func (s *memoryStore) organizationInvitesByOrganization(organizationID string) []*OrganizationInvite {
	s.mu.RLock()
	defer s.mu.RUnlock()

	organizationID = strings.TrimSpace(organizationID)
	out := make([]*OrganizationInvite, 0)
	for _, item := range s.orgInvites {
		if item.OrganizationID != organizationID {
			continue
		}
		out = append(out, cloneOrganizationInvite(item))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].InvitedAt.After(out[j].InvitedAt)
	})
	return out
}

func (s *memoryStore) claimOrganizationInvites(email, subjectID, cognitoSub string, assignRole func(UserRoleInput) error) ([]*OrganizationInvite, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	normalizedEmail := normalizeAuthIdentifier(email)
	if normalizedEmail == "" || strings.TrimSpace(subjectID) == "" {
		return nil, nil
	}

	claimed := make([]*OrganizationInvite, 0)
	for _, item := range s.orgInvites {
		if item.Status != "pending" || normalizeAuthIdentifier(item.Email) != normalizedEmail {
			continue
		}
		for _, role := range item.PermissionRoles {
			if assignRole != nil {
				if err := assignRole(UserRoleInput{
					SubjectID:      strings.TrimSpace(subjectID),
					CognitoSub:     strings.TrimSpace(cognitoSub),
					OrganizationID: item.OrganizationID,
					Role:           role,
				}); err != nil {
					return claimed, err
				}
			}
		}
		item.Status = "accepted"
		item.ClaimedSubjectID = strings.TrimSpace(subjectID)
		item.ClaimedCognitoSub = strings.TrimSpace(cognitoSub)
		item.ClaimedAt = time.Now().UTC()
		s.appendClaim(item.ID, "organization_invite", "organization.invite_claimed", item)
		claimed = append(claimed, cloneOrganizationInvite(item))
	}
	sort.Slice(claimed, func(i, j int) bool {
		return claimed[i].InvitedAt.Before(claimed[j].InvitedAt)
	})
	return claimed, nil
}

func (s *postgresFlowershowStore) createOrganizationInvite(input OrganizationInviteInput) (*OrganizationInvite, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("store unavailable")
	}
	input = validateOrganizationInviteInput(input)
	if input.OrganizationID == "" {
		return nil, errors.New("organization_id is required")
	}
	if input.Email == "" {
		return nil, errors.New("email is required")
	}
	if _, ok := s.organizationByID(input.OrganizationID); !ok {
		return nil, errors.New("organization not found")
	}

	item := &OrganizationInvite{
		ID:               newID("orginvite"),
		OrganizationID:   input.OrganizationID,
		FirstName:        input.FirstName,
		LastName:         input.LastName,
		Email:            input.Email,
		OrganizationRole: input.OrganizationRole,
		PermissionRoles:  append([]string(nil), input.PermissionRoles...),
		Status:           "pending",
		InvitedBySubject: input.InvitedBySubject,
		InvitedByName:    input.InvitedByName,
		InvitedAt:        time.Now().UTC(),
	}
	_, err := s.pool.Exec(context.Background(), `
		insert into as_flowershow_m_organization_invites (
		  id, organization_id, first_name, last_name, email, organization_role,
		  permission_roles, status, invited_by_subject, invited_by_name, invited_at,
		  claimed_subject_id, claimed_cognito_sub, claimed_at
		)
		values ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,'','',null)
	`,
		item.ID, item.OrganizationID, item.FirstName, item.LastName, item.Email, item.OrganizationRole,
		item.PermissionRoles, item.Status, item.InvitedBySubject, item.InvitedByName, item.InvitedAt,
	)
	if err != nil {
		return nil, err
	}
	return cloneOrganizationInvite(item), nil
}

func (s *postgresFlowershowStore) organizationInvitesByOrganization(organizationID string) []*OrganizationInvite {
	if s == nil || s.pool == nil {
		return nil
	}
	rows, err := s.pool.Query(context.Background(), `
		select id, organization_id, first_name, last_name, email, organization_role,
		       permission_roles, status, invited_by_subject, invited_by_name, invited_at,
		       claimed_subject_id, claimed_cognito_sub, claimed_at
		from as_flowershow_m_organization_invites
		where organization_id = $1
		order by invited_at desc, id desc
	`, strings.TrimSpace(organizationID))
	if err != nil {
		return nil
	}
	defer rows.Close()

	out := make([]*OrganizationInvite, 0)
	for rows.Next() {
		item, err := scanOrganizationInvite(rows)
		if err != nil {
			return nil
		}
		out = append(out, item)
	}
	return out
}

func (s *postgresFlowershowStore) claimOrganizationInvites(email, subjectID, cognitoSub string, assignRole func(UserRoleInput) error) ([]*OrganizationInvite, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("store unavailable")
	}
	normalizedEmail := normalizeAuthIdentifier(email)
	subjectID = strings.TrimSpace(subjectID)
	cognitoSub = strings.TrimSpace(cognitoSub)
	if normalizedEmail == "" || subjectID == "" {
		return nil, nil
	}

	rows, err := s.pool.Query(context.Background(), `
		select id, organization_id, first_name, last_name, email, organization_role,
		       permission_roles, status, invited_by_subject, invited_by_name, invited_at,
		       claimed_subject_id, claimed_cognito_sub, claimed_at
		from as_flowershow_m_organization_invites
		where lower(email) = $1 and status = 'pending'
		order by invited_at asc, id asc
	`, normalizedEmail)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pending := make([]*OrganizationInvite, 0)
	for rows.Next() {
		item, err := scanOrganizationInvite(rows)
		if err != nil {
			return nil, err
		}
		pending = append(pending, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	claimed := make([]*OrganizationInvite, 0, len(pending))
	for _, item := range pending {
		for _, role := range item.PermissionRoles {
			if assignRole != nil {
				if err := assignRole(UserRoleInput{
					SubjectID:      subjectID,
					CognitoSub:     cognitoSub,
					OrganizationID: item.OrganizationID,
					Role:           role,
				}); err != nil {
					return claimed, err
				}
			}
		}
		claimedAt := time.Now().UTC()
		_, err := s.pool.Exec(context.Background(), `
			update as_flowershow_m_organization_invites
			set status = 'accepted',
			    claimed_subject_id = $2,
			    claimed_cognito_sub = $3,
			    claimed_at = $4
			where id = $1 and status = 'pending'
		`, item.ID, subjectID, cognitoSub, claimedAt)
		if err != nil {
			return claimed, err
		}
		item.Status = "accepted"
		item.ClaimedSubjectID = subjectID
		item.ClaimedCognitoSub = cognitoSub
		item.ClaimedAt = claimedAt
		claimed = append(claimed, item)
	}
	return claimed, nil
}

func (s *postgresFlowershowStore) personByEmail(email string) (*Person, bool) {
	return s.mem.personByEmail(email)
}
