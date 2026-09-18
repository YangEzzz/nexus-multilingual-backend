package repository

import (
	"errors"

	"choice-matrix-backend/internal/models"

	"gorm.io/gorm"
)

type AvailableLanguageWithUsage struct {
	models.AvailableLanguage
	UsageCount int64 `json:"usage_count"`
}

type AvailableLanguageRepository struct {
	db *gorm.DB
}

func NewAvailableLanguageRepository(db *gorm.DB) *AvailableLanguageRepository {
	return &AvailableLanguageRepository{db: db}
}

func (r *AvailableLanguageRepository) List() ([]AvailableLanguageWithUsage, error) {
	var languages []models.AvailableLanguage
	if err := r.db.Order("sort_order asc, id asc").Find(&languages).Error; err != nil {
		return nil, err
	}

	result := make([]AvailableLanguageWithUsage, 0, len(languages))
	for _, language := range languages {
		var usageCount int64
		if err := r.db.Model(&models.Language{}).Distinct("project_id").Where("code = ?", language.Code).Count(&usageCount).Error; err != nil {
			return nil, err
		}
		result = append(result, AvailableLanguageWithUsage{
			AvailableLanguage: language,
			UsageCount:        usageCount,
		})
	}
	return result, nil
}

func (r *AvailableLanguageRepository) Create(language *models.AvailableLanguage) error {
	return r.db.Create(language).Error
}

func (r *AvailableLanguageRepository) FindByID(id uint) (*models.AvailableLanguage, error) {
	var language models.AvailableLanguage
	if err := r.db.First(&language, id).Error; err != nil {
		return nil, err
	}
	return &language, nil
}

func (r *AvailableLanguageRepository) Update(language *models.AvailableLanguage, previousCode string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(language).Error; err != nil {
			return err
		}
		if previousCode == language.Code {
			return tx.Model(&models.Language{}).
				Where("code = ?", language.Code).
				Update("name", language.Name).Error
		}
		return nil
	})
}

func (r *AvailableLanguageRepository) CountUsage(code string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Language{}).Distinct("project_id").Where("code = ?", code).Count(&count).Error
	return count, err
}

func (r *AvailableLanguageRepository) Delete(id uint) error {
	language, err := r.FindByID(id)
	if err != nil {
		return err
	}
	usageCount, err := r.CountUsage(language.Code)
	if err != nil {
		return err
	}
	if usageCount > 0 {
		return ErrAvailableLanguageInUse
	}
	return r.db.Delete(language).Error
}

func (r *AvailableLanguageRepository) FindByCodes(codes []string) (map[string]models.AvailableLanguage, error) {
	result := make(map[string]models.AvailableLanguage, len(codes))
	if len(codes) == 0 {
		return result, nil
	}

	var languages []models.AvailableLanguage
	if err := r.db.Where("code IN ?", codes).Find(&languages).Error; err != nil {
		return nil, err
	}
	for _, language := range languages {
		result[language.Code] = language
	}
	return result, nil
}

var ErrAvailableLanguageInUse = errors.New("available language is used by projects")
