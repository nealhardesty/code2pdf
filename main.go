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
	outputFile      string
	blacklistRegexs []string
	skipDirs        []string
	useGitignore    bool
	fontSize        float64
	fontName        string
	lineNumbers     bool
	landscape       bool
}

// FileEntry represents a file to be included in the PDF
type FileEntry struct {
	path     string
	content  string
	language string
}

// Common directories to skip by default
var defaultSkipDirs = []string{
	".git", "node_modules", "vendor", "venv", ".venv",
	"__pycache__", "dist", "build", ".idea", ".vscode",
}

// Common files to skip by default
var defaultBlacklistRegexes = []string{
	".gitignore", ".DS_Store", "Thumbs.db",
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

func main() {
	config := parseFlags()

	// Compile blacklist regexes
	var blacklistRegexes []*regexp.Regexp
	for _, pattern := range config.blacklistRegexs {
		if pattern != "" {
			regex, err := regexp.Compile(pattern)
			if err != nil {
				fmt.Printf("Error compiling blacklist regex '%s': %v\n", pattern, err)
				os.Exit(1)
			}
			blacklistRegexes = append(blacklistRegexes, regex)
		}
	}

	// Load gitignore patterns if needed
	var gitignorePatterns []string
	if config.useGitignore {
		gitignorePatterns = loadGitignorePatterns()
	}

	// Collect files
	files, err := collectFiles(".", blacklistRegexes, config.skipDirs, gitignorePatterns)
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

	fmt.Printf("PDF created successfully: %s\n", config.outputFile)
}

func parseFlags() Config {
	outputFile := flag.String("o", "code.pdf", "Output PDF file name")
	blacklistRegex := flag.String("blacklist", strings.Join(defaultBlacklistRegexes, ","), "Regex pattern for files to ignore (comma-separated for multiple patterns)")
	skipDirsStr := flag.String("skip-dirs", strings.Join(defaultSkipDirs, ","), "Comma-separated list of directories to skip")
	useGitignore := flag.Bool("gitignore", true, "Use ./.gitignore patterns for blacklisting as well")
	fontSize := flag.Float64("font-size", 7.0, "Font size for code")
	fontName := flag.String("font", "Courier", "Font name (Courier, Helvetica, Times)")
	lineNumbers := flag.Bool("line-numbers", false, "Include line numbers in the PDF")
	landscape := flag.Bool("landscape", true, "Use landscape orientation instead of portrait")

	// Define a custom flag for multiple regex patterns
	var blacklistPatterns multiFlag
	flag.Var(&blacklistPatterns, "b", "Regex pattern to ignore (can be specified multiple times)")

	flag.Parse()

	skipDirs := strings.Split(*skipDirsStr, ",")

	// Combine both ways of specifying blacklist patterns
	var blacklistRegexs []string

	// Add patterns from the -blacklist flag (comma-separated)
	if *blacklistRegex != "" {
		blacklistRegexs = append(blacklistRegexs, strings.Split(*blacklistRegex, ",")...)
	}

	// Add patterns from the -b flag (specified multiple times)
	blacklistRegexs = append(blacklistRegexs, blacklistPatterns...)

	return Config{
		outputFile:      *outputFile,
		blacklistRegexs: blacklistRegexs,
		skipDirs:        skipDirs,
		useGitignore:    *useGitignore,
		fontSize:        *fontSize,
		fontName:        *fontName,
		lineNumbers:     *lineNumbers,
		landscape:       *landscape,
	}
}

// multiFlag is a custom flag type that can be specified multiple times
type multiFlag []string

func (f *multiFlag) String() string {
	return strings.Join(*f, ",")
}

func (f *multiFlag) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func loadGitignorePatterns() []string {
	gitignoreFile := ".gitignore"
	if _, err := os.Stat(gitignoreFile); os.IsNotExist(err) {
		return nil
	}

	file, err := os.Open(gitignoreFile)
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

func shouldSkipDir(dirPath string, skipDirs []string) bool {
	dir := filepath.Base(dirPath)
	for _, skipDir := range skipDirs {
		if skipDir == dir {
			return true
		}
	}
	return false
}

func collectFiles(root string, blacklistRegexes []*regexp.Regexp, skipDirs, gitignorePatterns []string) ([]FileEntry, error) {
	var files []FileEntry

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories in the skip list
		if info.IsDir() {
			if shouldSkipDir(path, skipDirs) {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip files matching any blacklist regex
		if len(blacklistRegexes) > 0 {
			for _, regex := range blacklistRegexes {
				if regex.MatchString(path) {
					return nil
				}
			}
		}

		// Skip files matching gitignore patterns
		if matchesGitignore(path, gitignorePatterns) {
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
			})
		}

		return nil
	})

	return files, err
}

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
	for i := 0; i < n; i++ {
		if buf[i] == 0 {
			return false
		}
	}

	return true
}

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
		pdf.Cell(0, 5, fmt.Sprintf("%d. %s/%s", i+1, baseDir, file.path))
		pdf.Ln(5)
	}

	// Add each file
	for _, file := range files {
		pdf.AddPage()

		// Add file header
		pdf.SetFont(config.fontName, "B", config.fontSize+2)
		pdf.Cell(0, 10, fmt.Sprintf("%s/%s", baseDir, file.path))
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
