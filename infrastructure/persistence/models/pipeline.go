package models

// Pipeline represents a processing pipeline for a repository.
type Pipeline struct {
	Base
	RepoID int64  `gorm:"uniqueIndex;not null"`
	Steps  []Step `gorm:"foreignKey:PipelineID;constraint:OnDelete:CASCADE"`
}

// Step represents a single step in a pipeline.
type Step struct {
	Base
	PipelineID   uint             `gorm:"index;not null"`
	Kind         string           `gorm:"size:100;not null"`
	Dependencies []StepDependency `gorm:"foreignKey:StepID;constraint:OnDelete:CASCADE"`
}

// StepDependency links a step to another step it depends on.
type StepDependency struct {
	Base
	StepID      uint `gorm:"not null"`
	DependsOnID uint `gorm:"index;not null"`
}
