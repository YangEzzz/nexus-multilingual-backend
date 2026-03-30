package repository

import (
	"choice-matrix-backend/internal/models"

	"gorm.io/gorm"
)

type LanguageRepository struct {
	db *gorm.DB
}

func NewLanguageRepository(db *gorm.DB) *LanguageRepository {
	return &LanguageRepository{db: db}
}

// GetByProject returns all configured languages for a project
func (r *LanguageRepository) GetByProject(projectID uint) ([]models.Language, error) {
	var langs []models.Language
	err := r.db.Where("project_id = ?", projectID).Order("created_at asc").Find(&langs).Error
	return langs, err
}

// SetForProject replaces all language entries for a project with the given list.
// Existing translations are NOT deleted.
func (r *LanguageRepository) SetForProject(projectID uint, langs []models.Language) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Delete old language configs for this project
		if err := tx.Where("project_id = ?", projectID).Delete(&models.Language{}).Error; err != nil {
			return err
		}
		// Insert new ones
		if len(langs) == 0 {
			return nil
		}
		return tx.Create(&langs).Error
	})
}
