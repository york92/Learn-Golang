package repository

import (
	"database/sql"
	"fmt"
	"inventory/model"
	"strings"
	"time"
)

type ProductRepo struct {
	db *sql.DB
}

func NewProductRepo(db *sql.DB) *ProductRepo {
	return &ProductRepo{db: db}
}

func (r *ProductRepo) Create(tx *sql.Tx, p *model.Product) (int64, error) {
	query := `INSERT INTO products (name, spec, price, stock, warning_threshold, version, created_at, updated_at)
	          VALUES (?, ?, ?, ?, ?, 0, ?, ?)`
	now := time.Now()
	var res sql.Result
	var err error
	if tx != nil {
		res, err = tx.Exec(query, p.Name, p.Spec, p.Price, p.Stock, p.WarningThreshold, now, now)
	} else {
		res, err = r.db.Exec(query, p.Name, p.Spec, p.Price, p.Stock, p.WarningThreshold, now, now)
	}
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (r *ProductRepo) GetByID(id int64) (*model.Product, error) {
	return r.getByIDWithDB(r.db, id)
}

func (r *ProductRepo) GetByIDForUpdate(tx *sql.Tx, id int64) (*model.Product, error) {
	// SQLite 无 SELECT FOR UPDATE，通过事务+乐观锁保证
	query := `SELECT id, name, spec, price, stock, warning_threshold, version, deleted, created_at, updated_at
	          FROM products WHERE id = ? AND deleted = 0`
	row := tx.QueryRow(query, id)
	return scanProduct(row)
}

func (r *ProductRepo) getByIDWithDB(db *sql.DB, id int64) (*model.Product, error) {
	query := `SELECT id, name, spec, price, stock, warning_threshold, version, deleted, created_at, updated_at
	          FROM products WHERE id = ? AND deleted = 0`
	row := db.QueryRow(query, id)
	return scanProduct(row)
}

func (r *ProductRepo) List(req model.ListProductsReq) ([]*model.Product, int64, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 || req.PageSize > 100 {
		req.PageSize = 20
	}

	where := "WHERE deleted = 0"
	args := []interface{}{}

	if req.Keyword != "" {
		where += " AND (name LIKE ? OR spec LIKE ?)"
		kw := "%" + req.Keyword + "%"
		args = append(args, kw, kw)
	}

	// count
	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM products %s", where)
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// data
	offset := (req.Page - 1) * req.PageSize
	listSQL := fmt.Sprintf(`SELECT id, name, spec, price, stock, warning_threshold, version, deleted, created_at, updated_at
	                        FROM products %s ORDER BY id DESC LIMIT ? OFFSET ?`, where)
	args = append(args, req.PageSize, offset)
	rows, err := r.db.Query(listSQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var products []*model.Product
	for rows.Next() {
		p, err := scanProductRow(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	return products, total, rows.Err()
}

// UpdateOptimistic 乐观锁更新库存（核心方法）
// 只有 version 匹配且 stock >= minStock 时才更新，返回 affected rows
func (r *ProductRepo) UpdateStockOptimistic(tx *sql.Tx, id, stockDelta, oldVersion, minStock int64) (int64, error) {
	query := `UPDATE products
	          SET stock = stock + ?, version = version + 1, updated_at = ?
	          WHERE id = ? AND version = ? AND stock >= ? AND deleted = 0`
	res, err := tx.Exec(query, stockDelta, time.Now(), id, oldVersion, minStock)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// UpdateInfo 更新商品基本信息（非库存字段）
func (r *ProductRepo) UpdateInfo(id int64, req model.UpdateProductReq) error {
	sets := []string{}
	args := []interface{}{}

	if req.Name != nil {
		sets = append(sets, "name = ?")
		args = append(args, *req.Name)
	}
	if req.Spec != nil {
		sets = append(sets, "spec = ?")
		args = append(args, *req.Spec)
	}
	if req.Price != nil {
		sets = append(sets, "price = ?")
		args = append(args, *req.Price)
	}
	if req.WarningThreshold != nil {
		sets = append(sets, "warning_threshold = ?")
		args = append(args, *req.WarningThreshold)
	}

	if len(sets) == 0 {
		return nil
	}
	sets = append(sets, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE products SET %s WHERE id = ? AND deleted = 0", strings.Join(sets, ", "))
	res, err := r.db.Exec(query, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// SoftDelete 软删除
func (r *ProductRepo) SoftDelete(id int64) error {
	res, err := r.db.Exec("UPDATE products SET deleted = 1, updated_at = ? WHERE id = ? AND deleted = 0", time.Now(), id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// helper
func scanProduct(row *sql.Row) (*model.Product, error) {
	p := &model.Product{}
	err := row.Scan(&p.ID, &p.Name, &p.Spec, &p.Price, &p.Stock,
		&p.WarningThreshold, &p.Version, &p.Deleted, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func scanProductRow(rows *sql.Rows) (*model.Product, error) {
	p := &model.Product{}
	err := rows.Scan(&p.ID, &p.Name, &p.Spec, &p.Price, &p.Stock,
		&p.WarningThreshold, &p.Version, &p.Deleted, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}
