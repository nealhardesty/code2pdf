package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/jung-kurt/gofpdf"
)

// Config holds the application configuration
type Config struct {
	outputFile  string
	fontSize    float64
	fontName    string
	lineNumbers bool
	landscape   bool
}

// FileEntry represents a file to be included in the PDF
type FileEntry struct {
	path     string
	content  string
	language string
	size     int64  // Add size field
	modTime  string // Add modTime field
}


// File extensions to language mappings for syntax highlighting information
var extensionToLanguage = map[string]string{
	".go":   "Go",
	".py":   "Python",
	".rs":   "Rust",
	".js":   "JavaScript",
	".ts":   "TypeScript",
	".html": "HTML",
	".css":  "CSS",
	".java": "Java",
	".c":    "C",
	".cpp":  "C++",
	".h":    "C/C++ Header",
	".rb":   "Ruby",
	".php":  "PHP",
	".sh":   "Shell",
	".md":   "Markdown",
	".json": "JSON",
	".xml":  "XML",
	".yml":  "YAML",
	".yaml": "YAML",
	".toml": "TOML",
	".sql":  "SQL",
	".txt":  "Text",
}

// main is the entry point of the application. It parses command line flags,
// loads ignore patterns from .gitignore and .code2pdf.ignore files, collects
// files from the current directory, and generates a PDF document.
func main() {
	config := parseFlags()

	// Load gitignore patterns from .gitignore and .code2pdf.ignore
	gitignorePatterns := loadGitignorePatterns(".gitignore")
	code2pdfPatterns := loadGitignorePatterns(".code2pdf.ignore")
	allPatterns := append(gitignorePatterns, code2pdfPatterns...)

	// Output which ignore files are being used
	if len(gitignorePatterns) > 0 {
		fmt.Println("Respecting .gitignore for file filtering")
	}
	if len(code2pdfPatterns) > 0 {
		fmt.Println("Respecting .code2pdf.ignore for file filtering")
	}
	if len(allPatterns) == 0 {
		fmt.Println("No ignore files found - processing all text files")
	}

	// Collect files
	files, err := collectFiles(".", allPatterns)
	if err != nil {
		fmt.Printf("Error collecting files: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("No files found to include in the PDF")
		os.Exit(0)
	}

	// Create PDF
	err = createPDF(files, config)
	if err != nil {
		fmt.Printf("Error creating PDF: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nPDF created successfully: %s\n", config.outputFile)
}

// parseFlags parses command line arguments and returns a Config struct
// with the application settings.
func parseFlags() Config {
	outputFile := flag.String("o", "code.pdf", "Output PDF file name")
	fontSize := flag.Float64("font-size", 7.0, "Font size for code")
	fontName := flag.String("font", "Courier", "Font name (Courier, Helvetica, Times)")
	lineNumbers := flag.Bool("line-numbers", false, "Include line numbers in the PDF")
	landscape := flag.Bool("landscape", true, "Use landscape orientation instead of portrait")

	flag.Parse()

	return Config{
		outputFile:  *outputFile,
		fontSize:    *fontSize,
		fontName:    *fontName,
		lineNumbers: *lineNumbers,
		landscape:   *landscape,
	}
}


// loadGitignorePatterns reads ignore patterns from the specified file.
// It returns a slice of pattern strings, ignoring empty lines and comments.
func loadGitignorePatterns(filename string) []string {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil
	}
	defer file.Close()

	var patterns []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" && !strings.HasPrefix(line, "#") {
			patterns = append(patterns, line)
		}
	}

	return patterns
}

// matchesGitignore checks if a given file path matches any of the gitignore patterns.
// It supports basic gitignore syntax including wildcards and directory patterns.
func matchesGitignore(path string, patterns []string) bool {
	if len(patterns) == 0 {
		return false
	}

	for _, pattern := range patterns {
		// Handle negation patterns
		if strings.HasPrefix(pattern, "!") {
			continue // Simplified for this example
		}

		// Handle directory patterns
		if strings.HasSuffix(pattern, "/") {
			dirPattern := pattern[:len(pattern)-1]
			if strings.HasPrefix(path, dirPattern) {
				return true
			}
			continue
		}

		// Handle exact matches
		if pattern == path {
			return true
		}

		// Handle wildcard patterns (simplified)
		if strings.Contains(pattern, "*") {
			// Convert the glob pattern to a regex pattern
			regexPattern := "^" + strings.ReplaceAll(pattern, "*", ".*") + "$"
			matched, err := regexp.MatchString(regexPattern, path)
			if err == nil && matched {
				return true
			}
		}
	}

	return false
}


// collectFiles walks through the directory tree starting from root and collects
// all text files that don't match the ignore patterns. Returns a slice of FileEntry
// structs containing file metadata and content.
func collectFiles(root string, gitignorePatterns []string) ([]FileEntry, error) {
	var files []FileEntry

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip files matching gitignore patterns
		if matchesGitignore(path, gitignorePatterns) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Don't process directories themselves
		if info.IsDir() {
			return nil
		}

		// Skip binary files and consider only text files
		ext := strings.ToLower(filepath.Ext(path))
		_, isCodeFile := extensionToLanguage[ext]

		// If not recognized extension, check if it might be text
		if !isCodeFile {
			if isTextFile(path) {
				extensionToLanguage[ext] = "Text"
				isCodeFile = true
			}
		}

		if isCodeFile {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			files = append(files, FileEntry{
				path:     path,
				content:  string(content),
				language: extensionToLanguage[ext],
				size:     info.Size(),                                  // Store file size
				modTime:  info.ModTime().Format("2006-01-02 15:04:05"), // Store last modified time
			})
		}

		return nil
	})

	return files, err
}

// isTextFile determines if a file is a text file by reading the first 512 bytes
// and checking for null bytes which are common in binary files.
func isTextFile(filePath string) bool {
	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read the first 512 bytes
	buf := make([]byte, 512)
	n, err := file.Read(buf)
	if err != nil {
		return false
	}

	// Check if there are any null bytes (common in binary files)
	for i := range n {
		if buf[i] == 0 {
			return false
		}
	}

	return true
}

// formatFileSize converts a file size in bytes to a human-readable format
// using appropriate units (B, KB, MB, GB, etc.).
func formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}

// createPDF generates a PDF document from the collected files using the specified
// configuration. It creates a title page, table of contents, and includes each file
// with proper formatting and page breaks.
func createPDF(files []FileEntry, config Config) error {
	baseDir := currentDirectoryBase()

	// Set orientation based on config
	orientation := "P" // Portrait by default
	if config.landscape {
		orientation = "L" // Landscape
	}

	currentSection := "???"

	pdf := gofpdf.New(orientation, "mm", "A4", "")

	// Add page numbering in the footer
	pdf.SetFooterFunc(func() {
		// Set font for page numbers
		pdf.SetFont("Arial", "I", 8)

		// Go to 1.5 cm from bottom of the page
		pdf.SetY(-15)

		// Print page number right-aligned
		pdf.CellFormat(0, 10, fmt.Sprintf("%s   -   [%d]", currentSection, pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.SetFont(config.fontName, "", config.fontSize)

	// Add a title page
	pdf.AddPage()
	pdf.SetFont(config.fontName, "B", 24)
	pdf.Cell(0, 10, baseDir)
	pdf.Ln(20)

	currentSection = "Table of Contents"
	// Add table of contents
	pdf.SetFont(config.fontName, "B", 12)
	pdf.Cell(0, 10, "Table of Contents:")
	pdf.Ln(10)

	pdf.SetFont(config.fontName, "", 12)

	for i, file := range files {
		humanReadableSize := formatFileSize(file.size) // Format file size
		pdf.Cell(0, 5, fmt.Sprintf("%d. %s/%s (%s, Last Modified: %s)", i+1, baseDir, file.path, humanReadableSize, file.modTime))
		pdf.Ln(5)
	}

	// Add each file
	for _, file := range files {
		humanReadableSize := formatFileSize(file.size) // Format file size
		fmt.Printf("Importing %s (%s, Last Modified: %s)\n", file.path, humanReadableSize, file.modTime)
		pdf.AddPage()

		// Add file header
		pdf.SetFont(config.fontName, "B", config.fontSize+2)
		pdf.Cell(0, 10, fmt.Sprintf("%s/%s (%s, Last Modified: %s)", baseDir, file.path, humanReadableSize, file.modTime))
		pdf.Ln(10)

		// Counter for continued pages
		continuedPage := 1

		currentSection = fmt.Sprintf("%s/%s page %d", baseDir, file.path, continuedPage)

		// Add file content
		pdf.SetFont(config.fontName, "", config.fontSize)
		lines := strings.Split(file.content, "\n")
		for i, line := range lines {
			if config.lineNumbers {
				// Add line numbers
				lineNum := fmt.Sprintf("%4d | ", i+1)
				pdf.Cell(20, 5, lineNum)
			}

			// Handle tabs (replace with spaces)
			line = strings.ReplaceAll(line, "\t", "    ")

			// Add the actual code line
			pdf.Cell(0, 5, line)
			pdf.Ln(5)

			// Calculate appropriate page break threshold based on orientation
			pageBreakThreshold := 270.0 // Default for portrait
			if config.landscape {
				pageBreakThreshold = 170.0 // Adjusted for landscape
			}

			// Add page break if we're near the bottom
			if pdf.GetY() > pageBreakThreshold {
				currentSection = fmt.Sprintf("%s/%s page %d", baseDir, file.path, continuedPage)
				pdf.AddPage()

				// Re-add the file info on the new page
				pdf.SetFont(config.fontName, "B", config.fontSize+2)
				pdf.Cell(0, 10, baseDir+"/"+file.path+" (continued)")
				continuedPage++
				pdf.Ln(10)
				pdf.SetFont(config.fontName, "", config.fontSize)
			}
		}
	}

	return pdf.OutputFileAndClose(config.outputFile)
}

// currentDirectoryBase returns the base name of the current working directory.
// It's used for the PDF title and headers. Returns "???" if unable to determine.
func currentDirectoryBase() string {
	// Get the current working directory
	dir, err := os.Getwd()
	if err != nil {
		return "???"
	}

	// Get just the name of the directory (not the full path)
	dirName := filepath.Base(dir)

	return dirName
}
