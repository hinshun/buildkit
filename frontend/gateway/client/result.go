package client

import (
	"context"
	"sync"

	"github.com/moby/buildkit/client/llb"
	"github.com/moby/buildkit/solver/pb"
	"github.com/pkg/errors"
)

type BuildFunc func(context.Context, Client) (*Result, error)

type Result struct {
	mu         sync.Mutex
	Ref        Reference
	Refs       map[string]Reference
	Metadata   map[string][]byte
	Definition *pb.Definition
	output     llb.Output
}

func NewResult() *Result {
	return &Result{}
}

func (r *Result) SetDefinition(def *pb.Definition) error {
	op, err := llb.NewDefinitionOp(def)
	if err != nil {
		return err
	}

	r.output = op.Output()
	return nil
}

func (r *Result) AddMeta(k string, v []byte) {
	r.mu.Lock()
	if r.Metadata == nil {
		r.Metadata = map[string][]byte{}
	}
	r.Metadata[k] = v
	r.mu.Unlock()
}

func (r *Result) AddRef(k string, ref Reference) {
	r.mu.Lock()
	if r.Refs == nil {
		r.Refs = map[string]Reference{}
	}
	r.Refs[k] = ref
	r.mu.Unlock()
}

func (r *Result) SetRef(ref Reference) {
	r.Ref = ref
}

func (r *Result) SingleRef() (Reference, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.Refs != nil && r.Ref == nil {
		return nil, errors.Errorf("invalid map result")
	}

	return r.Ref, nil
}
