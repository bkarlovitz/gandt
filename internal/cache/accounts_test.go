package cache

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
)

func TestAccountRepositoryCreateListGetUpdateDelete(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	repo := NewAccountRepository(db)

	account, err := repo.Create(ctx, CreateAccountParams{
		Email:       "me@example.com",
		DisplayName: "Me",
		HistoryID:   "100",
		Color:       "#4287f5",
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if account.ID == "" || account.ID == "me@example.com" {
		t.Fatalf("account id is not opaque: %q", account.ID)
	}
	if account.Email != "me@example.com" || account.DisplayName != "Me" {
		t.Fatalf("unexpected account: %#v", account)
	}

	listed, err := repo.List(ctx)
	if err != nil {
		t.Fatalf("list accounts: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != account.ID {
		t.Fatalf("listed accounts = %#v, want created account", listed)
	}

	got, err := repo.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get account: %v", err)
	}
	if got.Email != account.Email || got.HistoryID != "100" || got.Color != "#4287f5" {
		t.Fatalf("got account = %#v, want created values", got)
	}

	syncedAt := time.Date(2026, 4, 24, 12, 30, 0, 0, time.UTC)
	if err := repo.UpdateSyncMetadata(ctx, account.ID, "200", syncedAt); err != nil {
		t.Fatalf("update sync metadata: %v", err)
	}
	got, err = repo.Get(ctx, account.ID)
	if err != nil {
		t.Fatalf("get updated account: %v", err)
	}
	if got.HistoryID != "200" || got.LastSyncAt == nil || !got.LastSyncAt.Equal(syncedAt) {
		t.Fatalf("sync metadata = history %q at %v, want 200 at %v", got.HistoryID, got.LastSyncAt, syncedAt)
	}

	if err := repo.Delete(ctx, account.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}
	if _, err := repo.Get(ctx, account.ID); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("get deleted account error = %v, want ErrAccountNotFound", err)
	}
}

func TestAccountRepositoryRejectsDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	repo := NewAccountRepository(db)

	if _, err := repo.Create(ctx, CreateAccountParams{Email: "me@example.com"}); err != nil {
		t.Fatalf("create first account: %v", err)
	}
	if _, err := repo.Create(ctx, CreateAccountParams{Email: "me@example.com"}); !errors.Is(err, ErrDuplicateEmail) {
		t.Fatalf("duplicate account error = %v, want ErrDuplicateEmail", err)
	}
}

func TestAccountRepositoryDeleteCascadesOwnedRows(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	repo := NewAccountRepository(db)

	account, err := repo.Create(ctx, CreateAccountParams{Email: "me@example.com"})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO labels (account_id, id, name, type) VALUES (?, 'INBOX', 'Inbox', 'system')
`, account.ID); err != nil {
		t.Fatalf("insert label: %v", err)
	}

	if err := repo.Delete(ctx, account.ID); err != nil {
		t.Fatalf("delete account: %v", err)
	}

	var labels int
	if err := db.GetContext(ctx, &labels, "SELECT COUNT(*) FROM labels WHERE account_id = ?", account.ID); err != nil {
		t.Fatalf("count labels: %v", err)
	}
	if labels != 0 {
		t.Fatalf("labels after account delete = %d, want 0", labels)
	}
}

func TestAccountRepositoryMissingRows(t *testing.T) {
	ctx := context.Background()
	db := migratedTestDB(t)
	repo := NewAccountRepository(db)

	if _, err := repo.Get(ctx, "missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("get missing error = %v, want ErrAccountNotFound", err)
	}
	if err := repo.UpdateSyncMetadata(ctx, "missing", "1", time.Now()); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("update missing error = %v, want ErrAccountNotFound", err)
	}
	if err := repo.Delete(ctx, "missing"); !errors.Is(err, ErrAccountNotFound) {
		t.Fatalf("delete missing error = %v, want ErrAccountNotFound", err)
	}
}

func migratedTestDB(t *testing.T) *sqlx.DB {
	t.Helper()

	ctx := context.Background()
	db, err := OpenPath(ctx, t.TempDir()+"/cache.db")
	if err != nil {
		t.Fatalf("open cache: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Fatalf("close cache: %v", err)
		}
	})
	if err := Migrate(ctx, db); err != nil {
		t.Fatalf("migrate cache: %v", err)
	}
	return db
}
