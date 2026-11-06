package dbfs

import (
	"net/url"
	"path"
	"strings"
	"testing"

	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/fs"
	"github.com/gofrs/uuid"
)

func TestGenerateImageBedSavePath(t *testing.T) {
	// Existing policies still store {uuid}; they must adopt the new naming rule
	// without editing the policy or touching existing entities/session paths.
	policy := &ent.StoragePolicy{Type: types.PolicyTypeImageBed, DirNameRule: "cloudreve-test/{uid}", FileNameRule: "{uuid}"}
	for _, tc := range []struct{ name, ext string }{
		{"交互示例.html", ".html"}, {"照片.PNG", ".PNG"},
		{"archive.tar.gz", ".gz"}, {"README", ""}, {".env", ".env"},
		{"note.", ""}, {"file.中文", ""}, {"file.{uid}", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := &fs.UploadRequest{Props: &fs.UploadProps{Uri: &fs.URI{U: &url.URL{Path: "/folder/" + tc.name}}, SavePath: "cloudreve-test/7/old-object"}}
			got := generateSavePath(policy, req, &ent.User{ID: 7})
			if path.Dir(got) != "cloudreve-test/7" || path.Ext(got) != tc.ext {
				t.Fatalf("unexpected physical path %q (extension %q)", got, tc.ext)
			}
			id, err := uuid.FromString(strings.TrimSuffix(path.Base(got), tc.ext))
			if err != nil || id.Version() != 4 {
				t.Fatalf("expected UUID v4 + original extension, got %q", got)
			}
			if req.Props.SavePath != "cloudreve-test/7/old-object" || policy.FileNameRule != "{uuid}" {
				t.Fatal("path generation mutated an existing session or policy")
			}
			if generateSavePath(policy, req, &ent.User{ID: 7}) == got {
				t.Fatal("different uploads must receive different physical names")
			}
		})
	}
}

func TestGenerateOrdinarySavePathKeepsPolicyRule(t *testing.T) {
	p := &ent.StoragePolicy{Type: types.PolicyTypeLocal, DirNameRule: "existing/{uid}", FileNameRule: "{originname}"}
	req := &fs.UploadRequest{Props: &fs.UploadProps{Uri: &fs.URI{U: &url.URL{Path: "/folder/中文.HTML"}}}}
	if got := generateSavePath(p, req, &ent.User{ID: 7}); got != "existing/7/中文.HTML" {
		t.Fatalf("ordinary policy changed: %q", got)
	}
}
