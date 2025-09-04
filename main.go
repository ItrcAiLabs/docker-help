package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/docker/docker/api/types"
	containerTypes "github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
)

type ContainerSummary struct {
	ID     string
	Name   string
	Image  string
	State  string
	Status string
	Ports  string
}

type App struct {
	ui         *tview.Application
	table      *tview.Table
	details    *tview.TextView
	logs       *tview.TextView
	statusBar  *tview.TextView
	cli        *client.Client
	containers []ContainerSummary
}

func main() {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Docker client error: %v", err)
	}

	app := &App{
		ui:        tview.NewApplication(),
		table:     tview.NewTable().SetSelectable(true, false),
		details:   tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		logs:      tview.NewTextView().SetDynamicColors(true).SetWrap(true),
		statusBar: tview.NewTextView().SetDynamicColors(true),
		cli:       cli,
	}

	app.setupUI()
	if err := app.refresh(); err != nil {
		app.setStatus("[red]Error loading containers: %v", err)
	}

	if err := app.ui.Run(); err != nil {
		log.Fatalf("UI error: %v", err)
	}
}

func (a *App) setupUI() {
	headers := []string{"Name", "Image", "State", "Status", "Ports"}
	for c, h := range headers {
		cell := tview.NewTableCell("[::b]" + h).
			SetAlign(tview.AlignCenter).
			SetSelectable(false)
		a.table.SetCell(0, c, cell)
	}
	a.table.SetFixed(1, 0)
	a.table.SetBorder(true).SetTitle("Docker Containers")
	a.table.SetSelectedFunc(func(row, column int) {
		a.showForRow(row)
	})
	a.table.SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEscape {
			a.ui.Stop()
		}
	})
	a.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Rune() {
		case 'q', 'Q':
			a.ui.Stop()
			return nil
		case 'r', 'R':
			if err := a.refresh(); err != nil {
				a.setStatus("[red]Refresh error: %v", err)
			}
			return nil
		}
		return event
	})
	a.details.SetBorder(true).SetTitle("Container Details")
	a.logs.SetBorder(true).SetTitle("Last 5 Logs")
	a.statusBar.SetBorder(true).SetTitle("Status")

	left := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.table, 0, 4, true).
		AddItem(a.statusBar, 1, 0, false)
	right := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(a.details, 0, 2, false).
		AddItem(a.logs, 0, 1, false)
	root := tview.NewFlex().SetDirection(tview.FlexColumn).
		AddItem(left, 0, 3, true).
		AddItem(right, 0, 4, false)
	a.ui.SetRoot(root, true).EnableMouse(true)
}

func (a *App) setStatus(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	a.statusBar.SetText(time.Now().Format("15:04:05 ") + msg)
}

func (a *App) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	containers, err := a.cli.ContainerList(ctx, containerTypes.ListOptions{All: true})
	if err != nil {
		return err
	}
	sort.SliceStable(containers, func(i, j int) bool {
		ci, cj := containers[i], containers[j]
		if ci.State != cj.State {
			return ci.State == "running"
		}
		return firstName(ci.Names) < firstName(cj.Names)
	})
	a.containers = make([]ContainerSummary, 0, len(containers))
	a.table.Clear()
	headers := []string{"Name", "Image", "State", "Status", "Ports"}
	for c, h := range headers {
		a.table.SetCell(0, c, tview.NewTableCell("[::b]"+h).SetAlign(tview.AlignCenter).SetSelectable(false))
	}
	for r, ctn := range containers {
		row := r + 1
		name := firstName(ctn.Names)
		ports := formatPorts(ctn.Ports)
		summary := ContainerSummary{
			ID:     ctn.ID,
			Name:   name,
			Image:  ctn.Image,
			State:  ctn.State,
			Status: ctn.Status,
			Ports:  ports,
		}
		a.containers = append(a.containers, summary)
		a.table.SetCell(row, 0, cell(name))
		a.table.SetCell(row, 1, cell(ctn.Image))
		a.table.SetCell(row, 2, cell(summary.State))
		a.table.SetCell(row, 3, cell(summary.Status))
		a.table.SetCell(row, 4, cell(summary.Ports))
	}
	a.setStatus("[green]Refreshed. %d containers.", len(a.containers))
	if len(a.containers) > 0 {
		a.table.Select(1, 0)
		a.showForRow(1)
	} else {
		a.details.SetText("No containers found.")
		a.logs.SetText("")
	}
	return nil
}

func (a *App) showForRow(row int) {
	idx := row - 1
	if idx < 0 || idx >= len(a.containers) {
		return
	}
	c := a.containers[idx]
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ins, err := a.cli.ContainerInspect(ctx, c.ID)
	if err != nil {
		a.details.SetText(fmt.Sprintf("[red]Inspect error: %v", err))
	} else {
		started := parseTime(ins.State.StartedAt)
		var b strings.Builder
		fmt.Fprintf(&b, "[::b]Name:[-] %s\n", c.Name)
		fmt.Fprintf(&b, "[::b]ID:[-] %s\n", c.ID)
		fmt.Fprintf(&b, "[::b]Image:[-] %s\n", c.Image)
		fmt.Fprintf(&b, "[::b]State:[-] %s\n", ins.State.Status)
		fmt.Fprintf(&b, "[::b]Status:[-] %s\n", c.Status)
		if !started.IsZero() {
			fmt.Fprintf(&b, "[::b]Started:[-] %s (%s ago)\n", started.Local().Format(time.RFC1123), since(started))
		}
		a.details.SetText(b.String())
	}
	logs, err := a.fetchLogs(c.ID, 5)
	if err != nil {
		a.logs.SetText(fmt.Sprintf("[red]Logs error: %v", err))
	} else {
		if strings.TrimSpace(logs) == "" {
			logs = "(no logs)"
		}
		a.logs.SetText(logs)
	}
}
func (a *App) fetchLogs(containerID string, n int) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	rdr, err := a.cli.ContainerLogs(ctx, containerID, containerTypes.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprint(n),
		Timestamps: true,
	})
	if err != nil {
		return "", err
	}
	defer rdr.Close()
	b, err := io.ReadAll(rdr)
	if err != nil {
		return "", err
	}
	s := strings.ReplaceAll(string(b), "\r", "")
	return strings.TrimSpace(s), nil
}


func cell(text string) *tview.TableCell {
	return tview.NewTableCell(text).SetMaxWidth(0).SetExpansion(1)
}

func firstName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

func formatPorts(ports []types.Port) string {
	if len(ports) == 0 {
		return "-"
	}
	var out []string
	for _, p := range ports {
		if p.PublicPort > 0 {
			out = append(out, fmt.Sprintf("%s:%d->%d/%s", nz(p.IP, "0.0.0.0"), p.PublicPort, p.PrivatePort, strings.ToLower(p.Type)))
		} else {
			out = append(out, fmt.Sprintf("%d/%s", p.PrivatePort, strings.ToLower(p.Type)))
		}
	}
	return strings.Join(out, ", ")
}

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func since(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%d seconds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%d minutes", int(d.Minutes()))
	} else if d < 24*time.Hour {
		return fmt.Sprintf("%d hours", int(d.Hours()))
	}
	return fmt.Sprintf("%d days", int(d.Hours()/24))
}

func nz(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
