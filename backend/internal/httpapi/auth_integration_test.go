package httpapi_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/enricojoe/dailychecker/internal/auth"
	"github.com/enricojoe/dailychecker/internal/config"
	"github.com/enricojoe/dailychecker/internal/httpapi"
	"github.com/enricojoe/dailychecker/internal/testhelper"
	"github.com/enricojoe/dailychecker/internal/users"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

var (
	testRouter    http.Handler
	testDB        *sqlx.DB
	testJWTSecret = "httpapi-integration-test-secret"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)

	db, skip := testhelper.OpenTestDB()
	if skip {
		fmt.Println("DATABASE_URL not set — skipping httpapi integration tests")
		os.Exit(0)
	}
	testDB = db

	cfg := &config.Config{
		JWTSecret:       testJWTSecret,
		AccessTokenTTL:  time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
	}

	userRepo := users.NewRepository(testDB)
	tokenRepo := auth.NewTokenRepository(testDB)
	authSvc := auth.NewService(userRepo, tokenRepo, cfg)

	testRouter = httpapi.NewRouter(authSvc, cfg.JWTSecret)

	code := m.Run()
	testDB.Close()
	os.Exit(code)
}

// --- helpers ----------------------------------------------------------------

// uniquePhone returns a phone number that is unique within a test run.
func uniquePhone() string {
	return fmt.Sprintf("+1555%09d", time.Now().UnixNano()%1_000_000_000)
}

// doReq fires a request against testRouter and returns the recorder.
func doReq(t *testing.T, method, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("doReq: marshal: %v", err)
		}
		reqBody = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, path, reqBody)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	testRouter.ServeHTTP(w, req)
	return w
}

func post(t *testing.T, path string, body interface{}, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return doReq(t, http.MethodPost, path, body, headers)
}

func get(t *testing.T, path string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	return doReq(t, http.MethodGet, path, nil, headers)
}

func bearer(token string) map[string]string {
	return map[string]string{"Authorization": "Bearer " + token}
}

func assertStatus(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Errorf("status: want %d, got %d — body: %s", want, w.Code, w.Body.String())
	}
}

// decodeJSON parses the recorder body into dst.
func decodeJSON(t *testing.T, w *httptest.ResponseRecorder, dst interface{}) {
	t.Helper()
	if err := json.NewDecoder(w.Body).Decode(dst); err != nil {
		t.Fatalf("decodeJSON: %v — body was: %s", err, w.Body.String())
	}
}

// tokenPairFrom decodes a TokenPair from the recorder body.
func tokenPairFrom(t *testing.T, w *httptest.ResponseRecorder) (access, refresh string) {
	t.Helper()
	var pair struct {
		Access  string `json:"access"`
		Refresh string `json:"refresh"`
	}
	decodeJSON(t, w, &pair)
	if pair.Access == "" {
		t.Fatal("tokenPairFrom: access token is empty")
	}
	if pair.Refresh == "" {
		t.Fatal("tokenPairFrom: refresh token is empty")
	}
	return pair.Access, pair.Refresh
}

// --- tests ------------------------------------------------------------------

// TestHealthz verifies the existing health endpoint is unaffected.
func TestHealthz(t *testing.T) {
	w := get(t, "/healthz", nil)
	assertStatus(t, w, http.StatusOK)
}

// TestFullAuthCycle covers the complete happy path:
// register → login → /me → refresh (rotation) → /me with new token → logout → logout again.
func TestFullAuthCycle(t *testing.T) {
	phone := uniquePhone()
	const password = "TestPass123!"

	// 1. Register.
	w := post(t, "/api/auth/register", map[string]interface{}{
		"name": "Cycle User", "phone": phone, "password": password,
	}, nil)
	assertStatus(t, w, http.StatusCreated)

	var regBody struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Phone string `json:"phone"`
	}
	decodeJSON(t, w, &regBody)
	if regBody.ID == "" {
		t.Fatal("register: id missing from response")
	}
	if regBody.Name != "Cycle User" {
		t.Errorf("register: name want %q got %q", "Cycle User", regBody.Name)
	}

	// 2. Login.
	w = post(t, "/api/auth/login", map[string]interface{}{
		"phone": phone, "password": password,
	}, nil)
	assertStatus(t, w, http.StatusOK)
	access1, refresh1 := tokenPairFrom(t, w)

	// 3. GET /api/me with access token.
	w = get(t, "/api/me", bearer(access1))
	assertStatus(t, w, http.StatusOK)
	var meBody struct {
		ID    string `json:"id"`
		Phone string `json:"phone"`
	}
	decodeJSON(t, w, &meBody)
	if meBody.ID != regBody.ID {
		t.Errorf("/me: id mismatch: want %q got %q", regBody.ID, meBody.ID)
	}

	// 4. Refresh — produces a new token pair and rotates the old refresh token.
	w = post(t, "/api/auth/refresh", map[string]interface{}{
		"refresh": refresh1,
	}, nil)
	assertStatus(t, w, http.StatusOK)
	access2, refresh2 := tokenPairFrom(t, w)

	// 5. Old refresh token must be rejected after rotation.
	w = post(t, "/api/auth/refresh", map[string]interface{}{
		"refresh": refresh1,
	}, nil)
	assertStatus(t, w, http.StatusUnauthorized)

	// 6. GET /api/me with the new access token.
	w = get(t, "/api/me", bearer(access2))
	assertStatus(t, w, http.StatusOK)

	// 7. Logout — revokes refresh2.
	w = post(t, "/api/auth/logout", map[string]interface{}{
		"refresh": refresh2,
	}, nil)
	assertStatus(t, w, http.StatusNoContent)

	// 8. Logout again with the same (now revoked) token — must fail.
	w = post(t, "/api/auth/logout", map[string]interface{}{
		"refresh": refresh2,
	}, nil)
	assertStatus(t, w, http.StatusUnauthorized)
}

// TestRegisterDuplicate verifies that registering the same phone twice returns 409.
func TestRegisterDuplicate(t *testing.T) {
	phone := uniquePhone()
	body := map[string]interface{}{
		"name": "Dup User", "phone": phone, "password": "password123",
	}
	w := post(t, "/api/auth/register", body, nil)
	assertStatus(t, w, http.StatusCreated)

	w = post(t, "/api/auth/register", body, nil)
	assertStatus(t, w, http.StatusConflict)
}

// TestRegisterValidation verifies that missing or invalid fields return 422.
func TestRegisterValidation(t *testing.T) {
	cases := []struct {
		name string
		body map[string]interface{}
	}{
		{"missing name", map[string]interface{}{"phone": "+155500001111", "password": "password123"}},
		{"missing phone", map[string]interface{}{"name": "X", "password": "password123"}},
		{"password too short", map[string]interface{}{"name": "X", "phone": "+155500001111", "password": "short"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := post(t, "/api/auth/register", tc.body, nil)
			assertStatus(t, w, http.StatusUnprocessableEntity)
		})
	}
}

// TestLoginWrongCredentials covers bad password and unknown phone → both 401.
func TestLoginWrongCredentials(t *testing.T) {
	phone := uniquePhone()
	post(t, "/api/auth/register", map[string]interface{}{
		"name": "Login Test", "phone": phone, "password": "correct-password",
	}, nil)

	t.Run("wrong password", func(t *testing.T) {
		w := post(t, "/api/auth/login", map[string]interface{}{
			"phone": phone, "password": "wrong-password",
		}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})
	t.Run("unknown phone", func(t *testing.T) {
		w := post(t, "/api/auth/login", map[string]interface{}{
			"phone": "+99000000000", "password": "correct-password",
		}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestProtectedRouteRejections verifies that /api/me returns 401 for
// missing, invalid, expired, and wrong-secret access tokens.
func TestProtectedRouteRejections(t *testing.T) {
	const fakeUserID = "00000000-0000-0000-0000-000000000001"

	t.Run("no header", func(t *testing.T) {
		w := get(t, "/api/me", nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("malformed token", func(t *testing.T) {
		w := get(t, "/api/me", bearer("this-is-not-a-jwt"))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("expired token", func(t *testing.T) {
		// Craft a token with a past expiry using the correct secret.
		expiredTok, err := auth.IssueAccessToken(fakeUserID, testJWTSecret, -time.Minute)
		if err != nil {
			t.Fatalf("IssueAccessToken: %v", err)
		}
		w := get(t, "/api/me", bearer(expiredTok))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("wrong secret", func(t *testing.T) {
		wrongTok, err := auth.IssueAccessToken(fakeUserID, "wrong-secret", time.Minute)
		if err != nil {
			t.Fatalf("IssueAccessToken wrong-secret: %v", err)
		}
		w := get(t, "/api/me", bearer(wrongTok))
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("bearer prefix missing", func(t *testing.T) {
		tok, err := auth.IssueAccessToken(fakeUserID, testJWTSecret, time.Minute)
		if err != nil {
			t.Fatalf("IssueAccessToken: %v", err)
		}
		// Send token without "Bearer " prefix.
		w := doReq(t, http.MethodGet, "/api/me", nil, map[string]string{"Authorization": tok})
		assertStatus(t, w, http.StatusUnauthorized)
	})
}

// TestRefreshNegative covers unknown and revoked refresh tokens.
func TestRefreshNegative(t *testing.T) {
	t.Run("unknown token", func(t *testing.T) {
		w := post(t, "/api/auth/refresh", map[string]interface{}{
			"refresh": "completely-unknown-token-value",
		}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})

	t.Run("revoked token", func(t *testing.T) {
		// Register + login to get a real refresh token, then revoke it via logout.
		phone := uniquePhone()
		post(t, "/api/auth/register", map[string]interface{}{
			"name": "Revoke Test", "phone": phone, "password": "password123",
		}, nil)
		w := post(t, "/api/auth/login", map[string]interface{}{
			"phone": phone, "password": "password123",
		}, nil)
		assertStatus(t, w, http.StatusOK)
		_, refresh := tokenPairFrom(t, w)

		// Revoke via logout.
		post(t, "/api/auth/logout", map[string]interface{}{"refresh": refresh}, nil)

		// Refresh with revoked token must fail.
		w = post(t, "/api/auth/refresh", map[string]interface{}{"refresh": refresh}, nil)
		assertStatus(t, w, http.StatusUnauthorized)
	})
}
