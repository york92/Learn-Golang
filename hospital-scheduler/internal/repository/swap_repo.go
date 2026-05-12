package repository

import (
	"context"
	"fmt"
	"time"

	"hospital-scheduler/internal/domain"
)

type SwapRepo struct{ db *DB }

func NewSwapRepo(db *DB) *SwapRepo { return &SwapRepo{db} }

func (r *SwapRepo) Create(ctx context.Context, s *domain.SwapRequest) error {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO swap_requests(requester_id,requester_slot_id,target_staff_id,reason,status)
		 VALUES(?,?,?,?,?)`,
		s.RequesterID, s.RequesterSlotID, s.TargetStaffID, s.Reason, domain.SwapPending)
	if err != nil {
		return fmt.Errorf("insert swap: %w", err)
	}
	id, _ := res.LastInsertId()
	s.ID = id
	s.CreatedAt = time.Now()
	s.Status = domain.SwapPending
	return nil
}

func (r *SwapRepo) GetByID(ctx context.Context, id int64) (*domain.SwapRequest, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id,requester_id,requester_slot_id,target_staff_id,reason,status,
		        review_note,reviewed_by,created_at,updated_at
		 FROM swap_requests WHERE id=?`, id)
	return r.scanSwap(row)
}

func (r *SwapRepo) ListPending(ctx context.Context) ([]*domain.SwapRequest, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,requester_id,requester_slot_id,target_staff_id,reason,status,
		        review_note,reviewed_by,created_at,updated_at
		 FROM swap_requests WHERE status='PENDING' ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var list []*domain.SwapRequest
	for rows.Next() {
		s, err := r.scanSwap(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, s)
	}
	return list, rows.Err()
}

func (r *SwapRepo) Approve(ctx context.Context, id int64, reviewerID int64, note string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE swap_requests SET status='APPROVED',review_note=?,reviewed_by=?,
		 updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		note, reviewerID, id)
	return err
}

func (r *SwapRepo) Reject(ctx context.Context, id int64, reviewerID int64, note string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE swap_requests SET status='REJECTED',review_note=?,reviewed_by=?,
		 updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		note, reviewerID, id)
	return err
}

func (r *SwapRepo) scanSwap(s scanner) (*domain.SwapRequest, error) {
	var req domain.SwapRequest
	var targetStaffID *int64
	var reviewedBy *int64
	err := s.Scan(&req.ID, &req.RequesterID, &req.RequesterSlotID,
		&targetStaffID, &req.Reason, &req.Status,
		&req.ReviewNote, &reviewedBy,
		&req.CreatedAt, &req.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan swap: %w", err)
	}
	req.TargetStaffID = targetStaffID
	req.ReviewedBy = reviewedBy
	return &req, nil
}
