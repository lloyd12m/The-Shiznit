package ui

import (
	"automg-go/internal/insta"
	"fmt"
	"time"

	"github.com/rivo/tview"
)

type Dashboard struct {
	App              *tview.Application
	Stats            *insta.Stats
	Logs             *tview.TextView
	CheckerStatsView *tview.TextView
	ClaimerStatsView *tview.TextView
	Banner           *tview.TextView
	Root             tview.Primitive
	StartTime        time.Time
}

func NewDashboard(stats *insta.Stats) *Dashboard {
	app := tview.NewApplication()

	banner := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("[yellow]\n+-+-+-+ +-+-+-+-+-+-+-+\n|T|h|e| |S|h|i|z|n|i|t|\n+-+-+-+ +-+-+-+-+-+-+-+")
	banner.SetBorder(false)

	checkerStatsView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("Initializing Checker...")
	checkerStatsView.SetBorder(true).SetTitle(" Checker Statistics ")

	claimerStatsView := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignCenter).
		SetText("Initializing Claimer...")
	claimerStatsView.SetBorder(true).SetTitle(" Claimer Statistics ")

	logs := tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWordWrap(true).
		SetChangedFunc(func() { app.Draw() })
	logs.SetBorder(true).SetTitle(" Activity Logs ")

	return &Dashboard{
		App:              app,
		Stats:            stats,
		Logs:             logs,
		CheckerStatsView: checkerStatsView,
		ClaimerStatsView: claimerStatsView,
		Banner:           banner,
		StartTime:        time.Now(),
	}
}

func (d *Dashboard) Log(message string, color string) {
	timestamp := time.Now().Format("15:04:05")
	fmt.Fprintf(d.Logs, "[%s][%s] %s[-]\n", color, timestamp, message)
}

// ShowAlert displays a native Windows MessageBox without stopping background work.
func (d *Dashboard) ShowAlert(title, message string) {
	ShowNativeMessageBox(title, message)
}

// ShowAlertAndStop displays a native MessageBox and stops the dashboard after OK.
func (d *Dashboard) ShowAlertAndStop(title, message string) {
	ShowNativeMessageBox(title, message)
	d.App.Stop()
}

// ShowStandaloneAlert displays a native MessageBox before the dashboard starts.
func ShowStandaloneAlert(title, message string) {
	ShowNativeMessageBox(title, message)
}

func (d *Dashboard) UpdateStats() {
	for {
		d.Stats.Checker.Lock.Lock()
		checked := d.Stats.Checker.Checked
		checkerRPS := d.Stats.Checker.WindowChecked
		checkerErrors := d.Stats.Checker.Errors
		d.Stats.Checker.WindowChecked = 0
		d.Stats.Checker.Lock.Unlock()

		// R/S is the number of completed requests in the previous one-second window.
		// This matches the metric used by the public C++ project.
		d.Stats.Claimer.Lock.Lock()
		attempts := d.Stats.Claimer.Attempts
		claimerErrors := d.Stats.Claimer.Errors
		success := d.Stats.Claimer.Success
		rps := d.Stats.Claimer.WindowAttempts
		d.Stats.Claimer.WindowAttempts = 0
		d.Stats.Claimer.Lock.Unlock()

		checkerText := fmt.Sprintf(
			"[cyan]User Checked: [white]%-8d  |  [blue]Checker R/S: [white]%-8d  |  [red]Errors: [white]%-8d",
			checked, checkerRPS, checkerErrors,
		)

		claimerText := fmt.Sprintf(
			"[yellow]Attempts: [white]%d  |  [red]Errors: [white]%d  |  [green]Success: [white]%d  |  [blue]R/S: [white]%d",
			attempts, claimerErrors, success, rps,
		)

		d.App.QueueUpdateDraw(func() {
			d.CheckerStatsView.SetText(checkerText)
			d.ClaimerStatsView.SetText(claimerText)
		})

		time.Sleep(1 * time.Second)
	}
}

func (d *Dashboard) Run() error {
	go d.UpdateStats()

	flex := tview.NewFlex().SetDirection(tview.FlexRow).
		AddItem(d.Banner, 6, 1, false).
		AddItem(d.CheckerStatsView, 3, 1, false).
		AddItem(d.ClaimerStatsView, 3, 1, false).
		AddItem(d.Logs, 0, 1, true)

	d.Root = flex
	return d.App.SetRoot(d.Root, true).Run()
}
