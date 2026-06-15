package pdfconv

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestSignExact(t *testing.T) {
	c := NewClient("http://example.com", "mykey", "my-secret-key")

	// Known inputs from API spec example
	method := "POST"
	path := "/openapi/v2/upload"
	query := ""
	ts := "1718178600"
	nonce := "f47ac10b58cc4372a5670e02b2c3d479"
	bodyLen := "62"

	// Independently compute expected HMAC per spec:
	// signStr = METHOD\nPATH\nQUERY\nTIMESTAMP\nNONCE\nBODY_LENGTH
	signStr := method + "\n" + path + "\n" + query + "\n" + ts + "\n" + nonce + "\n" + bodyLen
	h := hmac.New(sha256.New, []byte("my-secret-key"))
	h.Write([]byte(signStr))
	expected := hex.EncodeToString(h.Sum(nil))

	got := c.sign(method, path, query, ts, nonce, bodyLen)
	if got != expected {
		t.Errorf("sign() = %q, want %q", got, expected)
	}

	// Changing any field must produce a different signature
	for _, tc := range []struct{ name, val string }{
		{"method", "GET"},
		{"path", "/openapi/v2/status"},
		{"query", "taskID=abc"},
		{"ts", "0"},
		{"nonce", "different"},
		{"bodyLen", "0"},
	} {
		var alt string
		switch tc.name {
		case "method":
			alt = c.sign(tc.val, path, query, ts, nonce, bodyLen)
		case "path":
			alt = c.sign(method, tc.val, query, ts, nonce, bodyLen)
		case "query":
			alt = c.sign(method, path, tc.val, ts, nonce, bodyLen)
		case "ts":
			alt = c.sign(method, path, query, tc.val, nonce, bodyLen)
		case "nonce":
			alt = c.sign(method, path, query, ts, tc.val, bodyLen)
		case "bodyLen":
			alt = c.sign(method, path, query, ts, nonce, tc.val)
		}
		if alt == got {
			t.Errorf("changing %s did not change signature", tc.name)
		}
	}
}

func TestAuthHeaders(t *testing.T) {
	c := NewClient("http://example.com", "mykey", "mysecret")
	h := c.authHeaders("POST", "/openapi/v2/upload", "", "100")

	if h["X-Api-Key"] != "mykey" {
		t.Errorf("X-Api-Key: got %q", h["X-Api-Key"])
	}
	for _, k := range []string{"X-Timestamp", "X-Nonce", "X-Signature"} {
		if h[k] == "" {
			t.Errorf("missing %s", k)
		}
	}

	// Different requests must produce different signatures (nonce changes)
	h2 := c.authHeaders("POST", "/openapi/v2/upload", "", "100")
	if h["X-Signature"] == h2["X-Signature"] {
		t.Error("signatures should differ due to different nonce/timestamp")
	}
}

func TestRequestUpload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/openapi/v2/upload" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		for _, h := range []string{"X-Api-Key", "X-Timestamp", "X-Nonce", "X-Signature"} {
			if r.Header.Get(h) == "" {
				t.Errorf("missing header %s", h)
			}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"taskID":        "task123",
				"uploadURL":     "https://r2.example.com/upload",
				"uploadHeaders": map[string]string{"Content-Type": "application/octet-stream"},
				"commitURL":     "/openapi/v2/commit?taskID=task123",
				"expireAt":      1234567890,
			},
		})
	}))
	defer srv.Close()

	info, err := NewClient(srv.URL, "k", "s").RequestUpload(context.Background(), "test.pdf", "pdf2word", 1024)
	if err != nil {
		t.Fatal(err)
	}
	if info.TaskID != "task123" {
		t.Errorf("TaskID: got %q", info.TaskID)
	}
}

func TestRequestUpload_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized", "message": "bad key"})
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "k", "s").RequestUpload(context.Background(), "f.pdf", "pdf2word", 100)
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T: %v", err, err)
	}
	if apiErr.Code != "unauthorized" || apiErr.HTTPStatus != 401 {
		t.Errorf("unexpected APIError: %+v", apiErr)
	}
}

func TestUploadFile(t *testing.T) {
	content := []byte("pdf bytes")
	tmp := filepath.Join(t.TempDir(), "f.pdf")
	os.WriteFile(tmp, content, 0644)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("expected PUT, got %s", r.Method)
		}
		body, _ := io.ReadAll(r.Body)
		if string(body) != string(content) {
			t.Errorf("body mismatch")
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	err := NewClient("http://x.com", "k", "s").UploadFile(context.Background(), srv.URL, map[string]string{"X-Upload-Token": "tok"}, tmp)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCommitConversion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/openapi/v2/commit" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("taskID") == "" {
			t.Error("missing taskID")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"code": 0, "data": map[string]string{"status": "queued"}})
	}))
	defer srv.Close()

	if err := NewClient(srv.URL, "k", "s").CommitConversion(context.Background(), "task123"); err != nil {
		t.Fatal(err)
	}
}

func TestGetStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"taskID": "task123", "status": "processing", "progress": 50,
		})
	}))
	defer srv.Close()

	s, err := NewClient(srv.URL, "k", "s").GetStatus(context.Background(), "task123")
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != "processing" || s.Progress != 50 {
		t.Errorf("unexpected status: %+v", s)
	}
}

func TestGetDownloadURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"taskID": "task123", "downloadURL": "https://files.example.com/out.docx",
				"filename": "report.docx", "expireAt": 9999,
			},
		})
	}))
	defer srv.Close()

	d, err := NewClient(srv.URL, "k", "s").GetDownloadURL(context.Background(), "task123")
	if err != nil {
		t.Fatal(err)
	}
	if d.Filename != "report.docx" {
		t.Errorf("Filename: got %q", d.Filename)
	}
}

func TestGetUsage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/openapi/v2/usage" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"code": 0,
			"data": map[string]interface{}{
				"enabled":     true,
				"description": "",
				"countMode":   "both",
				"calls":       map[string]int{"total": 200, "used": 55, "remaining": 145},
				"pages":       map[string]int{"total": 300, "used": 445, "remaining": -145},
			},
		})
	}))
	defer srv.Close()

	u, err := NewClient(srv.URL, "k", "s").GetUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !u.Enabled || u.CountMode != "both" || u.Calls.Remaining != 145 || u.Pages.Remaining != -145 {
		t.Errorf("unexpected usage: %+v", u)
	}
}

func TestDownloadFile(t *testing.T) {
	content := []byte("docx content")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(content)
	}))
	defer srv.Close()

	out := filepath.Join(t.TempDir(), "result.docx")
	if err := NewClient("http://x.com", "k", "s").DownloadFile(context.Background(), srv.URL, out); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != string(content) {
		t.Error("file content mismatch")
	}
}
