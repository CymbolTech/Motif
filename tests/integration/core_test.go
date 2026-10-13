//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/motifengine/motifengine/internal/auth"
	"github.com/motifengine/motifengine/internal/events"
	"github.com/motifengine/motifengine/internal/points"
	"github.com/motifengine/motifengine/internal/processing"
	"github.com/motifengine/motifengine/internal/rewards"
	"github.com/nats-io/nats.go"
)

type testDatabase struct {
	pool *pgxpool.Pool
	url  string
}

type controlledRewardProvider struct {
	mu           sync.Mutex
	deliveryErr  error
	status       rewards.DeliveryStatus
	deliverCalls int
	statusCalls  int
}

func (p *controlledRewardProvider) Deliver(_ context.Context, request rewards.DeliveryRequest) (rewards.DeliveryResult, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deliverCalls++
	if p.deliveryErr != nil {
		return rewards.DeliveryResult{}, p.deliveryErr
	}
	return rewards.DeliveryResult{ProviderReference: "integration:" + request.RedemptionID}, nil
}

func (p *controlledRewardProvider) Status(_ context.Context, _ rewards.DeliveryRequest) (rewards.DeliveryStatus, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.statusCalls++
	return p.status, nil
}

func disposableDatabase(t *testing.T) testDatabase {
	t.Helper()
	controlURL := os.Getenv("MOTIF_INTEGRATION_DATABASE_URL")
	if controlURL == "" {
		t.Fatal("MOTIF_INTEGRATION_DATABASE_URL must point to a local PostgreSQL control database")
	}
	parsed, err := url.Parse(controlURL)
	if err != nil || (parsed.Scheme != "postgres" && parsed.Scheme != "postgresql") || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatal("integration tests require a local PostgreSQL URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	control, err := pgx.Connect(ctx, controlURL)
	if err != nil {
		t.Fatalf("connect to control database: %v", err)
	}
	t.Cleanup(func() { _ = control.Close(context.Background()) })
	name := fmt.Sprintf("motif_it_%d", time.Now().UnixNano())
	if _, err := control.Exec(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create disposable database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := control.Exec(context.Background(), "DROP DATABASE "+name+" WITH (FORCE)"); err != nil {
			t.Errorf("drop disposable database %s: %v", name, err)
		}
	})
	parsed.Path = "/" + name
	testURL := parsed.String()
	runMigration(t, testURL, "up")
	pool, err := pgxpool.New(ctx, testURL)
	if err != nil {
		t.Fatalf("open disposable database pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return testDatabase{pool: pool, url: testURL}
}

func runMigration(t *testing.T, databaseURL, direction string) {
	t.Helper()
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "./apps/migrate", direction)
	command.Dir = root
	command.Env = append(os.Environ(), "DATABASE_URL="+databaseURL)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("migrate %s: %v\n%s", direction, err, strings.TrimSpace(string(output)))
	}
}

func TestMigrationsRoundTrip(t *testing.T) {
	db := disposableDatabase(t)
	for version := 23; version > 0; version-- {
		var current int
		if err := db.pool.QueryRow(context.Background(), `SELECT max(version) FROM schema_migrations`).Scan(&current); err != nil {
			t.Fatalf("migration version before down: %v", err)
		}
		if current != version {
			t.Fatalf("migration version = %d; want %d", current, version)
		}
		runMigration(t, db.url, "down")
	}
	var migrationTable *string
	if err := db.pool.QueryRow(context.Background(), `SELECT to_regclass('public.schema_migrations')::text`).Scan(&migrationTable); err != nil {
		t.Fatal(err)
	}
	if migrationTable != nil {
		t.Fatalf("migration table remained after complete rollback: %s", *migrationTable)
	}
	runMigration(t, db.url, "up")
	var current int
	if err := db.pool.QueryRow(context.Background(), `SELECT max(version) FROM schema_migrations`).Scan(&current); err != nil {
		t.Fatal(err)
	}
	if current != 23 {
		t.Fatalf("migration version after reapply = %d; want 23", current)
	}
}

func seedCurrency(t *testing.T, db *pgxpool.Pool) points.Scope {
	t.Helper()
	ctx := context.Background()
	scope := points.Scope{TenantID: "integration-tenant", EnvironmentID: "sandbox"}
	for _, query := range []string{
		`INSERT INTO tenants (id, name) VALUES ('integration-tenant', 'Integration tenant')`,
		`INSERT INTO tenant_plans (tenant_id, plan_id) VALUES ('integration-tenant', 'starter')`,
		`INSERT INTO environments (tenant_id, id, name) VALUES ('integration-tenant', 'sandbox', 'Sandbox')`,
		`INSERT INTO currencies (id, tenant_id, environment_id, code, name) VALUES ('integration-currency', 'integration-tenant', 'sandbox', 'points', 'Points')`,
		`INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ('integration-currency', 1, '{}'::jsonb)`,
	} {
		if _, err := db.Exec(ctx, query); err != nil {
			t.Fatalf("seed currency: %v", err)
		}
	}
	return scope
}

func TestLedgerRejectsNegativeSpend(t *testing.T) {
	db := disposableDatabase(t)
	scope := seedCurrency(t, db.pool)
	_, err := points.NewLedger(db.pool).Spend(context.Background(), scope, "user-1", "points", -5, "negative-spend", nil)
	if err == nil {
		t.Fatal("negative spend unexpectedly succeeded")
	}
	var count int
	if err := db.pool.QueryRow(context.Background(), `SELECT count(*) FROM point_ledger_entries`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("ledger entries = %d; want 0", count)
	}
}

func TestLedgerConcurrentSpendAndIdempotency(t *testing.T) {
	db := disposableDatabase(t)
	scope := seedCurrency(t, db.pool)
	ledger := points.NewLedger(db.pool)
	ctx := context.Background()
	award, err := ledger.Award(ctx, scope, "user-1", "points", 100, "award-once", nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := ledger.Award(ctx, scope, "user-1", "points", 100, "award-once", nil)
	if err != nil || again.ID != award.ID {
		t.Fatalf("duplicate award = %+v, %v; want original entry %d", again, err, award.ID)
	}
	if _, err := ledger.Award(ctx, scope, "user-1", "points", 101, "award-once", nil); !errors.Is(err, points.ErrIdempotencyConflict) {
		t.Fatalf("changed award error = %v; want idempotency conflict", err)
	}
	var results [2]error
	var workers sync.WaitGroup
	for index := range results {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			_, results[index] = ledger.Spend(ctx, scope, "user-1", "points", 80, fmt.Sprintf("spend-%d", index), nil)
		}(index)
	}
	workers.Wait()
	successes, insufficient := 0, 0
	for _, err := range results {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, points.ErrInsufficientBalance):
			insufficient++
		default:
			t.Fatalf("unexpected concurrent spend error: %v", err)
		}
	}
	if successes != 1 || insufficient != 1 {
		t.Fatalf("concurrent spends: %d succeeded, %d insufficient; want 1 each", successes, insufficient)
	}
	var balance, ledgerSum, entries int64
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE external_user_id = 'user-1'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT coalesce(sum(amount), 0), count(*) FROM point_ledger_entries`).Scan(&ledgerSum, &entries); err != nil {
		t.Fatal(err)
	}
	if balance != 20 || ledgerSum != 20 || entries != 2 {
		t.Fatalf("balance=%d ledger_sum=%d entries=%d; want 20, 20, 2", balance, ledgerSum, entries)
	}
}

func TestConversionCreatesSpendableLots(t *testing.T) {
	db := disposableDatabase(t)
	scope := seedCurrency(t, db.pool)
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO currencies (id, tenant_id, environment_id, code, name) VALUES ('bonus-currency', 'integration-tenant', 'sandbox', 'bonus', 'Bonus')`,
		`INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ('bonus-currency', 1, '{"expiry_days":30}'::jsonb)`,
	} {
		if _, err := db.pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	ledger := points.NewLedger(db.pool)
	if _, err := ledger.Award(ctx, scope, "user-1", "points", 100, "initial-award", nil); err != nil {
		t.Fatal(err)
	}
	request := points.ConversionRequest{Scope: scope, ExternalUserID: "user-1", SourceCurrency: "points", TargetCurrency: "bonus", Amount: 40, RateNumerator: 1, RateDenominator: 2, IdempotencyKey: "convert-once"}
	conversion, err := points.Convert(ctx, db.pool, request)
	if err != nil {
		t.Fatal(err)
	}
	again, err := points.Convert(ctx, db.pool, request)
	if err != nil || again.ID != conversion.ID {
		t.Fatalf("duplicate conversion = %+v, %v; want original %s", again, err, conversion.ID)
	}
	if _, err := ledger.Spend(ctx, scope, "user-1", "bonus", 10, "spend-converted", nil); err != nil {
		t.Fatalf("converted points could not be spent: %v", err)
	}
	var sourceBalance, sourceLots, targetBalance, targetLots int64
	if err := db.pool.QueryRow(ctx, `SELECT a.available_balance, coalesce(sum(l.remaining_amount), 0) FROM point_accounts a LEFT JOIN point_lots l ON l.account_id = a.id JOIN currencies c ON c.id = a.currency_id WHERE c.code = 'points' GROUP BY a.id`).Scan(&sourceBalance, &sourceLots); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT a.available_balance, coalesce(sum(l.remaining_amount), 0) FROM point_accounts a LEFT JOIN point_lots l ON l.account_id = a.id JOIN currencies c ON c.id = a.currency_id WHERE c.code = 'bonus' GROUP BY a.id`).Scan(&targetBalance, &targetLots); err != nil {
		t.Fatal(err)
	}
	if sourceBalance != 60 || sourceLots != 60 || targetBalance != 10 || targetLots != 10 {
		t.Fatalf("conversion balances/lots = %d/%d source, %d/%d target; want 60/60, 10/10", sourceBalance, sourceLots, targetBalance, targetLots)
	}
	var targetExpiry *time.Time
	if err := db.pool.QueryRow(ctx, `SELECT l.expires_at FROM point_lots l JOIN point_ledger_entries e ON e.id = l.award_entry_id WHERE e.entry_type = 'conversion_credit'`).Scan(&targetExpiry); err != nil {
		t.Fatal(err)
	}
	if targetExpiry == nil || targetExpiry.Before(time.Now().Add(29*24*time.Hour)) {
		t.Fatalf("converted lot expiry = %v; want about 30 days", targetExpiry)
	}
	// Force a lot shortage after the balance check, then verify every write in
	// the failed conversion rolls back together.
	if _, err := db.pool.Exec(ctx, `UPDATE point_lots SET remaining_amount = 0 WHERE account_id IN (SELECT id FROM point_accounts WHERE currency_id = 'integration-currency')`); err != nil {
		t.Fatal(err)
	}
	var ledgerBefore int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries`).Scan(&ledgerBefore); err != nil {
		t.Fatal(err)
	}
	request.IdempotencyKey = "conversion-with-missing-lots"
	request.Amount = 10
	if _, err := points.Convert(ctx, db.pool, request); err == nil {
		t.Fatal("conversion with insufficient lots unexpectedly succeeded")
	}
	var ledgerAfter, conversionsAfter int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries`).Scan(&ledgerAfter); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_conversions`).Scan(&conversionsAfter); err != nil {
		t.Fatal(err)
	}
	if ledgerAfter != ledgerBefore || conversionsAfter != 1 {
		t.Fatalf("failed conversion left %d ledger entries and %d conversions; want %d and 1", ledgerAfter, conversionsAfter, ledgerBefore)
	}
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE currency_id = 'integration-currency'`).Scan(&sourceBalance); err != nil || sourceBalance != 60 {
		t.Fatalf("source balance after failed conversion = %d, %v; want 60", sourceBalance, err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE currency_id = 'bonus-currency'`).Scan(&targetBalance); err != nil || targetBalance != 10 {
		t.Fatalf("target balance after failed conversion = %d, %v; want 10", targetBalance, err)
	}
}

func TestConcurrentConversionIdempotency(t *testing.T) {
	db := disposableDatabase(t)
	scope := seedCurrency(t, db.pool)
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO currencies (id, tenant_id, environment_id, code, name) VALUES ('bonus-currency', 'integration-tenant', 'sandbox', 'bonus', 'Bonus')`,
		`INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ('bonus-currency', 1, '{}'::jsonb)`,
	} {
		if _, err := db.pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := points.NewLedger(db.pool).Award(ctx, scope, "user-1", "points", 100, "initial-award", nil); err != nil {
		t.Fatal(err)
	}
	for attempt := 0; attempt < 16; attempt++ {
		request := points.ConversionRequest{Scope: scope, ExternalUserID: "user-1", SourceCurrency: "points", TargetCurrency: "bonus", Amount: 1, RateNumerator: 1, RateDenominator: 1, IdempotencyKey: fmt.Sprintf("concurrent-%d", attempt)}
		start := make(chan struct{})
		type result struct {
			conversion points.Conversion
			err        error
		}
		outcomes := make(chan result, 2)
		for worker := 0; worker < 2; worker++ {
			go func() {
				<-start
				conversion, err := points.Convert(ctx, db.pool, request)
				outcomes <- result{conversion, err}
			}()
		}
		close(start)
		first, second := <-outcomes, <-outcomes
		if first.err != nil || second.err != nil || first.conversion.ID == "" || first.conversion.ID != second.conversion.ID {
			t.Fatalf("attempt %d duplicate conversion = %+v, %+v; want same successful result", attempt, first, second)
		}
	}
	var conversions, sourceBalance, targetBalance int
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_conversions`).Scan(&conversions); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE currency_id = 'integration-currency'`).Scan(&sourceBalance); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE currency_id = 'bonus-currency'`).Scan(&targetBalance); err != nil {
		t.Fatal(err)
	}
	if conversions != 16 || sourceBalance != 84 || targetBalance != 16 {
		t.Fatalf("conversions=%d balances=%d/%d; want 16 and 84/16", conversions, sourceBalance, targetBalance)
	}
}

func TestConversionLotBackfill(t *testing.T) {
	db := disposableDatabase(t)
	scope := seedCurrency(t, db.pool)
	ctx := context.Background()
	for _, query := range []string{
		`INSERT INTO currencies (id, tenant_id, environment_id, code, name) VALUES ('bonus-currency', 'integration-tenant', 'sandbox', 'bonus', 'Bonus')`,
		`INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ('bonus-currency', 1, '{}'::jsonb)`,
	} {
		if _, err := db.pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	ledger := points.NewLedger(db.pool)
	if _, err := ledger.Award(ctx, scope, "user-1", "points", 100, "initial-award", nil); err != nil {
		t.Fatal(err)
	}
	request := points.ConversionRequest{Scope: scope, ExternalUserID: "user-1", SourceCurrency: "points", TargetCurrency: "bonus", Amount: 40, RateNumerator: 1, RateDenominator: 2, IdempotencyKey: "old-conversion"}
	conversion, err := points.Convert(ctx, db.pool, request)
	if err != nil {
		t.Fatal(err)
	}
	// Recreate the exact legacy state: the ledger and balances exist, but the
	// conversion debit has no allocations and its credit has no lot.
	if _, err := db.pool.Exec(ctx, `UPDATE point_lots SET remaining_amount = original_amount WHERE account_id IN (SELECT id FROM point_accounts WHERE external_user_id = 'user-1')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `DELETE FROM point_lot_allocations WHERE ledger_entry_id IN (SELECT id FROM point_ledger_entries WHERE provenance->>'conversion_id' = $1)`, conversion.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := db.pool.Exec(ctx, `DELETE FROM point_lots WHERE award_entry_id IN (SELECT id FROM point_ledger_entries WHERE provenance->>'conversion_id' = $1)`, conversion.ID); err != nil {
		t.Fatal(err)
	}
	runMigration(t, db.url, "down")
	runMigration(t, db.url, "up")
	runMigration(t, db.url, "up")
	if _, err := ledger.Spend(ctx, scope, "user-1", "bonus", 20, "spend-restored", nil); err != nil {
		t.Fatalf("restored converted points could not be spent: %v", err)
	}
	var sourceBalance, sourceLots, targetBalance, targetLots int64
	for _, item := range []struct {
		code    string
		balance *int64
		lots    *int64
	}{
		{"points", &sourceBalance, &sourceLots},
		{"bonus", &targetBalance, &targetLots},
	} {
		if err := db.pool.QueryRow(ctx, `SELECT a.available_balance, coalesce(sum(l.remaining_amount), 0) FROM point_accounts a LEFT JOIN point_lots l ON l.account_id = a.id JOIN currencies c ON c.id = a.currency_id WHERE c.code = $1 GROUP BY a.id`, item.code).Scan(item.balance, item.lots); err != nil {
			t.Fatal(err)
		}
	}
	if sourceBalance != 60 || sourceLots != 60 || targetBalance != 0 || targetLots != 0 {
		t.Fatalf("backfilled balances/lots = %d/%d source, %d/%d target; want 60/60, 0/0", sourceBalance, sourceLots, targetBalance, targetLots)
	}
}

func TestConvertedPointsRedeemReward(t *testing.T) {
	db := disposableDatabase(t)
	scope := seedCurrency(t, db.pool)
	ctx := context.Background()
	apiKey := "redemption-integration-key"
	for _, query := range []string{
		`INSERT INTO currencies (id, tenant_id, environment_id, code, name) VALUES ('bonus-currency', 'integration-tenant', 'sandbox', 'bonus', 'Bonus')`,
		`INSERT INTO currency_policy_versions (currency_id, version, policy) VALUES ('bonus-currency', 1, '{}'::jsonb)`,
		`INSERT INTO rewards (id, tenant_id, environment_id, name, reward_type, cost_currency_id, cost_amount, payload) VALUES ('integration-reward', 'integration-tenant', 'sandbox', 'Digital reward', 'digital_unlock', 'bonus-currency', 15, '{}'::jsonb)`,
	} {
		if _, err := db.pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.pool.Exec(ctx, `INSERT INTO integration_credentials (tenant_id, environment_id, key_hash) VALUES ($1, $2, $3)`, scope.TenantID, scope.EnvironmentID, auth.Digest(apiKey)); err != nil {
		t.Fatal(err)
	}
	ledger := points.NewLedger(db.pool)
	if _, err := ledger.Award(ctx, scope, "user-1", "points", 40, "initial-award", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := points.Convert(ctx, db.pool, points.ConversionRequest{Scope: scope, ExternalUserID: "user-1", SourceCurrency: "points", TargetCurrency: "bonus", Amount: 40, RateNumerator: 1, RateDenominator: 2, IdempotencyKey: "convert-for-reward"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(rewards.NewHandler(db.pool, rewards.MockProvider{}))
	t.Cleanup(server.Close)
	post := func() rewards.Redemption {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/rewards/integration-reward/redemptions", bytes.NewBufferString(`{"external_user_id":"user-1","idempotency_key":"redeem-once"}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Motif-API-Key", apiKey)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("redemption response = %d: %s", response.StatusCode, body)
		}
		var redemption rewards.Redemption
		if err := json.NewDecoder(response.Body).Decode(&redemption); err != nil {
			t.Fatal(err)
		}
		return redemption
	}
	first := post()
	second := post()
	if first.ID == "" || first.ID != second.ID || first.Status != "fulfilled" || second.Status != "fulfilled" || first.ProviderReference == "" {
		t.Fatalf("redemption/idempotency = %+v, %+v; want one fulfilled delivery", first, second)
	}
	var balance, lots int64
	var spends, attempts int
	if err := db.pool.QueryRow(ctx, `SELECT a.available_balance, coalesce(sum(l.remaining_amount), 0) FROM point_accounts a LEFT JOIN point_lots l ON l.account_id = a.id WHERE a.external_user_id = 'user-1' AND a.currency_id = 'bonus-currency' GROUP BY a.id`).Scan(&balance, &lots); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'spend'`).Scan(&spends); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM reward_delivery_attempts`).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if balance != 5 || lots != 5 || spends != 1 || attempts != 1 {
		t.Fatalf("redemption balance/lots/spends/deliveries = %d/%d/%d/%d; want 5/5/1/1", balance, lots, spends, attempts)
	}
}

func seedPaidReward(t *testing.T, db *pgxpool.Pool) (points.Scope, string) {
	t.Helper()
	scope := seedCurrency(t, db)
	apiKey := "reward-provider-integration-key"
	ctx := context.Background()
	if _, err := db.Exec(ctx, `INSERT INTO rewards (id, tenant_id, environment_id, name, reward_type, cost_currency_id, cost_amount, payload) VALUES ('integration-reward', 'integration-tenant', 'sandbox', 'Paid reward', 'digital_unlock', 'integration-currency', 15, '{}'::jsonb)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO integration_credentials (tenant_id, environment_id, key_hash) VALUES ($1, $2, $3)`, scope.TenantID, scope.EnvironmentID, auth.Digest(apiKey)); err != nil {
		t.Fatal(err)
	}
	if _, err := points.NewLedger(db).Award(ctx, scope, "user-1", "points", 20, "reward-seed-award", nil); err != nil {
		t.Fatal(err)
	}
	return scope, apiKey
}

func sendRewardPost(t *testing.T, server *httptest.Server, apiKey, path string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, server.URL+path, bytes.NewBufferString(`{"external_user_id":"user-1","idempotency_key":"provider-case"}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Motif-API-Key", apiKey)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body
}

func rewardBalance(t *testing.T, db *pgxpool.Pool) (int64, int64) {
	t.Helper()
	var balance, lots int64
	if err := db.QueryRow(context.Background(), `SELECT a.available_balance, coalesce(sum(l.remaining_amount), 0) FROM point_accounts a LEFT JOIN point_lots l ON l.account_id = a.id WHERE a.external_user_id = 'user-1' AND a.currency_id = 'integration-currency' GROUP BY a.id`).Scan(&balance, &lots); err != nil {
		t.Fatal(err)
	}
	return balance, lots
}

func TestRewardDefinitiveFailureRefundsAndRetryChargesOnce(t *testing.T) {
	db := disposableDatabase(t)
	scope, apiKey := seedPaidReward(t, db.pool)
	provider := &controlledRewardProvider{deliveryErr: errors.New("provider rejected delivery")}
	server := httptest.NewServer(rewards.NewHandler(db.pool, provider))
	t.Cleanup(server.Close)
	status, body := sendRewardPost(t, server, apiKey, "/v1/rewards/integration-reward/redemptions")
	if status != http.StatusInternalServerError {
		t.Fatalf("definitive provider failure returned %d: %s; want 500", status, body)
	}
	var redemptionID, redemptionStatus string
	if err := db.pool.QueryRow(context.Background(), `SELECT id, status FROM redemptions WHERE tenant_id = $1 AND environment_id = $2`, scope.TenantID, scope.EnvironmentID).Scan(&redemptionID, &redemptionStatus); err != nil {
		t.Fatal(err)
	}
	if balance, lots := rewardBalance(t, db.pool); redemptionStatus != "failed" || balance != 20 || lots != 20 {
		t.Fatalf("failed reward = %s, balance/lots = %d/%d; want failed and 20/20", redemptionStatus, balance, lots)
	}
	provider.mu.Lock()
	provider.deliveryErr = nil
	provider.mu.Unlock()
	status, body = sendRewardPost(t, server, apiKey, "/v1/redemptions/"+redemptionID+"/retry")
	if status != http.StatusOK {
		t.Fatalf("reward retry returned %d: %s; want 200", status, body)
	}
	var redeemed rewards.Redemption
	if err := json.Unmarshal(body, &redeemed); err != nil {
		t.Fatal(err)
	}
	if redeemed.ID != redemptionID || redeemed.Status != "fulfilled" || redeemed.ProviderReference == "" {
		t.Fatalf("retried redemption = %+v; want fulfilled with provider reference", redeemed)
	}
	if balance, lots := rewardBalance(t, db.pool); balance != 5 || lots != 5 {
		t.Fatalf("retry balance/lots = %d/%d; want 5/5", balance, lots)
	}
	var spends, releases, attempts int
	for _, item := range []struct {
		query string
		value *int
	}{
		{`SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'spend'`, &spends},
		{`SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'award' AND idempotency_key LIKE 'redemption:%:release'`, &releases},
		{`SELECT count(*) FROM reward_delivery_attempts`, &attempts},
	} {
		if err := db.pool.QueryRow(context.Background(), item.query).Scan(item.value); err != nil {
			t.Fatal(err)
		}
	}
	if spends != 2 || releases != 1 || attempts != 2 {
		t.Fatalf("retry ledger/attempts = %d spends, %d releases, %d attempts; want 2/1/2", spends, releases, attempts)
	}
}

func TestUnknownRewardDeliveryReconcilesWithoutRedelivery(t *testing.T) {
	for _, scenario := range []struct {
		name        string
		state       string
		wantBalance int64
	}{
		{name: "fulfilled", state: "fulfilled", wantBalance: 5},
		{name: "failed", state: "failed", wantBalance: 20},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			db := disposableDatabase(t)
			_, apiKey := seedPaidReward(t, db.pool)
			provider := &controlledRewardProvider{deliveryErr: rewards.ErrDeliveryUnknown, status: rewards.DeliveryStatus{State: scenario.state, ProviderReference: "provider-confirmed", FailureReason: "provider-confirmed-failure"}}
			server := httptest.NewServer(rewards.NewHandler(db.pool, provider))
			t.Cleanup(server.Close)
			status, body := sendRewardPost(t, server, apiKey, "/v1/rewards/integration-reward/redemptions")
			if status != http.StatusAccepted {
				t.Fatalf("unknown delivery returned %d: %s; want 202", status, body)
			}
			if balance, lots := rewardBalance(t, db.pool); balance != 5 || lots != 5 {
				t.Fatalf("unresolved delivery balance/lots = %d/%d; want reserved 5/5", balance, lots)
			}
			reconciler := rewards.NewReconciler(db.pool, provider, slog.New(slog.NewTextHandler(io.Discard, nil)))
			processed, err := reconciler.ReconcileOnce(context.Background(), 10)
			if err != nil || processed != 1 {
				t.Fatalf("reconciliation processed %d, error %v; want one", processed, err)
			}
			processed, err = reconciler.ReconcileOnce(context.Background(), 10)
			if err != nil || processed != 0 {
				t.Fatalf("duplicate reconciliation processed %d, error %v; want zero", processed, err)
			}
			var redemptionStatus string
			if err := db.pool.QueryRow(context.Background(), `SELECT status FROM redemptions`).Scan(&redemptionStatus); err != nil {
				t.Fatal(err)
			}
			balance, lots := rewardBalance(t, db.pool)
			provider.mu.Lock()
			deliverCalls, statusCalls := provider.deliverCalls, provider.statusCalls
			provider.mu.Unlock()
			if redemptionStatus != scenario.state || balance != scenario.wantBalance || lots != scenario.wantBalance || deliverCalls != 1 || statusCalls != 1 {
				t.Fatalf("reconciled state=%s balance/lots=%d/%d calls=%d/%d; want %s, %d/%d, 1/1", redemptionStatus, balance, lots, deliverCalls, statusCalls, scenario.state, scenario.wantBalance, scenario.wantBalance)
			}
		})
	}
}

func TestEventOutboxJetStreamAndIdempotentAward(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	ctx := context.Background()
	apiKey := "integration-only-api-key"
	if _, err := db.pool.Exec(ctx, `INSERT INTO integration_credentials (tenant_id, environment_id, key_hash) VALUES ('integration-tenant', 'sandbox', $1)`, auth.Digest(apiKey)); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO event_types (tenant_id, environment_id, name, schema) VALUES ('integration-tenant', 'sandbox', 'order.completed', '{}'::jsonb)`,
		`INSERT INTO rules (id, tenant_id, environment_id, name, event_type, conditions, action, status, published_at) VALUES ('integration-rule', 'integration-tenant', 'sandbox', 'Award points', 'order.completed', '{}'::jsonb, '{"type":"award_points","currency":"points","amount":25}'::jsonb, 'published', now())`,
		`INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, status, published_at) VALUES ('integration-rule:v1', 'integration-rule', 1, 'order.completed', '{}'::jsonb, '{"type":"award_points","currency":"points","amount":25}'::jsonb, 'published', now())`,
	} {
		if _, err := db.pool.Exec(ctx, query); err != nil {
			t.Fatalf("seed event rule: %v", err)
		}
	}
	server := httptest.NewServer(events.NewHandler(db.pool))
	t.Cleanup(server.Close)
	postEvent := func() events.Receipt {
		t.Helper()
		request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/events", bytes.NewBufferString(`{"event_id":"event-1","event_type":"order.completed","user_id":"user-1","payload":{"amount":10}}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("X-Motif-API-Key", apiKey)
		response, err := server.Client().Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		if response.StatusCode != http.StatusAccepted {
			body, _ := io.ReadAll(response.Body)
			t.Fatalf("event response = %d: %s", response.StatusCode, body)
		}
		var receipt events.Receipt
		if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil {
			t.Fatal(err)
		}
		return receipt
	}
	receipt := postEvent()
	var pending bool
	if err := db.pool.QueryRow(ctx, `SELECT published_at IS NULL FROM event_outbox WHERE event_id = $1`, receipt.EventID).Scan(&pending); err != nil || !pending {
		t.Fatalf("outbox was not durable before worker start: pending=%v error=%v", pending, err)
	}
	natsURL := os.Getenv("MOTIF_INTEGRATION_NATS_URL")
	if natsURL == "" {
		t.Fatal("MOTIF_INTEGRATION_NATS_URL must point to an isolated local JetStream server")
	}
	parsed, err := url.Parse(natsURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatal("integration tests require a local NATS URL")
	}
	nc, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatalf("connect to isolated NATS: %v", err)
	}
	t.Cleanup(nc.Close)
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream("MOTIF_EVENTS"); err != nil && !errors.Is(err, nats.ErrStreamNotFound) {
			t.Errorf("delete isolated NATS stream: %v", err)
		}
	})
	if _, err := js.StreamInfo("MOTIF_EVENTS"); err == nil {
		t.Fatal("NATS stream already exists; refusing to use a shared development server")
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		t.Fatalf("inspect NATS stream: %v", err)
	}
	workerCtx, stopWorker := context.WithCancel(context.Background())
	t.Cleanup(stopWorker)
	processor := processing.New(db.pool, js, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err := processor.Start(workerCtx); err != nil {
		t.Fatalf("start worker processor: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var status string
		var awards int
		if err := db.pool.QueryRow(ctx, `SELECT status FROM events WHERE id = $1`, receipt.EventID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'award'`).Scan(&awards); err != nil {
			t.Fatal(err)
		}
		if status == "processed" && awards == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("event did not process: status=%s awards=%d", status, awards)
		}
		time.Sleep(50 * time.Millisecond)
	}
	duplicate := postEvent()
	if duplicate.EventID != receipt.EventID {
		t.Fatalf("duplicate event ID = %d; want %d", duplicate.EventID, receipt.EventID)
	}
	consumer, err := js.ConsumerInfo("MOTIF_EVENTS", "motif-event-processor")
	if err != nil {
		t.Fatal(err)
	}
	previousDelivery := consumer.Delivered.Stream
	message, _ := json.Marshal(map[string]int64{"event_id": receipt.EventID})
	if _, err := js.Publish("motif.events.received", message); err != nil {
		t.Fatal(err)
	}
	deadline = time.Now().Add(10 * time.Second)
	for {
		consumer, err = js.ConsumerInfo("MOTIF_EVENTS", "motif-event-processor")
		if err != nil {
			t.Fatal(err)
		}
		if consumer.Delivered.Stream > previousDelivery && consumer.NumAckPending == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("duplicate broker delivery was not acknowledged")
		}
		time.Sleep(50 * time.Millisecond)
	}
	var awards, executions, balance, attempts int
	var published bool
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'award'`).Scan(&awards); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM rule_executions WHERE event_id = $1`, receipt.EventID).Scan(&executions); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE external_user_id = 'user-1'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT published_at IS NOT NULL, attempts FROM event_outbox WHERE event_id = $1`, receipt.EventID).Scan(&published, &attempts); err != nil {
		t.Fatal(err)
	}
	if awards != 1 || executions != 1 || balance != 25 || !published || attempts < 1 {
		t.Fatalf("awards=%d executions=%d balance=%d published=%v attempts=%d; want 1, 1, 25, true, >=1", awards, executions, balance, published, attempts)
	}
}

func TestOutboxSurvivesWorkerRestart(t *testing.T) {
	db := disposableDatabase(t)
	seedCurrency(t, db.pool)
	ctx := context.Background()
	apiKey := "restart-integration-key"
	if _, err := db.pool.Exec(ctx, `INSERT INTO integration_credentials (tenant_id, environment_id, key_hash) VALUES ('integration-tenant', 'sandbox', $1)`, auth.Digest(apiKey)); err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{
		`INSERT INTO event_types (tenant_id, environment_id, name, schema) VALUES ('integration-tenant', 'sandbox', 'restart.event', '{}'::jsonb)`,
		`INSERT INTO rules (id, tenant_id, environment_id, name, event_type, conditions, action, status, published_at) VALUES ('restart-rule', 'integration-tenant', 'sandbox', 'Restart award', 'restart.event', '{}'::jsonb, '{"type":"award_points","currency":"points","amount":7}'::jsonb, 'published', now())`,
		`INSERT INTO rule_versions (id, rule_id, version, event_type, conditions, action, status, published_at) VALUES ('restart-rule:v1', 'restart-rule', 1, 'restart.event', '{}'::jsonb, '{"type":"award_points","currency":"points","amount":7}'::jsonb, 'published', now())`,
	} {
		if _, err := db.pool.Exec(ctx, query); err != nil {
			t.Fatal(err)
		}
	}
	natsURL := os.Getenv("MOTIF_INTEGRATION_NATS_URL")
	parsed, err := url.Parse(natsURL)
	if err != nil || (parsed.Hostname() != "localhost" && parsed.Hostname() != "127.0.0.1") {
		t.Fatal("integration tests require an isolated local NATS URL")
	}
	firstConnection, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(firstConnection.Close)
	firstJetStream, err := firstConnection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := firstJetStream.StreamInfo("MOTIF_EVENTS"); err == nil {
		t.Fatal("NATS stream already exists; refusing to use a shared development server")
	} else if !errors.Is(err, nats.ErrStreamNotFound) {
		t.Fatal(err)
	}
	workerContext, stopFirst := context.WithCancel(context.Background())
	t.Cleanup(stopFirst)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if err := processing.New(db.pool, firstJetStream, logger).Start(workerContext); err != nil {
		t.Fatal(err)
	}
	if _, err := firstJetStream.ConsumerInfo("MOTIF_EVENTS", "motif-event-processor"); err != nil {
		t.Fatalf("first worker did not establish durable consumer: %v", err)
	}
	stopFirst()
	firstConnection.Close()
	server := httptest.NewServer(events.NewHandler(db.pool))
	t.Cleanup(server.Close)
	request, err := http.NewRequest(http.MethodPost, server.URL+"/v1/events", bytes.NewBufferString(`{"event_id":"restart-event-1","event_type":"restart.event","user_id":"user-1","payload":{}}`))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Motif-API-Key", apiKey)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("event receipt after worker stopped = %d: %s", response.StatusCode, body)
	}
	var receipt events.Receipt
	if err := json.NewDecoder(response.Body).Decode(&receipt); err != nil {
		t.Fatal(err)
	}
	var pending bool
	if err := db.pool.QueryRow(ctx, `SELECT published_at IS NULL FROM event_outbox WHERE event_id = $1`, receipt.EventID).Scan(&pending); err != nil || !pending {
		t.Fatalf("outbox before worker restart: pending=%v error=%v", pending, err)
	}
	secondConnection, err := nats.Connect(natsURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(secondConnection.Close)
	secondJetStream, err := secondConnection.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := secondJetStream.DeleteStream("MOTIF_EVENTS"); err != nil && !errors.Is(err, nats.ErrStreamNotFound) {
			t.Errorf("delete isolated NATS stream: %v", err)
		}
	})
	secondContext, stopSecond := context.WithCancel(context.Background())
	t.Cleanup(stopSecond)
	if err := processing.New(db.pool, secondJetStream, logger).Start(secondContext); err != nil {
		t.Fatalf("restart processor: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		var status string
		var awards int
		if err := db.pool.QueryRow(ctx, `SELECT status FROM events WHERE id = $1`, receipt.EventID).Scan(&status); err != nil {
			t.Fatal(err)
		}
		if err := db.pool.QueryRow(ctx, `SELECT count(*) FROM point_ledger_entries WHERE entry_type = 'award'`).Scan(&awards); err != nil {
			t.Fatal(err)
		}
		if status == "processed" && awards == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("outbox was not recovered after worker restart: status=%s awards=%d", status, awards)
		}
		time.Sleep(50 * time.Millisecond)
	}
	var balance int64
	var published bool
	if err := db.pool.QueryRow(ctx, `SELECT available_balance FROM point_accounts WHERE external_user_id = 'user-1'`).Scan(&balance); err != nil {
		t.Fatal(err)
	}
	if err := db.pool.QueryRow(ctx, `SELECT published_at IS NOT NULL FROM event_outbox WHERE event_id = $1`, receipt.EventID).Scan(&published); err != nil {
		t.Fatal(err)
	}
	if balance != 7 || !published {
		t.Fatalf("restart balance=%d published=%v; want 7 and true", balance, published)
	}
}
