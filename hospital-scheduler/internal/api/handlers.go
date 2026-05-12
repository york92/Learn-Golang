package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"hospital-scheduler/internal/domain"
	"hospital-scheduler/internal/repository"
	"hospital-scheduler/internal/service"
)

type Handler struct {
	svc           *service.ScheduleService
	staffRepo     *repository.StaffRepo
	slotRepo      *repository.SlotRepo
	deptRepo      *repository.DeptRepo
	shiftTypeRepo *repository.ShiftTypeRepo
}

func NewHandler(
	svc *service.ScheduleService,
	staffRepo *repository.StaffRepo,
	slotRepo *repository.SlotRepo,
	deptRepo *repository.DeptRepo,
	shiftTypeRepo *repository.ShiftTypeRepo,
) *Handler {
	return &Handler{svc, staffRepo, slotRepo, deptRepo, shiftTypeRepo}
}

// ─── Departments ──────────────────────────────────────────────────────────────

func (h *Handler) ListDepartments(w http.ResponseWriter, r *http.Request) {
	depts, err := h.deptRepo.ListAll(r.Context())
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, depts)
}

func (h *Handler) CreateDepartment(w http.ResponseWriter, r *http.Request) {
	var d domain.Department
	if err := json.NewDecoder(r.Body).Decode(&d); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}
	if err := h.deptRepo.Create(r.Context(), &d); err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 201, d)
}

// ─── Shift Types ──────────────────────────────────────────────────────────────

func (h *Handler) ListShiftTypes(w http.ResponseWriter, r *http.Request) {
	types, err := h.shiftTypeRepo.ListAll(r.Context())
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, types)
}

// ─── Staff ────────────────────────────────────────────────────────────────────

func (h *Handler) ListStaff(w http.ResponseWriter, r *http.Request) {
	deptIDStr := r.URL.Query().Get("department_id")
	ctx := r.Context()

	var staff interface{}
	var err error

	if deptIDStr != "" {
		deptID, _ := strconv.ParseInt(deptIDStr, 10, 64)
		staff, err = h.staffRepo.ListByDepartment(ctx, deptID)
	} else {
		staff, err = h.staffRepo.ListAll(ctx)
	}

	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, staff)
}

func (h *Handler) CreateStaff(w http.ResponseWriter, r *http.Request) {
	var s domain.Staff
	if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}
	if err := h.staffRepo.Create(r.Context(), &s); err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 201, s)
}

func (h *Handler) GetStaff(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	s, err := h.staffRepo.GetByID(r.Context(), id)
	if err != nil {
		JSONErr(w, 404, "staff not found")
		return
	}
	JSON(w, 200, s)
}

// ─── Slots ────────────────────────────────────────────────────────────────────

func (h *Handler) CreateSlot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DepartmentID  int64                  `json:"department_id"`
		ShiftTypeID   int64                  `json:"shift_type_id"`
		Date          string                 `json:"date"`
		RequiredStaff int                    `json:"required_staff"`
		RequiredRole  domain.Role            `json:"required_role"`
		RequiredQuals []domain.Qualification `json:"required_quals"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}

	date, err := time.Parse("2006-01-02", req.Date)
	if err != nil {
		JSONErr(w, 400, "invalid date format, expected YYYY-MM-DD")
		return
	}

	slot := &domain.Slot{
		DepartmentID:  req.DepartmentID,
		ShiftTypeID:   req.ShiftTypeID,
		Date:          date,
		RequiredStaff: req.RequiredStaff,
		RequiredRole:  req.RequiredRole,
		RequiredQuals: req.RequiredQuals,
	}
	if slot.RequiredStaff == 0 {
		slot.RequiredStaff = 1
	}

	if err := h.slotRepo.Create(r.Context(), slot); err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 201, slot)
}

func (h *Handler) ListSlots(w http.ResponseWriter, r *http.Request) {
	deptID, _ := strconv.ParseInt(r.URL.Query().Get("department_id"), 10, 64)
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	from := time.Now().Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, 7)

	if fromStr != "" {
		from, _ = time.Parse("2006-01-02", fromStr)
	}
	if toStr != "" {
		to, _ = time.Parse("2006-01-02", toStr)
	}

	slots, err := h.svc.GetSchedule(r.Context(), deptID, from, to)
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, slots)
}

// ─── Assignments ──────────────────────────────────────────────────────────────

func (h *Handler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StaffID   int64  `json:"staff_id"`
		SlotID    int64  `json:"slot_id"`
		Note      string `json:"note"`
		CreatedBy int64  `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}

	resp, err := h.svc.Assign(r.Context(), service.AssignRequest{
		StaffID:   req.StaffID,
		SlotID:    req.SlotID,
		Source:    domain.SourceManual,
		Note:      req.Note,
		CreatedBy: req.CreatedBy,
	})
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}

	if len(resp.Violations) > 0 {
		JSON(w, 422, map[string]interface{}{
			"blocked":    true,
			"violations": resp.Violations,
		})
		return
	}

	JSON(w, 201, map[string]interface{}{
		"assignment": resp.Assignment,
		"warnings":   resp.Warnings,
	})
}

func (h *Handler) CancelAssignment(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err := h.svc.CancelAssignment(r.Context(), id, 0); err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSONMsg(w, 200, "assignment canceled")
}

// ─── Auto Schedule ────────────────────────────────────────────────────────────

func (h *Handler) AutoSchedule(w http.ResponseWriter, r *http.Request) {
	var req struct {
		DepartmentID int64  `json:"department_id"`
		From         string `json:"from"`
		To           string `json:"to"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}

	from, err := time.Parse("2006-01-02", req.From)
	if err != nil {
		JSONErr(w, 400, "invalid from date")
		return
	}
	to, err := time.Parse("2006-01-02", req.To)
	if err != nil {
		JSONErr(w, 400, "invalid to date")
		return
	}

	result, err := h.svc.AutoSchedule(r.Context(), req.DepartmentID, from, to)
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, result)
}

// ─── Workload ─────────────────────────────────────────────────────────────────

func (h *Handler) GetWorkloadReport(w http.ResponseWriter, r *http.Request) {
	deptID, _ := strconv.ParseInt(r.URL.Query().Get("department_id"), 10, 64)
	report, err := h.svc.GetWorkloadReport(r.Context(), deptID)
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, report)
}

// ─── Emergency ────────────────────────────────────────────────────────────────

func (h *Handler) EmergencyCandidates(w http.ResponseWriter, r *http.Request) {
	slotID, _ := strconv.ParseInt(chi.URLParam(r, "slotId"), 10, 64)
	resp, err := h.svc.FindEmergencyCandidates(r.Context(), service.EmergencyRequest{
		SlotID: slotID,
	})
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, resp)
}

func (h *Handler) EmergencyAssign(w http.ResponseWriter, r *http.Request) {
	var req struct {
		StaffID   int64  `json:"staff_id"`
		SlotID    int64  `json:"slot_id"`
		Note      string `json:"note"`
		CreatedBy int64  `json:"created_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}

	resp, err := h.svc.Assign(r.Context(), service.AssignRequest{
		StaffID:           req.StaffID,
		SlotID:            req.SlotID,
		Source:            domain.SourceEmergency,
		Note:              req.Note,
		CreatedBy:         req.CreatedBy,
		EmergencyOverride: true,
	})
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}

	if len(resp.Violations) > 0 {
		JSON(w, 422, map[string]interface{}{
			"blocked":    true,
			"violations": resp.Violations,
		})
		return
	}

	JSON(w, 201, map[string]interface{}{
		"assignment": resp.Assignment,
		"warnings":   resp.Warnings,
	})
}

// ─── Swaps ────────────────────────────────────────────────────────────────────

func (h *Handler) CreateSwapRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RequesterID   int64  `json:"requester_id"`
		SlotID        int64  `json:"slot_id"`
		TargetStaffID *int64 `json:"target_staff_id"`
		Reason        string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}

	swap, err := h.svc.CreateSwapRequest(r.Context(), req.RequesterID, req.SlotID, req.TargetStaffID, req.Reason)
	if err != nil {
		JSONErr(w, 422, err.Error())
		return
	}
	JSON(w, 201, swap)
}

func (h *Handler) ListPendingSwaps(w http.ResponseWriter, r *http.Request) {
	swaps, err := h.svc.ListPendingSwaps(r.Context())
	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSON(w, 200, swaps)
}

func (h *Handler) ReviewSwap(w http.ResponseWriter, r *http.Request) {
	swapID, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	var req struct {
		Action     string `json:"action"` // "approve" or "reject"
		ReviewerID int64  `json:"reviewer_id"`
		Note       string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONErr(w, 400, "invalid request body")
		return
	}

	var err error
	switch req.Action {
	case "approve":
		err = h.svc.ApproveSwap(r.Context(), swapID, req.ReviewerID, req.Note)
	case "reject":
		err = h.svc.RejectSwap(r.Context(), swapID, req.ReviewerID, req.Note)
	default:
		JSONErr(w, 400, "action must be 'approve' or 'reject'")
		return
	}

	if err != nil {
		JSONErr(w, 500, err.Error())
		return
	}
	JSONMsg(w, 200, "swap request "+req.Action+"d")
}
