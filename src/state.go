package state

import (
	"base"
)

// Global state of this site. API package updates this and site
// package uses it to render the pages.
type SiteState struct {
	Years []*base.Year
}

var StateInstance SiteState
