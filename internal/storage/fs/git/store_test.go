package git

import (
	"bytes"
	"context"
	"encoding/pem"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/require"
	"go.flipt.io/flipt/internal/containers"
	"go.flipt.io/flipt/internal/storage"
	"go.flipt.io/flipt/internal/storage/fs"
	"go.uber.org/zap/zaptest"
)

var gitRepoURL = os.Getenv("TEST_GIT_REPO_URL")

func Test_Store_String(t *testing.T) {
	require.Equal(t, "git", (&SnapshotStore{}).String())
}

func Test_Store_Subscribe_Hash(t *testing.T) {
	head := os.Getenv("TEST_GIT_REPO_HEAD")
	if head == "" {
		t.Skip("Set non-empty TEST_GIT_REPO_HEAD env var to run this test.")
		return
	}

	// this helper will fail if there is a problem with this option
	// the only difference in behaviour is that the poll loop
	// will silently (intentionally) not run
	testStore(t, WithRef(head))
}

func Test_Store_Subscribe(t *testing.T) {
	ch := make(chan struct{})
	store, skip := testStore(t, WithPollOptions(
		fs.WithInterval(time.Second),
		fs.WithNotify(t, func(modified bool) {
			if modified {
				close(ch)
			}
		}),
	))
	if skip {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// pull repo
	workdir := memfs.New()
	repo, err := git.Clone(memory.NewStorage(), workdir, &git.CloneOptions{
		Auth:          &http.BasicAuth{Username: "root", Password: "password"},
		URL:           gitRepoURL,
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName("main"),
	})
	require.NoError(t, err)

	tree, err := repo.Worktree()
	require.NoError(t, err)

	require.NoError(t, tree.Checkout(&git.CheckoutOptions{
		Branch: "refs/heads/main",
	}))

	// update features.yml
	fi, err := workdir.OpenFile("features.yml", os.O_TRUNC|os.O_RDWR, os.ModePerm)
	require.NoError(t, err)

	updated := []byte(`namespace: production
flags:
    - key: foo
      name: Foo`)

	_, err = fi.Write(updated)
	require.NoError(t, err)
	require.NoError(t, fi.Close())

	// commit changes
	_, err = tree.Commit("chore: update features.yml", &git.CommitOptions{
		All:    true,
		Author: &object.Signature{Email: "dev@flipt.io", Name: "dev"},
	})
	require.NoError(t, err)

	// push new commit
	require.NoError(t, repo.Push(&git.PushOptions{
		Auth:       &http.BasicAuth{Username: "root", Password: "password"},
		RemoteName: "origin",
	}))

	// wait until the snapshot is updated or
	// we timeout
	select {
	case <-ch:
	case <-time.After(time.Minute):
		t.Fatal("timed out waiting for snapshot")
	}

	require.NoError(t, err)

	t.Log("received new snapshot")

	require.NoError(t, store.View(func(s storage.ReadOnlyStore) error {
		_, err = s.GetFlag(ctx, "production", "foo")
		return err
	}))
}

func Test_Store_SelfSignedSkipTLS(t *testing.T) {
	ts := httptest.NewTLSServer(nil)
	defer ts.Close()
	// This is not a valid Git source, but it still proves the point that a
	// well-known server with a self-signed certificate will be accepted by Flipt
	// when configuring the TLS options for the source
	gitRepoURL = ts.URL
	err := testStoreWithError(t, WithInsecureTLS(false))
	require.ErrorContains(t, err, "tls: failed to verify certificate: x509: certificate signed by unknown authority")
	err = testStoreWithError(t, WithInsecureTLS(true))
	// This time, we don't expect a tls validation error anymore
	require.ErrorIs(t, err, transport.ErrRepositoryNotFound)
}

func Test_Store_SelfSignedCABytes(t *testing.T) {
	ts := httptest.NewTLSServer(nil)
	defer ts.Close()
	var buf bytes.Buffer
	pemCert := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ts.Certificate().Raw,
	}
	err := pem.Encode(&buf, pemCert)
	require.NoError(t, err)

	// This is not a valid Git source, but it still proves the point that a
	// well-known server with a self-signed certificate will be accepted by Flipt
	// when configuring the TLS options for the source
	gitRepoURL = ts.URL
	err = testStoreWithError(t)
	require.ErrorContains(t, err, "tls: failed to verify certificate: x509: certificate signed by unknown authority")
	err = testStoreWithError(t, WithCABundle(buf.Bytes()))
	// This time, we don't expect a tls validation error anymore
	require.ErrorIs(t, err, transport.ErrRepositoryNotFound)
}

func testStore(t *testing.T, opts ...containers.Option[SnapshotStore]) (*SnapshotStore, bool) {
	t.Helper()

	if gitRepoURL == "" {
		t.Skip("Set non-empty TEST_GIT_REPO_URL env var to run this test.")
		return nil, true
	}

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	source, err := NewSnapshotStore(ctx, zaptest.NewLogger(t), gitRepoURL,
		append([]containers.Option[SnapshotStore]{
			WithRef("main"),
			WithAuth(&http.BasicAuth{
				Username: "root",
				Password: "password",
			}),
		},
			opts...)...,
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = source.Close()
	})

	return source, false
}

func testStoreWithError(t *testing.T, opts ...containers.Option[SnapshotStore]) error {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	source, err := NewSnapshotStore(ctx, zaptest.NewLogger(t), gitRepoURL,
		append([]containers.Option[SnapshotStore]{
			WithRef("main"),
			WithAuth(&http.BasicAuth{
				Username: "root",
				Password: "password",
			}),
		},
			opts...)...,
	)
	if err != nil {
		return err
	}

	t.Cleanup(func() {
		_ = source.Close()
	})

	return nil
}
