package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"hospital-scheduler/internal/domain"
)

type AssignmentRepo struct{ db *DB }

func NewAssignmentRepo(db *DB) *AssignmentRepo { return &AssignmentRepo{db} }

// Create inserts an assignment and updates the slot count in one transaction.
// Returns ErrDuplicateAssignment if the staff is already assigned to this slot.
func (r *AssignmentRepo) Create(ctx context.Context, a *domain.Assignment,
	slotRepo *SlotRepo, workloadRepo *WorkloadRepo) error {

	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		// Pre-check for duplicate (prevents UNIQUE 500 from leaking through)
		var cnt int
		row := tx.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM assignments
			 WHERE staff_id=? AND slot_id=? AND status!='CANCELED'`,
			a.StaffID, a.SlotID)
		if err := row.Scan(&cnt); err != nil {
			return fmt.Errorf("check duplicate: %w", err)
		}
		if cnt > 0 {
			return ErrDuplicateAssignment
		}

		res, err := tx.ExecContext(ctx,
			`INSERT INTO assignments(staff_id,slot_id,status,source,note,created_by)
			 VALUES(?,?,?,?,?,?)`,
			a.StaffID, a.SlotID, a.Status, a.Source, a.Note, a.CreatedBy)
		if err != nil {
			return fmt.Errorf("insert assignment: %w", err)
		}
		id, _ := res.LastInsertId()
		a.ID = id
		a.CreatedAt = time.Now()
		a.UpdatedAt = time.Now()

		return slotRepo.IncrAssigned(ctx, tx, a.SlotID)
	})
}

// ErrDuplicateAssignment is returned when the same staff+slot already exists.
var ErrDuplicateAssignment = fmt.Errorf("duplicate assignment: staff already assigned to this slot")

// Cancel soft-deletes an assignment and decrements the slot counter.
func (r *AssignmentRepo) Cancel(ctx context.Context, id int64, slotRepo *SlotRepo) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		var slotID int64
		if err := tx.QueryRowContext(ctx,
			`SELECT slot_id FROM assignments WHERE id=? AND status!='CANCELED'`, id,
		).Scan(&slotID); err != nil {
			return fmt.Errorf("get assignment for cancel: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE assignments SET status='CANCELED',updated_at=CURRENT_TIMESTAMP WHERE id=?`, id,
		); err != nil {
			return err
		}
		return slotRepo.DecrAssigned(ctx, tx, slotID)
	})
}

func (r *AssignmentRepo) GetByID(ctx context.Context, id int64) (*domain.Assignment, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,staff_id,slot_id,status,source,note,created_by,created_at,updated_at
		 FROM assignments WHERE id=?`, id)
	return scanAssignment(row)
}

// GetByStaff returns active assignments for a staff member in a date window.
func (r *AssignmentRepo) GetByStaff(ctx context.Context, staffID int64, from, to time.Time) ([]*domain.Assignment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT a.id,a.staff_id,a.slot_id,a.status,a.source,a.note,a.created_by,a.created_at,a.updated_at
		 FROM assignments a
		 JOIN slots s ON s.id=a.slot_id
		 WHERE a.staff_id=? AND s.date>=? AND s.date<=? AND a.status!='CANCELED'
		 ORDER BY s.date, s.shift_type_id`,
		staffID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return drainAssignments(rows)
}

// GetBySlot returns all active assignments for a slot.
func (r *AssignmentRepo) GetBySlot(ctx context.Context, slotID int64) ([]*domain.Assignment, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,staff_id,slot_id,status,source,note,created_by,created_at,updated_at
		 FROM assignments WHERE slot_id=? AND status!='CANCELED'`, slotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return drainAssignments(rows)
}

// GetRecentByStaff returns assignments in the last N days (used by rule checks).
func (r *AssignmentRepo) GetRecentByStaff(ctx context.Context, staffID int64, days int) ([]*domain.Assignment, error) {
	from := time.Now().AddDate(0, 0, -days)
	to   := time.Now().AddDate(0, 0, 30) // include near future
	return r.GetByStaff(ctx, staffID, from, to)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func drainAssignments(rows *sql.Rows) ([]*domain.Assignment, error) {
	var list []*domain.Assignment
	for rows.Next() {
		a, err := scanAssignment(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, a)
	}
	return list, rows.Err()
}

func scanAssignment(s scanner) (*domain.Assignment, error) {
	var a domain.Assignment
	err := s.Scan(&a.ID, &a.StaffID, &a.SlotID, &a.Status,
		&a.Source, &a.Note, &a.CreatedBy, &a.CreatedAt, &a.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan assignment: %w", err)
	}
	return &a, nil
}
