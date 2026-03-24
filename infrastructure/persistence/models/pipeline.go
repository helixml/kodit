package models

// Pipeline represents a processing pipeline.
type Pipeline struct {
	Base
	Name string `gorm:"uniqueIndex;size:255;not null"`
}

// Step represents a single step in a pipeline.
type Step struct {
	Base
	Name string `gorm:"size:255;not null"`
	Kind string `gorm:"size:100;not null"`
}

// PipelineStep associates a step with a pipeline.
type PipelineStep struct {
	Base
	PipelineID uint     `gorm:"uniqueIndex;not null"`
	Pipeline   Pipeline `gorm:"constraint:OnDelete:CASCADE"`
	StepID     uint     `gorm:"uniqueIndex;not null"`
	Step       Step     `gorm:"constraint:OnDelete:CASCADE"`
}

// StepDependency links a step to another step it depends on.
type StepDependency struct {
	Base
	StepID      uint `gorm:"uniqueIndex;not null"`
	Step        Step `gorm:"foreignKey:StepID;constraint:OnDelete:CASCADE"`
	DependsOnID uint `gorm:"uniqueIndex;not null"`
	DependsOn   Step `gorm:"foreignKey:DependsOnID;constraint:OnDelete:CASCADE"`
}
