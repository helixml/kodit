package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestWriteProtect_GET_PassesWithoutKey(t *testing.T) {
	config := NewAuthConfigWithKeys([]string{"secret"})
	handler := WriteProtect(config)(okHandler())

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET without key: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWriteProtect_HEAD_PassesWithoutKey(t *testing.T) {
	config := NewAuthConfigWithKeys([]string{"secret"})
	handler := WriteProtect(config)(okHandler())

	req := httptest.NewRequest(http.MethodHead, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HEAD without key: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWriteProtect_OPTIONS_PassesWithoutKey(t *testing.T) {
	config := NewAuthConfigWithKeys([]string{"secret"})
	handler := WriteProtect(config)(okHandler())

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS without key: status = %d, want %d", w.Code, http.StatusOK)
	}
}

func TestWriteProtect_MutatingMethods_RequireKey(t *testing.T) {
	config := NewAuthConfigWithKeys([]string{"secret"})
	handler := WriteProtect(config)(okHandler())

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("%s without key: status = %d, want %d", method, w.Code, http.StatusUnauthorized)
		}
	}
}

func TestWriteProtect_MutatingMethods_PassWithValidKey(t *testing.T) {
	config := NewAuthConfigWithKeys([]string{"secret"})
	handler := WriteProtect(config)(okHandler())

	methods := []string{http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		req.Header.Set("X-API-KEY", "secret")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s with valid key: status = %d, want %d", method, w.Code, http.StatusOK)
		}
	}
}

func TestWriteProtect_Disabled_PassesAll(t *testing.T) {
	config := NewAuthConfigWithKeys(nil)
	handler := WriteProtect(config)(okHandler())

	methods := []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete}
	for _, method := range methods {
		req := httptest.NewRequest(method, "/", nil)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("%s with auth disabled: status = %d, want %d", method, w.Code, http.StatusOK)
		}
	}
}

func TestWriteProtect_InvalidKey_Rejected(t *testing.T) {
	config := NewAuthConfigWithKeys([]string{"secret"})
	handler := WriteProtect(config)(okHandler())

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("X-API-KEY", "wrong")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("POST with invalid key: status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}
