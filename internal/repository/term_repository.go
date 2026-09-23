package repository

import (
	"choice-matrix-backend/internal/models"
	"choice-matrix-backend/internal/utils"
	"fmt"
	"sort"
	"strings"
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

type TermListFilter struct {
	Page          int    `json:"page,omitempty"`
	PageSize      int    `json:"page_size,omitempty"`
	Search        string `json:"search,omitempty"`
	ChineseSearch string `json:"chinese_search,omitempty"`
	EnglishSearch string `json:"english_search,omitempty"`
	Module        string `json:"module,omitempty"`
	Status        string `json:"status,omitempty"`
	ExcludeDraft  bool   `json:"exclude_draft,omitempty"`
}

type TermListSummary struct {
	Total            int64            `json:"total"`
	StatusCounts     map[string]int64 `json:"status_counts"`
	ReadyReviewCount int64            `json:"ready_review_count"`
}

type TermListResult struct {
	Items          []models.Term    `json:"items"`
	Total          int64            `json:"total"`
	Page           int              `json:"page"`
	PageSize       int              `json:"page_size"`
	TotalPages     int              `json:"total_pages"`
	StatusCounts   map[string]int64 `json:"status_counts"`
	Modules        []string         `json:"modules"`
	ProjectSummary TermListSummary  `json:"project_summary"`
}

type BulkOperation string

const (
	BulkOperationApproveReview BulkOperation = "approve_review"
	BulkOperationPublish       BulkOperation = "publish"
	BulkOperationDelete        BulkOperation = "delete"
)

type TermBulkScope struct {
	Mode        string         `json:"mode"`
	IDs         []uint         `json:"ids,omitempty"`
	Filter      TermListFilter `json:"filter,omitempty"`
	ExcludedIDs []uint         `json:"excluded_ids,omitempty"`
}

type TermBulkResult struct {
	Operation    BulkOperation    `json:"operation"`
	Matched      int              `json:"matched"`
	Executable   int              `json:"executable"`
	Succeeded    int              `json:"succeeded"`
	Skipped      int              `json:"skipped"`
	StatusCounts map[string]int64 `json:"status_counts"`
	SkipReasons  map[string]int   `json:"skip_reasons"`
}

func IsValidBulkOperation(operation BulkOperation) bool {
	switch operation {
	case BulkOperationApproveReview, BulkOperationPublish, BulkOperationDelete:
		return true
	default:
		return false
	}
}

const readyReviewCondition = `
	terms.status NOT IN (?, ?)
	AND TRIM(terms.key) <> ''
	AND NOT EXISTS (
		SELECT 1 FROM languages
		WHERE languages.project_id = terms.project_id
		AND NOT EXISTS (
			SELECT 1 FROM translations
			WHERE translations.term_id = terms.id
			AND translations.language_code = languages.code
			AND TRIM(translations.content) <> ''
		)
	)`

func termListBaseQuery(db *gorm.DB, projectID uint, filter TermListFilter) *gorm.DB {
	query := db.Model(&models.Term{}).Where("terms.project_id = ?", projectID)
	if filter.ExcludeDraft {
		query = query.Where("terms.status <> ?", models.TermStatusDraft)
	}
	if search := strings.TrimSpace(filter.Search); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where("LOWER(terms.key) LIKE ? OR LOWER(terms.module) LIKE ?", pattern, pattern)
	}
	if search := strings.TrimSpace(filter.ChineseSearch); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(`EXISTS (
			SELECT 1 FROM translations
			WHERE translations.term_id = terms.id
			AND translations.language_code IN ?
			AND LOWER(translations.content) LIKE ?
		)`, []string{"cn", "zh", "zh-CN", "cht"}, pattern)
	}
	if search := strings.TrimSpace(filter.EnglishSearch); search != "" {
		pattern := "%" + strings.ToLower(search) + "%"
		query = query.Where(`EXISTS (
			SELECT 1 FROM translations
			WHERE translations.term_id = terms.id
			AND translations.language_code = ?
			AND LOWER(translations.content) LIKE ?
		)`, "en", pattern)
	}
	if filter.Module == "__none__" {
		query = query.Where("terms.module = ''")
	} else if filter.Module != "" && filter.Module != "all" {
		query = query.Where("terms.module = ?", filter.Module)
	}
	if filter.Status == "ready_review" {
		query = query.Where(readyReviewCondition, models.TermStatusReview, models.TermStatusPublished)
	} else if filter.Status != "" && filter.Status != "all" {
		query = query.Where("terms.status = ?", filter.Status)
	}
	return query
}

func termStatusCounts(query *gorm.DB) (map[string]int64, error) {
	type statusCount struct {
		Status string
		Count  int64
	}
	rows := make([]statusCount, 0, 4)
	if err := query.Select("terms.status, count(*) AS count").Group("terms.status").Scan(&rows).Error; err != nil {
		return nil, err
	}
	counts := map[string]int64{"draft": 0, "pending": 0, "review": 0, "published": 0}
	for _, row := range rows {
		counts[row.Status] = row.Count
	}
	return counts, nil
}

// ListByProjectPage returns one filtered page plus the aggregate data required
// to render pagination, selection scope, filters, and project summary cards.
func (r *TermRepository) ListByProjectPage(projectID uint, filter TermListFilter) (*TermListResult, error) {
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}

	filteredQuery := termListBaseQuery(r.db, projectID, filter)
	var total int64
	if err := filteredQuery.Count(&total).Error; err != nil {
		return nil, err
	}
	statusCounts, err := termStatusCounts(termListBaseQuery(r.db, projectID, filter))
	if err != nil {
		return nil, err
	}

	var modules []string
	moduleQuery := r.db.Model(&models.Term{}).Where("project_id = ?", projectID)
	if filter.ExcludeDraft {
		moduleQuery = moduleQuery.Where("status <> ?", models.TermStatusDraft)
	}
	if err := moduleQuery.Distinct("module").Order("module ASC").Pluck("module", &modules).Error; err != nil {
		return nil, err
	}

	projectFilter := TermListFilter{ExcludeDraft: filter.ExcludeDraft}
	projectQuery := termListBaseQuery(r.db, projectID, projectFilter)
	var projectTotal int64
	if err := projectQuery.Count(&projectTotal).Error; err != nil {
		return nil, err
	}
	projectStatusCounts, err := termStatusCounts(termListBaseQuery(r.db, projectID, projectFilter))
	if err != nil {
		return nil, err
	}
	var readyReviewCount int64
	if err := termListBaseQuery(r.db, projectID, projectFilter).
		Where(readyReviewCondition, models.TermStatusReview, models.TermStatusPublished).
		Count(&readyReviewCount).Error; err != nil {
		return nil, err
	}

	totalPages := int((total + int64(filter.PageSize) - 1) / int64(filter.PageSize))
	if totalPages < 1 {
		totalPages = 1
	}
	if filter.Page > totalPages {
		filter.Page = totalPages
	}

	items := make([]models.Term, 0, filter.PageSize)
	if err := termListBaseQuery(r.db, projectID, filter).
		Preload("Translations").
		Preload("HistoryLogs", func(db *gorm.DB) *gorm.DB { return db.Order("created_at DESC") }).
		Order("terms.created_at DESC").
		Offset((filter.Page - 1) * filter.PageSize).
		Limit(filter.PageSize).
		Find(&items).Error; err != nil {
		return nil, err
	}

	return &TermListResult{
		Items:        items,
		Total:        total,
		Page:         filter.Page,
		PageSize:     filter.PageSize,
		TotalPages:   totalPages,
		StatusCounts: statusCounts,
		Modules:      modules,
		ProjectSummary: TermListSummary{
			Total:            projectTotal,
			StatusCounts:     projectStatusCounts,
			ReadyReviewCount: readyReviewCount,
		},
	}, nil
}

func termBulkScopeQuery(db *gorm.DB, projectID uint, scope TermBulkScope) (*gorm.DB, error) {
	var query *gorm.DB
	switch scope.Mode {
	case "explicit":
		if len(scope.IDs) == 0 {
			return nil, fmt.Errorf("请至少选择一个词条")
		}
		query = db.Model(&models.Term{}).
			Where("terms.project_id = ?", projectID).
			Where("terms.id IN ?", scope.IDs)
	case "filtered":
		query = termListBaseQuery(db, projectID, scope.Filter)
	default:
		return nil, fmt.Errorf("不支持的选择范围")
	}

	if len(scope.ExcludedIDs) > 0 {
		query = query.Where("terms.id NOT IN ?", scope.ExcludedIDs)
	}
	return query, nil
}

func loadScopedTerms(db *gorm.DB, projectID uint, scope TermBulkScope, lock bool) ([]models.Term, error) {
	query, err := termBulkScopeQuery(db, projectID, scope)
	if err != nil {
		return nil, err
	}
	if lock {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	terms := make([]models.Term, 0)
	err = query.Preload("Translations").Order("terms.id ASC").Find(&terms).Error
	return terms, err
}

func termTranslationMap(term models.Term) map[string]string {
	translations := make(map[string]string, len(term.Translations))
	for _, translation := range term.Translations {
		translations[translation.LanguageCode] = translation.Content
	}
	return translations
}

func hasMissingProjectTranslation(term models.Term, projectLanguages []models.Language) bool {
	translations := termTranslationMap(term)
	for _, language := range projectLanguages {
		if strings.TrimSpace(translations[language.Code]) == "" {
			return true
		}
	}
	return false
}

func hasPlaceholderMismatch(term models.Term, projectLanguages []models.Language) bool {
	translations := termTranslationMap(term)
	source := pickPlaceholderSource(term, projectLanguages, translations)
	if len(utils.ExtractPlaceholders(source)) == 0 {
		return false
	}
	for _, language := range projectLanguages {
		missing, extra := utils.DiffPlaceholders(source, translations[language.Code])
		if len(missing) > 0 || len(extra) > 0 {
			return true
		}
	}
	return false
}

func bulkSkipReason(term models.Term, projectLanguages []models.Language, operation BulkOperation) string {
	switch operation {
	case BulkOperationApproveReview:
		if term.Status == models.TermStatusReview || term.Status == models.TermStatusPublished {
			return "status_not_allowed"
		}
	case BulkOperationPublish:
		if term.Status != models.TermStatusReview {
			return "status_not_allowed"
		}
	case BulkOperationDelete:
		if term.Status == models.TermStatusPublished {
			return "published_protected"
		}
		return ""
	}

	if strings.TrimSpace(term.Key) == "" {
		return "missing_key"
	}
	if hasMissingProjectTranslation(term, projectLanguages) {
		return "missing_translation"
	}
	if hasPlaceholderMismatch(term, projectLanguages) {
		return "placeholder_mismatch"
	}
	return ""
}

func evaluateBulkTerms(terms []models.Term, projectLanguages []models.Language, operation BulkOperation) (TermBulkResult, []models.Term) {
	result := TermBulkResult{
		Operation:    operation,
		Matched:      len(terms),
		StatusCounts: map[string]int64{"draft": 0, "pending": 0, "review": 0, "published": 0},
		SkipReasons:  make(map[string]int),
	}
	eligible := make([]models.Term, 0, len(terms))
	for _, term := range terms {
		result.StatusCounts[string(term.Status)]++
		if reason := bulkSkipReason(term, projectLanguages, operation); reason != "" {
			result.SkipReasons[reason]++
			continue
		}
		eligible = append(eligible, term)
	}
	result.Executable = len(eligible)
	result.Skipped = result.Matched - result.Executable
	return result, eligible
}

func loadProjectLanguages(db *gorm.DB, projectID uint) ([]models.Language, error) {
	var languages []models.Language
	err := db.Where("project_id = ?", projectID).Order("id ASC").Find(&languages).Error
	return languages, err
}

func (r *TermRepository) PreviewBulk(projectID uint, operation BulkOperation, scope TermBulkScope) (*TermBulkResult, error) {
	if !IsValidBulkOperation(operation) {
		return nil, fmt.Errorf("不支持的批量操作")
	}
	terms, err := loadScopedTerms(r.db, projectID, scope, false)
	if err != nil {
		return nil, err
	}
	languages, err := loadProjectLanguages(r.db, projectID)
	if err != nil {
		return nil, err
	}
	result, _ := evaluateBulkTerms(terms, languages, operation)
	return &result, nil
}

// TermsForScope resolves an explicit or filtered selection on the server.
// Callers use this for read-only bulk operations such as export and AI jobs.
func (r *TermRepository) TermsForScope(projectID uint, scope TermBulkScope) ([]models.Term, error) {
	return loadScopedTerms(r.db, projectID, scope, false)
}

func (r *TermRepository) ProjectLanguages(projectID uint) ([]models.Language, error) {
	return loadProjectLanguages(r.db, projectID)
}

func sampleTermKeys(terms []models.Term, limit int) []string {
	if limit > len(terms) {
		limit = len(terms)
	}
	keys := make([]string, 0, limit)
	for _, term := range terms[:limit] {
		keys = append(keys, term.Key)
	}
	return keys
}

func (r *TermRepository) ExecuteBulk(projectID uint, userID uint, operation BulkOperation, scope TermBulkScope) (*TermBulkResult, error) {
	if !IsValidBulkOperation(operation) {
		return nil, fmt.Errorf("不支持的批量操作")
	}

	var result TermBulkResult
	err := r.db.Transaction(func(tx *gorm.DB) error {
		terms, err := loadScopedTerms(tx, projectID, scope, true)
		if err != nil {
			return err
		}
		languages, err := loadProjectLanguages(tx, projectID)
		if err != nil {
			return err
		}

		var eligible []models.Term
		result, eligible = evaluateBulkTerms(terms, languages, operation)
		if len(eligible) == 0 {
			return nil
		}

		ids := make([]uint, 0, len(eligible))
		for _, term := range eligible {
			ids = append(ids, term.ID)
		}

		action := ""
		switch operation {
		case BulkOperationApproveReview:
			action = "批量校对通过"
			if err := tx.Model(&models.Term{}).
				Where("project_id = ? AND id IN ?", projectID, ids).
				Update("status", models.TermStatusReview).Error; err != nil {
				return err
			}
		case BulkOperationPublish:
			action = "批量发布词条"
			if err := tx.Model(&models.Term{}).
				Where("project_id = ? AND id IN ?", projectID, ids).
				Update("status", models.TermStatusPublished).Error; err != nil {
				return err
			}
		case BulkOperationDelete:
			action = "批量删除词条"
			if err := tx.Where("term_id IN ?", ids).Delete(&models.Translation{}).Error; err != nil {
				return err
			}
			if err := tx.Where("term_id IN ?", ids).Delete(&models.HistoryLog{}).Error; err != nil {
				return err
			}
			if err := tx.Where("project_id = ? AND id IN ?", projectID, ids).Delete(&models.Term{}).Error; err != nil {
				return err
			}
		}

		if operation != BulkOperationDelete {
			logs := make([]models.HistoryLog, 0, len(ids))
			for _, id := range ids {
				logs = append(logs, models.HistoryLog{TermID: id, UserID: userID, Action: action})
			}
			if err := tx.Create(&logs).Error; err != nil {
				return err
			}
		}

		if err := tx.Create(&models.ProjectHistoryLog{
			ProjectID: projectID,
			UserID:    userID,
			Action:    action,
			ItemCount: len(eligible),
			TermKeys:  sampleTermKeys(eligible, 100),
		}).Error; err != nil {
			return err
		}
		result.Succeeded = len(eligible)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// ProjectStats holds summary data for the dashboard
type ProjectStats struct {
	Total     int64 `json:"total"`
	Draft     int64 `json:"draft"`
	Pending   int64 `json:"pending"`
	Review    int64 `json:"review"`
	Published int64 `json:"published"`
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
	Stats          *ProjectStats               `json:"stats"`
	Languages      []DashboardLanguageCoverage `json:"languages"`
	RecentLogs     []ProjectActivityLog        `json:"recent_logs"`
	ActivityTrend  []DashboardTrendPoint       `json:"activity_trend"`
	Focus          DashboardFocus              `json:"focus"`
	LastActivityAt *time.Time                  `json:"last_activity_at"`
}

// ProjectActivityLog is the shared response shape for term-level and project-level events.
type ProjectActivityLog struct {
	ID        string       `json:"id"`
	Scope     string       `json:"scope"`
	TermID    *uint        `json:"term_id,omitempty"`
	UserID    uint         `json:"user_id"`
	Action    string       `json:"action"`
	ItemCount int          `json:"item_count,omitempty"`
	TermKeys  []string     `json:"term_keys,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
	Term      *models.Term `json:"term,omitempty"`
	User      models.User  `json:"user,omitempty"`
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
	var termChanges24h int64
	if err := r.db.Model(&models.HistoryLog{}).
		Joins("JOIN terms ON terms.id = history_logs.term_id").
		Where("terms.project_id = ? AND history_logs.created_at >= ?", projectID, now.Add(-24*time.Hour)).
		Count(&termChanges24h).Error; err != nil {
		return nil, err
	}
	var projectChanges24h int64
	if err := r.db.Model(&models.ProjectHistoryLog{}).
		Where("project_id = ? AND created_at >= ?", projectID, now.Add(-24*time.Hour)).
		Count(&projectChanges24h).Error; err != nil {
		return nil, err
	}
	recentChanges24h := termChanges24h + projectChanges24h

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
	var projectTrendRows []trendRow
	if err := r.db.Model(&models.ProjectHistoryLog{}).
		Select("TO_CHAR(created_at AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD') as date, count(*) as count").
		Where("project_id = ? AND created_at >= ?", projectID, now.AddDate(0, 0, -6)).
		Group("date").
		Order("date asc").
		Scan(&projectTrendRows).Error; err != nil {
		return nil, err
	}

	trendMap := make(map[string]int64, len(trendRows)+len(projectTrendRows))
	for _, row := range trendRows {
		trendMap[row.Date] += row.Count
	}
	for _, row := range projectTrendRows {
		trendMap[row.Date] += row.Count
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

// BatchCreateTerm is one row in a batch-create request. ClientID is echoed back
// so the frontend can match persisted rows with its preview rows.
type BatchCreateTerm struct {
	ClientID     string            `json:"clientId"`
	Module       string            `json:"module"`
	Key          string            `json:"key"`
	Description  string            `json:"description"`
	Translations map[string]string `json:"translations"`
}

type BatchCreatedTerm struct {
	ClientID string      `json:"clientId"`
	Term     models.Term `json:"term"`
}

func batchCreateStatus(projectLangs []models.Language, filledLanguages map[string]bool) models.TermStatus {
	if len(filledLanguages) == 0 {
		return models.TermStatusDraft
	}

	for _, lang := range projectLangs {
		if !filledLanguages[lang.Code] {
			return models.TermStatusPending
		}
	}
	if len(projectLangs) > 0 {
		return models.TermStatusReview
	}
	return models.TermStatusPending
}

// BatchCreate creates a group of terms atomically. Status is derived on the
// server from project-language coverage so single and batch callers cannot
// publish content by supplying a status value.
func (r *TermRepository) BatchCreate(projectID uint, createData []BatchCreateTerm, userID uint) ([]BatchCreatedTerm, error) {
	created := make([]BatchCreatedTerm, 0, len(createData))
	termKeys := make([]string, 0, len(createData))

	err := r.db.Transaction(func(tx *gorm.DB) error {
		var projectLangs []models.Language
		if err := tx.Where("project_id = ?", projectID).Find(&projectLangs).Error; err != nil {
			return err
		}

		allowedLanguages := make(map[string]bool, len(projectLangs))
		for _, lang := range projectLangs {
			allowedLanguages[lang.Code] = true
		}

		for _, data := range createData {
			translations := make([]models.Translation, 0, len(data.Translations))
			filledLanguages := make(map[string]bool, len(data.Translations))
			for langCode, content := range data.Translations {
				if !allowedLanguages[langCode] {
					return fmt.Errorf("语言 %s 未配置在当前项目中", langCode)
				}
				content = strings.TrimSpace(content)
				if content == "" {
					continue
				}
				translations = append(translations, models.Translation{
					LanguageCode: langCode,
					Content:      content,
				})
				filledLanguages[langCode] = true
			}

			term := models.Term{
				ProjectID:    projectID,
				Module:       strings.TrimSpace(data.Module),
				Key:          strings.TrimSpace(data.Key),
				Description:  strings.TrimSpace(data.Description),
				Status:       batchCreateStatus(projectLangs, filledLanguages),
				Translations: translations,
			}

			if err := tx.Create(&term).Error; err != nil {
				return err
			}
			created = append(created, BatchCreatedTerm{ClientID: data.ClientID, Term: term})
			termKeys = append(termKeys, term.Key)
		}

		return tx.Create(&models.ProjectHistoryLog{
			ProjectID: projectID,
			UserID:    userID,
			Action:    "通过快捷批量创建录入",
			ItemCount: len(created),
			TermKeys:  termKeys,
		}).Error
	})

	return created, err
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

type placeholderValidationIssue struct {
	TermKey      string
	LanguageCode string
	Missing      []string
	Extra        []string
}

func (r *TermRepository) validatePublishPlaceholders(tx *gorm.DB, termIDs []uint) error {
	if len(termIDs) == 0 {
		return nil
	}

	var terms []models.Term
	if err := tx.Preload("Translations").Where("id IN ?", termIDs).Find(&terms).Error; err != nil {
		return err
	}

	var issues []placeholderValidationIssue
	for _, term := range terms {
		var projectLangs []models.Language
		if err := tx.Where("project_id = ?", term.ProjectID).Find(&projectLangs).Error; err != nil {
			return err
		}

		translations := make(map[string]string, len(term.Translations))
		for _, tr := range term.Translations {
			translations[tr.LanguageCode] = tr.Content
		}

		source := pickPlaceholderSource(term, projectLangs, translations)
		if len(utils.ExtractPlaceholders(source)) == 0 {
			continue
		}

		for _, lang := range projectLangs {
			missing, extra := utils.DiffPlaceholders(source, translations[lang.Code])
			if len(missing) > 0 || len(extra) > 0 {
				issues = append(issues, placeholderValidationIssue{
					TermKey:      term.Key,
					LanguageCode: lang.Code,
					Missing:      missing,
					Extra:        extra,
				})
			}
		}
	}

	if len(issues) == 0 {
		return nil
	}

	return fmt.Errorf("占位符不一致，无法发布: %s", formatPlaceholderIssues(issues))
}

func pickPlaceholderSource(term models.Term, projectLangs []models.Language, translations map[string]string) string {
	for _, lang := range projectLangs {
		if lang.IsSource && strings.TrimSpace(translations[lang.Code]) != "" {
			return translations[lang.Code]
		}
	}

	for _, code := range []string{"cn", "zh", "zh-CN", "en"} {
		if strings.TrimSpace(translations[code]) != "" {
			return translations[code]
		}
	}

	for _, tr := range term.Translations {
		if strings.TrimSpace(tr.Content) != "" {
			return tr.Content
		}
	}

	return term.Description
}

func formatPlaceholderIssues(issues []placeholderValidationIssue) string {
	parts := make([]string, 0, len(issues))
	for i, issue := range issues {
		if i >= 3 {
			parts = append(parts, "更多词条请回到工作台查看")
			break
		}

		details := make([]string, 0, 2)
		if len(issue.Missing) > 0 {
			details = append(details, "缺少 "+strings.Join(issue.Missing, ", "))
		}
		if len(issue.Extra) > 0 {
			details = append(details, "多出 "+strings.Join(issue.Extra, ", "))
		}
		parts = append(parts, fmt.Sprintf("%s/%s %s", issue.TermKey, issue.LanguageCode, strings.Join(details, "，")))
	}
	return strings.Join(parts, "；")
}

// Publish sets a term's status to published (manual step)
func (r *TermRepository) Publish(termID uint, userID uint) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := r.validatePublishPlaceholders(tx, []uint{termID}); err != nil {
			return err
		}

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
		if err := r.validatePublishPlaceholders(tx, termIDs); err != nil {
			return err
		}

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
	items := make([]ImportJSONItem, 0, len(data))
	for key, content := range data {
		items = append(items, ImportJSONItem{
			Key:     key,
			Content: content,
		})
	}
	return r.ImportJSONItems(projectID, langCode, items)
}

type ImportJSONItem struct {
	Module  string `json:"module"`
	Key     string `json:"key"`
	Content string `json:"content"`
}

// ImportJSONItems bulk-upserts terms and translations from normalized JSON import rows.
// It supports both flat keys and object imports with module information.
func (r *TermRepository) ImportJSONItems(projectID uint, langCode string, items []ImportJSONItem) (created, updated int, err error) {
	err = r.db.Transaction(func(tx *gorm.DB) error {
		for _, item := range items {
			key := strings.TrimSpace(item.Key)
			module := strings.TrimSpace(item.Module)
			content := item.Content
			if key == "" || content == "" {
				continue
			}

			// 1. Find or create the Term
			var term models.Term
			result := tx.Where("project_id = ? AND module = ? AND key = ?", projectID, module, key).First(&term)

			if result.Error != nil {
				// Term doesn't exist — create it
				term = models.Term{
					ProjectID: projectID,
					Key:       key,
					Module:    module,
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

// ProjectLogs returns recent term-level and project-level logs in one timeline.
func (r *TermRepository) ProjectLogs(projectID uint, limit int) ([]ProjectActivityLog, error) {
	var termLogs []models.HistoryLog
	if err := r.db.
		Joins("JOIN terms ON terms.id = history_logs.term_id").
		Where("terms.project_id = ?", projectID).
		Preload("User").
		Preload("Term").
		Order("history_logs.created_at DESC").
		Limit(limit).
		Find(&termLogs).Error; err != nil {
		return nil, err
	}

	var projectLogs []models.ProjectHistoryLog
	if err := r.db.
		Where("project_id = ?", projectID).
		Preload("User").
		Order("created_at DESC").
		Limit(limit).
		Find(&projectLogs).Error; err != nil {
		return nil, err
	}

	logs := make([]ProjectActivityLog, 0, len(termLogs)+len(projectLogs))
	for i := range termLogs {
		log := &termLogs[i]
		termID := log.TermID
		logs = append(logs, ProjectActivityLog{
			ID:        fmt.Sprintf("term-%d", log.ID),
			Scope:     "term",
			TermID:    &termID,
			UserID:    log.UserID,
			Action:    log.Action,
			CreatedAt: log.CreatedAt,
			Term:      &log.Term,
			User:      log.User,
		})
	}
	for i := range projectLogs {
		log := &projectLogs[i]
		logs = append(logs, ProjectActivityLog{
			ID:        fmt.Sprintf("project-%d", log.ID),
			Scope:     "project",
			UserID:    log.UserID,
			Action:    log.Action,
			ItemCount: log.ItemCount,
			TermKeys:  log.TermKeys,
			CreatedAt: log.CreatedAt,
			User:      log.User,
		})
	}

	sort.SliceStable(logs, func(i, j int) bool {
		return logs[i].CreatedAt.After(logs[j].CreatedAt)
	})
	if len(logs) > limit {
		logs = logs[:limit]
	}
	return logs, nil
}
