package repository

import (
	"database/sql"
	"fmt"
	"inventory/model"
	"time"
)

type WarningRepo struct {
	db *sql.DB
}

func NewWarningRepo(db *sql.DB) *WarningRepo {
	return &WarningRepo{db: db}
}

// HasUnresolved 判断某商品是否已存在未处理的预警（防重复预警）
func (r *WarningRepo) HasUnresolved(productID int64) (bool, error) {
	var count int
	err := r.db.QueryRow(
		"SELECT COUNT(*) FROM stock_warnings WHERE product_id = ? AND is_resolved = 0", productID,
	).Scan(&count)
	return count > 0, err
}

// Create 新建预警记录
func (r *WarningRepo) Create(w *model.StockWarning) (int64, error) {
	query := `INSERT INTO stock_warnings (product_id, current_stock, threshold, is_resolved, triggered_at)
	          VALUES (?, ?, ?, 0, ?)`
	res, err := r.db.Exec(query, w.ProductID, w.CurrentStock, w.Threshold, time.Now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// Resolve 标记预警已处理
func (r *WarningRepo) Resolve(id int64) error {
	now := time.Now()
	res, err := r.db.Exec(
		"UPDATE stock_warnings SET is_resolved = 1, resolved_at = ? WHERE id = ? AND is_resolved = 0",
		now, id,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// List 查询预警列表
func (r *WarningRepo) List(onlyUnresolved bool, page, pageSize int) ([]*model.StockWarning, int64, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}

	where := "WHERE 1=1"
	args := []interface{}{}
	if onlyUnresolved {
		where += " AND w.is_resolved = 0"
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM stock_warnings w %s", where)
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	listSQL := fmt.Sprintf(`
		SELECT w.id, w.product_id, p.name, w.current_stock, w.threshold,
		       w.is_resolved, w.triggered_at, w.resolved_at
		FROM stock_warnings w
		LEFT JOIN products p ON w.product_id = p.id
		%s ORDER BY w.id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, pageSize, offset)

	rows, err := r.db.Query(listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var warnings []*model.StockWarning
	for rows.Next() {
		w := &model.StockWarning{}
		err := rows.Scan(&w.ID, &w.ProductID, &w.ProductName, &w.CurrentStock, &w.Threshold,
			&w.IsResolved, &w.TriggeredAt, &w.ResolvedAt)
		if err != nil {
			return nil, 0, err
		}
		warnings = append(warnings, w)
	}
	return warnings, total, rows.Err()
}
