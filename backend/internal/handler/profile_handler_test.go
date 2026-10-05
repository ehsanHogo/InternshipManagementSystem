package handler

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestDecodeProfileRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		name, body string
		valid      bool
	}{
		{"editable fields", `{"fullName":"نام","phone":"","jobTitle":"عنوان"}`, true},
		{"protected field", `{"fullName":"نام","email":"other@example.test"}`, false},
		{"wrong type", `{"fullName":123}`, false},
		{"trailing object", `{"fullName":"نام"} {"role":"ADMIN"}`, false},
		{"trailing invalid data", `{"fullName":"نام"} garbage`, false},
		{"oversized body", `{"fullName":"` + strings.Repeat("a", 4096) + `"}`, false},
		{"empty body", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("PUT", "/api/profile", strings.NewReader(test.body))
			var input profileUpdateRequest
			if valid := decodeProfileRequest(ctx, &input); valid != test.valid {
				t.Fatalf("valid = %v, want %v", valid, test.valid)
			}
		})
	}
}
