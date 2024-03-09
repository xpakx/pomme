package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	dbus "github.com/godbus/dbus/v5"
	lipgloss "github.com/charmbracelet/lipgloss"
        progress "github.com/charmbracelet/bubbles/progress"
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#626262")).Render
var pomodoroStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Render
var breakStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Render

func main() {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		panic(err)
	}
	defer conn.Close()
	p := tea.NewProgram(initialModel(conn))

	if _, err := p.Run(); err != nil {
		fmt.Printf("error: %v", err)
		os.Exit(1)
	}
}

type model struct {
	msg string
	Dbus *dbus.Conn
	progress progress.Model
	pomodoro Pomodoro
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
	    case "p":
		    pom := m.getPomodoroData()
		    cmd := m.progress.SetPercent(pom.Elapsed/float64(pom.StateDuration))
		    m.pomodoro = pom
		    return m, cmd
	    }
    case progress.FrameMsg:
	    pm, cmd := m.progress.Update(msg)
	    m.progress = pm.(progress.Model)
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

	s += "\n\n"


	s += helpStyle("\nPress q to quit.\n")

	return s
}
