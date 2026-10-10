package repository

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"choice-matrix-backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type LocaleReleaseRepository struct{ db *gorm.DB }

func NewLocaleReleaseRepository(db *gorm.DB) *LocaleReleaseRepository {
	return &LocaleReleaseRepository{db: db}
}

func cloneLocaleData(previous models.LocaleData) models.LocaleData {
	copy := make(models.LocaleData, len(previous))
	for language, modules := range previous {
		copy[language] = make(map[string]map[string]string, len(modules))
		for module, entries := range modules {
			copy[language][module] = make(map[string]string, len(entries))
			for key, value := range entries {
				copy[language][module][key] = value
			}
		}
	}
	return copy
}

// mergePublishedTerms only overlays non-empty translations. Absence never deletes
// a value from the previous public release.
func mergePublishedTerms(data models.LocaleData, terms []models.Term, allowed map[string]bool) {
	for _, term := range terms {
		for _, translation := range term.Translations {
			if !allowed[translation.LanguageCode] || strings.TrimSpace(translation.Content) == "" {
				continue
			}
			if data[translation.LanguageCode] == nil {
				data[translation.LanguageCode] = make(map[string]map[string]string)
			}
			if data[translation.LanguageCode][term.Module] == nil {
				data[translation.LanguageCode][term.Module] = make(map[string]string)
			}
			data[translation.LanguageCode][term.Module][term.Key] = translation.Content
		}
	}
}

func renderLocaleJSON(modules map[string]map[string]string) (string, error) {
	root := make(map[string]interface{})
	for key, value := range modules[""] {
		root[key] = value
	}
	for module, entries := range modules {
		if module == "" {
			continue
		}
		if _, collision := root[module]; collision {
			return "", fmt.Errorf("全局 key %q 与模块名冲突", module)
		}
		root[module] = entries
	}
	content, err := json.MarshalIndent(root, "", "  ")
	return string(content), err
}

func (r *LocaleReleaseRepository) Latest(projectID uint) (*models.LocaleRelease, error) {
	var release models.LocaleRelease
	err := r.db.Where("project_id = ?", projectID).Order("version DESC").Preload("Files").First(&release).Error
	return &release, err
}

func (r *LocaleReleaseRepository) Publish(projectID, userID uint) (*models.LocaleRelease, error) {
	var release models.LocaleRelease
	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Serialize concurrent publishes of the same project.
		var project models.I18nProject
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&project, projectID).Error; err != nil {
			return err
		}
		var previous models.LocaleRelease
		err := tx.Where("project_id = ?", projectID).Order("version DESC").First(&previous).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		data := cloneLocaleData(previous.Data)
		var languages []models.Language
		if err := tx.Where("project_id = ?", projectID).Find(&languages).Error; err != nil {
			return err
		}
		if len(languages) == 0 {
			return fmt.Errorf("项目尚未配置语言")
		}
		allowed := make(map[string]bool)
		for _, language := range languages {
			allowed[language.Code] = true
			if data[language.Code] == nil {
				data[language.Code] = make(map[string]map[string]string)
			}
		}
		// Languages in an earlier release remain available, just like older keys.
		for language := range data {
			allowed[language] = true
		}
		var terms []models.Term
		if err := tx.Where("project_id = ? AND status = ?", projectID, models.TermStatusPublished).
			Preload("Translations").Order("id ASC").Find(&terms).Error; err != nil {
			return err
		}
		mergePublishedTerms(data, terms, allowed)
		release = models.LocaleRelease{
			ProjectID: projectID, Version: previous.Version + 1,
			Data: data, PublishedBy: userID,
		}
		if err := tx.Create(&release).Error; err != nil {
			return err
		}
		for language, modules := range data {
			content, err := renderLocaleJSON(modules)
			if err != nil {
				return err
			}
			hash := sha256.Sum256([]byte(content))
			file := models.LocaleReleaseFile{
				ReleaseID: release.ID, LanguageCode: language,
				Content: content, SHA256: hex.EncodeToString(hash[:]),
			}
			if err := tx.Create(&file).Error; err != nil {
				return err
			}
			release.Files = append(release.Files, file)
		}
		return tx.Create(&models.ProjectHistoryLog{
			ProjectID: projectID,
			UserID:    userID,
			Action:    fmt.Sprintf("发布了线上语言文件 v%d", release.Version),
			ItemCount: len(terms),
		}).Error
	}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	return &release, err
}

func (r *LocaleReleaseRepository) PublicFile(projectCode, languageCode string) (*models.LocaleReleaseFile, error) {
	var file models.LocaleReleaseFile
	err := r.db.Model(&models.LocaleReleaseFile{}).
		Joins("JOIN locale_releases ON locale_releases.id = locale_release_files.release_id").
		Joins("JOIN i18n_projects ON i18n_projects.id = locale_releases.project_id").
		Where("i18n_projects.code = ? AND locale_release_files.language_code = ?", projectCode, languageCode).
		Order("locale_releases.version DESC").First(&file).Error
	return &file, err
}
