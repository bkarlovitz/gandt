package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/bkarlovitz/gandt/internal/gmail"
	gandtsync "github.com/bkarlovitz/gandt/internal/sync"
	tea "github.com/charmbracelet/bubbletea"
)

type Mode int

const (
	ModeNormal Mode = iota
	ModeSearch
	ModeCompose
	ModeCommand
	ModeHelp
	ModeLabelPrompt
	ModeCacheDashboard
	ModeCachePolicyEditor
	ModeAccountSwitcher
)

type Pane int

const (
	PaneLabels Pane = iota
	PaneList
	PaneReader
)

type Model struct {
	config                config.Config
	keys                  KeyMap
	styles                Styles
	mailbox               Mailbox
	accounts              []AccountState
	activeAccount         int
	mode                  Mode
	width                 int
	height                int
	focus                 Pane
	selectedLabel         int
	selectedMessage       int
	selectedThreadMessage int
	readerOpen            bool
	renderMode            string
	showQuotes            bool
	quitting              bool
	commandInput          string
	labelPrompt           labelPromptState
	statusMessage         string
	fatalError            string
	offline               bool
	addingAccount         bool
	addAccount            AccountAdder
	removingAccount       bool
	removeAccount         AccountRemover
	pendingRemoveAccount  string
	replacingCreds        bool
	replaceCreds          CredentialReplacer
	threadLoader          ThreadLoader
	loadingThreadID       string
	manualRefresher       ManualRefresher
	refreshingAccount     string
	toastMessage          string
	triageActor           TriageActor
	cacheDashboardLoader  CacheDashboardLoader
	loadingCacheDashboard bool
	cacheDashboard        CacheDashboard
	cachePolicyStore      CachePolicyStore
	loadingCachePolicies  bool
	savingCachePolicy     bool
	cachePolicyTable      CachePolicyTable
	selectedCachePolicy   int
	cacheExclusionStore   CacheExclusionStore
	pendingCacheExclusion *CacheExclusionPreview
	loadingCacheExclusion bool
	cachePurgeStore       CachePurgeStore
	loadingCachePurge     bool
	pendingCachePurge     *CachePurgePreview
	cacheWipeStore        CacheWipeStore
	pendingCacheWipeStep  int
	loadingCacheWipe      bool
	nextActionID          int
	pendingActions        map[int]triageSnapshot
	undo                  *undoState
	now                   func() time.Time
	syncCoordinator       SyncCoordinator
	syncContext           context.Context
	syncCancel            context.CancelFunc
	syncHadInput          bool
}

type Option func(*Model)

type triageSnapshot struct {
	Mailbox               Mailbox
	SelectedMessage       int
	SelectedThreadMessage int
}

type undoState struct {
	Request  TriageActionRequest
	Snapshot triageSnapshot
	Expires  time.Time
}

type labelPromptState struct {
	Action TriageActionKind
	Input  string
}

func WithAccountAdder(adder AccountAdder) Option {
	return func(m *Model) {
		if adder != nil {
			m.addAccount = adder
		}
	}
}

func WithAccountRemover(remover AccountRemover) Option {
	return func(m *Model) {
		if remover != nil {
			m.removeAccount = remover
		}
	}
}

func WithCredentialReplacer(replacer CredentialReplacer) Option {
	return func(m *Model) {
		if replacer != nil {
			m.replaceCreds = replacer
		}
	}
}

func WithMailbox(mailbox Mailbox) Option {
	return func(m *Model) {
		m.mailbox = mailbox
		m.accounts = []AccountState{{Account: mailbox.Account, Mailbox: mailbox, Unread: unreadCount(mailbox)}}
		m.activeAccount = 0
	}
}

func WithAccounts(accounts []AccountState) Option {
	return func(m *Model) {
		if len(accounts) == 0 {
			return
		}
		m.accounts = normalizeAccountStates(accounts)
		m.activeAccount = 0
		m.mailbox = m.accounts[0].Mailbox
	}
}

func WithThreadLoader(loader ThreadLoader) Option {
	return func(m *Model) {
		if loader != nil {
			m.threadLoader = loader
		}
	}
}

func WithManualRefresher(refresher ManualRefresher) Option {
	return func(m *Model) {
		if refresher != nil {
			m.manualRefresher = refresher
		}
	}
}

func WithTriageActor(actor TriageActor) Option {
	return func(m *Model) {
		if actor != nil {
			m.triageActor = actor
		}
	}
}

func WithCacheDashboardLoader(loader CacheDashboardLoader) Option {
	return func(m *Model) {
		if loader != nil {
			m.cacheDashboardLoader = loader
		}
	}
}

func WithCachePolicyStore(store CachePolicyStore) Option {
	return func(m *Model) {
		if store != nil {
			m.cachePolicyStore = store
		}
	}
}

func WithCacheExclusionStore(store CacheExclusionStore) Option {
	return func(m *Model) {
		if store != nil {
			m.cacheExclusionStore = store
		}
	}
}

func WithCachePurgeStore(store CachePurgeStore) Option {
	return func(m *Model) {
		if store != nil {
			m.cachePurgeStore = store
		}
	}
}

func WithCacheWipeStore(store CacheWipeStore) Option {
	return func(m *Model) {
		if store != nil {
			m.cacheWipeStore = store
		}
	}
}

func WithNow(fn func() time.Time) Option {
	return func(m *Model) {
		if fn != nil {
			m.now = fn
		}
	}
}

type SyncCoordinator interface {
	Next(context.Context, bool) gandtsync.CoordinatorUpdate
}

func WithSyncCoordinator(coordinator SyncCoordinator) Option {
	return func(m *Model) {
		if coordinator != nil {
			m.syncCoordinator = coordinator
		}
	}
}

func New(cfg config.Config, opts ...Option) Model {
	model := Model{
		config:         cfg,
		keys:           DefaultKeyMap(),
		styles:         NewStyles(os.Getenv("NO_COLOR") != ""),
		mailbox:        fakeMailbox(),
		mode:           ModeNormal,
		focus:          PaneList,
		renderMode:     string(cfg.UI.RenderModeDefault),
		now:            time.Now,
		pendingActions: map[int]triageSnapshot{},
	}
	for _, opt := range opts {
		opt(&model)
	}
	if len(model.accounts) == 0 {
		model.accounts = []AccountState{{Account: model.mailbox.Account, Mailbox: model.mailbox, Unread: unreadCount(model.mailbox)}}
	}
	if model.syncCoordinator != nil {
		model.syncContext, model.syncCancel = context.WithCancel(context.Background())
	}
	return model
}

func (m Model) Init() tea.Cmd {
	return m.runSyncCycle(true)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
	case addAccountDoneMsg:
		m.addingAccount = false
		if msg.Err != nil {
			m.statusMessage = "add account failed: " + msg.Err.Error()
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("added account %s", msg.Result.Account)
		m.mailbox = RealAccountMailbox(msg.Result.Account, msg.Result.Labels, msg.Result.MessagesByLabel)
		m.accounts = append(m.accounts, AccountState{
			Account:     msg.Result.Account,
			DisplayName: msg.Result.DisplayName,
			Color:       msg.Result.Color,
			SyncStatus:  "synced",
			Unread:      unreadCount(m.mailbox),
			Mailbox:     m.mailbox,
		})
		m.activeAccount = len(m.accounts) - 1
		m.selectedLabel = clamp(m.selectedLabel, 0, len(m.mailbox.Labels)-1)
	case removeAccountDoneMsg:
		m.removingAccount = false
		m.pendingRemoveAccount = ""
		if msg.Err != nil {
			m.statusMessage = "remove account failed: " + msg.Err.Error()
			return m, nil
		}
		if msg.Result.RevokeError {
			m.statusMessage = fmt.Sprintf("removed account %s; token revoke failed", msg.Result.Account)
		} else {
			m.statusMessage = fmt.Sprintf("removed account %s", msg.Result.Account)
		}
		if strings.EqualFold(m.mailbox.Account, msg.Result.Account) {
			m.mailbox = NoAccountMailbox()
			m.selectedLabel = 0
			m.selectedMessage = 0
			m.readerOpen = false
		}
		m.removeAccountState(msg.Result.Account)
	case replaceCredentialsDoneMsg:
		m.replacingCreds = false
		if msg.Err != nil {
			m.statusMessage = "replace credentials failed: " + msg.Err.Error()
			return m, nil
		}
		m.statusMessage = "replaced OAuth client credentials"
	case threadLoadDoneMsg:
		m.loadingThreadID = ""
		if msg.Err != nil {
			if IsOfflineError(msg.Err) {
				m.offline = true
				m.statusMessage = "offline: cached mail available"
				return m, nil
			}
			m.statusMessage = "load thread failed: " + msg.Err.Error()
			return m, nil
		}
		m.applyThreadLoadResult(msg.Result)
		m.statusMessage = "loaded thread"
	case refreshDoneMsg:
		m.refreshingAccount = ""
		if msg.Err != nil {
			m.applyStatusOrError("sync failed: "+msg.Err.Error(), msg.Err, msg.Request.Account)
			return m, nil
		}
		summary := msg.Result.Summary
		if summary == "" {
			summary = refreshDoneSummary(msg.Request)
		}
		m.statusMessage = summary
		m.toastMessage = summary
	case triageDoneMsg:
		snapshot, ok := m.pendingActions[msg.ID]
		delete(m.pendingActions, msg.ID)
		if msg.Err != nil {
			if ok {
				m.mailbox = snapshot.Mailbox
				m.selectedMessage = snapshot.SelectedMessage
				m.selectedThreadMessage = snapshot.SelectedThreadMessage
			}
			if m.undo != nil && sameTriageRequest(m.undo.Request, msg.Request) {
				m.undo = nil
			}
			m.applyStatusOrError("action failed: "+msg.Err.Error(), msg.Err, msg.Request.Account)
			return m, nil
		}
		m.reconcileTriageResult(msg.Request, msg.Result)
		summary := msg.Result.Summary
		if msg.Request.Undo {
			summary = triageDoneSummary(msg.Request)
		} else if summary == "" {
			summary = triageDoneSummary(msg.Request)
		}
		m.statusMessage = summary
		m.toastMessage = summary
	case cacheDashboardDoneMsg:
		m.loadingCacheDashboard = false
		if msg.Err != nil {
			m.statusMessage = "cache dashboard failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.cacheDashboard = msg.Result
		m.mode = ModeCacheDashboard
		m.statusMessage = "cache dashboard loaded"
	case cachePolicyLoadDoneMsg:
		m.loadingCachePolicies = false
		if msg.Err != nil {
			m.statusMessage = "cache policy failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.cachePolicyTable = msg.Result
		m.selectedCachePolicy = clamp(m.selectedCachePolicy, 0, len(m.cachePolicyTable.Rows)-1)
		m.mode = ModeCachePolicyEditor
		m.statusMessage = "cache policy loaded"
	case cachePolicySaveDoneMsg:
		m.savingCachePolicy = false
		if msg.Err != nil {
			m.statusMessage = "cache policy save failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		if len(m.cachePolicyTable.Rows) > 0 {
			m.cachePolicyTable.Rows[clamp(m.selectedCachePolicy, 0, len(m.cachePolicyTable.Rows)-1)] = msg.Row
		}
		m.statusMessage = "cache policy saved"
		m.toastMessage = m.statusMessage
		if m.manualRefresher != nil {
			return m.startRefresh(RefreshAll)
		}
	case cachePolicyResetDoneMsg:
		m.savingCachePolicy = false
		if msg.Err != nil {
			m.statusMessage = "cache policy reset failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		if len(m.cachePolicyTable.Rows) > 0 {
			m.cachePolicyTable.Rows[clamp(m.selectedCachePolicy, 0, len(m.cachePolicyTable.Rows)-1)] = msg.Row
		}
		m.statusMessage = "cache policy reset"
		m.toastMessage = m.statusMessage
		if m.manualRefresher != nil {
			return m.startRefresh(RefreshAll)
		}
	case cacheExclusionPreviewDoneMsg:
		m.loadingCacheExclusion = false
		if msg.Err != nil {
			m.statusMessage = "cache exclusion preview failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.pendingCacheExclusion = &msg.Preview
		m.statusMessage = cacheExclusionPreviewSummary(msg.Preview)
		m.toastMessage = m.statusMessage
	case cacheExclusionConfirmDoneMsg:
		m.loadingCacheExclusion = false
		m.pendingCacheExclusion = nil
		if msg.Err != nil {
			m.statusMessage = "cache exclusion failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("cache exclusion saved; purged %d messages, %d attachment files", msg.Result.DeletedMessages, msg.Result.DeletedAttachmentFiles)
		if len(msg.Result.AttachmentDeleteErrors) > 0 {
			m.statusMessage += fmt.Sprintf("; %d attachment cleanup errors", len(msg.Result.AttachmentDeleteErrors))
		}
		m.toastMessage = m.statusMessage
	case cachePurgePreviewDoneMsg:
		m.loadingCachePurge = false
		if msg.Err != nil {
			m.statusMessage = "cache purge failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		if msg.Preview.Request.DryRun {
			m.pendingCachePurge = nil
		} else {
			m.pendingCachePurge = &msg.Preview
		}
		m.statusMessage = cachePurgePreviewSummary(msg.Preview)
		m.toastMessage = m.statusMessage
	case cachePurgeExecuteDoneMsg:
		m.loadingCachePurge = false
		m.pendingCachePurge = nil
		if msg.Err != nil {
			m.statusMessage = "cache purge failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("cache purge complete: deleted %d messages, %d attachment files", msg.Result.DeletedMessages, msg.Result.DeletedAttachmentFiles)
		if len(msg.Result.AttachmentDeleteErrors) > 0 {
			m.statusMessage += fmt.Sprintf("; %d attachment cleanup errors", len(msg.Result.AttachmentDeleteErrors))
		}
		m.toastMessage = m.statusMessage
	case cacheCompactDoneMsg:
		if msg.Err != nil {
			m.statusMessage = "cache compact failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.statusMessage = "cache compact complete"
		m.toastMessage = m.statusMessage
	case cacheWipeDoneMsg:
		m.loadingCacheWipe = false
		m.pendingCacheWipeStep = 0
		if msg.Err != nil {
			m.statusMessage = "cache wipe failed: " + msg.Err.Error()
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.statusMessage = fmt.Sprintf("cache wipe complete: removed %d database files, %d attachment files", msg.Result.DatabaseFilesRemoved, msg.Result.AttachmentFilesRemoved)
		if len(msg.Result.AttachmentDeleteErrors) > 0 {
			m.statusMessage += fmt.Sprintf("; %d attachment cleanup errors", len(msg.Result.AttachmentDeleteErrors))
		}
		m.toastMessage = m.statusMessage
	case SyncUpdateMsg:
		if msg.Stopped {
			return m, nil
		}
		m.applyStatusOrError(msg.Summary, msg.Err, m.mailbox.Account)
		return m, m.runSyncCycle(m.consumeSyncActivity())
	case ErrorMsg:
		m.applyStatusOrError("", msg.Err, m.mailbox.Account)
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) View() string {
	if m.quitting {
		return ""
	}
	if m.fatalError != "" {
		return "G&T\n\nFatal error: " + m.fatalError
	}

	if m.mode == ModeNormal {
		return m.renderMailbox()
	}
	if m.mode == ModeCacheDashboard {
		return m.renderCacheDashboard()
	}
	if m.mode == ModeCachePolicyEditor {
		return m.renderCachePolicyEditor()
	}
	if m.mode == ModeAccountSwitcher {
		return m.renderAccountSwitcher()
	}

	var b strings.Builder
	b.WriteString(m.styles.Header.Render("G&T"))
	b.WriteString("\n\n")

	switch m.mode {
	case ModeHelp:
		b.WriteString("Help\n\n")
		b.WriteString(m.keys.HelpText())
	case ModeSearch:
		b.WriteString("Search mode\n\nPress Esc to return.")
	case ModeCompose:
		b.WriteString("Compose mode\n\nPress Esc to return.")
	case ModeCommand:
		b.WriteString("Command mode\n\n")
		b.WriteString(m.commandInput)
		b.WriteString("\n\nPress Esc to return.")
	case ModeLabelPrompt:
		b.WriteString("Label\n\n")
		b.WriteString(m.labelPrompt.Input)
		b.WriteString("\n\nPress Esc to return.")
	case ModeAccountSwitcher:
		b.WriteString(m.renderAccountSwitcher())
	}

	return b.String()
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	m.syncHadInput = true

	if key == "ctrl+c" {
		m.stopSync()
		m.quitting = true
		return m, tea.Quit
	}
	if m.mode == ModeNormal && m.pendingCacheExclusion != nil {
		return m.handleCacheExclusionConfirmation(key)
	}
	if m.mode == ModeNormal && m.pendingCachePurge != nil {
		return m.handleCachePurgeConfirmation(key)
	}
	if m.mode == ModeNormal && m.pendingCacheWipeStep > 0 {
		return m.handleCacheWipeConfirmation(key)
	}
	if m.mode == ModeNormal && m.pendingRemoveAccount != "" {
		return m.handleRemoveAccountConfirmation(key)
	}

	switch m.mode {
	case ModeHelp:
		switch key {
		case "esc", "?":
			m.mode = ModeNormal
		case "q":
			m.stopSync()
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	case ModeCacheDashboard:
		switch key {
		case "esc":
			m.mode = ModeNormal
		case ":":
			m.mode = ModeCommand
			m.commandInput = ":"
		case "q":
			m.stopSync()
			m.quitting = true
			return m, tea.Quit
		}
		return m, nil
	case ModeCachePolicyEditor:
		return m.handleCachePolicyKey(key)
	case ModeAccountSwitcher:
		return m.handleAccountSwitcherKey(key)
	case ModeSearch, ModeCompose, ModeCommand, ModeLabelPrompt:
		if m.mode == ModeCommand {
			return m.handleCommandKey(msg)
		}
		if m.mode == ModeLabelPrompt {
			return m.handleLabelPromptKey(msg)
		}
		if key == "esc" {
			m.mode = ModeNormal
		}
		return m, nil
	}

	switch key {
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.switchAccountByOrdinal(int(key[0] - '1'))
	case "q", "esc":
		m.stopSync()
		m.quitting = true
		return m, tea.Quit
	case "?":
		m.mode = ModeHelp
	case "j", "down":
		m.moveSelection(1)
	case "k", "up":
		m.moveSelection(-1)
	case "g":
		m.jumpSelection(false)
	case "G":
		m.jumpSelection(true)
	case "enter":
		m.readerOpen = true
		m.focus = PaneReader
		return m, m.loadSelectedThreadIfNeeded()
	case "J":
		m.moveThreadMessage(1)
	case "K":
		m.moveThreadMessage(-1)
	case "N":
		m.moveSelection(1)
		m.readerOpen = true
		m.focus = PaneReader
		return m, m.loadSelectedThreadIfNeeded()
	case "P":
		m.moveSelection(-1)
		m.readerOpen = true
		m.focus = PaneReader
		return m, m.loadSelectedThreadIfNeeded()
	case "V":
		m.toggleRenderMode()
	case "B":
		m.statusMessage = "browser open unavailable in read-only mode"
	case "z":
		m.showQuotes = !m.showQuotes
		if m.showQuotes {
			m.statusMessage = "quotes shown"
		} else {
			m.statusMessage = "quotes collapsed"
		}
	case "tab":
		m.nextPane()
	case "/":
		m.mode = ModeSearch
	case "c", "f":
		m.mode = ModeCompose
	case "e":
		return m.startSelectedTriageAction(TriageArchive)
	case "#":
		return m.startSelectedTriageAction(TriageTrash)
	case "!":
		return m.startSelectedTriageAction(TriageSpam)
	case "s":
		return m.startSelectedTriageAction(TriageStar)
	case "u":
		return m.startSelectedTriageAction(TriageUnread)
	case "U":
		return m.startUndo()
	case "m":
		return m.startSelectedTriageAction(TriageMute)
	case "+":
		return m.startLabelPrompt(TriageLabelAdd)
	case "-":
		return m.startLabelPrompt(TriageLabelRemove)
	case "r":
		return m.startRefresh(RefreshDelta)
	case "R":
		return m.startRefresh(RefreshRelistLabel)
	case "ctrl+a":
		m.mode = ModeAccountSwitcher
	case ":":
		m.mode = ModeCommand
		m.commandInput = ":"
	}

	return m, nil
}

func (m Model) handleCommandKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		m.mode = ModeNormal
		m.commandInput = ""
		return m, nil
	case tea.KeyEnter:
		return m.submitCommand()
	case tea.KeyBackspace, tea.KeyCtrlH:
		if len(m.commandInput) > 1 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
		return m, nil
	}

	key := msg.String()
	if key == "esc" {
		m.mode = ModeNormal
		m.commandInput = ""
		return m, nil
	}
	if key == "backspace" || key == "ctrl+h" {
		if len(m.commandInput) > 1 {
			m.commandInput = m.commandInput[:len(m.commandInput)-1]
		}
		return m, nil
	}
	for _, r := range msg.Runes {
		m.commandInput += string(r)
	}
	return m, nil
}

func (m Model) handleCachePolicyKey(key string) (tea.Model, tea.Cmd) {
	if key == "ctrl+c" {
		m.stopSync()
		m.quitting = true
		return m, tea.Quit
	}
	switch key {
	case "esc":
		m.mode = ModeNormal
		m.statusMessage = "cache policy canceled"
	case "q":
		m.stopSync()
		m.quitting = true
		return m, tea.Quit
	case ":":
		m.mode = ModeCommand
		m.commandInput = ":"
	case "j", "down":
		m.selectedCachePolicy = clamp(m.selectedCachePolicy+1, 0, len(m.cachePolicyTable.Rows)-1)
	case "k", "up":
		m.selectedCachePolicy = clamp(m.selectedCachePolicy-1, 0, len(m.cachePolicyTable.Rows)-1)
	case "d":
		m.updateSelectedCachePolicy(cyclePolicyDepth)
	case "t":
		m.updateSelectedCachePolicy(cyclePolicyRetention)
	case "a":
		m.updateSelectedCachePolicy(cyclePolicyAttachment)
	case "+", "=":
		m.updateSelectedCachePolicy(func(row CachePolicyRow) CachePolicyRow {
			return adjustPolicyAttachmentMax(row, 1)
		})
	case "-":
		m.updateSelectedCachePolicy(func(row CachePolicyRow) CachePolicyRow {
			return adjustPolicyAttachmentMax(row, -1)
		})
	case "s":
		if m.savingCachePolicy {
			m.statusMessage = "cache policy save already running"
			return m, nil
		}
		row, ok := m.selectedCachePolicyRow()
		if !ok {
			m.statusMessage = "no cache policy selected"
			return m, nil
		}
		m.savingCachePolicy = true
		m.statusMessage = "saving cache policy..."
		return m, m.runCachePolicySave(row)
	case "x":
		if m.savingCachePolicy {
			m.statusMessage = "cache policy reset already running"
			return m, nil
		}
		row, ok := m.selectedCachePolicyRow()
		if !ok {
			m.statusMessage = "no cache policy selected"
			return m, nil
		}
		m.savingCachePolicy = true
		m.statusMessage = "resetting cache policy..."
		return m, m.runCachePolicyReset(row)
	}
	return m, nil
}

func (m *Model) updateSelectedCachePolicy(fn func(CachePolicyRow) CachePolicyRow) {
	row, ok := m.selectedCachePolicyRow()
	if !ok {
		return
	}
	m.cachePolicyTable.Rows[clamp(m.selectedCachePolicy, 0, len(m.cachePolicyTable.Rows)-1)] = fn(row)
	m.statusMessage = "cache policy edited"
}

func (m Model) selectedCachePolicyRow() (CachePolicyRow, bool) {
	if len(m.cachePolicyTable.Rows) == 0 {
		return CachePolicyRow{}, false
	}
	return m.cachePolicyTable.Rows[clamp(m.selectedCachePolicy, 0, len(m.cachePolicyTable.Rows)-1)], true
}

func (m Model) handleCacheExclusionConfirmation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		request := m.pendingCacheExclusion.Request
		m.loadingCacheExclusion = true
		m.statusMessage = "saving cache exclusion..."
		m.toastMessage = m.statusMessage
		return m, m.runCacheExclusionConfirm(request)
	case "n", "N", "esc":
		m.pendingCacheExclusion = nil
		m.statusMessage = "cache exclusion canceled"
		m.toastMessage = m.statusMessage
		return m, nil
	default:
		m.statusMessage = "confirm cache exclusion with y or n"
		m.toastMessage = m.statusMessage
		return m, nil
	}
}

func (m Model) handleCachePurgeConfirmation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		request := m.pendingCachePurge.Request
		m.loadingCachePurge = true
		m.statusMessage = "running cache purge..."
		m.toastMessage = m.statusMessage
		return m, m.runCachePurgeExecute(request)
	case "n", "N", "esc":
		m.pendingCachePurge = nil
		m.statusMessage = "cache purge canceled"
		m.toastMessage = m.statusMessage
		return m, nil
	default:
		m.statusMessage = "confirm cache purge with y or n"
		m.toastMessage = m.statusMessage
		return m, nil
	}
}

func (m Model) handleCacheWipeConfirmation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		if m.pendingCacheWipeStep == 1 {
			m.pendingCacheWipeStep = 2
			m.statusMessage = "cache wipe confirmation 2/2: press y again to delete local cache"
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.loadingCacheWipe = true
		m.statusMessage = "wiping cache..."
		m.toastMessage = m.statusMessage
		return m, m.runCacheWipe()
	case "n", "N", "esc":
		m.pendingCacheWipeStep = 0
		m.statusMessage = "cache wipe canceled"
		m.toastMessage = m.statusMessage
		return m, nil
	default:
		m.statusMessage = "confirm cache wipe with y or n"
		m.toastMessage = m.statusMessage
		return m, nil
	}
}

func (m Model) handleRemoveAccountConfirmation(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "y", "Y":
		account := m.pendingRemoveAccount
		m.removingAccount = true
		m.statusMessage = "removing account..."
		m.toastMessage = m.statusMessage
		return m, m.runRemoveAccount(account)
	case "n", "N", "esc":
		m.pendingRemoveAccount = ""
		m.statusMessage = "remove account canceled"
		m.toastMessage = m.statusMessage
		return m, nil
	default:
		m.statusMessage = "confirm account removal with y or n"
		m.toastMessage = m.statusMessage
		return m, nil
	}
}

func (m Model) handleAccountSwitcherKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc", "ctrl+a":
		m.mode = ModeNormal
	case "j", "down":
		m.activeAccount = clamp(m.activeAccount+1, 0, len(m.accounts)-1)
	case "k", "up":
		m.activeAccount = clamp(m.activeAccount-1, 0, len(m.accounts)-1)
	case "enter":
		m.applyActiveAccount()
		m.mode = ModeNormal
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		m.switchAccountByOrdinal(int(key[0] - '1'))
		m.mode = ModeNormal
	}
	return m, nil
}

func (m *Model) switchAccountByOrdinal(index int) {
	if index < 0 || index >= len(m.accounts) {
		m.statusMessage = "account shortcut unavailable"
		return
	}
	m.activeAccount = index
	m.applyActiveAccount()
}

func (m *Model) applyActiveAccount() {
	if m.activeAccount < 0 || m.activeAccount >= len(m.accounts) {
		return
	}
	m.mailbox = m.accounts[m.activeAccount].Mailbox
	m.selectedLabel = clamp(m.selectedLabel, 0, len(m.mailbox.Labels)-1)
	m.selectedMessage = 0
	m.selectedThreadMessage = 0
	m.readerOpen = false
	m.statusMessage = "switched to " + m.mailbox.Account
}

func (m *Model) removeAccountState(account string) {
	next := m.accounts[:0]
	for _, state := range m.accounts {
		if !strings.EqualFold(state.Account, account) && !strings.EqualFold(state.Mailbox.Account, account) {
			next = append(next, state)
		}
	}
	m.accounts = next
	if len(m.accounts) == 0 {
		m.activeAccount = 0
		m.mailbox = NoAccountMailbox()
		return
	}
	m.activeAccount = clamp(m.activeAccount, 0, len(m.accounts)-1)
	m.mailbox = m.accounts[m.activeAccount].Mailbox
}

func (m Model) submitCommand() (tea.Model, tea.Cmd) {
	command := strings.TrimSpace(strings.TrimPrefix(m.commandInput, ":"))
	m.mode = ModeNormal
	m.commandInput = ""
	fields := strings.Fields(command)
	commandName := command
	if len(fields) > 0 {
		commandName = fields[0]
	}

	switch commandName {
	case "add-account":
		if m.addingAccount || m.removingAccount || m.replacingCreds {
			m.statusMessage = "account operation already running"
			return m, nil
		}
		if m.addAccount == nil {
			m.statusMessage = "add account unavailable"
			return m, nil
		}
		m.addingAccount = true
		m.statusMessage = "adding account..."
		return m, m.runAddAccount()
	case "remove-account":
		if m.addingAccount || m.removingAccount || m.replacingCreds {
			m.statusMessage = "account operation already running"
			return m, nil
		}
		if m.removeAccount == nil {
			m.statusMessage = "remove account unavailable"
			return m, nil
		}
		account := m.mailbox.Account
		if len(fields) > 1 {
			account = fields[1]
		}
		if account == "" || (m.mailbox.NoAccounts && len(fields) <= 1) {
			m.statusMessage = "remove account unavailable: no account"
			return m, nil
		}
		m.pendingRemoveAccount = account
		m.statusMessage = fmt.Sprintf("remove account %s? y confirm / n cancel", account)
		m.toastMessage = m.statusMessage
		return m, nil
	case "replace-credentials":
		if m.addingAccount || m.removingAccount || m.replacingCreds {
			m.statusMessage = "account operation already running"
			return m, nil
		}
		if m.replaceCreds == nil {
			m.statusMessage = "replace credentials unavailable"
			return m, nil
		}
		m.replacingCreds = true
		m.statusMessage = "replacing credentials..."
		return m, m.runReplaceCredentials()
	case "sync-all":
		return m.startRefresh(RefreshAll)
	case "cache":
		if m.loadingCacheDashboard {
			m.statusMessage = "cache dashboard already loading"
			m.toastMessage = m.statusMessage
			return m, nil
		}
		if m.cacheDashboardLoader == nil {
			m.statusMessage = "cache dashboard unavailable"
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.loadingCacheDashboard = true
		m.statusMessage = "loading cache dashboard..."
		m.toastMessage = m.statusMessage
		return m, m.runCacheDashboard()
	case "cache-policy":
		if m.loadingCachePolicies || m.savingCachePolicy {
			m.statusMessage = "cache policy operation already running"
			m.toastMessage = m.statusMessage
			return m, nil
		}
		if m.cachePolicyStore == nil {
			m.statusMessage = "cache policy unavailable"
			m.toastMessage = m.statusMessage
			return m, nil
		}
		m.loadingCachePolicies = true
		m.statusMessage = "loading cache policy..."
		m.toastMessage = m.statusMessage
		return m, m.runCachePolicyLoad()
	case "cache-exclude":
		return m.startCacheExclusionPreview(fields)
	case "cache-purge":
		return m.startCachePurgePreview(fields)
	case "cache-compact":
		return m.startCacheCompact()
	case "cache-wipe":
		return m.startCacheWipe()
	case "quit", "q":
		m.stopSync()
		m.quitting = true
		return m, tea.Quit
	case "":
		return m, nil
	default:
		m.statusMessage = "unknown command: " + command
		return m, nil
	}
}

func (m Model) startCacheExclusionPreview(fields []string) (tea.Model, tea.Cmd) {
	if len(fields) < 3 {
		m.statusMessage = "usage: :cache-exclude <sender|domain|label> <value>"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	matchType := fields[1]
	if !validCacheExclusionType(matchType) {
		m.statusMessage = "invalid cache exclusion type: " + matchType
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if m.cacheExclusionStore == nil {
		m.statusMessage = "cache exclusion unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if m.loadingCacheExclusion {
		m.statusMessage = "cache exclusion already running"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	request := CacheExclusionRequest{
		Account:    m.mailbox.Account,
		MatchType:  matchType,
		MatchValue: strings.TrimSpace(strings.Join(fields[2:], " ")),
	}
	m.loadingCacheExclusion = true
	m.pendingCacheExclusion = nil
	m.statusMessage = "previewing cache exclusion..."
	m.toastMessage = m.statusMessage
	return m, m.runCacheExclusionPreview(request)
}

func (m Model) startCachePurgePreview(fields []string) (tea.Model, tea.Cmd) {
	if m.cachePurgeStore == nil {
		m.statusMessage = "cache purge unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if m.loadingCachePurge {
		m.statusMessage = "cache purge already running"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	request, err := parseCachePurgeRequest(fields)
	if err != nil {
		m.statusMessage = err.Error()
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if request.Account == "" {
		request.Account = m.mailbox.Account
	}
	m.loadingCachePurge = true
	m.pendingCachePurge = nil
	m.statusMessage = "planning cache purge..."
	m.toastMessage = m.statusMessage
	return m, m.runCachePurgePreview(request)
}

func (m Model) startCacheCompact() (tea.Model, tea.Cmd) {
	if m.cachePurgeStore == nil {
		m.statusMessage = "cache compact unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	m.statusMessage = "compacting cache..."
	m.toastMessage = m.statusMessage
	return m, m.runCacheCompact()
}

func (m Model) startCacheWipe() (tea.Model, tea.Cmd) {
	if m.cacheWipeStore == nil {
		m.statusMessage = "cache wipe unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if m.loadingCacheWipe {
		m.statusMessage = "cache wipe already running"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	m.pendingCacheWipeStep = 1
	m.statusMessage = "cache wipe confirmation 1/2: press y to continue"
	m.toastMessage = m.statusMessage
	return m, nil
}

func cacheExclusionPreviewSummary(preview CacheExclusionPreview) string {
	return fmt.Sprintf("cache exclusion preview: %d messages, %d bodies, %d attachments, %s; y confirm / n cancel",
		preview.MessageCount,
		preview.BodyCount,
		preview.AttachmentCount,
		formatBytes(preview.EstimatedBytes),
	)
}

func (m Model) startLabelPrompt(action TriageActionKind) (tea.Model, tea.Cmd) {
	if len(m.mailbox.Messages) == 0 {
		m.statusMessage = "no message selected"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if action == TriageLabelRemove && len(removableLabels(m.mailbox.Messages[m.selectedMessage].LabelIDs)) == 0 {
		m.statusMessage = "no removable labels"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	m.mode = ModeLabelPrompt
	m.labelPrompt = labelPromptState{Action: action}
	if action == TriageLabelAdd {
		m.statusMessage = "add label"
	} else {
		m.statusMessage = "remove label"
	}
	return m, nil
}

func (m Model) handleLabelPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyEsc:
		return m.cancelLabelPrompt()
	case tea.KeyEnter:
		return m.submitLabelPrompt()
	case tea.KeyBackspace, tea.KeyCtrlH:
		if len(m.labelPrompt.Input) > 0 {
			m.labelPrompt.Input = m.labelPrompt.Input[:len(m.labelPrompt.Input)-1]
		}
		return m, nil
	}

	key := msg.String()
	if key == "esc" {
		return m.cancelLabelPrompt()
	}
	if key == "backspace" || key == "ctrl+h" {
		if len(m.labelPrompt.Input) > 0 {
			m.labelPrompt.Input = m.labelPrompt.Input[:len(m.labelPrompt.Input)-1]
		}
		return m, nil
	}
	for _, r := range msg.Runes {
		m.labelPrompt.Input += string(r)
	}
	return m, nil
}

func (m Model) cancelLabelPrompt() (tea.Model, tea.Cmd) {
	m.mode = ModeNormal
	m.labelPrompt = labelPromptState{}
	m.statusMessage = "label canceled"
	return m, nil
}

func (m Model) submitLabelPrompt() (tea.Model, tea.Cmd) {
	action := m.labelPrompt.Action
	input := strings.TrimSpace(m.labelPrompt.Input)
	m.mode = ModeNormal
	m.labelPrompt = labelPromptState{}

	labelID, labelName, create := m.resolvePromptLabel(action, input)
	if labelID == "" {
		m.statusMessage = "label unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	request := m.triageRequest(action, labelID)
	request.LabelName = labelName
	request.CreateLabel = create
	request.Add = action == TriageLabelAdd
	return m.startTriageAction(request)
}

func (m Model) resolvePromptLabel(action TriageActionKind, input string) (string, string, bool) {
	if action == TriageLabelRemove && input == "" {
		message := m.mailbox.Messages[clamp(m.selectedMessage, 0, len(m.mailbox.Messages)-1)]
		labels := removableLabels(message.LabelIDs)
		if len(labels) == 0 {
			return "", "", false
		}
		label := m.labelByID(labels[0])
		return labels[0], firstNonEmpty(label.Name, labels[0]), false
	}
	if input == "" {
		return "", "", false
	}
	if label := m.labelByNameOrID(input); label.ID != "" {
		return labelKey(label), label.Name, false
	}
	if action == TriageLabelAdd {
		return userLabelID(input), input, true
	}
	return "", "", false
}

func (m Model) runAddAccount() tea.Cmd {
	return func() tea.Msg {
		result, err := m.addAccount.AddAccount()
		return addAccountDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runRemoveAccount(account string) tea.Cmd {
	return func() tea.Msg {
		result, err := m.removeAccount.RemoveAccount(account)
		return removeAccountDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runReplaceCredentials() tea.Cmd {
	return func() tea.Msg {
		return replaceCredentialsDoneMsg{Err: m.replaceCreds.ReplaceCredentials()}
	}
}

func (m Model) runLoadThread(message Message) tea.Cmd {
	return func() tea.Msg {
		result, err := m.threadLoader.LoadThread(ThreadLoadRequest{
			Account: m.mailbox.Account,
			Message: message,
		})
		return threadLoadDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runRefresh(request RefreshRequest) tea.Cmd {
	return func() tea.Msg {
		result, err := m.manualRefresher.Refresh(request)
		return refreshDoneMsg{Request: request, Result: result, Err: err}
	}
}

func (m Model) runTriageAction(id int, request TriageActionRequest) tea.Cmd {
	return func() tea.Msg {
		result, err := m.triageActor.ApplyAction(request)
		return triageDoneMsg{ID: id, Request: request, Result: result, Err: err}
	}
}

func (m Model) runCacheDashboard() tea.Cmd {
	return func() tea.Msg {
		result, err := m.cacheDashboardLoader.LoadCacheDashboard()
		return cacheDashboardDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runCachePolicyLoad() tea.Cmd {
	return func() tea.Msg {
		result, err := m.cachePolicyStore.LoadCachePolicies()
		return cachePolicyLoadDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runCachePolicySave(row CachePolicyRow) tea.Cmd {
	return func() tea.Msg {
		result, err := m.cachePolicyStore.SaveCachePolicy(row)
		return cachePolicySaveDoneMsg{Row: result, Err: err}
	}
}

func (m Model) runCachePolicyReset(row CachePolicyRow) tea.Cmd {
	return func() tea.Msg {
		result, err := m.cachePolicyStore.ResetCachePolicy(row)
		return cachePolicyResetDoneMsg{Row: result, Err: err}
	}
}

func (m Model) runCacheExclusionPreview(request CacheExclusionRequest) tea.Cmd {
	return func() tea.Msg {
		preview, err := m.cacheExclusionStore.PreviewCacheExclusion(request)
		return cacheExclusionPreviewDoneMsg{Preview: preview, Err: err}
	}
}

func (m Model) runCacheExclusionConfirm(request CacheExclusionRequest) tea.Cmd {
	return func() tea.Msg {
		result, err := m.cacheExclusionStore.ConfirmCacheExclusion(request)
		return cacheExclusionConfirmDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runCachePurgePreview(request CachePurgeRequest) tea.Cmd {
	return func() tea.Msg {
		preview, err := m.cachePurgeStore.PlanCachePurge(request)
		return cachePurgePreviewDoneMsg{Preview: preview, Err: err}
	}
}

func (m Model) runCachePurgeExecute(request CachePurgeRequest) tea.Cmd {
	return func() tea.Msg {
		result, err := m.cachePurgeStore.ExecuteCachePurge(request)
		return cachePurgeExecuteDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runCacheCompact() tea.Cmd {
	return func() tea.Msg {
		return cacheCompactDoneMsg{Err: m.cachePurgeStore.CompactCache()}
	}
}

func (m Model) runCacheWipe() tea.Cmd {
	return func() tea.Msg {
		result, err := m.cacheWipeStore.WipeCache()
		return cacheWipeDoneMsg{Result: result, Err: err}
	}
}

func (m Model) runSyncCycle(active bool) tea.Cmd {
	if m.syncCoordinator == nil {
		return nil
	}
	ctx := m.syncContext
	if ctx == nil {
		ctx = context.Background()
	}
	return func() tea.Msg {
		update := m.syncCoordinator.Next(ctx, active)
		return SyncUpdateMsg{
			AccountID: update.AccountID,
			Summary:   update.Summary,
			Err:       update.Err,
			Stopped:   update.Stopped,
			Fallback:  update.Fallback,
		}
	}
}

func (m *Model) applyStatusOrError(summary string, err error, account string) {
	if err == nil {
		if summary != "" {
			m.statusMessage = summary
		}
		return
	}
	if IsFatalError(err) {
		m.fatalError = err.Error()
		return
	}
	if errors.Is(err, gmail.ErrUnauthorized) {
		m.statusMessage = "re-authenticate " + firstNonEmpty(account, m.mailbox.Account)
		m.toastMessage = m.statusMessage
		return
	}
	if summary == "" {
		summary = err.Error()
	}
	m.statusMessage = summary
	m.toastMessage = summary
}

func (m Model) startRefresh(kind RefreshKind) (tea.Model, tea.Cmd) {
	if m.manualRefresher == nil {
		m.statusMessage = "sync unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	request := m.refreshRequest(kind)
	if request.Account == "" {
		m.statusMessage = "sync unavailable: no account"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if m.refreshingAccount == request.Account {
		m.statusMessage = "sync already running for " + request.Account
		m.toastMessage = m.statusMessage
		return m, nil
	}
	m.refreshingAccount = request.Account
	m.statusMessage = refreshProgressSummary(request)
	m.toastMessage = m.statusMessage
	return m, m.runRefresh(request)
}

func (m Model) startSelectedTriageAction(kind TriageActionKind) (tea.Model, tea.Cmd) {
	if len(m.mailbox.Messages) == 0 {
		m.statusMessage = "no message selected"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	return m.startTriageAction(m.triageRequest(kind, ""))
}

func (m Model) startTriageAction(request TriageActionRequest) (tea.Model, tea.Cmd) {
	return m.startTriageActionWithUndo(request, true, nil)
}

func (m Model) startTriageActionWithUndo(request TriageActionRequest, recordUndo bool, overrideSnapshot *triageSnapshot) (tea.Model, tea.Cmd) {
	if m.triageActor == nil {
		m.statusMessage = "action unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if request.Account == "" {
		request.Account = m.mailbox.Account
	}
	if request.MessageID == "" || request.ThreadID == "" {
		if len(m.mailbox.Messages) == 0 {
			m.statusMessage = "no message selected"
			m.toastMessage = m.statusMessage
			return m, nil
		}
		message := m.mailbox.Messages[clamp(m.selectedMessage, 0, len(m.mailbox.Messages)-1)]
		if request.MessageID == "" {
			request.MessageID = message.ID
		}
		if request.ThreadID == "" {
			request.ThreadID = message.ThreadID
		}
	}
	m.nextActionID++
	actionID := m.nextActionID
	snapshot := triageSnapshot{
		Mailbox:               cloneMailbox(m.mailbox),
		SelectedMessage:       m.selectedMessage,
		SelectedThreadMessage: m.selectedThreadMessage,
	}
	if overrideSnapshot != nil {
		snapshot = *overrideSnapshot
	}
	m.pendingActions[actionID] = snapshot
	if recordUndo {
		m.undo = &undoState{
			Request:  request,
			Snapshot: snapshot,
			Expires:  m.now().Add(30 * time.Second),
		}
	}
	m.applyOptimisticAction(request)
	summary := triageProgressSummary(request)
	m.statusMessage = summary
	m.toastMessage = summary
	return m, m.runTriageAction(actionID, request)
}

func (m Model) startUndo() (tea.Model, tea.Cmd) {
	if m.undo == nil {
		m.statusMessage = "nothing to undo"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	if !m.now().Before(m.undo.Expires) {
		m.undo = nil
		m.statusMessage = "undo expired"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	inverse, ok := inverseTriageRequest(m.undo.Request)
	if !ok {
		m.undo = nil
		m.statusMessage = "undo unavailable"
		m.toastMessage = m.statusMessage
		return m, nil
	}
	inverse.Account = firstNonEmpty(inverse.Account, m.undo.Request.Account)
	inverse.MessageID = firstNonEmpty(inverse.MessageID, m.undo.Request.MessageID)
	inverse.ThreadID = firstNonEmpty(inverse.ThreadID, m.undo.Request.ThreadID)
	inverse.Undo = true

	current := triageSnapshot{
		Mailbox:               cloneMailbox(m.mailbox),
		SelectedMessage:       m.selectedMessage,
		SelectedThreadMessage: m.selectedThreadMessage,
	}
	m.mailbox = cloneMailbox(m.undo.Snapshot.Mailbox)
	m.selectedMessage = m.undo.Snapshot.SelectedMessage
	m.selectedThreadMessage = m.undo.Snapshot.SelectedThreadMessage
	m.undo = nil
	return m.startTriageActionWithUndo(inverse, false, &current)
}

func (m Model) triageRequest(kind TriageActionKind, labelID string) TriageActionRequest {
	message := m.mailbox.Messages[clamp(m.selectedMessage, 0, len(m.mailbox.Messages)-1)]
	request := TriageActionRequest{
		Kind:      kind,
		Account:   m.mailbox.Account,
		MessageID: message.ID,
		ThreadID:  message.ThreadID,
		LabelID:   labelID,
	}
	switch kind {
	case TriageStar:
		request.Add = !message.Starred
	case TriageUnread:
		request.Add = !message.Unread
	default:
		request.Add = true
	}
	return request
}

func (m *Model) applyOptimisticAction(request TriageActionRequest) {
	switch request.Kind {
	case TriageArchive, TriageTrash, TriageSpam:
		m.removeMessageFromCurrentView(request.MessageID)
	case TriageStar:
		m.updateMessageCopies(request.MessageID, func(message *Message) {
			message.Starred = request.Add
			message.LabelIDs = setLabel(message.LabelIDs, "STARRED", request.Add)
		})
	case TriageUnread:
		m.updateMessageCopies(request.MessageID, func(message *Message) {
			message.Unread = request.Add
			message.LabelIDs = setLabel(message.LabelIDs, "UNREAD", request.Add)
		})
	case TriageLabelAdd:
		message := m.messageByID(request.MessageID)
		m.ensureMailboxLabel(request.LabelID, request.LabelName, message.Unread)
		m.updateMessageCopies(request.MessageID, func(message *Message) {
			message.LabelIDs = setLabel(message.LabelIDs, request.LabelID, true)
		})
		m.addMessageToLabel(request.LabelID, message)
	case TriageLabelRemove:
		message := m.messageByID(request.MessageID)
		m.updateMessageCopies(request.MessageID, func(message *Message) {
			message.LabelIDs = setLabel(message.LabelIDs, request.LabelID, false)
		})
		m.removeMessageFromLabel(request.LabelID, request.MessageID, message.Unread)
		if len(m.mailbox.Labels) > 0 && labelKey(m.mailbox.Labels[clamp(m.selectedLabel, 0, len(m.mailbox.Labels)-1)]) == request.LabelID {
			m.mailbox.Messages = removeMessageByID(m.mailbox.Messages, request.MessageID)
			m.selectedMessage = clamp(m.selectedMessage, 0, len(m.mailbox.Messages)-1)
		}
	case TriageMute:
		m.updateMessageCopies(request.MessageID, func(message *Message) {
			message.Muted = true
			message.LabelIDs = setLabel(message.LabelIDs, "MUTED", true)
		})
	}
}

func (m *Model) reconcileTriageResult(request TriageActionRequest, result TriageActionResult) {
	if !request.CreateLabel || result.LabelID == "" || result.LabelID == request.LabelID {
		return
	}
	oldID := request.LabelID
	newID := result.LabelID
	newName := firstNonEmpty(result.LabelName, request.LabelName, newID)
	for i := range m.mailbox.Labels {
		if labelKey(m.mailbox.Labels[i]) == oldID {
			m.mailbox.Labels[i].ID = newID
			m.mailbox.Labels[i].Name = newName
		}
	}
	if m.mailbox.MessagesByLabel != nil {
		if messages, ok := m.mailbox.MessagesByLabel[oldID]; ok {
			delete(m.mailbox.MessagesByLabel, oldID)
			for i := range messages {
				messages[i].LabelIDs = replaceLabelID(messages[i].LabelIDs, oldID, newID)
			}
			m.mailbox.MessagesByLabel[newID] = messages
		}
	}
	m.updateMessageCopies(request.MessageID, func(message *Message) {
		message.LabelIDs = replaceLabelID(message.LabelIDs, oldID, newID)
	})
}

func (m *Model) removeMessageFromCurrentView(messageID string) {
	m.mailbox.Messages = removeMessageByID(m.mailbox.Messages, messageID)
	if len(m.mailbox.Labels) > 0 && len(m.mailbox.MessagesByLabel) > 0 {
		key := labelKey(m.mailbox.Labels[clamp(m.selectedLabel, 0, len(m.mailbox.Labels)-1)])
		m.mailbox.MessagesByLabel[key] = removeMessageByID(m.mailbox.MessagesByLabel[key], messageID)
	}
	m.selectedMessage = clamp(m.selectedMessage, 0, len(m.mailbox.Messages)-1)
	m.selectedThreadMessage = 0
}

func (m *Model) updateMessageCopies(messageID string, update func(*Message)) {
	for i := range m.mailbox.Messages {
		if m.mailbox.Messages[i].ID == messageID {
			update(&m.mailbox.Messages[i])
		}
	}
	for labelID, messages := range m.mailbox.MessagesByLabel {
		for i := range messages {
			if messages[i].ID == messageID {
				update(&messages[i])
			}
		}
		m.mailbox.MessagesByLabel[labelID] = messages
	}
}

func (m Model) messageByID(messageID string) Message {
	for _, message := range m.mailbox.Messages {
		if message.ID == messageID {
			return message
		}
	}
	for _, messages := range m.mailbox.MessagesByLabel {
		for _, message := range messages {
			if message.ID == messageID {
				return message
			}
		}
	}
	return Message{ID: messageID}
}

func (m *Model) ensureMailboxLabel(labelID string, labelName string, unread bool) {
	if labelID == "" {
		return
	}
	for i := range m.mailbox.Labels {
		if labelKey(m.mailbox.Labels[i]) == labelID {
			if unread {
				m.mailbox.Labels[i].Unread++
			}
			return
		}
	}
	count := 0
	if unread {
		count = 1
	}
	m.mailbox.Labels = append(m.mailbox.Labels, Label{ID: labelID, Name: firstNonEmpty(labelName, labelID), Unread: count})
}

func (m *Model) addMessageToLabel(labelID string, message Message) {
	if labelID == "" {
		return
	}
	messages := m.mailbox.MessagesByLabel[labelID]
	for _, existing := range messages {
		if existing.ID == message.ID {
			return
		}
	}
	message.LabelIDs = setLabel(message.LabelIDs, labelID, true)
	if m.mailbox.MessagesByLabel == nil {
		m.mailbox.MessagesByLabel = map[string][]Message{}
	}
	m.mailbox.MessagesByLabel[labelID] = append([]Message{message}, messages...)
}

func (m *Model) removeMessageFromLabel(labelID string, messageID string, unread bool) {
	if labelID == "" {
		return
	}
	if m.mailbox.MessagesByLabel != nil {
		m.mailbox.MessagesByLabel[labelID] = removeMessageByID(m.mailbox.MessagesByLabel[labelID], messageID)
	}
	if unread {
		for i := range m.mailbox.Labels {
			if labelKey(m.mailbox.Labels[i]) == labelID && m.mailbox.Labels[i].Unread > 0 {
				m.mailbox.Labels[i].Unread--
			}
		}
	}
}

func (m Model) refreshRequest(kind RefreshKind) RefreshRequest {
	request := RefreshRequest{
		Kind:    kind,
		Account: m.mailbox.Account,
	}
	if len(m.mailbox.Labels) == 0 {
		return request
	}
	label := m.mailbox.Labels[clamp(m.selectedLabel, 0, len(m.mailbox.Labels)-1)]
	request.LabelID = labelKey(label)
	request.LabelName = label.Name
	return request
}

func triageProgressSummary(request TriageActionRequest) string {
	if request.Undo {
		return "undoing..."
	}
	return triageDoneSummary(request)
}

func triageDoneSummary(request TriageActionRequest) string {
	if request.Undo {
		return "undone"
	}
	switch request.Kind {
	case TriageArchive:
		return "archived"
	case TriageTrash:
		return "trashed"
	case TriageUntrash:
		return "restored from trash"
	case TriageSpam:
		return "marked spam"
	case TriageUnspam:
		return "restored from spam"
	case TriageStar:
		if request.Add {
			return "starred"
		}
		return "unstarred"
	case TriageUnread:
		if request.Add {
			return "marked unread"
		}
		return "marked read"
	case TriageLabelAdd:
		return "label added"
	case TriageLabelRemove:
		return "label removed"
	case TriageMute:
		return "muted"
	default:
		return "action complete"
	}
}

func inverseTriageRequest(request TriageActionRequest) (TriageActionRequest, bool) {
	inverse := request
	inverse.Undo = true
	switch request.Kind {
	case TriageArchive:
		inverse.Kind = TriageLabelAdd
		inverse.LabelID = "INBOX"
		inverse.Add = true
	case TriageTrash:
		inverse.Kind = TriageUntrash
	case TriageSpam:
		inverse.Kind = TriageUnspam
	case TriageStar:
		inverse.Add = !request.Add
	case TriageUnread:
		inverse.Add = !request.Add
	case TriageLabelAdd:
		inverse.Kind = TriageLabelRemove
	case TriageLabelRemove:
		inverse.Kind = TriageLabelAdd
	case TriageMute:
		inverse.Kind = TriageLabelRemove
		inverse.LabelID = "MUTED"
	default:
		return TriageActionRequest{}, false
	}
	return inverse, true
}

func sameTriageRequest(a, b TriageActionRequest) bool {
	return a.Kind == b.Kind &&
		a.Account == b.Account &&
		a.MessageID == b.MessageID &&
		a.ThreadID == b.ThreadID &&
		a.LabelID == b.LabelID &&
		a.Add == b.Add &&
		a.Undo == b.Undo
}

func removeMessageByID(messages []Message, messageID string) []Message {
	out := make([]Message, 0, len(messages))
	for _, message := range messages {
		if message.ID != messageID {
			out = append(out, message)
		}
	}
	return out
}

func setLabel(labels []string, labelID string, present bool) []string {
	if labelID == "" {
		return labels
	}
	index := -1
	for i, existing := range labels {
		if existing == labelID {
			index = i
			break
		}
	}
	if present {
		if index >= 0 {
			return labels
		}
		return append(append([]string{}, labels...), labelID)
	}
	if index < 0 {
		return labels
	}
	out := append([]string{}, labels[:index]...)
	return append(out, labels[index+1:]...)
}

func replaceLabelID(labels []string, oldID string, newID string) []string {
	out := append([]string{}, labels...)
	for i, labelID := range out {
		if labelID == oldID {
			out[i] = newID
		}
	}
	return out
}

func (m Model) labelByNameOrID(value string) Label {
	for _, label := range m.mailbox.Labels {
		if strings.EqualFold(label.Name, value) || strings.EqualFold(labelKey(label), value) {
			return label
		}
	}
	return Label{}
}

func (m Model) labelByID(labelID string) Label {
	for _, label := range m.mailbox.Labels {
		if labelKey(label) == labelID {
			return label
		}
	}
	return Label{}
}

func removableLabels(labelIDs []string) []string {
	system := map[string]bool{
		"INBOX":   true,
		"UNREAD":  true,
		"STARRED": true,
		"SPAM":    true,
		"TRASH":   true,
		"MUTED":   true,
	}
	out := make([]string, 0, len(labelIDs))
	for _, labelID := range labelIDs {
		if labelID == "" || system[labelID] {
			continue
		}
		out = append(out, labelID)
	}
	return out
}

func userLabelID(name string) string {
	var b strings.Builder
	b.WriteString("Label_")
	for _, r := range strings.TrimSpace(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == len("Label_") {
		return ""
	}
	return b.String()
}

func cloneMailbox(mailbox Mailbox) Mailbox {
	clone := mailbox
	clone.Labels = append([]Label{}, mailbox.Labels...)
	clone.Messages = cloneMessages(mailbox.Messages)
	if mailbox.MessagesByLabel != nil {
		clone.MessagesByLabel = make(map[string][]Message, len(mailbox.MessagesByLabel))
		for labelID, messages := range mailbox.MessagesByLabel {
			clone.MessagesByLabel[labelID] = cloneMessages(messages)
		}
	}
	return clone
}

func cloneMessages(messages []Message) []Message {
	out := make([]Message, len(messages))
	for i, message := range messages {
		out[i] = message
		out[i].Body = append([]string{}, message.Body...)
		out[i].LabelIDs = append([]string{}, message.LabelIDs...)
		out[i].Attachments = append([]Attachment{}, message.Attachments...)
		out[i].ThreadMessages = append([]ThreadMessage{}, message.ThreadMessages...)
	}
	return out
}

func refreshProgressSummary(request RefreshRequest) string {
	switch request.Kind {
	case RefreshRelistLabel:
		return "refreshing " + firstNonEmpty(request.LabelName, request.LabelID, "label") + "..."
	case RefreshAll:
		return "syncing all accounts..."
	default:
		return "syncing..."
	}
}

func refreshDoneSummary(request RefreshRequest) string {
	switch request.Kind {
	case RefreshRelistLabel:
		return "refreshed " + firstNonEmpty(request.LabelName, request.LabelID, "label")
	case RefreshAll:
		return "sync-all complete"
	default:
		return "sync complete"
	}
}

func (m *Model) consumeSyncActivity() bool {
	active := m.syncHadInput
	m.syncHadInput = false
	return active
}

func (m *Model) stopSync() {
	if m.syncCancel != nil {
		m.syncCancel()
	}
}

func (m *Model) loadSelectedThreadIfNeeded() tea.Cmd {
	if m.threadLoader == nil || len(m.mailbox.Messages) == 0 {
		return nil
	}
	message := m.mailbox.Messages[m.selectedMessage]
	if messageHasReadableBody(message) || message.ThreadID == "" || m.loadingThreadID == message.ThreadID {
		return nil
	}
	m.loadingThreadID = message.ThreadID
	m.statusMessage = "loading thread..."
	return m.runLoadThread(message)
}

func (m *Model) applyThreadLoadResult(result ThreadLoadResult) {
	update := func(messages []Message) {
		for i := range messages {
			if !messageMatchesLoadResult(messages[i], result) {
				continue
			}
			messages[i].Body = append([]string{}, result.Body...)
			messages[i].Attachments = append([]Attachment{}, result.Attachments...)
			messages[i].ThreadMessages = append([]ThreadMessage{}, result.ThreadMessages...)
			if result.CacheState != "" {
				messages[i].CacheState = result.CacheState
			}
		}
	}
	update(m.mailbox.Messages)
	for labelID, messages := range m.mailbox.MessagesByLabel {
		update(messages)
		m.mailbox.MessagesByLabel[labelID] = messages
	}
	if index := threadMessageIndex(result.ThreadMessages, result.MessageID); index >= 0 {
		m.selectedThreadMessage = index
	}
}

func messageMatchesLoadResult(message Message, result ThreadLoadResult) bool {
	if result.MessageID != "" && message.ID == result.MessageID {
		return true
	}
	return result.ThreadID != "" && message.ThreadID == result.ThreadID
}

func messageHasReadableBody(message Message) bool {
	if len(message.Body) > 0 {
		return true
	}
	for _, threadMessage := range message.ThreadMessages {
		if len(threadMessage.Body) > 0 {
			return true
		}
	}
	return false
}

func threadMessageIndex(messages []ThreadMessage, id string) int {
	if id == "" {
		return -1
	}
	for i, message := range messages {
		if message.ID == id {
			return i
		}
	}
	return -1
}

func (m *Model) moveSelection(delta int) {
	switch m.focus {
	case PaneLabels:
		m.selectedLabel = clamp(m.selectedLabel+delta, 0, len(m.mailbox.Labels)-1)
		m.updateSelectedLabelMessages()
	default:
		m.selectedMessage = clamp(m.selectedMessage+delta, 0, len(m.mailbox.Messages)-1)
		m.selectedThreadMessage = 0
	}
}

func (m *Model) jumpSelection(bottom bool) {
	target := 0
	if bottom {
		switch m.focus {
		case PaneLabels:
			target = len(m.mailbox.Labels) - 1
		default:
			target = len(m.mailbox.Messages) - 1
		}
	}

	switch m.focus {
	case PaneLabels:
		m.selectedLabel = clamp(target, 0, len(m.mailbox.Labels)-1)
		m.updateSelectedLabelMessages()
	default:
		m.selectedMessage = clamp(target, 0, len(m.mailbox.Messages)-1)
		m.selectedThreadMessage = 0
	}
}

func (m *Model) moveThreadMessage(delta int) {
	if len(m.mailbox.Messages) == 0 {
		return
	}
	message := m.mailbox.Messages[m.selectedMessage]
	if len(message.ThreadMessages) == 0 {
		m.statusMessage = "single-message thread"
		return
	}
	m.selectedThreadMessage = clamp(m.selectedThreadMessage+delta, 0, len(message.ThreadMessages)-1)
}

func (m *Model) toggleRenderMode() {
	switch m.renderMode {
	case "plaintext":
		m.renderMode = "html2text"
	case "html2text":
		m.renderMode = "glamour"
	default:
		m.renderMode = "plaintext"
	}
	m.statusMessage = "render mode: " + m.renderMode
}

func (m *Model) updateSelectedLabelMessages() {
	if len(m.mailbox.MessagesByLabel) == 0 || len(m.mailbox.Labels) == 0 {
		return
	}
	key := labelKey(m.mailbox.Labels[m.selectedLabel])
	m.mailbox.Messages = m.mailbox.MessagesByLabel[key]
	m.selectedMessage = clamp(m.selectedMessage, 0, len(m.mailbox.Messages)-1)
	m.selectedThreadMessage = 0
}

func (m *Model) nextPane() {
	if m.width > 0 && m.width < 80 {
		if m.focus == PaneReader {
			m.focus = PaneList
			m.readerOpen = false
			return
		}
		m.focus = PaneReader
		m.readerOpen = true
		return
	}

	if m.width >= 120 {
		switch m.focus {
		case PaneLabels:
			m.focus = PaneList
		case PaneList:
			m.focus = PaneReader
			m.readerOpen = true
		default:
			m.focus = PaneLabels
		}
		return
	}

	if m.focus == PaneReader {
		m.focus = PaneList
		return
	}
	m.focus = PaneReader
	m.readerOpen = true
}

func clamp(value, min, max int) int {
	if max < min {
		return min
	}
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func normalizeAccountStates(accounts []AccountState) []AccountState {
	out := make([]AccountState, 0, len(accounts))
	for _, account := range accounts {
		if account.Mailbox.Account == "" {
			account.Mailbox = RealAccountMailbox(account.Account, nil)
		}
		if account.Account == "" {
			account.Account = account.Mailbox.Account
		}
		if account.Unread == 0 {
			account.Unread = unreadCount(account.Mailbox)
		}
		out = append(out, account)
	}
	return out
}

func unreadCount(mailbox Mailbox) int {
	total := 0
	for _, label := range mailbox.Labels {
		if strings.EqualFold(label.ID, "UNREAD") || strings.EqualFold(label.Name, "Unread") {
			return label.Unread
		}
		if strings.EqualFold(label.ID, "INBOX") || strings.EqualFold(label.Name, "Inbox") {
			total = label.Unread
		}
	}
	return total
}
