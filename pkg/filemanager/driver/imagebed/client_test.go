package imagebed

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func testClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	s := httptest.NewTLSServer(h)
	t.Cleanup(s.Close)
	c, err := NewClient(Config{Endpoint: s.URL, Root: "cloudreve-test", Token: "synthetic-test-token"}, s.Client())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestUploadPrivateAndReadRange(t *testing.T) {
	var stored bool
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer synthetic-test-token" {
			t.Error("missing backend authentication")
		}
		switch r.URL.Path {
		case "/api/manage/storageCapabilities":
			io.WriteString(w, `{"privateUpload":true,"version":1,"serverSideRouting":true}`)
		case "/upload":
			if r.URL.Query().Has("uploadChannel") || r.URL.Query().Has("channelName") {
				t.Error("Cloudreve must not select ImageBed's internal storage channel")
			}
			if r.URL.Query().Get("access") != "private" || r.URL.Query().Get("uploadFolder") != "cloudreve-test/1" || r.URL.Query().Get("uploadNameType") != "origin" {
				t.Error("incorrect private upload parameters")
			}
			f, fh, err := r.FormFile("file")
			if err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			defer f.Close()
			b, _ := io.ReadAll(f)
			if fh.Filename != "unique.txt" || string(b) != "abcdef" {
				t.Error("multipart content differs")
			}
			stored = true
			io.WriteString(w, `[{"src":"/file/cloudreve-test/1/unique.txt","access":"private","storageUploadId":"test-operation"}]`)
		case "/file/cloudreve-test/1/unique.txt":
			if r.URL.Query().Get("from") != "admin" {
				t.Error("read must use authenticated admin preview")
			}
			if !stored {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", "6")
			w.Header().Set("X-Imagebed-Upload-Id", "test-operation")
			if r.Method == "HEAD" {
				return
			}
			if r.Header.Get("Range") != "bytes=2-4" {
				t.Error("Range not preserved")
			}
			w.Header().Set("Content-Length", "3")
			w.Header().Set("Content-Range", "bytes 2-4/6")
			w.WriteHeader(206)
			io.WriteString(w, "cde")
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	if err := c.Put(context.Background(), "cloudreve-test/1/unique.txt", strings.NewReader("abcdef"), 6, "text/plain", "test-operation"); err != nil {
		t.Fatal(err)
	}
	r, err := c.Content(context.Background(), "cloudreve-test/1/unique.txt", "GET", http.Header{"Range": {"bytes=2-4"}, "Cookie": {"do-not-forward"}})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Body.Close()
	b, _ := io.ReadAll(r.Body)
	if r.StatusCode != 206 || string(b) != "cde" {
		t.Fatalf("range read = %d %q", r.StatusCode, b)
	}
}

func TestOldServerIsRejectedBeforeUpload(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { requests++; io.WriteString(w, `<html>old server</html>`) })
	err := c.Put(context.Background(), "cloudreve-test/unique.txt", strings.NewReader("x"), 1, "text/plain", "test-operation")
	if err == nil || requests != 1 {
		t.Fatalf("old server must fail before writes: %v requests=%d", err, requests)
	}
}

func TestExistingExtensionlessObjectKeepsStoredPath(t *testing.T) {
	object := "cloudreve-test/7/56b627d6-d4c7-4e32-b44b-e2563d599819"
	var methods []string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		switch r.URL.Path {
		case "/file/" + object:
			w.Header().Set("Content-Length", "3")
			if r.Method == http.MethodGet {
				io.WriteString(w, "old")
			}
		case "/api/manage/delete/" + object:
			io.WriteString(w, `{"success":true}`)
		default:
			t.Errorf("stored path was rewritten: %s", r.URL.Path)
			w.WriteHeader(404)
		}
	})
	for _, method := range []string{http.MethodHead, http.MethodGet} {
		r, err := c.Content(context.Background(), object, method, nil)
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil || r.StatusCode != 200 || (method == http.MethodGet && string(b) != "old") {
			t.Fatalf("legacy read failed: %d %q %v", r.StatusCode, b, err)
		}
	}
	if err := c.Delete(context.Background(), object); err != nil {
		t.Fatal(err)
	}
	if strings.Join(methods, ",") != "HEAD,GET,DELETE" {
		t.Fatalf("unexpected legacy operations: %v", methods)
	}
}

func TestServerWithoutUnifiedRoutingIsRejectedBeforeWrite(t *testing.T) {
	requests := 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		io.WriteString(w, `{"privateUpload":true,"version":1}`)
	})
	if err := c.Put(context.Background(), "cloudreve-test/unique.txt", strings.NewReader("x"), 1, "text/plain", "operation"); err == nil || requests != 1 {
		t.Fatalf("missing server routing must fail before writing: %v requests=%d", err, requests)
	}
}

func TestPathConfinement(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unsafe path caused network request: %s", r.URL)
	})
	for _, p := range []string{"cloudreve-test", "other/x", "cloudreve-test-other/x", "cloudreve-test/../x", "cloudreve-test/%2e%2e/x", "cloudreve-test//x", "cloudreve-test/./x", "cloudreve-test/a,b", `cloudreve-test\x`, "/cloudreve-test/x", "cloudreve-test/a..b", "cloudreve-test/x?y"} {
		t.Run(p, func(t *testing.T) {
			if err := c.Delete(context.Background(), p); err == nil {
				t.Error("unsafe path accepted")
			}
		})
	}
}

func TestDeleteRequiresBusinessSuccess(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, `{"success":false}`) })
	if err := c.Delete(context.Background(), "cloudreve-test/unique.txt"); err == nil {
		t.Fatal("HTTP 200 alone is not a successful deletion")
	}
}

func TestUploadVerificationFailureCleansExactObject(t *testing.T) {
	stored, deleted := false, false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/manage/storageCapabilities":
			io.WriteString(w, `{"privateUpload":true,"version":1,"serverSideRouting":true}`)
		case "/upload":
			stored = true
			io.WriteString(w, `[{"src":"/file/cloudreve-test/unique.txt","access":"private","storageUploadId":"test-operation"}]`)
		case "/file/cloudreve-test/unique.txt":
			if !stored {
				w.WriteHeader(404)
			} else {
				w.Header().Set("Content-Length", "2")
				w.Header().Set("X-Imagebed-Upload-Id", "test-operation")
			}
		case "/api/manage/delete/cloudreve-test/unique.txt":
			deleted = true
			json.NewEncoder(w).Encode(map[string]bool{"success": true})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	})
	if err := c.Put(context.Background(), "cloudreve-test/unique.txt", strings.NewReader("x"), 1, "text/plain", "test-operation"); err == nil || !deleted {
		t.Fatalf("verification failure must clean up: err=%v deleted=%v", err, deleted)
	}
}

func TestCredentialsNeverFollowRedirects(t *testing.T) {
	leaked := false
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) })
	if err := c.Delete(context.Background(), "cloudreve-test/unique.txt"); err == nil || leaked {
		t.Fatalf("redirect must fail without forwarding credentials: %v leak=%v", err, leaked)
	}
}

func TestConcurrentUploadRetriesAreIdempotent(t *testing.T) {
	var mu sync.Mutex
	stored, uploads, deletes := false, 0, 0
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/manage/storageCapabilities":
			io.WriteString(w, `{"privateUpload":true,"version":1,"serverSideRouting":true}`)
		case "/upload":
			uploads++
			stored = true
			io.WriteString(w, `[{"src":"/file/cloudreve-test/unique.txt","access":"private","storageUploadId":"test-operation"}]`)
		case "/file/cloudreve-test/unique.txt":
			if !stored {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", "1")
			w.Header().Set("X-Imagebed-Upload-Id", "test-operation")
		case "/api/manage/delete/cloudreve-test/unique.txt":
			deletes++
			io.WriteString(w, `{"success":true}`)
		}
	})
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := c.Put(context.Background(), "cloudreve-test/unique.txt", strings.NewReader("x"), 1, "text/plain", "test-operation"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if uploads != 1 || deletes != 0 {
		t.Fatalf("concurrent retry uploaded %d times and deleted %d objects", uploads, deletes)
	}
}

func TestUncertainUploadResponseReconcilesWithoutDeletion(t *testing.T) {
	stored, deleted := false, false
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/manage/storageCapabilities":
			io.WriteString(w, `{"privateUpload":true,"version":1,"serverSideRouting":true}`)
		case "/upload":
			stored = true
			io.WriteString(w, `invalid-truncated-response`)
		case "/file/cloudreve-test/unique.txt":
			if !stored {
				w.WriteHeader(404)
				return
			}
			w.Header().Set("Content-Length", "1")
			w.Header().Set("X-Imagebed-Upload-Id", "test-operation")
		case "/api/manage/delete/cloudreve-test/unique.txt":
			deleted = true
			io.WriteString(w, `{"success":true}`)
		}
	})
	if err := c.Put(context.Background(), "cloudreve-test/unique.txt", strings.NewReader("x"), 1, "text/plain", "test-operation"); err != nil || deleted {
		t.Fatalf("reconciliation err=%v deleted=%v", err, deleted)
	}
}
