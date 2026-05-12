package repository

import (
	"context"
	"fmt"

	"hospital-scheduler/internal/domain"
)

type DeptRepo struct{ db *DB }

func NewDeptRepo(db *DB) *DeptRepo { return &DeptRepo{db} }

func (r *DeptRepo) Create(ctx context.Context, d *domain.Department) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO departments(name,code,is_active) VALUES(?,?,1)`,
		d.Name, d.Code)
	if err != nil {
		return fmt.Errorf("insert dept: %w", err)
	}
	id, _ := res.LastInsertId()
	d.ID = id
	return nil
}

func (r *DeptRepo) GetByID(ctx context.Context, id int64) (*domain.Department, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,name,code,is_active FROM departments WHERE id=?`, id)
	var d domain.Department
	if err := row.Scan(&d.ID, &d.Name, &d.Code, &d.IsActive); err != nil {
		return nil, fmt.Errorf("get dept: %w", err)
	}
	return &d, nil
}

func (r *DeptRepo) ListAll(ctx context.Context) ([]*domain.Department, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,name,code,is_active FROM departments WHERE is_active=1 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.Department
	for rows.Next() {
		var d domain.Department
		if err := rows.Scan(&d.ID, &d.Name, &d.Code, &d.IsActive); err != nil {
			return nil, err
		}
		list = append(list, &d)
	}
	return list, rows.Err()
}

type ShiftTypeRepo struct{ db *DB }

func NewShiftTypeRepo(db *DB) *ShiftTypeRepo { return &ShiftTypeRepo{db} }

func (r *ShiftTypeRepo) GetByID(ctx context.Context, id int64) (*domain.ShiftType, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,code,name,start_hour,start_minute,end_hour,end_minute
		 FROM shift_types WHERE id=?`, id)
	var s domain.ShiftType
	if err := row.Scan(&s.ID, &s.Code, &s.Name, &s.StartHour, &s.StartMinute, &s.EndHour, &s.EndMinute); err != nil {
		return nil, fmt.Errorf("get shift type: %w", err)
	}
	return &s, nil
}

func (r *ShiftTypeRepo) GetByCode(ctx context.Context, code domain.ShiftTypeCode) (*domain.ShiftType, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,code,name,start_hour,start_minute,end_hour,end_minute
		 FROM shift_types WHERE code=?`, code)
	var s domain.ShiftType
	if err := row.Scan(&s.ID, &s.Code, &s.Name, &s.StartHour, &s.StartMinute, &s.EndHour, &s.EndMinute); err != nil {
		return nil, fmt.Errorf("get shift type by code: %w", err)
	}
	return &s, nil
}

func (r *ShiftTypeRepo) ListAll(ctx context.Context) ([]*domain.ShiftType, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,code,name,start_hour,start_minute,end_hour,end_minute FROM shift_types`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.ShiftType
	for rows.Next() {
		var s domain.ShiftType
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.StartHour, &s.StartMinute, &s.EndHour, &s.EndMinute); err != nil {
			return nil, err
		}
		list = append(list, &s)
	}
	return list, rows.Err()
}

func (r *ShiftTypeRepo) Create(ctx context.Context, s *domain.ShiftType) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO shift_types(code,name,start_hour,start_minute,end_hour,end_minute)
		 VALUES(?,?,?,?,?,?)`,
		s.Code, s.Name, s.StartHour, s.StartMinute, s.EndHour, s.EndMinute)
	if err != nil {
		return fmt.Errorf("insert shift type: %w", err)
	}
	id, _ := res.LastInsertId()
	s.ID = id
	return nil
}
