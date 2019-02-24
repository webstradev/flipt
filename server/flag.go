package server

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	flipt "github.com/markphelps/flipt/proto"
)

func (s *Server) GetFlag(ctx context.Context, req *flipt.GetFlagRequest) (*flipt.Flag, error) {
	if req.Key == "" {
		return nil, EmptyFieldError("key")
	}
	return s.FlagStore.GetFlag(ctx, req)
}

func (s *Server) ListFlags(ctx context.Context, req *flipt.ListFlagRequest) (*flipt.FlagList, error) {
	flags, err := s.FlagStore.ListFlags(ctx, req)
	if err != nil {
		return nil, err
	}

	var resp flipt.FlagList

	for i := range flags {
		resp.Flags = append(resp.Flags, flags[i])
	}

	return &resp, nil
}

func (s *Server) CreateFlag(ctx context.Context, req *flipt.CreateFlagRequest) (*flipt.Flag, error) {
	if req.Key == "" {
		return nil, EmptyFieldError("key")
	}
	if req.Name == "" {
		return nil, EmptyFieldError("name")
	}
	return s.FlagStore.CreateFlag(ctx, req)
}

func (s *Server) UpdateFlag(ctx context.Context, req *flipt.UpdateFlagRequest) (*flipt.Flag, error) {
	if req.Key == "" {
		return nil, EmptyFieldError("key")
	}
	if req.Name == "" {
		return nil, EmptyFieldError("name")
	}
	return s.FlagStore.UpdateFlag(ctx, req)
}

func (s *Server) DeleteFlag(ctx context.Context, req *flipt.DeleteFlagRequest) (*empty.Empty, error) {
	if req.Key == "" {
		return nil, EmptyFieldError("key")
	}

	if err := s.FlagStore.DeleteFlag(ctx, req); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

func (s *Server) CreateVariant(ctx context.Context, req *flipt.CreateVariantRequest) (*flipt.Variant, error) {
	if req.FlagKey == "" {
		return nil, EmptyFieldError("flagKey")
	}
	if req.Key == "" {
		return nil, EmptyFieldError("key")
	}
	return s.FlagStore.CreateVariant(ctx, req)
}

func (s *Server) UpdateVariant(ctx context.Context, req *flipt.UpdateVariantRequest) (*flipt.Variant, error) {
	if req.Id == "" {
		return nil, EmptyFieldError("id")
	}
	if req.FlagKey == "" {
		return nil, EmptyFieldError("flagKey")
	}
	if req.Key == "" {
		return nil, EmptyFieldError("key")
	}
	return s.FlagStore.UpdateVariant(ctx, req)
}

func (s *Server) DeleteVariant(ctx context.Context, req *flipt.DeleteVariantRequest) (*empty.Empty, error) {
	if req.Id == "" {
		return nil, EmptyFieldError("id")
	}
	if req.FlagKey == "" {
		return nil, EmptyFieldError("flagKey")
	}

	if err := s.FlagStore.DeleteVariant(ctx, req); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
