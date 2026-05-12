package repository

import (
	"database/sql"
	"fmt"
	"inventory/model"
	"time"
)

type LogRepo struct {
	db *sql.DB
}

func NewLogRepo(db *sql.DB) *LogRepo {
	return &LogRepo{db: db}
}

// Create 写入流水（在事务中调用）
func (r *LogRepo) Create(tx *sql.Tx, log *model.StockLog) (int64, error) {
	query := `INSERT INTO stock_logs (product_id, change_type, quantity, before_stock, after_stock, remark, operator, created_at)
	          VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	res, err := tx.Exec(query, log.ProductID, log.ChangeType, log.Quantity,
		log.BeforeStock, log.AfterStock, log.Remark, log.Operator, now)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// List 查询流水列表（联查商品名称）
func (r *LogRepo) List(req model.ListLogsReq) ([]*model.StockLog, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 200 {
		req.PageSize = 50
	}

	where := "WHERE 1=1"
	args := []interface{}{}

	if req.ProductID > 0 {
		where += " AND l.product_id = ?"
		args = append(args, req.ProductID)
	}
	if req.ChangeType != "" {
		where += " AND l.change_type = ?"
		args = append(args, string(req.ChangeType))
	}
	if req.StartTime != "" {
		where += " AND l.created_at >= ?"
		args = append(args, req.StartTime)
	}
	if req.EndTime != "" {
		where += " AND l.created_at <= ?"
		args = append(args, req.EndTime)
	}

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM stock_logs l %s", where)
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (req.Page - 1) * req.PageSize
	listSQL := fmt.Sprintf(`
		SELECT l.id, l.product_id, p.name, l.change_type, l.quantity,
		       l.before_stock, l.after_stock, l.remark, l.operator, l.created_at
		FROM stock_logs l
		LEFT JOIN products p ON l.product_id = p.id
		%s ORDER BY l.id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, req.PageSize, offset)

	rows, err := r.db.Query(listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var logs []*model.StockLog
	for rows.Next() {
		l := &model.StockLog{}
		err := rows.Scan(&l.ID, &l.ProductID, &l.ProductName, &l.ChangeType,
			&l.Quantity, &l.BeforeStock, &l.AfterStock, &l.Remark, &l.Operator, &l.CreatedAt)
		if err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, total, rows.Err()
}
