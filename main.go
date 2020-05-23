package main

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
)

// pretty logo :)
const logo = `
______     ______     __   __     __    __     ______     __   __
/\  ___\   /\  __ \   /\ "-.\ \   /\ "-./  \   /\  __ \   /\ "-.\ \
\ \ \____  \ \ \/\ \  \ \ \-.  \  \ \ \-./\ \  \ \ \/\ \  \ \ \-.  \
\ \_____\  \ \_____\  \ \_\\"\_\  \ \_\ \ \_\  \ \_____\  \ \_\\"\_\
\/_____/   \/_____/   \/_/ \/_/   \/_/  \/_/   \/_____/   \/_/ \/_/ `

const version = "v1.1.0"

//////////////////////
// Helper Functions //
//////////////////////

func genServInfo(sysPort uint32) string {
	if sysPort == 22 {
		return "ssh info here"
	}
	return "unsupported service"
}

func swapServices(app *tview.Application, view *View) {
	clearCommand(app, view)
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(view.Connections, 0, 15, true)
	flex.AddItem(view.Status, 1, 0, false)
	view.Flex = flex
	app.SetRoot(view.Flex, true)
	app.SetInputCapture(connectionsInput(app, view))
}

func examineInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		switch key {
		case tcell.KeyEscape:
			swapServices(app, view)
		case tcell.KeyEnter:
			swapServices(app, view)
		case tcell.KeyUp:
		case tcell.KeyDown:
		}
		return event
	}
}


//////////
// Main //
//////////

func main() {

	app := tview.NewApplication()
	defer app.Stop()

	// Clear screen between writes
	app.SetBeforeDrawFunc(func(s tcell.Screen) bool {
		s.Clear()
		return false
	})

	// Build connections
	connections := tview.NewTable()
	connections.SetSelectable(true, true)
	connections.SetSelectedStyle(-1, tcell.ColorGreen, tcell.AttrBold).SetBackgroundColor(-1)
	connections.Select(0, 0)

	// Build status
	status := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	status.SetBackgroundColor(-1)

	// Build flexbox (contains other components)
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(connections, 0, 15, true)
	flex.AddItem(status, 1, 0, false)

	// Compose main View
	mainView := View{
        Flex: flex,
        Connections: connections,
        ServiceList: nil,
        Status: status,
    }
	app.SetInputCapture(connectionsInput(app, &mainView))

	// Spawn goroutine to update
	go updateView(app, &mainView)
	// Run it
	if err := app.SetRoot(mainView.Flex, true).Run(); err != nil {
		panic(err)
	}

}
