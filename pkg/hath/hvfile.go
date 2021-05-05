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

func (f *HVFile) String() string {
	return fmt.Sprintf("%s-%v-%v-%v-%s", f.Hash, f.Size, f.Xres, f.Yres, f.Type)
}
