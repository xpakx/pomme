package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	dbus "github.com/godbus/dbus/v5"
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
}

func initialModel(Dbus *dbus.Conn) model {
	return model {msg: "Hello", Dbus: Dbus}
}

func (m model) sendNotification(Name string, Text string) {
	obj := m.Dbus.Object("org.freedesktop.Notifications", "/org/freedesktop/Notifications")
	call := obj.Call("org.freedesktop.Notifications.Notify", 0, "", uint32(0), "", Name, Text, []string{}, map[string]dbus.Variant{}, int32(5000))
	if call.Err != nil {
		panic(call.Err)
	}
}

func (m model) getPomodoroData() {
	obj := m.Dbus.Object("org.gnome.Pomodoro", "/org/gnome/Pomodoro")

	var props map[string]dbus.Variant
	err := obj.Call("org.freedesktop.DBus.Properties.GetAll", 0, "org.gnome.Pomodoro").Store(&props)
	if err != nil {
		panic(err)
	}
	fmt.Println(props)
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
		    m.getPomodoroData()
	    }
    }
    return m, nil
}

func (m model) View() string {

	var Reset  = "\033[0m"
	var Blue   = "\033[34m"
	var Red    = "\033[31m"

	s := Blue + m.msg + Reset
	s += "\n\n"

	s += Red + "\nPress q to quit.\n" + Reset

	return s
}
