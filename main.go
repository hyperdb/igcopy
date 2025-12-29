package main

import (
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

var (
	inputDir  string
	outputDir string
)

// Supported image extensions
var imageExts = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
	".gif":  true,
	".bmp":  true,
	".webp": true,
	".tiff": true,
	".tif":  true,
	".heic": true,
}

func main() {
	flag.StringVar(&inputDir, "input", "", "Input directory")
	flag.StringVar(&outputDir, "output", "", "Output directory")
	flag.Parse()

	if inputDir == "" || outputDir == "" {
		flag.Usage()
		os.Exit(1)
	}

	if err := run(); err != nil {
		log.Fatalf("Error: %v", err)
	}
}

func run() error {
	// Cache for database connections: map[directory_path]*sql.DB
	// Using a map to avoid re-opening DB for every file in the same directory.
	dbs := make(map[string]*sql.DB)

	// Ensure all DBs are closed when we return
	defer func() {
		for _, db := range dbs {
			db.Close()
		}
	}()

	err := filepath.WalkDir(inputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories (we only process files)
		if d.IsDir() {
			return nil
		}

		// Check if it is an image file
		if !isImage(path) {
			return nil
		}

		// Calculate relative path to mirror structure
		relPath, err := filepath.Rel(inputDir, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path: %w", err)
		}

		destPath := filepath.Join(outputDir, relPath)
		destDir := filepath.Dir(destPath)
		fileName := filepath.Base(destPath)

		// Ensure output directory exists
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", destDir, err)
		}

		// Get or open DB for this directory
		db, ok := dbs[destDir]
		if !ok {
			dbPath := filepath.Join(destDir, "igcopy.db")
			db, err = sql.Open("sqlite3", dbPath)
			if err != nil {
				return fmt.Errorf("failed to open db %s: %w", dbPath, err)
			}

			if err := initDB(db); err != nil {
				db.Close()
				return fmt.Errorf("failed to init db %s: %w", dbPath, err)
			}
			dbs[destDir] = db
		}

		// Check if file is already registered
		exists, err := fileExistsInDB(db, fileName)
		if err != nil {
			return fmt.Errorf("failed to check db for %s: %w", fileName, err)
		}

		if exists {
			fmt.Printf("[Skipped] %s (already in DB)\n", relPath)
			return nil
		}

		// Copy the file
		fmt.Printf("[Copying] %s\n", relPath)
		if err := copyFile(path, destPath); err != nil {
			return fmt.Errorf("failed to copy file to %s: %w", destPath, err)
		}

		// Register in DB
		if err := registerFileInDB(db, fileName); err != nil {
			// If registration fails, should we remove the copied file?
			// For safety, let's keep it but logging is important.
			// Ideally, we might want to wrap this in a transaction if we were strict,
			// but for a file copy tool, just erroring out is acceptable.
			return fmt.Errorf("failed to register %s in db: %w", fileName, err)
		}

		return nil
	})

	return err
}

func isImage(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return imageExts[ext]
}

func initDB(db *sql.DB) error {
	query := `CREATE TABLE IF NOT EXISTS files (name TEXT PRIMARY KEY);`
	_, err := db.Exec(query)
	return err
}

func fileExistsInDB(db *sql.DB, name string) (bool, error) {
	var count int
	query := `SELECT count(*) FROM files WHERE name = ?`
	err := db.QueryRow(query, name).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func registerFileInDB(db *sql.DB, name string) error {
	query := `INSERT INTO files (name) VALUES (?)`
	_, err := db.Exec(query, name)
	return err
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
