package api

import (
	"net/http"
	"testing"
)

// =============================================================================
// Nas.IoTQueryUInfo 测试
// =============================================================================

func TestNas_IoTQueryUInfo(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %q, want GET", r.Method)
		}
		q := r.URL.Query()
		if q.Get("method") != "iotqueryuinfo" {
			t.Errorf("method param = %q, want iotqueryuinfo", q.Get("method"))
		}
		if q.Get("device_id") != "test_device_123" {
			t.Errorf("device_id = %q, want test_device_123", q.Get("device_id"))
		}

		w.Write([]byte(`{
			"request_id": "1234567890",
			"error_code": 0,
			"error_msg": "success",
			"data": {
				"has_privilege": 1,
				"is_svip": 0,
				"is_iot_svip": 1,
				"start_time": 1640995200,
				"end_time": 1672531200,
				"now": 1713756789
			}
		}`))
	})
	defer ts.Close()

	resp, err := c.Nas.IoTQueryUInfo(ctx(), &IoTQueryUInfoParams{
		DeviceID: "test_device_123",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ErrorCode != 0 {
		t.Errorf("ErrorCode = %d, want 0", resp.ErrorCode)
	}
	if resp.Data == nil {
		t.Fatal("Data is nil")
	}
	if resp.Data.HasPrivilege != 1 {
		t.Errorf("HasPrivilege = %d, want 1", resp.Data.HasPrivilege)
	}
	if resp.Data.IsSVIP != 0 {
		t.Errorf("IsSVIP = %d, want 0", resp.Data.IsSVIP)
	}
	if resp.Data.IsIoTSVIP != 1 {
		t.Errorf("IsIoTSVIP = %d, want 1", resp.Data.IsIoTSVIP)
	}
	if resp.Data.StartTime != 1640995200 {
		t.Errorf("StartTime = %d, want 1640995200", resp.Data.StartTime)
	}
	if resp.Data.EndTime != 1672531200 {
		t.Errorf("EndTime = %d, want 1672531200", resp.Data.EndTime)
	}
	if resp.Data.Now != 1713756789 {
		t.Errorf("Now = %d, want 1713756789", resp.Data.Now)
	}
	if resp.RequestID.String() != "1234567890" {
		t.Errorf("RequestID = %s, want 1234567890", resp.RequestID.String())
	}
}

func TestNas_IoTQueryUInfo_NilParams(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("device_id") != "" {
			t.Errorf("device_id should be empty, got %q", q.Get("device_id"))
		}
		w.Write([]byte(`{
			"request_id": "1",
			"error_code": 0,
			"error_msg": "success",
			"data": {
				"has_privilege": 0,
				"is_svip": 0,
				"is_iot_svip": 0,
				"start_time": 0,
				"end_time": 0,
				"now": 1713756789
			}
		}`))
	})
	defer ts.Close()

	resp, err := c.Nas.IoTQueryUInfo(ctx(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Data == nil {
		t.Fatal("Data is nil")
	}
	if resp.Data.HasPrivilege != 0 {
		t.Errorf("HasPrivilege = %d, want 0", resp.Data.HasPrivilege)
	}
}

func TestNas_IoTQueryUInfo_APIError(t *testing.T) {
	ts, c := newTestServer(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"error_code":2,"error_msg":"param error"}`))
	})
	defer ts.Close()

	_, err := c.Nas.IoTQueryUInfo(ctx(), nil)
	if err == nil {
		t.Fatal("expected error for API error")
	}
	if !IsErrno(err, 2) {
		t.Errorf("expected errno=2, got: %v", err)
	}
}
