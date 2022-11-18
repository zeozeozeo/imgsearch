package search

import (
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"github.com/zeozeozeo/imagesim"
)

func IndexImages(root string) error {
	file, err := os.Create("database.txt")
	if err != nil {
		return err
	}
	defer file.Close()

	err = filepath.Walk(root, func(path string, info fs.FileInfo, err error) error {
		// skip directories
		if info.IsDir() {
			return nil
		}

		entry, processErr := processImage(path)
		if processErr != nil {
			return nil
		}
		file.WriteString(entry + "\n")

		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func processImage(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("failed to read \"%s\": %s", path, err)
		return "", err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		log.Printf("failed to decode \"%s\": %s", path, err)
		return "", err
	}

	hash := imagesim.Hash(img)
	log.Printf("indexed \"%s\" (format: %s, hash: %d)", path, format, hash)

	// url, hash
	entry := fmt.Sprintf("%s %d", filepath.Base(path), hash)
	return entry, nil
}
