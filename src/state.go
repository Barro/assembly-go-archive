package state

import (
	"base"
)

// Global state of this site. API package updates this and site
// package uses it to render the pages.
type SiteState struct {
	Years []*base.Year
}

type Navigable interface {
	Context() *interface{}
	Parent() *Navigable
	Next() *Navigable
	Prev() *Navigable
}

type NavYear struct {
	context *base.Year
	next    *NavYear
	prev    *NavYear
}

func (year *NavYear) Context() *base.Year {
	return year.context
}
func (year *NavYear) Parent() *NavYear {
	return nil
}
func (year *NavYear) Next() *NavYear {
	return year.next
}
func (year *NavYear) Prev() *NavYear {
	return year.prev
}

type NavSection struct {
	context *base.Section
	parent  *NavYear
	next    *NavSection
	prev    *NavSection
}

func (section *NavSection) Context() *base.Section {
	return section.context
}
func (section *NavSection) Parent() *NavYear {
	return section.parent
}
func (section *NavSection) Next() *NavSection {
	return section.next
}
func (section *NavSection) Prev() *NavSection {
	return section.prev
}

type NavEntry struct {
	context *base.Entry
	parent  *NavSection
	next    *NavEntry
	prev    *NavEntry
}

func (entry *NavEntry) Context() *base.Entry {
	return entry.context
}
func (entry *NavEntry) Parent() *NavSection {
	return entry.parent
}
func (entry *NavEntry) Next() *NavEntry {
	return entry.next
}
func (entry *NavEntry) Prev() *NavEntry {
	return entry.prev
}

var StateInstance SiteState
var Pages map[string]NavEntry
