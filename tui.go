package main

import (
	"fmt"
	"github.com/gdamore/tcell"

	"github.com/rivo/tview"
	"sort"
	"strconv"
	"time"
)

const refreshRate = 100 * time.Millisecond

var connTable map[int]*Conn = make(map[int]*Conn) // keep track of tui display of connections

type View struct {
	Flex           *tview.Flex
	Connections    *tview.Table
	ServiceList    []Service
	Status         *tview.TextView
	CurrentCommand string
	CurrentMove    int
}

//////////////////////
// Helper Functions //
//////////////////////

// Returns index of service in serviceList and Conn currently selected
func cellInfo(view *View) (int, int) {
	connTotal := 0
	r, _ := view.Connections.GetSelection()
	for s := 0; s < len(view.ServiceList); s++ {
		if r >= connTotal && r <= connTotal+len(view.ServiceList[s].Conns) {
			return s, r - connTotal - 1
		}
		connTotal += len(view.ServiceList[s].Conns) + 1
	}
	return -1, -1
}

// Replaces statusbar with text input and returns string given
func takeInput(app *tview.Application, view *View, msg string, str *string, appe int) {
	inputField := tview.NewInputField().
		SetLabel(msg).
		SetFieldWidth(30)
	inputField.SetLabelColor(tcell.ColorWhite).
		SetFieldBackgroundColor(-1).
		SetBackgroundColor(-1)
	inputField.SetDoneFunc(func(key tcell.Key) {
		if appe == 1 {
		} else {
			*str = inputField.GetText()
		}
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(inputField, 1, 0, true)
	view.Flex = flex
	app.SetInputCapture(nullInput(app, view))
	app.SetRoot(flex, true)
}

// Ignores all keyboard input
func nullInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		return event
	}
}

// Clears current command/movement buffer
func clearCommand(app *tview.Application, view *View) {
	view.CurrentCommand = ""
	view.CurrentMove = 1
	view.Status.SetText("")
}

// Function to update current view forever
func updateView(app *tview.Application, view *View) {
	for {
		app.QueueUpdateDraw(func() {
			view.genConnections()
		})
		time.Sleep(refreshRate)
	}
}

// Examine connections based on current selection
func examineConn(app *tview.Application, view *View, conn *Conn) {
	sep := tview.NewBox()
	sep.SetBackgroundColor(tcell.ColorGreen)
	examineBox := tview.NewTable()
	examineBox.SetBackgroundColor(-1)
	// check which service this is running over
	// then switch statement for collecting info
	examineBox.SetCell(0, 0, tview.NewTableCell(strconv.Itoa(int(conn.LocalPort))))

	// Build and replace flex
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(view.Connections, 0, 8, false)
	flex.AddItem(sep, 1, 0, false)
	flex.AddItem(examineBox, 0, 8, true)
	flex.AddItem(view.Status, 1, 0, false)
	view.Flex = flex
	app.SetRoot(view.Flex, true)
	view.CurrentCommand = "examine"
	app.SetInputCapture(examineInput(app, view))
}

func outputConnections(view *View, currentConns map[uint32]map[int32]*Conn) {
	if len(currentConns) == 0 {
		view.Connections.SetSelectable(false, false)
		view.Connections.Select(0, 0)
		view.Connections.SetCell(1, 0, tview.NewTableCell("No established connections found.").SetAlign(tview.AlignCenter).SetExpansion(1))
		view.Connections.SetCell(2, 0, tview.NewTableCell("[::b]conmon "+version+"[::]").SetAlign(tview.AlignCenter).SetExpansion(1))
		if len(listeningPorts) != 0 {
			view.Connections.SetCell(5, 0, tview.NewTableCell(fmt.Sprintf("[::b]Listening ports:[::-]")).SetAlign(tview.AlignCenter))
			for index, conn := range listeningPorts {
				view.Connections.SetCell(index+7, 0, tview.NewTableCell(fmt.Sprintf("%s:[::b]%d[::-]", conn.Laddr.IP, conn.Laddr.Port)).SetAlign(tview.AlignCenter))
			}
		}
	} else {
		view.Connections.SetSelectable(true, false) // Select rows only
		connNum := 0
		rowNum := 0
		portList := []uint32{}

		for sysPort, _ := range currentConns {
			portList = append(portList, sysPort)
		}

		sort.Slice(portList, func(i, j int) bool {
			return portList[i] < portList[j]
		})

		for _, sysPort := range portList {

			connList := []*Conn{}
			for _, conn := range currentConns[sysPort] {
				connList = append(connList, conn)
			}
			sort.Slice(connList, func(i, j int) bool {
				return connList[i].Pid < connList[j].Pid
			})
			// Output port header
			// if service in map
			// add service name
			// map[int32]func (handler func for data + examining? interface?)

			// Output each connection for that port
			for _, conn := range connList {
				numAndPort := fmt.Sprintf(" %d | [::b]%d[::-] --> %d", connNum, conn.LocalPort, conn.RemotePort)
				sourceAddr := fmt.Sprintf("| %s", conn.LocalIp)
				var pidInfo string
				if conn.Meta == "(dead)" {
					pidInfo = "| [palevioletred:white:r] " + strconv.Itoa(int(conn.Pid)) + " [-:-:-] |"
				} else {
					pidInfo = "| [white:black:r] " + strconv.Itoa(int(conn.Pid)) + " [-:-:-] |"
				}
				view.Connections.SetCell(rowNum, 0, tview.NewTableCell(numAndPort).SetReference(conn))
				view.Connections.SetCell(rowNum, 1, tview.NewTableCell(sourceAddr).SetReference(conn))
				view.Connections.SetCell(rowNum, 2, tview.NewTableCell(pidInfo)).
					SetCell(rowNum, 3, tview.NewTableCell(conn.Info)).
					SetCell(rowNum, 4, tview.NewTableCell(conn.Meta).SetExpansion(1))

				connNum++
				rowNum++

			}
		}

	}
}

func connectionsInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		var conn *Conn
		cellRef := (view.Connections.GetCell(view.Connections.GetSelection())).
			GetReference()
		if cellRef != nil {
			conn = cellRef.(*Conn)
		}
		key := event.Key()
		switch key {
		case tcell.KeyEscape:
			switch view.CurrentCommand {
			case "examine":
				swapServices(app, view)
			default:
				app.Stop()
			}
		// Enter will examine connection
		case tcell.KeyEnter:
			if conn != nil {
				examineConn(app, view, conn)
			}
		case tcell.KeyRune:
			switch event.Rune() {
			// Kill command
			case 'd':
				if conn != nil {
					killProcess(conn.Pid)
					view.Status.SetText(fmt.Sprintf("killed %d", conn.Pid))
				}
			// Killswitch (cut the cord) (blocks all in firewall)
			case 'c':
				view.Status.SetText("cut the cord?")
			// Block IP
			case 'b':
				switch view.CurrentCommand {
				case "block":
					view.Status.SetText("blocked")
					// block cmd here (add to fw)
				default:
					s, i := cellInfo(view)
					if i != -1 {
						view.Status.SetText("block " + view.ServiceList[s].Conns[i].LocalIp + "?")
						view.CurrentCommand = "block"
					} else {
						view.Status.SetText("block all connections?")
					}
				}
			// Movement command (i.e. b5r5m will block 5 rows for 5 mins)
			case 'r':
				switch view.CurrentCommand {
				case "block":
					switch view.CurrentMove {
					case 0:
						//print("blocking x rows")
					}
				}
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				cm := strconv.Itoa(view.CurrentMove) + string(event.Rune())
				view.CurrentMove, _ = strconv.Atoi(cm)
				// make currentmove -1 when its new
			}
		}
		return event
	}
}
