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
	path    string
	content string
	size    int64  // Add size field
	modTime string // Add modTime field
}



// main is the entry point of the application. It parses command line flags,
// loads ignore patterns from .gitignore and .code2pdf.ignore files, collects
// files from the current directory with detailed logging, and generates a PDF
// document with comprehensive processing statistics.
func main() {
	config := parseFlags()

	// Load gitignore patterns from .gitignore and .code2pdf.ignore
	gitignorePatterns := loadGitignorePatterns(".gitignore")
	code2pdfPatterns := loadGitignorePatterns(".code2pdf.ignore")

	// Output which ignore files are being used
	if len(gitignorePatterns) > 0 {
		fmt.Println("Respecting .gitignore for file filtering")
	}
	if len(code2pdfPatterns) > 0 {
		fmt.Println("Respecting .code2pdf.ignore for file filtering")
	}
	if len(gitignorePatterns) == 0 && len(code2pdfPatterns) == 0 {
		fmt.Println("No ignore files found - processing all text files")
	}

	// Collect files
	files, stats, err := collectFiles(".", gitignorePatterns, code2pdfPatterns)
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
	fmt.Printf("Statistics: %d files included, %d files/directories ignored\n", stats.Included, stats.Ignored)
	
	// Display top 5 file extensions
	if len(stats.Extensions) > 0 {
		fmt.Printf("Top file types included:\n")
		
		// Convert map to slice of pairs for sorting
		type ExtCount struct {
			ext   string
			count int
		}
		var extCounts []ExtCount
		for ext, count := range stats.Extensions {
			extCounts = append(extCounts, ExtCount{ext, count})
		}
		
		// Sort by count descending
		for i := 0; i < len(extCounts)-1; i++ {
			for j := i + 1; j < len(extCounts); j++ {
				if extCounts[i].count < extCounts[j].count {
					extCounts[i], extCounts[j] = extCounts[j], extCounts[i]
				}
			}
		}
		
		// Display top 5
		limit := len(extCounts)
		if limit > 5 {
			limit = 5
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  %s: %d files\n", extCounts[i].ext, extCounts[i].count)
		}
	}
}

// parseFlags parses command line arguments and returns a Config struct
// with the application settings.
func parseFlags() Config {
	outputFile := flag.String("o", "code.pdf", "Output PDF file name")
	fontSize := flag.Float64("font-size", 7.0, "Font size for code")
	fontName := flag.String("font", "Courier", "Font name (Courier, Helvetica, Times)")
	lineNumbers := flag.Bool("line-numbers", false, "Include line numbers in the PDF")
	landscape := flag.Bool("landscape", true, "Use landscape orientation instead of portrait")
	version := flag.Bool("version", false, "Show version and exit")
	
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "code2pdf v%s - Convert code directories to PDF documents\n\n", Version)
		fmt.Fprintf(os.Stderr, "Usage: %s [options]\n\nOptions:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nFile filtering respects .gitignore and .code2pdf.ignore files.\n")
	}

	flag.Parse()
	
	if *version {
		fmt.Printf("code2pdf %s\n", Version)
		os.Exit(0)
	}

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

// IgnoreMatch holds information about what rule matched and from which file
type IgnoreMatch struct {
	Matched bool
	Rule    string
	Source  string
}

// matchesGitignore checks if a given file path matches any ignore patterns from
// .gitignore or .code2pdf.ignore files, plus the default .git/ directory exclusion.
// It supports basic gitignore syntax including wildcards and directory patterns.
// Returns detailed match information including the specific rule and source file.
func matchesGitignore(path string, gitignorePatterns, code2pdfPatterns []string) IgnoreMatch {
	// Check default .git/ ignore first
	cleanPath := strings.TrimPrefix(path, "./")
	baseName := filepath.Base(cleanPath)
	
	if strings.HasPrefix(cleanPath, ".git/") || cleanPath == ".git" {
		return IgnoreMatch{Matched: true, Rule: ".git/", Source: "(default)"}
	}

	// Check .gitignore patterns
	for _, pattern := range gitignorePatterns {
		if checkPatternMatch(pattern, cleanPath, baseName) {
			return IgnoreMatch{Matched: true, Rule: pattern, Source: ".gitignore"}
		}
	}
	
	// Check .code2pdf.ignore patterns
	for _, pattern := range code2pdfPatterns {
		if checkPatternMatch(pattern, cleanPath, baseName) {
			return IgnoreMatch{Matched: true, Rule: pattern, Source: ".code2pdf.ignore"}
		}
	}

	return IgnoreMatch{Matched: false}
}

// checkPatternMatch checks if a pattern matches the given path
func checkPatternMatch(pattern, cleanPath, baseName string) bool {
	// Handle negation patterns
	if strings.HasPrefix(pattern, "!") {
		return false // Simplified for this example
	}

	// Handle directory patterns
	if strings.HasSuffix(pattern, "/") {
		dirPattern := pattern[:len(pattern)-1]
		if strings.HasPrefix(cleanPath, dirPattern+"/") || cleanPath == dirPattern {
			return true
		}
		return false
	}

	// Handle exact matches (both full path and basename)
	if pattern == cleanPath || pattern == baseName {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		// Convert the glob pattern to a regex pattern
		regexPattern := "^" + strings.ReplaceAll(regexp.QuoteMeta(pattern), "\\*", ".*") + "$"
		// Check against both full path and basename
		if matched, err := regexp.MatchString(regexPattern, cleanPath); err == nil && matched {
			return true
		}
		if matched, err := regexp.MatchString(regexPattern, baseName); err == nil && matched {
			return true
		}
	}

	return false
}


// FileStats holds statistics about file processing
type FileStats struct {
	Included   int
	Ignored    int
	Extensions map[string]int
}

// collectFiles walks through the directory tree starting from root and collects
// all text files that don't match the ignore patterns from .gitignore and .code2pdf.ignore.
// Returns a slice of FileEntry structs with file metadata and content, plus detailed
// processing statistics including file counts and extension breakdowns.
func collectFiles(root string, gitignorePatterns, code2pdfPatterns []string) ([]FileEntry, FileStats, error) {
	var files []FileEntry
	stats := FileStats{
		Extensions: make(map[string]int),
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip files matching gitignore patterns
		// Skip files matching gitignore patterns
		match := matchesGitignore(path, gitignorePatterns, code2pdfPatterns)
		if match.Matched {
			if info.IsDir() {
				fmt.Printf("Ignoring directory %s [%s: %s]\n", path, match.Source, match.Rule)
				stats.Ignored++
				return filepath.SkipDir
			}
			fmt.Printf("Ignoring %s [%s: %s]\n", path, match.Source, match.Rule)
			stats.Ignored++
			return nil
		}

		// Don't process directories themselves
		if info.IsDir() {
			return nil
		}

		// Check if this is a text file
		if isTextFile(path) {
			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			files = append(files, FileEntry{
				path:    path,
				content: string(content),
				size:    info.Size(),                                  // Store file size
				modTime: info.ModTime().Format("2006-01-02 15:04:05"), // Store last modified time
			})
			stats.Included++
			// Count extensions (use file extension for display)
			ext := strings.ToLower(filepath.Ext(path))
			if ext == "" {
				ext = "(no extension)"
			}
			stats.Extensions[ext]++
		} else {
			// File is not a text file
			fmt.Printf("Skipping binary file: %s\n", path)
			stats.Ignored++
		}

		return nil
	})

	return files, stats, err
}

// isTextFile determines if a file is a text file by analyzing its content.
// Uses multiple heuristics including null byte detection, UTF-8 validation,
// and printable character ratio analysis.
func isTextFile(filePath string) bool {
	file, err := os.Open(filePath)
	if err != nil {
		return false
	}
	defer file.Close()

	// Read up to 8KB for better detection
	buf := make([]byte, 8192)
	n, err := file.Read(buf)
	if err != nil && n == 0 {
		return false
	}
	
	data := buf[:n]
	
	// Empty files are considered text
	if n == 0 {
		return true
	}
	
	// Check for null bytes (strong indicator of binary content)
	nullCount := 0
	for i := 0; i < n; i++ {
		if data[i] == 0 {
			nullCount++
		}
	}
	
	// If more than 1% null bytes, likely binary
	if float64(nullCount)/float64(n) > 0.01 {
		return false
	}
	
	// Check if content is valid UTF-8
	if !isValidUTF8(data) {
		return false
	}
	
	// Count printable characters
	printableCount := 0
	for _, b := range data {
		if isPrintableASCII(b) || b == '\t' || b == '\n' || b == '\r' {
			printableCount++
		}
	}
	
	// If less than 70% printable characters, likely binary
	printableRatio := float64(printableCount) / float64(n)
	return printableRatio >= 0.70
}

// isValidUTF8 checks if the data is valid UTF-8
func isValidUTF8(data []byte) bool {
	for len(data) > 0 {
		r, size := decodeUTF8Rune(data)
		if r == 0xFFFD && size == 1 {
			return false // Invalid UTF-8 sequence
		}
		data = data[size:]
	}
	return true
}

// decodeUTF8Rune decodes a single UTF-8 rune from data
func decodeUTF8Rune(data []byte) (rune, int) {
	if len(data) == 0 {
		return 0, 0
	}
	
	b0 := data[0]
	
	// ASCII
	if b0 < 0x80 {
		return rune(b0), 1
	}
	
	// Multi-byte sequences
	if b0 < 0xC2 {
		return 0xFFFD, 1
	}
	
	if b0 < 0xE0 {
		if len(data) < 2 {
			return 0xFFFD, 1
		}
		b1 := data[1]
		if b1 < 0x80 || b1 >= 0xC0 {
			return 0xFFFD, 1
		}
		return rune(b0&0x1F)<<6 | rune(b1&0x3F), 2
	}
	
	if b0 < 0xF0 {
		if len(data) < 3 {
			return 0xFFFD, 1
		}
		b1, b2 := data[1], data[2]
		if b1 < 0x80 || b1 >= 0xC0 || b2 < 0x80 || b2 >= 0xC0 {
			return 0xFFFD, 1
		}
		return rune(b0&0x0F)<<12 | rune(b1&0x3F)<<6 | rune(b2&0x3F), 3
	}
	
	if b0 < 0xF8 {
		if len(data) < 4 {
			return 0xFFFD, 1
		}
		b1, b2, b3 := data[1], data[2], data[3]
		if b1 < 0x80 || b1 >= 0xC0 || b2 < 0x80 || b2 >= 0xC0 || b3 < 0x80 || b3 >= 0xC0 {
			return 0xFFFD, 1
		}
		return rune(b0&0x07)<<18 | rune(b1&0x3F)<<12 | rune(b2&0x3F)<<6 | rune(b3&0x3F), 4
	}
	
	return 0xFFFD, 1
}

// isPrintableASCII checks if a byte is a printable ASCII character
func isPrintableASCII(b byte) bool {
	return b >= 32 && b <= 126
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
