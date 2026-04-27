package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/0xnu/mcp-core/internal/config"
	"github.com/0xnu/mcp-core/internal/core"
	"github.com/0xnu/mcp-core/internal/registry"
)

var (
	configPath  = flag.String("config", "", "path to config file")
	showVersion = flag.Bool("version", false, "show version")
)

var Version = "0.1.0-dev"

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("mcp-core %s\n", Version)
		os.Exit(0)
	}

	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	reg := registry.NewRegistry(cfg)
	router := core.NewRouter(reg)
	server := core.NewServer(cfg, router)

	server.SetReloadHandler(func(cfg *config.Config) error {
		return reg.Reload(cfg)
	})

	lifecycle := core.NewLifecycle(server, router, reg)

	log.Printf("mcp-core %s starting on %s", Version, cfg.ListenAddr())

	if err := lifecycle.Run(); err != nil {
		log.Fatalf("fatal error: %v", err)
	}
}

func loadConfig() (*config.Config, error) {
	if *configPath != "" {
		loader := config.NewLoader(*configPath)
		return loader.Load()
	}

	loader := config.NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		cfg = config.ScaffoldConfig()
		path := config.DefaultConfigPath()

		if err := cfg.Save(path); err != nil {
			log.Printf("warning: could not save default config: %v", err)
		}

		log.Printf("no config found, using defaults (saved to %s)", path)
	}

	return cfg, nil
}

func init() {
	_ = context.Background()
}
