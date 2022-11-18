package main

import (
	"bytes"
	"embed"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"html/template"
	"image"
	_ "image/jpeg"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/zeozeozeo/imgsearch/search"
)

//go:embed static/*
var staticFs embed.FS

//go:embed templates/*
var templatesFs embed.FS

func main() {
	// parse flags
	dbPath := flag.String("path", "database.txt", "path to database file")
	port := flag.Uint("port", 8080, "port to start Gin on")
	isReleaseMode := flag.Bool("release", true, "specifies whether Gin should run in release mode")
	doServeImages := flag.Bool("serve-images", false, "specifies whether the images/ folder should be served")
	doEmbedResources := flag.Bool(
		"embed",
		true,
		"specifies whether templates and CSS should be embedded into the executable",
	)
	flag.Parse()

	log.Println("run with -h or -help to show help")
	log.Printf("loading database \"%s\"", *dbPath)

	db, err := loadDatabase(*dbPath)
	if err != nil {
		panic(err)
	}

	// setup Gin routes
	if *isReleaseMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.Default()

	// load templates
	if *doEmbedResources {
		templ := template.Must(template.New("").ParseFS(templatesFs, "templates/*.html"))
		r.SetHTMLTemplate(templ)
		r.StaticFileFS("/static/style.css", "static/style.css", http.FS(staticFs))
	} else {
		r.LoadHTMLGlob("templates/*")
		r.Static("/static", "./static")
	}

	r.MaxMultipartMemory = 8 << 20 // 8 MiB
	if *doServeImages {
		log.Println("serving the ./images folder")
		r.Static("/images", "./images")
	}

	r.GET("/", func(c *gin.Context) {
		serveLanding(c, db)
	})

	r.POST("/search", func(c *gin.Context) {
		serveSearch(c, db)
	})

	// start Gin
	log.Printf("starting on port %d (use -port to change it)", *port)
	r.Run(fmt.Sprintf(":%d", *port))
}

func serveLanding(c *gin.Context, db *search.Database) {
	c.HTML(http.StatusOK, "index.html", gin.H{
		"indexedImages": humanize.Comma(int64(len(db.Images))),
	})
}

func serveSearch(c *gin.Context, db *search.Database) {
	ip := c.ClientIP()
	url := c.PostForm("url")
	urlValid := isUrlValid(url)
	var img image.Image
	var b64img string
	var imgFormat string

	file, err := c.FormFile("file")
	if err != nil && !urlValid {
		log.Printf("[%s] url is invalid and no file was provided, redirecting to \"/\"", ip)
		c.Redirect(http.StatusFound, "/")
		return
	}

	// decode image from file or URL
	if file != nil {
		filename := filepath.Base(file.Filename)
		log.Printf("[%s] got file \"%s\"", ip, filename)
		img, b64img, imgFormat, err = decodeImageFromFormFile(file)
		if err != nil {
			c.String(
				http.StatusInternalServerError,
				"failed to decode image from file \"%s\": %s",
				filename,
				err,
			)
			return
		}
	} else if urlValid {
		log.Printf("[%s] requesting URL \"%s\"", ip, url)
		img, b64img, imgFormat, err = decodeImageFromURL(url)
		if err != nil {
			c.String(
				http.StatusInternalServerError,
				"failed to decode image from url \"%s\": %s",
				url,
				err,
			)
			return
		}
	}

	serveSearchResults(c, img, b64img, imgFormat, db)
}

func serveSearchResults(c *gin.Context, img image.Image, b64img, format string, db *search.Database) {
	ip := c.ClientIP()
	log.Printf("[%s] searching", ip)

	start := time.Now()
	results := db.Search(img)
	urls := search.GetURLStrings(results)

	elapsed := time.Since(start)
	log.Printf("[%s] found %d results in %s", ip, len(results), elapsed)

	elapsedMs := elapsed.Seconds() * 1000
	c.HTML(http.StatusOK, "search.html", gin.H{
		"resultsAmount": humanize.Comma(int64(len(results))),
		"elapsedMs":     roundFloat(elapsedMs, 3), // 3 decimal points
		"results":       urls,
		"base64img":     b64img,
		"previewFormat": format,
	})
}

func decodeImageFromFormFile(file *multipart.FileHeader) (image.Image, string, string, error) {
	src, err := file.Open()
	if err != nil {
		return nil, "", "", err
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return nil, "", "", err
	}
	img, format, err := image.Decode(bytes.NewBuffer(data))
	if err != nil {
		return nil, "", format, err
	}

	return img, base64.StdEncoding.EncodeToString(data), format, nil
}

func decodeImageFromURL(url string) (image.Image, string, string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", "", fmt.Errorf("got response code %d from \"%s\" (expected 200)", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}
	img, format, err := image.Decode(bytes.NewBuffer(data))
	if err != nil {
		return nil, "", format, err
	}

	return img, base64.StdEncoding.EncodeToString(data), format, nil
}

func loadDatabase(path string) (*search.Database, error) {
	file, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			log.Printf("\"%s\" not found, indexing images/\n", path)

			err = search.IndexImages("images")
			return nil, err
		} else {
			panic(err)
		}
	}
	defer file.Close()

	// load the database
	start := time.Now()
	db := search.LoadDatabase(file)
	log.Printf("loaded database in %s (%d images)", time.Since(start), len(db.Images))
	file.Close()
	return db, nil
}

func isUrlValid(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}

func roundFloat(val float64, precision uint) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}
