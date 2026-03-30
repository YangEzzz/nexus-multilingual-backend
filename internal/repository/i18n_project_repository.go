package repository

import (
	"choice-matrix-backend/internal/models"

	"gorm.io/gorm"
)

type I18nProjectRepository struct {
	db *gorm.DB
}

func NewI18nProjectRepository(db *gorm.DB) *I18nProjectRepository {
	return &I18nProjectRepository{db: db}
}

func (r *I18nProjectRepository) List() ([]models.I18nProject, error) {
	var projects []models.I18nProject
	if err := r.db.Preload("Languages").Order("created_at desc").Find(&projects).Error; err != nil {
		return nil, err
	}
	return projects, nil
}

func (r *I18nProjectRepository) FindByID(id uint) (*models.I18nProject, error) {
	var project models.I18nProject
	if err := r.db.Preload("Languages").First(&project, id).Error; err != nil {
		return nil, err
	}
	return &project, nil
}

func (r *I18nProjectRepository) Create(project *models.I18nProject) error {
	return r.db.Create(project).Error
}

func (r *I18nProjectRepository) Update(project *models.I18nProject) error {
	return r.db.Save(project).Error
}

func (r *I18nProjectRepository) Delete(id uint) error {
	return r.db.Delete(&models.I18nProject{}, id).Error
}
