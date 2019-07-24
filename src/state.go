package state

import (
	"base"
)

type FileInfo struct {
	Checksum string
}

// Global state of this site. API package updates this and site
// package uses it to render the pages.
type SiteState struct {
	Years []*base.Year
}

type YoutubeAsset struct {
	Id string
}

type ImageAsset struct {
	Default base.ImageInfo
}

type VimeoAsset struct {
	Id string
}

var StateInstance SiteState
