package handler

import (
	"encoding/json"
	"errors"
	"inventory/model"
	"inventory/pkg/response"
	"inventory/service"
	"net/http"
	"strconv"
)

type StockHandler struct {
	svc *service.StockService
}

func NewStockHandler(svc *service.StockService) *StockHandler {
	return &StockHandler{svc: svc}
}

// POST /api/stock/in
func (h *StockHandler) StockIn(w http.ResponseWriter, r *http.Request) {
	h.operate(w, r, model.ChangeTypeIn)
}

// POST /api/stock/out
func (h *StockHandler) StockOut(w http.ResponseWriter, r *http.Request) {
	h.operate(w, r, model.ChangeTypeOut)
}

// POST /api/stock/loss
func (h *StockHandler) StockLoss(w http.ResponseWriter, r *http.Request) {
	h.operate(w, r, model.ChangeTypeLoss)
}

func (h *StockHandler) operate(w http.ResponseWriter, r *http.Request, forceType model.ChangeType) {
	var req model.StockOperationReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求体格式错误: "+err.Error())
		return
	}
	req.ChangeType = forceType // 由路由决定类型，不信任 body 中的 change_type

	stockLog, err := h.svc.Operate(req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidOperation):
			response.BadRequest(w, err.Error())
		case errors.Is(err, service.ErrProductNotFound):
			response.NotFound(w, err.Error())
		case errors.Is(err, service.ErrInsufficientStock):
			response.BadRequest(w, err.Error())
		case errors.Is(err, service.ErrConcurrentConflict):
			response.Conflict(w, err.Error())
		default:
			response.InternalError(w, err.Error())
		}
		return
	}
	response.Success(w, stockLog)
}

// GET /api/logs
func (h *StockHandler) ListLogs(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	productID, _ := strconv.ParseInt(q.Get("product_id"), 10, 64)
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))

	req := model.ListLogsReq{
		ProductID:  productID,
		ChangeType: model.ChangeType(q.Get("change_type")),
		StartTime:  q.Get("start_time"),
		EndTime:    q.Get("end_time"),
		Page:       page,
		PageSize:   pageSize,
	}

	logs, total, err := h.svc.GetLogs(req)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if logs == nil {
		logs = []*model.StockLog{}
	}
	response.Success(w, response.PageData{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    logs,
	})
}
