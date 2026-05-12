package response

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PageData struct {
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
	Items    interface{} `json:"items"`
}

func JSON(w http.ResponseWriter, statusCode int, code int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(Response{Code: code, Message: message, Data: data})
}

func Success(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, 0, "success", data)
}

func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, 0, "created", data)
}

func BadRequest(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusBadRequest, 400, msg, nil)
}

func NotFound(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusNotFound, 404, msg, nil)
}

func Conflict(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusConflict, 409, msg, nil)
}

func InternalError(w http.ResponseWriter, msg string) {
	JSON(w, http.StatusInternalServerError, 500, msg, nil)
}
