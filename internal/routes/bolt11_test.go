package routes

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/lescuer97/nutmix/api/cashu"
	"github.com/lescuer97/nutmix/internal/lightning"
)

func TestRenderLNBackendEndOfLife(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, test := range []struct {
		detail  string
		name    string
		code    cashu.ErrorCode
		minting bool
	}{
		{name: "minting", minting: true, code: cashu.MINTING_DISABLED, detail: "Minting is temporarily unavailable"},
		{name: "melting", minting: false, code: cashu.UNKNOWN, detail: "Melting is temporarily unavailable"},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			if !renderLNBackendEndOfLife(ctx, fmt.Errorf("wrapped: %w", lightning.ErrLNBackendEndOfLife), test.minting) {
				t.Fatal("expected error to be handled")
			}
			if recorder.Code != 503 {
				t.Fatalf("status = %d, want 503", recorder.Code)
			}
			want := fmt.Sprintf(`"detail":"%s"`, test.detail)
			body := recorder.Body.String()
			if !containsAll(body, fmt.Sprintf(`"code":%d`, test.code), want) || strings.Contains(body, "Strike") || strings.Contains(body, "end of life") {
				t.Fatalf("unexpected response body: %s", body)
			}
		})
	}
}

func containsAll(value string, values ...string) bool {
	for _, item := range values {
		if !strings.Contains(value, item) {
			return false
		}
	}
	return true
}
