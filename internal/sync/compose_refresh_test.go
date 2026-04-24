package sync

import "testing"

func TestPlanPostComposeRefresh(t *testing.T) {
	sent := PlanPostComposeRefresh(ComposeOperationSend, "acct-1", "thread-1")
	if sent.AccountID != "acct-1" || sent.ThreadID != "thread-1" || len(sent.LabelIDs) != 1 || sent.LabelIDs[0] != "SENT" {
		t.Fatalf("sent plan = %#v", sent)
	}
	draft := PlanPostComposeRefresh(ComposeOperationDraftUpdate, "acct-1", "thread-1")
	if len(draft.LabelIDs) != 1 || draft.LabelIDs[0] != "DRAFT" {
		t.Fatalf("draft plan = %#v", draft)
	}
}
