// Package imagebed implements ImageBed's opt-in private storage protocol.
package imagebed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Config struct {
	Endpoint, Root, Token string
}

type Client struct {
	config Config
	http   *http.Client
}

var safePath = regexp.MustCompile(`^[a-zA-Z0-9_./-]+$`)

func validPath(p string) bool {
	return p != "" && safePath.MatchString(p) && !strings.Contains(p, "..") &&
		!strings.HasPrefix(p, "/") && path.Clean(p) == p && !strings.HasSuffix(p, "/")
}

func NewClient(config Config, client *http.Client) (*Client, error) {
	u, err := url.Parse(config.Endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("ImageBed endpoint must be an HTTPS origin without credentials, path or query")
	}
	config.Endpoint = strings.TrimSuffix(config.Endpoint, "/")
	if !validPath(config.Root) || strings.TrimSpace(config.Token) == "" || strings.ContainsAny(config.Token, "\r\n") {
		return nil, errors.New("ImageBed requires a safe nonempty root directory and API token")
	}
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Minute}
	}
	// Never follow redirects with a server credential, including same-origin login pages.
	c := *client
	c.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
	return &Client{config: config, http: &c}, nil
}

func (c *Client) checkPath(p string) error {
	if !validPath(p) || !strings.HasPrefix(p, c.config.Root+"/") {
		return errors.New("ImageBed object path must stay strictly inside the configured root and use safe physical names")
	}
	return nil
}

func (c *Client) request(ctx context.Context, method, route string, body io.Reader, headers http.Header, size int64) (*http.Response, error) {
	r, err := http.NewRequestWithContext(ctx, method, c.config.Endpoint+route, body)
	if err != nil {
		return nil, errors.New("failed to construct ImageBed request")
	}
	for k, v := range headers {
		r.Header[k] = append([]string(nil), v...)
	}
	r.Header.Set("Authorization", "Bearer "+c.config.Token)
	if body != nil {
		r.ContentLength = size
	}
	response, err := c.http.Do(r)
	if err != nil {
		// Do not expose transport/proxy errors containing credential-bearing requests.
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, errors.New("ImageBed request failed (network or TLS error)")
	}
	return response, nil
}

func decode(response *http.Response, dst any) error {
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("ImageBed returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 64<<10)).Decode(dst); err != nil {
		return errors.New("ImageBed returned an invalid API response")
	}
	return nil
}

func (c *Client) CheckCapabilities(ctx context.Context) error {
	r, err := c.request(ctx, "GET", "/api/manage/storageCapabilities", nil, nil, 0)
	if err != nil {
		return err
	}
	var result struct {
		PrivateUpload     bool `json:"privateUpload"`
		ServerSideRouting bool `json:"serverSideRouting"`
		Version           int  `json:"version"`
	}
	if err := decode(r, &result); err != nil {
		return fmt.Errorf("ImageBed private-upload capability check failed; update ImageBed first: %w", err)
	}
	if !result.PrivateUpload || result.Version != 1 || !result.ServerSideRouting {
		return errors.New("ImageBed does not support server-routed private storage; update ImageBed first")
	}
	return nil
}

// Content returns a response owned by the caller. Only file conditional/range
// headers cross this boundary; browser credentials and cookies never do.
func (c *Client) Content(ctx context.Context, object, method string, headers http.Header) (*http.Response, error) {
	if err := c.checkPath(object); err != nil {
		return nil, err
	}
	if method != http.MethodGet && method != http.MethodHead {
		return nil, errors.New("invalid ImageBed content method")
	}
	h := http.Header{}
	for _, k := range []string{"Range", "If-Range", "If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since"} {
		if v := headers.Get(k); v != "" {
			h.Set(k, v)
		}
	}
	return c.request(ctx, method, "/file/"+object+"?from=admin", nil, h, 0)
}

func (c *Client) Delete(ctx context.Context, object string) error {
	if err := c.checkPath(object); err != nil {
		return err
	}
	r, err := c.request(ctx, "DELETE", "/api/manage/delete/"+object, nil, nil, 0)
	if err != nil {
		return err
	}
	var result struct {
		Success bool `json:"success"`
	}
	if err := decode(r, &result); err != nil {
		return err
	}
	if !result.Success {
		return errors.New("ImageBed did not confirm object deletion")
	}
	return nil
}

func (c *Client) Put(ctx context.Context, object string, content io.Reader, size int64, mimeType, operationID string) (retErr error) {
	if err := c.checkPath(object); err != nil {
		return err
	}
	if size < 0 {
		return errors.New("negative ImageBed object size")
	}
	if operationID == "" || !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(operationID) {
		return errors.New("ImageBed upload requires a stable upload-session identifier")
	}
	unlock, err := lockObject(ctx, c.config.Endpoint+"/"+object)
	if err != nil {
		return err
	}
	defer unlock()
	// This happens before any write. Old deployments must not silently publish files.
	if err := c.CheckCapabilities(ctx); err != nil {
		return err
	}
	head, err := c.Content(ctx, object, "HEAD", nil)
	if err != nil {
		return err
	}
	head.Body.Close()
	if head.StatusCode == http.StatusOK && head.Header.Get("X-Imagebed-Upload-Id") == operationID && head.Header.Get("Content-Length") == strconv.FormatInt(size, 10) {
		// Consume retry bodies too: Cloudreve verifies its request reader count.
		n, err := io.Copy(io.Discard, io.LimitReader(contextReader{ctx, content}, size+1))
		if err != nil {
			return err
		}
		if n != size {
			return errors.New("ImageBed retry size differs from the declared size")
		}
		return nil // A previous request succeeded but its response was lost.
	}
	if head.StatusCode != http.StatusNotFound {
		return fmt.Errorf("ImageBed destination must not exist (HTTP %d); physical objects are immutable", head.StatusCode)
	}

	// Stage only on disk, validating the complete stream before any remote write.
	// Cloudreve's relay is a single request; ImageBed currently buffers it itself.
	f, err := os.CreateTemp("", "cloudreve-imagebed-*")
	if err != nil {
		return errors.New("failed to create ImageBed upload staging file")
	}
	defer os.Remove(f.Name())
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(contextReader{ctx, content}, size+1))
	if err != nil {
		return fmt.Errorf("failed to stage ImageBed upload: %w", err)
	}
	if n != size {
		return errors.New("ImageBed upload size differs from the declared size")
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}

	var framing bytes.Buffer
	m := multipart.NewWriter(&framing)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	if strings.ContainsAny(mimeType, "\r\n") {
		return errors.New("invalid ImageBed content type")
	}
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+path.Base(object)+`"`)
	h.Set("Content-Type", mimeType)
	if _, err := m.CreatePart(h); err != nil {
		return err
	}
	prefix := append([]byte(nil), framing.Bytes()...)
	framing.Reset()
	if err := m.Close(); err != nil {
		return err
	}
	suffix := append([]byte(nil), framing.Bytes()...)
	body := io.MultiReader(bytes.NewReader(prefix), f, bytes.NewReader(suffix))
	query := url.Values{"access": {"private"}, "uploadNameType": {"origin"}, "uploadFolder": {path.Dir(object)}, "autoRetry": {"false"}, "storageUploadId": {operationID}}
	// Reconcile ambiguous responses, including timeouts, before cleanup. Only an
	// object carrying this stable operation ID is eligible for deletion.
	defer func() {
		if retErr == nil {
			return
		}
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		response, err := c.Content(cleanupCtx, object, "HEAD", nil)
		if err != nil {
			retErr = errors.Join(retErr, errors.New("ImageBed upload outcome is unknown; retry the same session or cancel it in Cloudreve"))
			return
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK || response.Header.Get("X-Imagebed-Upload-Id") != operationID {
			return
		}
		if response.Header.Get("Content-Length") == strconv.FormatInt(size, 10) {
			retErr = nil
			return
		}
		if err := c.Delete(cleanupCtx, object); err != nil {
			retErr = errors.Join(retErr, fmt.Errorf("ImageBed cleanup failed for %s: %w", object, err))
		}
	}()
	r, err := c.request(ctx, "POST", "/upload?"+query.Encode(), body, http.Header{"Content-Type": {m.FormDataContentType()}}, int64(len(prefix)+len(suffix))+size)
	if err != nil {
		return err
	}
	var result []struct {
		Src      string `json:"src"`
		Access   string `json:"access"`
		UploadID string `json:"storageUploadId"`
	}
	if err := decode(r, &result); err != nil {
		return err
	}
	if len(result) != 1 || result[0].Src != "/file/"+object || result[0].Access != "private" || result[0].UploadID != operationID {
		return errors.New("ImageBed returned an unexpected object path or did not acknowledge private storage")
	}
	r, err = c.Content(ctx, object, "HEAD", nil)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != http.StatusOK || r.Header.Get("Content-Length") != strconv.FormatInt(size, 10) || r.Header.Get("X-Imagebed-Upload-Id") != operationID {
		return errors.New("ImageBed stored object could not be verified (status or size mismatch)")
	}
	return nil
}

// A single Cloudreve instance owns this storage prefix in v1. Lock across driver
// instances, not just Client instances, so concurrent HTTP retries cannot race.
var objectLocks = struct {
	sync.Mutex
	entries map[string]*objectLock
}{entries: make(map[string]*objectLock)}

type objectLock struct {
	held chan struct{}
	refs int
}

func lockObject(ctx context.Context, key string) (func(), error) {
	objectLocks.Lock()
	e := objectLocks.entries[key]
	if e == nil {
		e = &objectLock{held: make(chan struct{}, 1)}
		objectLocks.entries[key] = e
	}
	e.refs++
	objectLocks.Unlock()
	releaseRef := func() {
		objectLocks.Lock()
		defer objectLocks.Unlock()
		e.refs--
		if e.refs == 0 {
			delete(objectLocks.entries, key)
		}
	}
	select {
	case e.held <- struct{}{}:
		return func() { <-e.held; releaseRef() }, nil
	case <-ctx.Done():
		releaseRef()
		return nil, ctx.Err()
	}
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}
