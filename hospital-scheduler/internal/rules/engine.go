package rules

import (
	"context"
	"fmt"
	"time"

	"hospital-scheduler/internal/domain"
)

// RuleContext carries all data a rule needs to evaluate
type RuleContext struct {
	Staff      *domain.Staff
	Slot       *domain.Slot
	ShiftType  *domain.ShiftType
	Workload   *domain.WorkloadAccount
	// All active assignments for this staff in the relevant window
	Assignments []*domain.Assignment
}

// Rule is the interface every rule must implement
type Rule interface {
	Code() string
	Name() string
	Severity() domain.RuleSeverity
	Validate(ctx context.Context, rc *RuleContext) *domain.RuleViolation
}

// Engine runs all registered rules and returns violations
type Engine struct {
	hardRules []Rule
	softRules []Rule
}

func NewEngine(cfg RuleEngineConfig) *Engine {
	e := &Engine{}

	// Hard rules – block if violated
	e.hardRules = []Rule{
		&NoDoubleBookingRule{},
		&QualificationMatchRule{},
		&MaxConsecutiveHoursRule{MaxHours: cfg.MaxConsecutiveHours},
		&MinRestBetweenShiftsRule{MinRestHours: cfg.MinRestBetweenShifts},
	}

	// Soft rules – warn if violated
	e.softRules = []Rule{
		&MaxConsecutiveShiftsRule{Max: cfg.MaxConsecutiveShifts},
		&MaxWeeklyHoursRule{MaxHours: cfg.MaxWeeklyHours},
		&MaxNightShiftsPerMonthRule{Max: cfg.MaxNightShiftsPerMonth},
		&FairnessRule{MaxDiffPct: cfg.MaxWorkloadDiffPercent},
	}

	return e
}

type RuleEngineConfig struct {
	MaxConsecutiveHours    float64
	MinRestBetweenShifts   float64
	MaxConsecutiveShifts   int
	MaxWeeklyHours         float64
	MaxNightShiftsPerMonth int
	MaxWorkloadDiffPercent float64
}

// Validate runs all rules. Returns all violations (hard + soft).
// Callers should check if any IsHard() == true to decide whether to block.
func (e *Engine) Validate(ctx context.Context, rc *RuleContext) []domain.RuleViolation {
	var violations []domain.RuleViolation

	for _, r := range e.hardRules {
		if v := r.Validate(ctx, rc); v != nil {
			violations = append(violations, *v)
		}
	}

	for _, r := range e.softRules {
		if v := r.Validate(ctx, rc); v != nil {
			violations = append(violations, *v)
		}
	}

	return violations
}

// HasHardViolation returns true if any hard rule was violated
func HasHardViolation(vs []domain.RuleViolation) bool {
	for _, v := range vs {
		if v.IsHard() {
			return true
		}
	}
	return false
}

// ─── Hard Rule: No Double Booking ────────────────────────────────────────────

type NoDoubleBookingRule struct{}

func (r *NoDoubleBookingRule) Code() string                    { return "H001" }
func (r *NoDoubleBookingRule) Name() string                    { return "禁止双重排班" }
func (r *NoDoubleBookingRule) Severity() domain.RuleSeverity  { return domain.SeverityHard }

func (r *NoDoubleBookingRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	slotStart := rc.Slot.StartTime()
	slotEnd := rc.Slot.EndTime()

	for _, a := range rc.Assignments {
		if a.Status == domain.AssignCanceled {
			continue
		}
		if a.Slot == nil {
			continue
		}
		existStart := a.Slot.StartTime()
		existEnd := a.Slot.EndTime()
		// Overlap check: [s1,e1) overlaps [s2,e2) if s1 < e2 && s2 < e1
		if slotStart.Before(existEnd) && existStart.Before(slotEnd) {
			return &domain.RuleViolation{
				RuleCode: r.Code(),
				Severity: domain.SeverityHard,
				Message: fmt.Sprintf("员工 %s 在 %s 已有排班，时间段冲突",
					rc.Staff.Name, slotStart.Format("2006-01-02 15:04")),
				Suggestion: "请选择其他时间段或其他员工",
			}
		}
	}
	return nil
}

// ─── Hard Rule: Qualification Match ─────────────────────────────────────────

type QualificationMatchRule struct{}

func (r *QualificationMatchRule) Code() string                   { return "H002" }
func (r *QualificationMatchRule) Name() string                   { return "资质匹配检查" }
func (r *QualificationMatchRule) Severity() domain.RuleSeverity { return domain.SeverityHard }

func (r *QualificationMatchRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	if len(rc.Slot.RequiredQuals) == 0 {
		return nil
	}

	staffQuals := make(map[domain.Qualification]bool)
	for _, q := range rc.Staff.Qualifications {
		staffQuals[q] = true
	}

	for _, req := range rc.Slot.RequiredQuals {
		if !staffQuals[req] {
			return &domain.RuleViolation{
				RuleCode: r.Code(),
				Severity: domain.SeverityHard,
				Message: fmt.Sprintf("员工 %s 缺少资质: %s",
					rc.Staff.Name, req),
				Suggestion: "请安排具备该资质的人员",
			}
		}
	}

	// Role check
	if rc.Slot.RequiredRole != "" && rc.Staff.Role != rc.Slot.RequiredRole {
		return &domain.RuleViolation{
			RuleCode: r.Code(),
			Severity: domain.SeverityHard,
			Message: fmt.Sprintf("员工角色 %s 不满足岗位要求 %s",
				rc.Staff.Role, rc.Slot.RequiredRole),
			Suggestion: "请安排符合角色要求的人员",
		}
	}

	return nil
}

// ─── Hard Rule: Max Consecutive Hours ────────────────────────────────────────

type MaxConsecutiveHoursRule struct{ MaxHours float64 }

func (r *MaxConsecutiveHoursRule) Code() string                   { return "H003" }
func (r *MaxConsecutiveHoursRule) Name() string                   { return "连续工作时长上限" }
func (r *MaxConsecutiveHoursRule) Severity() domain.RuleSeverity { return domain.SeverityHard }

func (r *MaxConsecutiveHoursRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	if rc.Workload == nil || rc.Workload.LastShiftEnd == nil {
		return nil
	}
	// If the new slot starts before last shift ended + buffer, check cumulative
	newStart := rc.Slot.StartTime()
	lastEnd := *rc.Workload.LastShiftEnd

	gap := newStart.Sub(lastEnd).Hours()
	if gap < 0 {
		gap = 0
	}

	if gap == 0 {
		// Continuous work: add new shift hours
		shiftHours := rc.ShiftType.DurationHours()
		total := shiftHours // simplified; production would sum chain
		if total > r.MaxHours {
			return &domain.RuleViolation{
				RuleCode: r.Code(),
				Severity: domain.SeverityHard,
				Message: fmt.Sprintf("连续工作时长将达 %.1f 小时，超过上限 %.1f 小时",
					total, r.MaxHours),
				Suggestion: "必须安排休息后再排班",
			}
		}
	}
	return nil
}

// ─── Hard Rule: Min Rest Between Shifts ──────────────────────────────────────

type MinRestBetweenShiftsRule struct{ MinRestHours float64 }

func (r *MinRestBetweenShiftsRule) Code() string                   { return "H004" }
func (r *MinRestBetweenShiftsRule) Name() string                   { return "班次间最低休息时长" }
func (r *MinRestBetweenShiftsRule) Severity() domain.RuleSeverity { return domain.SeverityHard }

func (r *MinRestBetweenShiftsRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	if rc.Workload == nil || rc.Workload.LastShiftEnd == nil {
		return nil
	}
	newStart := rc.Slot.StartTime()
	lastEnd := *rc.Workload.LastShiftEnd
	restHours := newStart.Sub(lastEnd).Hours()

	if restHours >= 0 && restHours < r.MinRestHours {
		return &domain.RuleViolation{
			RuleCode: r.Code(),
			Severity: domain.SeverityHard,
			Message: fmt.Sprintf("距上次排班结束仅 %.1f 小时，最低要求 %.1f 小时",
				restHours, r.MinRestHours),
			Suggestion: fmt.Sprintf("最早可在 %s 之后排班",
				lastEnd.Add(time.Duration(r.MinRestHours*float64(time.Hour))).Format("2006-01-02 15:04")),
		}
	}
	return nil
}

// ─── Soft Rule: Max Consecutive Shifts ───────────────────────────────────────

type MaxConsecutiveShiftsRule struct{ Max int }

func (r *MaxConsecutiveShiftsRule) Code() string                   { return "S001" }
func (r *MaxConsecutiveShiftsRule) Name() string                   { return "连续排班天数建议上限" }
func (r *MaxConsecutiveShiftsRule) Severity() domain.RuleSeverity { return domain.SeveritySoft }

func (r *MaxConsecutiveShiftsRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	if rc.Workload == nil {
		return nil
	}
	if rc.Workload.ConsecutiveShifts >= r.Max {
		return &domain.RuleViolation{
			RuleCode: r.Code(),
			Severity: domain.SeveritySoft,
			Message: fmt.Sprintf("员工 %s 已连续排班 %d 天（建议上限 %d 天）",
				rc.Staff.Name, rc.Workload.ConsecutiveShifts, r.Max),
			Suggestion: "建议安排轮休",
		}
	}
	return nil
}

// ─── Soft Rule: Max Weekly Hours ─────────────────────────────────────────────

type MaxWeeklyHoursRule struct{ MaxHours float64 }

func (r *MaxWeeklyHoursRule) Code() string                   { return "S002" }
func (r *MaxWeeklyHoursRule) Name() string                   { return "每周最大工时建议" }
func (r *MaxWeeklyHoursRule) Severity() domain.RuleSeverity { return domain.SeveritySoft }

func (r *MaxWeeklyHoursRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	if rc.Workload == nil {
		return nil
	}
	projected := rc.Workload.WeekHours
	if rc.ShiftType != nil {
		projected += rc.ShiftType.DurationHours()
	}
	if projected > r.MaxHours {
		return &domain.RuleViolation{
			RuleCode: r.Code(),
			Severity: domain.SeveritySoft,
			Message: fmt.Sprintf("本周工时将达 %.1f 小时，超过建议 %.1f 小时",
				projected, r.MaxHours),
			Suggestion: "建议合理分配工时",
		}
	}
	return nil
}

// ─── Soft Rule: Max Night Shifts Per Month ────────────────────────────────────

type MaxNightShiftsPerMonthRule struct{ Max int }

func (r *MaxNightShiftsPerMonthRule) Code() string                   { return "S003" }
func (r *MaxNightShiftsPerMonthRule) Name() string                   { return "每月夜班次数建议" }
func (r *MaxNightShiftsPerMonthRule) Severity() domain.RuleSeverity { return domain.SeveritySoft }

func (r *MaxNightShiftsPerMonthRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	if rc.Workload == nil || rc.Slot.ShiftType == nil {
		return nil
	}
	if rc.Slot.ShiftType.Code != domain.ShiftNight {
		return nil
	}
	if rc.Workload.NightShiftsThisMonth >= r.Max {
		return &domain.RuleViolation{
			RuleCode: r.Code(),
			Severity: domain.SeveritySoft,
			Message: fmt.Sprintf("员工 %s 本月夜班已达 %d 次（建议上限 %d 次）",
				rc.Staff.Name, rc.Workload.NightShiftsThisMonth, r.Max),
			Suggestion: "建议安排夜班较少的员工",
		}
	}
	return nil
}

// ─── Soft Rule: Fairness (Workload Balance) ───────────────────────────────────

type FairnessRule struct{ MaxDiffPct float64 }

func (r *FairnessRule) Code() string                   { return "S004" }
func (r *FairnessRule) Name() string                   { return "工时公平性检查" }
func (r *FairnessRule) Severity() domain.RuleSeverity { return domain.SeveritySoft }

func (r *FairnessRule) Validate(_ context.Context, rc *RuleContext) *domain.RuleViolation {
	// This rule needs dept avg passed in workload; simplified check here
	if rc.Workload == nil {
		return nil
	}
	// Placeholder: in production, fetch dept avg and compare
	_ = r.MaxDiffPct
	return nil
}
