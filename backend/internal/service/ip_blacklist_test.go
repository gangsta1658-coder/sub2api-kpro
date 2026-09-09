package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type ipBlacklistSettingRepo struct {
	values map[string]string
}

type blockingIPBlacklistSettingRepo struct {
	mu              sync.Mutex
	values          map[string]string
	snapshotReady   chan struct{}
	releaseSnapshot chan struct{}
	blockNextGetAll bool
}

func (r *blockingIPBlacklistSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *blockingIPBlacklistSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *blockingIPBlacklistSettingRepo) Set(_ context.Context, key, value string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.values[key] = value
	return nil
}

func (r *blockingIPBlacklistSettingRepo) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := r.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (r *blockingIPBlacklistSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *blockingIPBlacklistSettingRepo) GetAll(ctx context.Context) (map[string]string, error) {
	r.mu.Lock()
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	block := r.blockNextGetAll
	r.blockNextGetAll = false
	r.mu.Unlock()

	if block {
		close(r.snapshotReady)
		select {
		case <-r.releaseSnapshot:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return result, nil
}

func (r *blockingIPBlacklistSettingRepo) Delete(context.Context, string) error { return nil }

func (r *ipBlacklistSettingRepo) Get(context.Context, string) (*Setting, error) {
	return nil, ErrSettingNotFound
}

func (r *ipBlacklistSettingRepo) GetValue(_ context.Context, key string) (string, error) {
	value, ok := r.values[key]
	if !ok {
		return "", ErrSettingNotFound
	}
	return value, nil
}

func (r *ipBlacklistSettingRepo) Set(_ context.Context, key, value string) error {
	r.values[key] = value
	return nil
}

func (r *ipBlacklistSettingRepo) GetMultiple(context.Context, []string) (map[string]string, error) {
	return map[string]string{}, nil
}

func (r *ipBlacklistSettingRepo) SetMultiple(_ context.Context, values map[string]string) error {
	for key, value := range values {
		r.values[key] = value
	}
	return nil
}

func (r *ipBlacklistSettingRepo) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(r.values))
	for key, value := range r.values {
		result[key] = value
	}
	return result, nil
}

func (r *ipBlacklistSettingRepo) Delete(context.Context, string) error { return nil }

func TestNormalizeIPBlacklistCanonicalizesAndDeduplicates(t *testing.T) {
	got, err := NormalizeIPBlacklist([]string{
		" 47.92.239.70 ",
		"47.92.239.70",
		"203.0.113.99/24",
		"2001:db8::1",
	})
	if err != nil {
		t.Fatalf("NormalizeIPBlacklist() error = %v", err)
	}
	want := []string{"2001:db8::1", "203.0.113.0/24", "47.92.239.70"}
	if len(got) != len(want) {
		t.Fatalf("NormalizeIPBlacklist() = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("NormalizeIPBlacklist()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestNormalizeIPBlacklistRejectsInvalidEntries(t *testing.T) {
	for _, value := range []string{"not-an-ip", "203.0.113.1/33", "2001:db8::1/129"} {
		if _, err := NormalizeIPBlacklist([]string{value}); err == nil {
			t.Errorf("NormalizeIPBlacklist(%q) accepted invalid entry", value)
		}
	}
}

func TestParseIPBlacklistRejectsMalformedPersistenceValues(t *testing.T) {
	for _, value := range []string{"", "null", `["203.0.113.0/24", null]`, `[""]`} {
		if _, err := parseIPBlacklistSetting(value); err == nil {
			t.Fatalf("parseIPBlacklistSetting(%q) accepted a malformed value", value)
		}
	}
}

func TestSettingServiceIPBlacklistPersistsAndMatchesCIDR(t *testing.T) {
	repo := &ipBlacklistSettingRepo{values: map[string]string{}}
	service := NewSettingService(repo, &config.Config{})

	got, err := service.SetIPBlacklist(context.Background(), []string{"203.0.113.0/24", "47.92.239.70"})
	if err != nil {
		t.Fatalf("SetIPBlacklist() error = %v", err)
	}
	if len(got) != 2 || !service.IsIPBlacklisted("203.0.113.42") || !service.IsIPBlacklisted("47.92.239.70") {
		t.Fatalf("blacklist did not match expected addresses: entries=%v", got)
	}
	if service.IsIPBlacklisted("203.0.114.42") {
		t.Fatal("blacklist matched an address outside the configured CIDR")
	}

	service.storeIPBlacklist(nil)
	if err := service.LoadIPBlacklistSettings(context.Background()); err != nil {
		t.Fatalf("LoadIPBlacklistSettings() error = %v", err)
	}
	if !service.IsIPBlacklisted("203.0.113.42") {
		t.Fatal("persisted blacklist was not loaded into the runtime cache")
	}
}

func TestSettingServiceIPBlacklistRetainsLastValidRulesOnMalformedReload(t *testing.T) {
	repo := &ipBlacklistSettingRepo{values: map[string]string{
		SettingKeyIPBlacklist: `["203.0.113.0/24"]`,
	}}
	svc := NewSettingService(repo, &config.Config{})
	if err := svc.LoadIPBlacklistSettings(context.Background()); err != nil {
		t.Fatalf("initial LoadIPBlacklistSettings() error = %v", err)
	}
	repo.values[SettingKeyIPBlacklist] = `null`
	if err := svc.LoadIPBlacklistSettings(context.Background()); err == nil {
		t.Fatal("malformed reload unexpectedly succeeded")
	}
	if !svc.IsIPBlacklisted("203.0.113.42") {
		t.Fatal("malformed reload cleared the last valid runtime rule")
	}
}

func TestSettingServiceParseSettingsRetainsLastValidRulesOnMalformedValue(t *testing.T) {
	repo := &ipBlacklistSettingRepo{values: map[string]string{}}
	svc := NewSettingService(repo, &config.Config{})
	if _, err := svc.SetIPBlacklist(context.Background(), []string{"203.0.113.0/24"}); err != nil {
		t.Fatalf("SetIPBlacklist() error = %v", err)
	}
	repo.values[SettingKeyIPBlacklist] = `{"broken":true}`
	settings, err := svc.GetAllSettings(context.Background())
	if err != nil {
		t.Fatalf("GetAllSettings() error = %v", err)
	}
	if len(settings.IPBlacklist) != 1 || settings.IPBlacklist[0] != "203.0.113.0/24" {
		t.Fatalf("malformed settings replaced last valid rules: %#v", settings.IPBlacklist)
	}
}

func TestPartialSettingsRefreshSerializesIPBlacklistCache(t *testing.T) {
	tests := []struct {
		name   string
		update func(context.Context, *SettingService) error
	}{
		{
			name: "system settings",
			update: func(ctx context.Context, svc *SettingService) error {
				return svc.UpdateSettingsOmitting(ctx, &SystemSettings{}, OmittedSettingKeys{
					SettingKeyIPBlacklist: struct{}{},
				})
			},
		},
		{
			name: "settings with auth source defaults",
			update: func(ctx context.Context, svc *SettingService) error {
				return svc.UpdateSettingsWithAuthSourceDefaultsOmitting(ctx, &SystemSettings{}, nil, OmittedSettingKeys{
					SettingKeyIPBlacklist: struct{}{},
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &blockingIPBlacklistSettingRepo{
				values: map[string]string{
					SettingKeyIPBlacklist: `["203.0.113.0/24"]`,
				},
				snapshotReady:   make(chan struct{}),
				releaseSnapshot: make(chan struct{}),
				blockNextGetAll: true,
			}
			svc := NewSettingService(repo, &config.Config{})
			if err := svc.LoadIPBlacklistSettings(context.Background()); err != nil {
				t.Fatalf("load initial IP blacklist: %v", err)
			}

			updateDone := make(chan error, 1)
			go func() {
				updateDone <- tt.update(context.Background(), svc)
			}()

			select {
			case <-repo.snapshotReady:
			case <-time.After(time.Second):
				t.Fatal("partial settings update did not begin its cache refresh")
			}

			released := false
			defer func() {
				if !released {
					close(repo.releaseSnapshot)
				}
			}()
			if svc.ipBlacklistMu.TryLock() {
				svc.ipBlacklistMu.Unlock()
				t.Fatal("partial settings cache refresh did not hold the IP blacklist lock")
			}

			addDone := make(chan error, 1)
			go func() {
				_, err := svc.AddIPBlacklistEntry(context.Background(), "47.92.239.70")
				addDone <- err
			}()

			close(repo.releaseSnapshot)
			released = true

			select {
			case err := <-updateDone:
				if err != nil {
					t.Fatalf("partial settings update: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("partial settings update did not finish")
			}
			select {
			case err := <-addDone:
				if err != nil {
					t.Fatalf("add IP blacklist entry: %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("IP blacklist update did not finish")
			}
			if !svc.IsIPBlacklisted("47.92.239.70") {
				t.Fatal("partial settings refresh replaced the newer IP blacklist cache")
			}
		})
	}
}
