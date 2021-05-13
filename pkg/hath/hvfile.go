package hath

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cast"

	"github.com/mayocream/hath-go/pkg/hath/util"
)

// HVFile ...
type HVFile struct {
	Hash string `json:"hash"`
	Type string `json:"type"`
	Size int    `json:"size"`
	Xres int    `json:"xres"`
	Yres int    `json:"yres"`

	Data []byte `json:"-"`
}

// NewHVFileFromFileID ...
func NewHVFileFromFileID(fileID string) (*HVFile, error) {
	if !util.ValidHVFileID(fileID) {
		return nil, errors.New("invalid fileID")
	}
	parts := strings.Split(fileID, "-")
	return &HVFile{
		Hash: parts[0],
		Type: parts[4],
		Size: cast.ToInt(parts[1]),
		Xres: cast.ToInt(parts[2]),
		Yres: cast.ToInt(parts[3]),
	}, nil
}

// FileID main key
func (f *HVFile) FileID() string {
	return fmt.Sprintf("%s-%v-%v-%v-%s", f.Hash, f.Size, f.Xres, f.Yres, f.Type)
}

// MIMEType MIME Type
func (f *HVFile) MIMEType() string {
	switch f.Type {
	case "jpg":
		return ContentTypeJPG
	case "png":
		return ContentTypePNG
	case "gif":
		return ContentTypeWEBM
	case "wbm":
		return ContentTypeWEBM
	default:
		return ContentTypeOctet
	}
}