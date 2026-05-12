package handler

import (
	"inventory/pkg/response"
	"inventory/service"
	"net/http"
	"strconv"
)

type WarningHandler struct {
	svc *service.WarningService
}

func NewWarningHandler(svc *service.WarningService) *WarningHandler {
	return &WarningHandler{svc: svc}
}

// GET /api/warnings
func (h *WarningHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	onlyUnresolved := q.Get("only_unresolved") == "true" || q.Get("only_unresolved") == "1"
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))

	warnings, total, err := h.svc.List(onlyUnresolved, page, pageSize)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	response.Success(w, response.PageData{
		Total:    total,
		Page:     page,
		PageSize: pageSize,
		Items:    warnings,
	})
}

// PUT /api/warnings/{id}/resolve
func (h *WarningHandler) Resolve(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		response.BadRequest(w, "无效的预警 ID")
		return
	}
	if err := h.svc.Resolve(id); err != nil {
		response.BadRequest(w, err.Error())
		return
	}
	response.Success(w, map[string]string{"message": "已标记为处理完成"})
}
