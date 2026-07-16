package utils

import (
	"regexp"
	"sort"
)

var placeholderRegexp = regexp.MustCompile(`\{[a-zA-Z0-9_.-]+\}`)

func ExtractPlaceholders(text string) []string {
	matches := placeholderRegexp.FindAllString(text, -1)
	seen := make(map[string]bool, len(matches))
	result := make([]string, 0, len(matches))

	for _, item := range matches {
		if seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}

	sort.Strings(result)
	return result
}

func DiffPlaceholders(source string, target string) (missing []string, extra []string) {
	sourceSet := make(map[string]bool)
	targetSet := make(map[string]bool)

	for _, item := range ExtractPlaceholders(source) {
		sourceSet[item] = true
	}
	for _, item := range ExtractPlaceholders(target) {
		targetSet[item] = true
	}

	for item := range sourceSet {
		if !targetSet[item] {
			missing = append(missing, item)
		}
	}
	for item := range targetSet {
		if !sourceSet[item] {
			extra = append(extra, item)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)
	return missing, extra
}
