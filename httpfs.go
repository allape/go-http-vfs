package vfs

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type VFS interface {
	fs.StatFS
	fs.ReadDirFS
	fs.ReadFileFS
	fs.StatFS
	SetHttpClient(client *http.Client)
	GetHttpClient() *http.Client
	SetLogger(logger *log.Logger)
	GetLogger() *log.Logger
}

type File interface {
	fs.ReadDirFile
	io.ReaderFrom
}

type FileInfo interface {
	fs.FileInfo
}

type DirEntry interface {
	fs.DirEntry
}

type HttpFileInfo struct {
	FileInfo
	name  string
	size  int64
	mode  fs.FileMode
	mtime time.Time
	isDir bool
}

func (d *HttpFileInfo) Name() string {
	return d.name
}

func (d *HttpFileInfo) Size() int64 {
	return d.size
}

func (d *HttpFileInfo) Mode() fs.FileMode {
	return d.mode
}

func (d *HttpFileInfo) ModTime() time.Time {
	return d.mtime
}

func (d *HttpFileInfo) IsDir() bool {
	return d.isDir
}

func (d *HttpFileInfo) Sys() any {
	return nil
}

type HttpDirEntry struct {
	DirEntry
	info *HttpFileInfo
}

func (d *HttpDirEntry) Name() string {
	return d.info.Name()
}

func (d *HttpDirEntry) IsDir() bool {
	return d.info.IsDir()
}

func (d *HttpDirEntry) Type() fs.FileMode {
	return d.info.Mode()
}

func (d *HttpDirEntry) Info() (fs.FileInfo, error) {
	return d.info, nil
}

type OpenFunc func(name string) (fs.File, error)

type HttpVFS struct {
	VFS

	Root     string
	OpenFunc OpenFunc

	Logger     *log.Logger
	HttpClient *http.Client
}

func NewHttpVFS(root, tag string) (*HttpVFS, error) {
	root = strings.Trim(root, "/")
	return &HttpVFS{
		Root: root,

		Logger:     log.New(os.Stderr, tag+" ", log.LstdFlags),
		HttpClient: &http.Client{},
	}, nil
}

func (d *HttpVFS) SetHttpClient(client *http.Client) {
	d.HttpClient = client
}

func (d *HttpVFS) GetHttpClient() *http.Client {
	return d.HttpClient
}

func (d *HttpVFS) SetLogger(logger *log.Logger) {
	d.Logger = logger
}

func (d *HttpVFS) GetLogger() *log.Logger {
	if d.Logger == nil {
		return log.New(io.Discard, "", 0)
	}
	return d.Logger
}

func (d *HttpVFS) Open(name string) (fs.File, error) {
	if d.OpenFunc == nil {
		return nil, errors.New("func Open is not implemented")
	}
	return d.OpenFunc(name)
}

func (d *HttpVFS) ReadDir(name string) ([]fs.DirEntry, error) {
	file, err := d.OpenFunc(name)
	if err != nil {
		return nil, err
	}
	if f, ok := file.(fs.ReadDirFile); ok {
		return f.ReadDir(-1)
	}
	return nil, fs.ErrInvalid
}

func (d *HttpVFS) ReadFile(name string) ([]byte, error) {
	file, err := d.OpenFunc(name)
	if err != nil {
		return nil, err
	}

	var buf []byte
	writer := bytes.NewBuffer(buf)

	_, err = io.Copy(writer, file)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (d *HttpVFS) Stat(name string) (fs.FileInfo, error) {
	file, err := d.OpenFunc(name)
	if err != nil {
		return nil, err
	}

	return file.Stat()
}

type URL struct {
	*url.URL
}

func (d *URL) Clone() (*URL, error) {
	u, err := url.Parse(d.String())
	if err != nil {
		return nil, err
	}
	return &URL{u}, nil
}
