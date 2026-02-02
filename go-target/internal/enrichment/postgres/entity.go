package postgres

import (
	"time"
)

// EnrichmentEntity is the GORM model for the enrichments_v2 table.
type EnrichmentEntity struct {
	ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	Type      string    `gorm:"column:type;not null;index"`
	Subtype   string    `gorm:"column:subtype;not null;index"`
	Content   string    `gorm:"column:content;type:text;not null"`
	CreatedAt time.Time `gorm:"column:created_at;not null"`
	UpdatedAt time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for EnrichmentEntity.
func (EnrichmentEntity) TableName() string {
	return "enrichments_v2"
}

// AssociationEntity is the GORM model for the enrichment_associations table.
type AssociationEntity struct {
	ID           int64     `gorm:"column:id;primaryKey;autoIncrement"`
	EnrichmentID int64     `gorm:"column:enrichment_id;not null;index"`
	EntityType   string    `gorm:"column:entity_type;size:50;not null;index"`
	EntityID     string    `gorm:"column:entity_id;size:255;not null;index"`
	CreatedAt    time.Time `gorm:"column:created_at;not null"`
	UpdatedAt    time.Time `gorm:"column:updated_at;not null"`
}

// TableName returns the table name for AssociationEntity.
func (AssociationEntity) TableName() string {
	return "enrichment_associations"
}
