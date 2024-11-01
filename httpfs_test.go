package vfs

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
	"testing"
	"time"
)

const (
	TestDataFolder = "testdata"
	DufsHost       = "127.0.0.1"
	DufsPort       = "8080"
)

//goland:noinspection HttpUrlsUsage
var DufsAddr = "http://" + DufsHost + ":" + DufsPort

type (
	HashString string
	FileName   string
)

func Sha256(data []byte) (HashString, error) {
	hasher := sha256.New()
	_, err := hasher.Write(data)
	if err != nil {
		return "", err
	}
	return HashString(hex.EncodeToString(hasher.Sum(nil))), nil
}

func CreateTestData() (HashString, FileName, []byte, error) {
	// random bytes
	data := make([]byte, 1024+rand.Intn(4096))
	_, err := crand.Read(data)
	if err != nil {
		return "", "", data, err
	}
	hash, err := Sha256(data)
	if err != nil {
		return "", "", data, err
	}

	file, err := os.CreateTemp(TestDataFolder, string(hash)+"-*.bin")
	defer func() {
		_ = file.Close()
	}()

	_, err = io.Copy(file, bytes.NewReader(data))
	if err != nil {
		return hash, FileName(file.Name()), data, err
	}

	return hash, FileName(path.Base(file.Name())), data, nil
}

func CheckDufsServer() {
	log.Println("Run dufs with:")
	for {
		log.Println("dufs -A", "--bind", DufsHost, "--port", DufsPort, TestDataFolder)
		//goland:noinspection HttpUrlsUsage
		err := exec.Command("curl", DufsAddr).Run()
		if err == nil {
			log.Println("dufs server is running")
			break
		}
		time.Sleep(1 * time.Second)
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

func TestDufsRead(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	hash, filename, _, err := CreateTestData()
	if err != nil {
		t.Fatal(err)
	}

	log.Println("Test file:", filename, ",hash:", hash)

	file, err := dufs.Open(string(filename))
	if err != nil {
		t.Fatal(err)
	}

	stat, err := file.Stat()
	if err != nil {
		t.Fatal(err)
	}

	if stat.Name() != string(filename) {
		t.Fatal("file name should be", filename)
	}

	if stat.IsDir() {
		t.Fatal(filename, "should not be a directory")
	}

	var buf []byte
	writer := bytes.NewBuffer(buf)
	_, err = io.Copy(writer, file)
	if err != nil {
		t.Fatal(err)
	}

	remoteHash, err := Sha256(writer.Bytes())
	if err != nil {
		t.Fatal(err)
	}

	if hash != remoteHash {
		t.Fatal("hash mismatch")
	}
}

func TestDufsWrite(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	hash, filename, data, err := CreateTestData()
	if err != nil {
		t.Fatal(err)
	}

	file, err := dufs.Open(fmt.Sprintf("test-%d.bin", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}

	if f, ok := file.(io.ReaderFrom); ok {
		n, err := f.ReadFrom(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		} else if n != int64(len(data)) {
			t.Fatalf("file size mismatch, expected %d, got %d", len(data), n)
		}
	} else {
		t.Fatal("file should be io.ReaderFrom")
	}

	buf, err := os.ReadFile(path.Join(TestDataFolder, string(filename)))
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
}
