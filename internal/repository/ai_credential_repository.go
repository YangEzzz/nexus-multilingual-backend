package repository

import (
	"choice-matrix-backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AICredentialRepository struct {
	db *gorm.DB
}

func NewAICredentialRepository(db *gorm.DB) *AICredentialRepository {
	return &AICredentialRepository{db: db}
}

func (r *AICredentialRepository) FindByUserID(userID uint) (*models.AICredential, error) {
	credential := &models.AICredential{}
	if err := r.db.Where("user_id = ?", userID).First(credential).Error; err != nil {
		return nil, err
	}
	return credential, nil
}

func (r *AICredentialRepository) Save(credential *models.AICredential) error {
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"encrypted_key", "key_last_four", "model", "updated_at"}),
	}).Create(credential).Error
}

func (r *AICredentialRepository) DeleteByUserID(userID uint) error {
	return r.db.Where("user_id = ?", userID).Delete(&models.AICredential{}).Error
}
