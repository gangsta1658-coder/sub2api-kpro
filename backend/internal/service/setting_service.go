package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/xai"
	"golang.org/x/sync/singleflight"
)

const (
	GrokDefaultBaseURLModeAPI     = "api"
	GrokDefaultBaseURLModeUSEast1 = "us-east-1"
	GrokDefaultBaseURLModeUSWest2 = "us-west-2"
	GrokDefaultBaseURLModeEUWest1 = "eu-west-1"
	GrokDefaultBaseURLModeCLI     = "cli"
)

func normalizeGrokDefaultBaseURLMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case GrokDefaultBaseURLModeAPI:
		return GrokDefaultBaseURLModeAPI
	case GrokDefaultBaseURLModeUSEast1:
		return GrokDefaultBaseURLModeUSEast1
	case GrokDefaultBaseURLModeUSWest2:
		return GrokDefaultBaseURLModeUSWest2
	case GrokDefaultBaseURLModeEUWest1:
		return GrokDefaultBaseURLModeEUWest1
	case GrokDefaultBaseURLModeCLI:
		return GrokDefaultBaseURLModeCLI
	default:
		return GrokDefaultBaseURLModeCLI
	}
}

func GrokBaseURLForMode(mode string) string {
	switch normalizeGrokDefaultBaseURLMode(mode) {
	case GrokDefaultBaseURLModeAPI:
		return xai.DefaultBaseURL
	case GrokDefaultBaseURLModeUSEast1:
		return xai.DefaultUSEast1BaseURL
	case GrokDefaultBaseURLModeUSWest2:
		return xai.DefaultUSWest2BaseURL
	case GrokDefaultBaseURLModeEUWest1:
		return xai.DefaultEUWest1BaseURL
	default:
		return xai.DefaultCLIBaseURL
	}
}

func (s *SettingService) GetGrokDefaultBaseURLMode(ctx context.Context) string {
	if s == nil || s.settingRepo == nil {
		return GrokDefaultBaseURLModeCLI
	}
	dbCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), gatewayForwardingDBTimeout)
	defer cancel()
	raw, err := s.settingRepo.GetValue(dbCtx, SettingKeyGrokDefaultBaseURLMode)
	if err != nil {
		return GrokDefaultBaseURLModeCLI
	}
	return normalizeGrokDefaultBaseURLMode(raw)
}

func (s *SettingService) GetGrokDefaultBaseURL(ctx context.Context) string {
	return GrokBaseURLForMode(s.GetGrokDefaultBaseURLMode(ctx))
}

func (s *SettingService) ResolveGrokBaseURL(ctx context.Context, account *Account) string {
	def := xai.DefaultCLIBaseURL
	if s != nil {
		def = s.GetGrokDefaultBaseURL(ctx)
	}
	if account == nil {
		return def
	}
	return account.GetGrokBaseURLOr(def)
}

var (
	ErrRegistrationDisabled   = infraerrors.Forbidden("REGISTRATION_DISABLED", "registration is currently disabled")
	ErrSettingNotFound        = infraerrors.NotFound("SETTING_NOT_FOUND", "setting not found")
	ErrDefaultSubGroupInvalid = infraerrors.BadRequest(
		"DEFAULT_SUBSCRIPTION_GROUP_INVALID",
		"default subscription group must exist and be subscription type",
	)
	ErrDefaultSubGroupDuplicate = infraerrors.BadRequest(
		"DEFAULT_SUBSCRIPTION_GROUP_DUPLICATE",
		"default subscription group cannot be duplicated",
	)
)

type SettingRepository interface {
	Get(ctx context.Context, key string) (*Setting, error)
	GetValue(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string) error
	GetMultiple(ctx context.Context, keys []string) (map[string]string, error)
	SetMultiple(ctx context.Context, settings map[string]string) error
	GetAll(ctx context.Context) (map[string]string, error)
	Delete(ctx context.Context, key string) error
}

// DefaultSubscriptionGroupReader validates group references used by default subscriptions.
type DefaultSubscriptionGroupReader interface {
	GetByID(ctx context.Context, id int64) (*Group, error)
}

// WebSearchManagerBuilder creates a websearch.Manager from config (injected by infra layer).
// proxyURLs maps proxy ID to resolved URL for provider-level proxy support.
type WebSearchManagerBuilder func(cfg *WebSearchEmulationConfig, proxyURLs map[int64]string)

// SettingService 系统设置服务
type SettingService struct {
	settingRepo                 SettingRepository
	defaultSubGroupReader       DefaultSubscriptionGroupReader
	proxyRepo                   ProxyRepository // for resolving websearch provider proxy URLs
	cfg                         *config.Config
	onUpdate                    func() // Callback when settings are updated (for cache invalidation)
	version                     string // Application version
	webSearchManagerBuilder     WebSearchManagerBuilder
	antigravityUAVersionCache   atomic.Value // *cachedAntigravityUserAgentVersion
	antigravityUAVersionSF      singleflight.Group
	openAICodexUACache          atomic.Value // *cachedOpenAICodexUserAgent
	openAICodexUASF             singleflight.Group
	openAICodexVersionCache     atomic.Value // *cachedOpenAICodexClientVersion
	openAICodexVersionSF        singleflight.Group
	codexRestrictionPolicyCache atomic.Value // *cachedCodexRestrictionPolicy
	codexRestrictionPolicySF    singleflight.Group

	cyberSessionBlockRuntimeCache atomic.Value // *cachedCyberSessionBlockRuntime
	cyberSessionBlockRuntimeSF    singleflight.Group

	// panelRateLimitCache 面板 API 限流配置进程内缓存（*cachedPanelRateLimitSettings）。
	// 面板每个认证请求都会读取，禁止在热路径上直接访问 DB。
	panelRateLimitCache atomic.Value
	panelRateLimitSF    singleflight.Group

	// openAIQuotaAutoPauseSettingsCache holds the most recently observed quota auto-pause
	// settings. GetOpenAIQuotaAutoPauseSettings reads this atomic.Value on the request hot
	// path without ever blocking on the DB; when the cached entry expires, a background
	// goroutine refreshes it via openAIQuotaAutoPauseSettingsSF (stale-while-revalidate).
	// This per-service field also gives tests natural isolation — each SettingService
	// instance owns its own cache, no shared package-level state.
	openAIQuotaAutoPauseSettingsCache atomic.Value // *cachedOpenAIQuotaAutoPauseSettings
	openAIQuotaAutoPauseSettingsSF    singleflight.Group
	openAIAPIKeyHealthBreakerCache    atomic.Value // *cachedOpenAIAPIKeyHealthBreakerSettings

	channelMonitorRuntimeListenersMu sync.Mutex
	channelMonitorRuntimeListeners   []func()

	ipBlacklistMu    sync.Mutex
	ipBlacklistCache atomic.Value // *cachedIPBlacklist
}

type cachedIPBlacklist struct {
	patterns []string
	rules    *ip.CompiledIPRules
}

// DefaultPlatformQuotaSetting 单 platform 三档限额（nil = 沿用上层；0 = 显式禁用；>0 = 上限）
type DefaultPlatformQuotaSetting struct {
	DailyLimitUSD   *float64 `json:"daily"`
	WeeklyLimitUSD  *float64 `json:"weekly"`
	MonthlyLimitUSD *float64 `json:"monthly"`
}

type ProviderDefaultGrantSettings struct {
	Balance          float64
	Concurrency      int
	Subscriptions    []DefaultSubscriptionSetting
	GrantOnSignup    bool
	GrantOnFirstBind bool
	PlatformQuotas   map[string]*DefaultPlatformQuotaSetting // key = platform name
}

type AuthSourceDefaultSettings struct {
	Email                        ProviderDefaultGrantSettings
	LinuxDo                      ProviderDefaultGrantSettings
	OIDC                         ProviderDefaultGrantSettings
	WeChat                       ProviderDefaultGrantSettings
	GitHub                       ProviderDefaultGrantSettings
	Google                       ProviderDefaultGrantSettings
	DingTalk                     ProviderDefaultGrantSettings
	ForceEmailOnThirdPartySignup bool
}

type authSourceDefaultKeySet struct {
	// source 是 auth source 标识（如 "email"、"github"），仅用于 parse 时
	// slog.Warn 诊断输出，不再参与 key 拼接（platformQuotas 字段已存完整 key）。
	source           string
	balance          string
	concurrency      string
	subscriptions    string
	grantOnSignup    string
	grantOnFirstBind string
	platformQuotas   string // SettingKeyAuthSourcePlatformQuotas(source)
}

var (
	emailAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "email",
		balance:          SettingKeyAuthSourceDefaultEmailBalance,
		concurrency:      SettingKeyAuthSourceDefaultEmailConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultEmailSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultEmailGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultEmailGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("email"),
	}
	linuxDoAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "linuxdo",
		balance:          SettingKeyAuthSourceDefaultLinuxDoBalance,
		concurrency:      SettingKeyAuthSourceDefaultLinuxDoConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultLinuxDoSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultLinuxDoGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultLinuxDoGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("linuxdo"),
	}
	oidcAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "oidc",
		balance:          SettingKeyAuthSourceDefaultOIDCBalance,
		concurrency:      SettingKeyAuthSourceDefaultOIDCConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultOIDCSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultOIDCGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultOIDCGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("oidc"),
	}
	weChatAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "wechat",
		balance:          SettingKeyAuthSourceDefaultWeChatBalance,
		concurrency:      SettingKeyAuthSourceDefaultWeChatConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultWeChatSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultWeChatGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultWeChatGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("wechat"),
	}
	gitHubAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "github",
		balance:          SettingKeyAuthSourceDefaultGitHubBalance,
		concurrency:      SettingKeyAuthSourceDefaultGitHubConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultGitHubSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultGitHubGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultGitHubGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("github"),
	}
	googleAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "google",
		balance:          SettingKeyAuthSourceDefaultGoogleBalance,
		concurrency:      SettingKeyAuthSourceDefaultGoogleConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultGoogleSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultGoogleGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultGoogleGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("google"),
	}
	dingTalkAuthSourceDefaultKeys = authSourceDefaultKeySet{
		source:           "dingtalk",
		balance:          SettingKeyAuthSourceDefaultDingTalkBalance,
		concurrency:      SettingKeyAuthSourceDefaultDingTalkConcurrency,
		subscriptions:    SettingKeyAuthSourceDefaultDingTalkSubscriptions,
		grantOnSignup:    SettingKeyAuthSourceDefaultDingTalkGrantOnSignup,
		grantOnFirstBind: SettingKeyAuthSourceDefaultDingTalkGrantOnFirstBind,
		platformQuotas:   SettingKeyAuthSourcePlatformQuotas("dingtalk"),
	}
)

const (
	defaultAuthSourceBalance     = 0
	defaultAuthSourceConcurrency = 5
	defaultWeChatConnectMode     = "open"
	defaultWeChatConnectScopes   = "snsapi_login"
	defaultWeChatConnectFrontend = "/auth/wechat/callback"
	defaultGitHubOAuthAuthorize  = "https://github.com/login/oauth/authorize"
	defaultGitHubOAuthToken      = "https://github.com/login/oauth/access_token"
	defaultGitHubOAuthUserInfo   = "https://api.github.com/user"
	defaultGitHubOAuthEmails     = "https://api.github.com/user/emails"
	defaultGitHubOAuthScopes     = "read:user user:email"
	defaultGitHubOAuthFrontend   = "/auth/oauth/callback"
	defaultGoogleOAuthAuthorize  = "https://accounts.google.com/o/oauth2/v2/auth"
	defaultGoogleOAuthToken      = "https://oauth2.googleapis.com/token"
	defaultGoogleOAuthUserInfo   = "https://openidconnect.googleapis.com/v1/userinfo"
	defaultGoogleOAuthScopes     = "openid email profile"
	defaultGoogleOAuthFrontend   = "/auth/oauth/callback"
	defaultLoginAgreementMode    = "modal"
	defaultLoginAgreementDate    = "2026-03-31"
)

// NewSettingService 创建系统设置服务实例
func NewSettingService(settingRepo SettingRepository, cfg *config.Config) *SettingService {
	svc := &SettingService{
		settingRepo: settingRepo,
		cfg:         cfg,
	}
	svc.storeIPBlacklist(nil)
	return svc
}

// NormalizeIPBlacklist validates and canonicalizes IPv4/IPv6 addresses and CIDRs.
func NormalizeIPBlacklist(patterns []string) ([]string, error) {
	if len(patterns) > 1000 {
		return nil, fmt.Errorf("ip blacklist cannot contain more than 1000 entries")
	}
	seen := make(map[string]struct{}, len(patterns))
	result := make([]string, 0, len(patterns))
	for _, raw := range patterns {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		canonical := ""
		if strings.Contains(value, "/") {
			_, network, err := net.ParseCIDR(value)
			if err != nil || network == nil {
				return nil, fmt.Errorf("invalid IP or CIDR: %s", value)
			}
			canonical = network.String()
		} else if parsed := net.ParseIP(value); parsed != nil {
			canonical = parsed.String()
		} else {
			return nil, fmt.Errorf("invalid IP or CIDR: %s", value)
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}
	sort.Strings(result)
	return result, nil
}

func (s *SettingService) storeIPBlacklist(patterns []string) {
	if s == nil {
		return
	}
	copyPatterns := append([]string(nil), patterns...)
	s.ipBlacklistCache.Store(&cachedIPBlacklist{patterns: copyPatterns, rules: ip.CompileIPRules(copyPatterns)})
}

func (s *SettingService) cachedIPBlacklistPatterns() []string {
	if s == nil {
		return []string{}
	}
	cached, _ := s.ipBlacklistCache.Load().(*cachedIPBlacklist)
	if cached == nil {
		return []string{}
	}
	return append([]string(nil), cached.patterns...)
}

// LoadIPBlacklistSettings loads the site-wide IP blacklist at startup.
func (s *SettingService) LoadIPBlacklistSettings(ctx context.Context) error {
	if s == nil || s.settingRepo == nil {
		return nil
	}
	s.ipBlacklistMu.Lock()
	defer s.ipBlacklistMu.Unlock()
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyIPBlacklist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			s.storeIPBlacklist(nil)
			return nil
		}
		return fmt.Errorf("get ip blacklist: %w", err)
	}
	patterns, err := parseIPBlacklistSetting(raw)
	if err != nil {
		return err
	}
	s.storeIPBlacklist(patterns)
	return nil
}

// GetIPBlacklist returns the currently active normalized rules.
func (s *SettingService) GetIPBlacklist(ctx context.Context) ([]string, error) {
	if s == nil || s.settingRepo == nil {
		return []string{}, nil
	}
	s.ipBlacklistMu.Lock()
	defer s.ipBlacklistMu.Unlock()
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyIPBlacklist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("get ip blacklist: %w", err)
	}
	patterns, err := parseIPBlacklistSetting(raw)
	if err != nil {
		return nil, err
	}
	s.storeIPBlacklist(patterns)
	return append([]string(nil), patterns...), nil
}

// AddIPBlacklistEntry validates and appends one site-wide rule while holding
// the same lock used by replacement writes. The read-modify-write is atomic
// within this process, so concurrent administrators cannot lose an entry.
func (s *SettingService) AddIPBlacklistEntry(ctx context.Context, entry string) ([]string, error) {
	normalized, err := NormalizeIPBlacklist([]string{entry})
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_IP_BLACKLIST", err.Error())
	}
	if len(normalized) == 0 {
		return nil, infraerrors.BadRequest("INVALID_IP_BLACKLIST", "ip is required")
	}

	s.ipBlacklistMu.Lock()
	defer s.ipBlacklistMu.Unlock()
	items, err := s.getIPBlacklistFromStore(ctx)
	if err != nil {
		return nil, err
	}
	return s.setIPBlacklistLocked(ctx, append(items, normalized[0]))
}

// RemoveIPBlacklistEntry removes one normalized site-wide rule atomically.
func (s *SettingService) RemoveIPBlacklistEntry(ctx context.Context, entry string) ([]string, error) {
	normalized, err := NormalizeIPBlacklist([]string{entry})
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_IP_BLACKLIST", err.Error())
	}
	if len(normalized) == 0 {
		return nil, infraerrors.BadRequest("INVALID_IP_BLACKLIST", "ip is required")
	}

	s.ipBlacklistMu.Lock()
	defer s.ipBlacklistMu.Unlock()
	items, err := s.getIPBlacklistFromStore(ctx)
	if err != nil {
		return nil, err
	}
	filtered := make([]string, 0, len(items))
	for _, item := range items {
		if item != normalized[0] {
			filtered = append(filtered, item)
		}
	}
	return s.setIPBlacklistLocked(ctx, filtered)
}

// SetIPBlacklist persists and immediately activates the normalized rules.
func (s *SettingService) SetIPBlacklist(ctx context.Context, patterns []string) ([]string, error) {
	s.ipBlacklistMu.Lock()
	defer s.ipBlacklistMu.Unlock()
	return s.setIPBlacklistLocked(ctx, patterns)
}

func (s *SettingService) getIPBlacklistFromStore(ctx context.Context) ([]string, error) {
	if s == nil || s.settingRepo == nil {
		return []string{}, nil
	}
	raw, err := s.settingRepo.GetValue(ctx, SettingKeyIPBlacklist)
	if err != nil {
		if errors.Is(err, ErrSettingNotFound) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("get ip blacklist: %w", err)
	}
	patterns, err := parseIPBlacklistSetting(raw)
	if err != nil {
		return nil, err
	}
	return patterns, nil
}

func (s *SettingService) setIPBlacklistLocked(ctx context.Context, patterns []string) ([]string, error) {
	normalized, err := NormalizeIPBlacklist(patterns)
	if err != nil {
		return nil, infraerrors.BadRequest("INVALID_IP_BLACKLIST", err.Error())
	}
	payload, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("marshal ip blacklist: %w", err)
	}
	if err := s.settingRepo.Set(ctx, SettingKeyIPBlacklist, string(payload)); err != nil {
		return nil, err
	}
	s.storeIPBlacklist(normalized)
	if s.onUpdate != nil {
		s.onUpdate()
	}
	return normalized, nil
}

// IsIPBlacklisted checks a resolved client IP against the active site rules.
func (s *SettingService) IsIPBlacklisted(clientIP string) bool {
	if s == nil {
		return false
	}
	cached, _ := s.ipBlacklistCache.Load().(*cachedIPBlacklist)
	if cached == nil || cached.rules == nil || cached.rules.PatternCount == 0 || net.ParseIP(strings.TrimSpace(clientIP)) == nil {
		return false
	}
	allowed, _ := ip.CheckIPRestrictionWithCompiledRules(clientIP, nil, cached.rules)
	return !allowed
}

// SetDefaultSubscriptionGroupReader injects an optional group reader for default subscription validation.
func (s *SettingService) SetDefaultSubscriptionGroupReader(reader DefaultSubscriptionGroupReader) {
	s.defaultSubGroupReader = reader
}

// SetProxyRepository injects a proxy repo for resolving websearch provider proxy URLs.
func (s *SettingService) SetProxyRepository(repo ProxyRepository) {
	s.proxyRepo = repo
}

func (s *SettingService) LoadForwardedClientIPSettings(ctx context.Context) error {
	if s == nil || s.cfg == nil || s.settingRepo == nil {
		return nil
	}

	values, err := s.settingRepo.GetMultiple(ctx, []string{
		SettingKeyAPIKeyACLTrustForwardedIP,
		SettingKeyForwardedClientIPHeaders,
		settingKeyForwardedClientIPModeV2,
	})
	if err != nil {
		s.cfg.SetForwardedClientIPSettings(false, nil)
		return fmt.Errorf("get forwarded client ip settings: %w", err)
	}

	enabled := s.cfg.Security.TrustForwardedIPForAPIKeyACL
	headers := s.cfg.ForwardedClientIPSettings().Headers
	storedValue, hasStoredValue := values[SettingKeyAPIKeyACLTrustForwardedIP]
	if hasStoredValue {
		enabled = storedValue == "true"
	}

	var headersErr error
	if storedHeaders, ok := values[SettingKeyForwardedClientIPHeaders]; ok {
		headers, headersErr = parseForwardedClientIPHeadersSetting(storedHeaders)
		if headersErr != nil {
			enabled = false
			headers = []string{}
			headersErr = fmt.Errorf("load forwarded client ip headers: %w", headersErr)
		}
	}

	updates := make(map[string]string)
	if _, hasStoredHeaders := values[SettingKeyForwardedClientIPHeaders]; !hasStoredHeaders {
		headersJSON, marshalErr := json.Marshal(headers)
		if marshalErr != nil {
			headers = []string{}
			headersErr = errors.Join(headersErr, fmt.Errorf("marshal forwarded client ip headers: %w", marshalErr))
			headersJSON = []byte("[]")
		}
		updates[SettingKeyForwardedClientIPHeaders] = string(headersJSON)
	}
	if values[settingKeyForwardedClientIPModeV2] != "true" {
		updates[settingKeyForwardedClientIPModeV2] = "true"
		// Before this migration, new installations persisted false by default.
		// Restore compatibility only when no trusted-proxy policy was configured.
		if headersErr == nil && hasStoredValue && !enabled && !s.cfg.Server.TrustedProxiesConfigured {
			enabled = true
			updates[SettingKeyAPIKeyACLTrustForwardedIP] = "true"
		}
	}
	if len(updates) > 0 {
		if err := s.settingRepo.SetMultiple(ctx, updates); err != nil {
			s.cfg.SetForwardedClientIPSettings(enabled, headers)
			return errors.Join(headersErr, fmt.Errorf("migrate forwarded client ip setting: %w", err))
		}
	}

	s.cfg.SetForwardedClientIPSettings(enabled, headers)
	return headersErr
}

// GetAllSettings 获取所有系统设置
func (s *SettingService) GetAllSettings(ctx context.Context) (*SystemSettings, error) {
	settings, err := s.settingRepo.GetAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}

	return s.parseSettings(settings), nil
}

// SetOnUpdateCallback sets a callback function to be called when settings are updated
// This is used for cache invalidation (e.g., HTML cache in frontend server)
func (s *SettingService) SetOnUpdateCallback(callback func()) {
	s.onUpdate = callback
}

// SubscribeChannelMonitorRuntime registers a listener that is invoked after
// settings are successfully persisted (and process caches refreshed).
// Used by ChannelMonitorRunner / ChannelMonitorV2Aggregator for immediate
// mode flips without waiting for poll intervals.
func (s *SettingService) SubscribeChannelMonitorRuntime(listener func()) (unsubscribe func()) {
	if s == nil || listener == nil {
		return func() {}
	}
	s.channelMonitorRuntimeListenersMu.Lock()
	s.channelMonitorRuntimeListeners = append(s.channelMonitorRuntimeListeners, listener)
	idx := len(s.channelMonitorRuntimeListeners) - 1
	s.channelMonitorRuntimeListenersMu.Unlock()
	return func() {
		s.channelMonitorRuntimeListenersMu.Lock()
		defer s.channelMonitorRuntimeListenersMu.Unlock()
		if idx < 0 || idx >= len(s.channelMonitorRuntimeListeners) {
			return
		}
		s.channelMonitorRuntimeListeners[idx] = nil
	}
}

func (s *SettingService) notifyChannelMonitorRuntimeListeners() {
	if s == nil {
		return
	}
	s.channelMonitorRuntimeListenersMu.Lock()
	listeners := make([]func(), 0, len(s.channelMonitorRuntimeListeners))
	for _, l := range s.channelMonitorRuntimeListeners {
		if l != nil {
			listeners = append(listeners, l)
		}
	}
	s.channelMonitorRuntimeListenersMu.Unlock()
	for _, l := range listeners {
		func(fn func()) {
			defer func() {
				if recovered := recover(); recovered != nil {
					_ = recovered // keep settings path healthy
				}
			}()
			fn()
		}(l)
	}
}

// SetVersion sets the application version for injection into public settings
func (s *SettingService) SetVersion(version string) {
	s.version = version
}

// getStringOrDefault 获取字符串值或默认值
func (s *SettingService) getStringOrDefault(settings map[string]string, key, defaultValue string) string {
	if value, ok := settings[key]; ok && value != "" {
		return value
	}
	return defaultValue
}
