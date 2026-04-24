package auth

import (
	"context"
	"testing"
	"time"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/gmail"
	"github.com/jmoiron/sqlx"
	"golang.org/x/oauth2"
)

func TestAccountBootstrapperPersistsProfileLabelsAndToken(t *testing.T) {
	ctx := context.Background()
	db := cacheTestDB(t)
	store := NewSecretStore(newFakeKeyring())
	bootstrapper := NewAccountBootstrapper(db, store)
	token := &oauth2.Token{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 4, 24, 14, 0, 0, 0, time.UTC),
	}

	account, err := bootstrapper.Bootstrap(ctx, fakeGmailBootstrapClient{
		profile: gmail.Profile{EmailAddress: "Me@Example.com", HistoryID: "98765"},
		labels: []gmail.Label{
			{ID: "INBOX", Name: "Inbox", Type: "system", Unread: 2, Total: 5},
			{ID: "Label_1", Name: "Receipts", Type: "user", Unread: 1, Total: 3, ColorBG: "#111111", ColorFG: "#eeeeee"},
		},
	}, token, "")
	if err != nil {
		t.Fatalf("bootstrap account: %v", err)
	}

	accounts := cache.NewAccountRepository(db)
	gotAccount, err := accounts.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if gotAccount.Email != "Me@Example.com" || gotAccount.HistoryID != "98765" {
		t.Fatalf("account = %#v, want profile values", gotAccount)
	}
	if gotAccount.Color == "" || gotAccount.Color != DeterministicAccountColor("me@example.com") {
		t.Fatalf("account color = %q, want deterministic color", gotAccount.Color)
	}

	labels, err := cache.NewLabelRepository(db).List(ctx, account.ID)
	if err != nil {
		t.Fatalf("list labels: %v", err)
	}
	if len(labels) != 2 {
		t.Fatalf("labels = %#v, want 2", labels)
	}
	custom := findLabel(labels, "Label_1")
	if custom.ID != "Label_1" || custom.ColorBG != "#111111" {
		t.Fatalf("custom label = %#v, want persisted Gmail label", custom)
	}

	storedToken, err := store.OAuthToken(account.ID)
	if err != nil {
		t.Fatalf("load token: %v", err)
	}
	if storedToken.AccessToken != token.AccessToken || storedToken.RefreshToken != token.RefreshToken {
		t.Fatalf("stored token = %#v, want %#v", storedToken, token)
	}
}

func TestAccountBootstrapperUsesConfiguredColor(t *testing.T) {
	ctx := context.Background()
	db := cacheTestDB(t)
	bootstrapper := NewAccountBootstrapper(db, NewSecretStore(newFakeKeyring()))

	account, err := bootstrapper.Bootstrap(ctx, fakeGmailBootstrapClient{
		profile: gmail.Profile{EmailAddress: "me@example.com", HistoryID: "1"},
	}, &oauth2.Token{AccessToken: "access-token"}, "#123456")
	if err != nil {
		t.Fatalf("bootstrap account: %v", err)
	}
	if account.Color != "#123456" {
		t.Fatalf("account color = %q, want configured color", account.Color)
	}
}

func TestDeterministicAccountColorIsStable(t *testing.T) {
	first := DeterministicAccountColor("Me@Example.com")
	second := DeterministicAccountColor(" me@example.com ")
	if first != second {
		t.Fatalf("colors differ: %q vs %q", first, second)
	}
}

type fakeGmailBootstrapClient struct {
	profile gmail.Profile
	labels  []gmail.Label
}

func (f fakeGmailBootstrapClient) Profile(context.Context) (gmail.Profile, error) {
	return f.profile, nil
}

func (f fakeGmailBootstrapClient) Labels(context.Context) ([]gmail.Label, error) {
	return f.labels, nil
}

func findLabel(labels []cache.Label, id string) cache.Label {
	for _, label := range labels {
		if label.ID == id {
			return label
		}
	}
	return cache.Label{}
}

func cacheTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	ctx := context.Background()
	db, err := cache.OpenPath(ctx, t.TempDir()+"/cache.db")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close cache: %v", err)
		}
	})
	if err := cache.Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	return db
}
