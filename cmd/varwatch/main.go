package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
	"github.com/watanabe3tipapa/var-watcher/internal/plugin"
	"github.com/watanabe3tipapa/var-watcher/internal/tui"
	"github.com/watanabe3tipapa/var-watcher/internal/web"
)

func main() {
	var (
		tuiMode   = flag.Bool("tui", false, "run terminal UI")
		webMode   = flag.Bool("web", false, "run web UI")
		addr      = flag.String("addr", ":8080", "web UI listen address")
		cfgPath   = flag.String("config", "", "config file (default ~/.varwatch/config.json)")
		pluginDir = flag.String("plugins", "plugins", "plugins directory")
	)
	flag.Parse()

	if !*tuiMode && !*webMode {
		*tuiMode = true
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config load: %v", err)
	}
	if *cfgPath == "" {
		*cfgPath = config.DefaultPath()
	}

	bus := engine.NewBus()
	e := engine.New(cfg, bus)

	if ws, err := plugin.Discover(*pluginDir, bus); err != nil {
		fmt.Fprintf(os.Stderr, "plugin scan: %v\n", err)
	} else {
		for _, w := range ws {
			if e.AddPlugin(w) {
				fmt.Printf("plugin loaded: %s\n", w.Name)
			}
		}
	}

	prereq := engine.CheckPrerequisites(e.Watchers())

	if *webMode {
		srv := web.NewServer(e)
		srv.UseEmbedded()
		go func() {
			fmt.Printf("web UI: http://localhost%s\n", *addr)
			if err := srv.Run(*addr); err != nil {
				log.Printf("web server: %v", err)
			}
		}()
	}

	if *tuiMode {
		if err := tui.Run(e, prereq, *cfgPath); err != nil {
			log.Fatalf("tui: %v", err)
		}
	} else {
		select {}
	}
}
