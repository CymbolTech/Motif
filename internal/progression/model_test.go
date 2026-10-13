package progression

import "testing"

func TestValidateAchievementRequest(t *testing.T) {
	request, err := ValidateAchievementRequest(AchievementRequest{
		TenantID: " acme ", EnvironmentID: " sandbox ", Name: "First lesson",
		EventType: "lesson.completed", RequiredCount: 3,
	})
	if err != nil {
		t.Fatalf("ValidateAchievementRequest() error = %v", err)
	}
	if request.TenantID != "acme" || request.EnvironmentID != "sandbox" {
		t.Fatalf("request was not normalized: %+v", request)
	}
}

func TestValidateAchievementRequestRejectsUnsafeEventType(t *testing.T) {
	_, err := ValidateAchievementRequest(AchievementRequest{
		TenantID: "acme", EnvironmentID: "sandbox", Name: "First lesson",
		EventType: "lesson completed", RequiredCount: 1,
	})
	if err == nil {
		t.Fatal("ValidateAchievementRequest() error = nil, want invalid event type")
	}
}

func TestValidateLevelRequest(t *testing.T) {
	request, err := ValidateLevelRequest(LevelRequest{TenantID: " acme ", EnvironmentID: " sandbox ", Level: 2, Name: "Builder", XPThreshold: 100})
	if err != nil {
		t.Fatalf("ValidateLevelRequest() error = %v", err)
	}
	if request.TenantID != "acme" || request.EnvironmentID != "sandbox" {
		t.Fatalf("request was not normalized: %+v", request)
	}
	for _, invalid := range []LevelRequest{
		{TenantID: "acme", EnvironmentID: "sandbox", Level: 0, Name: "Starter"},
		{TenantID: "acme", EnvironmentID: "sandbox", Level: 1, Name: "", XPThreshold: 0},
		{TenantID: "acme", EnvironmentID: "sandbox", Level: 1, Name: "Starter", XPThreshold: -1},
	} {
		if _, err := ValidateLevelRequest(invalid); err == nil {
			t.Fatalf("ValidateLevelRequest(%+v) accepted invalid request", invalid)
		}
	}
}
