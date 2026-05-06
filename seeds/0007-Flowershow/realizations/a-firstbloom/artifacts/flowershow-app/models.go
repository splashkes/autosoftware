package main

import "time"

// --- Core Domain ---

type Organization struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Level    string `json:"level"` // society, district, region, province, country, global
	ParentID string `json:"parent_id,omitempty"`
}

type OrganizationInput struct {
	Name     string `json:"name"`
	Level    string `json:"level"`
	ParentID string `json:"parent_id,omitempty"`
}

type Show struct {
	ID             string    `json:"id"`
	Slug           string    `json:"slug"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	Location       string    `json:"location"`
	Date           string    `json:"date"`
	Season         string    `json:"season"`
	Status         string    `json:"status"` // draft, published, completed, archived
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type ShowInput struct {
	OrganizationID string `json:"organization_id"`
	Name           string `json:"name"`
	Location       string `json:"location"`
	Date           string `json:"date"`
	Season         string `json:"season"`
}

type ShowJudgeAssignment struct {
	ID         string    `json:"id"`
	ShowID     string    `json:"show_id"`
	PersonID   string    `json:"person_id"`
	AssignedAt time.Time `json:"assigned_at"`
}

type Person struct {
	ID                string `json:"id"`
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Initials          string `json:"initials"`
	Email             string `json:"email,omitempty"`
	Phone             string `json:"phone,omitempty"`
	Specialties       string `json:"specialties,omitempty"`
	Qualifications    string `json:"qualifications,omitempty"`
	Notes             string `json:"notes,omitempty"`
	IsJudge           bool   `json:"is_judge,omitempty"`
	PublicDisplayMode string `json:"public_display_mode,omitempty"`
}

type PersonInput struct {
	FirstName         string `json:"first_name"`
	LastName          string `json:"last_name"`
	Email             string `json:"email,omitempty"`
	Phone             string `json:"phone,omitempty"`
	Specialties       string `json:"specialties,omitempty"`
	Qualifications    string `json:"qualifications,omitempty"`
	Notes             string `json:"notes,omitempty"`
	IsJudge           bool   `json:"is_judge,omitempty"`
	PublicDisplayMode string `json:"public_display_mode,omitempty"`
	OrganizationID    string `json:"organization_id,omitempty"`
	OrganizationRole  string `json:"organization_role,omitempty"`
}

type PersonOrganization struct {
	PersonID       string `json:"person_id"`
	OrganizationID string `json:"organization_id"`
	Role           string `json:"role"` // member, judge, officer
}

type OrganizationInvite struct {
	ID                string    `json:"id"`
	OrganizationID    string    `json:"organization_id"`
	FirstName         string    `json:"first_name"`
	LastName          string    `json:"last_name"`
	Email             string    `json:"email"`
	OrganizationRole  string    `json:"organization_role,omitempty"`
	PermissionRoles   []string  `json:"permission_roles,omitempty"`
	Status            string    `json:"status"` // pending, accepted, revoked
	InvitedBySubject  string    `json:"invited_by_subject,omitempty"`
	InvitedByName     string    `json:"invited_by_name,omitempty"`
	InvitedAt         time.Time `json:"invited_at"`
	ClaimedSubjectID  string    `json:"claimed_subject_id,omitempty"`
	ClaimedCognitoSub string    `json:"claimed_cognito_sub,omitempty"`
	ClaimedAt         time.Time `json:"claimed_at,omitempty"`
}

type OrganizationInviteInput struct {
	OrganizationID   string   `json:"organization_id"`
	FirstName        string   `json:"first_name"`
	LastName         string   `json:"last_name"`
	Email            string   `json:"email"`
	OrganizationRole string   `json:"organization_role,omitempty"`
	PermissionRoles  []string `json:"permission_roles,omitempty"`
	InvitedBySubject string   `json:"invited_by_subject,omitempty"`
	InvitedByName    string   `json:"invited_by_name,omitempty"`
}

// ShowHelperInvite is a show-scoped, low-privilege share link that lets an
// admin hand out a token to volunteers ("can you help add photos?"). Holders
// of the token can redeem it for a ShowBadgeSession that gates lightweight
// helper-only flows in the show. The token is shown ONCE on creation and
// only its sha256 hash is persisted.
type ShowHelperInvite struct {
	ID        string     `json:"id"`
	ShowID    string     `json:"show_id"`
	Token     string     `json:"-"` // 32-byte URL-safe random; never serialized
	TokenHash string     `json:"-"` // sha256 of token, what's stored in DB
	Label     string     `json:"label,omitempty"`
	CreatedBy string     `json:"created_by"`
	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt time.Time  `json:"expires_at"`
	RevokedAt *time.Time `json:"revoked_at,omitempty"`
}

type ShowHelperInviteInput struct {
	ShowID        string `json:"show_id"`
	Label         string `json:"label,omitempty"`
	CreatedBy     string `json:"created_by"`
	ExpiresInDays int    `json:"expires_in_days,omitempty"`
}

// ShowBadgeSession is a lightweight identity attached to a helper after they
// redeem a ShowHelperInvite token. It has no Cognito coupling — just name,
// email, optional matched person, and the show id it belongs to.
type ShowBadgeSession struct {
	ID              string     `json:"id"`
	ShowID          string     `json:"show_id"`
	InviteID        string     `json:"invite_id"`
	Name            string     `json:"name"`
	Email           string     `json:"email"`
	MatchedPersonID string     `json:"matched_person_id,omitempty"`
	SessionToken    string     `json:"-"` // signed cookie value, never serialized
	SessionHash     string     `json:"-"` // sha256 of session token, what's stored
	CreatedAt       time.Time  `json:"created_at"`
	LastSeenAt      time.Time  `json:"last_seen_at"`
	ExpiresAt       time.Time  `json:"expires_at"`
	RevokedAt       *time.Time `json:"revoked_at,omitempty"`
}

type ShowBadgeSessionInput struct {
	ShowID          string `json:"show_id"`
	InviteID        string `json:"invite_id"`
	Name            string `json:"name"`
	Email           string `json:"email"`
	MatchedPersonID string `json:"matched_person_id,omitempty"`
	ExpiresInHours  int    `json:"expires_in_hours,omitempty"`
}

// --- Schedule Hierarchy ---

type ShowSchedule struct {
	ID                         string `json:"id"`
	ShowID                     string `json:"show_id"`
	SourceDocumentID           string `json:"source_document_id,omitempty"`
	EffectiveStandardEditionID string `json:"effective_standard_edition_id,omitempty"`
	Notes                      string `json:"notes,omitempty"`
}

type Division struct {
	ID             string `json:"id"`
	ShowScheduleID string `json:"show_schedule_id"`
	Code           string `json:"code,omitempty"`
	Title          string `json:"title"`
	Domain         string `json:"domain"` // horticulture, design, special, other
	SortOrder      int    `json:"sort_order"`
}

type DivisionInput struct {
	ShowScheduleID string `json:"show_schedule_id"`
	Code           string `json:"code,omitempty"`
	Title          string `json:"title"`
	Domain         string `json:"domain"`
	SortOrder      int    `json:"sort_order"`
}

type Section struct {
	ID         string `json:"id"`
	DivisionID string `json:"division_id"`
	Code       string `json:"code,omitempty"`
	Title      string `json:"title"`
	SortOrder  int    `json:"sort_order"`
}

type SectionInput struct {
	DivisionID string `json:"division_id"`
	Code       string `json:"code,omitempty"`
	Title      string `json:"title"`
	SortOrder  int    `json:"sort_order"`
}

type ShowClass struct {
	ID                string   `json:"id"`
	SectionID         string   `json:"section_id"`
	ClassNumber       string   `json:"class_number"`
	SortOrder         int      `json:"sort_order"`
	Title             string   `json:"title"`
	Domain            string   `json:"domain"`
	Description       string   `json:"description,omitempty"`
	SpecimenCount     int      `json:"specimen_count,omitempty"`
	Unit              string   `json:"unit,omitempty"`
	MeasurementRule   string   `json:"measurement_rule,omitempty"`
	NamingRequirement string   `json:"naming_requirement,omitempty"`
	ContainerRule     string   `json:"container_rule,omitempty"`
	EligibilityRule   string   `json:"eligibility_rule,omitempty"`
	ScheduleNotes     string   `json:"schedule_notes,omitempty"`
	TaxonRefs         []string `json:"taxon_refs,omitempty"`
}

type ShowClassInput struct {
	SectionID         string   `json:"section_id"`
	ClassNumber       string   `json:"class_number"`
	SortOrder         int      `json:"sort_order"`
	Title             string   `json:"title"`
	Domain            string   `json:"domain"`
	Description       string   `json:"description,omitempty"`
	SpecimenCount     int      `json:"specimen_count,omitempty"`
	Unit              string   `json:"unit,omitempty"`
	MeasurementRule   string   `json:"measurement_rule,omitempty"`
	NamingRequirement string   `json:"naming_requirement,omitempty"`
	ContainerRule     string   `json:"container_rule,omitempty"`
	EligibilityRule   string   `json:"eligibility_rule,omitempty"`
	ScheduleNotes     string   `json:"schedule_notes,omitempty"`
	TaxonRefs         []string `json:"taxon_refs,omitempty"`
}

// ClassSplit represents an on-the-day partition of a class for placement
// scoping. Splits are auto-coded a/b/c (in creation order) within each class
// and may carry an optional human label such as "Yellow" or "Double".
type ClassSplit struct {
	ID        string    `json:"id"`
	ClassID   string    `json:"class_id"`
	SplitCode string    `json:"split_code"` // "a", "b", "c", auto-assigned in order
	Label     string    `json:"label,omitempty"`
	SortOrder int       `json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

// ClassSplitInput is the user-supplied payload for class_splits.create. The
// split code is assigned by the store based on existing splits for the class.
type ClassSplitInput struct {
	ClassID string `json:"class_id"`
	Label   string `json:"label,omitempty"`
}

// --- Entries ---

type Entry struct {
	ID             string     `json:"id"`
	ShowID         string     `json:"show_id"`
	ClassID        string     `json:"class_id"`
	SplitID        string     `json:"split_id,omitempty"`
	PersonID       string     `json:"person_id"`
	Name           string     `json:"name"`
	Notes          string     `json:"notes,omitempty"`
	Suppressed     bool       `json:"suppressed,omitempty"`
	ArchivedAt     *time.Time `json:"archived_at,omitempty"`
	Placement      int        `json:"placement,omitempty"` // 1=first, 2=second, 3=third, 0=unplaced
	Points         float64    `json:"points,omitempty"`
	SpecialStatus  bool       `json:"special_status,omitempty"`
	SpecialAwardID string     `json:"special_award_id,omitempty"`
	TaxonRefs      []string   `json:"taxon_refs,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

type EntryInput struct {
	ShowID    string   `json:"show_id"`
	ClassID   string   `json:"class_id"`
	PersonID  string   `json:"person_id"`
	Name      string   `json:"name"`
	Notes     string   `json:"notes,omitempty"`
	TaxonRefs []string `json:"taxon_refs,omitempty"`
}

type ShowCredit struct {
	ID          string    `json:"id"`
	ShowID      string    `json:"show_id"`
	PersonID    string    `json:"person_id,omitempty"`
	DisplayName string    `json:"display_name,omitempty"`
	CreditLabel string    `json:"credit_label"`
	Notes       string    `json:"notes,omitempty"`
	SortOrder   int       `json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
}

type ShowCreditInput struct {
	ShowID      string `json:"show_id"`
	PersonID    string `json:"person_id,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	CreditLabel string `json:"credit_label"`
	Notes       string `json:"notes,omitempty"`
	SortOrder   int    `json:"sort_order"`
}

// --- Media ---

type Media struct {
	ID           string    `json:"id"`
	EntryID      string    `json:"entry_id"`
	MediaType    string    `json:"media_type"` // photo, video
	URL          string    `json:"url"`
	ContentType  string    `json:"content_type,omitempty"`
	IsCover      bool      `json:"is_cover,omitempty"`
	EntityKind   string    `json:"entity_kind,omitempty"` // "entry" (default) or "class"
	ClassID      string    `json:"class_id,omitempty"`
	ThumbnailURL string    `json:"thumbnail_url,omitempty"`
	FileName     string    `json:"file_name"`
	StorageKey   string    `json:"storage_key,omitempty"`
	FileSize     int64     `json:"file_size"`
	Width        int       `json:"width,omitempty"`
	Height       int       `json:"height,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// AttachClassMediaInput carries the metadata needed to record media that is
// attached to a class rather than to an entry. EntryID stays empty for these
// rows.
type AttachClassMediaInput struct {
	ClassID      string `json:"class_id"`
	MediaType    string `json:"media_type"`
	URL          string `json:"url"`
	ContentType  string `json:"content_type,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	FileName     string `json:"file_name"`
	StorageKey   string `json:"storage_key,omitempty"`
	FileSize     int64  `json:"file_size,omitempty"`
	Width        int    `json:"width,omitempty"`
	Height       int    `json:"height,omitempty"`
}

// --- Taxonomy ---

type Taxon struct {
	ID             string `json:"id"`
	TaxonType      string `json:"taxon_type"` // botanical, scientific_name, cultivar, characteristic, design_type, presentation_rule, award_dimension, free_tag
	Name           string `json:"name"`
	ScientificName string `json:"scientific_name,omitempty"`
	Description    string `json:"description,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
}

type TaxonInput struct {
	TaxonType      string `json:"taxon_type"`
	Name           string `json:"name"`
	ScientificName string `json:"scientific_name,omitempty"`
	Description    string `json:"description,omitempty"`
	ParentID       string `json:"parent_id,omitempty"`
}

type TaxonRelation struct {
	ID           string `json:"id"`
	FromTaxonID  string `json:"from_taxon_id"`
	ToTaxonID    string `json:"to_taxon_id"`
	RelationType string `json:"relation_type"` // parent_of, synonym_of, related_to
}

// --- Awards ---

type AwardDefinition struct {
	ID             string   `json:"id"`
	OrganizationID string   `json:"organization_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Season         string   `json:"season"`
	TaxonFilters   []string `json:"taxon_filters,omitempty"`
	ScoringRule    string   `json:"scoring_rule"` // sum, max, count
	MinEntries     int      `json:"min_entries,omitempty"`
}

type AwardInput struct {
	OrganizationID string   `json:"organization_id"`
	Name           string   `json:"name"`
	Description    string   `json:"description,omitempty"`
	Season         string   `json:"season"`
	TaxonFilters   []string `json:"taxon_filters,omitempty"`
	ScoringRule    string   `json:"scoring_rule"`
	MinEntries     int      `json:"min_entries,omitempty"`
}

type AwardResult struct {
	AwardID  string  `json:"award_id"`
	PersonID string  `json:"person_id"`
	Score    float64 `json:"score"`
	Rank     int     `json:"rank"`
}

type UserRole struct {
	ID             string    `json:"id"`
	SubjectID      string    `json:"subject_id,omitempty"`
	CognitoSub     string    `json:"cognito_sub"`
	OrganizationID string    `json:"organization_id,omitempty"`
	ShowID         string    `json:"show_id,omitempty"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
}

type UserRoleInput struct {
	SubjectID      string `json:"subject_id,omitempty"`
	CognitoSub     string `json:"cognito_sub"`
	OrganizationID string `json:"organization_id,omitempty"`
	ShowID         string `json:"show_id,omitempty"`
	Role           string `json:"role"`
}

type Vote struct {
	ID        string    `json:"id"`
	EntryID   string    `json:"entry_id"`
	VoterID   string    `json:"voter_id"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
}

// --- Standards & Provenance ---

type StandardDocument struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	IssuingOrg  string `json:"issuing_org_id"`
	DomainScope string `json:"domain_scope"`
	Description string `json:"description,omitempty"`
}

type StandardEdition struct {
	ID                 string `json:"id"`
	StandardDocumentID string `json:"standard_document_id"`
	EditionLabel       string `json:"edition_label"`
	PublicationYear    int    `json:"publication_year"`
	RevisionDate       string `json:"revision_date,omitempty"`
	Status             string `json:"status"` // current, superseded, draft
	SourceURL          string `json:"source_url,omitempty"`
	SourceKind         string `json:"source_kind,omitempty"` // official_pdf, print_only, excerpt_pdf, catalog_record
}

type SourceDocument struct {
	ID              string `json:"id"`
	OrganizationID  string `json:"organization_id"`
	ShowID          string `json:"show_id,omitempty"`
	Title           string `json:"title"`
	DocumentType    string `json:"document_type"` // rulebook, schedule, fair_book, newsletter, results_sheet, catalog_record
	PublicationDate string `json:"publication_date,omitempty"`
	SourceURL       string `json:"source_url,omitempty"`
	LocalPath       string `json:"local_path,omitempty"`
	Checksum        string `json:"checksum,omitempty"`
}

type SourceCitation struct {
	ID                   string  `json:"id"`
	SourceDocumentID     string  `json:"source_document_id"`
	TargetType           string  `json:"target_type"`
	TargetID             string  `json:"target_id"`
	PageFrom             string  `json:"page_from,omitempty"`
	PageTo               string  `json:"page_to,omitempty"`
	QuotedText           string  `json:"quoted_text,omitempty"`
	ExtractionConfidence float64 `json:"extraction_confidence,omitempty"`
}

// --- Rules ---

type StandardRule struct {
	ID                string `json:"id"`
	StandardEditionID string `json:"standard_edition_id"`
	Domain            string `json:"domain"`
	RuleType          string `json:"rule_type"` // definition, presentation, measurement, eligibility, scale_of_points, naming
	SubjectLabel      string `json:"subject_label"`
	Body              string `json:"body"`
	PageRef           string `json:"page_ref,omitempty"`
}

type ClassRuleOverride struct {
	ID                 string `json:"id"`
	ShowClassID        string `json:"show_class_id"`
	BaseStandardRuleID string `json:"base_standard_rule_id,omitempty"`
	OverrideType       string `json:"override_type"` // replace, narrow, extend, local_only
	Body               string `json:"body"`
	Rationale          string `json:"rationale,omitempty"`
}

// --- Judging ---

type JudgingRubric struct {
	ID                string `json:"id"`
	StandardEditionID string `json:"standard_edition_id,omitempty"`
	ShowID            string `json:"show_id,omitempty"`
	Domain            string `json:"domain"`
	Title             string `json:"title"`
}

type JudgingCriterion struct {
	ID              string `json:"id"`
	JudgingRubricID string `json:"judging_rubric_id"`
	Name            string `json:"name"`
	MaxPoints       int    `json:"max_points"`
	SortOrder       int    `json:"sort_order"`
}

type EntryScorecard struct {
	ID         string  `json:"id"`
	EntryID    string  `json:"entry_id"`
	JudgeID    string  `json:"judge_id"`
	RubricID   string  `json:"rubric_id"`
	TotalScore float64 `json:"total_score"`
	Notes      string  `json:"notes,omitempty"`
}

type EntryCriterionScore struct {
	ID          string  `json:"id"`
	ScorecardID string  `json:"scorecard_id"`
	CriterionID string  `json:"criterion_id"`
	Score       float64 `json:"score"`
	Comment     string  `json:"comment,omitempty"`
}

// --- Ledger ---

type FlowershowObject struct {
	ID         string    `json:"id"`
	ObjectType string    `json:"object_type"`
	Slug       string    `json:"slug,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	CreatedBy  string    `json:"created_by"`
}

type FlowershowClaim struct {
	ID                string    `json:"id"`
	ObjectID          string    `json:"object_id"`
	ClaimSeq          int64     `json:"claim_seq"`
	ClaimType         string    `json:"claim_type"`
	AcceptedAt        time.Time `json:"accepted_at"`
	AcceptedBy        string    `json:"accepted_by"`
	SupersedesClaimID string    `json:"supersedes_claim_id,omitempty"`
	Payload           any       `json:"payload,omitempty"`
}

// --- Leaderboard ---

type LeaderboardEntry struct {
	PersonID    string  `json:"person_id"`
	PersonName  string  `json:"person_name"`
	Initials    string  `json:"initials"`
	TotalPoints float64 `json:"total_points"`
	EntryCount  int     `json:"entry_count"`
	FirstCount  int     `json:"first_count"`
	Rank        int     `json:"rank"`
}
