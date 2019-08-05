package state

import (
	"base"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

// 128 kilobytes is able to hold 2000 entries with 50 bytes/entry +
// some extra. We should never be even close to this metadata size for
// any year, section, or entry.
var MAX_METADATA_SIZE int64 = 128 * 1024

type FileInfo struct {
	Checksum string
}

// Global state of this site. API package updates this and site
// package uses it to render the pages.
type SiteState struct {
	SiteRoot string
	DataDir  string
	Years    []*base.Year
}

type YoutubeAsset struct {
	Id string
}

type ImageAsset struct {
	Default base.ImageInfo
	Sources []base.ImageInfo
}

type VimeoAsset struct {
	Id string
}

func (s *SiteState) UpdateYear(year string) error {
	return nil
}

func (s *SiteState) UpdateSection(year string, section string) error {
	return nil
}

func ReadMetaBytes(directory string) ([]byte, error) {
	meta_path := filepath.Join(directory, "meta.json")
	stats, err_stat := os.Stat(meta_path)
	if err_stat != nil {
		return nil, err_stat
	}
	if stats.Size() > MAX_METADATA_SIZE {
		return nil, fmt.Errorf(
			"%s size of %d bytes exceeds the maximum of %d bytes!",
			meta_path,
			stats.Size(),
			MAX_METADATA_SIZE)
	}
	if data, err_read := ioutil.ReadFile(meta_path); err_read != nil {
		return nil, err_read
	} else {
		return data, nil
	}
}

type ImageInfoMeta struct {
	Filename string
	Type     string
	Checksum string
	Size     base.Resolution
}

type ThumbnailsMeta struct {
	Default ImageInfoMeta
	Sources []ImageInfoMeta
}

type EntryAsset struct {
	Type string
	Data interface{}
}

func validate_image_info_meta(info ImageInfoMeta) error {
	if len(info.Filename) < len("a.png") {
		return fmt.Errorf(
			"Image file name '%s' is too short to be a valid one",
			info.Filename)
	}
	if len(info.Type) < len("image/png") {
		return fmt.Errorf(
			"Image %s type '%s' is too short to be a valid one",
			info.Filename,
			info.Type)
	}
	if len(info.Checksum) < 6 {
		return fmt.Errorf(
			"Image %s checksum '%s' does not contain enough entropy",
			info.Filename,
			info.Checksum)
	}
	if info.Size.X < 16 || info.Size.Y < 16 {
		return fmt.Errorf(
			"Image %s size %dx%d is too small!",
			info.Filename,
			info.Size.X,
			info.Size.Y)
	}
	return nil
}

func (asset *EntryAsset) UnmarshalJSON(data []byte) error {
	type AssetType struct {
		Type string
	}
	var asset_type AssetType
	err_type := json.Unmarshal(data, &asset_type)
	if err_type != nil {
		return err_type
	}
	asset.Type = asset_type.Type
	if asset_type.Type == "image" {
		type AssetData struct {
			Data ThumbnailsMeta
		}
		var asset_data AssetData
		err_data := json.Unmarshal(data, &asset_data)
		if err_data != nil {
			return err_data
		}
		if err := validate_image_info_meta(asset_data.Data.Default); err != nil {
			return err
		}

		image_sources := make([]base.ImageInfo, len(asset_data.Data.Sources))
		for index, image := range asset_data.Data.Sources {
			if err := validate_image_info_meta(image); err != nil {
				return fmt.Errorf(
					"Source image error %s: %v",
					asset_data.Data.Default.Filename, err)
			}
			image_sources[index] = get_entry_image("", "", image)
		}
		image_data := ImageAsset{
			Default: get_entry_image("", "", asset_data.Data.Default),
			Sources: image_sources,
		}
		asset.Data = image_data
	} else if asset_type.Type == "youtube" {
		type AssetData struct {
			Data YoutubeAsset
		}
		var asset_data AssetData
		err_data := json.Unmarshal(data, &asset_data)
		if err_data != nil {
			return err_data
		}
		asset.Data = asset_data.Data
	} else {
		return fmt.Errorf("Unknown asset type %s", asset_type)
	}
	return nil
}

type EntryMeta struct {
	Title         string
	Author        string `json:""`
	Asset         EntryAsset
	Description   string                      `json:""`
	ExternalLinks []base.ExternalLinksSection `json:"external-links"`
	Thumbnails    ThumbnailsMeta
}

type YearMeta struct {
	Sections []string
}

type SectionMeta struct {
	Name        string
	Description string
	IsRanked    bool `json:"is-ranked"`
	IsOngoing   bool `json:"is-ongoing"`
	Entries     []string
}

func ReadYear(
	fs_directory string,
	data_path string,
	path_prefix string,
	key string) (*base.Year, error) {
	_, err_key := regexp.MatchString("^0-9+$", key)
	if err_key != nil {
		return nil, fmt.Errorf(
			"Year %s is not a valid integer key %s", path_prefix, key)
	}
	year, err_conv := strconv.Atoi(key)
	if err_conv != nil {
		// We are definitely not reading a year directory.
		return nil, nil
	}
	// Check if we are in a valid year range. Some other number
	// indicates some random test directory.
	if year < 1992 || year > 9999 {
		return nil, nil
	}

	data, err_meta := ReadMetaBytes(fs_directory)
	if err_meta != nil {
		return nil, err_meta
	}
	if data == nil {
		return nil, nil
	}

	var meta YearMeta
	err_unmarshal := json.Unmarshal(data, &meta)
	if err_unmarshal != nil {
		return nil, err_unmarshal
	}
	var sections []*base.Section
	for _, section_key := range meta.Sections {
		section_fs_directory := filepath.Join(fs_directory, section_key)
		section_data_path := fmt.Sprintf(
			"%s/%s", data_path, section_key)
		section_path_prefix := fmt.Sprintf(
			"%s/%s", path_prefix, section_key)
		section, err_section := ReadSection(
			section_fs_directory,
			section_data_path,
			section_path_prefix,
			section_key)
		if err_section != nil {
			return nil, err_section
		}
		sections = append(sections, section)
	}
	result := base.Year{
		Key:      key,
		Path:     path_prefix,
		Year:     year,
		Name:     key,
		Sections: sections,
	}
	return &result, nil
}

func ReadSection(
	fs_directory string,
	data_path string,
	path_prefix string,
	key string) (*base.Section, error) {
	_, err_key := regexp.MatchString("^[a-z]([a-z0-9]+-)*[a-z0-9]+$", key)
	if err_key != nil {
		return nil, fmt.Errorf(
			"Section for %s has invalid key %s", path_prefix, key)
	}

	data, err_meta := ReadMetaBytes(fs_directory)
	if err_meta != nil {
		return nil, err_meta
	}
	if data == nil {
		return nil, nil
	}
	var meta SectionMeta
	err_unmarshal := json.Unmarshal(data, &meta)
	if err_unmarshal != nil {
		return nil, err_unmarshal
	}
	var entries []*base.Entry
	for _, entry_key := range meta.Entries {
		entry_fs_directory := filepath.Join(fs_directory, entry_key)
		entry_data_path := fmt.Sprintf("%s/%s", data_path, entry_key)
		entry_path_prefix := fmt.Sprintf("%s/%s", path_prefix, entry_key)
		entry, err_entry := ReadEntry(
			entry_fs_directory, entry_data_path, entry_path_prefix, entry_key)
		if err_entry != nil {
			return nil, err_entry
		}
		entries = append(entries, entry)
	}
	result := base.Section{
		Key:         key,
		Path:        path_prefix,
		Name:        meta.Name,
		Description: meta.Description,
		IsRanked:    meta.IsRanked,
		IsOngoing:   meta.IsOngoing,
		Entries:     entries,
	}
	return &result, nil
}

func get_entry_image(
	directory string,
	fs_directory string,
	meta ImageInfoMeta) base.ImageInfo {
	image_path := meta.Filename
	if directory != "" {
		image_path = path.Clean(fmt.Sprintf("%s/%s", directory, meta.Filename))
	}
	fs_path := meta.Filename
	if fs_directory != "" {
		fs_path = path.Clean(fmt.Sprintf("%s/%s", fs_directory, meta.Filename))
	}
	result := base.ImageInfo{
		Path:     image_path,
		FsPath:   fs_path,
		Checksum: meta.Checksum,
		Size:     meta.Size,
		Type:     meta.Type,
	}
	return result
}

func ReadEntry(
	fs_directory string,
	data_path string,
	path_prefix string,
	key string) (*base.Entry, error) {
	_, err_key := regexp.MatchString("^[a-z]([a-z0-9]+-)*[a-z0-9]+$", key)
	if err_key != nil {
		return nil, fmt.Errorf(
			"Entry for %s has invalid key %s", path_prefix, key)
	}

	data, err_meta := ReadMetaBytes(fs_directory)
	if err_meta != nil {
		return nil, fmt.Errorf("%s: %v", key, err_meta)
	}
	if data == nil {
		return nil, nil
	}
	var meta EntryMeta
	err_unmarshal := json.Unmarshal(data, &meta)
	if err_unmarshal != nil {
		return nil, fmt.Errorf("%s: %v", key, err_unmarshal)
	}
	if err := validate_image_info_meta(meta.Thumbnails.Default); err != nil {
		return nil, fmt.Errorf("%s: %v", key, err)
	}
	image_sources := make([]base.ImageInfo, len(meta.Thumbnails.Sources))
	for index, image := range meta.Thumbnails.Sources {
		if err := validate_image_info_meta(image); err != nil {
			return nil, fmt.Errorf("Source image error %s: %v", key, err)
		}
		image_sources[index] = get_entry_image(
			data_path, fs_directory, image)
	}
	result := base.Entry{
		Key:         key,
		Path:        path_prefix,
		Title:       meta.Title,
		Author:      meta.Author,
		Description: meta.Description,
		Asset: base.Asset{
			Type: meta.Asset.Type,
			Data: meta.Asset.Data,
		},
		Thumbnails: base.Thumbnails{
			Default: get_entry_image(
				data_path, fs_directory, meta.Thumbnails.Default),
			Sources: image_sources,
		},
		ExternalLinks: meta.ExternalLinks,
	}
	// Adjust the incomplete path:
	if result.Asset.Type == "image" {
		asset_data := result.Asset.Data.(ImageAsset)
		asset_data.Default.Path = fmt.Sprintf(
			"%s/%s", data_path, asset_data.Default.Path)
		asset_data.Default.FsPath = fmt.Sprintf(
			"%s/%s", fs_directory, asset_data.Default.Path)
		result.Asset.Data = asset_data
		for index, _ := range asset_data.Sources {
			asset_data.Sources[index].Path = fmt.Sprintf(
				"%s/%s", data_path, asset_data.Sources[index].Path)
			asset_data.Sources[index].FsPath = fmt.Sprintf(
				"%s/%s", fs_directory, asset_data.Sources[index].Path)
		}
	}
	return &result, nil
}

func New(fs_directory string, site_root string) (*SiteState, error) {
	infos, err_dir := ioutil.ReadDir(fs_directory)
	if err_dir != nil {
		return nil, err_dir
	}
	var year_candidates []string
	for _, info := range infos {
		if !info.IsDir() {
			continue
		}
		if _, err := regexp.MatchString("^0-9+$", info.Name()); err != nil {
			continue
		}
		year_candidates = append(year_candidates, info.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(year_candidates)))
	var years []*base.Year
	for _, year_candidate := range year_candidates {
		year_dir := filepath.Join(fs_directory, year_candidate)
		year_data := fmt.Sprintf("%s/_data/%s", site_root, year_candidate)
		year_prefix := fmt.Sprintf("%s/%s", site_root, year_candidate)
		year, err := ReadYear(year_dir, year_data, year_prefix, year_candidate)
		if err != nil {
			return nil, err
		}
		if year == nil {
			continue
		}
		years = append(years, year)
	}
	state := SiteState{
		SiteRoot: site_root,
		DataDir:  fs_directory,
		Years:    years,
	}
	return &state, nil
}

var StateInstance SiteState
