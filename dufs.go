package gohtvfs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type PathType string

const (
	//PathTypeFile PathType = "File"

	PathTypeDir PathType = "Dir"
)

type DufsJSONIndex struct {
	Href         string         `json:"href"`
	Kind         string         `json:"kind"`
	UriPrefix    string         `json:"uri_prefix"`
	AllowUpload  bool           `json:"allow_upload"`
	AllowDelete  bool           `json:"allow_delete"`
	AllowSearch  bool           `json:"allow_search"`
	AllowArchive bool           `json:"allow_archive"`
	DirExists    bool           `json:"dir_exists"`
	Auth         bool           `json:"auth"`
	User         string         `json:"user"`
	Paths        []DufsJSONFile `json:"paths"`
}

type DufsJSONFile struct {
	PathType PathType `json:"path_type"`
	Name     string   `json:"name"`
	MTime    int64    `json:"mtime"`
	Size     int64    `json:"size"`
}

type DufsVFS struct {
	*HttpVFS
}

func NewDufsVFS(root string) (*DufsVFS, error) {
	root = strings.Trim(root, "/")

	base, err := NewHttpVFS(root, "[dufs]")
	if err != nil {
		return nil, err
	}

	dufs := &DufsVFS{
		HttpVFS: base,
	}

	base.OpenFunc = func(name string) (fs.File, error) {
		href, err := dufs.appendToRoot(name)
		if err != nil {
			return nil, err
		}

		return NewDufsFile(
			dufs,
			name,
			*href,
		), nil
	}

	return dufs, nil
}

func (d *DufsVFS) appendToRoot(name string) (*URL, error) {
	u, err := url.Parse(d.Root)
	if err != nil {
		return nil, err
	}

	var segments []string
	for _, s := range strings.Split(name, "/") {
		if s != "" {
			segments = append(segments, s)
		}
	}

	u.Path = strings.Trim(u.Path, "/") + "/" + strings.Join(segments, "/")

	if strings.HasPrefix(name, "/") && !strings.HasSuffix(u.Path, "/") {
		u.Path += "/"
	}

	return &URL{
		URL: u,
	}, nil
}

func (d *DufsVFS) copyOrRename(dst, src string, isRenaming bool) error {
	httpMethod := "COPY"
	if isRenaming {
		httpMethod = "MOVE"
	}

	srcHref, err := d.appendToRoot(src)
	if err != nil {
		return err
	}

	dstHref, err := d.appendToRoot(dst)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(httpMethod, srcHref.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Add("Destination", dstHref.String())

	resp, err := d.GetHttpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			return fs.ErrNotExist
		}
		return errors.New(resp.Status)
	}

	return nil
}

func (d *DufsVFS) Mkdir(name string, _ fs.FileMode) error {
	dir, err := d.appendToRoot(name)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("MKCOL", dir.String(), nil)
	if err != nil {
		return err
	}

	resp, err := d.GetHttpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusMethodNotAllowed {
			return fs.ErrExist
		}
		return errors.New(resp.Status)
	}

	return nil
}

func (d *DufsVFS) Remove(name string) error {
	file, err := d.appendToRoot(name)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodDelete, file.String(), nil)
	if err != nil {
		return err
	}

	resp, err := d.GetHttpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if resp.StatusCode == http.StatusNotFound {
			return fs.ErrNotExist
		}
		return errors.New(resp.Status)
	}

	return nil
}

func (d *DufsVFS) Rename(oldname, newname string) error {
	return d.copyOrRename(newname, oldname, true)
}

func (d *DufsVFS) Copy(dst, src string) error {
	return d.copyOrRename(dst, src, false)
}

func NewDufsFile(fs *DufsVFS, name string, Href URL) *DufsFile {
	return &DufsFile{
		FS:                fs,
		Name:              name,
		Href:              Href,
		indexLocker:       &sync.Mutex{},
		cachedStateLocker: &sync.Mutex{},
	}
}

type DufsFile struct {
	File
	io.Seeker
	io.ReaderAt
	io.Writer
	io.WriterTo
	io.WriterAt

	index       int64
	cachedState fs.FileInfo

	indexLocker       sync.Locker
	cachedStateLocker sync.Locker

	FS   VFS
	Name string
	Href URL
}

func (d *DufsFile) determineIsDir(resp *http.Response) bool {
	return resp.Header.Get("Content-Disposition") == "" &&
		resp.Header.Get("Content-Type") == "application/json" &&
		resp.Header.Get("Cache-Control") == "no-cache"
}

func (d *DufsFile) jsonize() (*URL, error) {
	href, err := d.Href.Clone()
	if err != nil {
		return nil, err
	}
	query := href.Query()
	query.Add("json", "")
	href.RawQuery = query.Encode()
	return href, nil
}

func (d *DufsFile) json(method string, headers http.Header) (*http.Response, error) {
	href, err := d.jsonize()
	if err != nil {
		return nil, err
	}

	link := href.String()

	req, err := http.NewRequest(method, link, nil)
	if err != nil {
		return nil, err
	}

	req.Header = headers

	resp, err := d.FS.GetHttpClient().Do(req)
	if err != nil {
		d.FS.GetLogger().Println("Get file", link, "with error:", err)
		return nil, err
	}

	d.FS.GetLogger().Println("Get file", link, "with status code:", resp.StatusCode)
	if resp.StatusCode == http.StatusNotFound {
		return nil, fs.ErrNotExist
	} else if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fs.ErrInvalid
	}

	return resp, nil
}

func (d *DufsFile) get(headers http.Header) (*http.Response, error) {
	return d.json(http.MethodGet, headers)
}

func (d *DufsFile) head() (*http.Response, error) {
	resp, err := d.json(http.MethodHead, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	return resp, nil
}

func (d *DufsFile) Close() error {
	return nil
}

// Read
// Inefficient with short p: use WriteTo or io.Copy instead
func (d *DufsFile) Read(p []byte) (int, error) {
	d.indexLocker.Lock()
	defer d.indexLocker.Unlock()

	end := d.index + int64(len(p)) - 1

	stat, err := d.CachedStat()
	if err != nil {
		return 0, err
	}

	if end >= stat.Size() {
		end = stat.Size() - 1
	}

	if d.index >= end {
		return 0, io.EOF
	}

	header := http.Header{}
	header.Set("Range", fmt.Sprintf("bytes=%d-%d", d.index, end))

	resp, err := d.get(header)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if d.determineIsDir(resp) {
		return 0, fs.ErrInvalid
	}

	d.index = end + 1

	buf := bytes.NewBuffer(nil)
	n, err := io.CopyN(buf, resp.Body, resp.ContentLength)

	copy(p, buf.Bytes())

	return int(n), err
}

func (d *DufsFile) ReadAt(p []byte, off int64) (int, error) {
	_, err := d.Seek(off, io.SeekStart)
	if err != nil {
		return 0, err
	}
	return d.Read(p)
}

func (d *DufsFile) ReadFrom(reader io.Reader) (int64, error) {
	href := d.Href.String()
	contentLength := int64(0)
	req, err := http.NewRequest(http.MethodPut, href, NewSumReader(reader, &contentLength))
	if err != nil {
		return 0, err
	}

	resp, err := d.FS.GetHttpClient().Do(req)
	if err != nil {
		d.FS.GetLogger().Println("Put file error:", err)
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	d.FS.GetLogger().Println("Put file", href, "with ReadFrom result in status code:", resp.StatusCode)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, errors.New(resp.Status)
	}

	d.cachedState = nil

	return contentLength, nil
}

func (d *DufsFile) ReadDir(n int) ([]fs.DirEntry, error) {
	resp, err := d.get(nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if !d.determineIsDir(resp) {
		return nil, fs.ErrInvalid
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var root DufsJSONIndex
	err = json.Unmarshal(data, &root)
	if err != nil {
		return nil, err
	}

	var entries []fs.DirEntry
	for _, file := range root.Paths {
		entries = append(entries, &HttpDirEntry{
			info: &HttpFileInfo{
				name:  file.Name,
				size:  file.Size,
				mode:  fs.ModePerm,
				mtime: time.UnixMilli(file.MTime),
				isDir: file.PathType == PathTypeDir,
			},
		})
		if n > 0 && len(entries) >= n {
			break
		}
	}

	return entries, nil
}

func (d *DufsFile) Stat() (fs.FileInfo, error) {
	resp, err := d.head()
	if err != nil {
		return nil, err
	}

	lastModified := resp.Header.Get("Last-Modified")
	if lastModified == "" {
		lastModified = resp.Header.Get("Date")
	}

	var mtime time.Time

	if lastModified != "" {
		mtime, err = time.Parse(time.RFC1123, lastModified)
		if err != nil {
			return nil, err
		}
	}

	isDir := d.determineIsDir(resp)
	size := int64(0)

	if !isDir {
		size = resp.ContentLength
	}

	stat := &HttpFileInfo{
		name:  d.Name,
		size:  size,
		mode:  fs.ModePerm,
		mtime: mtime,
		isDir: isDir,
	}

	d.cachedState = stat

	return stat, nil
}

func (d *DufsFile) CachedStat() (fs.FileInfo, error) {
	d.cachedStateLocker.Lock()
	defer d.cachedStateLocker.Unlock()

	if d.cachedState != nil {
		return d.cachedState, nil
	}

	return d.Stat()
}

func (d *DufsFile) WriteTo(writer io.Writer) (int64, error) {
	resp, err := d.get(http.Header{})
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	return io.Copy(writer, resp.Body)
}

// Write
// Inefficient with short p: use WriteTo instead
func (d *DufsFile) Write(p []byte) (n int, err error) {
	return d.WriteAt(p, d.index)
}

func (d *DufsFile) WriteAt(p []byte, offset int64) (n int, err error) {
	d.indexLocker.Lock()
	defer d.indexLocker.Unlock()

	stat, err := d.CachedStat()
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) && offset == 0 {
			reader := bytes.NewReader(p)
			n, err := d.ReadFrom(reader)
			d.index = n
			return int(n), err
		}
		return 0, err
	}

	href := d.Href.String()
	req, err := http.NewRequest(http.MethodPatch, href, bytes.NewReader(p))
	if err != nil {
		return 0, err
	}

	end := offset + int64(len(p)) - 1
	if offset >= stat.Size() {
		req.Header.Add("x-update-range", "append")
	} else {
		req.Header.Add("x-update-range", fmt.Sprintf("bytes=%d-%d", d.index, end))
	}

	resp, err := d.FS.GetHttpClient().Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	d.FS.GetLogger().Println("Patch file", href, "with WriteAt result in status code:", resp.StatusCode, req.Header.Get("x-update-range"))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return 0, errors.New(resp.Status)
	}

	d.index = end + 1
	d.cachedState = nil

	return len(p), nil
}

func (d *DufsFile) Seek(offset int64, whence int) (int64, error) {
	stat, err := d.CachedStat()
	if err != nil {
		return 0, err
	}

	d.indexLocker.Lock()
	defer d.indexLocker.Unlock()

	switch whence {
	case io.SeekStart:
		d.index = offset
	case io.SeekCurrent:
		d.index += offset
	case io.SeekEnd:
		d.index = stat.Size() + offset
	}

	if d.index < 0 {
		return 0, errors.New("dufs: negative offset")
	} else if d.index > stat.Size() {
		return 0, errors.New("dufs: offset out of range")
	}

	return d.index, nil
}

func (d *DufsFile) String() string {
	return d.Href.String()
}
