package entitlements

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrLimitExceeded is returned when a tenant has consumed the configured
// allowance for a plan entitlement.
var ErrLimitExceeded = errors.New("plan entitlement limit exceeded")

type LimitError struct {
	Key     string
	Limit   int64
	Current int64
	Want    int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("entitlement %s allows %d, current %d, requested %d", e.Key, e.Limit, e.Current, e.Want)
}

func (e *LimitError) Unwrap() error { return ErrLimitExceeded }

type Plan struct {
	ID           string
	Name         string
	Entitlements map[string]int64
}

var supportedKeys = map[string]struct{}{
	"max_event_payload_bytes": {},
	"max_rules":               {},
	"max_currencies":          {},
	"max_rewards":             {},
	"max_leaderboards":        {},
}

func ValidateLimits(values map[string]int64) error {
	for key, value := range values {
		if _, ok := supportedKeys[key]; !ok {
			return fmt.Errorf("unsupported entitlement %q", key)
		}
		if value < 0 {
			return fmt.Errorf("%s must be a non-negative integer", key)
		}
	}
	return nil
}

// Store is the single boundary for loading and checking plan entitlements.
// The small TTL cache protects high-volume event ingestion from repeatedly
// decoding the same plan JSON while keeping plan changes responsive.
type Store struct {
	db    *pgxpool.Pool
	ttl   time.Duration
	mu    sync.Mutex
	cache map[string]cachedPlan
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

type cachedPlan struct {
	plan    Plan
	expires time.Time
}

func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db, ttl: 30 * time.Second, cache: make(map[string]cachedPlan)}
}

func (s *Store) Load(ctx context.Context, tenantID string) (Plan, error) {
	now := time.Now()
	s.mu.Lock()
	if cached, ok := s.cache[tenantID]; ok && now.Before(cached.expires) {
		s.mu.Unlock()
		return clonePlan(cached.plan), nil
	}
	s.mu.Unlock()

	var planID, name string
	var baseRaw, overridesRaw []byte
	err := s.db.QueryRow(ctx, `
		SELECT p.id, p.name, p.entitlements, tp.overrides
		FROM tenant_plans tp
		JOIN plans p ON p.id = tp.plan_id
		WHERE tp.tenant_id = $1 AND p.active = true`, tenantID).Scan(&planID, &name, &baseRaw, &overridesRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, fmt.Errorf("tenant %s has no active plan: %w", tenantID, err)
	}
	if err != nil {
		return Plan{}, err
	}
	base, err := decodeLimits(baseRaw)
	if err != nil {
		return Plan{}, fmt.Errorf("plan %s entitlements: %w", planID, err)
	}
	overrides, err := decodeLimits(overridesRaw)
	if err != nil {
		return Plan{}, fmt.Errorf("tenant %s overrides: %w", tenantID, err)
	}
	for key, value := range overrides {
		base[key] = value
	}
	plan := Plan{ID: planID, Name: name, Entitlements: base}
	s.mu.Lock()
	s.cache[tenantID] = cachedPlan{plan: clonePlan(plan), expires: now.Add(s.ttl)}
	s.mu.Unlock()
	return plan, nil
}

func (s *Store) Invalidate(tenantID string) {
	s.mu.Lock()
	delete(s.cache, tenantID)
	s.mu.Unlock()
}

func (s *Store) Limit(ctx context.Context, tenantID, key string) (int64, error) {
	plan, err := s.Load(ctx, tenantID)
	if err != nil {
		return 0, err
	}
	limit, ok := plan.Entitlements[key]
	if !ok {
		return 0, nil
	}
	return limit, nil
}

func (s *Store) Check(ctx context.Context, tenantID, key string, current, want int64) error {
	return s.check(ctx, s.db, tenantID, key, current, want)
}

// CheckWith evaluates a limit using the supplied connection or transaction.
// Resource creation uses this form so the count and insert can share a
// transaction-level advisory lock.
func (s *Store) CheckWith(ctx context.Context, querier rowQuerier, tenantID, key string, current, want int64) error {
	return s.check(ctx, querier, tenantID, key, current, want)
}

func (s *Store) check(ctx context.Context, querier rowQuerier, tenantID, key string, current, want int64) error {
	plan, err := loadPlan(ctx, querier, tenantID)
	if err != nil {
		return err
	}
	limit, ok := plan.Entitlements[key]
	if !ok || limit <= 0 || current+want <= limit {
		return nil
	}
	return &LimitError{Key: key, Limit: limit, Current: current, Want: want}
}

func loadPlan(ctx context.Context, querier rowQuerier, tenantID string) (Plan, error) {
	var planID, name string
	var baseRaw, overridesRaw []byte
	err := querier.QueryRow(ctx, `
		SELECT p.id, p.name, p.entitlements, tp.overrides
		FROM tenant_plans tp
		JOIN plans p ON p.id = tp.plan_id
		WHERE tp.tenant_id = $1 AND p.active = true`, tenantID).Scan(&planID, &name, &baseRaw, &overridesRaw)
	if errors.Is(err, pgx.ErrNoRows) {
		return Plan{}, fmt.Errorf("tenant %s has no active plan: %w", tenantID, err)
	}
	if err != nil {
		return Plan{}, err
	}
	base, err := decodeLimits(baseRaw)
	if err != nil {
		return Plan{}, fmt.Errorf("plan %s entitlements: %w", planID, err)
	}
	overrides, err := decodeLimits(overridesRaw)
	if err != nil {
		return Plan{}, fmt.Errorf("tenant %s overrides: %w", tenantID, err)
	}
	for key, value := range overrides {
		base[key] = value
	}
	return Plan{ID: planID, Name: name, Entitlements: base}, nil
}

func decodeLimits(raw []byte) (map[string]int64, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return map[string]int64{}, nil
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(values))
	for key, rawValue := range values {
		var value int64
		if err := json.Unmarshal(rawValue, &value); err != nil || value < 0 {
			return nil, fmt.Errorf("%s must be a non-negative integer", key)
		}
		result[key] = value
	}
	if err := ValidateLimits(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ParseLimits validates a JSON object used for plan entitlements or tenant
// overrides and returns a typed copy suitable for API responses.
func ParseLimits(raw []byte) (map[string]int64, error) {
	return decodeLimits(raw)
}

func clonePlan(plan Plan) Plan {
	copy := Plan{ID: plan.ID, Name: plan.Name, Entitlements: make(map[string]int64, len(plan.Entitlements))}
	for key, value := range plan.Entitlements {
		copy.Entitlements[key] = value
	}
	return copy
}

// Keys returns deterministic entitlement names for admin responses and docs.
func (p Plan) Keys() []string {
	keys := make([]string, 0, len(p.Entitlements))
	for key := range p.Entitlements {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
