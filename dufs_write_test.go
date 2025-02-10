package gohtvfs

import (
	"bytes"
	crand "crypto/rand"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path"
	"testing"
	"time"
)

func TestDufsReadFrom(t *testing.T) {
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

func TestDufsWrite(t *testing.T) {
	CheckDufsServer()
	dufs, err := NewDufsVFS(DufsAddr)
	if err != nil {
		t.Fatal(err)
	}

	file, err := dufs.Open(fmt.Sprintf("test-write-%d.bin", time.Now().UnixNano()))
	if err != nil {
		t.Fatal(err)
	}

	dufsFile := file.(*DufsFile)

	randomBytes := make([]byte, rand.Intn(1024)+1024)

	var writtenData []byte

	for i := 0; i < 10; i++ {
		n, err := crand.Read(randomBytes)
		if err != nil {
			t.Fatal(err)
		} else if n != len(randomBytes) {
			t.Fatalf("randomBytes size mismatch, expected %d, got %d", len(randomBytes), n)
		}

		writtenData = append(writtenData, randomBytes...)

		n, err = dufsFile.Write(randomBytes)
		if err != nil {
			t.Fatal(err)
		} else if n != len(randomBytes) {
			t.Fatalf("write size mismatch, expected %d, got %d", len(randomBytes), n)
		}
	}

	localFile, err := os.Open(path.Join(TestDataFolder, dufsFile.Name))
	if err != nil {
		t.Fatal(err)
	}

	localBytes, err := io.ReadAll(localFile)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(writtenData, localBytes) != 0 {
		t.Fatal("written data mismatch")
	}

	err = localFile.Close()
	if err != nil {
		t.Fatal(err)
	}

	// replace last randomBytes with new randomBytes
	n, err := crand.Read(randomBytes)
	if err != nil {
		t.Fatal(err)
	} else if n != len(randomBytes) {
		t.Fatalf("randomBytes size mismatch, expected %d, got %d", len(randomBytes), n)
	}

	writtenData = writtenData[:len(writtenData)-len(randomBytes)]
	writtenData = append(writtenData, randomBytes...)

	_, err = dufsFile.Seek(int64(len(writtenData)-len(randomBytes)), io.SeekStart)
	if err != nil {
		t.Fatal(err)
	}

	n, err = dufsFile.WriteAt(randomBytes, int64(len(writtenData)-len(randomBytes)))
	if err != nil {
		t.Fatal(err)
	} else if n != len(randomBytes) {
		t.Fatalf("writeAt size mismatch, expected %d, got %d", len(randomBytes), n)
	}

	localFile, err = os.Open(path.Join(TestDataFolder, dufsFile.Name))
	if err != nil {
		t.Fatal(err)
	}

	localBytes, err = io.ReadAll(localFile)
	if err != nil {
		t.Fatal(err)
	}

	if bytes.Compare(writtenData, localBytes) != 0 {
		t.Fatal("written data mismatch")
	}

	err = localFile.Close()
	if err != nil {
		t.Fatal(err)
	}
	err = dufsFile.Close()
	if err != nil {
		t.Fatal(err)
	}
}
