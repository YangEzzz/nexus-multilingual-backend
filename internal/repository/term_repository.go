package repository

import (
	"choice-matrix-backend/internal/models"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TermRepository struct {
	db *gorm.DB
}

func NewTermRepository(db *gorm.DB) *TermRepository {
	return &TermRepository{db: db}
}

// ListByProject returns all terms for a project, preloading their translations
func (r *TermRepository) ListByProject(projectID uint) ([]models.Term, error) {
	var terms []models.Term
	err := r.db.Preload("Translations").
		Preload("HistoryLogs", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at DESC")
		}).
		Where("project_id = ?", projectID).
		Order("created_at DESC").
		Find(&terms).Error
	return terms, err
}

// ProjectStats holds summary data for the dashboard
type ProjectStats struct {
	Total     int64            `json:"total"`
	Draft     int64            `json:"draft"`
	Pending   int64            `json:"pending"`
	Review    int64            `json:"review"`
	Published int64            `json:"published"`
	// LangCoverage: map[langCode] -> translated count
	LangCoverage map[string]int64 `json:"lang_coverage"`
}

// Stats returns lightweight aggregate stats for a project
func (r *TermRepository) Stats(projectID uint) (*ProjectStats, error) {
	stats := &ProjectStats{LangCoverage: make(map[string]int64)}

	type statusCount struct {
		Status models.TermStatus
		Count  int64
	}
	var rows []statusCount
	if err := r.db.Model(&models.Term{}).
		Select("status, count(*) as count").
		Where("project_id = ?", projectID).
		Group("status").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		stats.Total += row.Count
		switch row.Status {
		case models.TermStatusDraft:
			stats.Draft = row.Count
		case models.TermStatusPending:
			stats.Pending = row.Count
		case models.TermStatusReview:
			stats.Review = row.Count
		case models.TermStatusPublished:
			stats.Published = row.Count
		}
	}

	// Language coverage: count non-empty translations per language
	type langCount struct {
		LanguageCode string
		Count        int64
	}
	var langRows []langCount
	if err := r.db.Model(&models.Translation{}).
		Select("translations.language_code, count(*) as count").
		Joins("JOIN terms ON terms.id = translations.term_id").
		Where("terms.project_id = ? AND translations.content != ''", projectID).
		Group("translations.language_code").
		Scan(&langRows).Error; err != nil {
		return nil, err
	}
	for _, lr := range langRows {
		stats.LangCoverage[lr.LanguageCode] = lr.Count
	}

	return stats, nil
}

type DashboardLanguageCoverage struct {
	Code            string `json:"code"`
	Name            string `json:"name"`
	IsSource        bool   `json:"is_source"`
	TranslatedCount int64  `json:"translated_count"`
	MissingCount    int64  `json:"missing_count"`
	Progress        int64  `json:"progress"`
}

type DashboardTrendPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type DashboardFocus struct {
	TermsReadyForReview int64 `json:"terms_ready_for_review"`
	TermsPending        int64 `json:"terms_pending"`
	DraftTerms          int64 `json:"draft_terms"`
	PublishedTerms      int64 `json:"published_terms"`
	RecentChanges24h    int64 `json:"recent_changes_24h"`
	StaleTerms          int64 `json:"stale_terms"`
}

type ProjectDashboard struct {
	Stats           *ProjectStats               `json:"stats"`
	Languages       []DashboardLanguageCoverage `json:"languages"`
	RecentLogs      []models.HistoryLog         `json:"recent_logs"`
	ActivityTrend   []DashboardTrendPoint       `json:"activity_trend"`
	Focus           DashboardFocus              `json:"focus"`
	LastActivityAt  *time.Time                  `json:"last_activity_at"`
}


// FindByID gets a single term by its ID (not including historical logs)
func (r *TermRepository) FindByID(id uint) (*models.Term, error) {
	var term models.Term
	err := r.db.Preload("Translations").First(&term, id).Error
	return &term, err
}

func (r *TermRepository) Dashboard(projectID uint, recentLimit int) (*ProjectDashboard, error) {
	stats, err := r.Stats(projectID)
	if err != nil {
		return nil, err
	}

	dashboard := &ProjectDashboard{
		Stats: stats,
	}

	var projectLangs []models.Language
	if err := r.db.Where("project_id = ?", projectID).Order("created_at asc").Find(&projectLangs).Error; err != nil {
		return nil, err
	}

	type langCount struct {
		LanguageCode string
		Count        int64
	}
	var langRows []langCount
	if err := r.db.Model(&models.Translation{}).
		Select("translations.language_code, count(*) as count").
		Joins("JOIN terms ON terms.id = translations.term_id").
		Where("terms.project_id = ? AND translations.content != ''", projectID).
		Group("translations.language_code").
		Scan(&langRows).Error; err != nil {
		return nil, err
	}
	langCounts := make(map[string]int64, len(langRows))
	for _, row := range langRows {
		langCounts[row.LanguageCode] = row.Count
	}

	totalTerms := stats.Total
	for _, lang := range projectLangs {
		translated := langCounts[lang.Code]
		missing := totalTerms - translated
		if missing < 0 {
			missing = 0
		}
		progress := int64(0)
		if totalTerms > 0 {
			progress = translated * 100 / totalTerms
		}
		dashboard.Languages = append(dashboard.Languages, DashboardLanguageCoverage{
			Code:            lang.Code,
			Name:            lang.Name,
			IsSource:        lang.IsSource,
			TranslatedCount: translated,
			MissingCount:    missing,
			Progress:        progress,
		})
	}

	logs, err := r.ProjectLogs(projectID, recentLimit)
	if err != nil {
		return nil, err
	}
	dashboard.RecentLogs = logs
	if len(logs) > 0 {
		dashboard.LastActivityAt = &logs[0].CreatedAt
	}

	now := time.Now()
	var recentChanges24h int64
	if err := r.db.Model(&models.HistoryLog{}).
		Joins("JOIN terms ON terms.id = history_logs.term_id").
		Where("terms.project_id = ? AND history_logs.created_at >= ?", projectID, now.Add(-24*time.Hour)).
		Count(&recentChanges24h).Error; err != nil {
		return nil, err
	}

	var staleTerms int64
	if err := r.db.Model(&models.Term{}).
		Where("project_id = ? AND status <> ? AND updated_at < ?", projectID, models.TermStatusPublished, now.Add(-7*24*time.Hour)).
		Count(&staleTerms).Error; err != nil {
		return nil, err
	}

	dashboard.Focus = DashboardFocus{
		TermsReadyForReview: stats.Review,
		TermsPending:        stats.Pending,
		DraftTerms:          stats.Draft,
		PublishedTerms:      stats.Published,
		RecentChanges24h:    recentChanges24h,
		StaleTerms:          staleTerms,
	}

	type trendRow struct {
		Date  string
		Count int64
	}
	var trendRows []trendRow
	if err := r.db.Model(&models.HistoryLog{}).
		Select("TO_CHAR(history_logs.created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date, count(*) as count").
		Joins("JOIN terms ON terms.id = history_logs.term_id").
		Where("terms.project_id = ? AND history_logs.created_at >= ?", projectID, now.AddDate(0, 0, -6)).
		Group("date").
		Order("date asc").
		Scan(&trendRows).Error; err != nil {
		return nil, err
	}

	trendMap := make(map[string]int64, len(trendRows))
	for _, row := range trendRows {
		trendMap[row.Date] = row.Count
	}

	for i := 6; i >= 0; i-- {
		day := now.AddDate(0, 0, -i).Format("2006-01-02")
		dashboard.ActivityTrend = append(dashboard.ActivityTrend, DashboardTrendPoint{
			Date:  day,
			Count: trendMap[day],
		})
	}

	return dashboard, nil
}

// Create inserts a new term along with its initial translations and history log
func (r *TermRepository) Create(term *models.Term) error {
	return r.db.Create(term).Error
}

// Update saves changes to a term's metadata, upserts its translations,
// and auto-computes status based on project language coverage.
func (r *TermRepository) Update(term *models.Term, translations map[string]string, userID uint, action string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		return r.updateInTx(tx, term, translations, userID, action)
	})
}

// updateInTx is the internal logic for updating a single term inside a transaction
func (r *TermRepository) updateInTx(tx *gorm.DB, term *models.Term, translations map[string]string, userID uint, action string) error {
	// 1. Fetch existing translations to detect what actually changed
	var existing []models.Translation
	if err := tx.Where("term_id = ?", term.ID).Find(&existing).Error; err != nil {
		return err
	}
	
	existingMap := make(map[string]string)
	for _, t := range existing {
		existingMap[t.LanguageCode] = t.Content
	}

	// 2. Build list of translations that are new or updated
	var transList []models.Translation
	for langCode, content := range translations {
		if existingContent, exists := existingMap[langCode]; !exists || existingContent != content {
			transList = append(transList, models.Translation{
				TermID:       term.ID,
				LanguageCode: langCode,
				Content:      content,
			})
		}
	}

	// 3. Batch Upsert translations ONLY if there's an actual change
	if len(transList) > 0 {
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "term_id"}, {Name: "language_code"}},
			DoUpdates: clause.AssignmentColumns([]string{"content"}),
		}).Create(&transList).Error; err != nil {
			return err
		}
	}

	// 2. Auto-compute status based on project language coverage
	//    (skip if caller explicitly set "published" — that is handled by Publish endpoint)
	if term.Status != models.TermStatusPublished {
		// Fetch all languages configured for this project
		var projectLangs []models.Language
		if err := tx.Where("project_id = ?", term.ProjectID).Find(&projectLangs).Error; err != nil {
			return err
		}

		// Fetch all non-empty translations for this term
		var filledTranslations []models.Translation
		if err := tx.Where("term_id = ? AND content != ''", term.ID).Find(&filledTranslations).Error; err != nil {
			return err
		}
		filledSet := make(map[string]bool)
		for _, t := range filledTranslations {
			filledSet[t.LanguageCode] = true
		}

		allFilled := len(projectLangs) > 0
		for _, lang := range projectLangs {
			if !filledSet[lang.Code] {
				allFilled = false
				break
			}
		}

		if allFilled {
			term.Status = models.TermStatusReview
		} else {
			term.Status = models.TermStatusPending
		}
	} else {
		// Published → review if content changed
		// Check if content actually changed in metadata or translations
		// For now, simpler to always drop to review if Update is called on a published term
		term.Status = models.TermStatusReview
	}

	// 3. Update term metadata only (avoid overwriting created_at with zero value)
	if err := tx.Model(&models.Term{}).Where("id = ?", term.ID).Updates(map[string]interface{}{
		"module":      term.Module,
		"key":         term.Key,
		"description": term.Description,
		"status":      term.Status,
	}).Error; err != nil {
		return err
	}

	// 4. Append history log
	logEntry := models.HistoryLog{
		TermID: term.ID,
		UserID: userID,
		Action: action,
	}
	return tx.Create(&logEntry).Error
}

// BatchUpdate saves changes to multiple terms at once.
type BatchUpdateTerm struct {
	ID           uint
	ProjectID    uint
	Module       string
	Key          string
	Description  string
	Status       models.TermStatus
	Translations map[string]string
}

func (r *TermRepository) BatchUpdate(projectID uint, updateData []BatchUpdateTerm, userID uint) (int, error) {
	count := 0
	err := r.db.Transaction(func(tx *gorm.DB) error {
		for _, data := range updateData {
			term := &models.Term{
				ID:          data.ID,
				ProjectID:   projectID,
				Module:      data.Module,
				Key:         data.Key,
				Description: data.Description,
				Status:      data.Status,
			}
			if err := r.updateInTx(tx, term, data.Translations, userID, "在批量编辑模式下更新了内容"); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

// Publish sets a term's status to published (manual step)
func (r *TermRepository) Publish(termID uint, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Term{}).Where("id = ?", termID).
			Update("status", models.TermStatusPublished).Error; err != nil {
			return err
		}
		logEntry := models.HistoryLog{
			TermID: termID,
			UserID: userID,
			Action: "手动发布了词条",
		}
		return tx.Create(&logEntry).Error
	})
}

// BatchPublish sets multiple terms to published status
func (r *TermRepository) BatchPublish(termIDs []uint, userID uint) error {
	if len(termIDs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Update status
		if err := tx.Model(&models.Term{}).Where("id IN ?", termIDs).
			Update("status", models.TermStatusPublished).Error; err != nil {
			return err
		}
		
		// Insert logs for each term
		var logs []models.HistoryLog
		for _, termID := range termIDs {
			logs = append(logs, models.HistoryLog{
				TermID: termID,
				UserID: userID,
				Action: "手动发布了词条",
			})
		}
		return tx.Create(&logs).Error
	})
}

// Delete removes a term along with its translations and history logs
func (r *TermRepository) Delete(termID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("term_id = ?", termID).Delete(&models.Translation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("term_id = ?", termID).Delete(&models.HistoryLog{}).Error; err != nil {
			return err
		}
		return tx.Delete(&models.Term{}, termID).Error
	})
}

// BatchDelete removes multiple terms and their associated records
func (r *TermRepository) BatchDelete(termIDs []uint) error {
	if len(termIDs) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("term_id IN ?", termIDs).Delete(&models.Translation{}).Error; err != nil {
			return err
		}
		if err := tx.Where("term_id IN ?", termIDs).Delete(&models.HistoryLog{}).Error; err != nil {
			return err
		}
		return tx.Where("id IN ?", termIDs).Delete(&models.Term{}).Error
	})
}

// ImportJSON bulk-upserts terms and translations from a JSON flat map.
// It returns the count of newly created terms and updated translations.
func (r *TermRepository) ImportJSON(projectID uint, langCode string, data map[string]string) (created, updated int, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		for key, content := range data {
			if key == "" || content == "" {
				continue
			}

			// 1. Find or create the Term
			var term models.Term
			result := tx.Where("project_id = ? AND key = ?", projectID, key).First(&term)

			if result.Error != nil {
				// Term doesn't exist — create it
				term = models.Term{
					ProjectID: projectID,
					Key:       key,
					Module:    "",
					Status:    models.TermStatusPending,
				}
				if err := tx.Create(&term).Error; err != nil {
					return err
				}
				created++
			}

			// 2. Upsert the translation for this language
			var trans models.Translation
			findResult := tx.Where("term_id = ? AND language_code = ?", term.ID, langCode).First(&trans)
			if findResult.Error != nil {
				// Create new translation
				trans = models.Translation{
					TermID:       term.ID,
					LanguageCode: langCode,
					Content:      content,
				}
				if err := tx.Create(&trans).Error; err != nil {
					return err
				}
			} else {
				// Update existing translation
				if err := tx.Model(&trans).Update("content", content).Error; err != nil {
					return err
				}
			}
			updated++
		}
		return nil
	})
	return
}

// ProjectLogs returns recent logs for a project
func (r *TermRepository) ProjectLogs(projectID uint, limit int) ([]models.HistoryLog, error) {
	var logs []models.HistoryLog
	err := r.db.
		Joins("JOIN terms ON terms.id = history_logs.term_id").
		Where("terms.project_id = ?", projectID).
		Preload("User").
		Preload("Term").
		Order("history_logs.created_at DESC").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}
