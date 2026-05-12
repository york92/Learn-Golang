package service

import (
	"database/sql"
	"errors"
	"fmt"
	"inventory/model"
	"inventory/pkg/notify"
	"inventory/repository"
	"log"
)

var (
	ErrInsufficientStock = errors.New("库存不足")
	ErrConcurrentConflict = errors.New("并发冲突，请重试")
	ErrInvalidOperation  = errors.New("操作参数无效")
)

// warningEvent 预警触发事件
type warningEvent struct {
	productID    int64
	productName  string
	currentStock int64
	threshold    int64
}

type StockService struct {
	db          *sql.DB
	productRepo *repository.ProductRepo
	logRepo     *repository.LogRepo
	warningRepo *repository.WarningRepo
	notifier    notify.Notifier
	warnCh      chan warningEvent
}

func NewStockService(
	db *sql.DB,
	productRepo *repository.ProductRepo,
	logRepo *repository.LogRepo,
	warningRepo *repository.WarningRepo,
	notifier notify.Notifier,
) *StockService {
	s := &StockService{
		db:          db,
		productRepo: productRepo,
		logRepo:     logRepo,
		warningRepo: warningRepo,
		notifier:    notifier,
		warnCh:      make(chan warningEvent, 128),
	}
	go s.processWarnings()
	return s
}

// Operate 执行库存操作（入库/出库/损耗）
// 核心流程：事务 + 乐观锁，最多重试 3 次
func (s *StockService) Operate(req model.StockOperationReq) (*model.StockLog, error) {
	// 参数校验
	if req.ProductID <= 0 {
		return nil, fmt.Errorf("%w: product_id 无效", ErrInvalidOperation)
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("%w: quantity 必须大于 0", ErrInvalidOperation)
	}
	if req.ChangeType != model.ChangeTypeIn &&
		req.ChangeType != model.ChangeTypeOut &&
		req.ChangeType != model.ChangeTypeLoss {
		return nil, fmt.Errorf("%w: change_type 无效", ErrInvalidOperation)
	}

	const maxRetry = 3
	for attempt := 1; attempt <= maxRetry; attempt++ {
		stockLog, err := s.doOperate(req)
		if err == nil {
			return stockLog, nil
		}
		if errors.Is(err, ErrConcurrentConflict) {
			log.Printf("[STOCK] 乐观锁冲突，第 %d 次重试 (product_id=%d)", attempt, req.ProductID)
			continue
		}
		// 其它错误（库存不足、商品不存在）直接返回
		return nil, err
	}
	return nil, fmt.Errorf("操作失败：连续 %d 次并发冲突", maxRetry)
}

func (s *StockService) doOperate(req model.StockOperationReq) (*model.StockLog, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	// 1. 在事务中读取商品（含 version）
	product, err := s.productRepo.GetByIDForUpdate(tx, req.ProductID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			err = ErrProductNotFound
		}
		return nil, err
	}

	// 2. 计算库存变动
	beforeStock := product.Stock
	var stockDelta int64
	var minStock int64 // UPDATE WHERE stock >= minStock

	switch req.ChangeType {
	case model.ChangeTypeIn:
		stockDelta = req.Quantity
		minStock = 0
	case model.ChangeTypeOut, model.ChangeTypeLoss:
		if product.Stock < req.Quantity {
			err = fmt.Errorf("%w：当前库存 %d，需要 %d", ErrInsufficientStock, product.Stock, req.Quantity)
			return nil, err
		}
		stockDelta = -req.Quantity
		minStock = req.Quantity // 确保扣减后 >= 0
	}

	afterStock := beforeStock + stockDelta

	// 3. 乐观锁更新库存
	affected, err := s.productRepo.UpdateStockOptimistic(tx, req.ProductID, stockDelta, product.Version, minStock)
	if err != nil {
		return nil, fmt.Errorf("update stock: %w", err)
	}
	if affected == 0 {
		// version 被其他并发请求改变，或库存在并发中已不足
		err = ErrConcurrentConflict
		return nil, err
	}

	// 4. 写流水（append-only）
	stockLog := &model.StockLog{
		ProductID:   req.ProductID,
		ChangeType:  req.ChangeType,
		Quantity:    req.Quantity,
		BeforeStock: beforeStock,
		AfterStock:  afterStock,
		Remark:      req.Remark,
		Operator:    req.Operator,
	}
	logID, err := s.logRepo.Create(tx, stockLog)
	if err != nil {
		return nil, fmt.Errorf("create log: %w", err)
	}
	stockLog.ID = logID

	// 5. 提交事务
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	// 6. 异步触发预警检查（不影响主流程）
	if product.WarningThreshold > 0 && afterStock <= product.WarningThreshold {
		select {
		case s.warnCh <- warningEvent{
			productID:    req.ProductID,
			productName:  product.Name,
			currentStock: afterStock,
			threshold:    product.WarningThreshold,
		}:
		default:
			log.Printf("[WARN] 预警 channel 已满，跳过 product_id=%d", req.ProductID)
		}
	}

	return stockLog, nil
}

// processWarnings 后台 goroutine 消费预警事件
func (s *StockService) processWarnings() {
	for event := range s.warnCh {
		s.handleWarning(event)
	}
}

func (s *StockService) handleWarning(e warningEvent) {
	// 防重复：同一商品未处理预警存在则跳过
	has, err := s.warningRepo.HasUnresolved(e.productID)
	if err != nil {
		log.Printf("[WARN] check unresolved error: %v", err)
		return
	}
	if has {
		return
	}

	// 写预警记录
	w := &model.StockWarning{
		ProductID:    e.productID,
		CurrentStock: e.currentStock,
		Threshold:    e.threshold,
	}
	if _, err := s.warningRepo.Create(w); err != nil {
		log.Printf("[WARN] create warning error: %v", err)
		return
	}

	// 发送通知
	s.notifier.SendWarning(e.productID, e.productName, e.currentStock, e.threshold)
}

// GetLogs 查询流水
func (s *StockService) GetLogs(req model.ListLogsReq) ([]*model.StockLog, int64, error) {
	return s.logRepo.List(req)
}
