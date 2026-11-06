package entitysource

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"github.com/cloudreve/Cloudreve/v4/pkg/boolset"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/driver"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs"
	"github.com/cloudreve/Cloudreve/v4/pkg/hashid"
	"github.com/cloudreve/Cloudreve/v4/pkg/logging"
)

type contentDriver struct {
	driver.Handler
	called int
	status int
}

func (d *contentDriver) Capabilities() *driver.Capabilities {
	f := &boolset.BooleanSet{}
	boolset.Set(driver.HandlerCapabilityProxyRequired, true, f)
	return &driver.Capabilities{StaticFeatures: f}
}
func (d *contentDriver) Content(ctx context.Context, e fs.Entity, method string, headers http.Header) (*http.Response, error) {
	d.called++
	status := d.status
	if status == 0 {
		status = 206
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Range": {"bytes 2-4/6"}, "Content-Length": {"3"}}, Body: io.NopCloser(strings.NewReader("cde")), Request: &http.Request{Method: method, URL: httptest.NewRequest("GET", "https://storage.test/file", nil).URL}}, nil
}
func testSource(t *testing.T, d *contentDriver) *entitySource {
	t.Helper()
	hasher, err := hashid.New("test")
	if err != nil {
		t.Fatal(err)
	}
	return &entitySource{handler: d, e: fs.NewEntity(&ent.Entity{ID: 1, Size: 6, Source: "cloudreve-test/unique.txt"}), policy: &ent.StoragePolicy{Settings: &types.PolicySetting{}}, o: &EntitySourceOptions{Ctx: context.Background()}, hasher: hasher, l: logging.NewConsoleLogger(logging.LevelError)}
}
func TestPrivateRemoteReadDoesNotGenerateRecursiveProxyURL(t *testing.T) {
	d := &contentDriver{}
	f := testSource(t, d)
	r, err := f.getRsc(2)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	b, _ := io.ReadAll(r)
	if d.called != 1 || string(b) != "cde" {
		t.Fatalf("private read failed: calls=%d body=%q", d.called, b)
	}
}
func TestPrivateRemoteServeSupportsHeadAndRange(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		t.Run(method, func(t *testing.T) {
			d := &contentDriver{}
			f := testSource(t, d)
			r := httptest.NewRequest(method, "http://cloudreve.test/content", nil)
			r.Header.Set("Range", "bytes=2-4")
			w := httptest.NewRecorder()
			f.Serve(w, r)
			if w.Code != 206 || d.called != 1 {
				t.Fatalf("serve=%d calls=%d", w.Code, d.called)
			}
		})
	}
}
