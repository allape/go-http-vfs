# VFS over HTTP

FYI, not thread safe.

## Adapted Server

- [dufs](dufs.go): https://github.com/sigoden/dufs
    - GET
    - PUT
    - PATCH
    - MKCOL
    - DELETE
    - COPY
    - MOVE

## Usage

- [dufs_fs_test.go](dufs_fs_test.go)
- [dufs_read_test.go](dufs_read_test.go)
- [dufs_write_test.go](dufs_write_test.go)
- [httpfs_test.go](httpfs_test.go)
