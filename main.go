package main

import (
	"errors"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	dbus "github.com/godbus/dbus/v5"
	// lipgloss "github.com/charmbracelet/lipgloss"
        progress "github.com/charmbracelet/bubbles/progress"
)

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
}

func initialModel(Dbus *dbus.Conn) model {
	return model {
		msg: "Hello", 
		Dbus: Dbus,
		progress: progress.New(progress.WithDefaultGradient()),
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

	var Reset  = "\033[0m"
	var Blue   = "\033[34m"
	var Red    = "\033[31m"

	s := Blue + m.msg + Reset + "\n\n" 
	s += m.progress.View() 
	s += "\n\n"


	s += Red + "\nPress q to quit.\n" + Reset

	return s
}
