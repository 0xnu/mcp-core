package auth

import (
	"fmt"
	"slices"
	"sync"
)

type AccessPolicy string

const (
	PolicyAllow      AccessPolicy = "allow"
	PolicyDeny       AccessPolicy = "deny"
	PolicyRestricted AccessPolicy = "restricted"
)

type PolicyEngine struct {
	mu            sync.RWMutex
	policies      map[string]AccessPolicy
	defaultPolicy AccessPolicy
}

func NewPolicyEngine() *PolicyEngine {
	return &PolicyEngine{
		policies:      make(map[string]AccessPolicy),
		defaultPolicy: PolicyAllow,
	}
}

func (pe *PolicyEngine) SetDefault(policy AccessPolicy) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.defaultPolicy = policy
}

func (pe *PolicyEngine) SetPolicy(backend string, policy AccessPolicy) {
	pe.mu.Lock()
	defer pe.mu.Unlock()
	pe.policies[backend] = policy
}

func (pe *PolicyEngine) Allow(backend string, token string, tool string) error {
	pe.mu.RLock()
	policy, exists := pe.policies[backend]
	if !exists {
		policy = pe.defaultPolicy
	}
	pe.mu.RUnlock()

	switch policy {
	case PolicyDeny:
		return fmt.Errorf("access denied to backend: %s", backend)
	case PolicyRestricted:
		return pe.evaluateRestricted(backend, token, tool)
	case PolicyAllow:
		return nil
	default:
		return nil
	}
}

func (pe *PolicyEngine) evaluateRestricted(_, token, tool string) error {
	return nil
}

type ACLRule struct {
	Backend string   `yaml:"backend"`
	Tools   []string `yaml:"tools,omitempty"`
	Allow   bool     `yaml:"allow"`
}

type ACLConfig struct {
	rules []ACLRule
	mu    sync.RWMutex
}

func NewACLConfig() *ACLConfig {
	return &ACLConfig{}
}

func (acl *ACLConfig) AddRule(rule ACLRule) {
	acl.mu.Lock()
	defer acl.mu.Unlock()
	acl.rules = append(acl.rules, rule)
}

func (acl *ACLConfig) Check(backend, tool string) bool {
	acl.mu.RLock()
	defer acl.mu.RUnlock()

	for _, rule := range acl.rules {
		if rule.Backend != backend {
			continue
		}
		if len(rule.Tools) == 0 {
			return rule.Allow
		}
		if slices.Contains(rule.Tools, tool) {
			return rule.Allow
		}
	}

	return true
}
