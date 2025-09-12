package cloudinary

import (
	"context"

	"github.com/cloudinary/cloudinary-go"
	"github.com/cloudinary/cloudinary-go/api/uploader"
	"github.com/jofosuware/go/shopit/config"
)

type CloudUploader interface {
	UploadToCloud(folder string, data interface{}) (*uploader.UploadResult, error)
	Destroy(id string) (*uploader.DestroyResult, error)
}

type Cloudinary struct {
	cld *cloudinary.Cloudinary
}

func NewCloudinary(cfg *config.Config) (*Cloudinary, error) {
	cld, err := cloudinary.NewFromParams(cfg.Cloudinary.Name, cfg.Cloudinary.Key, cfg.Cloudinary.Secret)
	return &Cloudinary{
		cld: cld,
	}, err
}

func (c *Cloudinary) UploadToCloud(folder string, data interface{}) (*uploader.UploadResult, error) {
	res, err := c.cld.Upload.Upload(context.Background(), data, uploader.UploadParams{Folder: folder})
	if err != nil {
		return &uploader.UploadResult{}, err
	}
	return res, nil
}

func (c *Cloudinary) Destroy(id string) (*uploader.DestroyResult, error) {
	res, err := c.cld.Upload.Destroy(context.Background(), uploader.DestroyParams{PublicID: id})
	if err != nil {
		return &uploader.DestroyResult{}, err
	}
	return res, nil
}
