package handler

import (
	"archive/tar"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/moby/moby/client"
)

type browseRequest struct {
	Container string `json:"container"`
	Path      string `json:"path"`
}

type fileEntryJSON struct {
	Name    string    `json:"name"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

type browseResponse struct {
	Entries []fileEntryJSON `json:"entries"`
	Path    string          `json:"path"`
}

type readFileRequest struct {
	Container string `json:"container"`
	Path      string `json:"path"`
}

type readFileResponse struct {
	Content   string `json:"content"`
	Truncated bool   `json:"truncated"`
	Binary    bool   `json:"binary"`
	Size      int64  `json:"size"`
}

func BrowseVolume(c *gin.Context) {
	var req browseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	docker, err := client.New(client.FromEnv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create docker client: " + err.Error()})
		return
	}

	result, err := docker.CopyFromContainer(c.Request.Context(), req.Container, client.CopyFromContainerOptions{
		SourcePath: req.Path,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "copy from container: " + err.Error()})
		return
	}
	defer result.Content.Close()

	if !result.Stat.Mode.IsDir() {
		c.JSON(http.StatusOK, browseResponse{
			Path: req.Path,
			Entries: []fileEntryJSON{{
				Name:    result.Stat.Name,
				IsDir:   false,
				Size:    result.Stat.Size,
				Mode:    result.Stat.Mode.String(),
				ModTime: result.Stat.Mtime,
			}},
		})
		return
	}

	tr := tar.NewReader(result.Content)
	var entries []fileEntryJSON
	seen := make(map[string]bool)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "read tar: " + err.Error()})
			return
		}

		name := strings.TrimPrefix(hdr.Name, "./")
		name = strings.TrimSuffix(name, "/")

		if name == "." || name == "" || seen[name] {
			continue
		}
		seen[name] = true

		entries = append(entries, fileEntryJSON{
			Name:    name,
			IsDir:   hdr.FileInfo().IsDir(),
			Size:    hdr.Size,
			Mode:    hdr.FileInfo().Mode().String(),
			ModTime: hdr.ModTime,
		})
	}

	if entries == nil {
		entries = []fileEntryJSON{}
	}

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return entries[i].Name < entries[j].Name
	})

	c.JSON(http.StatusOK, browseResponse{
		Path:    req.Path,
		Entries: entries,
	})
}

func ReadVolumeFile(c *gin.Context) {
	var req readFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	docker, err := client.New(client.FromEnv)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot create docker client: " + err.Error()})
		return
	}

	result, err := docker.CopyFromContainer(c.Request.Context(), req.Container, client.CopyFromContainerOptions{
		SourcePath: req.Path,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "copy from container: " + err.Error()})
		return
	}
	defer result.Content.Close()

	tr := tar.NewReader(result.Content)
	hdr, err := tr.Next()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read tar: " + err.Error()})
		return
	}

	const maxSize int64 = 1 * 1024 * 1024

	limited := io.LimitReader(tr, maxSize+1)
	raw, err := io.ReadAll(limited)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "read content: " + err.Error()})
		return
	}

	truncated := int64(len(raw)) > maxSize
	if truncated {
		raw = raw[:maxSize]
	}

	binary := !utf8.Valid(raw)

	c.JSON(http.StatusOK, readFileResponse{
		Content:   string(raw),
		Truncated: truncated,
		Binary:    binary,
		Size:      hdr.Size,
	})
}
