package api

import (
	"errors"
	"net/http"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *APIError
		want string
	}{
		{
			name: "with message",
			err:  &APIError{Errno: -6, Errmsg: "access denied"},
			want: "baidupan: API error errno=-6 msg=access denied",
		},
		{
			name: "without message",
			err:  &APIError{Errno: -9},
			want: "baidupan: API error errno=-9",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.want {
				t.Errorf("APIError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsErrno(t *testing.T) {
	apiErr := &APIError{Errno: -6, Errmsg: "access denied"}

	if !IsErrno(apiErr, -6) {
		t.Error("IsErrno should return true for matching errno")
	}

	if IsErrno(apiErr, -9) {
		t.Error("IsErrno should return false for non-matching errno")
	}

	if IsErrno(errors.New("other error"), -6) {
		t.Error("IsErrno should return false for non-APIError")
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	apiErr := &APIError{Errno: -6, Errmsg: "access denied", Response: &http.Response{StatusCode: 200}}
	var target *APIError
	if !errors.As(apiErr, &target) {
		t.Error("errors.As should work with APIError")
	}
	if target.Errno != -6 {
		t.Errorf("target.Errno = %d, want -6", target.Errno)
	}
}
