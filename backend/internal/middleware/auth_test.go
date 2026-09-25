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

func TestUniversityReviewRoleGuard(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const secret = "university-review-secret"

	router := gin.New()
	router.POST(
		"/university/internship-cases/1/approve-placement",
		RequireAuth(secret),
		RequireRole(model.RoleUniversitySupervisor),
		func(ctx *gin.Context) { ctx.Status(http.StatusNoContent) },
	)

	tests := []struct {
		name       string
		role       model.Role
		withToken  bool
		wantStatus int
	}{
		{name: "university supervisor", role: model.RoleUniversitySupervisor, withToken: true, wantStatus: http.StatusNoContent},
		{name: "student", role: model.RoleStudent, withToken: true, wantStatus: http.StatusForbidden},
		{name: "professor", role: model.RoleProfessor, withToken: true, wantStatus: http.StatusForbidden},
		{name: "company supervisor", role: model.RoleCompanySupervisor, withToken: true, wantStatus: http.StatusForbidden},
		{name: "admin follows existing semantics", role: model.RoleAdmin, withToken: true, wantStatus: http.StatusForbidden},
		{name: "unauthenticated", wantStatus: http.StatusUnauthorized},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/university/internship-cases/1/approve-placement", nil)
			if test.withToken {
				request.Header.Set("Authorization", "Bearer "+mustCreateToken(t, test.role, secret))
			}
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.Code, test.wantStatus)
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
