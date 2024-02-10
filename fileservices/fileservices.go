// Copyright 2023 Microsoft Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileservices

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/mux"
	"github.com/microsoft/jupyterpython/util"
	"github.com/rs/zerolog/log"
)

type FileMetadata struct {
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Filename    string    `json:"filename"` // remove this after CP change since we have name
	Size        int64     `json:"size"`
	LastModTime time.Time `json:"last_modified_time"`
	MIMEType    string    `json:"mime_type"` // remove this after CP change since we have type
}

const (
	FileType                 = "file"
	DirPath                  = "/mnt/data"
	DirType                  = "directory"
	ErrCodeFileNotFound      = "ERR_FILE_NOT_FOUND"
	ErrCodeDirNotFound       = "ERR_DIR_NOT_FOUND"
	ErrCodeFileAccess        = "ERR_FILE_ACCESS"
	ErrCodeSymlinkNotAllowed = "ERR_SYMLINK_NOT_ALLOWED"
)

func ListFilesHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	targetPath := DirPath

	// supports both listFiles and listFiles/{path}
	if customPath, ok := vars["path"]; ok && customPath != "" {
		// clean the path to prevent directory traversal attacks
		customPath = filepath.Clean("/" + customPath)
		targetPath = filepath.Join(DirPath, customPath)
	}

	files, err := os.ReadDir(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeDirNotFound, "File path not found")
		} else {
			log.Error().Err(err).Msg("Unable to read directory")
			util.SendHTTPResponse(w, http.StatusInternalServerError, "error reading directory"+err.Error(), true)
		}
		return
	}

	var metadataList []FileMetadata
	for _, f := range files {
		// Ignore if it is a symlink
		if f.Type()&os.ModeSymlink != 0 {
			continue
		}

		fullPath := filepath.Join(targetPath, f.Name())
		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			log.Error().Err(err).Str("file", f.Name()).Msg("Unable to get file info")
			continue
		}

		mimeType := mime.TypeByExtension(filepath.Ext(f.Name()))
		if mimeType == "" {
			mimeType = "application/octet-stream" // default MIME type
		}

		if fileInfo.IsDir() {
			metadataList = append(metadataList, FileMetadata{
				Name:        f.Name(),
				Type:        DirType,
				Filename:    f.Name(), // remove this after CP change since we have Name
				Size:        fileInfo.Size(),
				LastModTime: fileInfo.ModTime(),
				MIMEType:    mimeType, // remove this after CP change since we have type
			})
		} else {
			metadataList = append(metadataList, FileMetadata{
				Name:        f.Name(),
				Type:        FileType,
				Filename:    f.Name(), // remove this after CP change since we have Name
				Size:        fileInfo.Size(),
				LastModTime: fileInfo.ModTime(),
				MIMEType:    mimeType, // remove this after CP change since we have type
			})
		}
	}

	response, err := json.Marshal(metadataList)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal response")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error marshaling response"+err.Error(), true)
		return
	}

	log.Info().Msg("List files successfully.\n")
	log.Info().Msg(fmt.Sprintf("List files successfully" + string(response)))
	util.SendHTTPResponse(w, http.StatusOK, string(response), false)
}

func UploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// get custom path from URL
	vars := mux.Vars(r)
	targetPath := DirPath

	// supports both uploadFile and uploadFile/{path}
	if customPath, ok := vars["path"]; ok && customPath != "" {
		// clean the path to prevent directory traversal attacks
		customPath = filepath.Clean("/" + customPath)
		targetPath = filepath.Join(DirPath, customPath)
	}

	err := r.ParseMultipartForm(250 << 20) // 250MB limit
	if err != nil {
		log.Error().Err(err).Msg("Unable to parse form")
		util.SendHTTPResponse(w, http.StatusBadRequest, "error parsing form"+err.Error(), true)
		return
	}

	files := r.MultipartForm.File["file"]
	var metadataList []FileMetadata

	for _, file := range files {
		if err := processFile(file, &metadataList, targetPath); err != nil {
			log.Error().Err(err).Str("filename", file.Filename).Send()
			// choose to continue?
		}
	}

	response, err := json.Marshal(metadataList)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal response")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error marshaling response"+err.Error(), true)
		return
	}

	log.Info().Msg("Upload files successfully.\n")
	util.SendHTTPResponse(w, http.StatusOK, string(response), false)
}

// processFile handles the processing of each individual file and updates the metadata list.
func processFile(file *multipart.FileHeader, metadataList *[]FileMetadata, path string) error {
	src, err := file.Open()
	if err != nil {
		return err
	}
	defer src.Close()

	// url decode filename
	decodedFilename, err := url.QueryUnescape(file.Filename)
	if err != nil {
		log.Error().Err(err).Str("filename", file.Filename).Msg("Error decoding file name")
		return err
	}
	file.Filename = decodedFilename

	// create the directory if it doesn't exist
	os.MkdirAll(path, os.ModePerm)

	dstPath := filepath.Join(path, filepath.Base(file.Filename))
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return err
	}

	if fileInfo, err := dst.Stat(); err == nil {
		*metadataList = append(*metadataList, FileMetadata{
			Filename:    file.Filename,
			Size:        fileInfo.Size(),
			LastModTime: fileInfo.ModTime(),
		})
	} else {
		return err
	}

	if err := os.Chmod(dstPath, 0777); err != nil {
		return err
	}

	return nil
}

func DownloadFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	encodedFilename := vars["filename"]

	// URL decode the filename
	decodedFilename, err := url.QueryUnescape(encodedFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file name")
		util.SendHTTPResponse(w, http.StatusBadRequest, "error decoding file name"+err.Error(), true)
		return
	}

	// Use the decoded filename for further processing
	filename := filepath.Base(decodedFilename)

	targetPath := DirPath
	// supports both dowloadFile and dowloadFile/{path}/{fileName}
	if customPath, ok := vars["path"]; ok && customPath != "" {
		// clean the path to prevent directory traversal attacks
		customPath = filepath.Clean("/" + customPath)
		targetPath = filepath.Join(DirPath, customPath)
	}

	filePath := filepath.Join(targetPath, filename)

	fileInfo, err := os.Lstat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeFileNotFound, "File not found")
		} else {
			logAndRespond(w, http.StatusInternalServerError, ErrCodeFileAccess, "Error accessing file")
		}
		return
	}

	if fileInfo.Mode()&os.ModeSymlink != 0 {
		logAndRespond(w, http.StatusBadRequest, ErrCodeSymlinkNotAllowed, "Symlinks not allowed")
		return
	}

	http.ServeFile(w, r, filePath)
}

func logAndRespond(w http.ResponseWriter, statusCode int, errCode, errMsg string) {
	log.Error().Str("error_code", errCode).Msg(errMsg)
	util.SendHTTPResponse(w, statusCode, fmt.Sprintf("%s: %s", errCode, errMsg), true)
}

func DeleteFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	encodedFilename := vars["filename"]

	// URL decode the filename
	decodedFilename, err := url.QueryUnescape(encodedFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file name")
		util.SendHTTPResponse(w, http.StatusBadRequest, "error decoding file name"+err.Error(), true)
		return
	}

	// Use the decoded filename in further processing
	filename := filepath.Base(decodedFilename)
	filePath := filepath.Join(DirPath, filename)

	// Check if the file exists
	_, err = os.Lstat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeFileNotFound, "File not found")
		} else {
			logAndRespond(w, http.StatusInternalServerError, ErrCodeFileAccess, "Error accessing file")
		}
		return
	}

	// File exists, proceed with deletion
	err = os.Remove(filePath)
	if err != nil {
		log.Error().Err(err).Msg(fmt.Sprintf("Error deleting file %s", filename))
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error deleting file"+err.Error(), true)
		return
	}

	log.Info().Msg(fmt.Sprintf("File %s deleted successfully.\n", filename))
	util.SendHTTPResponse(w, http.StatusOK, "file deleted successfully", true)
}

func GetFileHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	encodedFilename := vars["filename"]

	// URL decode the filename
	decodedFilename, err := url.QueryUnescape(encodedFilename)
	if err != nil {
		log.Error().Err(err).Msg("Error decoding file name")
		util.SendHTTPResponse(w, http.StatusBadRequest, "error decoding file name"+err.Error(), true)
		return
	}

	// Use the decoded filename in further processing
	filename := filepath.Base(decodedFilename)
	filePath := filepath.Join(DirPath, filename)

	// if file exists, retrieve file information using os.Stat
	fileInfo, err := os.Lstat(filePath)
	// handle not found or other errors
	if err != nil {
		if os.IsNotExist(err) {
			logAndRespond(w, http.StatusNotFound, ErrCodeFileNotFound, "File not found")
		} else {
			logAndRespond(w, http.StatusInternalServerError, ErrCodeFileAccess, "Error accessing file")
		}
		return
	}

	mimeType := mime.TypeByExtension(filepath.Ext(filename))
	if mimeType == "" {
		mimeType = "application/octet-stream" // default MIME type
	}

	fileMetadata := FileMetadata{
		Filename:    filename,
		Size:        fileInfo.Size(),
		LastModTime: fileInfo.ModTime(),
		MIMEType:    mimeType,
	}

	response, err := json.Marshal(fileMetadata)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshal response")
		util.SendHTTPResponse(w, http.StatusInternalServerError, "error marshaling response"+err.Error(), true)
		return
	}

	log.Info().Msg(fmt.Sprintf("Get file %s successfully.\n", filename))
	util.SendHTTPResponse(w, http.StatusOK, string(response), false)
}
