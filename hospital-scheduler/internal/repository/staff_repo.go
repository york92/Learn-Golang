package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"hospital-scheduler/internal/domain"
)

type StaffRepo struct{ db *DB }

func NewStaffRepo(db *DB) *StaffRepo { return &StaffRepo{db} }

func (r *StaffRepo) Create(ctx context.Context, s *domain.Staff) error {
	return r.db.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.ExecContext(ctx,
			`INSERT INTO staff(employee_no,name,role,department_id,is_active)
			 VALUES(?,?,?,?,1)`,
			s.EmployeeNo, s.Name, s.Role, s.DepartmentID)
		if err != nil {
			return fmt.Errorf("insert staff: %w", err)
		}
		id, _ := res.LastInsertId()
		s.ID = id

		for _, q := range s.Qualifications {
			if _, err = tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO staff_qualifications(staff_id,qualification) VALUES(?,?)`,
				id, q); err != nil {
				return fmt.Errorf("insert qualification: %w", err)
			}
		}
		_, err = tx.ExecContext(ctx,
			`INSERT OR IGNORE INTO workload_accounts(staff_id) VALUES(?)`, id)
		return err
	})
}

func (r *StaffRepo) GetByID(ctx context.Context, id int64) (*domain.Staff, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,employee_no,name,role,department_id,is_active,created_at
		 FROM staff WHERE id=?`, id)
	s, err := scanStaff(row)
	if err != nil {
		return nil, err
	}
	// quals on same connection — row is already closed, safe to reuse
	s.Qualifications, err = r.getQuals(ctx, id)
	return s, err
}

// ListByDepartment fetches all staff then bulk-loads qualifications.
// Avoids nested query deadlock on single-connection SQLite.
func (r *StaffRepo) ListByDepartment(ctx context.Context, deptID int64) ([]*domain.Staff, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,employee_no,name,role,department_id,is_active,created_at
		 FROM staff WHERE department_id=? AND is_active=1 ORDER BY name`, deptID)
	if err != nil {
		return nil, err
	}
	list, ids, err := drainStaff(rows)
	if err != nil {
		return nil, err
	}
	return r.attachQuals(ctx, list, ids)
}

func (r *StaffRepo) ListAll(ctx context.Context) ([]*domain.Staff, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,employee_no,name,role,department_id,is_active,created_at
		 FROM staff WHERE is_active=1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	list, ids, err := drainStaff(rows)
	if err != nil {
		return nil, err
	}
	return r.attachQuals(ctx, list, ids)
}

// FindCandidates returns staff matching role + all required qualifications.
func (r *StaffRepo) FindCandidates(ctx context.Context, role domain.Role, quals []domain.Qualification) ([]*domain.Staff, error) {
	var rows *sql.Rows
	var err error

	if len(quals) == 0 {
		rows, err = r.db.QueryContext(ctx,
			`SELECT id,employee_no,name,role,department_id,is_active,created_at
			 FROM staff WHERE role=? AND is_active=1`, role)
	} else {
		placeholders := strings.Repeat("?,", len(quals))
		placeholders = placeholders[:len(placeholders)-1]
		args := []interface{}{role}
		for _, q := range quals {
			args = append(args, q)
		}
		args = append(args, len(quals))
		query := fmt.Sprintf(`
			SELECT s.id,s.employee_no,s.name,s.role,s.department_id,s.is_active,s.created_at
			FROM staff s
			WHERE s.role=? AND s.is_active=1
			  AND (SELECT COUNT(*) FROM staff_qualifications sq
			       WHERE sq.staff_id=s.id AND sq.qualification IN (%s)) = ?`,
			placeholders)
		rows, err = r.db.QueryContext(ctx, query, args...)
	}
	if err != nil {
		return nil, err
	}
	list, ids, err := drainStaff(rows)
	if err != nil {
		return nil, err
	}
	return r.attachQuals(ctx, list, ids)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// drainStaff reads all rows into memory and closes the cursor before returning.
func drainStaff(rows *sql.Rows) ([]*domain.Staff, []int64, error) {
	defer rows.Close()
	var list []*domain.Staff
	var ids  []int64
	for rows.Next() {
		s, err := scanStaff(rows)
		if err != nil {
			return nil, nil, err
		}
		list = append(list, s)
		ids  = append(ids, s.ID)
	}
	return list, ids, rows.Err()
}

// attachQuals does a single bulk IN query for qualifications, then maps them.
// Called only after the staff cursor is fully closed.
func (r *StaffRepo) attachQuals(ctx context.Context, list []*domain.Staff, ids []int64) ([]*domain.Staff, error) {
	if len(ids) == 0 {
		return list, nil
	}

	// Build IN clause
	ph := strings.Repeat("?,", len(ids))
	ph = ph[:len(ph)-1]
	args := make([]interface{}, len(ids))
	for i, id := range ids { args[i] = id }

	rows, err := r.db.QueryContext(ctx,
		fmt.Sprintf(`SELECT staff_id,qualification FROM staff_qualifications WHERE staff_id IN (%s)`, ph),
		args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	qmap := make(map[int64][]domain.Qualification)
	for rows.Next() {
		var sid int64
		var q   domain.Qualification
		if err := rows.Scan(&sid, &q); err != nil {
			return nil, err
		}
		qmap[sid] = append(qmap[sid], q)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for _, s := range list {
		s.Qualifications = qmap[s.ID] // nil → empty slice is fine
	}
	return list, nil
}

// getQuals fetches qualifications for a single staff (used after GetByID).
func (r *StaffRepo) getQuals(ctx context.Context, staffID int64) ([]domain.Qualification, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT qualification FROM staff_qualifications WHERE staff_id=?`, staffID)
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

type scanner interface {
	Scan(dest ...interface{}) error
}

func scanStaff(s scanner) (*domain.Staff, error) {
	var st domain.Staff
	err := s.Scan(&st.ID, &st.EmployeeNo, &st.Name, &st.Role,
		&st.DepartmentID, &st.IsActive, &st.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan staff: %w", err)
	}
	return &st, nil
}
