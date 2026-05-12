package domain

import "time"

// ─── Qualifications & Roles ───────────────────────────────────────────────────

type Qualification string

const (
	QualICU        Qualification = "ICU"
	QualPediatrics Qualification = "PEDIATRICS"
	QualSurgery    Qualification = "SURGERY"
	QualEmergency  Qualification = "EMERGENCY"
	QualGeneral    Qualification = "GENERAL"
)

type Role string

const (
	RoleDoctor Role = "DOCTOR"
	RoleNurse  Role = "NURSE"
	RoleIntern Role = "INTERN"
)

// ─── Staff ────────────────────────────────────────────────────────────────────

type Staff struct {
	ID             int64           `json:"id"`
	EmployeeNo     string          `json:"employee_no"`
	Name           string          `json:"name"`
	Role           Role            `json:"role"`
	DepartmentID   int64           `json:"department_id"`
	Qualifications []Qualification `json:"qualifications"`
	IsActive       bool            `json:"is_active"`
	CreatedAt      time.Time       `json:"created_at"`
}

// ─── Department ───────────────────────────────────────────────────────────────

type Department struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Code     string `json:"code"`
	IsActive bool   `json:"is_active"`
}

// ─── Shift Types ──────────────────────────────────────────────────────────────

type ShiftTypeCode string

const (
	ShiftMorning ShiftTypeCode = "MORNING"
	ShiftEvening ShiftTypeCode = "EVENING"
	ShiftNight   ShiftTypeCode = "NIGHT"
)

type ShiftType struct {
	ID          int64         `json:"id"`
	Code        ShiftTypeCode `json:"code"`
	Name        string        `json:"name"`
	StartHour   int           `json:"start_hour"`
	StartMinute int           `json:"start_minute"`
	EndHour     int           `json:"end_hour"`
	EndMinute   int           `json:"end_minute"`
}

func (s ShiftType) DurationHours() float64 {
	start := float64(s.StartHour) + float64(s.StartMinute)/60
	end := float64(s.EndHour) + float64(s.EndMinute)/60
	if end <= start {
		end += 24
	}
	return end - start
}

// ─── Slot ─────────────────────────────────────────────────────────────────────

type SlotStatus string

const (
	SlotOpen     SlotStatus = "OPEN"
	SlotFilled   SlotStatus = "FILLED"
	SlotLocked   SlotStatus = "LOCKED"
	SlotCanceled SlotStatus = "CANCELED"
)

type Slot struct {
	ID            int64           `json:"id"`
	DepartmentID  int64           `json:"department_id"`
	ShiftTypeID   int64           `json:"shift_type_id"`
	Date          time.Time       `json:"date"`
	RequiredStaff int             `json:"required_staff"`
	RequiredRole  Role            `json:"required_role"`
	RequiredQuals []Qualification `json:"required_quals"`
	Status        SlotStatus      `json:"status"`
	AssignedCount int             `json:"assigned_count"`
	CreatedAt     time.Time       `json:"created_at"`
	Department    *Department     `json:"department,omitempty"`
	ShiftType     *ShiftType      `json:"shift_type,omitempty"`
}

func (s *Slot) StartTime() time.Time {
	if s.ShiftType == nil {
		return s.Date
	}
	return time.Date(s.Date.Year(), s.Date.Month(), s.Date.Day(),
		s.ShiftType.StartHour, s.ShiftType.StartMinute, 0, 0, s.Date.Location())
}

func (s *Slot) EndTime() time.Time {
	if s.ShiftType == nil {
		return s.Date
	}
	end := time.Date(s.Date.Year(), s.Date.Month(), s.Date.Day(),
		s.ShiftType.EndHour, s.ShiftType.EndMinute, 0, 0, s.Date.Location())
	if s.ShiftType.EndHour <= s.ShiftType.StartHour {
		end = end.Add(24 * time.Hour)
	}
	return end
}

// ─── Assignment ───────────────────────────────────────────────────────────────

type AssignmentStatus string

const (
	AssignActive    AssignmentStatus = "ACTIVE"
	AssignCanceled  AssignmentStatus = "CANCELED"
	AssignEmergency AssignmentStatus = "EMERGENCY"
)

type AssignmentSource string

const (
	SourceAuto      AssignmentSource = "AUTO"
	SourceManual    AssignmentSource = "MANUAL"
	SourceEmergency AssignmentSource = "EMERGENCY"
	SourceSwap      AssignmentSource = "SWAP"
)

type Assignment struct {
	ID        int64            `json:"id"`
	StaffID   int64            `json:"staff_id"`
	SlotID    int64            `json:"slot_id"`
	Status    AssignmentStatus `json:"status"`
	Source    AssignmentSource `json:"source"`
	Note      string           `json:"note"`
	CreatedBy int64            `json:"created_by"`
	CreatedAt time.Time        `json:"created_at"`
	UpdatedAt time.Time        `json:"updated_at"`
	Staff     *Staff           `json:"staff,omitempty"`
	Slot      *Slot            `json:"slot,omitempty"`
}

// ─── Workload Account ─────────────────────────────────────────────────────────

type WorkloadAccount struct {
	StaffID              int64      `json:"staff_id"`
	TotalHours           float64    `json:"total_hours"`
	MonthHours           float64    `json:"month_hours"`
	WeekHours            float64    `json:"week_hours"`
	ConsecutiveShifts    int        `json:"consecutive_shifts"`
	LastShiftEnd         *time.Time `json:"last_shift_end,omitempty"`
	NightShiftsThisMonth int        `json:"night_shifts_this_month"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

// ─── Swap Request ─────────────────────────────────────────────────────────────

type SwapStatus string

const (
	SwapPending  SwapStatus = "PENDING"
	SwapApproved SwapStatus = "APPROVED"
	SwapRejected SwapStatus = "REJECTED"
)

type SwapRequest struct {
	ID              int64      `json:"id"`
	RequesterID     int64      `json:"requester_id"`
	RequesterSlotID int64      `json:"requester_slot_id"`
	TargetStaffID   *int64     `json:"target_staff_id,omitempty"`
	Reason          string     `json:"reason"`
	Status          SwapStatus `json:"status"`
	ReviewNote      string     `json:"review_note"`
	ReviewedBy      *int64     `json:"reviewed_by,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	Requester       *Staff     `json:"requester,omitempty"`
	Slot            *Slot      `json:"slot,omitempty"`
}

// ─── Rule Violation ───────────────────────────────────────────────────────────

type RuleSeverity string

const (
	SeverityHard RuleSeverity = "HARD"
	SeveritySoft RuleSeverity = "SOFT"
)

type RuleViolation struct {
	RuleCode   string       `json:"rule_code"`
	Severity   RuleSeverity `json:"severity"`
	Message    string       `json:"message"`
	Suggestion string       `json:"suggestion"`
}

func (v RuleViolation) IsHard() bool { return v.Severity == SeverityHard }

// ─── Schedule Result ──────────────────────────────────────────────────────────

type ScheduleResult struct {
	Assignments   []*Assignment   `json:"assignments"`
	Violations    []RuleViolation `json:"violations"`
	UnfilledSlots []*Slot         `json:"unfilled_slots"`
	Stats         ScheduleStats   `json:"stats"`
}

type ScheduleStats struct {
	TotalSlots    int `json:"total_slots"`
	FilledSlots   int `json:"filled_slots"`
	UnfilledSlots int `json:"unfilled_slots"`
	Warnings      int `json:"warnings"`
}

// ─── Audit Log ────────────────────────────────────────────────────────────────

type AuditLog struct {
	ID         int64     `json:"id"`
	EntityType string    `json:"entity_type"`
	EntityID   int64     `json:"entity_id"`
	Action     string    `json:"action"`
	OldValue   string    `json:"old_value"`
	NewValue   string    `json:"new_value"`
	OperatorID int64     `json:"operator_id"`
	CreatedAt  time.Time `json:"created_at"`
}
