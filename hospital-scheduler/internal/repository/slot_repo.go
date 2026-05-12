package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"hospital-scheduler/internal/domain"
)

type SlotRepo struct{ db *DB }

func NewSlotRepo(db *DB) *SlotRepo { return &SlotRepo{db} }

func (r *SlotRepo) Create(ctx context.Context, s *domain.Slot) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO slots(department_id,shift_type_id,date,required_staff,required_role,status)
			 VALUES(?,?,?,?,?,?)`,
			s.DepartmentID, s.ShiftTypeID,
			s.Date.Format("2006-01-02"),
			s.RequiredStaff, s.RequiredRole, domain.SlotOpen)
		if err != nil {
			return fmt.Errorf("insert slot: %w", err)
		}
		id, _ := res.LastInsertId()
		s.ID = id

		for _, q := range s.RequiredQuals {
			if _, err = tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO slot_qualifications(slot_id,qualification) VALUES(?,?)`,
				id, q); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SlotRepo) GetByID(ctx context.Context, id int64) (*domain.Slot, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,department_id,shift_type_id,date,required_staff,required_role,status,assigned_count,created_at
		 FROM slots WHERE id=?`, id)
	s, err := scanSlot(row)
	if err != nil {
		return nil, err
	}
	// getSlotQuals is safe here: row already closed
	s.RequiredQuals, err = r.getSlotQuals(ctx, id)
	return s, err
}

// ListByDateRange fetches slots then bulk-loads their qualifications.
func (r *SlotRepo) ListByDateRange(ctx context.Context, deptID int64, from, to time.Time) ([]*domain.Slot, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,department_id,shift_type_id,date,required_staff,required_role,status,assigned_count,created_at
		 FROM slots
		 WHERE department_id=? AND date>=? AND date<=? AND status!='CANCELED'
		 ORDER BY date,shift_type_id`,
		deptID,
		from.Format("2006-01-02"),
		to.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return r.drainAndAttachQuals(ctx, rows)
}

func (r *SlotRepo) ListOpen(ctx context.Context, deptID int64, date time.Time) ([]*domain.Slot, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,department_id,shift_type_id,date,required_staff,required_role,status,assigned_count,created_at
		 FROM slots WHERE department_id=? AND date=? AND status='OPEN'`,
		deptID, date.Format("2006-01-02"))
	if err != nil {
		return nil, err
	}
	return r.drainAndAttachQuals(ctx, rows)
}

func (r *SlotRepo) UpdateStatus(ctx context.Context, id int64, status domain.SlotStatus) error {
	_, err := r.db.ExecContext(ctx, `UPDATE slots SET status=? WHERE id=?`, status, id)
	return err
}

func (r *SlotRepo) IncrAssigned(ctx context.Context, tx *sql.Tx, slotID int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE slots SET assigned_count=assigned_count+1,
		 status=CASE WHEN assigned_count+1>=required_staff THEN 'FILLED' ELSE status END
		 WHERE id=?`, slotID)
	return err
}

func (r *SlotRepo) DecrAssigned(ctx context.Context, tx *sql.Tx, slotID int64) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE slots SET assigned_count=MAX(0,assigned_count-1),
		 status=CASE WHEN status='FILLED' THEN 'OPEN' ELSE status END
		 WHERE id=?`, slotID)
	return err
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// drainAndAttachQuals reads all slot rows into memory (closing the cursor),
// then does one bulk query for all qualifications.
func (r *SlotRepo) drainAndAttachQuals(ctx context.Context, rows *sql.Rows) ([]*domain.Slot, error) {
	defer rows.Close()
	var list []*domain.Slot
	var ids  []int64
	for rows.Next() {
		s, err := scanSlot(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
		ids  = append(ids, s.ID)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return list, nil
	}

	// Bulk-fetch slot qualifications
	ph := strings.Repeat("?,", len(ids))
	ph = ph[:len(ph)-1]
	args := make([]interface{}, len(ids))
	for i, id := range ids { args[i] = id }

	qrows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT slot_id,qualification FROM slot_qualifications WHERE slot_id IN (%s)`, ph),
		args...)
	if err != nil {
		return nil, err
	}
	defer qrows.Close()

	qmap := make(map[int64][]domain.Qualification)
	for qrows.Next() {
		var sid int64
		var q   domain.Qualification
		if err := qrows.Scan(&sid, &q); err != nil {
			return nil, err
		}
		qmap[sid] = append(qmap[sid], q)
	}
	if err := qrows.Err(); err != nil {
		return nil, err
	}

	for _, s := range list {
		s.RequiredQuals = qmap[s.ID]
	}
	return list, nil
}

func (r *SlotRepo) getSlotQuals(ctx context.Context, slotID int64) ([]domain.Qualification, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT qualification FROM slot_qualifications WHERE slot_id=?`, slotID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var qs []domain.Qualification
	for rows.Next() {
		var q domain.Qualification
		if err := rows.Scan(&q); err != nil {
			return nil, err
		}
		qs = append(qs, q)
	}
	return qs, rows.Err()
}

func scanSlot(s scanner) (*domain.Slot, error) {
	var slot    domain.Slot
	var dateStr string
	err := s.Scan(&slot.ID, &slot.DepartmentID, &slot.ShiftTypeID,
		&dateStr, &slot.RequiredStaff, &slot.RequiredRole,
		&slot.Status, &slot.AssignedCount, &slot.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan slot: %w", err)
	}
	slot.Date, _ = time.Parse("2006-01-02", dateStr)
	return &slot, nil
}
