package imagebed

import (
	"testing"

	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/driver"
)

func TestPolicyRequiresRelayAndPrivateStorage(t *testing.T) {
	p := &ent.StoragePolicy{Server: "https://example.test", BucketName: "cloudreve-test", SecretKey: "synthetic-token", AccessKey: "s3", Settings: &types.PolicySetting{ImageBedChannelName:"private"}}
	if _, err := New(p, nil); err == nil {
		t.Fatal("unrelayed policy must be rejected")
	}
	p.Settings.Relay = true
	d, err := New(p, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !d.Capabilities().StaticFeatures.Enabled(int(driver.HandlerCapabilityProxyRequired)) {
		t.Fatal("private content must require internal proxy")
	}
	if d.Capabilities().StaticFeatures.Enabled(int(driver.HandlerCapabilityInboundGet)) {
		t.Fatal("remote storage must not be represented as a local file")
	}
}
