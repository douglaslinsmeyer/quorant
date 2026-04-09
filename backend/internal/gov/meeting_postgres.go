package gov

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresMeetingRepository implements MeetingRepository using a pgxpool.
type PostgresMeetingRepository struct {
	pool *pgxpool.Pool
}

// NewPostgresMeetingRepository creates a new PostgresMeetingRepository backed by pool.
func NewPostgresMeetingRepository(pool *pgxpool.Pool) *PostgresMeetingRepository {
	return &PostgresMeetingRepository{pool: pool}
}

// ─── Create ──────────────────────────────────────────────────────────────────

// Create inserts a new meeting and returns the fully-populated row.
func (r *PostgresMeetingRepository) Create(ctx context.Context, m *Meeting) (*Meeting, error) {
	if m.Metadata == nil {
		m.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return nil, fmt.Errorf("meeting: Create marshal metadata: %w", err)
	}

	const q = `
		INSERT INTO meetings (
			org_id, title, meeting_type, status, scheduled_at,
			ended_at, location, is_virtual, virtual_link,
			notice_required_days, notice_sent_at,
			quorum_required, quorum_present, quorum_met,
			agenda_doc_id, minutes_doc_id, created_by, metadata
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11,
			$12, $13, $14,
			$15, $16, $17, $18
		)
		RETURNING id, org_id, title, meeting_type, status, scheduled_at,
		          ended_at, location, is_virtual, virtual_link,
		          notice_required_days, notice_sent_at,
		          quorum_required, quorum_present, quorum_met,
		          agenda_doc_id, minutes_doc_id, created_by, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		m.OrgID,
		m.Title,
		m.MeetingType,
		m.Status,
		m.ScheduledAt,
		m.EndedAt,
		m.Location,
		m.IsVirtual,
		m.VirtualLink,
		m.NoticeRequiredDays,
		m.NoticeSentAt,
		m.QuorumRequired,
		m.QuorumPresent,
		m.QuorumMet,
		m.AgendaDocID,
		m.MinutesDocID,
		m.CreatedBy,
		metadataJSON,
	)

	result, err := scanMeeting(row)
	if err != nil {
		return nil, fmt.Errorf("meeting: Create: %w", err)
	}
	return result, nil
}

// ─── FindByID ────────────────────────────────────────────────────────────────

// FindByID returns the meeting with the given ID, or nil,nil if not found or soft-deleted.
func (r *PostgresMeetingRepository) FindByID(ctx context.Context, id uuid.UUID) (*Meeting, error) {
	const q = `
		SELECT id, org_id, title, meeting_type, status, scheduled_at,
		       ended_at, location, is_virtual, virtual_link,
		       notice_required_days, notice_sent_at,
		       quorum_required, quorum_present, quorum_met,
		       agenda_doc_id, minutes_doc_id, created_by, metadata,
		       created_at, updated_at, deleted_at
		FROM meetings
		WHERE id = $1 AND deleted_at IS NULL`

	row := r.pool.QueryRow(ctx, q, id)
	result, err := scanMeeting(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("meeting: FindByID: %w", err)
	}
	return result, nil
}

// ─── ListByOrg ───────────────────────────────────────────────────────────────

// ListByOrg returns all non-deleted meetings for the given org, ordered by scheduled_at DESC.
func (r *PostgresMeetingRepository) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]Meeting, error) {
	const q = `
		SELECT id, org_id, title, meeting_type, status, scheduled_at,
		       ended_at, location, is_virtual, virtual_link,
		       notice_required_days, notice_sent_at,
		       quorum_required, quorum_present, quorum_met,
		       agenda_doc_id, minutes_doc_id, created_by, metadata,
		       created_at, updated_at, deleted_at
		FROM meetings
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY scheduled_at DESC`

	rows, err := r.pool.Query(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("meeting: ListByOrg: %w", err)
	}
	defer rows.Close()

	return collectMeetings(rows, "ListByOrg")
}

// ─── Update ──────────────────────────────────────────────────────────────────

// Update persists changes to an existing meeting and returns the updated row.
func (r *PostgresMeetingRepository) Update(ctx context.Context, m *Meeting) (*Meeting, error) {
	if m.Metadata == nil {
		m.Metadata = map[string]any{}
	}

	metadataJSON, err := json.Marshal(m.Metadata)
	if err != nil {
		return nil, fmt.Errorf("meeting: Update marshal metadata: %w", err)
	}

	const q = `
		UPDATE meetings SET
			title                = $1,
			meeting_type         = $2,
			status               = $3,
			scheduled_at         = $4,
			ended_at             = $5,
			location             = $6,
			is_virtual           = $7,
			virtual_link         = $8,
			notice_required_days = $9,
			notice_sent_at       = $10,
			quorum_required      = $11,
			quorum_present       = $12,
			quorum_met           = $13,
			agenda_doc_id        = $14,
			minutes_doc_id       = $15,
			metadata             = $16,
			updated_at           = now()
		WHERE id = $17 AND deleted_at IS NULL
		RETURNING id, org_id, title, meeting_type, status, scheduled_at,
		          ended_at, location, is_virtual, virtual_link,
		          notice_required_days, notice_sent_at,
		          quorum_required, quorum_present, quorum_met,
		          agenda_doc_id, minutes_doc_id, created_by, metadata,
		          created_at, updated_at, deleted_at`

	row := r.pool.QueryRow(ctx, q,
		m.Title,
		m.MeetingType,
		m.Status,
		m.ScheduledAt,
		m.EndedAt,
		m.Location,
		m.IsVirtual,
		m.VirtualLink,
		m.NoticeRequiredDays,
		m.NoticeSentAt,
		m.QuorumRequired,
		m.QuorumPresent,
		m.QuorumMet,
		m.AgendaDocID,
		m.MinutesDocID,
		metadataJSON,
		m.ID,
	)

	result, err := scanMeeting(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("meeting: Update: meeting %s not found or already deleted", m.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("meeting: Update: %w", err)
	}
	return result, nil
}

// ─── SoftDelete ──────────────────────────────────────────────────────────────

// SoftDelete marks a meeting as deleted without removing the row.
func (r *PostgresMeetingRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE meetings SET deleted_at = now() WHERE id = $1 AND deleted_at IS NULL`
	_, err := r.pool.Exec(ctx, q, id)
	if err != nil {
		return fmt.Errorf("meeting: SoftDelete: %w", err)
	}
	return nil
}

// ─── AddAttendee ─────────────────────────────────────────────────────────────

// AddAttendee inserts a meeting attendee record and returns the fully-populated row.
func (r *PostgresMeetingRepository) AddAttendee(ctx context.Context, a *MeetingAttendee) (*MeetingAttendee, error) {
	const q = `
		INSERT INTO meeting_attendees (meeting_id, user_id, role, rsvp_status, attended, arrived_at, left_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, meeting_id, user_id, role, rsvp_status, attended, arrived_at, left_at`

	row := r.pool.QueryRow(ctx, q,
		a.MeetingID,
		a.UserID,
		a.Role,
		a.RSVPStatus,
		a.Attended,
		a.ArrivedAt,
		a.LeftAt,
	)

	result, err := scanMeetingAttendee(row)
	if err != nil {
		return nil, fmt.Errorf("meeting: AddAttendee: %w", err)
	}
	return result, nil
}

// ─── ListAttendeesByMeeting ───────────────────────────────────────────────────

// ListAttendeesByMeeting returns all attendees for the given meeting, ordered by user_id ASC.
func (r *PostgresMeetingRepository) ListAttendeesByMeeting(ctx context.Context, meetingID uuid.UUID) ([]MeetingAttendee, error) {
	const q = `
		SELECT id, meeting_id, user_id, role, rsvp_status, attended, arrived_at, left_at
		FROM meeting_attendees
		WHERE meeting_id = $1
		ORDER BY user_id ASC`

	rows, err := r.pool.Query(ctx, q, meetingID)
	if err != nil {
		return nil, fmt.Errorf("meeting: ListAttendeesByMeeting: %w", err)
	}
	defer rows.Close()

	return collectMeetingAttendees(rows, "ListAttendeesByMeeting")
}

// ─── CreateMotion ─────────────────────────────────────────────────────────────

// CreateMotion inserts a meeting motion and returns the fully-populated row.
func (r *PostgresMeetingRepository) CreateMotion(ctx context.Context, m *MeetingMotion) (*MeetingMotion, error) {
	const q = `
		INSERT INTO meeting_motions (
			meeting_id, motion_number, title, description,
			moved_by, seconded_by, status,
			votes_for, votes_against, votes_abstain,
			result_notes, resource_type, resource_id
		) VALUES (
			$1, $2, $3, $4,
			$5, $6, $7,
			$8, $9, $10,
			$11, $12, $13
		)
		RETURNING id, meeting_id, motion_number, title, description,
		          moved_by, seconded_by, status,
		          votes_for, votes_against, votes_abstain,
		          result_notes, resource_type, resource_id, created_at`

	row := r.pool.QueryRow(ctx, q,
		m.MeetingID,
		m.MotionNumber,
		m.Title,
		m.Description,
		m.MovedBy,
		m.SecondedBy,
		m.Status,
		m.VotesFor,
		m.VotesAgainst,
		m.VotesAbstain,
		m.ResultNotes,
		m.ResourceType,
		m.ResourceID,
	)

	result, err := scanMeetingMotion(row)
	if err != nil {
		return nil, fmt.Errorf("meeting: CreateMotion: %w", err)
	}
	return result, nil
}

// ─── UpdateMotion ─────────────────────────────────────────────────────────────

// UpdateMotion persists changes to an existing motion and returns the updated row.
func (r *PostgresMeetingRepository) UpdateMotion(ctx context.Context, m *MeetingMotion) (*MeetingMotion, error) {
	const q = `
		UPDATE meeting_motions SET
			motion_number  = $1,
			title          = $2,
			description    = $3,
			moved_by       = $4,
			seconded_by    = $5,
			status         = $6,
			votes_for      = $7,
			votes_against  = $8,
			votes_abstain  = $9,
			result_notes   = $10,
			resource_type  = $11,
			resource_id    = $12
		WHERE id = $13
		RETURNING id, meeting_id, motion_number, title, description,
		          moved_by, seconded_by, status,
		          votes_for, votes_against, votes_abstain,
		          result_notes, resource_type, resource_id, created_at`

	row := r.pool.QueryRow(ctx, q,
		m.MotionNumber,
		m.Title,
		m.Description,
		m.MovedBy,
		m.SecondedBy,
		m.Status,
		m.VotesFor,
		m.VotesAgainst,
		m.VotesAbstain,
		m.ResultNotes,
		m.ResourceType,
		m.ResourceID,
		m.ID,
	)

	result, err := scanMeetingMotion(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("meeting: UpdateMotion: motion %s not found", m.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("meeting: UpdateMotion: %w", err)
	}
	return result, nil
}

// ─── ListMotionsByMeeting ─────────────────────────────────────────────────────

// ListMotionsByMeeting returns all motions for the given meeting, ordered by motion_number ASC.
func (r *PostgresMeetingRepository) ListMotionsByMeeting(ctx context.Context, meetingID uuid.UUID) ([]MeetingMotion, error) {
	const q = `
		SELECT id, meeting_id, motion_number, title, description,
		       moved_by, seconded_by, status,
		       votes_for, votes_against, votes_abstain,
		       result_notes, resource_type, resource_id, created_at
		FROM meeting_motions
		WHERE meeting_id = $1
		ORDER BY motion_number ASC`

	rows, err := r.pool.Query(ctx, q, meetingID)
	if err != nil {
		return nil, fmt.Errorf("meeting: ListMotionsByMeeting: %w", err)
	}
	defer rows.Close()

	return collectMeetingMotions(rows, "ListMotionsByMeeting")
}

// ─── CreateHearingLink ────────────────────────────────────────────────────────

// CreateHearingLink inserts a hearing link and returns the fully-populated row.
func (r *PostgresMeetingRepository) CreateHearingLink(ctx context.Context, h *HearingLink) (*HearingLink, error) {
	const q = `
		INSERT INTO hearing_links (
			meeting_id, violation_id,
			homeowner_notified_at, notice_doc_id,
			homeowner_attended, homeowner_statement,
			board_finding, fine_upheld, fine_modified_cents
		) VALUES (
			$1, $2,
			$3, $4,
			$5, $6,
			$7, $8, $9
		)
		RETURNING id, meeting_id, violation_id,
		          homeowner_notified_at, notice_doc_id,
		          homeowner_attended, homeowner_statement,
		          board_finding, fine_upheld, fine_modified_cents, created_at`

	row := r.pool.QueryRow(ctx, q,
		h.MeetingID,
		h.ViolationID,
		h.HomeownerNotifiedAt,
		h.NoticeDocID,
		h.HomeownerAttended,
		h.HomeownerStatement,
		h.BoardFinding,
		h.FineUpheld,
		h.FineModifiedCents,
	)

	result, err := scanHearingLink(row)
	if err != nil {
		return nil, fmt.Errorf("meeting: CreateHearingLink: %w", err)
	}
	return result, nil
}

// ─── FindHearingByViolation ───────────────────────────────────────────────────

// FindHearingByViolation returns the hearing link for the given violation, or nil,nil if not found.
func (r *PostgresMeetingRepository) FindHearingByViolation(ctx context.Context, violationID uuid.UUID) (*HearingLink, error) {
	const q = `
		SELECT id, meeting_id, violation_id,
		       homeowner_notified_at, notice_doc_id,
		       homeowner_attended, homeowner_statement,
		       board_finding, fine_upheld, fine_modified_cents, created_at
		FROM hearing_links
		WHERE violation_id = $1
		ORDER BY created_at DESC
		LIMIT 1`

	row := r.pool.QueryRow(ctx, q, violationID)
	result, err := scanHearingLink(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("meeting: FindHearingByViolation: %w", err)
	}
	return result, nil
}

// ─── UpdateHearingLink ────────────────────────────────────────────────────────

// UpdateHearingLink persists changes to an existing hearing link and returns the updated row.
func (r *PostgresMeetingRepository) UpdateHearingLink(ctx context.Context, h *HearingLink) (*HearingLink, error) {
	const q = `
		UPDATE hearing_links SET
			homeowner_notified_at = $1,
			notice_doc_id         = $2,
			homeowner_attended    = $3,
			homeowner_statement   = $4,
			board_finding         = $5,
			fine_upheld           = $6,
			fine_modified_cents   = $7
		WHERE id = $8
		RETURNING id, meeting_id, violation_id,
		          homeowner_notified_at, notice_doc_id,
		          homeowner_attended, homeowner_statement,
		          board_finding, fine_upheld, fine_modified_cents, created_at`

	row := r.pool.QueryRow(ctx, q,
		h.HomeownerNotifiedAt,
		h.NoticeDocID,
		h.HomeownerAttended,
		h.HomeownerStatement,
		h.BoardFinding,
		h.FineUpheld,
		h.FineModifiedCents,
		h.ID,
	)

	result, err := scanHearingLink(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("meeting: UpdateHearingLink: hearing link %s not found", h.ID)
	}
	if err != nil {
		return nil, fmt.Errorf("meeting: UpdateHearingLink: %w", err)
	}
	return result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// scanMeeting reads a single meetings row.
func scanMeeting(row pgx.Row) (*Meeting, error) {
	var m Meeting
	var metadataRaw []byte

	err := row.Scan(
		&m.ID,
		&m.OrgID,
		&m.Title,
		&m.MeetingType,
		&m.Status,
		&m.ScheduledAt,
		&m.EndedAt,
		&m.Location,
		&m.IsVirtual,
		&m.VirtualLink,
		&m.NoticeRequiredDays,
		&m.NoticeSentAt,
		&m.QuorumRequired,
		&m.QuorumPresent,
		&m.QuorumMet,
		&m.AgendaDocID,
		&m.MinutesDocID,
		&m.CreatedBy,
		&metadataRaw,
		&m.CreatedAt,
		&m.UpdatedAt,
		&m.DeletedAt,
	)
	if err != nil {
		return nil, err
	}

	if len(metadataRaw) > 0 {
		if err := json.Unmarshal(metadataRaw, &m.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal meeting metadata: %w", err)
		}
	}
	if m.Metadata == nil {
		m.Metadata = map[string]any{}
	}

	return &m, nil
}

// collectMeetings drains pgx.Rows into a slice of Meeting values.
func collectMeetings(rows pgx.Rows, op string) ([]Meeting, error) {
	var meetings []Meeting
	for rows.Next() {
		var m Meeting
		var metadataRaw []byte

		if err := rows.Scan(
			&m.ID,
			&m.OrgID,
			&m.Title,
			&m.MeetingType,
			&m.Status,
			&m.ScheduledAt,
			&m.EndedAt,
			&m.Location,
			&m.IsVirtual,
			&m.VirtualLink,
			&m.NoticeRequiredDays,
			&m.NoticeSentAt,
			&m.QuorumRequired,
			&m.QuorumPresent,
			&m.QuorumMet,
			&m.AgendaDocID,
			&m.MinutesDocID,
			&m.CreatedBy,
			&metadataRaw,
			&m.CreatedAt,
			&m.UpdatedAt,
			&m.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("meeting: %s scan: %w", op, err)
		}

		if len(metadataRaw) > 0 {
			if err := json.Unmarshal(metadataRaw, &m.Metadata); err != nil {
				return nil, fmt.Errorf("meeting: %s unmarshal metadata: %w", op, err)
			}
		}
		if m.Metadata == nil {
			m.Metadata = map[string]any{}
		}

		meetings = append(meetings, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("meeting: %s rows: %w", op, err)
	}
	return meetings, nil
}

// scanMeetingAttendee reads a single meeting_attendees row.
func scanMeetingAttendee(row pgx.Row) (*MeetingAttendee, error) {
	var a MeetingAttendee

	err := row.Scan(
		&a.ID,
		&a.MeetingID,
		&a.UserID,
		&a.Role,
		&a.RSVPStatus,
		&a.Attended,
		&a.ArrivedAt,
		&a.LeftAt,
	)
	if err != nil {
		return nil, err
	}

	return &a, nil
}

// collectMeetingAttendees drains pgx.Rows into a slice of MeetingAttendee values.
func collectMeetingAttendees(rows pgx.Rows, op string) ([]MeetingAttendee, error) {
	var attendees []MeetingAttendee
	for rows.Next() {
		var a MeetingAttendee

		if err := rows.Scan(
			&a.ID,
			&a.MeetingID,
			&a.UserID,
			&a.Role,
			&a.RSVPStatus,
			&a.Attended,
			&a.ArrivedAt,
			&a.LeftAt,
		); err != nil {
			return nil, fmt.Errorf("meeting: %s scan: %w", op, err)
		}

		attendees = append(attendees, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("meeting: %s rows: %w", op, err)
	}
	return attendees, nil
}

// scanMeetingMotion reads a single meeting_motions row.
func scanMeetingMotion(row pgx.Row) (*MeetingMotion, error) {
	var m MeetingMotion

	err := row.Scan(
		&m.ID,
		&m.MeetingID,
		&m.MotionNumber,
		&m.Title,
		&m.Description,
		&m.MovedBy,
		&m.SecondedBy,
		&m.Status,
		&m.VotesFor,
		&m.VotesAgainst,
		&m.VotesAbstain,
		&m.ResultNotes,
		&m.ResourceType,
		&m.ResourceID,
		&m.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// collectMeetingMotions drains pgx.Rows into a slice of MeetingMotion values.
func collectMeetingMotions(rows pgx.Rows, op string) ([]MeetingMotion, error) {
	var motions []MeetingMotion
	for rows.Next() {
		var m MeetingMotion

		if err := rows.Scan(
			&m.ID,
			&m.MeetingID,
			&m.MotionNumber,
			&m.Title,
			&m.Description,
			&m.MovedBy,
			&m.SecondedBy,
			&m.Status,
			&m.VotesFor,
			&m.VotesAgainst,
			&m.VotesAbstain,
			&m.ResultNotes,
			&m.ResourceType,
			&m.ResourceID,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("meeting: %s scan: %w", op, err)
		}

		motions = append(motions, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("meeting: %s rows: %w", op, err)
	}
	return motions, nil
}

// scanHearingLink reads a single hearing_links row.
func scanHearingLink(row pgx.Row) (*HearingLink, error) {
	var h HearingLink

	err := row.Scan(
		&h.ID,
		&h.MeetingID,
		&h.ViolationID,
		&h.HomeownerNotifiedAt,
		&h.NoticeDocID,
		&h.HomeownerAttended,
		&h.HomeownerStatement,
		&h.BoardFinding,
		&h.FineUpheld,
		&h.FineModifiedCents,
		&h.CreatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &h, nil
}
