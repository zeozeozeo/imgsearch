// Indexes a specified amount of images from the LAION-5B dataset:
// https://www.kaggle.com/datasets/vitaliykinakh/guie-laion5b-dataset
package main

import (
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"time"

	"github.com/zeozeozeo/imagesim"
)

const (
	MAX_IMAGES     = 30000
	MAX_GOROUTINES = 64
	JSON_PATH      = "GUIE_laion5b_dataset_en.json"
)

var (
	isWriting         bool
	runningGoroutines int
)

type LaionImage struct {
	URL string `json:"url"`
}

func main() {
	fmt.Println("reading data...")
	data, err := os.ReadFile(JSON_PATH)
	if err != nil {
		panic(err)
	}

	fmt.Println("loading json...")
	images := []LaionImage{}

	err = json.Unmarshal(data, &images)
	if err != nil {
		panic(err)
	}
	imagesLen := len(images)
	fmt.Printf("loaded %d images\n", imagesLen)

	// open database.txt for writing
	writeFile, err := os.Create("database.txt")
	if err != nil {
		panic(err)
	}
	defer writeFile.Close()

	// iterate through all images and skip images to output MAX_IMAGES
	for i := 0; i < MAX_IMAGES; i++ {
		imageIdx := int(
			math.Floor(
				float64(i) *
					(float64(imagesLen) +
						math.Floor(float64(imagesLen)/float64(MAX_IMAGES))) /
					float64(MAX_IMAGES),
			),
		)
		if imageIdx > imagesLen-1 {
			break
		}
		imageUrl := images[imageIdx].URL

		for runningGoroutines >= MAX_GOROUTINES {
			time.Sleep(1 * time.Microsecond)
		}

		go indexImage(imageUrl, writeFile, i)
	}

	for runningGoroutines > 0 {
		time.Sleep(1 * time.Microsecond)
	}
}

// returns if the image was indexed or not
func indexImage(url string, file io.Writer, idx int) {
	runningGoroutines++
	defer func() {
		runningGoroutines--
	}()

	// download image
	resp, err := http.Get(url)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return
	}

	// decode image
	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return
	}

	// calculate the image hash and write it to the database
	hash := imagesim.Hash(img)

	// wait until file is ready to write
	for isWriting {
		<-time.After(1 * time.Microsecond) // don't hog the cpu
	}

	isWriting = true
	file.Write([]byte(
		fmt.Sprintf("%s %d\n", url, hash),
	))
	isWriting = false

	fmt.Printf("[%d/%d] indexed image \"%s\"\n", idx+1, MAX_IMAGES, url)
}
