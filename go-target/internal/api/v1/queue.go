package v1

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit/internal/api/middleware"
	"github.com/helixml/kodit/internal/api/v1/dto"
	"github.com/helixml/kodit/internal/database"
	"github.com/helixml/kodit/internal/queue"
)

// QueueRouter handles queue API endpoints.
type QueueRouter struct {
	taskRepo       queue.TaskRepository
	taskStatusRepo queue.TaskStatusRepository
	logger         *slog.Logger
}

// NewQueueRouter creates a new QueueRouter.
func NewQueueRouter(
	taskRepo queue.TaskRepository,
	taskStatusRepo queue.TaskStatusRepository,
	logger *slog.Logger,
) *QueueRouter {
	if logger == nil {
		logger = slog.Default()
	}
	return &QueueRouter{
		taskRepo:       taskRepo,
		taskStatusRepo: taskStatusRepo,
		logger:         logger,
	}
}

// Routes returns the chi router for queue endpoints.
func (r *QueueRouter) Routes() chi.Router {
	router := chi.NewRouter()

	router.Get("/tasks", r.ListTasks)
	router.Get("/tasks/{id}", r.GetTask)
	router.Get("/stats", r.Stats)

	return router
}

// ListTasks handles GET /api/v1/queue/tasks.
func (r *QueueRouter) ListTasks(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	limit := 50
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Use Find with a query for pending tasks
	query := database.NewQuery().
		OrderDesc("priority").
		Limit(limit)

	tasks, err := r.taskRepo.Find(ctx, query)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := dto.TaskListResponse{
		Data:       tasksToDTO(tasks),
		TotalCount: len(tasks),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

// GetTask handles GET /api/v1/queue/tasks/{id}.
func (r *QueueRouter) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	task, err := r.taskRepo.Get(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, taskToDTO(task))
}

// Stats handles GET /api/v1/queue/stats.
func (r *QueueRouter) Stats(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	query := database.NewQuery().Limit(1000)
	count, _ := r.taskRepo.Count(ctx, query)

	response := dto.QueueStatsResponse{
		PendingCount: int(count),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

func tasksToDTO(tasks []queue.Task) []dto.TaskResponse {
	result := make([]dto.TaskResponse, len(tasks))
	for i, task := range tasks {
		result[i] = taskToDTO(task)
	}
	return result
}

func taskToDTO(task queue.Task) dto.TaskResponse {
	return dto.TaskResponse{
		ID:        task.ID(),
		DedupKey:  task.DedupKey(),
		Operation: string(task.Operation()),
		Priority:  task.Priority(),
		Payload:   task.Payload(),
		CreatedAt: task.CreatedAt(),
		UpdatedAt: task.UpdatedAt(),
	}
}
