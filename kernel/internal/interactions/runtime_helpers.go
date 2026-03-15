package interactions

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("runtime record not found")

type RuntimeService struct {
	pool *pgxpool.Pool
	now  func() time.Time
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func NewRuntimeService(pool *pgxpool.Pool) *RuntimeService {
	return &RuntimeService{
		pool: pool,
		now:  time.Now,
	}
}

func (s *RuntimeService) Ready() bool {
	return s != nil && s.pool != nil
}

func (s *RuntimeService) Ping(ctx context.Context) error {
	if !s.Ready() {
		return errors.New("runtime database is not configured")
	}
	return s.pool.Ping(ctx)
}

func (s *RuntimeService) nowUTC() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func newID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		panic(err)
	}
	return prefix + "_" + hex.EncodeToString(raw[:])
}

func newToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func normalizeIdentifier(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func jsonBytes(value map[string]interface{}) []byte {
	if len(value) == 0 {
		return []byte(`{}`)
	}

	payload, err := json.Marshal(value)
	if err != nil {
		return []byte(`{}`)
	}
	return payload
}

func parseJSON(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return map[string]interface{}{}
	}

	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return map[string]interface{}{}
	}
	if out == nil {
		return map[string]interface{}{}
	}
	return out
}

func nullString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}

func nullTimeValue(value *time.Time) interface{} {
	if value == nil || value.IsZero() {
		return nil
	}
	return value.UTC()
}

func nullInt(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func toTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	v := value.Time.UTC()
	return &v
}

func statusOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return strings.TrimSpace(value)
}

func rowNotFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func ensureServiceReady(s *RuntimeService) error {
	if !s.Ready() {
		return errors.New("runtime database is not configured")
	}
	return nil
}

func clampLimit(limit, fallback, maximum int) int {
	switch {
	case limit <= 0:
		return fallback
	case limit > maximum:
		return maximum
	default:
		return limit
	}
}

func scanPrincipal(row rowScanner) (Principal, error) {
	var item Principal
	var profile string
	var displayName sql.NullString
	var deactivatedAt sql.NullTime
	if err := row.Scan(
		&item.PrincipalID,
		&item.Kind,
		&displayName,
		&item.Status,
		&profile,
		&item.CreatedAt,
		&deactivatedAt,
	); err != nil {
		return Principal{}, rowNotFound(err)
	}
	if displayName.Valid {
		item.DisplayName = displayName.String
	}
	item.Profile = parseJSON(profile)
	item.DeactivatedAt = toTimePtr(deactivatedAt)
	return item, nil
}

func scanIdentifier(row rowScanner) (PrincipalIdentifier, error) {
	var item PrincipalIdentifier
	var metadata string
	var verifiedAt sql.NullTime
	if err := row.Scan(
		&item.IdentifierID,
		&item.PrincipalID,
		&item.IdentifierType,
		&item.Value,
		&item.NormalizedValue,
		&item.IsPrimary,
		&item.IsVerified,
		&verifiedAt,
		&metadata,
		&item.CreatedAt,
	); err != nil {
		return PrincipalIdentifier{}, rowNotFound(err)
	}
	item.VerifiedAt = toTimePtr(verifiedAt)
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanSession(row rowScanner) (Session, error) {
	var item Session
	var authContext string
	var principalID sql.NullString
	var lastSeenAt sql.NullTime
	var expiresAt sql.NullTime
	var endedAt sql.NullTime
	if err := row.Scan(
		&item.SessionID,
		&principalID,
		&item.Status,
		&authContext,
		&item.UserAgent,
		&item.IPAddress,
		&item.StartedAt,
		&lastSeenAt,
		&expiresAt,
		&endedAt,
	); err != nil {
		return Session{}, rowNotFound(err)
	}
	if principalID.Valid {
		item.PrincipalID = principalID.String
	}
	item.AuthContext = parseJSON(authContext)
	item.LastSeenAt = toTimePtr(lastSeenAt)
	item.ExpiresAt = toTimePtr(expiresAt)
	item.EndedAt = toTimePtr(endedAt)
	return item, nil
}

func scanAuthChallenge(row rowScanner) (AuthChallenge, error) {
	var item AuthChallenge
	var providerID sql.NullString
	var principalID sql.NullString
	var sessionID sql.NullString
	var deliveryTarget sql.NullString
	var scope string
	var metadata string
	var expiresAt sql.NullTime
	var usedAt sql.NullTime
	if err := row.Scan(
		&item.ChallengeID,
		&item.ChallengeKind,
		&providerID,
		&principalID,
		&sessionID,
		&deliveryTarget,
		&item.Status,
		&scope,
		&expiresAt,
		&usedAt,
		&metadata,
		&item.CreatedAt,
	); err != nil {
		return AuthChallenge{}, rowNotFound(err)
	}
	if providerID.Valid {
		item.ProviderID = providerID.String
	}
	if principalID.Valid {
		item.PrincipalID = principalID.String
	}
	if sessionID.Valid {
		item.SessionID = sessionID.String
	}
	if deliveryTarget.Valid {
		item.DeliveryTarget = deliveryTarget.String
	}
	item.Scope = parseJSON(scope)
	item.Metadata = parseJSON(metadata)
	item.ExpiresAt = toTimePtr(expiresAt)
	item.UsedAt = toTimePtr(usedAt)
	return item, nil
}

func scanHandle(row rowScanner) (Handle, error) {
	var item Handle
	var metadata string
	var redirectID sql.NullString
	var retiredAt sql.NullTime
	if err := row.Scan(
		&item.HandleID,
		&item.Namespace,
		&item.Handle,
		&item.SubjectKind,
		&item.SubjectID,
		&item.Status,
		&redirectID,
		&metadata,
		&item.CreatedAt,
		&retiredAt,
	); err != nil {
		return Handle{}, rowNotFound(err)
	}
	if redirectID.Valid {
		item.RedirectToHandleID = redirectID.String
	}
	item.Metadata = parseJSON(metadata)
	item.RetiredAt = toTimePtr(retiredAt)
	return item, nil
}

func scanAccessLink(row rowScanner) (AccessLink, error) {
	var item AccessLink
	var boundPrincipalID sql.NullString
	var createdByPrincipalID sql.NullString
	var scope string
	var metadata string
	var maxUses sql.NullInt32
	var expiresAt sql.NullTime
	var lastUsedAt sql.NullTime
	var revokedAt sql.NullTime
	if err := row.Scan(
		&item.AccessLinkID,
		&item.SubjectKind,
		&item.SubjectID,
		&boundPrincipalID,
		&scope,
		&item.Status,
		&maxUses,
		&item.UseCount,
		&expiresAt,
		&lastUsedAt,
		&createdByPrincipalID,
		&metadata,
		&item.CreatedAt,
		&revokedAt,
	); err != nil {
		return AccessLink{}, rowNotFound(err)
	}
	if boundPrincipalID.Valid {
		item.BoundPrincipalID = boundPrincipalID.String
	}
	if createdByPrincipalID.Valid {
		item.CreatedByPrincipalID = createdByPrincipalID.String
	}
	if maxUses.Valid {
		v := int(maxUses.Int32)
		item.MaxUses = &v
	}
	item.Scope = parseJSON(scope)
	item.Metadata = parseJSON(metadata)
	item.ExpiresAt = toTimePtr(expiresAt)
	item.LastUsedAt = toTimePtr(lastUsedAt)
	item.RevokedAt = toTimePtr(revokedAt)
	return item, nil
}

func scanPublication(row rowScanner) (Publication, error) {
	var item Publication
	var metadata string
	var publishAt sql.NullTime
	var unpublishAt sql.NullTime
	var startsAt sql.NullTime
	var endsAt sql.NullTime
	var timezone sql.NullString
	if err := row.Scan(
		&item.PublicationID,
		&item.SubjectKind,
		&item.SubjectID,
		&item.Status,
		&item.Visibility,
		&publishAt,
		&unpublishAt,
		&startsAt,
		&endsAt,
		&timezone,
		&item.AllDay,
		&metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return Publication{}, rowNotFound(err)
	}
	item.PublishAt = toTimePtr(publishAt)
	item.UnpublishAt = toTimePtr(unpublishAt)
	item.StartsAt = toTimePtr(startsAt)
	item.EndsAt = toTimePtr(endsAt)
	if timezone.Valid {
		item.Timezone = timezone.String
	}
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanStateTransition(row rowScanner) (StateTransition, error) {
	var item StateTransition
	var fromState sql.NullString
	var actorPrincipalID sql.NullString
	var actorSessionID sql.NullString
	var requestID sql.NullString
	var reason sql.NullString
	var metadata string
	if err := row.Scan(
		&item.TransitionID,
		&item.SubjectKind,
		&item.SubjectID,
		&item.Machine,
		&fromState,
		&item.ToState,
		&actorPrincipalID,
		&actorSessionID,
		&requestID,
		&reason,
		&metadata,
		&item.OccurredAt,
	); err != nil {
		return StateTransition{}, rowNotFound(err)
	}
	if fromState.Valid {
		item.FromState = fromState.String
	}
	if actorPrincipalID.Valid {
		item.ActorPrincipalID = actorPrincipalID.String
	}
	if actorSessionID.Valid {
		item.ActorSessionID = actorSessionID.String
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	if reason.Valid {
		item.Reason = reason.String
	}
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanActivityEvent(row rowScanner) (ActivityEvent, error) {
	var item ActivityEvent
	var actorPrincipalID sql.NullString
	var actorSessionID sql.NullString
	var requestID sql.NullString
	var data string
	if err := row.Scan(
		&item.ActivityID,
		&item.SubjectKind,
		&item.SubjectID,
		&actorPrincipalID,
		&actorSessionID,
		&requestID,
		&item.Name,
		&item.Visibility,
		&data,
		&item.OccurredAt,
	); err != nil {
		return ActivityEvent{}, rowNotFound(err)
	}
	if actorPrincipalID.Valid {
		item.ActorPrincipalID = actorPrincipalID.String
	}
	if actorSessionID.Valid {
		item.ActorSessionID = actorSessionID.String
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	item.Data = parseJSON(data)
	return item, nil
}

func scanJob(row rowScanner) (Job, error) {
	var item Job
	var dedupeKey sql.NullString
	var lockedAt sql.NullTime
	var lockedBy sql.NullString
	var payload string
	var lastError sql.NullString
	var finishedAt sql.NullTime
	if err := row.Scan(
		&item.JobID,
		&item.Queue,
		&item.Kind,
		&dedupeKey,
		&item.Status,
		&item.Priority,
		&item.RunAt,
		&lockedAt,
		&lockedBy,
		&item.Attempts,
		&item.MaxAttempts,
		&payload,
		&lastError,
		&item.CreatedAt,
		&finishedAt,
	); err != nil {
		return Job{}, rowNotFound(err)
	}
	if dedupeKey.Valid {
		item.DedupeKey = dedupeKey.String
	}
	if lockedBy.Valid {
		item.LockedBy = lockedBy.String
	}
	if lastError.Valid {
		item.LastError = lastError.String
	}
	item.Payload = parseJSON(payload)
	item.LockedAt = toTimePtr(lockedAt)
	item.FinishedAt = toTimePtr(finishedAt)
	return item, nil
}

func scanRealizationExecution(row rowScanner) (RealizationExecution, error) {
	var item RealizationExecution
	var routeSubdomain sql.NullString
	var routePathPrefix sql.NullString
	var previewPathPrefix sql.NullString
	var upstreamAddr sql.NullString
	var packageRef sql.NullString
	var principalID sql.NullString
	var sessionID sql.NullString
	var requestID sql.NullString
	var metadata string
	var healthyAt sql.NullTime
	var stoppedAt sql.NullTime
	var lastError sql.NullString
	if err := row.Scan(
		&item.ExecutionID,
		&item.Reference,
		&item.SeedID,
		&item.RealizationID,
		&item.Backend,
		&item.Mode,
		&item.Status,
		&routeSubdomain,
		&routePathPrefix,
		&previewPathPrefix,
		&upstreamAddr,
		&packageRef,
		&principalID,
		&sessionID,
		&requestID,
		&metadata,
		&item.StartedAt,
		&healthyAt,
		&stoppedAt,
		&lastError,
	); err != nil {
		return RealizationExecution{}, rowNotFound(err)
	}
	if routeSubdomain.Valid {
		item.RouteSubdomain = routeSubdomain.String
	}
	if routePathPrefix.Valid {
		item.RoutePathPrefix = routePathPrefix.String
	}
	if previewPathPrefix.Valid {
		item.PreviewPathPrefix = previewPathPrefix.String
	}
	if upstreamAddr.Valid {
		item.UpstreamAddr = upstreamAddr.String
	}
	if packageRef.Valid {
		item.ExecutionPackageRef = packageRef.String
	}
	if principalID.Valid {
		item.LaunchedByPrincipalID = principalID.String
	}
	if sessionID.Valid {
		item.LaunchedBySessionID = sessionID.String
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	if lastError.Valid {
		item.LastError = lastError.String
	}
	item.Metadata = parseJSON(metadata)
	item.HealthyAt = toTimePtr(healthyAt)
	item.StoppedAt = toTimePtr(stoppedAt)
	return item, nil
}

func scanRealizationExecutionEvent(row rowScanner) (RealizationExecutionEvent, error) {
	var item RealizationExecutionEvent
	var data string
	if err := row.Scan(
		&item.EventID,
		&item.ExecutionID,
		&item.Name,
		&data,
		&item.OccurredAt,
	); err != nil {
		return RealizationExecutionEvent{}, rowNotFound(err)
	}
	item.Data = parseJSON(data)
	return item, nil
}

func scanRealizationActivation(row rowScanner) (RealizationActivation, error) {
	var item RealizationActivation
	var executionID sql.NullString
	var principalID sql.NullString
	var sessionID sql.NullString
	var requestID sql.NullString
	var metadata string
	if err := row.Scan(
		&item.SeedID,
		&item.Reference,
		&executionID,
		&principalID,
		&sessionID,
		&requestID,
		&metadata,
		&item.ActivatedAt,
	); err != nil {
		return RealizationActivation{}, rowNotFound(err)
	}
	if executionID.Valid {
		item.ExecutionID = executionID.String
	}
	if principalID.Valid {
		item.ActivatedByPrincipalID = principalID.String
	}
	if sessionID.Valid {
		item.ActivatedBySessionID = sessionID.String
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanRealizationRouteBinding(row rowScanner) (RealizationRouteBinding, error) {
	var item RealizationRouteBinding
	var subdomain sql.NullString
	var pathPrefix sql.NullString
	var metadata string
	if err := row.Scan(
		&item.BindingID,
		&item.ExecutionID,
		&item.SeedID,
		&item.Reference,
		&item.BindingKind,
		&subdomain,
		&pathPrefix,
		&item.UpstreamAddr,
		&item.Status,
		&metadata,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return RealizationRouteBinding{}, rowNotFound(err)
	}
	if subdomain.Valid {
		item.Subdomain = subdomain.String
	}
	if pathPrefix.Valid {
		item.PathPrefix = pathPrefix.String
	}
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanOutboxMessage(row rowScanner) (OutboxMessage, error) {
	var item OutboxMessage
	var subjectKind sql.NullString
	var subjectID sql.NullString
	var recipientPrincipalID sql.NullString
	var recipientAddress sql.NullString
	var dedupeKey sql.NullString
	var payload string
	var sentAt sql.NullTime
	var canceledAt sql.NullTime
	if err := row.Scan(
		&item.MessageID,
		&subjectKind,
		&subjectID,
		&recipientPrincipalID,
		&recipientAddress,
		&item.Channel,
		&item.Template,
		&dedupeKey,
		&item.Status,
		&item.EnqueueAfter,
		&payload,
		&item.CreatedAt,
		&sentAt,
		&canceledAt,
	); err != nil {
		return OutboxMessage{}, rowNotFound(err)
	}
	if subjectKind.Valid {
		item.SubjectKind = subjectKind.String
	}
	if subjectID.Valid {
		item.SubjectID = subjectID.String
	}
	if recipientPrincipalID.Valid {
		item.RecipientPrincipalID = recipientPrincipalID.String
	}
	if recipientAddress.Valid {
		item.RecipientAddress = recipientAddress.String
	}
	if dedupeKey.Valid {
		item.DedupeKey = dedupeKey.String
	}
	item.Payload = parseJSON(payload)
	item.SentAt = toTimePtr(sentAt)
	item.CanceledAt = toTimePtr(canceledAt)
	return item, nil
}

func scanThread(row rowScanner) (Thread, error) {
	var item Thread
	var title sql.NullString
	var metadata string
	var closedAt sql.NullTime
	if err := row.Scan(
		&item.ThreadID,
		&item.SubjectKind,
		&item.SubjectID,
		&item.ThreadKind,
		&item.Status,
		&item.Visibility,
		&title,
		&metadata,
		&item.CreatedAt,
		&closedAt,
	); err != nil {
		return Thread{}, rowNotFound(err)
	}
	if title.Valid {
		item.Title = title.String
	}
	item.Metadata = parseJSON(metadata)
	item.ClosedAt = toTimePtr(closedAt)
	return item, nil
}

func scanThreadParticipant(row rowScanner) (ThreadParticipant, error) {
	var item ThreadParticipant
	var principalID sql.NullString
	var deliveryPolicy string
	var metadata string
	var leftAt sql.NullTime
	if err := row.Scan(
		&item.ParticipantID,
		&item.ThreadID,
		&principalID,
		&item.Role,
		&item.Status,
		&deliveryPolicy,
		&metadata,
		&item.JoinedAt,
		&leftAt,
	); err != nil {
		return ThreadParticipant{}, rowNotFound(err)
	}
	if principalID.Valid {
		item.PrincipalID = principalID.String
	}
	item.DeliveryPolicy = parseJSON(deliveryPolicy)
	item.Metadata = parseJSON(metadata)
	item.LeftAt = toTimePtr(leftAt)
	return item, nil
}

func scanMessage(row rowScanner) (Message, error) {
	var item Message
	var authorPrincipalID sql.NullString
	var authorSessionID sql.NullString
	var requestID sql.NullString
	var metadata string
	var editedAt sql.NullTime
	var deletedAt sql.NullTime
	if err := row.Scan(
		&item.MessageID,
		&item.ThreadID,
		&authorPrincipalID,
		&authorSessionID,
		&requestID,
		&item.Kind,
		&item.Visibility,
		&item.BodyFormat,
		&item.Body,
		&metadata,
		&item.CreatedAt,
		&editedAt,
		&deletedAt,
	); err != nil {
		return Message{}, rowNotFound(err)
	}
	if authorPrincipalID.Valid {
		item.AuthorPrincipalID = authorPrincipalID.String
	}
	if authorSessionID.Valid {
		item.AuthorSessionID = authorSessionID.String
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	item.Metadata = parseJSON(metadata)
	item.EditedAt = toTimePtr(editedAt)
	item.DeletedAt = toTimePtr(deletedAt)
	return item, nil
}

func scanSearchDocument(row rowScanner) (SearchDocument, error) {
	var item SearchDocument
	var language sql.NullString
	var title sql.NullString
	var summary sql.NullString
	var facets string
	var ranking string
	var publishedAt sql.NullTime
	var sortAt sql.NullTime
	if err := row.Scan(
		&item.DocumentID,
		&item.SubjectKind,
		&item.SubjectID,
		&item.Scope,
		&language,
		&title,
		&summary,
		&item.BodyText,
		&facets,
		&ranking,
		&publishedAt,
		&sortAt,
		&item.UpdatedAt,
	); err != nil {
		return SearchDocument{}, rowNotFound(err)
	}
	if language.Valid {
		item.Language = language.String
	}
	if title.Valid {
		item.Title = title.String
	}
	if summary.Valid {
		item.Summary = summary.String
	}
	item.Facets = parseJSON(facets)
	item.Ranking = parseJSON(ranking)
	item.PublishedAt = toTimePtr(publishedAt)
	item.SortAt = toTimePtr(sortAt)
	return item, nil
}

func scanGuardDecision(row rowScanner) (GuardDecision, error) {
	var item GuardDecision
	var requestID sql.NullString
	var sessionID sql.NullString
	var principalID sql.NullString
	var subjectKey sql.NullString
	var reason sql.NullString
	var metadata string
	if err := row.Scan(
		&item.DecisionID,
		&item.Namespace,
		&requestID,
		&sessionID,
		&principalID,
		&subjectKey,
		&item.Action,
		&item.Outcome,
		&reason,
		&metadata,
		&item.CreatedAt,
	); err != nil {
		return GuardDecision{}, rowNotFound(err)
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	if sessionID.Valid {
		item.SessionID = sessionID.String
	}
	if principalID.Valid {
		item.PrincipalID = principalID.String
	}
	if subjectKey.Valid {
		item.SubjectKey = subjectKey.String
	}
	if reason.Valid {
		item.Reason = reason.String
	}
	item.Metadata = parseJSON(metadata)
	return item, nil
}

func scanRiskEvent(row rowScanner) (RiskEvent, error) {
	var item RiskEvent
	var subjectKey sql.NullString
	var requestID sql.NullString
	var sessionID sql.NullString
	var principalID sql.NullString
	var data string
	var resolvedAt sql.NullTime
	if err := row.Scan(
		&item.RiskEventID,
		&item.Namespace,
		&subjectKey,
		&requestID,
		&sessionID,
		&principalID,
		&item.Kind,
		&item.Severity,
		&item.Status,
		&data,
		&item.CreatedAt,
		&resolvedAt,
	); err != nil {
		return RiskEvent{}, rowNotFound(err)
	}
	if subjectKey.Valid {
		item.SubjectKey = subjectKey.String
	}
	if requestID.Valid {
		item.RequestID = requestID.String
	}
	if sessionID.Valid {
		item.SessionID = sessionID.String
	}
	if principalID.Valid {
		item.PrincipalID = principalID.String
	}
	item.Data = parseJSON(data)
	item.ResolvedAt = toTimePtr(resolvedAt)
	return item, nil
}

func expectReady(s *RuntimeService) (*pgxpool.Pool, error) {
	if err := ensureServiceReady(s); err != nil {
		return nil, err
	}
	return s.pool, nil
}

func wrapErr(action string, err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrNotFound) {
		return err
	}
	return fmt.Errorf("%s: %w", action, err)
}
