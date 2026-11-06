package entitysource

import (
	"net/http"

	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/driver"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs"
)

type contentTransport struct {
	source driver.ContentSource
	entity fs.Entity
}

func (t contentTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return t.source.Content(r.Context(), t.entity, r.Method, r.Header)
}
