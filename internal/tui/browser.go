package tui

import (
	"context"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"
	"xsync/internal/fsbrowse"
	"xsync/internal/i18n"
)

var ErrCancelled = errors.New("cancelled")

func Text(vi, en string) string {
	if i18n.GetLanguage() == i18n.LangEN {
		return en
	}
	return vi
}

type BrowserOptions struct {
	Title, Start, Root string
	List               fsbrowse.ListFunc
	Mkdir              func(context.Context, string) error
	Selected           []BrowserSelection
	Multi              bool
	AllowEmpty         bool
	ExcludeRoot        bool
}

type BrowserSelection struct {
	Path string
	Dir  bool
}
type browserLoaded struct {
	id      int
	listing fsbrowse.Listing
	err     error
}
type browserModel struct {
	options                           BrowserOptions
	ctx                               context.Context
	cancel                            context.CancelFunc
	listing                           fsbrowse.Listing
	cache                             map[string]fsbrowse.Listing
	selected                          map[string]bool
	cursor, height, request           int
	loading, done, cancelled          bool
	inputMode, input, filter, message string
	chosen                            string
}

func newBrowser(ctx context.Context, o BrowserOptions) browserModel {
	m := browserModel{options: o, ctx: ctx, cache: make(map[string]fsbrowse.Listing), selected: make(map[string]bool), height: 24}
	for _, item := range o.Selected {
		m.selected[item.Path] = item.Dir
	}
	return m
}

func (m *browserModel) load(dir string, refresh bool) tea.Cmd {
	if m.cancel != nil {
		m.cancel()
	}
	m.request++
	m.inputMode, m.input, m.filter, m.message = "", "", "", ""
	m.cursor = 0
	if cached, ok := m.cache[dir]; ok && !refresh {
		m.listing, m.loading = cached, false
		return nil
	}
	m.loading = true
	id := m.request
	ctx, cancel := context.WithCancel(m.ctx)
	m.cancel = cancel
	list := m.options.List
	return func() tea.Msg { result, err := list(ctx, dir); return browserLoaded{id: id, listing: result, err: err} }
}

func (m browserModel) Init() tea.Cmd { return nil }

func (m browserModel) visible() []fsbrowse.Entry {
	var result []fsbrowse.Entry
	for _, e := range m.listing.Entries {
		if (!m.options.Multi && !e.Dir) || !strings.Contains(strings.ToLower(e.Name), strings.ToLower(m.filter)) {
			continue
		}
		result = append(result, e)
	}
	return result
}

func (m browserModel) relative(p string) string {
	if p == m.options.Root {
		return "."
	}
	return strings.TrimPrefix(p, strings.TrimRight(m.options.Root, "/")+"/")
}

func (m browserModel) covered(rel string) bool {
	for selected, dir := range m.selected {
		if selected == rel || selected == "." || (dir && strings.HasPrefix(rel, selected+"/")) {
			return true
		}
	}
	return false
}

func (m *browserModel) toggle(rel string, dir bool) {
	if _, ok := m.selected[rel]; ok {
		delete(m.selected, rel)
		return
	}
	if m.covered(rel) {
		m.message = Text("Bỏ chọn thư mục cha trước để chọn riêng các mục con.", "Unselect the parent folder first to choose individual children.")
		return
	}
	if dir {
		for old := range m.selected {
			if rel == "." || strings.HasPrefix(old, rel+"/") {
				delete(m.selected, old)
			}
		}
	}
	m.selected[rel] = dir
}

func (m browserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.height = msg.Height
	case browserLoaded:
		if msg.id != m.request {
			return m, nil
		}
		m.loading = false
		if msg.err != nil {
			m.message = msg.err.Error()
			return m, nil
		}
		if root := m.options.Root; root != "" && msg.listing.Path != root && !strings.HasPrefix(msg.listing.Path, root+"/") {
			m.message = Text("Đường dẫn phải nằm trong thư mục nguồn.", "The path must stay inside the source folder.")
			return m, nil
		}
		m.listing = msg.listing
		if len(m.cache) >= 128 {
			m.cache = make(map[string]fsbrowse.Listing)
		}
		m.cache[msg.listing.Path] = msg.listing
	case tea.KeyMsg:
		key := msg.String()
		if key == "ctrl+c" || (key == "esc" && m.inputMode == "") || (key == "q" && m.inputMode == "") {
			m.cancelled = true
			return m, tea.Quit
		}
		if m.inputMode != "" {
			switch key {
			case "ctrl+u":
				m.input = ""
			case "esc":
				m.inputMode, m.input = "", ""
			case "backspace", "ctrl+h":
				r := []rune(m.input)
				if len(r) > 0 {
					m.input = string(r[:len(r)-1])
				}
			case "enter":
				switch m.inputMode {
				case "search":
					m.filter, m.inputMode, m.cursor = m.input, "", 0
				case "path":
					dir := path.Clean(m.input)
					if !path.IsAbs(dir) {
						dir = path.Join(m.listing.Path, dir)
					}
					if m.options.Root != "" && dir != m.options.Root && !strings.HasPrefix(dir, m.options.Root+"/") {
						m.message = Text("Đường dẫn phải nằm trong thư mục nguồn.", "The path must stay inside the source folder.")
						m.inputMode = ""
						break
					}
					cmd := m.load(dir, false)
					return m, cmd
				case "mkdir":
					name := m.input
					if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\\\r\n\x00") {
						m.message = Text("Tên thư mục không hợp lệ.", "Invalid folder name.")
						m.inputMode = ""
						break
					}
					dir := path.Join(m.listing.Path, name)
					m.inputMode = ""
					m.loading = true
					m.request++
					id, ctx, mkdir, list := m.request, m.ctx, m.options.Mkdir, m.options.List
					m.cache = make(map[string]fsbrowse.Listing)
					return m, func() tea.Msg {
						if err := mkdir(ctx, dir); err != nil {
							return browserLoaded{id: id, err: err}
						}
						listing, err := list(ctx, dir)
						return browserLoaded{id: id, listing: listing, err: err}
					}
				}
			default:
				if msg.Type == tea.KeyRunes {
					m.input += string(msg.Runes)
				} else if msg.Type == tea.KeySpace {
					m.input += " "
				}
			}
			if m.inputMode == "search" {
				m.filter, m.cursor = m.input, 0
			}
			return m, nil
		}
		if key == "backspace" || key == "left" {
			if m.listing.Path != "" && m.listing.Path != m.options.Root {
				cmd := m.load(path.Dir(m.listing.Path), false)
				return m, cmd
			}
			return m, nil
		}
		if m.loading {
			return m, nil
		}
		items := m.visible()
		page := max(3, m.height-12)
		switch key {
		case "up", "k":
			m.cursor = max(0, m.cursor-1)
		case "down", "j":
			m.cursor = min(max(0, len(items)-1), m.cursor+1)
		case "pgup":
			m.cursor = max(0, m.cursor-page)
		case "pgdown":
			m.cursor = min(max(0, len(items)-1), m.cursor+page)
		case "home":
			m.cursor = 0
		case "end":
			m.cursor = max(0, len(items)-1)
		case "enter", "right":
			if len(items) > 0 && items[m.cursor].Dir {
				cmd := m.load(path.Join(m.listing.Path, items[m.cursor].Name), false)
				return m, cmd
			}
		case " ":
			if !m.options.Multi {
				m.chosen = m.listing.Path
				m.done = true
				return m, tea.Quit
			}
			if len(items) > 0 {
				e := items[m.cursor]
				m.toggle(m.relative(path.Join(m.listing.Path, e.Name)), e.Dir)
			}
		case "a":
			if m.options.Multi {
				if m.options.ExcludeRoot && m.relative(m.listing.Path) == "." {
					m.message = Text("Chọn các mục con để loại trừ; không loại trừ toàn bộ nguồn.", "Select children to exclude; the entire source cannot be excluded.")
				} else {
					m.toggle(m.relative(m.listing.Path), true)
				}
			}
		case "tab", "ctrl+s":
			if m.options.Multi {
				if len(m.selected) == 0 && !m.options.AllowEmpty {
					m.message = Text("Chọn ít nhất một mục, hoặc nhấn a để chọn cả thư mục.", "Select an item, or press a to select the entire folder.")
				} else {
					m.done = true
					return m, tea.Quit
				}
			}
		case "/":
			m.inputMode, m.input = "search", m.filter
		case "g":
			m.inputMode, m.input = "path", m.listing.Path
		case "r":
			cmd := m.load(m.listing.Path, true)
			return m, cmd
		case "n":
			if m.options.Mkdir != nil {
				m.inputMode, m.input = "mkdir", ""
			}
		}
	}
	return m, nil
}

func cleanTerminal(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '�'
		}
		return r
	}, s)
}

func (m browserModel) View() string {
	if m.done || m.cancelled {
		return ""
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(m.options.Title) + "\n" + cleanTerminal(m.listing.Path) + "\n")
	if m.options.Multi {
		b.WriteString(Text("↑↓ Di chuyển · Enter Mở · Backspace Lên · Space Chọn/bỏ · a Cả thư mục · Tab Xong", "↑↓ Move · Enter Open · Backspace Parent · Space Toggle · a Whole folder · Tab Done"))
	} else {
		b.WriteString(Text("↑↓ Di chuyển · Enter Mở · Backspace Lên · Space Dùng thư mục hiện tại", "↑↓ Move · Enter Open · Backspace Parent · Space Use current folder"))
	}
	b.WriteString("\n" + Text("/ Tìm · g Nhập đường dẫn · r Tải lại · q Hủy", "/ Search · g Go to path · r Refresh · q Cancel"))
	if m.options.Mkdir != nil {
		b.WriteString(Text(" · n Tạo thư mục", " · n New folder"))
	}
	b.WriteString("\n\n")
	if m.loading {
		b.WriteString(Text("Đang tải thư mục…\n", "Loading folder…\n"))
	} else {
		items := m.visible()
		page := max(3, m.height-12)
		start := max(0, m.cursor-page+1)
		for i := start; i < min(len(items), start+page); i++ {
			e := items[i]
			suffix := ""
			if e.Dir {
				suffix = "/"
			}
			prefix := "  "
			if i == m.cursor {
				prefix = "▸ "
			}
			if m.options.Multi {
				mark := "[ ] "
				if m.covered(m.relative(path.Join(m.listing.Path, e.Name))) {
					mark = "[x] "
				}
				prefix += mark
			}
			line := prefix + cleanTerminal(e.Name) + suffix
			if i == m.cursor {
				line = activeItemStyle.Render(line)
			}
			b.WriteString(line + "\n")
		}
		if len(items) == 0 {
			b.WriteString(Text("(Thư mục trống hoặc không có kết quả)\n", "(Empty folder or no matches)\n"))
		}
		b.WriteString(fmt.Sprintf("\n%d/%d", min(m.cursor+1, len(items)), len(items)))
		if m.options.Multi {
			b.WriteString(fmt.Sprintf(Text(" · %d mục đã chọn", " · %d selections"), len(m.selected)))
		}
	}
	if m.filter != "" {
		b.WriteString("\n" + Text("Tìm: ", "Search: ") + cleanTerminal(m.filter))
	}
	if m.inputMode != "" {
		b.WriteString("\n" + m.inputMode + "> " + cleanTerminal(m.input) + "▏")
	}
	if m.message != "" {
		b.WriteString("\n" + cleanTerminal(m.message))
	}
	return b.String() + "\n"
}

func Browse(ctx context.Context, o BrowserOptions) (string, []BrowserSelection, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	m := newBrowser(ctx, o)
	initial := m.load(o.Start, false)
	// Use a wrapper to schedule the first asynchronous listing through Init.
	result, err := tea.NewProgram(browserStart{browserModel: m, initial: initial}, tea.WithAltScreen()).Run()
	if err != nil {
		return "", nil, err
	}
	var model browserModel
	switch r := result.(type) {
	case browserStart:
		model = r.browserModel
	case browserModel:
		model = r
	}
	if model.cancelled || !model.done {
		return "", nil, ErrCancelled
	}
	var selections []BrowserSelection
	for rel, dir := range model.selected {
		selections = append(selections, BrowserSelection{Path: rel, Dir: dir})
	}
	sort.Slice(selections, func(i, j int) bool { return selections[i].Path < selections[j].Path })
	return model.chosen, selections, nil
}

type browserStart struct {
	browserModel
	initial tea.Cmd
}

func (m browserStart) Init() tea.Cmd { return m.initial }

// ChooseMenu returns cancellation to its caller so deferred SSH cleanup can run.
func ChooseMenu(title string, options []string, defaultIdx int) (int, error) {
	items := make([]MenuItem, len(options))
	for i, label := range options {
		items[i] = MenuItem{ID: fmt.Sprint(i), Label: label}
	}
	selected, err := ChooseItems(title, items, nil)
	if err != nil {
		return -1, err
	}
	for i, item := range items {
		if item.ID == selected.ID {
			return i, nil
		}
	}
	return -1, ErrCancelled
}
