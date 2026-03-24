package models

// Pipeline represents a processing pipeline.
type Pipeline struct {
	Base
	Name string `gorm:"uniqueIndex;size:255;not null"`
}

// Step represents a single step in a pipeline.
type Step struct {
	Base
	PipelineID uint   `gorm:"index;not null"`
	Name       string `gorm:"size:255;not null"`
	Kind       string `gorm:"size:100;not null"`
}

// StepDependency links a step to another step it depends on.
type StepDependency struct {
	Base
	StepID      uint `gorm:"index;not null"`
	DependsOnID uint `gorm:"index;not null"`
}
