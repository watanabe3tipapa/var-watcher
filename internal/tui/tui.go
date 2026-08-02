package tui

import (
	"fmt"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/watanabe3tipapa/var-watcher/internal/engine"
)

const maxLineLen = 2048

func Run(e *engine.Engine, prereq []engine.Prerequisite, cfgPath string) error {
	app := tview.NewApplication()
	maxLines := e.MaxLogLines()

	logView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true).
		SetWordWrap(true).
		SetChangedFunc(func() { app.Draw() })
	logView.SetBorder(true).SetTitle("logs")

	header := tview.NewTextView().SetDynamicColors(true)
	header.SetTextAlign(tview.AlignCenter)

	list := tview.NewList().ShowSecondaryText(false)
	list.SetBorder(true).SetTitle("watchers")
	list.SetSelectedFunc(func(i int, _, _ string, _ rune) {
		toggle(app, e, list, header)
	})

	filter := tview.NewInputField().
		SetLabel("/ filter: ").
		SetFieldWidth(30)

	var warnText []string
	for _, p := range prereq {
		if !p.OK {
			warnText = append(warnText, "[red::b]!"+p.Message)
		}
	}
	warning := tview.NewTextView().SetDynamicColors(true)
	if len(warnText) > 0 {
		warning.SetText(strings.Join(warnText, "\n"))
		warning.SetBorder(true).SetTitle("warnings")
	}

	layout := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(header, 3, 0, false).
		AddItem(list, 5, 0, true)
	if len(warnText) > 0 {
		layout.AddItem(warning, len(warnText)+2, 0, false)
	}
	layout.AddItem(logView, 0, 1, false).
		AddItem(filter, 1, 0, false)

	app.SetRoot(layout, true)

	names := watcherNames(e)
	refreshHeader(header, e)
	refreshList(list, e, names)

	logCh, unsub := e.Logs()
	defer unsub()
	ring := engine.NewRing(maxLines)
	filterText := ""

	appendLog := func(entry string) {
		ring.Add(entry)
		if filterText == "" {
			fmt.Fprintln(logView, entry)
		} else if strings.Contains(strings.ToLower(entry), strings.ToLower(filterText)) {
			app.Draw()
		}
	}
	go func() {
		for line := range logCh {
			msg := line.Message
			if len(msg) > maxLineLen {
				msg = msg[:maxLineLen]
			}
			entry := fmt.Sprintf("[cyan]%s[white] [yellow]%s[white] %s",
				line.TS.Format("15:04:05"), line.Source, msg)
			app.QueueUpdateDraw(func() { appendLog(entry) })
		}
	}()

	applyFilter := func(t string) {
		filterText = t
		logView.Clear()
		if t == "" {
			for _, ln := range ring.All() {
				fmt.Fprintln(logView, ln)
			}
		} else {
			lt := strings.ToLower(t)
			for _, ln := range ring.All() {
				if strings.Contains(strings.ToLower(ln), lt) {
					fmt.Fprintln(logView, ln)
				}
			}
		}
		app.Draw()
	}

	app.SetInputCapture(func(ev *tcell.EventKey) *tcell.EventKey {
		switch ev.Key() {
		case tcell.KeyCtrlC:
			app.Stop()
			return nil
		case tcell.KeyEscape:
			if filterText != "" {
				filter.SetText("")
				applyFilter("")
				app.SetFocus(list)
				return nil
			}
		}
		// フィルタ入力中は英字・数字をそのまま入力に渡す
		if app.GetFocus() == filter {
			return ev
		}
		switch ev.Rune() {
		case 'q':
			app.Stop()
		case ' ':
			toggle(app, e, list, header)
		case '/':
			app.SetFocus(filter)
		case 'f':
			filter.SetText("")
			applyFilter("")
			app.SetFocus(list)
		case 's':
			if err := e.SaveConfig(cfgPath); err != nil {
				fmt.Fprintln(logView, "[red]config save error: "+err.Error())
			} else {
				fmt.Fprintln(logView, "[green]config saved to "+cfgPath)
			}
		case 'n':
			if err := e.Notify("var-watcher notification test"); err != nil {
				fmt.Fprintln(logView, "[red]notify error: "+err.Error())
			}
		}
		return ev
	})

	filter.SetChangedFunc(func(text string) { applyFilter(text) })
	filter.SetDoneFunc(func(key tcell.Key) { app.SetFocus(list) })

	return app.Run()
}

func watcherNames(e *engine.Engine) []string {
	info := e.List()
	out := make([]string, 0, len(info))
	for _, i := range info {
		out = append(out, i.Name)
	}
	return out
}

func toggle(app *tview.Application, e *engine.Engine, list *tview.List, header *tview.TextView) {
	idx := list.GetCurrentItem()
	names := watcherNames(e)
	if idx < 0 || idx >= len(names) {
		return
	}
	target := names[idx]
	enabled := false
	for _, i := range e.List() {
		if i.Name == target {
			enabled = i.Enabled
			break
		}
	}
	if enabled {
		_ = e.Stop(target)
	} else if err := e.Start(target); err != nil {
		fmt.Fprintln(header, "[red]failed to start "+target+": "+err.Error())
	}
	refreshList(list, e, names)
	refreshHeader(header, e)
}

func refreshList(list *tview.List, e *engine.Engine, names []string) {
	list.Clear()
	for _, name := range names {
		var info engine.WatcherInfo
		for _, i := range e.List() {
			if i.Name == name {
				info = i
				break
			}
		}
		status := "[gray]off"
		switch info.State {
		case "running":
			status = "[green]ON"
		case "error":
			status = "[red]ERR"
		}
		extra := ""
		if !info.Installed {
			extra = " (missing)"
		}
		list.AddItem(fmt.Sprintf("%s  %s%s", name, status, extra),
			info.Command+" "+strings.Join(info.Args, " "), 0, nil)
	}
}

func refreshHeader(h *tview.TextView, e *engine.Engine) {
	info := e.List()
	on := 0
	for _, i := range info {
		if i.Enabled {
			on++
		}
	}
	h.SetText(fmt.Sprintf("[blue::b]var-watcher[white] target=[yellow]%s[white] running=%d/%d",
		e.Target(), on, len(info)))
}
