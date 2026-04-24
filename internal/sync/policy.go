package sync

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/bkarlovitz/gandt/internal/cache"
	"github.com/bkarlovitz/gandt/internal/config"
	"github.com/jmoiron/sqlx"
)

type PolicySource string

const (
	PolicySourceDBExplicit     PolicySource = "db_explicit"
	PolicySourceConfig         PolicySource = "config"
	PolicySourceAccountDefault PolicySource = "account_default"
	PolicySourceGlobalDefault  PolicySource = "global_default"
)

type MessageContext struct {
	AccountID    string
	AccountEmail string
	From         string
	LabelIDs     []string
}

type LabelPolicy struct {
	AccountID       string
	LabelID         string
	Include         bool
	Depth           config.CacheDepth
	RetentionDays   *int
	AttachmentRule  config.AttachmentRule
	AttachmentMaxMB *int
	Source          PolicySource
}

type CacheDecision struct {
	AccountID       string
	LabelIDs        []string
	Include         bool
	Persist         bool
	Excluded        bool
	Exclusion       string
	Depth           config.CacheDepth
	RetentionDays   *int
	AttachmentRule  config.AttachmentRule
	AttachmentMaxMB *int
	Policies        []LabelPolicy
}

type PolicyEvaluator struct {
	config     config.Config
	policies   cache.SyncPolicyRepository
	exclusions cache.CacheExclusionRepository
}

func NewPolicyEvaluator(db *sqlx.DB, cfg config.Config) PolicyEvaluator {
	return PolicyEvaluator{
		config:     cfg,
		policies:   cache.NewSyncPolicyRepository(db),
		exclusions: cache.NewCacheExclusionRepository(db),
	}
}

func (e PolicyEvaluator) Evaluate(ctx context.Context, message MessageContext) (CacheDecision, error) {
	labels := uniqueNonEmpty(message.LabelIDs)
	decision := CacheDecision{
		AccountID:       message.AccountID,
		LabelIDs:        labels,
		Depth:           config.CacheDepthNone,
		AttachmentRule:  config.AttachmentRuleNone,
		AttachmentMaxMB: nil,
	}

	if reason, excluded, err := e.exclusionMatch(ctx, message); err != nil {
		return CacheDecision{}, err
	} else if excluded {
		decision.Excluded = true
		decision.Exclusion = reason
		return decision, nil
	}

	for _, labelID := range labels {
		policy, err := e.EffectiveForLabel(ctx, message.AccountID, message.AccountEmail, labelID)
		if err != nil {
			return CacheDecision{}, err
		}
		decision.Include = decision.Include || policy.Include
		decision.Depth = maxDepth(decision.Depth, policyDepth(policy))
		decision.AttachmentRule = maxAttachmentRule(decision.AttachmentRule, policy.AttachmentRule)
		if len(decision.Policies) == 0 {
			decision.RetentionDays = cloneInt(policy.RetentionDays)
			decision.AttachmentMaxMB = cloneInt(policy.AttachmentMaxMB)
		} else {
			decision.RetentionDays = longestRetention(decision.RetentionDays, policy.RetentionDays)
			decision.AttachmentMaxMB = maxOptionalInt(decision.AttachmentMaxMB, policy.AttachmentMaxMB)
		}
		decision.Policies = append(decision.Policies, policy)
	}

	if len(labels) == 0 {
		policy, err := e.EffectiveForLabel(ctx, message.AccountID, message.AccountEmail, cache.DefaultPolicyLabelID)
		if err != nil {
			return CacheDecision{}, err
		}
		decision.Policies = append(decision.Policies, policy)
		decision.Include = policy.Include
		decision.Depth = policyDepth(policy)
		decision.RetentionDays = policy.RetentionDays
		decision.AttachmentRule = policy.AttachmentRule
		decision.AttachmentMaxMB = policy.AttachmentMaxMB
	}

	decision.Persist = decision.Include && decision.Depth != config.CacheDepthNone
	return decision, nil
}

func (e PolicyEvaluator) EffectiveForLabel(ctx context.Context, accountID string, accountEmail string, labelID string) (LabelPolicy, error) {
	if policy, err := e.policies.Get(ctx, accountID, labelID); err == nil {
		return labelPolicyFromDB(policy, PolicySourceDBExplicit), nil
	} else if !errors.Is(err, cache.ErrSyncPolicyNotFound) {
		return LabelPolicy{}, err
	}

	if policy, ok := e.configPolicy(accountID, accountEmail, labelID); ok {
		return policy, nil
	}

	if labelID != cache.DefaultPolicyLabelID {
		if policy, err := e.policies.Get(ctx, accountID, cache.DefaultPolicyLabelID); err == nil {
			return labelPolicyFromDB(policy, PolicySourceAccountDefault), nil
		} else if !errors.Is(err, cache.ErrSyncPolicyNotFound) {
			return LabelPolicy{}, err
		}
	}

	return labelPolicyFromDefaults(accountID, labelID, e.config.Cache.Defaults), nil
}

func (e PolicyEvaluator) configPolicy(accountID string, accountEmail string, labelID string) (LabelPolicy, bool) {
	for i := len(e.config.Cache.Policies) - 1; i >= 0; i-- {
		policy := e.config.Cache.Policies[i]
		if !e.accountMatches(policy.Account, accountID, accountEmail) {
			continue
		}
		if policy.Label != labelID {
			continue
		}
		retention := optionalPositiveInt(policy.RetentionDays)
		attachmentMax := optionalPositiveInt(policy.AttachmentMaxMB)
		return LabelPolicy{
			AccountID:       accountID,
			LabelID:         labelID,
			Include:         policy.Depth != config.CacheDepthNone,
			Depth:           policy.Depth,
			RetentionDays:   retention,
			AttachmentRule:  policy.AttachmentRule,
			AttachmentMaxMB: attachmentMax,
			Source:          PolicySourceConfig,
		}, true
	}
	return LabelPolicy{}, false
}

func (e PolicyEvaluator) exclusionMatch(ctx context.Context, message MessageContext) (string, bool, error) {
	exclusions, err := e.exclusions.List(ctx, message.AccountID)
	if err != nil {
		return "", false, err
	}
	for _, exclusion := range exclusions {
		if exclusionApplies(exclusion.MatchType, exclusion.MatchValue, message) {
			return exclusion.MatchType + ":" + exclusion.MatchValue, true, nil
		}
	}
	for _, exclusion := range e.config.Cache.Exclusions {
		if !e.accountMatches(exclusion.Account, message.AccountID, message.AccountEmail) {
			continue
		}
		if exclusionApplies(exclusion.MatchType, exclusion.MatchValue, message) {
			return strings.ToLower(exclusion.MatchType) + ":" + exclusion.MatchValue, true, nil
		}
	}
	return "", false, nil
}

func labelPolicyFromDB(policy cache.SyncPolicy, source PolicySource) LabelPolicy {
	return LabelPolicy{
		AccountID:       policy.AccountID,
		LabelID:         policy.LabelID,
		Include:         policy.Include,
		Depth:           config.CacheDepth(policy.Depth),
		RetentionDays:   cloneInt(policy.RetentionDays),
		AttachmentRule:  config.AttachmentRule(policy.AttachmentRule),
		AttachmentMaxMB: cloneInt(policy.AttachmentMaxMB),
		Source:          source,
	}
}

func labelPolicyFromDefaults(accountID string, labelID string, defaults config.CacheDefaults) LabelPolicy {
	return LabelPolicy{
		AccountID:       accountID,
		LabelID:         labelID,
		Include:         defaults.Depth != config.CacheDepthNone,
		Depth:           defaults.Depth,
		RetentionDays:   optionalPositiveInt(defaults.RetentionDays),
		AttachmentRule:  defaults.AttachmentRule,
		AttachmentMaxMB: optionalPositiveInt(defaults.AttachmentMaxMB),
		Source:          PolicySourceGlobalDefault,
	}
}

func policyDepth(policy LabelPolicy) config.CacheDepth {
	if !policy.Include {
		return config.CacheDepthNone
	}
	return policy.Depth
}

func uniqueNonEmpty(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func (e PolicyEvaluator) accountMatches(policyAccount string, accountID string, accountEmail string) bool {
	policyAccount = strings.TrimSpace(policyAccount)
	if policyAccount == "" {
		return true
	}
	if account, ok := e.config.Accounts[policyAccount]; ok {
		if strings.TrimSpace(account.ID) != "" && account.ID == accountID {
			return true
		}
		if strings.TrimSpace(account.Email) != "" && strings.EqualFold(account.Email, accountEmail) {
			return true
		}
	}
	return policyAccount == accountID || strings.EqualFold(policyAccount, accountEmail)
}

func exclusionApplies(matchType string, matchValue string, message MessageContext) bool {
	matchType = strings.ToLower(strings.TrimSpace(matchType))
	matchValue = strings.ToLower(strings.TrimSpace(matchValue))
	switch matchType {
	case "sender":
		return strings.EqualFold(senderAddress(message.From), matchValue) || strings.EqualFold(strings.TrimSpace(message.From), matchValue)
	case "domain":
		return strings.EqualFold(senderDomain(message.From), matchValue)
	case "label":
		for _, labelID := range message.LabelIDs {
			if strings.EqualFold(labelID, matchValue) {
				return true
			}
		}
	}
	return false
}

func senderAddress(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	address, err := mail.ParseAddress(value)
	if err == nil {
		return strings.ToLower(address.Address)
	}
	return strings.ToLower(value)
}

func senderDomain(value string) string {
	address := senderAddress(value)
	_, domain, found := strings.Cut(address, "@")
	if !found {
		return ""
	}
	return strings.ToLower(domain)
}

func maxDepth(a config.CacheDepth, b config.CacheDepth) config.CacheDepth {
	if depthRank(b) > depthRank(a) {
		return b
	}
	return a
}

func depthRank(depth config.CacheDepth) int {
	switch depth {
	case config.CacheDepthFull:
		return 3
	case config.CacheDepthBody:
		return 2
	case config.CacheDepthMetadata:
		return 1
	default:
		return 0
	}
}

func longestRetention(current *int, next *int) *int {
	if current == nil || next == nil {
		return nil
	}
	if *next > *current {
		return cloneInt(next)
	}
	return cloneInt(current)
}

func maxAttachmentRule(a config.AttachmentRule, b config.AttachmentRule) config.AttachmentRule {
	if attachmentRank(b) > attachmentRank(a) {
		return b
	}
	return a
}

func attachmentRank(rule config.AttachmentRule) int {
	switch rule {
	case config.AttachmentRuleAll:
		return 2
	case config.AttachmentRuleUnderSize:
		return 1
	default:
		return 0
	}
}

func maxOptionalInt(a *int, b *int) *int {
	if a == nil {
		return cloneInt(b)
	}
	if b == nil {
		return cloneInt(a)
	}
	if *b > *a {
		return cloneInt(b)
	}
	return cloneInt(a)
}

func optionalPositiveInt(value int) *int {
	if value <= 0 {
		return nil
	}
	return &value
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	out := *value
	return &out
}
