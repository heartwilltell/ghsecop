package controller

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/1Password/connect-sdk-go/onepassword"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	ghsecopv1 "github.com/heartwilltell/ghsecop/api/v1"
	opconnect "github.com/heartwilltell/ghsecop/internal/connect"
	ghsec "github.com/heartwilltell/ghsecop/internal/github"
)

const finalizerName = "ghsecop.io/finalizer"

// GitHubSecretSyncReconciler reconciles GitHubSecretSync objects.
type GitHubSecretSyncReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	Connect opconnect.Client
	GitHub  ghsec.SecretSyncer

	// DefaultSyncInterval is used when the CR does not set syncIntervalSeconds.
	DefaultSyncInterval time.Duration
}

// +kubebuilder:rbac:groups=ghsecop.io,resources=githubsecretsyncs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ghsecop.io,resources=githubsecretsyncs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ghsecop.io,resources=githubsecretsyncs/finalizers,verbs=update

// Reconcile fetches the referenced 1Password item via Connect and syncs every
// selected field to GitHub Actions secrets on the target organization repos.
//
// Connect does not push change events, so the controller requeues on a polling
// interval (same model as the official 1Password Kubernetes operator).
func (r *GitHubSecretSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	var sync ghsecopv1.GitHubSecretSync
	if err := r.Get(ctx, req.NamespacedName, &sync); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	if !sync.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(&sync, finalizerName) {
			controllerutil.RemoveFinalizer(&sync, finalizerName)
			if err := r.Update(ctx, &sync); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}

	if !controllerutil.ContainsFinalizer(&sync, finalizerName) {
		controllerutil.AddFinalizer(&sync, finalizerName)
		if err := r.Update(ctx, &sync); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	interval := r.pollInterval(&sync)

	parsed, err := opconnect.ParseItemPath(sync.Spec.ItemPath)
	if err != nil {
		_ = r.setError(ctx, &sync, err)
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	item, err := r.Connect.GetItem(parsed.Vault, parsed.Item)
	if err != nil {
		_ = r.setError(ctx, &sync, fmt.Errorf("get 1Password item %q: %w", sync.Spec.ItemPath, err))
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	fields := opconnect.FieldMap(item, sync.Spec.Fields)
	if len(fields) == 0 {
		_ = r.setError(ctx, &sync, fmt.Errorf("no syncable fields found on item %q", sync.Spec.ItemPath))
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	secrets := ghsec.BuildSecretMap(fields, sync.Spec.SecretNamePrefix)
	syncedNames := secretNames(secrets)

	if sync.Status.Phase == ghsecopv1.PhaseSynced &&
		sync.Status.ItemVersion == item.Version &&
		sameStringSet(sync.Status.LastSyncedFields, syncedNames) {
		log.V(1).Info("item unchanged; requeue for next poll", "version", item.Version)
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	repos, err := r.GitHub.ListTargetRepos(ctx, ghsec.ListOptions{
		Organization:        sync.Spec.GitHub.Organization,
		Repositories:        sync.Spec.GitHub.Repositories,
		ExcludeRepositories: sync.Spec.GitHub.ExcludeRepositories,
		IncludeArchived:     sync.Spec.GitHub.IncludeArchived,
		IncludeForks:        sync.Spec.GitHub.IncludeForks,
	})
	if err != nil {
		_ = r.setError(ctx, &sync, fmt.Errorf("list GitHub repositories: %w", err))
		return ctrl.Result{RequeueAfter: interval}, nil
	}
	if len(repos) == 0 {
		_ = r.setError(ctx, &sync, fmt.Errorf("no target repositories found in organization %q", sync.Spec.GitHub.Organization))
		return ctrl.Result{RequeueAfter: interval}, nil
	}

	for _, repo := range repos {
		log.Info("syncing secrets", "repo", repo.Owner+"/"+repo.Name, "fields", syncedNames)
		if err := r.GitHub.UpsertRepoSecrets(ctx, repo, secrets); err != nil {
			_ = r.setError(ctx, &sync, err)
			return ctrl.Result{RequeueAfter: interval}, nil
		}
	}

	now := metav1.Now()
	sync.Status.Phase = ghsecopv1.PhaseSynced
	sync.Status.Message = fmt.Sprintf("Synced %d fields to %d repositories", len(secrets), len(repos))
	sync.Status.ItemVersion = item.Version
	sync.Status.LastSyncedFields = syncedNames
	sync.Status.SyncedRepositories = len(repos)
	sync.Status.LastSyncTime = &now
	meta.SetStatusCondition(&sync.Status.Conditions, metav1.Condition{
		Type:               ghsecopv1.ConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Synced",
		Message:            sync.Status.Message,
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&sync.Status.Conditions, metav1.Condition{
		Type:               ghsecopv1.ConditionSynced,
		Status:             metav1.ConditionTrue,
		Reason:             "Synced",
		Message:            sync.Status.Message,
		LastTransitionTime: now,
	})
	if err := r.Status().Update(ctx, &sync); err != nil {
		return ctrl.Result{}, err
	}

	log.Info("sync complete",
		"item", itemTitle(item),
		"version", item.Version,
		"repos", len(repos),
		"fields", len(secrets),
	)
	return ctrl.Result{RequeueAfter: interval}, nil
}

func (r *GitHubSecretSyncReconciler) pollInterval(sync *ghsecopv1.GitHubSecretSync) time.Duration {
	interval := r.DefaultSyncInterval
	if sync.Spec.SyncIntervalSeconds > 0 {
		interval = time.Duration(sync.Spec.SyncIntervalSeconds) * time.Second
	}
	if interval < 30*time.Second {
		interval = 30 * time.Second
	}
	return interval
}

func (r *GitHubSecretSyncReconciler) setError(ctx context.Context, sync *ghsecopv1.GitHubSecretSync, err error) error {
	now := metav1.Now()
	sync.Status.Phase = ghsecopv1.PhaseError
	sync.Status.Message = err.Error()
	meta.SetStatusCondition(&sync.Status.Conditions, metav1.Condition{
		Type:               ghsecopv1.ConditionReady,
		Status:             metav1.ConditionFalse,
		Reason:             "SyncFailed",
		Message:            err.Error(),
		LastTransitionTime: now,
	})
	meta.SetStatusCondition(&sync.Status.Conditions, metav1.Condition{
		Type:               ghsecopv1.ConditionSynced,
		Status:             metav1.ConditionFalse,
		Reason:             "SyncFailed",
		Message:            err.Error(),
		LastTransitionTime: now,
	})
	return r.Status().Update(ctx, sync)
}

func secretNames(secrets map[string]string) []string {
	names := make([]string, 0, len(secrets))
	for name := range secrets {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func sameStringSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aa := append([]string(nil), a...)
	bb := append([]string(nil), b...)
	sort.Strings(aa)
	sort.Strings(bb)
	for i := range aa {
		if aa[i] != bb[i] {
			return false
		}
	}
	return true
}

func itemTitle(item *onepassword.Item) string {
	if item == nil {
		return ""
	}
	if item.Title != "" {
		return item.Title
	}
	return item.ID
}

// SetupWithManager registers the controller with the manager.
func (r *GitHubSecretSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&ghsecopv1.GitHubSecretSync{}).
		Complete(r)
}
