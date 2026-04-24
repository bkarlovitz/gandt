package cache

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"
)

type OptimisticActionKind string

const (
	OptimisticArchive      OptimisticActionKind = "archive"
	OptimisticTrash        OptimisticActionKind = "trash"
	OptimisticSpam         OptimisticActionKind = "spam"
	OptimisticToggleStar   OptimisticActionKind = "toggle-star"
	OptimisticToggleUnread OptimisticActionKind = "toggle-unread"
	OptimisticLabelAdd     OptimisticActionKind = "label-add"
	OptimisticLabelRemove  OptimisticActionKind = "label-remove"
	OptimisticMute         OptimisticActionKind = "mute"
)

type OptimisticAction struct {
	Kind      OptimisticActionKind
	AccountID string
	MessageID string
	LabelID   string
	Add       bool
}

type ActionSnapshot struct {
	AccountID string
	MessageID string
	LabelIDs  []string
}

type OptimisticActionRepository struct {
	labels MessageLabelRepository
}

func NewOptimisticActionRepository(db *sqlx.DB) OptimisticActionRepository {
	return OptimisticActionRepository{labels: NewMessageLabelRepository(db)}
}

func (r OptimisticActionRepository) Apply(ctx context.Context, action OptimisticAction) (ActionSnapshot, error) {
	if action.AccountID == "" {
		return ActionSnapshot{}, errors.New("action account id is required")
	}
	if action.MessageID == "" {
		return ActionSnapshot{}, errors.New("action message id is required")
	}
	snapshot, err := r.snapshot(ctx, action.AccountID, action.MessageID)
	if err != nil {
		return ActionSnapshot{}, err
	}
	switch action.Kind {
	case OptimisticArchive:
		err = r.removeLabels(ctx, action, "INBOX")
	case OptimisticTrash:
		err = r.replaceSystemLocation(ctx, action, "TRASH")
	case OptimisticSpam:
		err = r.replaceSystemLocation(ctx, action, "SPAM")
	case OptimisticToggleStar:
		err = r.toggleLabel(ctx, action, "STARRED", action.Add)
	case OptimisticToggleUnread:
		err = r.toggleLabel(ctx, action, "UNREAD", action.Add)
	case OptimisticLabelAdd:
		err = r.addLabels(ctx, action, action.LabelID)
	case OptimisticLabelRemove:
		err = r.removeLabels(ctx, action, action.LabelID)
	case OptimisticMute:
		err = r.addLabels(ctx, action, "MUTED")
	default:
		err = errors.New("unsupported optimistic action")
	}
	if err != nil {
		return ActionSnapshot{}, err
	}
	return snapshot, nil
}

func (r OptimisticActionRepository) Revert(ctx context.Context, snapshot ActionSnapshot) error {
	return r.labels.ReplaceForMessage(ctx, snapshot.AccountID, snapshot.MessageID, snapshot.LabelIDs)
}

func (r OptimisticActionRepository) snapshot(ctx context.Context, accountID string, messageID string) (ActionSnapshot, error) {
	labels, err := r.labels.ListForMessage(ctx, accountID, messageID)
	if err != nil {
		return ActionSnapshot{}, err
	}
	out := ActionSnapshot{AccountID: accountID, MessageID: messageID, LabelIDs: make([]string, 0, len(labels))}
	for _, label := range labels {
		out.LabelIDs = append(out.LabelIDs, label.LabelID)
	}
	return out, nil
}

func (r OptimisticActionRepository) replaceSystemLocation(ctx context.Context, action OptimisticAction, labelID string) error {
	if err := r.removeLabels(ctx, action, "INBOX"); err != nil {
		return err
	}
	return r.addLabels(ctx, action, labelID)
}

func (r OptimisticActionRepository) toggleLabel(ctx context.Context, action OptimisticAction, labelID string, add bool) error {
	if add {
		return r.addLabels(ctx, action, labelID)
	}
	return r.removeLabels(ctx, action, labelID)
}

func (r OptimisticActionRepository) addLabels(ctx context.Context, action OptimisticAction, labelIDs ...string) error {
	for _, labelID := range labelIDs {
		if labelID == "" {
			continue
		}
		if err := r.labels.Upsert(ctx, MessageLabel{AccountID: action.AccountID, MessageID: action.MessageID, LabelID: labelID}); err != nil {
			return err
		}
	}
	return nil
}

func (r OptimisticActionRepository) removeLabels(ctx context.Context, action OptimisticAction, labelIDs ...string) error {
	for _, labelID := range labelIDs {
		if labelID == "" {
			continue
		}
		err := r.labels.Delete(ctx, action.AccountID, action.MessageID, labelID)
		if err != nil && !errors.Is(err, ErrMessageLabelAbsent) {
			return err
		}
	}
	return nil
}
