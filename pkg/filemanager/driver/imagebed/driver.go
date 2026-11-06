package imagebed

import (
	"context"
	"errors"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/pkg/boolset"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/driver"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs/mime"
)

type Driver struct {
	client *Client
	mime   mime.MimeDetector
}

var _ driver.Handler = (*Driver)(nil)
var _ driver.ContentSource = (*Driver)(nil)

func New(p *ent.StoragePolicy, detector mime.MimeDetector) (*Driver, error) {
	if p.Settings == nil || !p.Settings.Relay || p.Settings.ChunkSize != 0 || p.Settings.Encryption {
		return nil, errors.New("ImageBed requires relay upload, zero chunk size and disabled storage encryption")
	}
	c, err := NewClient(Config{Endpoint: p.Server, Root: p.BucketName, Token: p.SecretKey}, nil)
	if err != nil {
		return nil, err
	}
	return &Driver{client: c, mime: detector}, nil
}

func (d *Driver) Put(ctx context.Context, file *fs.UploadRequest) error {
	defer file.Close()
	if file.Offset != 0 {
		return errors.New("ImageBed does not support chunk offsets")
	}
	mimeType := file.Props.MimeType
	if mimeType == "" && d.mime != nil && file.Props.Uri != nil {
		mimeType = d.mime.TypeByName(file.Props.Uri.Name())
	}
	return d.client.Put(ctx, file.Props.SavePath, file, file.Props.Size, mimeType, file.Props.UploadSessionID)
}

func (d *Driver) Delete(ctx context.Context, files ...string) ([]string, error) {
	var failed []string
	var result error
	for _, object := range files {
		if err := d.client.Delete(ctx, object); err != nil {
			failed = append(failed, object)
			result = errors.Join(result, err)
		}
	}
	return failed, result
}

func (d *Driver) Content(ctx context.Context, e fs.Entity, method string, headers http.Header) (*http.Response, error) {
	return d.client.Content(ctx, e.Source(), method, headers)
}

func (d *Driver) Open(context.Context, string) (*os.File, error) {
	return nil, errors.New("ImageBed is not a local file system")
}
func (d *Driver) LocalPath(context.Context, string) string { return "" }
func (d *Driver) Thumb(context.Context, *time.Time, string, fs.Entity) (string, error) {
	return "", errors.New("ImageBed thumbnails use Cloudreve's local generator")
}
func (d *Driver) Source(context.Context, fs.Entity, *driver.GetSourceArgs) (string, error) {
	return "", errors.New("ImageBed content must be served by Cloudreve's authenticated proxy")
}
func (d *Driver) Token(context.Context, *fs.UploadSession, *fs.UploadRequest) (*fs.UploadCredential, error) {
	return nil, errors.New("ImageBed requires server-relayed upload")
}
func (d *Driver) CancelToken(context.Context, *fs.UploadSession) error { return nil }
func (d *Driver) CompleteUpload(ctx context.Context, session *fs.UploadSession) error {
	r, err := d.client.Content(ctx, session.Props.SavePath, "HEAD", nil)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 || r.Header.Get("Content-Length") != strconv.FormatInt(session.Props.Size, 10) || r.Header.Get("X-Imagebed-Upload-Id") != session.Props.UploadSessionID {
		return errors.New("ImageBed upload is incomplete")
	}
	return nil
}
func (d *Driver) List(context.Context, string, driver.ListProgressFunc, bool) ([]fs.PhysicalObject, error) {
	return nil, errors.New("ImageBed storage import is not supported; upload and organize files through Cloudreve")
}
func (d *Driver) MediaMeta(context.Context, string, string, string) ([]driver.MediaMeta, error) {
	return nil, errors.New("ImageBed media metadata uses Cloudreve's local generator")
}
func (d *Driver) Capabilities() *driver.Capabilities {
	f := &boolset.BooleanSet{}
	boolset.Set(driver.HandlerCapabilityProxyRequired, true, f)
	return &driver.Capabilities{StaticFeatures: f, ThumbProxy: true, MediaMetaProxy: true}
}
