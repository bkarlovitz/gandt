package ui

import (
	"errors"
	"strings"

	"github.com/bkarlovitz/gandt/internal/compose"
	"github.com/charmbracelet/huh"
)

type ComposeHeaderFormStatus string

const (
	ComposeHeaderEditing   ComposeHeaderFormStatus = "editing"
	ComposeHeaderCanceled  ComposeHeaderFormStatus = "canceled"
	ComposeHeaderSubmitted ComposeHeaderFormStatus = "submitted"
)

type ComposeHeaderFormInput struct {
	Kind            compose.ComposeKind
	ActiveAccountID string
	AccountEmail    string
	SendAsAliases   []compose.Address
	Original        compose.OriginalMessage
	Width           int
}

type ComposeHeaderForm struct {
	Form      *huh.Form
	Kind      compose.ComposeKind
	Status    ComposeHeaderFormStatus
	Width     int
	Headers   compose.Headers
	ToInput   string
	CcInput   string
	BccInput  string
	Subject   string
	From      compose.Address
	Aliases   []compose.Address
	lastError error
}

func NewComposeHeaderForm(input ComposeHeaderFormInput) *ComposeHeaderForm {
	aliases := input.SendAsAliases
	if len(aliases) == 0 {
		aliases = []compose.Address{compose.NewAddress(input.AccountEmail)}
	}
	from := aliases[0]
	kind := input.Kind
	if kind == "" {
		kind = compose.ComposeKindNew
	}

	form := ComposeHeaderForm{
		Kind:    kind,
		Status:  ComposeHeaderEditing,
		Width:   composeFormWidth(input.Width),
		From:    from,
		Aliases: cloneComposeAddresses(aliases),
		Headers: compose.Headers{
			ActiveAccountID: strings.TrimSpace(input.ActiveAccountID),
			AccountEmail:    strings.TrimSpace(input.AccountEmail),
			SendAs:          from,
		},
	}
	form.applyPrefill(input)
	form.Form = form.buildHuhForm()
	return &form
}

func (f *ComposeHeaderForm) Submit() (compose.Headers, error) {
	to, err := compose.ParseAddressList(f.ToInput)
	if err != nil {
		f.lastError = err
		return compose.Headers{}, err
	}
	cc, err := compose.ParseAddressList(f.CcInput)
	if err != nil {
		f.lastError = err
		return compose.Headers{}, err
	}
	bcc, err := compose.ParseAddressList(f.BccInput)
	if err != nil {
		f.lastError = err
		return compose.Headers{}, err
	}

	headers := compose.Headers{
		ActiveAccountID: f.Headers.ActiveAccountID,
		AccountEmail:    f.Headers.AccountEmail,
		SendAs:          f.From,
		To:              to,
		Cc:              cc,
		Bcc:             bcc,
		Subject:         strings.TrimSpace(f.Subject),
	}
	if err := headers.ValidateDraft(); err != nil {
		f.lastError = err
		return compose.Headers{}, err
	}
	f.Headers = headers
	f.Status = ComposeHeaderSubmitted
	f.lastError = nil
	return headers, nil
}

func (f *ComposeHeaderForm) Cancel() {
	f.Status = ComposeHeaderCanceled
}

func (f ComposeHeaderForm) Error() error {
	return f.lastError
}

func (f *ComposeHeaderForm) buildHuhForm() *huh.Form {
	options := make([]huh.Option[compose.Address], 0, len(f.Aliases))
	for _, alias := range f.Aliases {
		options = append(options, huh.NewOption(alias.String(), alias))
	}
	return huh.NewForm(huh.NewGroup(
		huh.NewSelect[compose.Address]().
			Title("From").
			Options(options...).
			Value(&f.From),
		huh.NewInput().
			Title("To").
			Value(&f.ToInput).
			Validate(validateOptionalAddressList),
		huh.NewInput().
			Title("Cc").
			Value(&f.CcInput).
			Validate(validateOptionalAddressList),
		huh.NewInput().
			Title("Bcc").
			Value(&f.BccInput).
			Validate(validateOptionalAddressList),
		huh.NewInput().
			Title("Subject").
			Value(&f.Subject),
	)).WithWidth(f.Width)
}

func (f *ComposeHeaderForm) applyPrefill(input ComposeHeaderFormInput) {
	original := input.Original
	switch f.Kind {
	case compose.ComposeKindReply:
		ctx := compose.NewReplyContext(original, compose.NewAddress(input.AccountEmail), false)
		f.ToInput = formatComposeAddressList(ctx.Recipients())
		f.Subject = ctx.Subject()
	case compose.ComposeKindReplyAll:
		ctx := compose.NewReplyContext(original, compose.NewAddress(input.AccountEmail), true)
		f.ToInput = formatComposeAddressList(ctx.Recipients())
		f.Subject = ctx.Subject()
	case compose.ComposeKindForward:
		ctx := compose.NewForwardContext(original)
		f.Subject = ctx.Subject()
	default:
		f.Kind = compose.ComposeKindNew
	}
}

func validateOptionalAddressList(value string) error {
	_, err := compose.ParseAddressList(value)
	return err
}

func composeFormWidth(width int) int {
	if width <= 0 {
		return 72
	}
	if width < 40 {
		return 40
	}
	return width
}

func formatComposeAddressList(addresses []compose.Address) string {
	formatted := make([]string, 0, len(addresses))
	for _, address := range addresses {
		if strings.TrimSpace(address.Email) == "" {
			continue
		}
		formatted = append(formatted, address.String())
	}
	return strings.Join(formatted, ", ")
}

func cloneComposeAddresses(addresses []compose.Address) []compose.Address {
	out := make([]compose.Address, len(addresses))
	copy(out, addresses)
	return out
}

func isComposeHeaderValidationError(err error) bool {
	return err != nil && !errors.Is(err, compose.ErrAccountRequired) && !errors.Is(err, compose.ErrSendAsRequired)
}
