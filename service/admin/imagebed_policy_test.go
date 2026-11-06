package admin

import (
	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"testing"
)

func TestImageBedPolicyValidation(t *testing.T) {
	ordinary := &ent.StoragePolicy{Type: types.PolicyTypeLocal, DirNameRule: "existing/path"}
	if err := prepareImageBedPolicy(ordinary); err != nil || ordinary.DirNameRule != "existing/path" {
		t.Fatal("legacy policy contract changed")
	}
	p := &ent.StoragePolicy{Type: types.PolicyTypeImageBed, Server: "https://example.test", BucketName: "cloudreve-test", SecretKey: "synthetic-token", AccessKey: "s3", MaxSize: 100 << 20, Settings: &types.PolicySetting{Relay: true, ImageBedChannelName: "private"}}
	if err := prepareImageBedPolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.DirNameRule != "cloudreve-test/{uid}" || p.FileNameRule != "{uuid}{ext}" || !p.IsPrivate {
		t.Fatal("physical paths must remain stable and confined")
	}
	p.Settings.Relay = false
	if err := prepareImageBedPolicy(p); err == nil {
		t.Fatal("invalid relay policy accepted")
	}
}
