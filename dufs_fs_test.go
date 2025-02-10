package vfs

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func TestDufsRemove(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	filename := fmt.Sprintf("test-remove-%d.bin", time.Now().UnixNano())

	file, err := dufs.Open(filename)
	if err != nil {
		t.Fatal(err)
	}

	dufsFile := file.(*DufsFile)
	_, err = dufsFile.Write([]byte("test"))
	if err != nil {
		t.Fatal(err)
	}

	err = dufsFile.Close()
	if err != nil {
		t.Fatal(err)
	}

	stat, err := dufs.Stat(filename)
	if err != nil {
		t.Fatal(err)
	} else if stat.Size() == 0 {
		t.Fatal("file should not be empty")
	}

	localStat, err := os.Stat(path.Join(TestDataFolder, filename))
	if err != nil {
		t.Fatal(err)
	} else if localStat.Size() == 0 {
		t.Fatal("file should not be empty")
	}

	err = dufs.Remove(filename)
	if err != nil {
		t.Fatal(err)
	}

	_, err = os.Stat(path.Join(TestDataFolder, filename))
	if err == nil {
		t.Fatal("file should be removed from local")
	}
}

func TestDufsCopy(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	hash, filename, _, err := CreateTestData()
	if err != nil {
		t.Fatal(err)
	}

	dst := "copy-" + string(filename)

	err = dufs.Copy(dst, string(filename))
	if err != nil {
		t.Fatal(err)
	}

	buf, err := os.ReadFile(path.Join(TestDataFolder, dst))
	if err != nil {
		t.Fatal(err)
	}

	localHash, err := Sha256(buf)
	if err != nil {
		t.Fatal(err)
	}

	if hash != localHash {
		t.Fatal("hash mismatch")
	}
}

func TestDufsString(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	root, err := dufs.Open("/")
	if err != nil {
		t.Fatal(err)
	}

	dufsFile := root.(*DufsFile)

	if !strings.HasSuffix(dufsFile.String(), "/") {
		t.Fatalf("root should end with /, but got %s", dufsFile.String())
	}
}

func TestDufsMkdir(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	dir := fmt.Sprintf("test-dir-utf8-中文/%d/", time.Now().UnixNano())
	err = dufs.Mkdir(dir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	stat, err := dufs.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}

	if !stat.IsDir() {
		t.Fatal(dir, "should be a directory")
	}

	// same name dir
	err = dufs.Mkdir(dir, 0755)
	t.Log(err)
	if err == nil {
		t.Fatalf("should not create same name dir")
	}
}

func TestDufsOnline(t *testing.T) {
	dufs, err := NewDufsVFS(DufsAddr + "1") // fake address, port 80801 should not be taken
	if err != nil {
		t.Fatal(err)
	}

	online, _ := dufs.Online(nil)
	if online {
		t.Fatal("dufs should be offline")
	}

	CheckDufsServer()

	dufs, err = NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}
	online, _ = dufs.Online(nil)
	if !online {
		t.Fatal("dufs should be online")
	}
}
