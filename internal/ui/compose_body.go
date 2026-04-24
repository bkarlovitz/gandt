package ui

import (
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

type ComposeBodyExternalEditor func(string) (string, error)

type ComposeBodyEditor struct {
	textarea        textarea.Model
	external        ComposeBodyExternalEditor
	width           int
	height          int
	validationError string
	draftSaved      bool
	cancelConfirm   bool
}

type composeBodyExternalEditedMsg struct {
	Body string
	Err  error
}

func NewComposeBodyEditor(initial string, width int, height int, external ComposeBodyExternalEditor) ComposeBodyEditor {
	area := textarea.New()
	area.ShowLineNumbers = false
	area.Prompt = ""
	area.Placeholder = ""
	area.SetValue(initial)
	area.Focus()

	editor := ComposeBodyEditor{
		textarea: area,
		external: external,
		width:    composeBodyWidth(width),
		height:   composeBodyHeight(height),
	}
	editor.resizeTextarea()
	return editor
}

func (e ComposeBodyEditor) Update(msg tea.Msg) (ComposeBodyEditor, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		e.width = composeBodyWidth(msg.Width)
		e.height = composeBodyHeight(msg.Height)
		e.resizeTextarea()
		return e, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlE:
			if e.external == nil {
				e.validationError = "external editor is unavailable"
				return e, nil
			}
			body := e.Body()
			return e, func() tea.Msg {
				edited, err := e.external(body)
				return composeBodyExternalEditedMsg{Body: edited, Err: err}
			}
		case tea.KeyCtrlC:
			e.cancelConfirm = true
			return e, nil
		}
	case composeBodyExternalEditedMsg:
		if msg.Err != nil {
			e.validationError = msg.Err.Error()
			return e, nil
		}
		e.textarea.SetValue(msg.Body)
		e.validationError = ""
		e.draftSaved = false
		return e, nil
	}

	var cmd tea.Cmd
	e.textarea, cmd = e.textarea.Update(msg)
	if _, ok := msg.(tea.KeyMsg); ok {
		e.draftSaved = false
	}
	return e, cmd
}

func (e ComposeBodyEditor) Body() string {
	return e.textarea.Value()
}

func (e ComposeBodyEditor) Size() (int, int) {
	return e.width, e.height
}

func (e ComposeBodyEditor) ValidationError() string {
	return e.validationError
}

func (e ComposeBodyEditor) DraftSaved() bool {
	return e.draftSaved
}

func (e ComposeBodyEditor) CancelConfirming() bool {
	return e.cancelConfirm
}

func (e ComposeBodyEditor) SaveDraft() (ComposeBodyEditor, string) {
	e.draftSaved = true
	e.validationError = ""
	return e, e.Body()
}

func (e ComposeBodyEditor) WithValidationError(err error) ComposeBodyEditor {
	if err == nil {
		e.validationError = ""
		return e
	}
	e.validationError = err.Error()
	return e
}

func (e *ComposeBodyEditor) resizeTextarea() {
	e.textarea.SetWidth(e.width)
	e.textarea.SetHeight(e.height)
}

func composeBodyWidth(width int) int {
	if width < 20 {
		return 20
	}
	return width
}

func composeBodyHeight(height int) int {
	if height < 3 {
		return 3
	}
	return height
}
