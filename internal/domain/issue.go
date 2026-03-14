package domain

import (
	"cmp"
	"slices"
	"strings"
	"time"
)

type Issue struct {
	ID           string
	Identifier   string
	Title        string
	Description  string
	Priority     *int
	Order        *int
	State        string
	Lane         string
	ReviewOf     string
	BranchName   string
	URL          string
	Labels       []string
	Dependencies []string
	BlockedBy    []string
	CreatedAt    *time.Time
	UpdatedAt    *time.Time
	Raw          map[string]any
}

type IssueState struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	State      string `json:"state"`
}

func (issue Issue) MatchesState(states []string) bool {
	for _, state := range states {
		if strings.EqualFold(strings.TrimSpace(issue.State), strings.TrimSpace(state)) {
			return true
		}
	}

	return false
}

func SortIssues(issues []Issue) {
	slices.SortFunc(issues, func(left Issue, right Issue) int {
		if diff := cmp.Compare(optionalIntValue(left.Priority), optionalIntValue(right.Priority)); diff != 0 {
			return diff
		}

		if diff := cmp.Compare(optionalIntValue(left.Order), optionalIntValue(right.Order)); diff != 0 {
			return diff
		}

		if diff := compareTimes(left.CreatedAt, right.CreatedAt); diff != 0 {
			return diff
		}

		return cmp.Compare(left.Identifier, right.Identifier)
	})
}

func optionalIntValue(value *int) int {
	if value == nil {
		return 1_000_000
	}

	return *value
}

func compareTimes(left *time.Time, right *time.Time) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return 1
	case right == nil:
		return -1
	default:
		if left.Before(*right) {
			return -1
		}
		if left.After(*right) {
			return 1
		}
		return 0
	}
}
