// Package server implements the GitService gRPC handler.
package server

import (
	"errors"

	codevaldpubsub "github.com/aosanya/CodeValdGit"
	pb "github.com/aosanya/CodeValdGit/gen/go/codevaldpubsub/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// toGRPCError maps CodeValdGit domain errors to the appropriate gRPC status.
// ErrMergeConflict is mapped to codes.Aborted with a MergeConflictInfo detail
// message so clients can unpack the conflicting file list.
// Unknown errors are wrapped as codes.Internal.
func toGRPCError(err error) error {
	var mergeErr *codevaldpubsub.ErrMergeConflict
	switch {
	case errors.As(err, &mergeErr):
		st := status.New(codes.Aborted, err.Error())
		st, _ = st.WithDetails(&pb.MergeConflictInfo{
			BranchId:         mergeErr.TaskID,
			ConflictingFiles: mergeErr.ConflictingFiles,
		})
		return st.Err()
	case errors.Is(err, codevaldpubsub.ErrRepoNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrRepoNotInitialised):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrRepoAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, codevaldpubsub.ErrBranchNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrBranchExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, codevaldpubsub.ErrTagNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrTagAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, codevaldpubsub.ErrFileNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrRefNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrDefaultBranchDeleteForbidden):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, codevaldpubsub.ErrImportJobNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrImportInProgress):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, codevaldpubsub.ErrImportJobNotCancellable):
		return status.Error(codes.FailedPrecondition, err.Error())
	case errors.Is(err, codevaldpubsub.ErrKeywordNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrKeywordAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, codevaldpubsub.ErrEdgeNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, codevaldpubsub.ErrInvalidRelationship):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, codevaldpubsub.ErrBlobContentUnavailable):
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		return status.Errorf(codes.Internal, "internal error: %v", err)
	}
}
