package handler

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"

	"github.com/smartedu/training-eval-system/internal/dto"
)

// JSON writes a JSON response with the given status code.
// Nil slices are converted to empty arrays to avoid null in JSON.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	// Convert nil slices to empty slices so JSON is [] not null
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice && rv.IsNil() {
		w.Write([]byte("[]\n"))
		return
	}
	json.NewEncoder(w).Encode(v)
}

// Error writes a JSON error response.
func Error(w http.ResponseWriter, status int, message string) {
	JSON(w, status, dto.ErrorResponse{Detail: message})
}

// Decode reads and decodes a JSON request body into v.
func Decode(r *http.Request, v any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(v)
}

// PathInt64 extracts an int64 path parameter from chi URL params.
func PathInt64(r *http.Request, param string) (int64, error) {
	s := r.PathValue(param)
	return strconv.ParseInt(s, 10, 64)
}

// QueryInt returns an integer query parameter with a default value.
func QueryInt(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}

// QueryStr returns a string query parameter with a default value.
func QueryStr(r *http.Request, key, defaultVal string) string {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	return s
}
