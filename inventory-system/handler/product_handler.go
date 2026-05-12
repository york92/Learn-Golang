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

type ProductHandler struct {
	svc *service.ProductService
}

func NewProductHandler(svc *service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

// POST /api/products
func (h *ProductHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req model.CreateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求体格式错误: "+err.Error())
		return
	}
	p, err := h.svc.Create(req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidParam) {
			response.BadRequest(w, err.Error())
		} else {
			response.InternalError(w, err.Error())
		}
		return
	}
	response.Created(w, p)
}

// GET /api/products
func (h *ProductHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	page, _ := strconv.Atoi(q.Get("page"))
	pageSize, _ := strconv.Atoi(q.Get("page_size"))
	req := model.ListProductsReq{
		Keyword:  q.Get("keyword"),
		Page:     page,
		PageSize: pageSize,
	}
	products, total, err := h.svc.List(req)
	if err != nil {
		response.InternalError(w, err.Error())
		return
	}
	if products == nil {
		products = []*model.Product{}
	}
	response.Success(w, response.PageData{
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		Items:    products,
	})
}

// GET /api/products/{id}
func (h *ProductHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		response.BadRequest(w, "无效的商品 ID")
		return
	}
	p, err := h.svc.GetByID(id)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			response.NotFound(w, err.Error())
		} else {
			response.InternalError(w, err.Error())
		}
		return
	}
	response.Success(w, p)
}

// PUT /api/products/{id}
func (h *ProductHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		response.BadRequest(w, "无效的商品 ID")
		return
	}
	var req model.UpdateProductReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "请求体格式错误: "+err.Error())
		return
	}
	p, err := h.svc.Update(id, req)
	if err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			response.NotFound(w, err.Error())
		} else {
			response.InternalError(w, err.Error())
		}
		return
	}
	response.Success(w, p)
}

// DELETE /api/products/{id}
func (h *ProductHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r, "id")
	if err != nil {
		response.BadRequest(w, "无效的商品 ID")
		return
	}
	if err := h.svc.Delete(id); err != nil {
		if errors.Is(err, service.ErrProductNotFound) {
			response.NotFound(w, err.Error())
		} else {
			response.InternalError(w, err.Error())
		}
		return
	}
	response.Success(w, map[string]string{"message": "删除成功"})
}
