package postgres

import (
	"encoding/json"
	"time"
)

// TaskEntity represents a task in the database.
type TaskEntity struct {
	ID        int64           `gorm:"column:id;primaryKey;autoIncrement"`
	DedupKey  string          `gorm:"column:dedup_key;type:varchar(255);index;not null"`
	Type      string          `gorm:"column:type;type:varchar(255);index;not null"`
	Payload   json.RawMessage `gorm:"column:payload;type:jsonb"`
	Priority  int             `gorm:"column:priority;not null"`
	CreatedAt time.Time       `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time       `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName returns the database table name.
func (TaskEntity) TableName() string {
	return "tasks"
}

// TaskStatusEntity represents a task status in the database.
type TaskStatusEntity struct {
	ID            string    `gorm:"column:id;type:varchar(255);primaryKey;index;not null"`
	CreatedAt     time.Time `gorm:"column:created_at;not null"`
	UpdatedAt     time.Time `gorm:"column:updated_at;not null"`
	Operation     string    `gorm:"column:operation;type:varchar(255);index;not null"`
	TrackableID   *int64    `gorm:"column:trackable_id;index"`
	TrackableType *string   `gorm:"column:trackable_type;type:varchar(255);index"`
	ParentID      *string   `gorm:"column:parent;type:varchar(255);index"`
	Message       string    `gorm:"column:message;type:text;default:''"`
	State         string    `gorm:"column:state;type:varchar(255);default:''"`
	Error         string    `gorm:"column:error;type:text;default:''"`
	Total         int       `gorm:"column:total;default:0"`
	Current       int       `gorm:"column:current;default:0"`
}

// TableName returns the database table name.
func (TaskStatusEntity) TableName() string {
	return "task_status"
}
