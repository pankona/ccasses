package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/pankona/ccasses/internal/parser"
	"github.com/pankona/ccasses/internal/server"
)

func claudeProjectsDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("get home dir: %w", err)
	}
	return filepath.Join(homeDir, ".claude", "projects"), nil
}

//go:embed all:web
var webFiles embed.FS

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "generate":
		if err := runGenerate(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	case "serve":
		port := 8080
		if len(os.Args) >= 4 && os.Args[2] == "--port" {
			p, err := strconv.Atoi(os.Args[3])
			if err != nil {
				fmt.Fprintf(os.Stderr, "invalid port: %s\n", os.Args[3])
				os.Exit(1)
			}
			port = p
		}
		if err := runServe(port); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintln(os.Stderr, "Usage: ccasses <command>")
	fmt.Fprintln(os.Stderr, "Commands:")
	fmt.Fprintln(os.Stderr, "  generate             Parse all sessions and output JSON to stdout")
	fmt.Fprintln(os.Stderr, "  serve [--port PORT]  Start HTTP server (default port: 8080)")
}

func runGenerate() error {
	projectsDir, err := claudeProjectsDir()
	if err != nil {
		return err
	}
	summaries, err := parser.ParseAllProjects(projectsDir)
	if err != nil {
		return fmt.Errorf("parse projects: %w", err)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(summaries)
}

func runServe(port int) error {
	// embed.FS から web/ サブディレクトリを取り出す
	webFS, err := fs.Sub(webFiles, "web")
	if err != nil {
		return fmt.Errorf("sub fs: %w", err)
	}

	srv, err := server.New(port, webFS)
	if err != nil {
		return err
	}
	return srv.Run()
}
