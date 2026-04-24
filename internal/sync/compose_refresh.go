package sync

type ComposeOperation string

const (
	ComposeOperationSend        ComposeOperation = "send"
	ComposeOperationDraftUpdate ComposeOperation = "draft_update"
)

type PostComposeRefreshPlan struct {
	AccountID string
	ThreadID  string
	LabelIDs  []string
}

func PlanPostComposeRefresh(operation ComposeOperation, accountID string, threadID string) PostComposeRefreshPlan {
	plan := PostComposeRefreshPlan{AccountID: accountID, ThreadID: threadID}
	switch operation {
	case ComposeOperationDraftUpdate:
		plan.LabelIDs = []string{"DRAFT"}
	case ComposeOperationSend:
		plan.LabelIDs = []string{"SENT"}
	}
	return plan
}
