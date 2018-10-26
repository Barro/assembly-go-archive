package base

type SiteSettings struct {
	DataDir      string
	StaticDir    string
	TemplatesDir string
}

type Resolution struct {
	X int
	Y int
}

type ThumbnailInfo struct {
	Path     string
	Checksum *string
	Size     Resolution
	Type     string
}

type TypedThumbnails struct {
	Type       string
	Thumbnails []ThumbnailInfo
}

type Thumbnails struct {
	Default ThumbnailInfo
	Extra   []TypedThumbnails
}

type ExternalLinksSection struct {
	Name  string
	Links []string
}

// Structure that has all known data about an entry.
type EntryInfo struct {
	Path          string
	Key           string
	Title         string
	Author        string
	Asset         string
	Description   string
	ExternalLinks []ExternalLinksSection
	Thumbnails    Thumbnails
}

type ThumbnailedEntry struct {
	Path       string
	Key        string
	Title      string
	Author     string
	Thumbnails Thumbnails
}

type Section struct {
	Path        string
	Key         string
	Name        string
	Description string
	IsRanked    bool
	Entries     []ThumbnailedEntry
}

type Year struct {
	Path     string
	Key      string
	Name     string
	Sections []Section
}
