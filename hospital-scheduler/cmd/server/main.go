package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hospital-scheduler/internal/api"
	"hospital-scheduler/internal/config"
	"hospital-scheduler/internal/repository"
	"hospital-scheduler/internal/rules"
	"hospital-scheduler/internal/service"
)

func main() {
	cfg := config.Load()
	log.Printf("🏥 Hospital Scheduler  port=%s  db=%s", cfg.Port, cfg.DBPath)

	// ── Database ──────────────────────────────────────────────────────────────
	db, err := repository.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(repository.SchemaSQL); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("✓ Database ready")

	// ── Seed default data ─────────────────────────────────────────────────────
	ctx := context.Background()
	if err := seedDefaults(ctx, db); err != nil {
		log.Printf("WARN seed: %v", err)
	}

	// ── Repositories ──────────────────────────────────────────────────────────
	staffRepo     := repository.NewStaffRepo(db)
	slotRepo      := repository.NewSlotRepo(db)
	assignRepo    := repository.NewAssignmentRepo(db)
	workloadRepo  := repository.NewWorkloadRepo(db)
	shiftTypeRepo := repository.NewShiftTypeRepo(db)
	swapRepo      := repository.NewSwapRepo(db)
	auditRepo     := repository.NewAuditRepo(db)
	deptRepo      := repository.NewDeptRepo(db)

	// ── Rule Engine ───────────────────────────────────────────────────────────
	engine := rules.NewEngine(rules.RuleEngineConfig{
		MaxConsecutiveHours:    cfg.Rules.MaxConsecutiveHours,
		MinRestBetweenShifts:   cfg.Rules.MinRestBetweenShifts,
		MaxConsecutiveShifts:   cfg.Rules.MaxConsecutiveShifts,
		MaxWeeklyHours:         cfg.Rules.MaxWeeklyHours,
		MaxNightShiftsPerMonth: cfg.Rules.MaxNightShiftsPerMonth,
		MaxWorkloadDiffPercent: cfg.Rules.MaxWorkloadDiffPercent,
	})

	// ── Service & HTTP ────────────────────────────────────────────────────────
	svc := service.NewScheduleService(
		staffRepo, slotRepo, assignRepo, workloadRepo,
		shiftTypeRepo, swapRepo, auditRepo, deptRepo, engine,
	)
	h      := api.NewHandler(svc, staffRepo, slotRepo, deptRepo, shiftTypeRepo)
	router := api.NewRouter(h)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Printf("✓ Listening  http://localhost:%s", cfg.Port)
		log.Printf("  API base   http://localhost:%s/api/v1", cfg.Port)
		log.Printf("  Health     http://localhost:%s/health", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()

	<-quit
	log.Println("Shutting down…")
	shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	log.Println("Bye.")
}

// seedDefaults inserts standard shift types + a demo department & staff on first run.
func seedDefaults(ctx context.Context, db *repository.DB) error {
	shifts := []struct {
		code        string
		name        string
		startH, endH int
	}{
		{"MORNING", "早班 08:00-16:00", 8, 16},
		{"EVENING", "晚班 16:00-00:00", 16, 0},
		{"NIGHT",   "夜班 00:00-08:00", 0, 8},
	}
	for _, s := range shifts {
		if _, err := db.ExecContext(ctx,
			`INSERT OR IGNORE INTO shift_types(code,name,start_hour,start_minute,end_hour,end_minute)
			 VALUES(?,?,?,0,?,0)`,
			s.code, s.name, s.startH, s.endH,
		); err != nil {
			return err
		}
	}

	// Demo department
	if _, err := db.ExecContext(ctx,
		`INSERT OR IGNORE INTO departments(name,code) VALUES('急诊科','ER')`,
	); err != nil {
		return err
	}

	// Fetch dept id
	var deptID int64
	if err := db.QueryRowContext(ctx,
		`SELECT id FROM departments WHERE code='ER'`,
	).Scan(&deptID); err != nil {
		return err
	}

	// Demo staff
	type staffSeed struct{ no, name, role, qual string }
	seeds := []staffSeed{
		{"D001", "张医生", "DOCTOR", "EMERGENCY"},
		{"D002", "李医生", "DOCTOR", "SURGERY"},
		{"N001", "王护士", "NURSE", "ICU"},
		{"N002", "陈护士", "NURSE", "ICU"},
		{"N003", "刘护士", "NURSE", "EMERGENCY"},
		{"N004", "赵护士", "NURSE", "GENERAL"},
		{"N005", "吴护士", "NURSE", "ICU"},
		{"N006", "郑护士", "NURSE", "GENERAL"},
	}

	for _, s := range seeds {
		var exists int
		_ = db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM staff WHERE employee_no=?`, s.no,
		).Scan(&exists)
		if exists > 0 {
			continue
		}
		res, err := db.ExecContext(ctx,
			`INSERT INTO staff(employee_no,name,role,department_id) VALUES(?,?,?,?)`,
			s.no, s.name, s.role, deptID,
		)
		if err != nil {
			continue
		}
		staffID, _ := res.LastInsertId()
		_, _ = db.ExecContext(ctx,
			`INSERT OR IGNORE INTO staff_qualifications(staff_id,qualification) VALUES(?,?)`,
			staffID, s.qual,
		)
		_, _ = db.ExecContext(ctx,
			`INSERT OR IGNORE INTO workload_accounts(staff_id) VALUES(?)`, staffID,
		)
	}

	log.Printf("✓ Seed data ready (dept_id=%d, %d shift types)", deptID, len(shifts))
	return nil
}
