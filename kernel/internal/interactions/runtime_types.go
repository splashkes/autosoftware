package interactions

import "time"

type Principal struct {
	PrincipalID   string                 `json:"principal_id"`
	Kind          string                 `json:"kind"`
	DisplayName   string                 `json:"display_name,omitempty"`
	Status        string                 `json:"status"`
	Profile       map[string]interface{} `json:"profile"`
	CreatedAt     time.Time              `json:"created_at"`
	DeactivatedAt *time.Time             `json:"deactivated_at,omitempty"`
}

type CreatePrincipalInput struct {
	PrincipalID string                 `json:"principal_id,omitempty"`
	Kind        string                 `json:"kind"`
	DisplayName string                 `json:"display_name,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Profile     map[string]interface{} `json:"profile,omitempty"`
}

type PrincipalIdentifier struct {
	IdentifierID    string                 `json:"identifier_id"`
	PrincipalID     string                 `json:"principal_id"`
	IdentifierType  string                 `json:"identifier_type"`
	Value           string                 `json:"value"`
	NormalizedValue string                 `json:"normalized_value,omitempty"`
	IsPrimary       bool                   `json:"is_primary"`
	IsVerified      bool                   `json:"is_verified"`
	VerifiedAt      *time.Time             `json:"verified_at,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
	CreatedAt       time.Time              `json:"created_at"`
}

type CreatePrincipalIdentifierInput struct {
	IdentifierID    string                 `json:"identifier_id,omitempty"`
	PrincipalID     string                 `json:"principal_id"`
	IdentifierType  string                 `json:"identifier_type"`
	Value           string                 `json:"value"`
	NormalizedValue string                 `json:"normalized_value,omitempty"`
	IsPrimary       bool                   `json:"is_primary"`
	IsVerified      bool                   `json:"is_verified"`
	VerifiedAt      *time.Time             `json:"verified_at,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type Session struct {
	SessionID   string                 `json:"session_id"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	Status      string                 `json:"status"`
	AuthContext map[string]interface{} `json:"auth_context"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	LastSeenAt  *time.Time             `json:"last_seen_at,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
	EndedAt     *time.Time             `json:"ended_at,omitempty"`
}

type CreateSessionInput struct {
	SessionID   string                 `json:"session_id,omitempty"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	Status      string                 `json:"status,omitempty"`
	AuthContext map[string]interface{} `json:"auth_context,omitempty"`
	UserAgent   string                 `json:"user_agent,omitempty"`
	IPAddress   string                 `json:"ip_address,omitempty"`
	StartedAt   *time.Time             `json:"started_at,omitempty"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
}

type ResolvedSession struct {
	SessionID   string                 `json:"session_id"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	Status      string                 `json:"status"`
	AuthContext map[string]interface{} `json:"auth_context"`
	ExpiresAt   *time.Time             `json:"expires_at,omitempty"`
}

type AuthChallenge struct {
	ChallengeID    string                 `json:"challenge_id"`
	ChallengeKind  string                 `json:"challenge_kind"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	PrincipalID    string                 `json:"principal_id,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	DeliveryTarget string                 `json:"delivery_target,omitempty"`
	Status         string                 `json:"status"`
	Scope          map[string]interface{} `json:"scope"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`
	UsedAt         *time.Time             `json:"used_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata"`
	CreatedAt      time.Time              `json:"created_at"`
}

type CreateAuthChallengeInput struct {
	ChallengeID    string                 `json:"challenge_id,omitempty"`
	ChallengeKind  string                 `json:"challenge_kind"`
	ProviderID     string                 `json:"provider_id,omitempty"`
	PrincipalID    string                 `json:"principal_id,omitempty"`
	SessionID      string                 `json:"session_id,omitempty"`
	DeliveryTarget string                 `json:"delivery_target,omitempty"`
	Verifier       string                 `json:"verifier,omitempty"`
	Scope          map[string]interface{} `json:"scope,omitempty"`
	Status         string                 `json:"status,omitempty"`
	ExpiresAt      *time.Time             `json:"expires_at,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type AuthChallengeIssue struct {
	Challenge AuthChallenge `json:"challenge"`
	Verifier  string        `json:"verifier,omitempty"`
}

type AuthIdentity struct {
	IdentityID      string                 `json:"identity_id"`
	ProviderID      string                 `json:"provider_id"`
	PrincipalID     string                 `json:"principal_id"`
	ProviderSubject string                 `json:"provider_subject"`
	Profile         map[string]interface{} `json:"profile"`
	LinkedAt        time.Time              `json:"linked_at"`
	LastSeenAt      *time.Time             `json:"last_seen_at,omitempty"`
}

type BindAuthIdentityInput struct {
	IdentityID      string                 `json:"identity_id,omitempty"`
	ProviderID      string                 `json:"provider_id"`
	PrincipalID     string                 `json:"principal_id"`
	ProviderSubject string                 `json:"provider_subject"`
	Profile         map[string]interface{} `json:"profile,omitempty"`
	LastSeenAt      *time.Time             `json:"last_seen_at,omitempty"`
}

type AuthorityBundle struct {
	BundleID     string                 `json:"bundle_id"`
	DisplayName  string                 `json:"display_name,omitempty"`
	Capabilities []string               `json:"capabilities"`
	Status       string                 `json:"status"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
	RetiredAt    *time.Time             `json:"retired_at,omitempty"`
}

type UpsertAuthorityBundleInput struct {
	BundleID     string                 `json:"bundle_id"`
	DisplayName  string                 `json:"display_name,omitempty"`
	Capabilities []string               `json:"capabilities"`
	Status       string                 `json:"status,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type AuthorityGrant struct {
	GrantID              string                 `json:"grant_id"`
	GrantorPrincipalID   string                 `json:"grantor_principal_id,omitempty"`
	GranteePrincipalID   string                 `json:"grantee_principal_id"`
	BundleID             string                 `json:"bundle_id"`
	CapabilitiesSnapshot []string               `json:"capabilities_snapshot"`
	ScopeKind            string                 `json:"scope_kind"`
	ScopeID              string                 `json:"scope_id"`
	DelegationMode       string                 `json:"delegation_mode"`
	Basis                string                 `json:"basis"`
	Status               string                 `json:"status"`
	EffectiveAt          *time.Time             `json:"effective_at,omitempty"`
	ExpiresAt            *time.Time             `json:"expires_at,omitempty"`
	SupersedesGrantID    string                 `json:"supersedes_grant_id,omitempty"`
	Reason               string                 `json:"reason,omitempty"`
	EvidenceRefs         []string               `json:"evidence_refs"`
	Metadata             map[string]interface{} `json:"metadata"`
	CreatedAt            time.Time              `json:"created_at"`
}

type CreateAuthorityGrantInput struct {
	GrantID            string                 `json:"grant_id,omitempty"`
	GrantorPrincipalID string                 `json:"grantor_principal_id,omitempty"`
	GranteePrincipalID string                 `json:"grantee_principal_id,omitempty"`
	BundleID           string                 `json:"bundle_id,omitempty"`
	ScopeKind          string                 `json:"scope_kind,omitempty"`
	ScopeID            string                 `json:"scope_id,omitempty"`
	DelegationMode     string                 `json:"delegation_mode,omitempty"`
	Basis              string                 `json:"basis,omitempty"`
	Status             string                 `json:"status,omitempty"`
	EffectiveAt        *time.Time             `json:"effective_at,omitempty"`
	ExpiresAt          *time.Time             `json:"expires_at,omitempty"`
	SupersedesGrantID  string                 `json:"supersedes_grant_id,omitempty"`
	Reason             string                 `json:"reason,omitempty"`
	EvidenceRefs       []string               `json:"evidence_refs,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type EffectiveAuthorityCapability struct {
	Capability string   `json:"capability"`
	ScopeKind  string   `json:"scope_kind"`
	ScopeID    string   `json:"scope_id"`
	GrantIDs   []string `json:"grant_ids"`
}

type PrincipalEffectiveAuthority struct {
	PrincipalID       string                         `json:"principal_id"`
	ComputedAt        time.Time                      `json:"computed_at"`
	ActiveGrants      []AuthorityGrant               `json:"active_grants"`
	EffectivePolicies []EffectiveAuthorityCapability `json:"effective_policies"`
}

type Handle struct {
	HandleID           string                 `json:"handle_id"`
	Namespace          string                 `json:"namespace"`
	Handle             string                 `json:"handle"`
	SubjectKind        string                 `json:"subject_kind"`
	SubjectID          string                 `json:"subject_id"`
	Status             string                 `json:"status"`
	RedirectToHandleID string                 `json:"redirect_to_handle_id,omitempty"`
	Metadata           map[string]interface{} `json:"metadata"`
	CreatedAt          time.Time              `json:"created_at"`
	RetiredAt          *time.Time             `json:"retired_at,omitempty"`
}

type AssignHandleInput struct {
	HandleID           string                 `json:"handle_id,omitempty"`
	Namespace          string                 `json:"namespace"`
	Handle             string                 `json:"handle"`
	SubjectKind        string                 `json:"subject_kind"`
	SubjectID          string                 `json:"subject_id"`
	Status             string                 `json:"status,omitempty"`
	RedirectToHandleID string                 `json:"redirect_to_handle_id,omitempty"`
	Metadata           map[string]interface{} `json:"metadata,omitempty"`
}

type AccessLink struct {
	AccessLinkID         string                 `json:"access_link_id"`
	SubjectKind          string                 `json:"subject_kind"`
	SubjectID            string                 `json:"subject_id"`
	BoundPrincipalID     string                 `json:"bound_principal_id,omitempty"`
	Scope                map[string]interface{} `json:"scope"`
	Status               string                 `json:"status"`
	MaxUses              *int                   `json:"max_uses,omitempty"`
	UseCount             int                    `json:"use_count"`
	ExpiresAt            *time.Time             `json:"expires_at,omitempty"`
	LastUsedAt           *time.Time             `json:"last_used_at,omitempty"`
	CreatedByPrincipalID string                 `json:"created_by_principal_id,omitempty"`
	Metadata             map[string]interface{} `json:"metadata"`
	CreatedAt            time.Time              `json:"created_at"`
	RevokedAt            *time.Time             `json:"revoked_at,omitempty"`
}

type CreateAccessLinkInput struct {
	AccessLinkID         string                 `json:"access_link_id,omitempty"`
	Token                string                 `json:"token,omitempty"`
	SubjectKind          string                 `json:"subject_kind"`
	SubjectID            string                 `json:"subject_id"`
	BoundPrincipalID     string                 `json:"bound_principal_id,omitempty"`
	Scope                map[string]interface{} `json:"scope,omitempty"`
	Status               string                 `json:"status,omitempty"`
	MaxUses              *int                   `json:"max_uses,omitempty"`
	ExpiresAt            *time.Time             `json:"expires_at,omitempty"`
	CreatedByPrincipalID string                 `json:"created_by_principal_id,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

type AccessLinkIssue struct {
	AccessLink AccessLink `json:"access_link"`
	Token      string     `json:"token"`
}

type Publication struct {
	PublicationID string                 `json:"publication_id"`
	SubjectKind   string                 `json:"subject_kind"`
	SubjectID     string                 `json:"subject_id"`
	Status        string                 `json:"status"`
	Visibility    string                 `json:"visibility"`
	PublishAt     *time.Time             `json:"publish_at,omitempty"`
	UnpublishAt   *time.Time             `json:"unpublish_at,omitempty"`
	StartsAt      *time.Time             `json:"starts_at,omitempty"`
	EndsAt        *time.Time             `json:"ends_at,omitempty"`
	Timezone      string                 `json:"timezone,omitempty"`
	AllDay        bool                   `json:"all_day"`
	Metadata      map[string]interface{} `json:"metadata"`
	CreatedAt     time.Time              `json:"created_at"`
	UpdatedAt     time.Time              `json:"updated_at"`
}

type UpsertPublicationInput struct {
	PublicationID string                 `json:"publication_id,omitempty"`
	SubjectKind   string                 `json:"subject_kind"`
	SubjectID     string                 `json:"subject_id"`
	Status        string                 `json:"status"`
	Visibility    string                 `json:"visibility,omitempty"`
	PublishAt     *time.Time             `json:"publish_at,omitempty"`
	UnpublishAt   *time.Time             `json:"unpublish_at,omitempty"`
	StartsAt      *time.Time             `json:"starts_at,omitempty"`
	EndsAt        *time.Time             `json:"ends_at,omitempty"`
	Timezone      string                 `json:"timezone,omitempty"`
	AllDay        bool                   `json:"all_day"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type StateTransition struct {
	TransitionID     string                 `json:"transition_id"`
	SubjectKind      string                 `json:"subject_kind"`
	SubjectID        string                 `json:"subject_id"`
	Machine          string                 `json:"machine"`
	FromState        string                 `json:"from_state,omitempty"`
	ToState          string                 `json:"to_state"`
	ActorPrincipalID string                 `json:"actor_principal_id,omitempty"`
	ActorSessionID   string                 `json:"actor_session_id,omitempty"`
	RequestID        string                 `json:"request_id,omitempty"`
	Reason           string                 `json:"reason,omitempty"`
	Metadata         map[string]interface{} `json:"metadata"`
	OccurredAt       time.Time              `json:"occurred_at"`
}

type RecordStateTransitionInput struct {
	TransitionID     string                 `json:"transition_id,omitempty"`
	SubjectKind      string                 `json:"subject_kind"`
	SubjectID        string                 `json:"subject_id"`
	Machine          string                 `json:"machine"`
	FromState        string                 `json:"from_state,omitempty"`
	ToState          string                 `json:"to_state"`
	ActorPrincipalID string                 `json:"actor_principal_id,omitempty"`
	ActorSessionID   string                 `json:"actor_session_id,omitempty"`
	RequestID        string                 `json:"request_id,omitempty"`
	Reason           string                 `json:"reason,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	OccurredAt       *time.Time             `json:"occurred_at,omitempty"`
}

type ActivityEvent struct {
	ActivityID       string                 `json:"activity_id"`
	SubjectKind      string                 `json:"subject_kind"`
	SubjectID        string                 `json:"subject_id"`
	ActorPrincipalID string                 `json:"actor_principal_id,omitempty"`
	ActorSessionID   string                 `json:"actor_session_id,omitempty"`
	RequestID        string                 `json:"request_id,omitempty"`
	Name             string                 `json:"name"`
	Visibility       string                 `json:"visibility"`
	Data             map[string]interface{} `json:"data"`
	OccurredAt       time.Time              `json:"occurred_at"`
}

type RecordActivityEventInput struct {
	ActivityID       string                 `json:"activity_id,omitempty"`
	SubjectKind      string                 `json:"subject_kind"`
	SubjectID        string                 `json:"subject_id"`
	ActorPrincipalID string                 `json:"actor_principal_id,omitempty"`
	ActorSessionID   string                 `json:"actor_session_id,omitempty"`
	RequestID        string                 `json:"request_id,omitempty"`
	Name             string                 `json:"name"`
	Visibility       string                 `json:"visibility,omitempty"`
	Data             map[string]interface{} `json:"data,omitempty"`
	OccurredAt       *time.Time             `json:"occurred_at,omitempty"`
}

type Job struct {
	JobID       string                 `json:"job_id"`
	Queue       string                 `json:"queue"`
	Kind        string                 `json:"kind"`
	DedupeKey   string                 `json:"dedupe_key,omitempty"`
	Status      string                 `json:"status"`
	Priority    int                    `json:"priority"`
	RunAt       time.Time              `json:"run_at"`
	LockedAt    *time.Time             `json:"locked_at,omitempty"`
	LockedBy    string                 `json:"locked_by,omitempty"`
	Attempts    int                    `json:"attempts"`
	MaxAttempts int                    `json:"max_attempts"`
	Payload     map[string]interface{} `json:"payload"`
	LastError   string                 `json:"last_error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	FinishedAt  *time.Time             `json:"finished_at,omitempty"`
}

type EnqueueJobInput struct {
	JobID       string                 `json:"job_id,omitempty"`
	Queue       string                 `json:"queue,omitempty"`
	Kind        string                 `json:"kind"`
	DedupeKey   string                 `json:"dedupe_key,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Priority    int                    `json:"priority,omitempty"`
	RunAt       *time.Time             `json:"run_at,omitempty"`
	MaxAttempts int                    `json:"max_attempts,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}

type JobClaimInput struct {
	Queue  string `json:"queue,omitempty"`
	Worker string `json:"worker"`
	Limit  int    `json:"limit,omitempty"`
}

type JobCompleteInput struct {
	FinishedAt *time.Time `json:"finished_at,omitempty"`
}

type JobFailInput struct {
	Error    string     `json:"error"`
	RetryAt  *time.Time `json:"retry_at,omitempty"`
	FailedAt *time.Time `json:"failed_at,omitempty"`
}

type RealizationExecution struct {
	ExecutionID           string                 `json:"execution_id"`
	Reference             string                 `json:"reference"`
	SeedID                string                 `json:"seed_id"`
	RealizationID         string                 `json:"realization_id"`
	Backend               string                 `json:"backend"`
	Mode                  string                 `json:"mode"`
	Status                string                 `json:"status"`
	RouteSubdomain        string                 `json:"route_subdomain,omitempty"`
	RoutePathPrefix       string                 `json:"route_path_prefix,omitempty"`
	PreviewPathPrefix     string                 `json:"preview_path_prefix,omitempty"`
	UpstreamAddr          string                 `json:"upstream_addr,omitempty"`
	ExecutionPackageRef   string                 `json:"execution_package_ref,omitempty"`
	LaunchedByPrincipalID string                 `json:"launched_by_principal_id,omitempty"`
	LaunchedBySessionID   string                 `json:"launched_by_session_id,omitempty"`
	RequestID             string                 `json:"request_id,omitempty"`
	Metadata              map[string]interface{} `json:"metadata"`
	StartedAt             time.Time              `json:"started_at"`
	HealthyAt             *time.Time             `json:"healthy_at,omitempty"`
	StoppedAt             *time.Time             `json:"stopped_at,omitempty"`
	LastError             string                 `json:"last_error,omitempty"`
}

type CreateRealizationExecutionInput struct {
	ExecutionID           string                 `json:"execution_id,omitempty"`
	Reference             string                 `json:"reference"`
	SeedID                string                 `json:"seed_id"`
	RealizationID         string                 `json:"realization_id"`
	Backend               string                 `json:"backend"`
	Mode                  string                 `json:"mode,omitempty"`
	Status                string                 `json:"status"`
	RouteSubdomain        string                 `json:"route_subdomain,omitempty"`
	RoutePathPrefix       string                 `json:"route_path_prefix,omitempty"`
	PreviewPathPrefix     string                 `json:"preview_path_prefix,omitempty"`
	UpstreamAddr          string                 `json:"upstream_addr,omitempty"`
	ExecutionPackageRef   string                 `json:"execution_package_ref,omitempty"`
	LaunchedByPrincipalID string                 `json:"launched_by_principal_id,omitempty"`
	LaunchedBySessionID   string                 `json:"launched_by_session_id,omitempty"`
	RequestID             string                 `json:"request_id,omitempty"`
	Metadata              map[string]interface{} `json:"metadata,omitempty"`
	StartedAt             *time.Time             `json:"started_at,omitempty"`
}

type UpdateRealizationExecutionInput struct {
	Status            string                 `json:"status,omitempty"`
	UpstreamAddr      string                 `json:"upstream_addr,omitempty"`
	PreviewPathPrefix string                 `json:"preview_path_prefix,omitempty"`
	HealthyAt         *time.Time             `json:"healthy_at,omitempty"`
	StoppedAt         *time.Time             `json:"stopped_at,omitempty"`
	LastError         string                 `json:"last_error,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type RealizationExecutionEvent struct {
	EventID     string                 `json:"event_id"`
	ExecutionID string                 `json:"execution_id"`
	Name        string                 `json:"name"`
	Data        map[string]interface{} `json:"data"`
	OccurredAt  time.Time              `json:"occurred_at"`
}

type RecordRealizationExecutionEventInput struct {
	EventID     string                 `json:"event_id,omitempty"`
	ExecutionID string                 `json:"execution_id"`
	Name        string                 `json:"name"`
	Data        map[string]interface{} `json:"data,omitempty"`
	OccurredAt  *time.Time             `json:"occurred_at,omitempty"`
}

type RealizationActivation struct {
	SeedID                 string                 `json:"seed_id"`
	Reference              string                 `json:"reference"`
	ExecutionID            string                 `json:"execution_id,omitempty"`
	ActivatedByPrincipalID string                 `json:"activated_by_principal_id,omitempty"`
	ActivatedBySessionID   string                 `json:"activated_by_session_id,omitempty"`
	RequestID              string                 `json:"request_id,omitempty"`
	Metadata               map[string]interface{} `json:"metadata"`
	ActivatedAt            time.Time              `json:"activated_at"`
}

type ActivateRealizationInput struct {
	SeedID                 string                 `json:"seed_id"`
	Reference              string                 `json:"reference"`
	ExecutionID            string                 `json:"execution_id"`
	ActivatedByPrincipalID string                 `json:"activated_by_principal_id,omitempty"`
	ActivatedBySessionID   string                 `json:"activated_by_session_id,omitempty"`
	RequestID              string                 `json:"request_id,omitempty"`
	Metadata               map[string]interface{} `json:"metadata,omitempty"`
}

type RealizationRouteBinding struct {
	BindingID    string                 `json:"binding_id"`
	ExecutionID  string                 `json:"execution_id"`
	SeedID       string                 `json:"seed_id"`
	Reference    string                 `json:"reference"`
	BindingKind  string                 `json:"binding_kind"`
	Subdomain    string                 `json:"subdomain,omitempty"`
	PathPrefix   string                 `json:"path_prefix,omitempty"`
	UpstreamAddr string                 `json:"upstream_addr"`
	Status       string                 `json:"status"`
	Metadata     map[string]interface{} `json:"metadata"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

type RealizationRouteBindingInput struct {
	BindingID    string                 `json:"binding_id,omitempty"`
	ExecutionID  string                 `json:"execution_id"`
	SeedID       string                 `json:"seed_id"`
	Reference    string                 `json:"reference"`
	BindingKind  string                 `json:"binding_kind"`
	Subdomain    string                 `json:"subdomain,omitempty"`
	PathPrefix   string                 `json:"path_prefix,omitempty"`
	UpstreamAddr string                 `json:"upstream_addr"`
	Status       string                 `json:"status,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type ProcessSample struct {
	SampleID     string                 `json:"sample_id"`
	ScopeKind    string                 `json:"scope_kind"`
	ServiceName  string                 `json:"service_name,omitempty"`
	ExecutionID  string                 `json:"execution_id,omitempty"`
	SeedID       string                 `json:"seed_id,omitempty"`
	Reference    string                 `json:"reference,omitempty"`
	PID          int                    `json:"pid,omitempty"`
	CPUPercent   float64                `json:"cpu_percent,omitempty"`
	RSSBytes     int64                  `json:"rss_bytes,omitempty"`
	VirtualBytes int64                  `json:"virtual_bytes,omitempty"`
	OpenFDs      int                    `json:"open_fds,omitempty"`
	LogBytes     int64                  `json:"log_bytes,omitempty"`
	Metadata     map[string]interface{} `json:"metadata"`
	ObservedAt   time.Time              `json:"observed_at"`
}

type RecordProcessSampleInput struct {
	SampleID     string                 `json:"sample_id,omitempty"`
	ScopeKind    string                 `json:"scope_kind"`
	ServiceName  string                 `json:"service_name,omitempty"`
	ExecutionID  string                 `json:"execution_id,omitempty"`
	SeedID       string                 `json:"seed_id,omitempty"`
	Reference    string                 `json:"reference,omitempty"`
	PID          int                    `json:"pid,omitempty"`
	CPUPercent   float64                `json:"cpu_percent,omitempty"`
	RSSBytes     int64                  `json:"rss_bytes,omitempty"`
	VirtualBytes int64                  `json:"virtual_bytes,omitempty"`
	OpenFDs      int                    `json:"open_fds,omitempty"`
	LogBytes     int64                  `json:"log_bytes,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ObservedAt   *time.Time             `json:"observed_at,omitempty"`
}

type ListProcessSamplesInput struct {
	ScopeKind   string `json:"scope_kind,omitempty"`
	ServiceName string `json:"service_name,omitempty"`
	ExecutionID string `json:"execution_id,omitempty"`
	Reference   string `json:"reference,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type ServiceEvent struct {
	EventID     string                 `json:"event_id"`
	ServiceName string                 `json:"service_name"`
	EventName   string                 `json:"event_name"`
	Severity    string                 `json:"severity"`
	Message     string                 `json:"message,omitempty"`
	BootID      string                 `json:"boot_id,omitempty"`
	PID         int                    `json:"pid,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	OccurredAt  time.Time              `json:"occurred_at"`
}

type RecordServiceEventInput struct {
	EventID     string                 `json:"event_id,omitempty"`
	ServiceName string                 `json:"service_name"`
	EventName   string                 `json:"event_name"`
	Severity    string                 `json:"severity,omitempty"`
	Message     string                 `json:"message,omitempty"`
	BootID      string                 `json:"boot_id,omitempty"`
	PID         int                    `json:"pid,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	OccurredAt  *time.Time             `json:"occurred_at,omitempty"`
}

type ListServiceEventsInput struct {
	ServiceName string `json:"service_name,omitempty"`
	EventName   string `json:"event_name,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type RealizationSuspension struct {
	SuspensionID      string                 `json:"suspension_id"`
	SeedID            string                 `json:"seed_id"`
	Reference         string                 `json:"reference"`
	ExecutionID       string                 `json:"execution_id,omitempty"`
	RouteSubdomain    string                 `json:"route_subdomain,omitempty"`
	RoutePathPrefix   string                 `json:"route_path_prefix,omitempty"`
	ReasonCode        string                 `json:"reason_code"`
	Message           string                 `json:"message"`
	RemediationTarget string                 `json:"remediation_target"`
	RemediationHint   string                 `json:"remediation_hint"`
	Status            string                 `json:"status"`
	Metadata          map[string]interface{} `json:"metadata"`
	CreatedAt         time.Time              `json:"created_at"`
	ClearedAt         *time.Time             `json:"cleared_at,omitempty"`
}

type UpsertRealizationSuspensionInput struct {
	SuspensionID      string                 `json:"suspension_id,omitempty"`
	SeedID            string                 `json:"seed_id"`
	Reference         string                 `json:"reference"`
	ExecutionID       string                 `json:"execution_id,omitempty"`
	RouteSubdomain    string                 `json:"route_subdomain,omitempty"`
	RoutePathPrefix   string                 `json:"route_path_prefix,omitempty"`
	ReasonCode        string                 `json:"reason_code"`
	Message           string                 `json:"message,omitempty"`
	RemediationTarget string                 `json:"remediation_target,omitempty"`
	RemediationHint   string                 `json:"remediation_hint,omitempty"`
	Status            string                 `json:"status,omitempty"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt         *time.Time             `json:"created_at,omitempty"`
}

type ListRealizationSuspensionsInput struct {
	Reference   string `json:"reference,omitempty"`
	ExecutionID string `json:"execution_id,omitempty"`
	ActiveOnly  bool   `json:"active_only,omitempty"`
	Limit       int    `json:"limit,omitempty"`
}

type OutboxMessage struct {
	MessageID            string                 `json:"message_id"`
	SubjectKind          string                 `json:"subject_kind,omitempty"`
	SubjectID            string                 `json:"subject_id,omitempty"`
	RecipientPrincipalID string                 `json:"recipient_principal_id,omitempty"`
	RecipientAddress     string                 `json:"recipient_address,omitempty"`
	Channel              string                 `json:"channel"`
	Template             string                 `json:"template"`
	DedupeKey            string                 `json:"dedupe_key,omitempty"`
	Status               string                 `json:"status"`
	EnqueueAfter         time.Time              `json:"enqueue_after"`
	Payload              map[string]interface{} `json:"payload"`
	CreatedAt            time.Time              `json:"created_at"`
	SentAt               *time.Time             `json:"sent_at,omitempty"`
	CanceledAt           *time.Time             `json:"canceled_at,omitempty"`
}

type EnqueueOutboxMessageInput struct {
	MessageID            string                 `json:"message_id,omitempty"`
	SubjectKind          string                 `json:"subject_kind,omitempty"`
	SubjectID            string                 `json:"subject_id,omitempty"`
	RecipientPrincipalID string                 `json:"recipient_principal_id,omitempty"`
	RecipientAddress     string                 `json:"recipient_address,omitempty"`
	Channel              string                 `json:"channel"`
	Template             string                 `json:"template"`
	DedupeKey            string                 `json:"dedupe_key,omitempty"`
	Status               string                 `json:"status,omitempty"`
	EnqueueAfter         *time.Time             `json:"enqueue_after,omitempty"`
	Payload              map[string]interface{} `json:"payload,omitempty"`
}

type Thread struct {
	ThreadID    string                 `json:"thread_id"`
	SubjectKind string                 `json:"subject_kind"`
	SubjectID   string                 `json:"subject_id"`
	ThreadKind  string                 `json:"thread_kind"`
	Status      string                 `json:"status"`
	Visibility  string                 `json:"visibility"`
	Title       string                 `json:"title,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	ClosedAt    *time.Time             `json:"closed_at,omitempty"`
}

type CreateThreadInput struct {
	ThreadID    string                 `json:"thread_id,omitempty"`
	SubjectKind string                 `json:"subject_kind"`
	SubjectID   string                 `json:"subject_id"`
	ThreadKind  string                 `json:"thread_kind,omitempty"`
	Status      string                 `json:"status,omitempty"`
	Visibility  string                 `json:"visibility,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type ThreadParticipant struct {
	ParticipantID  string                 `json:"participant_id"`
	ThreadID       string                 `json:"thread_id"`
	PrincipalID    string                 `json:"principal_id,omitempty"`
	Role           string                 `json:"role"`
	Status         string                 `json:"status"`
	DeliveryPolicy map[string]interface{} `json:"delivery_policy"`
	Metadata       map[string]interface{} `json:"metadata"`
	JoinedAt       time.Time              `json:"joined_at"`
	LeftAt         *time.Time             `json:"left_at,omitempty"`
}

type AddThreadParticipantInput struct {
	ParticipantID  string                 `json:"participant_id,omitempty"`
	ThreadID       string                 `json:"thread_id"`
	PrincipalID    string                 `json:"principal_id,omitempty"`
	Role           string                 `json:"role,omitempty"`
	Status         string                 `json:"status,omitempty"`
	DeliveryPolicy map[string]interface{} `json:"delivery_policy,omitempty"`
	Metadata       map[string]interface{} `json:"metadata,omitempty"`
}

type Message struct {
	MessageID         string                 `json:"message_id"`
	ThreadID          string                 `json:"thread_id"`
	AuthorPrincipalID string                 `json:"author_principal_id,omitempty"`
	AuthorSessionID   string                 `json:"author_session_id,omitempty"`
	RequestID         string                 `json:"request_id,omitempty"`
	Kind              string                 `json:"kind"`
	Visibility        string                 `json:"visibility"`
	BodyFormat        string                 `json:"body_format"`
	Body              string                 `json:"body"`
	Metadata          map[string]interface{} `json:"metadata"`
	CreatedAt         time.Time              `json:"created_at"`
	EditedAt          *time.Time             `json:"edited_at,omitempty"`
	DeletedAt         *time.Time             `json:"deleted_at,omitempty"`
}

type PostMessageInput struct {
	MessageID         string                 `json:"message_id,omitempty"`
	ThreadID          string                 `json:"thread_id"`
	AuthorPrincipalID string                 `json:"author_principal_id,omitempty"`
	AuthorSessionID   string                 `json:"author_session_id,omitempty"`
	RequestID         string                 `json:"request_id,omitempty"`
	Kind              string                 `json:"kind,omitempty"`
	Visibility        string                 `json:"visibility,omitempty"`
	BodyFormat        string                 `json:"body_format,omitempty"`
	Body              string                 `json:"body"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

type SearchDocument struct {
	DocumentID  string                 `json:"document_id"`
	SubjectKind string                 `json:"subject_kind"`
	SubjectID   string                 `json:"subject_id"`
	Scope       string                 `json:"scope"`
	Language    string                 `json:"language,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	BodyText    string                 `json:"body_text"`
	Facets      map[string]interface{} `json:"facets"`
	Ranking     map[string]interface{} `json:"ranking"`
	PublishedAt *time.Time             `json:"published_at,omitempty"`
	SortAt      *time.Time             `json:"sort_at,omitempty"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

type UpsertSearchDocumentInput struct {
	DocumentID  string                 `json:"document_id,omitempty"`
	SubjectKind string                 `json:"subject_kind"`
	SubjectID   string                 `json:"subject_id"`
	Scope       string                 `json:"scope,omitempty"`
	Language    string                 `json:"language,omitempty"`
	Title       string                 `json:"title,omitempty"`
	Summary     string                 `json:"summary,omitempty"`
	BodyText    string                 `json:"body_text,omitempty"`
	Facets      map[string]interface{} `json:"facets,omitempty"`
	Ranking     map[string]interface{} `json:"ranking,omitempty"`
	PublishedAt *time.Time             `json:"published_at,omitempty"`
	SortAt      *time.Time             `json:"sort_at,omitempty"`
}

type SearchDocumentsInput struct {
	Scope string `json:"scope,omitempty"`
	Query string `json:"query,omitempty"`
	Limit int    `json:"limit,omitempty"`
}

type RateLimitDecision struct {
	Namespace       string                 `json:"namespace"`
	SubjectKey      string                 `json:"subject_key"`
	Allowed         bool                   `json:"allowed"`
	HitCount        int64                  `json:"hit_count"`
	Limit           int64                  `json:"limit"`
	WindowStartedAt time.Time              `json:"window_started_at"`
	WindowEndsAt    time.Time              `json:"window_ends_at"`
	BlockedUntil    *time.Time             `json:"blocked_until,omitempty"`
	RetryAfter      time.Duration          `json:"retry_after,omitempty"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

type EnforceRateLimitInput struct {
	Namespace     string                 `json:"namespace"`
	SubjectKey    string                 `json:"subject_key"`
	Action        string                 `json:"action,omitempty"`
	Limit         int64                  `json:"limit"`
	Window        time.Duration          `json:"window,omitempty"`
	BlockDuration time.Duration          `json:"block_duration,omitempty"`
	RequestID     string                 `json:"request_id,omitempty"`
	SessionID     string                 `json:"session_id,omitempty"`
	PrincipalID   string                 `json:"principal_id,omitempty"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

type RateLimitError struct {
	Namespace  string        `json:"namespace"`
	SubjectKey string        `json:"subject_key"`
	Message    string        `json:"message"`
	RetryAfter time.Duration `json:"retry_after,omitempty"`
}

func (e *RateLimitError) Error() string {
	if e == nil || e.Message == "" {
		return "rate limited"
	}
	return e.Message
}

func (e *RateLimitError) Unwrap() error {
	return ErrRateLimited
}

type GuardDecision struct {
	DecisionID  string                 `json:"decision_id"`
	Namespace   string                 `json:"namespace"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	SubjectKey  string                 `json:"subject_key,omitempty"`
	Action      string                 `json:"action"`
	Outcome     string                 `json:"outcome"`
	Reason      string                 `json:"reason,omitempty"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
}

type RecordGuardDecisionInput struct {
	DecisionID  string                 `json:"decision_id,omitempty"`
	Namespace   string                 `json:"namespace"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	SubjectKey  string                 `json:"subject_key,omitempty"`
	Action      string                 `json:"action"`
	Outcome     string                 `json:"outcome"`
	Reason      string                 `json:"reason,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

type RiskEvent struct {
	RiskEventID string                 `json:"risk_event_id"`
	Namespace   string                 `json:"namespace"`
	SubjectKey  string                 `json:"subject_key,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	Kind        string                 `json:"kind"`
	Severity    string                 `json:"severity"`
	Status      string                 `json:"status"`
	Data        map[string]interface{} `json:"data"`
	CreatedAt   time.Time              `json:"created_at"`
	ResolvedAt  *time.Time             `json:"resolved_at,omitempty"`
}

type RecordRiskEventInput struct {
	RiskEventID string                 `json:"risk_event_id,omitempty"`
	Namespace   string                 `json:"namespace"`
	SubjectKey  string                 `json:"subject_key,omitempty"`
	RequestID   string                 `json:"request_id,omitempty"`
	SessionID   string                 `json:"session_id,omitempty"`
	PrincipalID string                 `json:"principal_id,omitempty"`
	Kind        string                 `json:"kind"`
	Severity    string                 `json:"severity"`
	Status      string                 `json:"status,omitempty"`
	Data        map[string]interface{} `json:"data,omitempty"`
}
