package diff

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/terraform-drift-detector/terraform_drift_detector/internal/models"
)

// CompareResources compares expected (state) and actual (cloud) resources.
func CompareResources(expected, actual *models.Resource, ignoreRules []string) models.DriftItem {
	item := models.DriftItem{
		ResourceID: expected.ID,
		Address:    expected.Address,
		Type:       expected.Type,
		Provider:   expected.Provider,
		CloudID:    expected.CloudID,
		Expected:   expected,
	}

	if actual == nil {
		item.DriftType = models.DriftTypeMissing
		item.Message = "resource exists in Terraform state but was not found in cloud"
		return item
	}

	item.Actual = actual
	attrDiffs := diffMaps("", expected.Attributes, actual.Attributes)
	tagDiffs := diffTags(expected.Tags, actual.Tags)

	item.Diff = attrDiffs
	item.TagsDiff = tagDiffs

	attrDrift := len(attrDiffs) > 0
	tagDrift := len(tagDiffs) > 0

	switch {
	case attrDrift && tagDrift:
		item.DriftType = models.DriftTypeModified
		item.Message = "resource attributes and tags differ from Terraform state"
	case attrDrift:
		item.DriftType = models.DriftTypeModified
		item.Message = "resource attributes differ from Terraform state"
	case tagDrift:
		item.DriftType = models.DriftTypeTagOnly
		item.Message = "only resource tags differ from Terraform state"
	default:
		item.DriftType = models.DriftTypeInSync
		item.Message = "resource matches Terraform state"
	}

	_ = ignoreRules
	return item
}

func diffMaps(prefix string, expected, actual map[string]any) []models.FieldDiff {
	var diffs []models.FieldDiff
	keys := unionKeys(expected, actual)
	for _, key := range keys {
		path := key
		if prefix != "" {
			path = prefix + "/" + key
		}
		ev, eok := expected[key]
		av, aok := actual[key]
		if !eok && !aok {
			continue
		}
		if !eok {
			diffs = append(diffs, models.FieldDiff{Path: path, Actual: av})
			continue
		}
		if !aok {
			diffs = append(diffs, models.FieldDiff{Path: path, Expected: ev})
			continue
		}
		diffs = append(diffs, diffValues(path, ev, av)...)
	}
	return diffs
}

func diffValues(path string, expected, actual any) []models.FieldDiff {
	if reflect.DeepEqual(expected, actual) {
		return nil
	}

	em, eMap := expected.(map[string]any)
	am, aMap := actual.(map[string]any)
	if eMap && aMap {
		return diffMaps(path, em, am)
	}

	es, eSlice := expected.([]string)
	as, aSlice := actual.([]string)
	if eSlice && aSlice {
		if reflect.DeepEqual(es, as) {
			return nil
		}
		return []models.FieldDiff{{Path: path, Expected: es, Actual: as}}
	}

	ea, eAnySlice := expected.([]any)
	aa, aAnySlice := actual.([]any)
	if eAnySlice && aAnySlice {
		if reflect.DeepEqual(ea, aa) {
			return nil
		}
		return []models.FieldDiff{{Path: path, Expected: ea, Actual: aa}}
	}

	return []models.FieldDiff{{Path: path, Expected: expected, Actual: actual}}
}

func diffTags(expected, actual map[string]string) map[string]models.TagDiff {
	diffs := map[string]models.TagDiff{}
	for k, ev := range expected {
		av, ok := actual[k]
		if !ok {
			diffs[k] = models.TagDiff{Expected: ev, Status: "removed"}
			continue
		}
		if ev != av {
			diffs[k] = models.TagDiff{Expected: ev, Actual: av, Status: "changed"}
		}
	}
	for k, av := range actual {
		if _, ok := expected[k]; !ok {
			diffs[k] = models.TagDiff{Actual: av, Status: "added"}
		}
	}
	if len(diffs) == 0 {
		return nil
	}
	return diffs
}

func unionKeys(a, b map[string]any) []string {
	seen := map[string]bool{}
	var keys []string
	for k := range a {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	for k := range b {
		if !seen[k] {
			seen[k] = true
			keys = append(keys, k)
		}
	}
	sortStrings(keys)
	return keys
}

func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if strings.Compare(s[i], s[j]) > 0 {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}

// BuildSummary aggregates drift items into a summary.
func BuildSummary(items []models.DriftItem) models.DriftSummary {
	summary := models.DriftSummary{TotalResources: len(items)}
	for _, item := range items {
		switch item.DriftType {
		case models.DriftTypeInSync:
			summary.InSync++
		case models.DriftTypeMissing:
			summary.Missing++
			summary.Drifted++
		case models.DriftTypeModified:
			summary.Modified++
			summary.Drifted++
		case models.DriftTypeTagOnly:
			summary.TagOnly++
			summary.Drifted++
		case models.DriftTypeUnmanaged:
			summary.Unmanaged++
			summary.Drifted++
		case models.DriftTypeFetchErr:
			summary.FetchErrors++
			summary.Drifted++
		}
	}
	return summary
}

// FetchErrorItem creates a drift item for a failed cloud fetch.
func FetchErrorItem(expected *models.Resource, err error) models.DriftItem {
	return models.DriftItem{
		ResourceID: expected.ID,
		Address:    expected.Address,
		Type:       expected.Type,
		Provider:   expected.Provider,
		CloudID:    expected.CloudID,
		DriftType:  models.DriftTypeFetchErr,
		Message:    fmt.Sprintf("failed to fetch resource from cloud: %v", err),
		Expected:   expected,
	}
}

// UnmanagedItem creates a drift item for a cloud resource not in Terraform state.
func UnmanagedItem(actual *models.Resource) models.DriftItem {
	return models.DriftItem{
		ResourceID: actual.CloudID,
		Address:    "(unmanaged)",
		Type:       actual.Type,
		Provider:   actual.Provider,
		CloudID:    actual.CloudID,
		DriftType:  models.DriftTypeUnmanaged,
		Message:    "resource exists in cloud but is not managed by Terraform state",
		Actual:     actual,
	}
}
