package main

import (
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

const timerDefault = 100

type Service struct {
	Name  string
	Port  uint32
	Conns []Conn
}

type Conn struct {
	LocalIp    string
	LocalPort  uint32
	RemoteIp   string
	RemotePort uint32
	Pid        int32
	Info       string
	Meta       string
	Timer      int
	Full       net.ConnectionStat
}

var currentConns map[uint32]map[int32]*Conn = make(map[uint32]map[int32]*Conn)
var previousConns map[int32]*Conn = make(map[int32]*Conn)
var listeningPorts [](net.ConnectionStat)

// Kills process and all its children
func killProcess(pid int32) bool {
	proc, err := process.NewProcess(pid)
	if err == nil {
		children, err := proc.Children()
		if err == nil {
			for z := 0; z < len(children); z++ {
				children[z].SendSignal(15)
			}
		}
		proc.SendSignal(15)
	} else {
		return false
	}
	return true
}

func (view *View) genConnections() {
	// Get current connections
	sysConnections, err := net.Connections("all")
	if err != nil {
		panic(err)
	}
	view.Connections.Clear()

	listeningPorts = []net.ConnectionStat{}
	nextPreviousConns := make(map[int32]*Conn)

	for _, sysConn := range sysConnections {
		sysPort := sysConn.Laddr.Port
		if sysConn.Status == "ESTABLISHED" {
			// Weird PID 0 bug
			if sysConn.Pid == 0 {
				continue
			}
			conn, ok := currentConns[sysPort][sysConn.Pid]
			// Connection already existed
			if ok {
				nextPreviousConns[sysConn.Pid] = conn
				delete(previousConns, sysConn.Pid)
				conn.Info = genServInfo(sysPort)
				// New connection found
			} else {
				newConn := Conn{
					LocalIp:    sysConn.Laddr.IP,
					LocalPort:  sysConn.Laddr.Port,
					RemoteIp:   sysConn.Raddr.IP,
					RemotePort: sysConn.Raddr.Port,
					Pid:        sysConn.Pid,
					Info:       genServInfo(sysPort),
					Timer:      timerDefault,
					Full:       sysConn,
				}
				nextPreviousConns[sysConn.Pid] = &newConn
				if _, ok := currentConns[sysPort]; !ok {
					currentConns[sysPort] = make(map[int32]*Conn)
				}
				currentConns[sysPort][sysConn.Pid] = &newConn
			}
		} else if sysConn.Status == "LISTEN" {
			listeningPorts = append(listeningPorts, sysConn)
		}
	}

	// check for dead conns
	for sysPid, deadConn := range previousConns {
		deadConn.Timer--
		deadConn.Meta = "(dead)"
		if deadConn.Timer <= 0 {
			delete(currentConns[deadConn.LocalPort], sysPid)
			if len(currentConns[deadConn.LocalPort]) == 0 {
				delete(currentConns, deadConn.LocalPort)
			}
		} else {
			nextPreviousConns[sysPid] = deadConn
		}
	}

	// Use current conns as previousConns for cycle
	previousConns = nextPreviousConns
	outputConnections(view, currentConns)

}
