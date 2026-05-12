package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RequestID)
	r.Use(corsMiddleware)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		JSON(w, 200, map[string]string{"status": "ok"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		// Departments
		r.Get("/departments", h.ListDepartments)
		r.Post("/departments", h.CreateDepartment)

		// Shift types
		r.Get("/shift-types", h.ListShiftTypes)

		// Staff
		r.Get("/staff", h.ListStaff)
		r.Post("/staff", h.CreateStaff)
		r.Get("/staff/{id}", h.GetStaff)

		// Slots
		r.Get("/slots", h.ListSlots)
		r.Post("/slots", h.CreateSlot)

		// Assignments
		r.Post("/assignments", h.CreateAssignment)
		r.Delete("/assignments/{id}", h.CancelAssignment)

		// Auto scheduling
		r.Post("/schedule/auto", h.AutoSchedule)

		// Workload report
		r.Get("/workload", h.GetWorkloadReport)

		// Emergency dispatch
		r.Get("/emergency/candidates/{slotId}", h.EmergencyCandidates)
		r.Post("/emergency/assign", h.EmergencyAssign)

		// Swap requests
		r.Post("/swaps", h.CreateSwapRequest)
		r.Get("/swaps/pending", h.ListPendingSwaps)
		r.Post("/swaps/{id}/review", h.ReviewSwap)
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}
