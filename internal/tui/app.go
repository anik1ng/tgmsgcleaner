// Package tui implements the terminal interface using Bubble Tea.
package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tg "github.com/anik1ng/tgmsgcleaner/internal/telegram"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
)

const (
	screenPicker = iota
	screenAuth
	screenPassword
	screenGroupList
	screenActions
	screenMessages
	screenConfirm
	screenDeleting
)

const (
	filterAll         = 0
	filterGroups      = 1
	filterSupergroups = 2
	filterChannels    = 3
	filterLeftWithMsg = 4
	filterLeftAll     = 5
)

var filterNames = []string{"All", "Groups", "Supergroups", "Channels", "Left+msg", "Left"}

var (
	tagGroup      = lipgloss.NewStyle().Foreground(lipgloss.Color("32")).Render("[GRP]")
	tagSupergroup = lipgloss.NewStyle().Foreground(lipgloss.Color("34")).Render("[SUP]")
	tagChannel    = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Render("[CHN]")
	tagLeft       = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("[LEFT]")
)

type App struct {
	client      *telegram.Client
	ctx         context.Context
	phone       string
	sessionPath string
	selfID      int64
	codeHash string
	authErr  string

	screen    int
	authInput textinput.Model
	passInput textinput.Model
	list      list.Model
	progress  progress.Model

	allChats        []tg.ChatInfo
	filter          int
	width           int
	height          int
	loadingMessages bool
	loadedCount     int
	loadTotal       int
	leftLoaded      bool
	loadingLeft     bool
	leftFound       int

	selectedChat *tg.ChatInfo
	actionList   list.Model
	msgList      list.Model
	messages     []tg.MessageInfo
	nextOffsetID int
	hasMore      bool
	loadingMore  bool

	confirmText    string
	confirmAction  string
	deletingStatus string
	deletingPct    float64
	deletingDone   bool
	deletingErr    string

	ExitAction   string // "switch", "add", "" — read by main.go after TUI exits
	accountCount int

	// Picker mode (no client)
	pickerList list.Model
	Selected   string // selected phone from picker — read by main.go
}

type accountItem struct {
	phone string
}

func (a accountItem) Title() string       { return a.phone }
func (a accountItem) Description() string { return "" }
func (a accountItem) FilterValue() string { return a.phone }

type Group struct {
	title      string
	desc       string
	chatType   tg.ChatType
	isLeft     bool
	myMessages int
}

type codeSentMsg struct{ codeHash string }
type authDoneMsg struct{ selfID int64 }
type authErrMsg struct{ err error }
type needPasswordMsg struct{}
type chatsLoadedMsg struct {
	chats  []tg.ChatInfo
	selfID int64
}
type msgCountMsg struct {
	chatIndex int
	count     int
	total     int
	err       error
	nextCmd   tea.Cmd
}
type msgCountDoneMsg struct{}
type messagesLoadedMsg struct {
	messages []tg.MessageInfo
	hasMore  bool
}

type deleteProgressMsg struct {
	deleted int
	total   int
	done    bool
	err     error
	nextCmd tea.Cmd
}

type reactionProgressMsg struct {
	checked int
	removed int
	done    bool
	err     error
	nextCmd tea.Cmd
}

type exportProgressMsg struct {
	exported int
	total    int
	done     bool
	err      error
	filePath string
	nextCmd  tea.Cmd
}

type leftChannelsMsg struct {
	chats   []tg.ChatInfo
	found   int
	done    bool
	err     error
	nextCmd tea.Cmd
}

type leftGroupMsg struct{}

type ActionItem struct {
	title string
	key   string
}

func (a ActionItem) Title() string       { return a.title }
func (a ActionItem) Description() string { return "" }
func (a ActionItem) FilterValue() string { return a.title }

type MessageItem struct {
	date string
	text string
}

func (m MessageItem) Title() string       { return m.date }
func (m MessageItem) Description() string { return m.text }
func (m MessageItem) FilterValue() string { return m.text }

func NewApp(client *telegram.Client, ctx context.Context, phone string, sessionPath string, isAuthorized bool, accountCount int) App {
	ti := textinput.New()
	ti.Placeholder = "Enter auth code..."
	ti.SetWidth(40)
	ti.Focus()

	pi := textinput.New()
	pi.Placeholder = "Enter 2FA password..."
	pi.SetWidth(40)
	pi.EchoMode = textinput.EchoPassword

	addAccountKey := key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add account"))
	switchAccountKey := key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "switch account"))
	leftChannelsKey := key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "find left channels"))
	if accountCount < 2 {
		switchAccountKey.SetEnabled(false)
	}

	l := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 24)
	l.Title = "All Chats"
	l.KeyMap.Quit.SetKeys("q")
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{addAccountKey, switchAccountKey, leftChannelsKey}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{addAccountKey, switchAccountKey, leftChannelsKey}
	}

	actionDelegate := list.NewDefaultDelegate()
	actionDelegate.ShowDescription = false
	al := list.New([]list.Item{}, actionDelegate, 80, 24)
	al.SetFilteringEnabled(false)

	ml := list.New([]list.Item{}, list.NewDefaultDelegate(), 80, 24)
	ml.Title = "Messages"

	p := progress.New()

	screen := screenAuth
	if isAuthorized {
		screen = screenGroupList
	}

	return App{
		client:      client,
		ctx:         ctx,
		phone:       phone,
		sessionPath: sessionPath,
		screen:      screen,
		authInput: ti,
		passInput: pi,
		list:         l,
		actionList:   al,
		msgList:      ml,
		progress:     p,
		accountCount: accountCount,
	}
}

func NewPickerApp(accounts []string) App {
	items := make([]list.Item, len(accounts))
	for i, phone := range accounts {
		items[i] = accountItem{phone: phone}
	}

	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false
	l := list.New(items, delegate, 40, 14)
	l.Title = "Select account"
	l.SetFilteringEnabled(false)
	l.KeyMap.Quit.SetKeys("q")

	return App{
		screen:     screenPicker,
		pickerList: l,
	}
}

func (a App) Init() tea.Cmd {
	if a.screen == screenAuth {
		return func() tea.Msg {
			codeHash, err := tg.SendCode(a.ctx, a.client, a.phone, a.sessionPath)
			if err != nil {
				return authErrMsg{err: err}
			}
			return codeSentMsg{codeHash: codeHash}
		}
	}
	if a.screen == screenGroupList {
		return a.loadChatsCmd()
	}
	return nil
}

func (a *App) loadChatsCmd() tea.Cmd {
	return func() tea.Msg {
		selfID, err := tg.GetSelfID(a.ctx, a.client)
		if err != nil {
			return authErrMsg{err: err}
		}
		chats, err := tg.GetChats(a.ctx, a.client)
		if err != nil {
			return authErrMsg{err: err}
		}
		return chatsLoadedMsg{chats: chats, selfID: selfID}
	}
}

func waitForCount(ch <-chan tg.MessageCountProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return msgCountDoneMsg{}
		}
		return msgCountMsg{
			chatIndex: p.ChatIndex,
			count:     p.Count,
			total:     p.Total,
			err:       p.Err,
			nextCmd:   waitForCount(ch),
		}
	}
}

func (a *App) applyFilter() {
	var items []list.Item
	for _, c := range a.allChats {
		switch a.filter {
		case filterGroups:
			if c.Type != tg.ChatTypeGroup {
				continue
			}
		case filterSupergroups:
			if c.Type != tg.ChatTypeSupergroup {
				continue
			}
		case filterChannels:
			if c.Type != tg.ChatTypeChannel {
				continue
			}
		case filterLeftAll:
			if !c.IsLeft {
				continue
			}
		case filterLeftWithMsg:
			if !c.IsLeft || c.MyMessages <= 0 {
				continue
			}
		}
		items = append(items, Group{
			title:      c.Title,
			desc:       formatDescription(c),
			chatType:   c.Type,
			isLeft:     c.IsLeft,
			myMessages: c.MyMessages,
		})
	}
	a.list.SetItems(items)
	a.list.Title = filterNames[a.filter]
}

func formatDescription(c tg.ChatInfo) string {
	var parts []string
	if c.IsLeft {
		if c.Username != "" {
			parts = append(parts, "t.me/"+c.Username)
		} else {
			parts = append(parts, fmt.Sprintf("t.me/c/%d", c.ID))
		}
	}
	if c.MemberCount > 0 {
		parts = append(parts, fmt.Sprintf("%d members", c.MemberCount))
	}
	if !c.IsLeft && c.Username != "" {
		parts = append(parts, "@"+c.Username)
	}
	if c.MyMessages >= 0 {
		parts = append(parts, fmt.Sprintf("%d your msgs", c.MyMessages))
	}
	desc := ""
	for i, p := range parts {
		if i > 0 {
			desc += " · "
		}
		desc += p
	}
	return desc
}

var statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

func (a App) filterCount() int {
	if a.leftLoaded {
		return 6 // All, Groups, Supergroups, Channels, Left+msg, Left
	}
	return 4 // All, Groups, Supergroups, Channels
}

func (a App) statusBar() string {
	totalMsgs := 0
	counted := 0
	for _, c := range a.allChats {
		if c.MyMessages >= 0 {
			totalMsgs += c.MyMessages
			counted++
		}
	}
	msgs := fmt.Sprintf("%d msgs", totalMsgs)
	if counted < len(a.allChats) {
		msgs += "..."
	}
	return statusStyle.Render(fmt.Sprintf("  %s · %d groups · %s", a.phone, len(a.allChats), msgs))
}

func (a *App) openActions(chat tg.ChatInfo) {
	a.selectedChat = &chat
	a.screen = screenActions
	title := chat.Title
	if chat.MyMessages >= 0 {
		title += fmt.Sprintf(" (%d msgs)", chat.MyMessages)
	}
	a.actionList.Title = title
	items := []list.Item{
		ActionItem{title: "View my messages", key: "view"},
		ActionItem{title: "Export my messages", key: "export"},
		ActionItem{title: "Delete my messages", key: "delete_msgs"},
		ActionItem{title: "Delete my reactions", key: "delete_reactions"},
	}
	if !chat.IsLeft {
		items = append(items, ActionItem{title: "Leave group", key: "leave"})
	}
	a.actionList.SetItems(items)
}

func (a *App) fetchMessagesCmd(offsetID int) tea.Cmd {
	chat := a.selectedChat
	selfID := a.selfID
	return func() tea.Msg {
		msgs, err := tg.FetchMyMessages(a.ctx, a.client, *chat, selfID, offsetID, 50)
		if err != nil {
			return authErrMsg{err: err}
		}
		return messagesLoadedMsg{
			messages: msgs,
			hasMore:  len(msgs) == 50,
		}
	}
}

func waitForDelete(ch <-chan tg.DeleteProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return deleteProgressMsg{done: true}
		}
		return deleteProgressMsg{
			deleted: p.Deleted,
			total:   p.Total,
			done:    p.Done,
			err:     p.Err,
			nextCmd: waitForDelete(ch),
		}
	}
}

func waitForReactions(ch <-chan tg.ReactionProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return reactionProgressMsg{done: true}
		}
		return reactionProgressMsg{
			checked: p.Checked,
			removed: p.Removed,
			done:    p.Done,
			err:     p.Err,
			nextCmd: waitForReactions(ch),
		}
	}
}

func waitForExport(ch <-chan tg.ExportProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return exportProgressMsg{done: true}
		}
		return exportProgressMsg{
			exported: p.Exported,
			total:    p.Total,
			done:     p.Done,
			err:      p.Err,
			filePath: p.FilePath,
			nextCmd:  waitForExport(ch),
		}
	}
}

func waitForLeftChannels(ch <-chan tg.LeftChannelsProgress) tea.Cmd {
	return func() tea.Msg {
		p, ok := <-ch
		if !ok {
			return leftChannelsMsg{done: true}
		}
		return leftChannelsMsg{
			chats:   p.Chats,
			found:   p.Found,
			done:    p.Done,
			err:     p.Err,
			nextCmd: waitForLeftChannels(ch),
		}
	}
}

func (a *App) buildMessageItems() []list.Item {
	items := make([]list.Item, 0, len(a.messages))
	for _, m := range a.messages {
		t := time.Unix(int64(m.Date), 0)
		items = append(items, MessageItem{
			date: t.Format("[2006-01-02 15:04]"),
			text: m.Text,
		})
	}
	if a.hasMore {
		items = append(items, MessageItem{
			date: "↓",
			text: "Load more messages... (Enter)",
		})
	}
	return items
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		if a.screen == screenPicker {
			a.pickerList.SetSize(msg.Width, msg.Height-4)
		} else {
			a.list.SetSize(msg.Width, msg.Height-6)
			a.actionList.SetSize(msg.Width, msg.Height-2)
			a.msgList.SetSize(msg.Width, msg.Height-2)
			a.progress.SetWidth(msg.Width - 10)
		}
		return a, nil

	case codeSentMsg:
		a.codeHash = msg.codeHash
		return a, nil

	case authDoneMsg:
		return a, a.loadChatsCmd()

	case chatsLoadedMsg:
		a.allChats = msg.chats
		a.selfID = msg.selfID
		a.applyFilter()
		a.screen = screenGroupList
		a.loadingMessages = true
		a.loadedCount = 0
		a.loadTotal = len(a.allChats)
		ch := make(chan tg.MessageCountProgress, 1)
		go tg.CountAllMyMessages(a.ctx, a.client, a.allChats, a.selfID, ch)
		return a, waitForCount(ch)

	case msgCountMsg:
		if msg.err == nil && msg.chatIndex < len(a.allChats) {
			a.allChats[msg.chatIndex].MyMessages = msg.count
		}
		a.loadedCount = msg.chatIndex + 1
		a.loadTotal = msg.total
		a.applyFilter()
		return a, msg.nextCmd

	case msgCountDoneMsg:
		a.loadingMessages = false
		a.applyFilter()
		return a, nil

	case messagesLoadedMsg:
		a.messages = append(a.messages, msg.messages...)
		a.hasMore = msg.hasMore
		a.loadingMore = false
		if len(msg.messages) > 0 {
			a.nextOffsetID = msg.messages[len(msg.messages)-1].ID
		}
		a.msgList.SetItems(a.buildMessageItems())
		return a, nil

	case deleteProgressMsg:
		if msg.err != nil {
			a.deletingErr = msg.err.Error()
			a.deletingDone = true
			return a, nil
		}
		if msg.total > 0 {
			a.deletingPct = float64(msg.deleted) / float64(msg.total)
		}
		a.deletingStatus = fmt.Sprintf("Deleting messages... %d/%d", msg.deleted, msg.total)
		if msg.done {
			a.deletingDone = true
			a.deletingStatus = fmt.Sprintf("Done! Deleted %d messages.", msg.deleted)
			if a.selectedChat != nil {
				a.selectedChat.MyMessages = 0
				for i := range a.allChats {
					if a.allChats[i].ID == a.selectedChat.ID {
						a.allChats[i].MyMessages = 0
						break
					}
				}
				a.applyFilter()
			}
			return a, nil
		}
		return a, msg.nextCmd

	case reactionProgressMsg:
		if msg.err != nil {
			a.deletingErr = msg.err.Error()
			a.deletingDone = true
			return a, nil
		}
		a.deletingStatus = fmt.Sprintf("Removing reactions... checked %d, removed %d", msg.checked, msg.removed)
		a.deletingPct = 0
		if msg.done {
			a.deletingDone = true
			a.deletingStatus = fmt.Sprintf("Done! Removed %d reactions.", msg.removed)
			return a, nil
		}
		return a, msg.nextCmd

	case exportProgressMsg:
		if msg.err != nil {
			a.deletingErr = msg.err.Error()
			a.deletingDone = true
			return a, nil
		}
		if msg.total > 0 {
			a.deletingPct = float64(msg.exported) / float64(msg.total)
		}
		a.deletingStatus = fmt.Sprintf("Exporting messages... %d/%d", msg.exported, msg.total)
		if msg.done {
			a.deletingDone = true
			a.deletingStatus = fmt.Sprintf("Done! Exported %d messages to %s", msg.exported, msg.filePath)
			return a, nil
		}
		return a, msg.nextCmd

	case leftChannelsMsg:
		if msg.err != nil {
			a.loadingLeft = false
			a.deletingErr = msg.err.Error()
			return a, nil
		}
		a.leftFound = msg.found
		if msg.done {
			a.loadingLeft = false
			a.leftLoaded = true
			a.allChats = append(a.allChats, msg.chats...)
			a.applyFilter()
			return a, nil
		}
		return a, msg.nextCmd

	case leftGroupMsg:
		for i, c := range a.allChats {
			if a.selectedChat != nil && c.ID == a.selectedChat.ID {
				a.allChats = append(a.allChats[:i], a.allChats[i+1:]...)
				break
			}
		}
		a.applyFilter()
		a.selectedChat = nil
		a.screen = screenGroupList
		return a, nil

	case authErrMsg:
		a.authErr = msg.err.Error()
		return a, nil

	case needPasswordMsg:
		a.screen = screenPassword
		a.authErr = ""
		a.passInput.Focus()
		return a, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		if a.screen == screenPicker {
			if msg.String() == "enter" {
				if item, ok := a.pickerList.SelectedItem().(accountItem); ok {
					a.Selected = item.phone
				}
				return a, tea.Quit
			}
			if msg.String() == "q" {
				return a, tea.Quit
			}
		}

		if msg.String() == "enter" && a.screen == screenAuth {
			code := a.authInput.Value()
			return a, func() tea.Msg {
				err := tg.SignIn(a.ctx, a.client, a.phone, code, a.codeHash)
				if err != nil {
					if errors.Is(err, auth.ErrPasswordAuthNeeded) {
						return needPasswordMsg{}
					}
					return authErrMsg{err: err}
				}
				selfID, _ := tg.GetSelfID(a.ctx, a.client)
				return authDoneMsg{selfID: selfID}
			}
		}

		if msg.String() == "enter" && a.screen == screenPassword {
			password := a.passInput.Value()
			return a, func() tea.Msg {
				err := tg.CheckPassword(a.ctx, a.client, password)
				if err != nil {
					return authErrMsg{err: err}
				}
				selfID, _ := tg.GetSelfID(a.ctx, a.client)
				return authDoneMsg{selfID: selfID}
			}
		}

		if a.screen == screenConfirm {
			if msg.String() == "y" {
				switch a.confirmAction {
				case "export":
					a.screen = screenDeleting
					a.deletingStatus = "Starting export..."
					a.deletingDone = false
					a.deletingErr = ""
					a.deletingPct = 0
					ch := make(chan tg.ExportProgress, 1)
					go tg.ExportMyMessages(a.ctx, a.client, *a.selectedChat, a.selfID, a.phone, ch)
					return a, waitForExport(ch)
				case "delete_msgs":
					a.screen = screenDeleting
					a.deletingStatus = "Starting deletion..."
					a.deletingDone = false
					a.deletingErr = ""
					a.deletingPct = 0
					ch := make(chan tg.DeleteProgress, 1)
					go tg.DeleteMyMessages(a.ctx, a.client, *a.selectedChat, a.selfID, ch)
					return a, waitForDelete(ch)
				case "delete_reactions":
					a.screen = screenDeleting
					a.deletingStatus = "Starting reaction removal..."
					a.deletingDone = false
					a.deletingErr = ""
					a.deletingPct = 0
					ch := make(chan tg.ReactionProgress, 1)
					go tg.RemoveMyReactions(a.ctx, a.client, *a.selectedChat, ch)
					return a, waitForReactions(ch)
				case "leave":
					return a, func() tea.Msg {
						err := tg.LeaveChat(a.ctx, a.client, *a.selectedChat, a.selfID)
						if err != nil {
							return authErrMsg{err: err}
						}
						return leftGroupMsg{}
					}
				}
			}
			if msg.String() == "esc" || msg.String() == "n" {
				a.screen = screenActions
				return a, nil
			}
			return a, nil
		}

		if a.screen == screenDeleting && a.deletingDone {
			if msg.String() == "esc" || msg.String() == "enter" {
				a.screen = screenActions
				a.deletingDone = false
				a.deletingErr = ""
				a.deletingStatus = ""
				a.deletingPct = 0
				return a, nil
			}
			return a, nil
		}

		if msg.String() == "enter" && a.screen == screenGroupList {
			if item, ok := a.list.SelectedItem().(Group); ok {
				if item.isLeft && item.myMessages <= 0 {
					return a, nil
				}
				for _, c := range a.allChats {
					if c.Title == item.title && c.Type == item.chatType {
						a.openActions(c)
						break
					}
				}
			}
			return a, nil
		}

		if msg.String() == "enter" && a.screen == screenActions {
			if item, ok := a.actionList.SelectedItem().(ActionItem); ok {
				switch item.key {
				case "view":
					a.screen = screenMessages
					a.messages = nil
					a.nextOffsetID = 0
					a.hasMore = true
					a.loadingMore = true
					a.msgList.Title = a.selectedChat.Title + " — Messages"
					a.msgList.SetItems([]list.Item{})
					return a, a.fetchMessagesCmd(0)
				case "export":
					count := ""
					if a.selectedChat.MyMessages >= 0 {
						count = fmt.Sprintf(" (%d messages)", a.selectedChat.MyMessages)
					}
					a.confirmText = fmt.Sprintf("Export your messages from \"%s\"%s to file?\n\nPress [y] to confirm, [esc] to cancel", a.selectedChat.Title, count)
					a.confirmAction = "export"
					a.screen = screenConfirm
				case "delete_msgs":
					count := ""
					if a.selectedChat.MyMessages >= 0 {
						count = fmt.Sprintf(" (%d messages)", a.selectedChat.MyMessages)
					}
					a.confirmText = fmt.Sprintf("Delete all your messages in \"%s\"%s?\n\nPress [y] to confirm, [esc] to cancel", a.selectedChat.Title, count)
					a.confirmAction = "delete_msgs"
					a.screen = screenConfirm
				case "delete_reactions":
					a.confirmText = fmt.Sprintf("Remove all your reactions in \"%s\"?\n\nPress [y] to confirm, [esc] to cancel", a.selectedChat.Title)
					a.confirmAction = "delete_reactions"
					a.screen = screenConfirm
				case "leave":
					a.confirmText = fmt.Sprintf("Leave \"%s\"?\n\nPress [y] to confirm, [esc] to cancel", a.selectedChat.Title)
					a.confirmAction = "leave"
					a.screen = screenConfirm
				}
			}
			return a, nil
		}

		if msg.String() == "enter" && a.screen == screenMessages {
			if a.hasMore && !a.loadingMore {
				idx := a.msgList.Index()
				if idx == len(a.msgList.Items())-1 {
					a.loadingMore = true
					return a, a.fetchMessagesCmd(a.nextOffsetID)
				}
			}
			return a, nil
		}

		if msg.String() == "esc" && a.screen == screenActions {
			a.screen = screenGroupList
			a.selectedChat = nil
			return a, nil
		}

		if msg.String() == "esc" && a.screen == screenMessages {
			a.screen = screenActions
			a.messages = nil
			return a, nil
		}

		if msg.String() == "l" && a.screen == screenGroupList && !a.leftLoaded && !a.loadingLeft {
			a.loadingLeft = true
			a.leftFound = 0
			ch := make(chan tg.LeftChannelsProgress, 1)
			go tg.GetLeftChannels(a.ctx, a.client, a.selfID, ch)
			return a, waitForLeftChannels(ch)
		}

		if msg.String() == "a" && a.screen == screenGroupList {
			a.ExitAction = "add"
			return a, tea.Quit
		}

		if msg.String() == "s" && a.screen == screenGroupList && a.accountCount > 1 {
			a.ExitAction = "switch"
			return a, tea.Quit
		}

		if msg.String() == "tab" && a.screen == screenGroupList {
			a.filter = (a.filter + 1) % a.filterCount()
			a.applyFilter()
			return a, nil
		}

		if msg.String() == "shift+tab" && a.screen == screenGroupList {
			fc := a.filterCount()
			a.filter = (a.filter - 1 + fc) % fc
			a.applyFilter()
			return a, nil
		}
	}

	var cmd tea.Cmd
	switch a.screen {
	case screenPicker:
		a.pickerList, cmd = a.pickerList.Update(msg)
	case screenAuth:
		a.authInput, cmd = a.authInput.Update(msg)
	case screenPassword:
		a.passInput, cmd = a.passInput.Update(msg)
	case screenGroupList:
		a.list, cmd = a.list.Update(msg)
	case screenActions:
		a.actionList, cmd = a.actionList.Update(msg)
	case screenMessages:
		a.msgList, cmd = a.msgList.Update(msg)
	}
	return a, cmd
}

func (a App) View() tea.View {
	var v tea.View
	v.AltScreen = true

	switch a.screen {
	case screenPicker:
		s := "🧹 tgmsgcleaner v0.1\n\n"
		s += a.pickerList.View()
		v.SetContent(s)
	case screenAuth:
		s := "🧹 tgmsgcleaner v0.1\n\nEnter your Telegram auth code:\n\n" + a.authInput.View() + "\n"
		if a.authErr != "" {
			s += "\n⚠️  " + a.authErr + "\n"
		}
		if a.codeHash == "" {
			s += "\nSending code..."
		}
		v.SetContent(s)
	case screenPassword:
		s := "🧹 tgmsgcleaner v0.1\n\nEnter your 2FA password:\n\n" + a.passInput.View() + "\n"
		if a.authErr != "" {
			s += "\n⚠️  " + a.authErr + "\n"
		}
		v.SetContent(s)
	case screenGroupList:
		content := "🧹 tgmsgcleaner v0.1\n\n"
		filterBar := "  Filter [Tab]: "
		fc := a.filterCount()
		for i := 0; i < fc; i++ {
			name := filterNames[i]
			if i == a.filter {
				filterBar += "[" + name + "]"
			} else {
				filterBar += " " + name + " "
			}
			if i < len(filterNames)-1 {
				filterBar += " "
			}
		}
		content += filterBar + "\n"
		if a.loadingMessages && a.loadTotal > 0 {
			pct := float64(a.loadedCount) / float64(a.loadTotal)
			content += fmt.Sprintf("  Counting messages... %d/%d\n", a.loadedCount, a.loadTotal)
			content += "  " + a.progress.ViewAs(pct) + "\n"
		}
		if a.loadingLeft {
			content += fmt.Sprintf("  Loading left channels... found %d\n", a.leftFound)
		}
		content += "\n" + a.list.View()
		content += "\n" + a.statusBar()
		v.SetContent(content)
	case screenActions:
		v.SetContent(a.actionList.View())
	case screenMessages:
		v.SetContent(a.msgList.View())
	case screenConfirm:
		v.SetContent("\n  " + a.confirmText + "\n")
	case screenDeleting:
		content := "\n  " + a.deletingStatus + "\n"
		if a.deletingPct > 0 && !a.deletingDone {
			content += "\n  " + a.progress.ViewAs(a.deletingPct) + "\n"
		}
		if a.deletingErr != "" {
			content += "\n  Error: " + a.deletingErr + "\n"
		}
		if a.deletingDone {
			content += "\n  Press [Enter] or [Esc] to go back.\n"
		}
		v.SetContent(content)
	}
	return v
}

func (g Group) Title() string {
	if g.isLeft {
		return tagLeft + " " + g.title
	}
	switch g.chatType {
	case tg.ChatTypeGroup:
		return tagGroup + " " + g.title
	case tg.ChatTypeSupergroup:
		return tagSupergroup + " " + g.title
	case tg.ChatTypeChannel:
		return tagChannel + " " + g.title
	default:
		return g.title
	}
}
func (g Group) Description() string { return g.desc }
func (g Group) FilterValue() string { return g.title }
