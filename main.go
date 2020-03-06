package main

import (
	"github.com/gdamore/tcell"
	"github.com/rivo/tview"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
	"os"
	"os/exec"
	"strconv"
	"time"
)

//////////////////////////////
// Constants and structures //
//////////////////////////////

const logo = `
______     ______     __   __     __    __     ______     __   __    
/\  ___\   /\  __ \   /\ "-.\ \   /\ "-./  \   /\  __ \   /\ "-.\ \   
\ \ \____  \ \ \/\ \  \ \ \-.  \  \ \ \-./\ \  \ \ \/\ \  \ \ \-.  \  
\ \_____\  \ \_____\  \ \_\\"\_\  \ \_\ \ \_\  \ \_____\  \ \_\\"\_\ 
\/_____/   \/_____/   \/_/ \/_/   \/_/  \/_/   \/_____/   \/_/ \/_/ `

const timerDefault = 30
const refreshRate = 500 * time.Millisecond

type View struct {
	header         *tview.TextView
	flex           *tview.Flex
	services       *tview.Table
	serviceList    []Service
	firewall       Firewall
	settings       *tview.Flex
	status         *tview.TextView
	nav            *tview.Table
	currentCommand string
	currentMove    int
}

type Service struct {
	name  string
	port  uint32
	conns []Conn
}

type Conn struct {
	num   int
	ip    string
	port  uint32
	pid   int32
	info  string
	timer int
}

type Firewall struct {
	fwPane      *tview.Flex
	fwCmd       string
	fwStatus    *tview.TextView
	inputRules  []string
	outputRules []string
	hash        string
}

//////////////////////
// Helper Functions //
//////////////////////

// Function to update current view forever
func updateView(app *tview.Application, view *View) {
	for {
		app.QueueUpdateDraw(func() {
			view.genServices()
		})
		time.Sleep(refreshRate)
	}
}

// Returns index of service in serviceList and Conn currently selected
func cellInfo(view *View) (int, int) {
	connTotal := 0
	r, _ := view.services.GetSelection()
	for s := 0; s < len(view.serviceList); s++ {
		if r >= connTotal && r <= connTotal+len(view.serviceList[s].conns) {
			return s, r - connTotal - 1
		}
		connTotal += len(view.serviceList[s].conns) + 1
	}
	return -1, -1
}

// Sets statusbar to given text
func alertInfo(app *tview.Application, view *View, msg string) {
	app.QueueUpdateDraw(func() {
		view.status.SetText(msg)
	})
}

// Replaces statusbar with text input and returns string given
func takeInput(app *tview.Application, view *View, msg string, str *string, appe int) {
	clearCommand(app, view)
	inputField := tview.NewInputField().
		SetLabel(msg).
		SetFieldWidth(30)
	inputField.SetLabelColor(tcell.ColorWhite).
		SetFieldBackgroundColor(-1).
		SetBackgroundColor(-1)
	inputField.SetDoneFunc(func(key tcell.Key) {
		if appe == 1 {
			view.firewall.inputRules = append(view.firewall.inputRules, inputField.GetText())
		} else {
			*str = inputField.GetText()
		}
		swapFirewall(app, view)
	})
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(view.header, 1, 0, false)
	flex.AddItem(view.firewall.fwPane, 0, 15, false)
	flex.AddItem(inputField, 1, 0, true)
	flex.AddItem(view.nav, 1, 0, false)
	view.flex = flex
	app.SetInputCapture(nullInput(app, view))
	app.SetRoot(flex, true)
}

// Ignores all keyboard input
func nullInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		return event
	}
}

// Changes text/color styling of given row with given diff offset
func highlight(table *tview.Table, r int, diff int) {
	for c := 0; c < table.GetColumnCount(); c++ {
		table.GetCell(r, c).SetBackgroundColor(-1)
		table.GetCell(r+diff, c).SetBackgroundColor(tcell.ColorGreen)
	}
}

// Clears current command/movement buffer
func clearCommand(app *tview.Application, view *View) {
	view.currentCommand = ""
	view.currentMove = 1
	alertInfo(app, view, "")
}

// Move over titles and control highlighting of list
func move(view *View, d int) {
	r, _ := view.services.GetSelection()
	maxRows := view.services.GetRowCount()
	if (r+d >= 0 && d == -1) || (r+d < maxRows && d == 1) {
		highlight(view.services, r, d)
	}
}

// Kills process and all its children
func kill(app *tview.Application, view *View, pid int32) {
	proc, err := process.NewProcess(pid)
	if err == nil {
		children, err := proc.Children()
		if err == nil {
			for z := 0; z < len(children); z++ {
				children[z].SendSignal(15)
			}
			proc.SendSignal(15)
			alertInfo(app, view, "killed process "+strconv.Itoa(int(pid)))
		} else {
			alertInfo(app, view, "error killing child")
		}
	} else {
		alertInfo(app, view, "error killing proccess")
	}
}

///////////////////
// Services Pane //
///////////////////

func genServInfo(service Service, conn net.ConnectionStat) string {
	if service.name == "ssh" {
		return "ssh info here"
	}
	return "unsupported service"
}

func swapServices(app *tview.Application, view *View) {
	clearCommand(app, view)
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	view.header.SetText("Services")
	flex.AddItem(view.header, 1, 0, false)
	flex.AddItem(view.services, 0, 15, true)
	flex.AddItem(view.status, 1, 0, false)
	flex.AddItem(view.nav, 1, 0, false)
	view.flex = flex
	app.SetRoot(view.flex, true)
	app.SetInputCapture(servicesInput(app, view))
}

func (view *View) genServices() {
	// Get current connections
	cs, err := net.Connections("all")
	if err != nil {
		panic(err)
	}
	rows := 0
	conNum := 0
	matchFound := false
	view.services.Clear()
	r, _ := view.services.GetSelection()
	for s := 0; s < len(view.serviceList); s++ {
		serviceColor := tcell.ColorRed
		currentConns := []Conn{}
		// TODO add support for other servicemanagers
		cmd := exec.Command("systemctl", "check", view.serviceList[s].name)
		out, err := cmd.CombinedOutput()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !(ok) {
				print("failed to run systemctl")
				os.Exit(1)
			}
		}
		if string(out) == "active\n" {
			serviceColor = tcell.ColorLightGreen
		}
		view.services.SetCell(rows, 0, tview.NewTableCell(view.serviceList[s].name).
			SetTextColor(serviceColor)).
			SetCell(rows, 1, tview.NewTableCell("port "+strconv.Itoa(int(view.serviceList[s].port))))
		rows++

		// Build current connections
		for c := 0; c < len(cs); c++ {
			if cs[c].Laddr.Port == view.serviceList[s].port && cs[c].Status == "ESTABLISHED" {
				// Locate old connection
				matchFound = false
				for z := 0; z < len(view.serviceList[s].conns); z++ {
					if cs[c].Pid == view.serviceList[s].conns[z].pid {
						oldConn := Conn{conNum,
							cs[c].Laddr.IP,
							cs[c].Laddr.Port,
							cs[c].Pid,
							genServInfo(view.serviceList[s], cs[c]),
							timerDefault}
						conNum++
						matchFound = true
						currentConns = append(currentConns, oldConn)
					}
				}
				// Find new connection
				if !matchFound {
					newConn := Conn{
						conNum,
						cs[c].Laddr.IP,
						cs[c].Laddr.Port,
						cs[c].Pid,
						genServInfo(view.serviceList[s], cs[c]),
						timerDefault}
					currentConns = append(currentConns, newConn)
					conNum++
				}
			}
		}

		// Countdown timers or remove dead conns
		for z := 0; z < len(view.serviceList[s].conns); z++ {
			matchFound = false
			for c := 0; c < len(cs); c++ {
				if cs[c].Laddr.Port == view.serviceList[s].port && cs[c].Status == "ESTABLISHED" {
					if view.serviceList[s].conns[z].pid == cs[c].Pid {
						matchFound = true
						break
					}
				}
			}
			if !matchFound {
				if view.serviceList[s].conns[z].timer > 0 {
					oldConn := Conn{
						conNum,
						view.serviceList[s].conns[z].ip,
						view.serviceList[s].conns[z].port,
						view.serviceList[s].conns[z].pid,
						"dead",
						view.serviceList[s].conns[z].timer - 1}
					currentConns = append(currentConns, oldConn)
					conNum++
				} else {
					// Move up if connection dies
					// not entirely sure this works
					if r == z+s+1 {
						move(view, 1)
					}
				}
			}
		}

		view.serviceList[s].conns = currentConns
		// Output conns
		for c := 0; c < len(currentConns); c++ {
			view.services.SetCell(rows, 0, tview.NewTableCell(strconv.Itoa(view.serviceList[s].conns[c].num)).SetAlign(tview.AlignRight)).
				SetCell(rows, 1, tview.NewTableCell(view.serviceList[s].conns[c].ip)).
				SetCell(rows, 2, tview.NewTableCell("(pid "+strconv.Itoa(int(view.serviceList[s].conns[c].pid))+")")).
				SetCell(rows, 3, tview.NewTableCell(view.serviceList[s].conns[c].info))
			rows++
		}

	}
	highlight(view.services, r, 0)
}

func servicesInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		switch key {
		case tcell.KeyEscape:
			switch view.currentCommand {
			case "examine":
				swapServices(app, view)
			default:
				app.Stop()
			}
		// Enter will examine connection
		case tcell.KeyEnter:
			clearCommand(app, view)
			alertInfo(app, view, "Examining")
			examineConn(app, view)
		case tcell.KeyUp:
			print(view.currentMove)
			for i := 0; i < view.currentMove; i++ {
				move(view, -1)
			}
			clearCommand(app, view)
		case tcell.KeyDown:
			move(view, 1)
			clearCommand(app, view)
		case tcell.KeyLeft:
			r, _ := view.services.GetSelection()
			view.services.Select(r, 0)
			clearCommand(app, view)
		case tcell.KeyRight:
			r, _ := view.services.GetSelection()
			view.services.Select(r, -1)
			clearCommand(app, view)
		case tcell.KeyRune:
			switch event.Rune() {
			case 't':
				swapSettings(app, view)
			case 'f':
				swapFirewall(app, view)
			// Movement cases
			case 'g': // g and G are ugly, need to calculate highlight offset
				view.services.Select(0, 0)
			case 'G':
				view.services.Select(view.services.GetRowCount(), 0)
			case 'k':
				move(view, -1)
				clearCommand(app, view)
			case 'j':
				move(view, 1)
				clearCommand(app, view)
			case 'h':
				r, _ := view.services.GetSelection()
				view.services.Select(r, 0)
				clearCommand(app, view)
			case 'l':
				r, _ := view.services.GetSelection()
				view.services.Select(r, -1)
				clearCommand(app, view)
			// Kill command
			case 'd':
				s, i := cellInfo(view)
				serv := view.serviceList[s]
				if view.currentCommand == "killall" {
					for x := 0; x < len(serv.conns); x++ {
						kill(app, view, serv.conns[x].pid)
					}
					clearCommand(app, view)
					alertInfo(app, view, "killed all ssh sessions")
					break
				}
				switch serv.name {
				case "ssh":
					if i != -1 {
						kill(app, view, serv.conns[i].pid)
					} else {
						alertInfo(app, view, "are you sure you want to kill all ssh sessions?")
						view.currentCommand = "killall"
					}

				case "apache2", "httpd":
					alertInfo(app, view, "you can't kill httpd connections. try blocking")
				}
			// Killswitch (cut the cord) (blocks all in firewall)
			case 'c':
				alertInfo(app, view, "cut the cord?")
			// Block IP
			case 'b':
				switch view.currentCommand {
				case "block":
					alertInfo(app, view, "blocked")
					// block cmd here (add to fw)
				default:
					s, i := cellInfo(view)
					if i != -1 {
						alertInfo(app, view, "block "+view.serviceList[s].conns[i].ip+"?")
						view.currentCommand = "block"
					} else {
						alertInfo(app, view, "block all connections?")
					}
				}
			// Movement command (i.e. b5r5m will block 5 rows for 5 mins)
			case 'r':
				switch view.currentCommand {
				case "block":
					switch view.currentMove {
					case 0:
						print("blocking x rows")
					}
				}
			case '1', '2', '3', '4', '5', '6', '7', '8', '9':
				cm := strconv.Itoa(view.currentMove) + string(event.Rune())
				view.currentMove, _ = strconv.Atoi(cm)
				// make currentmove -1 when its new
			}
		}
		return event
	}
}

// Examine connections based on current selection
func examineConn(app *tview.Application, view *View) {
	sep := tview.NewBox()
	sep.SetBackgroundColor(tcell.ColorGreen)
	examineBox := tview.NewTable()
	examineBox.SetBackgroundColor(-1)
	// check which service this is running over
	// then switch statement for collecting info
	s, _ := cellInfo(view)
	name := view.serviceList[s].name
	switch name {
	case "ssh":
		examineBox.SetCell(0, 0, tview.NewTableCell("ssh"))
	case "apache2", "httpd":
		examineBox.SetCell(0, 0, tview.NewTableCell("httpd"))
	default:
		examineBox.SetCell(0, 0, tview.NewTableCell("sorry, no clue"))
	}

	// Build and replace flex
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(view.header, 1, 0, false)
	flex.AddItem(view.services, 0, 8, false)
	flex.AddItem(sep, 1, 0, false)
	flex.AddItem(examineBox, 0, 8, true)
	flex.AddItem(view.status, 1, 0, false)
	flex.AddItem(view.nav, 1, 0, false)
	view.flex = flex
	app.SetRoot(view.flex, true)
	view.currentCommand = "examine"
	app.SetInputCapture(examineInput(app, view))
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

///////////////////
// Settings Pane //
///////////////////

func swapSettings(app *tview.Application, view *View) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	view.header.SetText("Settings")
	genSettings(view)
	flex.AddItem(view.header, 1, 0, false)
	flex.AddItem(view.settings, 0, 15, true)
	flex.AddItem(view.status, 1, 0, false)
	flex.AddItem(view.nav, 1, 0, false)
	view.flex = flex
	app.SetInputCapture(settingsInput(app, view))
	app.SetRoot(view.flex, true)
}

func genSettings(view *View) {
	settings := tview.NewFlex().SetDirection(tview.FlexRow)
	settings.AddItem(tview.NewCheckbox().SetLabel("Hit Enter to check box: "), 1, 0, true)
	view.settings = settings
}

func settingsInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		r, _ := view.services.GetSelection()
		switch key {
		case tcell.KeyEnter:
			alertInfo(app, view, "changing setting lul")
		case tcell.KeyRune:
			switch event.Rune() {
			case 's':
				swapServices(app, view)
			case 'f':
				swapFirewall(app, view)
			case 'h':
				view.services.Select(r, 0)
			case 'l':
				view.services.Select(r, 0)
			case 'c':
				alertInfo(app, view, "cut the cord?")
			}
		}
		return event
	}
}

///////////////////
// Firewall Pane //
///////////////////

func swapFirewall(app *tview.Application, view *View) {
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	view.header.SetText("Firewall")
	genFirewall(view)
	flex.AddItem(view.header, 1, 0, false)
	flex.AddItem(view.firewall.fwPane, 0, 1, true)
	flex.AddItem(view.status, 1, 0, false)
	flex.AddItem(view.firewall.fwStatus, 1, 0, false)
	flex.AddItem(view.nav, 1, 0, false)
	view.flex = flex
	app.SetInputCapture(firewallInput(app, view))
	app.SetRoot(view.flex, true)
}

// Compose and update firewall pane
func genFirewall(view *View) {
	fwPane := tview.NewFlex()

	// Build inputPane flex
	inputPane := tview.NewFlex().SetDirection(tview.FlexRow)
	inputTitle := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	inputTitle.SetBackgroundColor(-1)
	inputTitle.SetText("(i)nput")
	inputTable := tview.NewTable()
	inputTable.SetBackgroundColor(-1)

	// Build and run inputTable
	for r := 0; r < len(view.firewall.inputRules); r++ {
		inputTable.SetCell(r, 0, tview.NewTableCell(view.firewall.inputRules[r]))
		print("EXECUTING COMMAND /sbin/iptables ")
		print(view.firewall.inputRules[r])
		cmd := exec.Command(view.firewall.fwCmd, view.firewall.inputRules[r])
		out, err := cmd.CombinedOutput()
		if err != nil {
			if _, ok := err.(*exec.ExitError); !(ok) {
				print("failed to execute firewall rule")
			}
		}
		print(out)
	}

	// Build outputPane flex
	outputPane := tview.NewFlex().SetDirection(tview.FlexRow)
	outputTitle := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	outputTitle.SetBackgroundColor(-1)
	outputTitle.SetText("(o)utput")
	outputTable := tview.NewTable()
	outputTable.SetBackgroundColor(-1)

	// Build fwStatus
	cmd := exec.Command("sha256sum", view.firewall.fwCmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !(ok) {
			print("failed to hash (bin or sha256sum missing)")
			os.Exit(1)
		}
	}
	view.firewall.hash = string(out)
	view.firewall.fwStatus.SetText("fw (c)ommand is " + view.firewall.fwCmd + " hash is " + view.firewall.hash[0:20] + "...")

	// Compose fwPane
	inputPane.AddItem(inputTitle, 1, 0, false)
	inputPane.AddItem(inputTable, 0, 1, true)
	outputPane.AddItem(outputTitle, 1, 0, false)
	outputPane.AddItem(outputTable, 0, 1, true)
	fwPane.AddItem(inputPane, 0, 1, true)
	fwPane.AddItem(outputPane, 0, 1, false)
	view.firewall.fwPane = fwPane
}

func firewallInput(app *tview.Application, view *View) func(event *tcell.EventKey) *tcell.EventKey {
	return func(event *tcell.EventKey) *tcell.EventKey {
		key := event.Key()
		switch key {
		case tcell.KeyEnter:
			alertInfo(app, view, "changing setting lul")
		case tcell.KeyRune:
			switch event.Rune() {
			case 's':
				swapServices(app, view)
			case 't':
				swapSettings(app, view)
			case 'i':
				// add input rule
				newRule := "";
				takeInput(app, view, "Add new rule: ", &newRule, 1)
			case 'd':
				fwDefaults()
			case 'c':
				takeInput(app, view, "Enter a firewall binary path: ", &view.firewall.fwCmd, 0)
			}
		}
		return event
	}
}

func fwDefaults() {
	print("nothing happens...")
}

///////////////
// Help Pane //
///////////////

// services
// d --> kill conn
// dd on service --> kill all conns
// x -> settings
// f -> firewall
// h -> this menu

// firewall
// i --> add input cmd
// e --> edit firewall cmd
//

//////////
// Main //
//////////

func main() {

	// Check for superuser access
	if os.Getenv("SUDO_USER") == "" && os.Getenv("CONMON_OVR") == "" {
		println("Sorry, this program only works when run with sudo.")
		println("This is because you can't get PIDs and info otherwise.")
		println("If for some reason you want to run without sudo,")
		println("set the $CONMON_OVR environment variable to a non-empty value.")
		os.Exit(1)
	}

	app := tview.NewApplication()
	defer app.Stop()

	// Clear screen between writes
	app.SetBeforeDrawFunc(func(s tcell.Screen) bool {
		s.Clear()
		return false
	})

	// Build header
	header := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	header.SetBackgroundColor(-1)
	header.SetText("Services")

	// Build services
	services := tview.NewTable()
	services.SetSelectable(true, true)
	services.SetSelectedStyle(-1, -1, tcell.AttrBold).SetBackgroundColor(-1)
	services.Select(0, 0)
	// TODO DETECT SERVICES (add to checkboxes in settings)
	Service1 := Service{"ssh", 22, []Conn{}}
	Service2 := Service{"httpd", 80, []Conn{}}

	// Build settings
	settings := tview.NewFlex()
	settings.SetBackgroundColor(-1)

	// Build firewall
	firewall := Firewall{tview.NewFlex(), "/sbin/iptables", tview.NewTextView(), []string{}, []string{}, ""}
	firewall.fwPane.SetBackgroundColor(-1)
	firewall.fwStatus.SetTextAlign(tview.AlignCenter)
	firewall.fwStatus.SetBackgroundColor(-1)

	// Build status
	status := tview.NewTextView().SetTextAlign(tview.AlignCenter)
	status.SetBackgroundColor(-1)

	// Build navbar
	nav := tview.NewTable()
	nav.SetBackgroundColor(-1)
	nav.SetCell(0, 0, tview.NewTableCell(" main (localhost)").SetBackgroundColor(tcell.ColorGreen))
	nav.SetCell(0, 1, tview.NewTableCell(" ... (o)"))

	// Build flexbox
	flex := tview.NewFlex().SetDirection(tview.FlexRow)
	flex.AddItem(header, 1, 0, false)
	flex.AddItem(services, 0, 15, true)
	flex.AddItem(status, 1, 0, false)
	flex.AddItem(nav, 1, 0, false)

	// Compose main View
	mainView := View{header, flex, services, []Service{Service1, Service2}, firewall, settings, status, nav, "", 1}
	app.SetInputCapture(servicesInput(app, &mainView))

	// Spawn goroutine to update
	go updateView(app, &mainView)

	// Run it
	if err := app.SetRoot(mainView.flex, true).Run(); err != nil {
		panic(err)
	}

}
