package v1

import (
	"fmt"
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

	router.Get("/", r.ListTasks)
	router.Get("/{task_id}", r.GetTask)

	return router
}

// ListTasks handles GET /api/v1/queue.
// Supports optional task_type filter.
func (r *QueueRouter) ListTasks(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	limit := 50
	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Build query with optional task_type filter
	query := database.NewQuery().
		OrderDesc("priority").
		Limit(limit)

	// Add task_type filter if specified
	if taskType := req.URL.Query().Get("task_type"); taskType != "" {
		query = query.Equal("type", taskType)
	}

	tasks, err := r.taskRepo.Find(ctx, query)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	response := dto.TaskListResponse{
		Data: tasksToDTO(tasks),
	}

	middleware.WriteJSON(w, http.StatusOK, response)
}

// GetTask handles GET /api/v1/queue/{task_id}.
func (r *QueueRouter) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "task_id")
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

	middleware.WriteJSON(w, http.StatusOK, dto.TaskResponse{Data: taskToDTO(task)})
}

func tasksToDTO(tasks []queue.Task) []dto.TaskData {
	result := make([]dto.TaskData, len(tasks))
	for i, task := range tasks {
		result[i] = taskToDTO(task)
	}
	return result
}

func taskToDTO(task queue.Task) dto.TaskData {
	createdAt := task.CreatedAt()
	updatedAt := task.UpdatedAt()

	return dto.TaskData{
		Type: "task",
		ID:   fmt.Sprintf("%d", task.ID()),
		Attributes: dto.TaskAttributes{
			Type:      string(task.Operation()),
			Priority:  task.Priority(),
			Payload:   task.Payload(),
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}
}
