package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"internship-management-system/backend/internal/auth"
	"internship-management-system/backend/internal/model"
)

func TestRequireAuthAndRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "test-secret"

	router := gin.New()
	router.GET(
		"/student",
		RequireAuth(secret),
		RequireRole(model.RoleStudent),
		func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) },
	)

	studentToken := mustCreateToken(t, model.RoleStudent, secret)
	adminToken := mustCreateToken(t, model.RoleAdmin, secret)

	for name, testCase := range map[string]struct {
		token      string
		wantStatus int
	}{
		"student is allowed": {token: studentToken, wantStatus: http.StatusNoContent},
		"wrong role":         {token: adminToken, wantStatus: http.StatusForbidden},
		"missing token":      {wantStatus: http.StatusUnauthorized},
		"invalid token":      {token: "invalid", wantStatus: http.StatusUnauthorized},
	} {
		t.Run(name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/student", nil)
			if testCase.token != "" {
				request.Header.Set("Authorization", "Bearer "+testCase.token)
			}
			response := httptest.NewRecorder()

			router.ServeHTTP(response, request)
			if response.Code != testCase.wantStatus {
				t.Fatalf("got status %d, want %d", response.Code, testCase.wantStatus)
			}
		})
	}
}

func mustCreateToken(t *testing.T, role model.Role, secret string) string {
	t.Helper()
	token, err := auth.CreateToken(model.User{ID: 1, Role: role}, secret, time.Hour)
	if err != nil {
		t.Fatalf("create token: %v", err)
	}
	return token
}
