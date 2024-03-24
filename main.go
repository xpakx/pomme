package main

import (
	"errors"
	"fmt"
	"math"
	"os"
	"flag"

	progress "github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	lipgloss "github.com/charmbracelet/lipgloss"
	dbus "github.com/godbus/dbus/v5"
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
var commandStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#6c7086")).Render
var pomodoroStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Render
var breakStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Render

func main() {
	startFlag := flag.Bool("s", false, "start a pomodoro")
	stopFlag := flag.Bool("S", false, "stop a pomodoro")
	silentFlag := flag.Bool("m", false, "silent mode")
	flag.Parse()

	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	m := initialModel(conn)

	if *startFlag {
		m.startPomodoro()
	} else if *stopFlag {
		m.stopPomodoro()
	}

	if !*silentFlag {
		p := tea.NewProgram(&m)
		m.SetProgram(p)
		if _, err := p.Run(); err != nil {
			fmt.Printf("error: %v", err)
			os.Exit(1)
		}
	}
}

type model struct {
	msg string
	Dbus *dbus.Conn
	progress progress.Model
	pomodoro Pomodoro
	program *tea.Program
}

func (m *model) SetProgram(program *tea.Program) {
	m.program = program
}

func initialModel(Dbus *dbus.Conn) model {
	return model {
		msg: "Hello", 
		Dbus: Dbus,
		progress: progress.New(progress.WithGradient("#f2cdcd", "#f5c2e7")),
	}
}

func (m model) sendNotification(Name string, Text string) {
	obj := m.Dbus.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0), "", Name, Text, []string{}, map[string]dbus.Variant{}, int32(5000))
	if call.Err != nil {
		panic(call.Err)
	}
}

func (m model) getPomodoroData() Pomodoro {
	obj := m.Dbus.Object("org.gnome.Pomodoro", "/org/gnome/Pomodoro")

	var props map[string]dbus.Variant
	err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, "org.gnome.Pomodoro").Store(&props)
	if err != nil {
		panic(err)
	}
	pomodoro, err := TransformPomodoro(props)
	if err != nil {
		panic(err)
	}
	return pomodoro
}

type Pomodoro struct {
	IsPaused bool
	Elapsed float64
	State string
	StateDuration int
}

func TransformPomodoro(props map[string]dbus.Variant) (Pomodoro, error) {
	ElapsedVal, ok := props["Elapsed"]
	if !ok {
		return Pomodoro{}, errors.New("No Elapsed value")
	}
	Elapsed, ok := ElapsedVal.Value().(float64)
	if !ok {
		return Pomodoro{}, errors.New("Elapsed value is of wrong type")
	}

	IsPausedVal, ok := props["IsPaused"]
	if !ok {
		return Pomodoro{}, errors.New("No IsPaused value")
	}
	IsPaused, ok := IsPausedVal.Value().(bool)
	if !ok {
		return Pomodoro{}, errors.New("IsPaused value is of wrong type")
	}

	StateVal, ok := props["State"]
	if !ok {
		return Pomodoro{}, errors.New("No State value")
	}
	State, ok := StateVal.Value().(string)
	if !ok {
		return Pomodoro{}, errors.New("State value is of wrong type")
	}

	StateDurationVal, ok := props["StateDuration"]
	if !ok {
		return Pomodoro{}, errors.New("No StateDuration value")
	}
	StateDuration, ok := StateDurationVal.Value().(float64)
	if !ok {
		return Pomodoro{}, errors.New("StateDuration value is of wrong type")
	}

	return Pomodoro{IsPaused, Elapsed, State, int(StateDuration)}, nil
}

func (m model) pausePomodoro() {
	obj := m.Dbus.Object("org.gnome.Pomodoro", "/org/gnome/Pomodoro")
	call := obj.Call("org.gnome.Pomodoro.Pause", 0)
	if call.Err != nil {
		panic(call.Err)
	}
}

func (m model) resumePomodoro() {
	obj := m.Dbus.Object("org.gnome.Pomodoro", "/org/gnome/Pomodoro")
	call := obj.Call("org.gnome.Pomodoro.Resume", 0)
	if call.Err != nil {
		panic(call.Err)
	}
}

func (m model) stopPomodoro() {
	obj := m.Dbus.Object("org.gnome.Pomodoro", "/org/gnome/Pomodoro")
	call := obj.Call("org.gnome.Pomodoro.Stop", 0)
	if call.Err != nil {
		panic(call.Err)
	}
}

func (m model) startPomodoro() {
	obj := m.Dbus.Object("org.gnome.Pomodoro", "/org/gnome/Pomodoro")
	call := obj.Call("org.gnome.Pomodoro.Start", 0)
	if call.Err != nil {
		panic(call.Err)
	}
}


func (m model) subscribe() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	rules := []string{
		"type='signal',path='/org/gnome/Pomodoro',interface='org.freedesktop.DBus.Properties',member='PropertiesChanged'",
	}
	var flag uint = 0
	call := conn.BusObject().Call("org.freedesktop.DBus.Monitoring.BecomeMonitor", 0, rules, flag)
	if call.Err != nil {
		fmt.Fprintln(os.Stderr, "Failed to become monitor:", call.Err)
		return
	}

	c := make(chan *dbus.Message, 10)
	conn.Eavesdrop(c)
	for v := range c {
		processPropertyChange(v, m.program)
	}
}

type ElapsedMsg struct {
	Elapsed float64
}

type IsPausedMsg struct {
	IsPaused bool
}

type StateMsg struct {
	State string
}

type StateDurationMsg struct {
	StateDuration float64
}

func processPropertyChange(v *dbus.Message, program *tea.Program) {
	props := make(map[string]dbus.Variant)
	if len(v.Body) < 2 {
		return
	}
	props = v.Body[1].(map[string]dbus.Variant)

	ElapsedVal, ok := props["Elapsed"]
	if ok {
		Elapsed, ok := ElapsedVal.Value().(float64)
		if ok {
			program.Send(ElapsedMsg{Elapsed})
		}
	}

	IsPausedVal, ok := props["IsPaused"]
	if ok {
		IsPaused, ok := IsPausedVal.Value().(bool)
		if ok {
			program.Send(IsPausedMsg{IsPaused})
		}
	}

	StateVal, ok := props["State"]
	if ok {
		State, ok := StateVal.Value().(string)
		if ok {
			program.Send(StateMsg{State})
		}
	}

	StateDurationVal, ok := props["StateDuration"]
	if ok {
		StateDuration, ok := StateDurationVal.Value().(float64)
		if ok {
			program.Send(StateDurationMsg{StateDuration})
		}
	}
}


func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
	    switch msg.String() {
	    case "ctrl+c", "q":
		    return m, tea.Quit
	    case "n":
		    m.sendNotification("Test", "Hello world")
	    case "P":
		    pom := m.getPomodoroData()
		    percent := pom.Elapsed/float64(pom.StateDuration)
		    if math.IsNaN(percent) {
			    percent = 0
		    }
		    cmd := m.progress.SetPercent(percent)
		    m.pomodoro = pom
		    return m, cmd
	    case "p":
		    m.pausePomodoro()
	    case "s":
		    if m.pomodoro.IsPaused {
			    m.resumePomodoro()
		    } else {
			    m.startPomodoro()
		    }
	    case "S":
		    m.stopPomodoro()
	    case "m":
		    go m.subscribe()
	    }
    case progress.FrameMsg:
	    pm, cmd := m.progress.Update(msg)
	    m.progress = pm.(progress.Model)
	    return m, cmd
    case ElapsedMsg:
	    m.pomodoro.Elapsed = msg.Elapsed
	    percent := msg.Elapsed/float64(m.pomodoro.StateDuration)
	    if math.IsNaN(percent) {
		    percent = 0
	    }
	    cmd := m.progress.SetPercent(percent)
	    return m, cmd
    case StateMsg:
	    m.pomodoro.State = msg.State
	    cmd := m.progress.SetPercent(0)
	    return m, cmd
    case IsPausedMsg:
	    m.pomodoro.IsPaused = msg.IsPaused
	    return m, nil
    case StateDurationMsg:
	    m.pomodoro.StateDuration = int(msg.StateDuration)
	    percent := m.pomodoro.Elapsed/float64(msg.StateDuration)
	    if math.IsNaN(percent) {
		    percent = 0
	    }
	    cmd := m.progress.SetPercent(percent)
	    return m, cmd
	
    }
    return m, nil
}

func (m model) View() string {
	s := breakStyle(m.msg) + "\n\n" 
	s += m.progress.View() 
	s += "\n"
	if m.pomodoro.IsPaused {
		s += "⏸ "
	}
	if m.pomodoro.State == "pomodoro" {
		s += pomodoroStyle("[pomodoro]")
	} else if m.pomodoro.State == "long_break" || m.pomodoro.State == "short_break" {
		s += breakStyle("[break]")
	}

	s += "\n\n\n"


	s += commandStyle("q") + helpStyle(" quit • ")
	s += commandStyle("s") + helpStyle(" ⏵ • ")
	s += commandStyle("p") + helpStyle(" ⏸ • ")
	s += commandStyle("S") + helpStyle(" ⏹ • ")
	s += commandStyle("P") + helpStyle(" load data")

	s += "\n"

	return s
}
