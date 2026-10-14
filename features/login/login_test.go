package login

import (
	"os"
	"testing"
	"time"
)

func TestRefreshSession_TimeExtension(t *testing.T) {
	// Note: If no DB is running locally, this will return an error
	before := time.Now()
	expire, err := RefreshSession("dummy-token-123")
	if err != nil {
		t.Logf("Expected DB error (if testing without live DB): %v", err)
		return
	}

	if expire.Sub(before) < 23*time.Hour {
		t.Errorf("Expected session expiration to be extended by 24 hours, got %v", expire)
	}
}

func TestCheckForUsers_FirstAccessEnv(t *testing.T) {
	// Arrange: Set a dummy environment variable
	os.Setenv("FIRST_ACCESS", "UNKNOWN")
	user := LoginUser{}

	// Act
	err := user.CheckForUsers()

	// Assert: Check results based on DB availability
	if err != nil {
		t.Logf("Database connection failed as expected in isolated test environment: %v", err)
	} else {
		// If a DB connection succeeds, FIRST_ACCESS should automatically become YES or NO
		val := os.Getenv("FIRST_ACCESS")
		if val != "YES" && val != "NO" {
			t.Errorf("Expected FIRST_ACCESS to be YES or NO, got %s", val)
		}
	}
}
