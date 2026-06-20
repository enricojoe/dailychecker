package httpapi_test

import (
	"net/http"
	"testing"
)

// registerAndLogin registers a fresh user and returns its username and access
// token. Shared helpers (post, uniqueUsername, tokenPairFrom) live in
// auth_integration_test.go (same test binary).
func registerAndLogin(t *testing.T, password string) (username, access string) {
	t.Helper()
	username = uniqueUsername()
	w := post(t, "/api/auth/register", map[string]interface{}{
		"name": "Profile User", "username": username, "password": password,
	}, nil)
	assertStatus(t, w, http.StatusCreated)

	w = post(t, "/api/auth/login", map[string]interface{}{
		"username": username, "password": password,
	}, nil)
	assertStatus(t, w, http.StatusOK)
	access, _ = tokenPairFrom(t, w)
	return username, access
}

// TestUpdateProfile_NameAndUsername updates both name and username and verifies
// the change is persisted (reflected by GET /api/me).
func TestUpdateProfile_NameAndUsername(t *testing.T) {
	_, access := registerAndLogin(t, "TestPass123!")
	newUsername := uniqueUsername()

	w := doReq(t, http.MethodPatch, "/api/me", map[string]interface{}{
		"name":     "Renamed User",
		"username": newUsername,
	}, bearer(access))
	assertStatus(t, w, http.StatusOK)

	var body struct {
		Name     string `json:"name"`
		Username string `json:"username"`
	}
	decodeJSON(t, w, &body)
	if body.Name != "Renamed User" {
		t.Errorf("name: want %q got %q", "Renamed User", body.Name)
	}
	if body.Username != newUsername {
		t.Errorf("username: want %q got %q", newUsername, body.Username)
	}

	// Confirm persistence.
	w = get(t, "/api/me", bearer(access))
	assertStatus(t, w, http.StatusOK)
	decodeJSON(t, w, &body)
	if body.Username != newUsername {
		t.Errorf("/me username after update: want %q got %q", newUsername, body.Username)
	}
}

// TestUpdateProfile_UsernameTaken returns 409 when claiming another user's name.
func TestUpdateProfile_UsernameTaken(t *testing.T) {
	takenUsername, _ := registerAndLogin(t, "TestPass123!")
	_, access := registerAndLogin(t, "TestPass123!")

	w := doReq(t, http.MethodPatch, "/api/me", map[string]interface{}{
		"username": takenUsername,
	}, bearer(access))
	assertStatus(t, w, http.StatusConflict)
}

// TestUpdateProfile_ChangePassword verifies password rotation: wrong current
// password is rejected, the correct one succeeds, and the new password logs in.
func TestUpdateProfile_ChangePassword(t *testing.T) {
	const oldPassword = "TestPass123!"
	const newPassword = "BrandNewPass456!"
	username, access := registerAndLogin(t, oldPassword)

	// Wrong current password -> 401.
	w := doReq(t, http.MethodPatch, "/api/me", map[string]interface{}{
		"current_password": "wrongpassword",
		"new_password":     newPassword,
	}, bearer(access))
	assertStatus(t, w, http.StatusUnauthorized)

	// Missing current password -> 422.
	w = doReq(t, http.MethodPatch, "/api/me", map[string]interface{}{
		"new_password": newPassword,
	}, bearer(access))
	assertStatus(t, w, http.StatusUnprocessableEntity)

	// Correct current password -> 200.
	w = doReq(t, http.MethodPatch, "/api/me", map[string]interface{}{
		"current_password": oldPassword,
		"new_password":     newPassword,
	}, bearer(access))
	assertStatus(t, w, http.StatusOK)

	// New password logs in; old one no longer works.
	w = post(t, "/api/auth/login", map[string]interface{}{
		"username": username, "password": newPassword,
	}, nil)
	assertStatus(t, w, http.StatusOK)

	w = post(t, "/api/auth/login", map[string]interface{}{
		"username": username, "password": oldPassword,
	}, nil)
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestCheckUsername covers the public availability endpoint.
func TestCheckUsername(t *testing.T) {
	taken, _ := registerAndLogin(t, "TestPass123!")

	// Taken username -> available:false.
	w := get(t, "/api/auth/check-username?username="+taken, nil)
	assertStatus(t, w, http.StatusOK)
	var body struct {
		Available bool `json:"available"`
	}
	decodeJSON(t, w, &body)
	if body.Available {
		t.Errorf("check-username(%q): want available=false", taken)
	}

	// Unused username -> available:true.
	free := uniqueUsername()
	w = get(t, "/api/auth/check-username?username="+free, nil)
	assertStatus(t, w, http.StatusOK)
	decodeJSON(t, w, &body)
	if !body.Available {
		t.Errorf("check-username(%q): want available=true", free)
	}

	// Too short -> 422.
	w = get(t, "/api/auth/check-username?username=ab", nil)
	assertStatus(t, w, http.StatusUnprocessableEntity)
}
