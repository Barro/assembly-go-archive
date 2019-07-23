package base

type SiteSettings struct {
	SiteRoot     string
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

type ExternalLink struct {
	Href     string
	Contents string
	Notes    string
}

type ExternalLinksSection struct {
	Name  string
	Links []ExternalLink
}

// Structure that has all known data about an entry.
type Entry struct {
	Path          string
	Key           string
	Title         string
	Author        string
	Asset         string
	Description   string
	ExternalLinks []ExternalLinksSection
	Thumbnails    Thumbnails
}

type Section struct {
	Path        string
	Key         string
	Name        string
	Description string
	IsRanked    bool
	Entries     []*Entry
}

type Year struct {
	Year     int
	Path     string
	Key      string
	Name     string
	Sections []*Section
}
