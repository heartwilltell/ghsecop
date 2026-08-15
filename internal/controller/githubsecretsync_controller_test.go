package controller_test

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/1Password/connect-sdk-go/onepassword"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ghsecopv1 "github.com/heartwilltell/ghsecop/api/v1"
	"github.com/heartwilltell/ghsecop/internal/controller"
	ghsec "github.com/heartwilltell/ghsecop/internal/github"
)

type fakeConnect struct {
	item *onepassword.Item
	err  error
}

func (f *fakeConnect) GetItem(vault, item string) (*onepassword.Item, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.item, nil
}

type fakeGitHub struct {
	repos   []ghsec.RepoRef
	listErr error
	writes  map[string]map[string]string
	upsertE error
}

func (f *fakeGitHub) ListTargetRepos(ctx context.Context, opts ghsec.ListOptions) ([]ghsec.RepoRef, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.repos, nil
}

func (f *fakeGitHub) UpsertRepoSecrets(ctx context.Context, repo ghsec.RepoRef, secrets map[string]string) error {
	if f.upsertE != nil {
		return f.upsertE
	}
	if f.writes == nil {
		f.writes = map[string]map[string]string{}
	}
	copied := map[string]string{}
	for k, v := range secrets {
		copied[k] = v
	}
	f.writes[repo.Owner+"/"+repo.Name] = copied
	return nil
}

func TestReconcileSyncsAllFieldsToAllRepos(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ghsecopv1.AddToScheme(scheme)

	cr := &ghsecopv1.GitHubSecretSync{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "secure-node",
			Namespace:  "default",
			Finalizers: []string{"ghsecop.io/finalizer"},
		},
		Spec: ghsecopv1.GitHubSecretSyncSpec{
			ItemPath: "vaults/Infra/items/secure-node",
			GitHub: ghsecopv1.GitHubTarget{
				Organization: "acme",
			},
			SecretNamePrefix: "OP",
		},
	}

	k8s := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cr).WithObjects(cr).Build()
	gh := &fakeGitHub{
		repos: []ghsec.RepoRef{
			{Owner: "acme", Name: "api"},
			{Owner: "acme", Name: "web"},
		},
	}

	r := &controller.GitHubSecretSyncReconciler{
		Client:  k8s,
		Scheme:  scheme,
		Connect: &fakeConnect{item: &onepassword.Item{
			ID:      "item-1",
			Title:   "secure-node",
			Version: 3,
			Fields: []*onepassword.ItemField{
				{Label: "username", Value: "root"},
				{Label: "password", Value: "hunter2"},
			},
		}},
		GitHub:              gh,
		DefaultSyncInterval: time.Minute,
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "secure-node", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	if len(gh.writes) != 2 {
		t.Fatalf("expected writes to 2 repos, got %d", len(gh.writes))
	}
	for _, repo := range []string{"acme/api", "acme/web"} {
		secrets := gh.writes[repo]
		if secrets["OP_USERNAME"] != "root" || secrets["OP_PASSWORD"] != "hunter2" {
			t.Fatalf("unexpected secrets for %s: %#v", repo, secrets)
		}
	}

	var updated ghsecopv1.GitHubSecretSync
	if err := k8s.Get(context.Background(), types.NamespacedName{Name: "secure-node", Namespace: "default"}, &updated); err != nil {
		t.Fatalf("get status: %v", err)
	}
	if updated.Status.Phase != ghsecopv1.PhaseSynced {
		t.Fatalf("phase = %s, want Synced (msg=%s)", updated.Status.Phase, updated.Status.Message)
	}
	if updated.Status.SyncedRepositories != 2 {
		t.Fatalf("synced repos = %d", updated.Status.SyncedRepositories)
	}
	sort.Strings(updated.Status.LastSyncedFields)
	wantFields := []string{"OP_PASSWORD", "OP_USERNAME"}
	if fmt.Sprint(updated.Status.LastSyncedFields) != fmt.Sprint(wantFields) {
		t.Fatalf("fields = %v, want %v", updated.Status.LastSyncedFields, wantFields)
	}
}

func TestReconcileConnectError(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = ghsecopv1.AddToScheme(scheme)

	cr := &ghsecopv1.GitHubSecretSync{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "broken",
			Namespace:  "default",
			Finalizers: []string{"ghsecop.io/finalizer"},
		},
		Spec: ghsecopv1.GitHubSecretSyncSpec{
			ItemPath: "vaults/Infra/items/missing",
			GitHub:   ghsecopv1.GitHubTarget{Organization: "acme"},
		},
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(cr).WithObjects(cr).Build()

	r := &controller.GitHubSecretSyncReconciler{
		Client:              k8s,
		Scheme:              scheme,
		Connect:             &fakeConnect{err: fmt.Errorf("not found")},
		GitHub:              &fakeGitHub{},
		DefaultSyncInterval: time.Minute,
	}

	_, err := r.Reconcile(context.Background(), reconcile.Request{
		NamespacedName: types.NamespacedName{Name: "broken", Namespace: "default"},
	})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	var updated ghsecopv1.GitHubSecretSync
	_ = k8s.Get(context.Background(), types.NamespacedName{Name: "broken", Namespace: "default"}, &updated)
	if updated.Status.Phase != ghsecopv1.PhaseError {
		t.Fatalf("phase = %s, want Error", updated.Status.Phase)
	}
}
