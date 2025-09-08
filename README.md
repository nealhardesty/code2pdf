# code2pdf

Convert code directories to PDF documents. Analyzes directory structure, processes source code files, and generates a formatted PDF with syntax highlighting information and file metadata.

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

Respects `.gitignore` and `.code2pdf.ignore` files using standard [gitignore syntax](https://git-scm.com/docs/gitignore).

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
