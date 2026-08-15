package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GitHubSecretSyncSpec defines the desired state of GitHubSecretSync.
// It mirrors the 1Password OnePasswordItem shape (vault + item) and adds
// GitHub organization sync targets for Actions repository secrets.
type GitHubSecretSyncSpec struct {
	// ItemPath identifies the 1Password item in Connect, using the same form
	// as the official 1Password operator:
	//   vaults/<vault_id_or_title>/items/<item_id_or_title>
	// +kubebuilder:validation:Pattern=`^vaults/[^/]+/items/[^/]+$`
	// +kubebuilder:validation:Required
	ItemPath string `json:"itemPath"`

	// GitHub configures where secrets are synchronized.
	// +kubebuilder:validation:Required
	GitHub GitHubTarget `json:"github"`

	// Fields optionally limits which 1Password field labels are synced.
	// When empty, every non-empty field on the item is synced.
	// Field labels become GitHub Actions secret names (sanitized to
	// [A-Z0-9_]), optionally prefixed with SecretNamePrefix.
	// +optional
	Fields []string `json:"fields,omitempty"`

	// SecretNamePrefix is prepended to each synced secret name.
	// Example: prefix "OP_" turns field "db_password" into "OP_DB_PASSWORD".
	// +optional
	SecretNamePrefix string `json:"secretNamePrefix,omitempty"`

	// SyncIntervalSeconds overrides the global polling interval for this
	// resource. When unset or zero, the operator default is used.
	// +optional
	// +kubebuilder:validation:Minimum=30
	SyncIntervalSeconds int64 `json:"syncIntervalSeconds,omitempty"`
}

// GitHubTarget describes which GitHub organization (and optionally which
// repositories) receive the synced Actions secrets.
type GitHubTarget struct {
	// Organization is the GitHub organization whose repositories receive secrets.
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:Required
	Organization string `json:"organization"`

	// Repositories limits sync to the given repository names.
	// When empty, every non-archived, non-fork repository in the organization
	// is synced (subject to ExcludeRepositories).
	// +optional
	Repositories []string `json:"repositories,omitempty"`

	// ExcludeRepositories lists repository names to skip when syncing the
	// whole organization.
	// +optional
	ExcludeRepositories []string `json:"excludeRepositories,omitempty"`

	// IncludeArchived includes archived repositories when listing the org.
	// Defaults to false.
	// +optional
	IncludeArchived bool `json:"includeArchived,omitempty"`

	// IncludeForks includes forked repositories when listing the org.
	// Defaults to false.
	// +optional
	IncludeForks bool `json:"includeForks,omitempty"`
}

// GitHubSecretSyncStatus defines the observed state of GitHubSecretSync.
type GitHubSecretSyncStatus struct {
	// Phase is a high-level summary: Pending, Syncing, Synced, or Error.
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message is a human-readable status detail.
	// +optional
	Message string `json:"message,omitempty"`

	// ItemVersion is the 1Password item version last successfully synced.
	// Used to avoid rewriting GitHub secrets when nothing changed.
	// +optional
	ItemVersion int `json:"itemVersion,omitempty"`

	// LastSyncedFields lists the GitHub secret names written in the last sync.
	// +optional
	LastSyncedFields []string `json:"lastSyncedFields,omitempty"`

	// SyncedRepositories is the number of repositories updated in the last sync.
	// +optional
	SyncedRepositories int `json:"syncedRepositories,omitempty"`

	// LastSyncTime is when the last successful sync completed.
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`

	// Conditions represent the latest available observations.
	// +optional
	// +listType=map
	// +listMapKey=type
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

const (
	PhasePending = "Pending"
	PhaseSyncing = "Syncing"
	PhaseSynced  = "Synced"
	PhaseError   = "Error"

	ConditionReady = "Ready"
	ConditionSynced = "Synced"
)

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ghss
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Vault/Item",type=string,JSONPath=`.spec.itemPath`
// +kubebuilder:printcolumn:name="Org",type=string,JSONPath=`.spec.github.organization`
// +kubebuilder:printcolumn:name="Repos",type=integer,JSONPath=`.status.syncedRepositories`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GitHubSecretSync declares a 1Password item whose fields should be synced
// into GitHub Actions repository secrets for an organization.
type GitHubSecretSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitHubSecretSyncSpec   `json:"spec,omitempty"`
	Status GitHubSecretSyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GitHubSecretSyncList contains a list of GitHubSecretSync.
type GitHubSecretSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitHubSecretSync `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitHubSecretSync{}, &GitHubSecretSyncList{})
}
