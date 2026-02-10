package v1

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/helixml/kodit"
	"github.com/helixml/kodit/application/service"
	"github.com/helixml/kodit/domain/task"
	"github.com/helixml/kodit/infrastructure/api/middleware"
	"github.com/helixml/kodit/infrastructure/api/v1/dto"
)

// QueueRouter handles queue API endpoints.
type QueueRouter struct {
	client *kodit.Client
	logger *slog.Logger
}

// NewQueueRouter creates a new QueueRouter.
func NewQueueRouter(client *kodit.Client) *QueueRouter {
	return &QueueRouter{
		client: client,
		logger: client.Logger(),
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
//
//	@Summary		List tasks
//	@Description	List tasks in the queue
//	@Tags			queue
//	@Accept			json
//	@Produce		json
//	@Param			limit		query		int		false	"Max results (default: 50)"
//	@Param			task_type	query		string	false	"Filter by task type"
//	@Success		200			{object}	dto.TaskListResponse
//	@Failure		500			{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/queue [get]
func (r *QueueRouter) ListTasks(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	// Build params from query string
	params := &service.TaskListParams{Limit: 50}

	if limitStr := req.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			params.Limit = parsed
		}
	}

	if taskType := req.URL.Query().Get("task_type"); taskType != "" {
		op := task.Operation(taskType)
		params.Operation = &op
	}

	tasks, err := r.client.Tasks.List(ctx, params)
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
//
//	@Summary		Get task
//	@Description	Get a task by ID
//	@Tags			queue
//	@Accept			json
//	@Produce		json
//	@Param			task_id	path		int	true	"Task ID"
//	@Success		200		{object}	dto.TaskResponse
//	@Failure		404		{object}	map[string]string
//	@Failure		500		{object}	map[string]string
//	@Security		APIKeyAuth
//	@Router			/queue/{task_id} [get]
func (r *QueueRouter) GetTask(w http.ResponseWriter, req *http.Request) {
	ctx := req.Context()

	idStr := chi.URLParam(req, "task_id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	t, err := r.client.Tasks.Get(ctx, id)
	if err != nil {
		middleware.WriteError(w, req, err, r.logger)
		return
	}

	middleware.WriteJSON(w, http.StatusOK, dto.TaskResponse{Data: taskToDTO(t)})
}

func tasksToDTO(tasks []task.Task) []dto.TaskData {
	result := make([]dto.TaskData, len(tasks))
	for i, t := range tasks {
		result[i] = taskToDTO(t)
	}
	return result
}

func taskToDTO(t task.Task) dto.TaskData {
	createdAt := t.CreatedAt()
	updatedAt := t.UpdatedAt()

	return dto.TaskData{
		Type: "task",
		ID:   fmt.Sprintf("%d", t.ID()),
		Attributes: dto.TaskAttributes{
			Type:      string(t.Operation()),
			Priority:  t.Priority(),
			Payload:   t.Payload(),
			CreatedAt: &createdAt,
			UpdatedAt: &updatedAt,
		},
	}
}
