package admin

import (
	"errors"
	"github.com/cloudreve/Cloudreve/v4/ent"
	"github.com/cloudreve/Cloudreve/v4/inventory/types"
	"github.com/cloudreve/Cloudreve/v4/pkg/filemanager/driver/imagebed"
)

// New constraints apply only to ImageBed; existing provider contracts are unchanged.
func prepareImageBedPolicy(p *ent.StoragePolicy) error {
	if p.Type != types.PolicyTypeImageBed {
		return nil
	}
	if _, err := imagebed.New(p, nil); err != nil {
		return err
	}
	if p.MaxSize <= 0 || p.MaxSize > 100<<20 {
		return errors.New("ImageBed v1 requires a single-file size limit between 1 byte and 100 MiB")
	}
	if p.Settings.CustomProxy {
		return errors.New("ImageBed private content cannot use a public download CDN")
	}
	p.IsPrivate = true
	p.DirNameRule = p.BucketName + "/{uid}"
	p.FileNameRule = "{uuid}{ext}"
	return nil
}
