package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"hospital-scheduler/internal/domain"
	"hospital-scheduler/internal/repository"
	"hospital-scheduler/internal/rules"
	"hospital-scheduler/internal/scheduler"
)

type ScheduleService struct {
	staffRepo     *repository.StaffRepo
	slotRepo      *repository.SlotRepo
	assignRepo    *repository.AssignmentRepo
	workloadRepo  *repository.WorkloadRepo
	shiftTypeRepo *repository.ShiftTypeRepo
	swapRepo      *repository.SwapRepo
	auditRepo     *repository.AuditRepo
	deptRepo      *repository.DeptRepo
	engine        *rules.Engine
	autoSched     *scheduler.AutoScheduler
}

func NewScheduleService(
	staffRepo *repository.StaffRepo,
	slotRepo *repository.SlotRepo,
	assignRepo *repository.AssignmentRepo,
	workloadRepo *repository.WorkloadRepo,
	shiftTypeRepo *repository.ShiftTypeRepo,
	swapRepo *repository.SwapRepo,
	auditRepo *repository.AuditRepo,
	deptRepo *repository.DeptRepo,
	engine *rules.Engine,
) *ScheduleService {
	auto := scheduler.NewAutoScheduler(
		staffRepo, slotRepo, assignRepo, workloadRepo, shiftTypeRepo, engine)
	return &ScheduleService{
		staffRepo:     staffRepo,
		slotRepo:      slotRepo,
		assignRepo:    assignRepo,
		workloadRepo:  workloadRepo,
		shiftTypeRepo: shiftTypeRepo,
		swapRepo:      swapRepo,
		auditRepo:     auditRepo,
		deptRepo:      deptRepo,
		engine:        engine,
		autoSched:     auto,
	}
}

// ─── Assign ───────────────────────────────────────────────────────────────────

type AssignRequest struct {
	StaffID           int64
	SlotID            int64
	Source            domain.AssignmentSource
	Note              string
	CreatedBy         int64
	EmergencyOverride bool
}

type AssignResponse struct {
	Assignment *domain.Assignment
	// Violations contains hard-rule blocks — if non-empty, assignment was NOT created.
	Violations []domain.RuleViolation
	// Warnings contains soft-rule alerts — assignment WAS created despite these.
	Warnings []domain.RuleViolation
}

func (s *ScheduleService) Assign(ctx context.Context, req AssignRequest) (*AssignResponse, error) {
	staff, err := s.staffRepo.GetByID(ctx, req.StaffID)
	if err != nil {
		return nil, fmt.Errorf("get staff: %w", err)
	}

	slot, err := s.slotRepo.GetByID(ctx, req.SlotID)
	if err != nil {
		return nil, fmt.Errorf("get slot: %w", err)
	}

	shiftType, err := s.shiftTypeRepo.GetByID(ctx, slot.ShiftTypeID)
	if err != nil {
		return nil, fmt.Errorf("get shift type: %w", err)
	}
	slot.ShiftType = shiftType

	workload, _ := s.workloadRepo.Get(ctx, req.StaffID)

	// Load recent assignments for overlap / rest-interval checks.
	recentAssigns, _ := s.assignRepo.GetRecentByStaff(ctx, req.StaffID, 14)
	for _, a := range recentAssigns {
		if si, err := s.slotRepo.GetByID(ctx, a.SlotID); err == nil {
			if st, err := s.shiftTypeRepo.GetByID(ctx, si.ShiftTypeID); err == nil {
				si.ShiftType = st
			}
			a.Slot = si
		}
	}

	rc := &rules.RuleContext{
		Staff:       staff,
		Slot:        slot,
		ShiftType:   shiftType,
		Workload:    workload,
		Assignments: recentAssigns,
	}

	allViolations := s.engine.Validate(ctx, rc)

	var hardViolations, softViolations []domain.RuleViolation
	for _, v := range allViolations {
		if v.IsHard() {
			hardViolations = append(hardViolations, v)
		} else {
			softViolations = append(softViolations, v)
		}
	}

	if len(hardViolations) > 0 {
		return &AssignResponse{Violations: hardViolations, Warnings: softViolations}, nil
	}

	a := &domain.Assignment{
		StaffID:   req.StaffID,
		SlotID:    req.SlotID,
		Status:    domain.AssignActive,
		Source:    req.Source,
		Note:      req.Note,
		CreatedBy: req.CreatedBy,
	}
	if req.Source == domain.SourceEmergency {
		a.Status = domain.AssignEmergency
	}

	if err := s.assignRepo.Create(ctx, a, s.slotRepo, s.workloadRepo); err != nil {
		// Translate duplicate assignment into a hard-rule violation (422, not 500)
		if errors.Is(err, repository.ErrDuplicateAssignment) {
			return &AssignResponse{
				Violations: []domain.RuleViolation{{
					RuleCode:   "H001",
					Severity:   domain.SeverityHard,
					Message:    fmt.Sprintf("员工 %s 已排入该班次，不可重复排班", staff.Name),
					Suggestion: "请选择其他员工或其他班次",
				}},
			}, nil
		}
		return nil, fmt.Errorf("create assignment: %w", err)
	}

	_ = s.auditRepo.Log(ctx, &domain.AuditLog{
		EntityType: "assignment",
		EntityID:   a.ID,
		Action:     "create",
		NewValue:   fmt.Sprintf("staff=%d slot=%d source=%s", req.StaffID, req.SlotID, req.Source),
		OperatorID: req.CreatedBy,
	})

	return &AssignResponse{Assignment: a, Warnings: softViolations}, nil
}

// ─── Cancel Assignment ────────────────────────────────────────────────────────

func (s *ScheduleService) CancelAssignment(ctx context.Context, assignmentID, operatorID int64) error {
	if err := s.assignRepo.Cancel(ctx, assignmentID, s.slotRepo); err != nil {
		return fmt.Errorf("cancel assignment: %w", err)
	}
	_ = s.auditRepo.Log(ctx, &domain.AuditLog{
		EntityType: "assignment",
		EntityID:   assignmentID,
		Action:     "cancel",
		OperatorID: operatorID,
	})
	return nil
}

// ─── Auto Schedule ────────────────────────────────────────────────────────────

func (s *ScheduleService) AutoSchedule(ctx context.Context, deptID int64, from, to time.Time) (*domain.ScheduleResult, error) {
	return s.autoSched.ScheduleRange(ctx, deptID, from, to)
}

// ─── Get Schedule ─────────────────────────────────────────────────────────────

func (s *ScheduleService) GetSchedule(ctx context.Context, deptID int64, from, to time.Time) ([]*domain.Slot, error) {
	slots, err := s.slotRepo.ListByDateRange(ctx, deptID, from, to)
	if err != nil {
		return nil, err
	}
	for _, slot := range slots {
		if st, err := s.shiftTypeRepo.GetByID(ctx, slot.ShiftTypeID); err == nil {
			slot.ShiftType = st
		}
		if dept, err := s.deptRepo.GetByID(ctx, slot.DepartmentID); err == nil {
			slot.Department = dept
		}
	}
	return slots, nil
}

// ─── Workload Report ──────────────────────────────────────────────────────────

type WorkloadReport struct {
	Staff   *domain.Staff          `json:"staff"`
	Account *domain.WorkloadAccount `json:"account"`
	AvgHrs  float64                `json:"avg_hrs"`
	DiffPct float64                `json:"diff_pct"`
}

func (s *ScheduleService) GetWorkloadReport(ctx context.Context, deptID int64) ([]*WorkloadReport, error) {
	staff, err := s.staffRepo.ListByDepartment(ctx, deptID)
	if err != nil {
		return nil, err
	}
	avgHrs, _ := s.workloadRepo.GetDeptAvgHours(ctx, deptID)

	var reports []*WorkloadReport
	for _, st := range staff {
		acc, _ := s.workloadRepo.Get(ctx, st.ID)
		diff := 0.0
		if avgHrs > 0 {
			diff = (acc.MonthHours - avgHrs) / avgHrs * 100
		}
		reports = append(reports, &WorkloadReport{
			Staff:   st,
			Account: acc,
			AvgHrs:  avgHrs,
			DiffPct: diff,
		})
	}
	return reports, nil
}

// ─── Emergency Dispatch ───────────────────────────────────────────────────────

type EmergencyRequest struct {
	SlotID    int64
	Reason    string
	CreatedBy int64
}

type EmergencyResponse struct {
	Candidates []*EmergencyCandidate `json:"Candidates"`
}

type EmergencyCandidate struct {
	Staff      *domain.Staff          `json:"staff"`
	Workload   *domain.WorkloadAccount `json:"workload"`
	Violations []domain.RuleViolation  `json:"violations,omitempty"`
}

func (s *ScheduleService) FindEmergencyCandidates(ctx context.Context, req EmergencyRequest) (*EmergencyResponse, error) {
	slot, err := s.slotRepo.GetByID(ctx, req.SlotID)
	if err != nil {
		return nil, fmt.Errorf("get slot: %w", err)
	}
	shiftType, _ := s.shiftTypeRepo.GetByID(ctx, slot.ShiftTypeID)
	slot.ShiftType = shiftType

	candidates, err := s.staffRepo.FindCandidates(ctx, slot.RequiredRole, slot.RequiredQuals)
	if err != nil {
		return nil, err
	}

	var result []*EmergencyCandidate
	for _, staff := range candidates {
		workload, _ := s.workloadRepo.Get(ctx, staff.ID)
		recentAssigns, _ := s.assignRepo.GetRecentByStaff(ctx, staff.ID, 14)
		for _, a := range recentAssigns {
			if si, err := s.slotRepo.GetByID(ctx, a.SlotID); err == nil {
				if st, err := s.shiftTypeRepo.GetByID(ctx, si.ShiftTypeID); err == nil {
					si.ShiftType = st
				}
				a.Slot = si
			}
		}

		rc := &rules.RuleContext{
			Staff:       staff,
			Slot:        slot,
			ShiftType:   shiftType,
			Workload:    workload,
			Assignments: recentAssigns,
		}
		violations := s.engine.Validate(ctx, rc)
		if rules.HasHardViolation(violations) {
			continue
		}
		var soft []domain.RuleViolation
		for _, v := range violations {
			if !v.IsHard() {
				soft = append(soft, v)
			}
		}
		result = append(result, &EmergencyCandidate{Staff: staff, Workload: workload, Violations: soft})
	}
	return &EmergencyResponse{Candidates: result}, nil
}

// ─── Swap Requests ────────────────────────────────────────────────────────────

func (s *ScheduleService) CreateSwapRequest(ctx context.Context,
	requesterID, slotID int64, targetStaffID *int64, reason string) (*domain.SwapRequest, error) {

	assignments, err := s.assignRepo.GetBySlot(ctx, slotID)
	if err != nil {
		return nil, err
	}
	found := false
	for _, a := range assignments {
		if a.StaffID == requesterID {
			found = true; break
		}
	}
	if !found {
		return nil, fmt.Errorf("staff %d is not assigned to slot %d", requesterID, slotID)
	}

	req := &domain.SwapRequest{
		RequesterID:     requesterID,
		RequesterSlotID: slotID,
		TargetStaffID:   targetStaffID,
		Reason:          reason,
	}
	if err := s.swapRepo.Create(ctx, req); err != nil {
		return nil, err
	}
	return req, nil
}

func (s *ScheduleService) ApproveSwap(ctx context.Context, swapID, reviewerID int64, note string) error {
	swap, err := s.swapRepo.GetByID(ctx, swapID)
	if err != nil {
		return err
	}
	if swap.TargetStaffID == nil {
		return fmt.Errorf("no target staff specified for swap %d", swapID)
	}

	// Cancel requester's original assignment
	assignments, _ := s.assignRepo.GetBySlot(ctx, swap.RequesterSlotID)
	for _, a := range assignments {
		if a.StaffID == swap.RequesterID {
			_ = s.assignRepo.Cancel(ctx, a.ID, s.slotRepo)
			break
		}
	}

	// Assign target staff
	if _, err := s.Assign(ctx, AssignRequest{
		StaffID:   *swap.TargetStaffID,
		SlotID:    swap.RequesterSlotID,
		Source:    domain.SourceSwap,
		Note:      fmt.Sprintf("换班申请 #%d", swapID),
		CreatedBy: reviewerID,
	}); err != nil {
		return fmt.Errorf("create swap assignment: %w", err)
	}

	return s.swapRepo.Approve(ctx, swapID, reviewerID, note)
}

func (s *ScheduleService) RejectSwap(ctx context.Context, swapID, reviewerID int64, note string) error {
	return s.swapRepo.Reject(ctx, swapID, reviewerID, note)
}

func (s *ScheduleService) ListPendingSwaps(ctx context.Context) ([]*domain.SwapRequest, error) {
	return s.swapRepo.ListPending(ctx)
}
