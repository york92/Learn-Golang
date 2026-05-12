package model

import "time"

// ChangeType 库存变动类型
type ChangeType string

const (
	ChangeTypeIn   ChangeType = "IN"   // 入库
	ChangeTypeOut  ChangeType = "OUT"  // 出库
	ChangeTypeLoss ChangeType = "LOSS" // 损耗
)

// Product 商品
type Product struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	Spec             string    `json:"spec"`
	Price            float64   `json:"price"`
	Stock            int64     `json:"stock"`
	WarningThreshold int64     `json:"warning_threshold"`
	Version          int64     `json:"version"`
	Deleted          bool      `json:"deleted"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// StockLog 库存流水（只追加，不修改不删除）
type StockLog struct {
	ID          int64      `json:"id"`
	ProductID   int64      `json:"product_id"`
	ProductName string     `json:"product_name,omitempty"` // 联查冗余
	ChangeType  ChangeType `json:"change_type"`
	Quantity    int64      `json:"quantity"`
	BeforeStock int64      `json:"before_stock"`
	AfterStock  int64      `json:"after_stock"`
	Remark      string     `json:"remark"`
	Operator    string     `json:"operator"`
	CreatedAt   time.Time  `json:"created_at"`
}

// StockWarning 预警记录
type StockWarning struct {
	ID           int64     `json:"id"`
	ProductID    int64     `json:"product_id"`
	ProductName  string    `json:"product_name,omitempty"`
	CurrentStock int64     `json:"current_stock"`
	Threshold    int64     `json:"threshold"`
	IsResolved   bool      `json:"is_resolved"`
	TriggeredAt  time.Time `json:"triggered_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// ---- 请求/响应 DTO ----

type CreateProductReq struct {
	Name             string  `json:"name"`
	Spec             string  `json:"spec"`
	Price            float64 `json:"price"`
	InitialStock     int64   `json:"initial_stock"`
	WarningThreshold int64   `json:"warning_threshold"`
}

type UpdateProductReq struct {
	Name             *string  `json:"name"`
	Spec             *string  `json:"spec"`
	Price            *float64 `json:"price"`
	WarningThreshold *int64   `json:"warning_threshold"`
}

type StockOperationReq struct {
	ProductID  int64      `json:"product_id"`
	ChangeType ChangeType `json:"change_type"`
	Quantity   int64      `json:"quantity"`
	Remark     string     `json:"remark"`
	Operator   string     `json:"operator"`
}

type ListLogsReq struct {
	ProductID  int64      `form:"product_id"`
	ChangeType ChangeType `form:"change_type"`
	StartTime  string     `form:"start_time"`
	EndTime    string     `form:"end_time"`
	Page       int        `form:"page"`
	PageSize   int        `form:"page_size"`
}

type ListProductsReq struct {
	Keyword  string `form:"keyword"`
	Page     int    `form:"page"`
	PageSize int    `form:"page_size"`
}
