package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config via env vars with safe defaults
var (
	maxUploadMB       = envInt("MAX_UPLOAD_MB", 50)         // hard cap on upload size (MB)
	latexTimeoutSec   = envInt("LATEX_TIMEOUT_SEC", 120)    // timeout per pdflatex run (seconds)
	listenAddr        = envString("LISTEN_ADDR", ":5000")  // server listen address
	formFieldName     = envString("FORM_FIELD", "file")     // multipart form field name
	expectedZipSuffix = ".zip"
	texFilename       = envString("TEX_FILENAME", "document.tex")
)

func envString(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

type apiError struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func compileHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, apiError{Error: "Method not allowed"})
		return
	}

	// Limit body size to avoid OOM & zip bombs
	maxBytes := int64(maxUploadMB) * 1024 * 1024
	r.Body = http.MaxBytesReader(w, r.Body, maxBytes)

	if err := r.ParseMultipartForm(maxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Failed to parse multipart form", Details: err.Error()})
		return
	}

	file, hdr, err := r.FormFile(formFieldName)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("No %q file provided", formFieldName)})
		return
	}
	defer file.Close()

	filename := filepath.Base(hdr.Filename) // strip any path components
	if filename == "" || !strings.HasSuffix(strings.ToLower(filename), expectedZipSuffix) {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Invalid file type. Please upload a ZIP file."})
		return
	}

	tmpDir, err := os.MkdirTemp("", "latex-")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "Could not create temp dir"})
		return
	}
	defer os.RemoveAll(tmpDir)

	zipPath := filepath.Join(tmpDir, filename)
	if err := saveUploaded(file, zipPath); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Failed to save upload", Details: err.Error()})
		return
	}

	if err := unzipSecure(zipPath, tmpDir, maxBytes); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Invalid ZIP file", Details: err.Error()})
		return
	}

	texPath := filepath.Join(tmpDir, texFilename)
	if _, err := os.Stat(texPath); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: fmt.Sprintf("No %s found in the ZIP file", texFilename)})
		return
	}

	pdfName := strings.TrimSuffix(texFilename, filepath.Ext(texFilename)) + ".pdf"
	pdfPath := filepath.Join(tmpDir, pdfName)

	// Run pdflatex twice, with timeouts
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(latexTimeoutSec)*time.Second)
	defer cancel()

	if out, err := runPdflatex(ctx, tmpDir, texPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "LaTeX compilation failed", Details: tail(out, 8<<10)})
		return
	}
	// second pass
	if out, err := runPdflatex(ctx, tmpDir, texPath); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "LaTeX compilation failed (2nd pass)", Details: tail(out, 8<<10)})
		return
	}

	f, err := os.Open(pdfPath)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiError{Error: "PDF not generated"})
		return
	}
	defer f.Close()

	stat, _ := f.Stat()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", pdfName))
	http.ServeContent(w, r, pdfName, stat.ModTime(), f)
}

func saveUploaded(src multipart.File, dstPath string) error {
	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()
	_, err = io.Copy(dst, src)
	return err
}

// unzipSecure extracts zipPath into dest, preventing Zip Slip and limiting total uncompressed size.
func unzipSecure(zipPath, dest string, sizeLimit int64) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	var total int64
	for _, zf := range r.File {
		// Basic bomb guard: sum of uncompressed sizes
		total += int64(zf.UncompressedSize64)
		if total > sizeLimit*5 { // allow some expansion, adjust as needed
			return errors.New("uncompressed size too large")
		}

		cleanName := filepath.Clean(zf.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ":") {
			return fmt.Errorf("suspicious path: %s", zf.Name)
		}

		targetPath := filepath.Join(dest, cleanName)
		// Ensure targetPath is within dest
		if !strings.HasPrefix(targetPath, dest+string(filepath.Separator)) && targetPath != dest {
			return fmt.Errorf("zip slip detected: %s", zf.Name)
		}

		if zf.FileInfo().IsDir() {
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}

		rc, err := zf.Open()
		if err != nil {
			return err
		}
		func() {
			defer rc.Close()
			out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, zf.Mode())
			if err != nil {
				rc.Close()
				panic(err) // handled by outer recover in production or return err directly
			}
			defer out.Close()
			if _, err = io.Copy(out, rc); err != nil {
				panic(err)
			}
		}()
	}
	return nil
}

func runPdflatex(ctx context.Context, workDir, texPath string) (string, error) {
	cmd := exec.CommandContext(ctx, "pdflatex", "-interaction=nonstopmode", "-output-directory", workDir, texPath)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func tail(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}

func main() {
	log.Printf("Starting server on %s (MAX_UPLOAD_MB=%d, LATEX_TIMEOUT_SEC=%d)\n", listenAddr, maxUploadMB, latexTimeoutSec)
	http.HandleFunc("/compile", compileHandler)
	if err := http.ListenAndServe(listenAddr, nil); err != nil {
		log.Fatal(err)
	}
}
