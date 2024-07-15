
package main

import (
    "database/sql"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/rwcarlsen/goexif/exif"
    _ "github.com/lib/pq"
    "io/ioutil"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "time"
    "bytes"
    "image/jpeg"
    "github.com/nfnt/resize"
    "fmt"
    "github.com/patrickmn/go-cache"
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
      

    router.Run(":8080")
}

