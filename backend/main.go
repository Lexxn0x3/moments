package main

import (
	"database/sql"
	"fmt"
	"image/jpeg"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
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
  "github.com/adrium/goheif"
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
    Type           string    `json:"type"`
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
			log.Printf("Error reading form file: %v", err)
			c.String(http.StatusBadRequest, "Bad request")
			return
		}
		defer file.Close()

		uuid := uuid.New().String()
		extension := strings.ToLower(filepath.Ext(header.Filename))
		filename := uuid + extension

		filePath := filepath.Join(uploadDir, filename)
		out, err := os.Create(filePath)
		if err != nil {
			log.Printf("Error creating file: %v", err)
			c.String(http.StatusInternalServerError, "Unable to save the file")
			return
		}
		defer out.Close()

		_, err = out.ReadFrom(file)
		if err != nil {
			log.Printf("Error reading from file: %v", err)
			c.String(http.StatusInternalServerError, "Unable to save the file")
			return
		}

		var dateTaken time.Time
		var mediaType string

		switch extension {
		case ".jpg", ".jpeg", ".png":
			mediaType = "image"
			file.Seek(0, 0)
			exifData, err := exif.Decode(file)
			if err == nil {
				dateTaken, err = exifData.DateTime()
				if err != nil {
					dateTaken = time.Now()
				}
			} else {
				dateTaken = time.Now()
			}
		case ".mp4", ".mov", ".MOV":
			mediaType = "video"
			dateTaken = time.Now()
			previewPath := filepath.Join(uploadDir, uuid+"_preview.jpg")
			cmd := exec.Command("ffmpeg", "-i", filePath, "-ss", "00:00:01.000", "-vframes", "1", previewPath)
			output, err := cmd.CombinedOutput()
			if err != nil {
				log.Printf("Error generating video preview: %v\nFFmpeg output: %s", err, output)
				c.String(http.StatusInternalServerError, "Error generating video preview")
				return
			}
		case ".heic", ".HEIC":
			mediaType = "image"
			jpgFilename := uuid + ".jpg"
			jpgFilePath := filepath.Join(uploadDir, jpgFilename)
			err = convertHeicToJpg(filePath, jpgFilePath)
			if err != nil {
				log.Printf("Error converting HEIC to JPG: %v", err)
				c.String(http.StatusInternalServerError, "Error converting HEIC to JPG")
				return
			}
			filePath = jpgFilePath
			filename = jpgFilename
			file.Seek(0, 0)
			exifData, err := exif.Decode(file)
			if err == nil {
				dateTaken, err = exifData.DateTime()
				if err != nil {
					dateTaken = time.Now()
				}
			} else {
				dateTaken = time.Now()
			}
		default:
			log.Printf("Unsupported file type: %s", extension)
			c.String(http.StatusBadRequest, "Unsupported file type")
			return
		}

		metadata := header.Filename

		_, err = db.Exec("INSERT INTO photos (filename, metadata, date_taken, type) VALUES ($1, $2, $3, $4)", filename, metadata, dateTaken, mediaType)
		if err != nil {
			log.Printf("Error saving metadata to database: %v", err)
			c.String(http.StatusInternalServerError, "Failed to save metadata")
			return
		}

		c.String(http.StatusOK, "Upload successful")
	})  
router.GET("/api/photos", func(c *gin.Context) {
        rows, err := db.Query("SELECT filename, metadata, date_taken, photographer_id, event, type FROM photos ORDER BY date_taken DESC")
        if err != nil {
            log.Fatal(err)
        }
        defer rows.Close()

        var photos []Photo
        for rows.Next() {
            var photo Photo
            rows.Scan(&photo.Filename, &photo.Metadata, &photo.DateTaken, &photo.PhotographerID, &photo.Event, &photo.Type)
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

    // Determine if the file is an image or video based on the stored metadata
    var mediaType string
    err := db.QueryRow("SELECT type FROM photos WHERE filename = $1", filename).Scan(&mediaType)
    if err != nil {
        c.String(http.StatusNotFound, "File not found")
        return
    }

    var previewFilename string
    if mediaType == "video" {
        previewFilename = filename[:len(filename)-len(filepath.Ext(filename))] + "_preview.jpg"
    } else {
        previewFilename = filename
    }

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

    // Generate a new preview filename for the resized image
    resizedPreviewFilename := fmt.Sprintf("%s_%d.jpg", previewFilename, width)
    resizedPreviewFilePath := filepath.Join(uploadDir, resizedPreviewFilename)

    // Check if the resized preview image exists on disk
    if _, err := os.Stat(resizedPreviewFilePath); err == nil {
        // If the file exists, serve it directly
        c.File(resizedPreviewFilePath)
        return
    }

    // If the resized preview image does not exist, generate it
    filePath := filepath.Join(uploadDir, previewFilename)
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

    // Save resized image to disk
    out, err := os.Create(resizedPreviewFilePath)
    if err != nil {
        c.String(http.StatusInternalServerError, "Error creating preview file")
        return
    }
    defer out.Close()

    err = jpeg.Encode(out, resizedImg, nil)
    if err != nil {
        c.String(http.StatusInternalServerError, "Error encoding image")
        return
    }

    // Serve the newly created resized preview image
    c.File(resizedPreviewFilePath)
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
func convertHeicToJpg(input, output string) error {
	fileInput, err := os.Open(input)
	if err != nil {
		return err
	}
	defer fileInput.Close()

	img, err := goheif.Decode(fileInput)
	if err != nil {
		return err
	}

	fileOutput, err := os.OpenFile(output, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer fileOutput.Close()

	err = jpeg.Encode(fileOutput, img, nil)
	if err != nil {
		return err
	}

	return nil
}
