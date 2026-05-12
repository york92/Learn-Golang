package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"hospital-scheduler/internal/domain"
)

type WorkloadRepo struct{ db *DB }

func NewWorkloadRepo(db *DB) *WorkloadRepo { return &WorkloadRepo{db} }

func (r *WorkloadRepo) Get(ctx context.Context, staffID int64) (*domain.WorkloadAccount, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT staff_id,total_hours,month_hours,week_hours,consecutive_shifts,
		        last_shift_end,night_shifts_this_month,updated_at
		 FROM workload_accounts WHERE staff_id=?`, staffID)

	var w domain.WorkloadAccount
	var lastEnd sql.NullString
	err := row.Scan(&w.StaffID, &w.TotalHours, &w.MonthHours, &w.WeekHours,
		&w.ConsecutiveShifts, &lastEnd, &w.NightShiftsThisMonth, &w.UpdatedAt)
	if err == sql.ErrNoRows {
		// Return empty account
		return &domain.WorkloadAccount{StaffID: staffID}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan workload: %w", err)
	}

	if lastEnd.Valid && lastEnd.String != "" {
		t, err := time.Parse("2006-01-02 15:04:05", lastEnd.String)
		if err == nil {
			w.LastShiftEnd = &t
		}
	}
	return &w, nil
}

// AddHours adds shift hours to all relevant accumulators
func (r *WorkloadRepo) AddShift(ctx context.Context, tx *sql.Tx, staffID int64,
	hours float64, shiftEnd time.Time, isNight bool) error {

	nightAdd := 0
	if isNight {
		nightAdd = 1
	}

	_, err := tx.ExecContext(ctx, `
		INSERT INTO workload_accounts(staff_id,total_hours,month_hours,week_hours,
		    consecutive_shifts,last_shift_end,night_shifts_this_month,updated_at)
		VALUES(?,?,?,?,1,?,?,CURRENT_TIMESTAMP)
		ON CONFLICT(staff_id) DO UPDATE SET
		    total_hours             = total_hours + excluded.total_hours,
		    month_hours             = month_hours + excluded.month_hours,
		    week_hours              = week_hours  + excluded.week_hours,
		    consecutive_shifts      = consecutive_shifts + 1,
		    last_shift_end          = excluded.last_shift_end,
		    night_shifts_this_month = night_shifts_this_month + excluded.night_shifts_this_month,
		    updated_at              = CURRENT_TIMESTAMP`,
		staffID, hours, hours, hours,
		shiftEnd.Format("2006-01-02 15:04:05"),
		nightAdd)
	return err
}

// ResetWeekly resets weekly hours (call at start of each week via cron)
func (r *WorkloadRepo) ResetWeekly(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE workload_accounts SET week_hours=0, updated_at=CURRENT_TIMESTAMP`)
	return err
}

// ResetMonthly resets monthly counters
func (r *WorkloadRepo) ResetMonthly(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE workload_accounts SET month_hours=0, night_shifts_this_month=0,
		 updated_at=CURRENT_TIMESTAMP`)
	return err
}

// GetDeptAvgHours returns the average monthly hours for a department (for fairness check)
func (r *WorkloadRepo) GetDeptAvgHours(ctx context.Context, deptID int64) (float64, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT AVG(wa.month_hours)
		 FROM workload_accounts wa
		 JOIN staff s ON s.id=wa.staff_id
		 WHERE s.department_id=? AND s.is_active=1`, deptID)
	var avg sql.NullFloat64
	if err := row.Scan(&avg); err != nil {
		return 0, err
	}
	if !avg.Valid {
		return 0, nil
	}
	return avg.Float64, nil
}

// ListSortedByWorkload returns staff sorted by month_hours ascending (least-loaded first)
func (r *WorkloadRepo) ListSortedByWorkload(ctx context.Context, staffIDs []int64) ([]int64, error) {
	if len(staffIDs) == 0 {
		return nil, nil
	}

	// Build query with placeholders
	args := make([]interface{}, len(staffIDs))
	placeholders := ""
	for i, id := range staffIDs {
		args[i] = id
		if i > 0 {
			placeholders += ","
		}
		placeholders += "?"
	}

	rows, err := r.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT staff_id FROM workload_accounts
		 WHERE staff_id IN (%s) ORDER BY month_hours ASC`, placeholders), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sorted []int64
	seen := make(map[int64]bool)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		sorted = append(sorted, id)
		seen[id] = true
	}
	// Append any staff without workload records
	for _, id := range staffIDs {
		if !seen[id] {
			sorted = append(sorted, id)
		}
	}
	return sorted, rows.Err()
}
