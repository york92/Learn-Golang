package scheduler

import (
	"context"
	"fmt"
	"log"
	"time"

	"hospital-scheduler/internal/domain"
	"hospital-scheduler/internal/repository"
	"hospital-scheduler/internal/rules"
)

// AutoScheduler implements greedy scheduling with rule-based validation
type AutoScheduler struct {
	staffRepo      *repository.StaffRepo
	slotRepo       *repository.SlotRepo
	assignRepo     *repository.AssignmentRepo
	workloadRepo   *repository.WorkloadRepo
	shiftTypeRepo  *repository.ShiftTypeRepo
	engine         *rules.Engine
}

func NewAutoScheduler(
	staffRepo *repository.StaffRepo,
	slotRepo *repository.SlotRepo,
	assignRepo *repository.AssignmentRepo,
	workloadRepo *repository.WorkloadRepo,
	shiftTypeRepo *repository.ShiftTypeRepo,
	engine *rules.Engine,
) *AutoScheduler {
	return &AutoScheduler{
		staffRepo:     staffRepo,
		slotRepo:      slotRepo,
		assignRepo:    assignRepo,
		workloadRepo:  workloadRepo,
		shiftTypeRepo: shiftTypeRepo,
		engine:        engine,
	}
}

// ScheduleRange generates auto assignments for all open slots in a date range
func (s *AutoScheduler) ScheduleRange(ctx context.Context, deptID int64, from, to time.Time) (*domain.ScheduleResult, error) {
	result := &domain.ScheduleResult{}

	// Load all open slots
	slots, err := s.slotRepo.ListByDateRange(ctx, deptID, from, to)
	if err != nil {
		return nil, fmt.Errorf("load slots: %w", err)
	}

	result.Stats.TotalSlots = len(slots)

	for _, slot := range slots {
		if slot.Status != domain.SlotOpen {
			continue
		}

		// Enrich slot with shift type
		shiftType, err := s.shiftTypeRepo.GetByID(ctx, slot.ShiftTypeID)
		if err != nil {
			log.Printf("WARN: get shift type %d: %v", slot.ShiftTypeID, err)
			continue
		}
		slot.ShiftType = shiftType

		// How many more staff do we need?
		needed := slot.RequiredStaff - slot.AssignedCount
		assigned := 0

		for needed > 0 {
			candidate, violations, err := s.findBestCandidate(ctx, slot)
			if err != nil || candidate == nil {
				log.Printf("INFO: no candidate for slot %d on %s", slot.ID, slot.Date.Format("2006-01-02"))
				break
			}

			assignment := &domain.Assignment{
				StaffID: candidate.ID,
				SlotID:  slot.ID,
				Status:  domain.AssignActive,
				Source:  domain.SourceAuto,
			}

			if err := s.assignRepo.Create(ctx, assignment, s.slotRepo, s.workloadRepo); err != nil {
				log.Printf("WARN: create assignment: %v", err)
				break
			}

			slot.AssignedCount++
			needed--
			assigned++

			result.Assignments = append(result.Assignments, assignment)
			result.Violations = append(result.Violations, violations...)
		}

		if assigned > 0 {
			result.Stats.FilledSlots++
		} else {
			result.Stats.UnfilledSlots++
			result.UnfilledSlots = append(result.UnfilledSlots, slot)
		}
	}

	// Count warnings
	for _, v := range result.Violations {
		if !v.IsHard() {
			result.Stats.Warnings++
		}
	}

	return result, nil
}

// findBestCandidate finds the best staff member for a slot
// Returns: candidate, soft violations (warnings), error
func (s *AutoScheduler) findBestCandidate(ctx context.Context, slot *domain.Slot) (*domain.Staff, []domain.RuleViolation, error) {
	// Get qualified candidates
	candidates, err := s.staffRepo.FindCandidates(ctx, slot.RequiredRole, slot.RequiredQuals)
	if err != nil {
		return nil, nil, err
	}

	if len(candidates) == 0 {
		return nil, nil, nil
	}

	// Extract IDs and sort by workload (least loaded first)
	ids := make([]int64, len(candidates))
	for i, c := range candidates {
		ids[i] = c.ID
	}

	sortedIDs, err := s.workloadRepo.ListSortedByWorkload(ctx, ids)
	if err != nil {
		sortedIDs = ids // fallback to unsorted
	}

	// Build a map for quick lookup
	staffMap := make(map[int64]*domain.Staff, len(candidates))
	for _, c := range candidates {
		staffMap[c.ID] = c
	}

	// Try each candidate in workload-sorted order
	for _, staffID := range sortedIDs {
		staff, ok := staffMap[staffID]
		if !ok {
			continue
		}

		workload, _ := s.workloadRepo.Get(ctx, staffID)

		// Get recent assignments for conflict checking
		recentAssignments, err := s.assignRepo.GetRecentByStaff(ctx, staffID, 14)
		if err != nil {
			continue
		}

		// Enrich assignments with slot info for overlap detection
		for _, a := range recentAssignments {
			slotInfo, err := s.slotRepo.GetByID(ctx, a.SlotID)
			if err == nil {
				st, _ := s.shiftTypeRepo.GetByID(ctx, slotInfo.ShiftTypeID)
				slotInfo.ShiftType = st
				a.Slot = slotInfo
			}
		}

		rc := &rules.RuleContext{
			Staff:       staff,
			Slot:        slot,
			ShiftType:   slot.ShiftType,
			Workload:    workload,
			Assignments: recentAssignments,
		}

		violations := s.engine.Validate(ctx, rc)

		// Skip if any hard violation
		if rules.HasHardViolation(violations) {
			continue
		}

		// Collect only soft violations as warnings
		var softViolations []domain.RuleViolation
		for _, v := range violations {
			if !v.IsHard() {
				softViolations = append(softViolations, v)
			}
		}

		return staff, softViolations, nil
	}

	return nil, nil, nil
}
