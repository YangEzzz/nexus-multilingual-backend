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
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Terms reference the project, and translations/history logs reference
		// those terms. Remove the dependent rows first so this works with the
		// existing database constraints (which do not all cascade on delete).
		var termIDs []uint
		if err := tx.Model(&models.Term{}).
			Where("project_id = ?", id).
			Pluck("id", &termIDs).Error; err != nil {
			return err
		}

		if len(termIDs) > 0 {
			if err := tx.Where("term_id IN ?", termIDs).Delete(&models.Translation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("term_id IN ?", termIDs).Delete(&models.HistoryLog{}).Error; err != nil {
				return err
			}
			if err := tx.Where("id IN ?", termIDs).Delete(&models.Term{}).Error; err != nil {
				return err
			}
		}

		if err := tx.Where("project_id = ?", id).Delete(&models.Language{}).Error; err != nil {
			return err
		}
		if err := tx.Where("project_id = ?", id).Delete(&models.ProjectHistoryLog{}).Error; err != nil {
			return err
		}

		return tx.Delete(&models.I18nProject{}, id).Error
	})
}
