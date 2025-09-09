# code2pdf

Convert code directories to PDF documents. Analyzes directory structure, processes source code files with detailed logging and statistics, and generates a formatted PDF with syntax highlighting information and file metadata.

## Usage

```bash
go run .                              # Creates code.pdf
go run . -o myproject.pdf            # Custom output file
go run . -font-size 8 -line-numbers  # Formatting options
```

## Options

- `-o`: Output filename (default: code.pdf)
- `-font-size`: Font size (default: 7.0)
- `-font`: Font name - Courier, Helvetica, Times (default: Courier)
- `-line-numbers`: Show line numbers
- `-landscape`: Landscape orientation (default: true)

## File Filtering

Respects `.gitignore` and `.code2pdf.ignore` files using standard [gitignore syntax](https://git-scm.com/docs/gitignore). The `.git/` directory is always excluded by default. 

**Detailed Logging**: Shows which files are ignored and which ignore rule caused the exclusion, including the source file (`.gitignore`, `.code2pdf.ignore`, or `(default)`).

## Build

```bash
make build    # Build executable
make run      # Run with go run
make test     # Run tests
make clean    # Clean build artifacts
make mod      # Tidy and vendor dependencies
```

## Architecture

Single-file Go program using `github.com/jung-kurt/gofpdf` for PDF generation. Supports 20+ file extensions with automatic language detection and binary file filtering.

**Processing Statistics**: Displays comprehensive stats including:
- Total files included vs ignored
- Top 5 file types by count
- Real-time logging of ignored files with specific rules
