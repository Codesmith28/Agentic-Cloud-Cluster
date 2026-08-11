package tui

import "github.com/charmbracelet/bubbles/key"

type keyMap struct {
	NextPane    key.Binding
	PrevPane    key.Binding
	FocusCmd    key.Binding
	Escape      key.Binding
	Enter       key.Binding
	Refresh     key.Binding
	Monitor     key.Binding
	Cancel      key.Binding
	Unregister  key.Binding
	QuitSubview key.Binding
	Quit        key.Binding
}

var keys = keyMap{
	NextPane: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next pane"),
	),
	PrevPane: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev pane"),
	),
	FocusCmd: key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", "command bar"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "back"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "execute"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	),
	Monitor: key.NewBinding(
		key.WithKeys("m"),
		key.WithHelp("m", "monitor task"),
	),
	Cancel: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "cancel task"),
	),
	Unregister: key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", "unregister worker"),
	),
	QuitSubview: key.NewBinding(
		key.WithKeys("q"),
		key.WithHelp("q", "quit subview"),
	),
	Quit: key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", "exit"),
	),
}
