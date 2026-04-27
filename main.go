package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cc-switch/internal/config"
	"cc-switch/internal/handlers"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "import-current" {
		if len(os.Args) < 4 {
			fmt.Println("Usage: cc-switch import-current <claude|codex|all> <name>")
			os.Exit(1)
		}
		tool := os.Args[2]
		name := os.Args[3]
		if err := config.EnsureDirs(); err != nil {
			log.Fatal(err)
		}
		if err := config.EnsureClaudeJSON(); err != nil {
			log.Fatal(err)
		}
		store := config.NewStore()
		if err := store.Load(); err != nil {
			log.Fatal(err)
		}
		h := handlers.New(store)
		if err := h.HandleCLIImport(tool, name); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Imported successfully")
		return
	}

	if err := config.EnsureDirs(); err != nil {
		log.Fatalf("ensure dirs: %v", err)
	}
	if err := config.EnsureClaudeJSON(); err != nil {
		log.Fatalf("ensure claude.json: %v", err)
	}

	store := config.NewStore()
	if err := store.Load(); err != nil {
		log.Fatalf("load store: %v", err)
	}
	if err := store.Save(); err != nil {
		log.Fatalf("init store: %v", err)
	}

	h := handlers.New(store)
	mux := http.NewServeMux()
	h.Register(mux)

	// 静态文件与 SPA 回退
	staticDir := "static"
	fsHandler := http.StripPrefix("/ccswitch/", http.FileServer(http.Dir(staticDir)))
	serveIndex := func(w http.ResponseWriter) {
		b, err := os.ReadFile(filepath.Join(staticDir, "index.html"))
		if err != nil {
			http.NotFound(w, nil)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(b)
	}
	mux.HandleFunc("/ccswitch/", func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/ccswitch/")
		if path == "" || path == "/" {
			serveIndex(w)
			return
		}
		// 安全检查：防止目录遍历
		cleanPath := filepath.Clean(path)
		if strings.HasPrefix(cleanPath, "..") || strings.HasPrefix(cleanPath, "/") {
			serveIndex(w)
			return
		}
		fullPath := filepath.Join(staticDir, cleanPath)
		if info, err := os.Stat(fullPath); err == nil && !info.IsDir() {
			fsHandler.ServeHTTP(w, r)
			return
		}
		// 否则回退到 index.html（SPA 路由）
		serveIndex(w)
	})

	addr := "127.0.0.1:18080"
	if env := os.Getenv("CCSWITCH_ADDR"); env != "" {
		addr = env
	}

	displayAddr := addr
	if strings.HasPrefix(addr, ":") {
		displayAddr = "localhost" + addr
	}
	fmt.Printf("cc-switch starting on http://%s/ccswitch/\n", displayAddr)
	if !strings.HasPrefix(addr, "127.0.0.1:") && !strings.HasPrefix(addr, "localhost:") {
		fmt.Println("WARNING: No authentication is enabled. Anyone with network access can read/modify configurations.")
		fmt.Println("Use CCSWITCH_ADDR=127.0.0.1:18080 for local-only access, or protect remote access with a reverse proxy.")
	}
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
