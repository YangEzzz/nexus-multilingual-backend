package database

import (
	"choice-matrix-backend/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func seedAvailableLanguages(db *gorm.DB) error {
	defaults := []models.AvailableLanguage{
		{Code: "cn", Name: "简体中文", SortOrder: 10},
		{Code: "cht", Name: "繁体中文", SortOrder: 20},
		{Code: "en", Name: "英文", SortOrder: 30},
		{Code: "jp", Name: "日语", SortOrder: 40},
		{Code: "pt", Name: "葡萄牙语", SortOrder: 50},
		{Code: "es", Name: "西班牙语", SortOrder: 60},
		{Code: "ru", Name: "俄语", SortOrder: 70},
		{Code: "de", Name: "德语", SortOrder: 80},
		{Code: "fr", Name: "法语", SortOrder: 90},
		{Code: "ko", Name: "韩语", SortOrder: 100},
		{Code: "th", Name: "泰语", SortOrder: 110},
		{Code: "vi", Name: "越南语", SortOrder: 120},
		{Code: "ind", Name: "印尼语", SortOrder: 130},
		{Code: "tr", Name: "土耳其语", SortOrder: 140},
		{Code: "bn", Name: "孟加拉语", SortOrder: 150},
		{Code: "pl", Name: "波兰语", SortOrder: 160},
		{Code: "it", Name: "意大利语", SortOrder: 170},
	}
	return db.Clauses(clause.OnConflict{DoNothing: true}).Create(&defaults).Error
}
