// server_test.go tests the gRPC [Server] handler by wiring it to an in-memory
// [fakeGitManager] via a real gRPC connection (bufconn).  Each test covers a
// representative RPC — verifying that domain types are mapped to proto types
// correctly and that domain errors are translated to the expected gRPC status
// codes.
package server_test

import (
	"context"
	"errors"
	"net"
	"testing"

	codevaldpubsub "github.com/aosanya/CodeValdGit"
	pb "github.com/aosanya/CodeValdGit/gen/go/codevaldpubsub/v1"
	"github.com/aosanya/CodeValdGit/internal/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

// ── fakeGitManager ────────────────────────────────────────────────────────────

// fakeGitManager is a configurable stub that satisfies [codevaldpubsub.GitManager].
// Each field is a function so individual tests can inject specific behaviour
// (errors, return values) without resetting global state.
type fakeGitManager struct {
	initRepo            func(ctx context.Context, req codevaldpubsub.CreateRepoRequest) (codevaldpubsub.Repository, error)
	listRepositories    func(ctx context.Context) ([]codevaldpubsub.Repository, error)
	getRepository       func(ctx context.Context, repoID string) (codevaldpubsub.Repository, error)
	getRepositoryByName func(ctx context.Context, repoName string) (codevaldpubsub.Repository, error)
	deleteRepo          func(ctx context.Context, repoID string) error
	purgeRepo           func(ctx context.Context, repoID string) error
	createTag           func(ctx context.Context, req codevaldpubsub.CreateTagRequest) (codevaldpubsub.Tag, error)
	createBranch        func(ctx context.Context, req codevaldpubsub.CreateBranchRequest) (codevaldpubsub.Branch, error)
	getBranch           func(ctx context.Context, branchID string) (codevaldpubsub.Branch, error)
	listBranches        func(ctx context.Context, repoID string) ([]codevaldpubsub.Branch, error)
	deleteBranch        func(ctx context.Context, branchID string) error
	mergeBranch         func(ctx context.Context, branchID string) (codevaldpubsub.Branch, error)
	getTag              func(ctx context.Context, tagID string) (codevaldpubsub.Tag, error)
	listTags            func(ctx context.Context, repoID string) ([]codevaldpubsub.Tag, error)
	deleteTag           func(ctx context.Context, tagID string) error
	writeFile           func(ctx context.Context, req codevaldpubsub.WriteFileRequest) (codevaldpubsub.Commit, error)
	readFile            func(ctx context.Context, branchID, path string) (codevaldpubsub.Blob, error)
	deleteFile          func(ctx context.Context, req codevaldpubsub.DeleteFileRequest) (codevaldpubsub.Commit, error)
	listDirectory       func(ctx context.Context, branchID, path string) ([]codevaldpubsub.FileEntry, error)
	log                 func(ctx context.Context, branchID string, filter codevaldpubsub.LogFilter) ([]codevaldpubsub.CommitEntry, error)
	diffFunc            func(ctx context.Context, fromRef, toRef string) ([]codevaldpubsub.FileDiff, error)
	createKeyword       func(ctx context.Context, req codevaldpubsub.CreateKeywordRequest) (codevaldpubsub.Keyword, error)
	getKeyword          func(ctx context.Context, kwID string) (codevaldpubsub.Keyword, error)
	listKeywords        func(ctx context.Context, filter codevaldpubsub.KeywordFilter) ([]codevaldpubsub.Keyword, error)
	getKeywordTree      func(ctx context.Context, kwID string) ([]codevaldpubsub.KeywordTreeNode, error)
	updateKeyword       func(ctx context.Context, kwID string, req codevaldpubsub.UpdateKeywordRequest) (codevaldpubsub.Keyword, error)
	deleteKeyword       func(ctx context.Context, kwID string) error
	createEdge          func(ctx context.Context, req codevaldpubsub.CreateEdgeRequest) error
	deleteEdge          func(ctx context.Context, req codevaldpubsub.DeleteEdgeRequest) error
	getNeighborhood     func(ctx context.Context, branchID, entityID string, depth int) (codevaldpubsub.GraphResult, error)
	searchByKeywords    func(ctx context.Context, req codevaldpubsub.SearchByKeywordsRequest) (codevaldpubsub.GraphResult, error)
}

func (f *fakeGitManager) InitRepo(ctx context.Context, req codevaldpubsub.CreateRepoRequest) (codevaldpubsub.Repository, error) {
	if f.initRepo != nil {
		return f.initRepo(ctx, req)
	}
	return codevaldpubsub.Repository{ID: "repo-1", Name: req.Name, DefaultBranch: "main"}, nil
}
func (f *fakeGitManager) GetRepository(ctx context.Context, repoID string) (codevaldpubsub.Repository, error) {
	if f.getRepository != nil {
		return f.getRepository(ctx, repoID)
	}
	return codevaldpubsub.Repository{ID: repoID, Name: "test-repo", DefaultBranch: "main"}, nil
}
func (f *fakeGitManager) GetRepositoryByName(ctx context.Context, repoName string) (codevaldpubsub.Repository, error) {
	if f.getRepositoryByName != nil {
		return f.getRepositoryByName(ctx, repoName)
	}
	return codevaldpubsub.Repository{ID: "repo-1", Name: repoName, DefaultBranch: "main"}, nil
}
func (f *fakeGitManager) ListRepositories(ctx context.Context) ([]codevaldpubsub.Repository, error) {
	if f.listRepositories != nil {
		return f.listRepositories(ctx)
	}
	return []codevaldpubsub.Repository{{ID: "repo-1", Name: "test-repo", DefaultBranch: "main"}}, nil
}
func (f *fakeGitManager) DeleteRepo(ctx context.Context, repoID string) error {
	if f.deleteRepo != nil {
		return f.deleteRepo(ctx, repoID)
	}
	return nil
}
func (f *fakeGitManager) PurgeRepo(ctx context.Context, repoID string) error {
	if f.purgeRepo != nil {
		return f.purgeRepo(ctx, repoID)
	}
	return nil
}
func (f *fakeGitManager) CreateBranch(ctx context.Context, req codevaldpubsub.CreateBranchRequest) (codevaldpubsub.Branch, error) {
	if f.createBranch != nil {
		return f.createBranch(ctx, req)
	}
	return codevaldpubsub.Branch{ID: "branch-1", Name: req.Name}, nil
}
func (f *fakeGitManager) GetBranch(ctx context.Context, branchID string) (codevaldpubsub.Branch, error) {
	if f.getBranch != nil {
		return f.getBranch(ctx, branchID)
	}
	return codevaldpubsub.Branch{ID: branchID, Name: "main", IsDefault: true}, nil
}
func (f *fakeGitManager) ListBranches(ctx context.Context, repoID string) ([]codevaldpubsub.Branch, error) {
	if f.listBranches != nil {
		return f.listBranches(ctx, repoID)
	}
	return []codevaldpubsub.Branch{{ID: "branch-1", Name: "main", IsDefault: true}}, nil
}
func (f *fakeGitManager) DeleteBranch(ctx context.Context, branchID string) error {
	if f.deleteBranch != nil {
		return f.deleteBranch(ctx, branchID)
	}
	return nil
}
func (f *fakeGitManager) MergeBranch(ctx context.Context, branchID string) (codevaldpubsub.Branch, error) {
	if f.mergeBranch != nil {
		return f.mergeBranch(ctx, branchID)
	}
	return codevaldpubsub.Branch{ID: "branch-1", Name: "main", IsDefault: true}, nil
}
func (f *fakeGitManager) CreateTag(ctx context.Context, req codevaldpubsub.CreateTagRequest) (codevaldpubsub.Tag, error) {
	if f.createTag != nil {
		return f.createTag(ctx, req)
	}
	return codevaldpubsub.Tag{ID: "tag-1", Name: req.Name}, nil
}
func (f *fakeGitManager) GetTag(ctx context.Context, tagID string) (codevaldpubsub.Tag, error) {
	if f.getTag != nil {
		return f.getTag(ctx, tagID)
	}
	return codevaldpubsub.Tag{ID: tagID, Name: "v1.0.0"}, nil
}
func (f *fakeGitManager) ListTags(ctx context.Context, repoID string) ([]codevaldpubsub.Tag, error) {
	if f.listTags != nil {
		return f.listTags(ctx, repoID)
	}
	return []codevaldpubsub.Tag{{ID: "tag-1", Name: "v1.0.0"}}, nil
}
func (f *fakeGitManager) DeleteTag(ctx context.Context, tagID string) error {
	if f.deleteTag != nil {
		return f.deleteTag(ctx, tagID)
	}
	return nil
}
func (f *fakeGitManager) WriteFile(ctx context.Context, req codevaldpubsub.WriteFileRequest) (codevaldpubsub.Commit, error) {
	if f.writeFile != nil {
		return f.writeFile(ctx, req)
	}
	return codevaldpubsub.Commit{ID: "commit-1", SHA: "abc123", Message: req.Message}, nil
}
func (f *fakeGitManager) ReadFile(ctx context.Context, branchID, path string) (codevaldpubsub.Blob, error) {
	if f.readFile != nil {
		return f.readFile(ctx, branchID, path)
	}
	return codevaldpubsub.Blob{ID: "blob-1", Path: path, Content: "file content", Encoding: "utf-8"}, nil
}
func (f *fakeGitManager) DeleteFile(ctx context.Context, req codevaldpubsub.DeleteFileRequest) (codevaldpubsub.Commit, error) {
	if f.deleteFile != nil {
		return f.deleteFile(ctx, req)
	}
	return codevaldpubsub.Commit{ID: "commit-2", SHA: "def456", Message: "Delete " + req.Path}, nil
}
func (f *fakeGitManager) ListDirectory(ctx context.Context, branchID, path string) ([]codevaldpubsub.FileEntry, error) {
	if f.listDirectory != nil {
		return f.listDirectory(ctx, branchID, path)
	}
	return []codevaldpubsub.FileEntry{{Name: "README.md", Path: "README.md"}}, nil
}
func (f *fakeGitManager) Log(ctx context.Context, branchID string, filter codevaldpubsub.LogFilter) ([]codevaldpubsub.CommitEntry, error) {
	if f.log != nil {
		return f.log(ctx, branchID, filter)
	}
	return []codevaldpubsub.CommitEntry{{SHA: "abc123", Message: "initial commit"}}, nil
}
func (f *fakeGitManager) Diff(ctx context.Context, fromRef, toRef string) ([]codevaldpubsub.FileDiff, error) {
	if f.diffFunc != nil {
		return f.diffFunc(ctx, fromRef, toRef)
	}
	return []codevaldpubsub.FileDiff{{Path: "README.md", Operation: "added"}}, nil
}

func (f *fakeGitManager) ImportRepo(_ context.Context, _ codevaldpubsub.ImportRepoRequest) (codevaldpubsub.ImportJob, error) {
	return codevaldpubsub.ImportJob{ID: "fake-job-id", Status: "pending"}, nil
}

func (f *fakeGitManager) GetImportStatus(_ context.Context, _ string) (codevaldpubsub.ImportJob, error) {
	return codevaldpubsub.ImportJob{ID: "fake-job-id", Status: "pending"}, nil
}

func (f *fakeGitManager) CancelImport(_ context.Context, _ string) error {
	return nil
}

func (f *fakeGitManager) CreateKeyword(_ context.Context, req codevaldpubsub.CreateKeywordRequest) (codevaldpubsub.Keyword, error) {
	if f.createKeyword != nil {
		return f.createKeyword(context.Background(), req)
	}
	return codevaldpubsub.Keyword{}, nil
}

func (f *fakeGitManager) GetKeyword(_ context.Context, kwID string) (codevaldpubsub.Keyword, error) {
	if f.getKeyword != nil {
		return f.getKeyword(context.Background(), kwID)
	}
	return codevaldpubsub.Keyword{}, nil
}

func (f *fakeGitManager) ListKeywords(_ context.Context, filter codevaldpubsub.KeywordFilter) ([]codevaldpubsub.Keyword, error) {
	if f.listKeywords != nil {
		return f.listKeywords(context.Background(), filter)
	}
	return nil, nil
}

func (f *fakeGitManager) GetKeywordTree(_ context.Context, kwID string) ([]codevaldpubsub.KeywordTreeNode, error) {
	if f.getKeywordTree != nil {
		return f.getKeywordTree(context.Background(), kwID)
	}
	return nil, nil
}

func (f *fakeGitManager) UpdateKeyword(_ context.Context, kwID string, req codevaldpubsub.UpdateKeywordRequest) (codevaldpubsub.Keyword, error) {
	if f.updateKeyword != nil {
		return f.updateKeyword(context.Background(), kwID, req)
	}
	return codevaldpubsub.Keyword{}, nil
}

func (f *fakeGitManager) DeleteKeyword(_ context.Context, kwID string) error {
	if f.deleteKeyword != nil {
		return f.deleteKeyword(context.Background(), kwID)
	}
	return nil
}

func (f *fakeGitManager) CreateEdge(_ context.Context, req codevaldpubsub.CreateEdgeRequest) error {
	if f.createEdge != nil {
		return f.createEdge(context.Background(), req)
	}
	return nil
}

func (f *fakeGitManager) DeleteEdge(_ context.Context, req codevaldpubsub.DeleteEdgeRequest) error {
	if f.deleteEdge != nil {
		return f.deleteEdge(context.Background(), req)
	}
	return nil
}

func (f *fakeGitManager) GetNeighborhood(_ context.Context, branchID, entityID string, depth int) (codevaldpubsub.GraphResult, error) {
	if f.getNeighborhood != nil {
		return f.getNeighborhood(context.Background(), branchID, entityID, depth)
	}
	return codevaldpubsub.GraphResult{}, nil
}

func (f *fakeGitManager) SearchByKeywords(_ context.Context, req codevaldpubsub.SearchByKeywordsRequest) (codevaldpubsub.GraphResult, error) {
	if f.searchByKeywords != nil {
		return f.searchByKeywords(context.Background(), req)
	}
	return codevaldpubsub.GraphResult{}, nil
}

func (f *fakeGitManager) QueryGraph(_ context.Context, _ codevaldpubsub.QueryGraphRequest) (codevaldpubsub.GraphResult, error) {
	return codevaldpubsub.GraphResult{}, nil
}

func (f *fakeGitManager) FetchBranch(_ context.Context, req codevaldpubsub.FetchBranchRequest) (codevaldpubsub.FetchBranchJob, error) {
	return codevaldpubsub.FetchBranchJob{}, nil
}

func (f *fakeGitManager) GetFetchBranchStatus(_ context.Context, jobID string) (codevaldpubsub.FetchBranchJob, error) {
	return codevaldpubsub.FetchBranchJob{}, nil
}

func (f *fakeGitManager) GetBranchByName(_ context.Context, _ string, _ string) (codevaldpubsub.Branch, error) {
	return codevaldpubsub.Branch{}, nil
}

func (f *fakeGitManager) IndexPushedBranch(_ context.Context, _, _, _, _ string) error {
	return nil
}

// ── test server setup ─────────────────────────────────────────────────────────

const bufSize = 1024 * 1024

// newTestServer spins up a real gRPC server backed by the given manager and
// returns a client connected to it. The server and connection are cleaned up
// when t ends.
func newTestServer(t *testing.T, mgr codevaldpubsub.GitManager) pb.GitServiceClient {
	t.Helper()
	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	pb.RegisterGitServiceServer(srv, server.New(mgr))
	go func() { _ = srv.Serve(lis) }()
	t.Cleanup(func() {
		srv.GracefulStop()
		_ = lis.Close()
	})
	conn, err := grpc.NewClient("passthrough://bufnet",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("grpc.NewClient: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewGitServiceClient(conn)
}

// grpcCode extracts the gRPC status code from an error (codes.OK if nil).
func grpcCode(err error) codes.Code {
	if err == nil {
		return codes.OK
	}
	if s, ok := status.FromError(err); ok {
		return s.Code()
	}
	return codes.Unknown
}

// ── InitRepo ──────────────────────────────────────────────────────────────────

func TestServer_InitRepo_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		initRepo: func(_ context.Context, req codevaldpubsub.CreateRepoRequest) (codevaldpubsub.Repository, error) {
			return codevaldpubsub.Repository{
				ID:            "repo-99",
				Name:          req.Name,
				DefaultBranch: "main",
				AgencyID:      "agency-1",
			}, nil
		},
	})
	resp, err := client.InitRepo(context.Background(), &pb.InitRepoRequest{
		Name:          "my-repo",
		DefaultBranch: "main",
	})
	if err != nil {
		t.Fatalf("InitRepo: %v", err)
	}
	if resp.GetId() != "repo-99" {
		t.Errorf("resp.Id = %q, want %q", resp.GetId(), "repo-99")
	}
	if resp.GetName() != "my-repo" {
		t.Errorf("resp.Name = %q, want %q", resp.GetName(), "my-repo")
	}
}

func TestServer_InitRepo_AlreadyExists(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		initRepo: func(_ context.Context, _ codevaldpubsub.CreateRepoRequest) (codevaldpubsub.Repository, error) {
			return codevaldpubsub.Repository{}, codevaldpubsub.ErrRepoAlreadyExists
		},
	})
	_, err := client.InitRepo(context.Background(), &pb.InitRepoRequest{Name: "x"})
	if grpcCode(err) != codes.AlreadyExists {
		t.Errorf("got code %v, want AlreadyExists", grpcCode(err))
	}
}

// ── GetRepository ─────────────────────────────────────────────────────────────

func TestServer_GetRepository_NotInitialised(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		getRepository: func(_ context.Context, _ string) (codevaldpubsub.Repository, error) {
			return codevaldpubsub.Repository{}, codevaldpubsub.ErrRepoNotInitialised
		},
	})
	_, err := client.GetRepository(context.Background(), &pb.GetRepositoryRequest{})
	if grpcCode(err) != codes.NotFound {
		t.Errorf("got code %v, want NotFound", grpcCode(err))
	}
}

// ── DeleteRepo ────────────────────────────────────────────────────────────────

func TestServer_DeleteRepo_Success(t *testing.T) {
	called := false
	client := newTestServer(t, &fakeGitManager{
		deleteRepo: func(_ context.Context, _ string) error {
			called = true
			return nil
		},
	})
	_, err := client.DeleteRepo(context.Background(), &pb.DeleteRepoRequest{})
	if err != nil {
		t.Fatalf("DeleteRepo: %v", err)
	}
	if !called {
		t.Error("DeleteRepo handler not called")
	}
}

// ── CreateBranch ──────────────────────────────────────────────────────────────

func TestServer_CreateBranch_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		createBranch: func(_ context.Context, req codevaldpubsub.CreateBranchRequest) (codevaldpubsub.Branch, error) {
			return codevaldpubsub.Branch{ID: "br-1", Name: req.Name}, nil
		},
	})
	resp, err := client.CreateBranch(context.Background(), &pb.CreateBranchRequest{Name: "task/feature"})
	if err != nil {
		t.Fatalf("CreateBranch: %v", err)
	}
	if resp.GetName() != "task/feature" {
		t.Errorf("resp.Name = %q, want %q", resp.GetName(), "task/feature")
	}
}

func TestServer_CreateBranch_AlreadyExists(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		createBranch: func(_ context.Context, _ codevaldpubsub.CreateBranchRequest) (codevaldpubsub.Branch, error) {
			return codevaldpubsub.Branch{}, codevaldpubsub.ErrBranchExists
		},
	})
	_, err := client.CreateBranch(context.Background(), &pb.CreateBranchRequest{Name: "dupe"})
	if grpcCode(err) != codes.AlreadyExists {
		t.Errorf("got code %v, want AlreadyExists", grpcCode(err))
	}
}

// ── GetBranch ─────────────────────────────────────────────────────────────────

func TestServer_GetBranch_NotFound(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		getBranch: func(_ context.Context, _ string) (codevaldpubsub.Branch, error) {
			return codevaldpubsub.Branch{}, codevaldpubsub.ErrBranchNotFound
		},
	})
	_, err := client.GetBranch(context.Background(), &pb.GetBranchRequest{BranchId: "ghost"})
	if grpcCode(err) != codes.NotFound {
		t.Errorf("got code %v, want NotFound", grpcCode(err))
	}
}

// ── DeleteBranch ──────────────────────────────────────────────────────────────

func TestServer_DeleteBranch_DefaultForbidden(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		deleteBranch: func(_ context.Context, _ string) error {
			return codevaldpubsub.ErrDefaultBranchDeleteForbidden
		},
	})
	_, err := client.DeleteBranch(context.Background(), &pb.DeleteBranchRequest{BranchId: "main-id"})
	if grpcCode(err) != codes.FailedPrecondition {
		t.Errorf("got code %v, want FailedPrecondition", grpcCode(err))
	}
}

// ── MergeBranch ───────────────────────────────────────────────────────────────

func TestServer_MergeBranch_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		mergeBranch: func(_ context.Context, branchID string) (codevaldpubsub.Branch, error) {
			return codevaldpubsub.Branch{ID: "main-id", Name: "main", IsDefault: true, HeadCommitID: "commit-99"}, nil
		},
	})
	resp, err := client.MergeBranch(context.Background(), &pb.MergeBranchRequest{BranchId: "feature-id"})
	if err != nil {
		t.Fatalf("MergeBranch: %v", err)
	}
	if !resp.GetIsDefault() {
		t.Error("merged branch IsDefault = false, want true")
	}
	if resp.GetHeadCommitId() != "commit-99" {
		t.Errorf("HeadCommitId = %q, want %q", resp.GetHeadCommitId(), "commit-99")
	}
}

func TestServer_MergeBranch_Conflict(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		mergeBranch: func(_ context.Context, _ string) (codevaldpubsub.Branch, error) {
			return codevaldpubsub.Branch{}, &codevaldpubsub.ErrMergeConflict{
				TaskID:           "task-branch",
				ConflictingFiles: []string{"file.txt"},
			}
		},
	})
	_, err := client.MergeBranch(context.Background(), &pb.MergeBranchRequest{BranchId: "conflict-branch"})
	if grpcCode(err) != codes.Aborted {
		t.Errorf("got code %v, want Aborted", grpcCode(err))
	}
}

// ── WriteFile ─────────────────────────────────────────────────────────────────

func TestServer_WriteFile_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		writeFile: func(_ context.Context, req codevaldpubsub.WriteFileRequest) (codevaldpubsub.Commit, error) {
			return codevaldpubsub.Commit{ID: "c1", SHA: "abc", Message: req.Message}, nil
		},
	})
	resp, err := client.WriteFile(context.Background(), &pb.WriteFileRequest{
		BranchId: "branch-1",
		Path:     "README.md",
		Content:  "# Hello",
		Message:  "Add README",
	})
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if resp.GetSha() != "abc" {
		t.Errorf("commit.SHA = %q, want %q", resp.GetSha(), "abc")
	}
	if resp.GetMessage() != "Add README" {
		t.Errorf("commit.Message = %q, want %q", resp.GetMessage(), "Add README")
	}
}

func TestServer_WriteFile_BranchNotFound(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		writeFile: func(_ context.Context, _ codevaldpubsub.WriteFileRequest) (codevaldpubsub.Commit, error) {
			return codevaldpubsub.Commit{}, codevaldpubsub.ErrBranchNotFound
		},
	})
	_, err := client.WriteFile(context.Background(), &pb.WriteFileRequest{
		BranchId: "ghost-branch",
		Path:     "file.txt",
		Content:  "content",
	})
	if grpcCode(err) != codes.NotFound {
		t.Errorf("got code %v, want NotFound", grpcCode(err))
	}
}

// ── ReadFile ──────────────────────────────────────────────────────────────────

func TestServer_ReadFile_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		readFile: func(_ context.Context, _, path string) (codevaldpubsub.Blob, error) {
			return codevaldpubsub.Blob{ID: "blob-1", Path: path, Content: "file content"}, nil
		},
	})
	resp, err := client.ReadFile(context.Background(), &pb.ReadFileRequest{
		BranchId: "branch-1",
		Path:     "README.md",
	})
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if resp.GetContent() != "file content" {
		t.Errorf("blob.Content = %q, want %q", resp.GetContent(), "file content")
	}
}

func TestServer_ReadFile_FileNotFound(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		readFile: func(_ context.Context, _, _ string) (codevaldpubsub.Blob, error) {
			return codevaldpubsub.Blob{}, codevaldpubsub.ErrFileNotFound
		},
	})
	_, err := client.ReadFile(context.Background(), &pb.ReadFileRequest{
		BranchId: "branch-1",
		Path:     "ghost.txt",
	})
	if grpcCode(err) != codes.NotFound {
		t.Errorf("got code %v, want NotFound", grpcCode(err))
	}
}

// ── ListDirectory ─────────────────────────────────────────────────────────────

func TestServer_ListDirectory_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		listDirectory: func(_ context.Context, _, _ string) ([]codevaldpubsub.FileEntry, error) {
			return []codevaldpubsub.FileEntry{
				{Name: "main.go", Path: "main.go", IsDir: false, Size: 42},
				{Name: "src", Path: "src", IsDir: true},
			}, nil
		},
	})
	resp, err := client.ListDirectory(context.Background(), &pb.ListDirectoryRequest{
		BranchId: "branch-1",
		Path:     "",
	})
	if err != nil {
		t.Fatalf("ListDirectory: %v", err)
	}
	if len(resp.GetEntries()) != 2 {
		t.Fatalf("entries count = %d, want 2", len(resp.GetEntries()))
	}
}

// ── Log ───────────────────────────────────────────────────────────────────────

func TestServer_Log_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		log: func(_ context.Context, _ string, filter codevaldpubsub.LogFilter) ([]codevaldpubsub.CommitEntry, error) {
			return []codevaldpubsub.CommitEntry{
				{SHA: "sha1", Author: "alice", Message: "first commit"},
				{SHA: "sha2", Author: "bob", Message: "second commit"},
			}, nil
		},
	})
	resp, err := client.Log(context.Background(), &pb.LogRequest{BranchId: "branch-1"})
	if err != nil {
		t.Fatalf("Log: %v", err)
	}
	if len(resp.GetCommits()) != 2 {
		t.Fatalf("commits count = %d, want 2", len(resp.GetCommits()))
	}
}

// ── Diff ──────────────────────────────────────────────────────────────────────

func TestServer_Diff_Success(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		diffFunc: func(_ context.Context, fromRef, _ string) ([]codevaldpubsub.FileDiff, error) {
			return []codevaldpubsub.FileDiff{
				{Path: "added.txt", Operation: "added"},
				{Path: "removed.txt", Operation: "deleted"},
			}, nil
		},
	})
	resp, err := client.Diff(context.Background(), &pb.DiffRequest{FromRef: "main", ToRef: "feature"})
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if len(resp.GetDiffs()) != 2 {
		t.Fatalf("diffs count = %d, want 2", len(resp.GetDiffs()))
	}
}

func TestServer_Diff_RefNotFound(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		diffFunc: func(_ context.Context, _, _ string) ([]codevaldpubsub.FileDiff, error) {
			return nil, codevaldpubsub.ErrRefNotFound
		},
	})
	_, err := client.Diff(context.Background(), &pb.DiffRequest{FromRef: "ghost", ToRef: "main"})
	if grpcCode(err) != codes.NotFound {
		t.Errorf("got code %v, want NotFound", grpcCode(err))
	}
}

// ── Tag management ────────────────────────────────────────────────────────────

func TestServer_CreateTag_AlreadyExists(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		createTag: func(_ context.Context, _ codevaldpubsub.CreateTagRequest) (codevaldpubsub.Tag, error) {
			return codevaldpubsub.Tag{}, codevaldpubsub.ErrTagAlreadyExists
		},
	})
	_, err := client.CreateTag(context.Background(), &pb.CreateTagRequest{
		Name:     "v1.0.0",
		CommitId: "commit-1",
	})
	if grpcCode(err) != codes.AlreadyExists {
		t.Errorf("got code %v, want AlreadyExists", grpcCode(err))
	}
}

func TestServer_GetTag_NotFound(t *testing.T) {
	client := newTestServer(t, &fakeGitManager{
		getTag: func(_ context.Context, _ string) (codevaldpubsub.Tag, error) {
			return codevaldpubsub.Tag{}, codevaldpubsub.ErrTagNotFound
		},
	})
	_, err := client.GetTag(context.Background(), &pb.GetTagRequest{TagId: "ghost-tag"})
	if grpcCode(err) != codes.NotFound {
		t.Errorf("got code %v, want NotFound", grpcCode(err))
	}
}

// ── Error mapping completeness ────────────────────────────────────────────────

// TestServer_ErrorMapping verifies the complete error-to-gRPC-code table
// without requiring a real network round-trip (uses the server directly).
func TestServer_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want codes.Code
	}{
		{"ErrRepoNotFound", codevaldpubsub.ErrRepoNotFound, codes.NotFound},
		{"ErrRepoNotInitialised", codevaldpubsub.ErrRepoNotInitialised, codes.NotFound},
		{"ErrRepoAlreadyExists", codevaldpubsub.ErrRepoAlreadyExists, codes.AlreadyExists},
		{"ErrBranchNotFound", codevaldpubsub.ErrBranchNotFound, codes.NotFound},
		{"ErrBranchExists", codevaldpubsub.ErrBranchExists, codes.AlreadyExists},
		{"ErrTagNotFound", codevaldpubsub.ErrTagNotFound, codes.NotFound},
		{"ErrTagAlreadyExists", codevaldpubsub.ErrTagAlreadyExists, codes.AlreadyExists},
		{"ErrFileNotFound", codevaldpubsub.ErrFileNotFound, codes.NotFound},
		{"ErrRefNotFound", codevaldpubsub.ErrRefNotFound, codes.NotFound},
		{"ErrDefaultBranchDeleteForbidden", codevaldpubsub.ErrDefaultBranchDeleteForbidden, codes.FailedPrecondition},
		{"ErrMergeConflict", &codevaldpubsub.ErrMergeConflict{TaskID: "t", ConflictingFiles: []string{"f"}}, codes.Aborted},
		{"unknown", errors.New("mystery"), codes.Internal},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			client := newTestServer(t, &fakeGitManager{
				getRepository: func(_ context.Context, _ string) (codevaldpubsub.Repository, error) {
					return codevaldpubsub.Repository{}, tc.err
				},
			})
			_, err := client.GetRepository(context.Background(), &pb.GetRepositoryRequest{})
			if grpcCode(err) != tc.want {
				t.Errorf("error %v: got gRPC code %v, want %v", tc.err, grpcCode(err), tc.want)
			}
		})
	}
}
