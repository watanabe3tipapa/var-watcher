package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/watanabe3tipapa/var-watcher/internal/alert"
	"github.com/watanabe3tipapa/var-watcher/internal/config"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
	"github.com/watanabe3tipapa/var-watcher/internal/notify"
	"github.com/watanabe3tipapa/var-watcher/internal/plugin"
	"github.com/watanabe3tipapa/var-watcher/internal/store"
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

	var st *store.Store
	if st, err = store.Open(cfg.ResolveDbPath()); err != nil {
		fmt.Fprintf(os.Stderr, "log persistence disabled: %v\n", err)
		st = nil
	} else {
		if n, perr := st.Prune(time.Duration(cfg.RetentionDays) * 24 * time.Hour); perr == nil && n > 0 {
			fmt.Printf("cleaned %d persisted log(s) (retention %dd)\n", n, cfg.RetentionDays)
		}
		go persistLogs(bus, st)
		fmt.Printf("log persistence: %s\n", cfg.ResolveDbPath())
	}

	if ws, err := plugin.Discover(*pluginDir, bus); err != nil {
		fmt.Fprintf(os.Stderr, "plugin scan: %v\n", err)
	} else {
		for _, w := range ws {
			if e.AddPlugin(w) {
				fmt.Printf("plugin loaded: %s\n", w.Name)
			}
		}
	}

	am := alert.NewManager(cfg.Alerts, bus)
	am.OnFire(func(f alert.Fired, r alert.Rule) {
		msg := fmt.Sprintf("alert: %s (%d events)", f.Name, f.Count)
		if r.SoundName != "" {
			_ = notify.Sound(r.SoundName, msg)
		} else if cfg.Notify {
			_ = notify.Show(msg)
		}
		bus.Publish(engine.LogLine{Source: "alert", Level: "warn", Message: msg + " — " + f.Last})
	})
	go am.Run()

	prereq := engine.CheckPrerequisites(e.Watchers())

	if *webMode {
		srv := web.NewServer(e)
		srv.SetStore(st)
		srv.SetAlerts(am)
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

// persistLogs は bus を購読して SQLite に書き込む。書き込み失敗は回数上限付きで通知する。
func persistLogs(bus *engine.Bus, st *store.Store) {
	ch, unsub := bus.Subscribe()
	defer unsub()
	errs := 0
	for line := range ch {
		if err := st.Append(store.Line{
			TS: line.TS, Source: line.Source, Level: line.Level, Message: line.Message,
		}); err != nil {
			errs++
			if errs <= 3 || errs%100 == 0 {
				fmt.Fprintf(os.Stderr, "persist error: %v\n", err)
			}
		}
	}
}
