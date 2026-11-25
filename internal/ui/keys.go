package ui

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// keyMap is the single source of truth for key bindings; the footer and the
// help screen are both generated from it.
type keyMap struct {
	Up, Down, Focus      key.Binding
	Search, Select       key.Binding
	Ban, Unban, UnbanAll key.Binding
	Toggle, Reload       key.Binding
	ReloadAll, Refresh   key.Binding
	Help, Quit           key.Binding
}

func newKeyMap() keyMap {
	return keyMap{
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "yukarı")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "aşağı")),
		Focus:     key.NewBinding(key.WithKeys("tab", "shift+tab", "left", "right", "h", "l"), key.WithHelp("tab", "panel değiştir")),
		Search:    key.NewBinding(key.WithKeys("/"), key.WithHelp("/", "ara")),
		Select:    key.NewBinding(key.WithKeys(" "), key.WithHelp("space", "çoklu seçim")),
		Ban:       key.NewBinding(key.WithKeys("b"), key.WithHelp("b", "ban")),
		Unban:     key.NewBinding(key.WithKeys("u"), key.WithHelp("u", "unban")),
		UnbanAll:  key.NewBinding(key.WithKeys("U"), key.WithHelp("U", "tüm jail'lerden unban")),
		Toggle:    key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "jail start/stop")),
		Reload:    key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "jail reload")),
		ReloadAll: key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "global reload")),
		Refresh:   key.NewBinding(key.WithKeys("f5", "ctrl+r"), key.WithHelp("F5", "yenile")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "yardım")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "çık")),
	}
}

// ShortHelp implements help.KeyMap for the footer.
func (k keyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Ban, k.Unban, k.Toggle, k.Reload, k.Search, k.Help, k.Quit}
}

// FullHelp implements help.KeyMap for the help screen.
func (k keyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down, k.Focus, k.Search, k.Select},
		{k.Ban, k.Unban, k.UnbanAll},
		{k.Toggle, k.Reload, k.ReloadAll, k.Refresh},
		{k.Help, k.Quit},
	}
}

// isWrite reports whether msg triggers a state-changing action (disabled with --read-only).
func (k keyMap) isWrite(msg tea.KeyMsg) bool {
	return key.Matches(msg, k.Ban, k.Unban, k.UnbanAll, k.Toggle, k.Reload, k.ReloadAll)
}
