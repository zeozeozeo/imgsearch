package search

import (
	"bufio"
	"image"
	"io"
	"log"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/zeozeozeo/imagesim"
)

type Database struct {
	Images []Image
}

type Image struct {
	URL  string
	Hash uint64
}

type SearchImage struct {
	URL        string
	Difference float64
}

func LoadDatabase(reader io.Reader) *Database {
	r := bufio.NewReader(reader)
	database := &Database{}
	lineIdx := 0

	for {
		lineBytes, _, err := r.ReadLine()
		if err != nil {
			break
		}

		lineIdx++
		split := strings.Split(string(lineBytes), " ")
		if len(split) != 2 {
			log.Printf("line %d is invalid, expected 2 elements", lineIdx)
			continue
		}

		hash, err := strconv.ParseUint(split[1], 10, 64)
		if err != nil {
			log.Printf("failed to parse hash \"%s\" on line %d: %s", split[1], lineIdx, err)
			continue
		}

		image := Image{
			URL:  split[0],
			Hash: hash,
		}

		database.Images = append(database.Images, image)
	}

	return database
}

func (db *Database) Search(img image.Image) []SearchImage {
	results := []SearchImage{}
	hash := imagesim.Hash(img)
	bestDiff := math.MaxFloat64

	for _, dbImg := range db.Images {
		diff := imagesim.CompareHashes(hash, dbImg.Hash)

		if diff < bestDiff {
			bestDiff = diff
		} else if diff > bestDiff*1.5 {
			// discard accept images that are too far from the best score
			continue
		}

		results = append(results, SearchImage{
			URL:        dbImg.URL,
			Difference: diff,
		})
	}

	// TODO: insert values at the sorted indexes instead of sorting the entire array
	// TODO: add a limit parameter to this function
	sort.Slice(results, func(i, j int) bool {
		return results[i].Difference < results[j].Difference
	})

	return results
}

func GetURLStrings(imgs []SearchImage) []string {
	urls := []string{}
	for _, img := range imgs {
		urls = append(urls, img.URL)
	}
	return urls
}
