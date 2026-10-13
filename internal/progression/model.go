package progression

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

var identifierPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)
var eventTypePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)

type AchievementRequest struct {
	TenantID      string `json:"tenant_id"`
	EnvironmentID string `json:"environment_id"`
	Name          string `json:"name"`
	Description   string `json:"description"`
	EventType     string `json:"event_type"`
	RequiredCount int    `json:"required_count"`
}

type LevelRequest struct {
	TenantID      string `json:"tenant_id"`
	EnvironmentID string `json:"environment_id"`
	Level         int    `json:"level"`
	Name          string `json:"name"`
	XPThreshold   int64  `json:"xp_threshold"`
	Active        *bool  `json:"active,omitempty"`
}

type Achievement struct {
	ID            string    `json:"id"`
	TenantID      string    `json:"tenant_id"`
	EnvironmentID string    `json:"environment_id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	EventType     string    `json:"event_type"`
	RequiredCount int       `json:"required_count"`
	Active        bool      `json:"active"`
	CreatedAt     time.Time `json:"created_at"`
}

type UserAchievement struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	EventType     string     `json:"event_type"`
	RequiredCount int        `json:"required_count"`
	Progress      int        `json:"progress"`
	Unlocked      bool       `json:"unlocked"`
	UnlockedAt    *time.Time `json:"unlocked_at,omitempty"`
}

func ValidateAchievementRequest(request AchievementRequest) (AchievementRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	request.EventType = strings.TrimSpace(request.EventType)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		return AchievementRequest{}, fmt.Errorf("tenant_id and environment_id must be valid identifiers")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return AchievementRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if len(request.Description) > 500 {
		return AchievementRequest{}, fmt.Errorf("description must be 500 characters or fewer")
	}
	if !eventTypePattern.MatchString(request.EventType) {
		return AchievementRequest{}, fmt.Errorf("event_type must contain lowercase letters, numbers, '.', '_' or '-'")
	}
	if request.RequiredCount < 1 || request.RequiredCount > 100000 {
		return AchievementRequest{}, fmt.Errorf("required_count must be between 1 and 100000")
	}
	return request, nil
}

func ValidateLevelRequest(request LevelRequest) (LevelRequest, error) {
	request.TenantID = strings.TrimSpace(request.TenantID)
	request.EnvironmentID = strings.TrimSpace(request.EnvironmentID)
	request.Name = strings.TrimSpace(request.Name)
	if !identifierPattern.MatchString(request.TenantID) || !identifierPattern.MatchString(request.EnvironmentID) {
		return LevelRequest{}, fmt.Errorf("tenant_id and environment_id must be valid identifiers")
	}
	if request.Level < 1 || request.Level > 100000 {
		return LevelRequest{}, fmt.Errorf("level must be between 1 and 100000")
	}
	if request.Name == "" || len(request.Name) > 120 {
		return LevelRequest{}, fmt.Errorf("name must be between 1 and 120 characters")
	}
	if request.XPThreshold < 0 {
		return LevelRequest{}, fmt.Errorf("xp_threshold must be zero or greater")
	}
	return request, nil
}
