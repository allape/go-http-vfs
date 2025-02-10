package vfs

import (
	"bytes"
	"io"
	"log"
	"math/rand"
	"testing"
)

func TestDufsReadDir(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := dufs.ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}

	foundGitIgnore := false
	for _, entry := range entries {
		if entry.Name() == ".gitignore" {
			foundGitIgnore = true
			break
		}
	}

	if !foundGitIgnore {
		t.Fatal("should find .gitignore")
	}
}

func TestDufsRead(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	hash, filename, data, err := CreateTestData()
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

	if file, ok := file.(*DufsFile); ok {
		for i := 0; i < 100+rand.Intn(100); i++ {
			t.Log("Random read test index", i)

			randomIndex := rand.Int63n(stat.Size())
			_, err = file.Seek(randomIndex, io.SeekStart)
			if err != nil {
				t.Fatal(err)
			}

			bs := make([]byte, 1+rand.Intn(int(stat.Size()/2)))

			t.Log("Read at", randomIndex, "for", len(bs))

			n, err := file.Read(bs)
			if err != nil {
				t.Fatal(err)
			}

			bs = bs[:n]

			if bytes.Compare(bs, data[randomIndex:randomIndex+int64(len(bs))]) != 0 {
				t.Fatal("read data mismatch")
			}
		}
	}
}
