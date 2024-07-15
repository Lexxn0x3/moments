package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/nfnt/resize"
	"github.com/patrickmn/go-cache"
	"github.com/rwcarlsen/goexif/exif"
)

const (
    dbUser     = "postgres"
    dbPassword = "Robin2002"
    dbName     = "photo_gallery"
    uploadDir  = "../uploads"
)

type Photo struct {
    Filename       string    `json:"filename"`
    Metadata       string    `json:"metadata"`
    DateTaken      time.Time `json:"date_taken"`
    PhotographerID int       `json:"photographer_id"`
    Event          string    `json:"event"`
}

var previewCache = cache.New(cache.NoExpiration, cache.NoExpiration)

func main() {
    dbInfo := "host=localhost port=5432 user=" + dbUser + " password=" + dbPassword + " dbname=" + dbName + " sslmode=disable"
    db, err := sql.Open("postgres", dbInfo)
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    if _, err := os.Stat(uploadDir); os.IsNotExist(err) {
        os.Mkdir(uploadDir, os.ModePerm)
    }

    router := gin.Default()

    
router.POST("/api/upload", func(c *gin.Context) {
        file, header, err := c.Request.FormFile("image")
        if err != nil {
            c.String(http.StatusBadRequest, "Bad request")
            return
        }
        defer file.Close()

        uuid := uuid.New().String()
        extension := filepath.Ext(header.Filename)
        filename := uuid + extension

        filePath := filepath.Join(uploadDir, filename)
        out, err := os.Create(filePath)
        if err != nil {
            c.String(http.StatusInternalServerError, "Unable to save the file")
            return
        }
        defer out.Close()

        _, err = out.ReadFrom(file)
        if err != nil {
            c.String(http.StatusInternalServerError, "Unable to save the file")
            return
        }

        // Extract EXIF data
        file.Seek(0, 0) // Reset file pointer
        exifData, err := exif.Decode(file)
        var dateTaken time.Time
        if err == nil {
            dateTaken, err = exifData.DateTime()
            if err != nil {
                dateTaken = time.Now()
            }
        } else {
            dateTaken = time.Now()
        }

        metadata := header.Filename

        _, err = db.Exec("INSERT INTO photos (filename, metadata, date_taken) VALUES ($1, $2, $3)", filename, metadata, dateTaken)
        if err != nil {
            c.String(http.StatusInternalServerError, "Failed to save metadata")
            return
        }

        c.String(http.StatusOK, "Upload successful")
    })
    router.GET("/api/photos", func(c *gin.Context) {
        rows, err := db.Query("SELECT filename, metadata, date_taken, photographer_id, event FROM photos ORDER BY date_taken DESC")
        if err != nil {
            log.Fatal(err)
        }
        defer rows.Close()

        var photos []Photo
        for rows.Next() {
            var photo Photo
            rows.Scan(&photo.Filename, &photo.Metadata, &photo.DateTaken, &photo.PhotographerID, &photo.Event)
            photos = append(photos, photo)
        }

        c.JSON(http.StatusOK, photos)
    })

    router.GET("/api/photo/:filename", func(c *gin.Context) {
        filename := c.Param("filename")
        filePath := filepath.Join(uploadDir, filename)

        file, err := os.Open(filePath)
        if err != nil {
            c.String(http.StatusNotFound, "File not found")
            return
        }
        defer file.Close()

        fileBytes, err := ioutil.ReadAll(file)
        if err != nil {
            c.String(http.StatusInternalServerError, "Error reading file")
            return
        }

        c.Data(http.StatusOK, "image/jpeg", fileBytes)
    })

    router.GET("/api/photos/person/:userId", func(c *gin.Context) {
        userId := c.Param("userId")

        rows, err := db.Query("SELECT p.filename, p.metadata, p.date_taken, p.photographer_id, p.event FROM photos p JOIN photo_people pp ON p.id = pp.photo_id WHERE pp.user_id = $1", userId)
        if err != nil {
            log.Fatal(err)
        }
        defer rows.Close()

        var photos []Photo
        for rows.Next() {
            var photo Photo
            rows.Scan(&photo.Filename, &photo.Metadata, &photo.DateTaken, &photo.PhotographerID, &photo.Event)
            photos = append(photos, photo)
        }

        c.JSON(http.StatusOK, photos)
    })

    router.GET("/api/photo/preview/:filename/:level", func(c *gin.Context) {
    filename := c.Param("filename")
    level := c.Param("level")

    var width uint
    switch level {
    case "1":
        width = 300
    case "2":
        width = 600
    default:
        c.String(http.StatusBadRequest, "Invalid level")
        return
    }

    // Check cache
    cacheKey := fmt.Sprintf("%s_%d", filename, width)
    if cached, found := previewCache.Get(cacheKey); found {
        c.Data(http.StatusOK, "image/jpeg", cached.([]byte))
        return
    }

    filePath := filepath.Join(uploadDir, filename)
    file, err := os.Open(filePath)
    if err != nil {
        c.String(http.StatusNotFound, "File not found")
        return
    }
    defer file.Close()

    img, err := jpeg.Decode(file)
    if err != nil {
        c.String(http.StatusInternalServerError, "Error decoding image")
        return
    }

    // Resize image
    resizedImg := resize.Resize(width, 0, img, resize.Lanczos3)

    // Encode resized image to buffer
    var buf bytes.Buffer
    err = jpeg.Encode(&buf, resizedImg, nil)
    if err != nil {
        c.String(http.StatusInternalServerError, "Error encoding image")
        return
    }

    // Cache the resized image
    previewCache.Set(cacheKey, buf.Bytes(), cache.NoExpiration)

    c.Data(http.StatusOK, "image/jpeg", buf.Bytes())
})
      
router.GET("/api/video/:filename", func(c *gin.Context) {
    filename := c.Param("filename")
    filePath := filepath.Join(uploadDir, filename)

    file, err := os.Open(filePath)
    if err != nil {
        c.String(http.StatusNotFound, "File not found")
        return
    }
    defer file.Close()

    fileInfo, err := file.Stat()
    if err != nil {
        c.String(http.StatusInternalServerError, "Error getting file info")
        return
    }

    fileSize := fileInfo.Size()
    rangeHeader := c.GetHeader("Range")
    if rangeHeader == "" {
        // Serve the entire file
        if strings.HasSuffix(filename, ".mov") {
            c.Header("Content-Type", "video/quicktime")
        } else {
            c.Header("Content-Type", "video/mp4")
        }
        c.Header("Content-Length", fmt.Sprintf("%d", fileSize))
        http.ServeFile(c.Writer, c.Request, filePath)
        return
    }

    // Parse the range header
    rangeParts := strings.Split(strings.TrimPrefix(rangeHeader, "bytes="), "-")
    start, err := strconv.ParseInt(rangeParts[0], 10, 64)
    if err != nil {
        c.String(http.StatusBadRequest, "Invalid range")
        return
    }

    var end int64
    if len(rangeParts) > 1 && rangeParts[1] != "" {
        end, err = strconv.ParseInt(rangeParts[1], 10, 64)
        if err != nil {
            c.String(http.StatusBadRequest, "Invalid range")
            return
        }
    } else {
        end = fileSize - 1
    }

    if start > end || start < 0 || end >= fileSize {
        c.String(http.StatusRequestedRangeNotSatisfiable, "Requested range not satisfiable")
        return
    }

    // Set headers for partial content response
    if strings.HasSuffix(filename, ".mov") {
        c.Header("Content-Type", "video/quicktime")
    } else {
        c.Header("Content-Type", "video/mp4")
    }
    c.Header("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, fileSize))
    c.Header("Content-Length", fmt.Sprintf("%d", end-start+1))
    c.Status(http.StatusPartialContent)

    // Serve the requested byte range
    file.Seek(start, 0)
    io.CopyN(c.Writer, file, end-start+1)
})
    router.Run(":8080")
}

