package progression

import "testing"

func TestUserProgressionDefaultsToUnrankedShape(t *testing.T) {
	value := UserProgression{UserID: "user-1", Level: 0, LevelName: "Unranked"}
	if value.LevelName != "Unranked" || value.Level != 0 {
		t.Fatalf("unexpected default progression: %+v", value)
	}
}
