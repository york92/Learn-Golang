package repository

import (
	"context"
	"fmt"

	"hospital-scheduler/internal/domain"
)

type AuditRepo struct{ db *DB }

func NewAuditRepo(db *DB) *AuditRepo { return &AuditRepo{db} }

func (r *AuditRepo) Log(ctx context.Context, log *domain.AuditLog) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO audit_logs(entity_type,entity_id,action,old_value,new_value,operator_id)
		 VALUES(?,?,?,?,?,?)`,
		log.EntityType, log.EntityID, log.Action,
		log.OldValue, log.NewValue, log.OperatorID)
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func (r *AuditRepo) ListByEntity(ctx context.Context, entityType string, entityID int64) ([]*domain.AuditLog, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT id,entity_type,entity_id,action,old_value,new_value,operator_id,created_at
		 FROM audit_logs WHERE entity_type=? AND entity_id=? ORDER BY created_at DESC`,
		entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []*domain.AuditLog
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(&l.ID, &l.EntityType, &l.EntityID, &l.Action,
			&l.OldValue, &l.NewValue, &l.OperatorID, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, &l)
	}
	return logs, rows.Err()
}
