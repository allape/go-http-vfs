package gohtvfs

import (
	"bytes"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path"
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
	err := os.MkdirAll(TestDataFolder, 0755)
	if err != nil {
		return "", "", nil, err
	}

	// random bytes
	data := make([]byte, 1024*1024*(10+rand.Intn(90)))
	_, err = crand.Read(data)
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
