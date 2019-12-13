package llb

import (
	"github.com/moby/buildkit/solver/pb"
	"github.com/moby/buildkit/util/apicaps"
	digest "github.com/opencontainers/go-digest"
)

// Build returns a State representing the result of solving a LLB definition
// in the source state's filesystem. By default, this is
// `/buildkit.llb.definition`, but this is overridable.
//
// If there are states that depend on the result of a solve, it is more
// efficient to use Build to represent a lazy solve than using read or stat
// file APIs on a solved result.
func Build(source State, opts ...BuildOption) State {
	var info BuildInfo
	for _, opt := range opts {
		opt.SetBuildOption(&info)
	}

	build := NewBuild(source, &info, info.Constraints)
	return NewState(build.Output())
}

// BuildOption is an option for a definition-based build state.
type BuildOption interface {
	SetBuildOption(*BuildInfo)
}

type buildOptionFunc func(*BuildInfo)

func (fn buildOptionFunc) SetBuildOption(bi *BuildInfo) {
	fn(bi)
}

// BuildInfo contains options for a definition-based build state.
type BuildInfo struct {
	constraintsWrapper
	DefinitionFilename string
}

func (bi *BuildInfo) SetBuildOption(bi2 *BuildInfo) {
	*bi2 = *bi
}

var _ BuildOption = &BuildInfo{}

// WithFilename specifies the filename for the LLB definition file in the
// source state.
func WithFilename(fn string) BuildOption {
	return buildOptionFunc(func(bi *BuildInfo) {
		bi.DefinitionFilename = fn
	})
}

// NewBuild returns a new BuildOp that will solve using a definition in the
// source state.
func NewBuild(source State, info *BuildInfo, c Constraints) *BuildOp {
	return &BuildOp{builder: pb.LLBBuilder, root: source, inputs: []Output{source.Output()}, info: info, constraints: c}
}

// NewFrontend returns a new BuildOp that will solve using a frontend.
func NewFrontend(root State, c Constraints) *BuildOp {
	return &BuildOp{builder: pb.FrontendBuilder, root: root, inputs: []Output{root.Output()}, constraints: c}
}

// BuildOp is an Op implementation that represents a lazy solve using a
// definition or frontend.
type BuildOp struct {
	MarshalCache
	builder     pb.InputIndex
	root        State
	inputs      []Output
	info        *BuildInfo
	constraints Constraints
}

func (b *BuildOp) ToInput(c *Constraints) (*pb.Input, error) {
	dgst, _, _, err := b.Marshal(c)
	if err != nil {
		return nil, err
	}

	return &pb.Input{Digest: dgst, Index: pb.OutputIndex(0)}, nil
}

func (b *BuildOp) Vertex() Vertex {
	return b
}

func (b *BuildOp) Validate() error {
	return nil
}

func (b *BuildOp) Marshal(c *Constraints) (digest.Digest, []byte, *pb.OpMetadata, error) {
	if b.Cached(c) {
		return b.Load()
	}
	if err := b.Validate(); err != nil {
		return "", nil, nil, err
	}

	pbo := &pb.BuildOp{
		Builder: b.builder,
		Attrs:   make(map[string]string),
	}

	if b.builder == pb.LLBBuilder {
		pbo.Inputs = map[string]*pb.BuildInput{
			pb.LLBDefinitionInput: {Input: pb.InputIndex(0)}}

		if b.info.DefinitionFilename != "" {
			pbo.Attrs[pb.AttrLLBDefinitionFilename] = b.info.DefinitionFilename
		}
	} else {
		pbo.Args = b.root.GetArgs()
		pbo.Env = b.root.Env()
		pbo.Cwd = b.root.GetDir()
	}

	if b.constraints.Metadata.Caps == nil {
		b.constraints.Metadata.Caps = make(map[apicaps.CapID]bool)
	}
	b.constraints.Metadata.Caps[pb.CapBuildOpLLBFileName] = true

	pop, md := MarshalConstraints(c, &b.constraints)
	pop.Op = &pb.Op_Build{
		Build: pbo,
	}

	for _, input := range b.inputs {
		inp, err := input.ToInput(c)
		if err != nil {
			return "", nil, nil, err
		}

		pop.Inputs = append(pop.Inputs, inp)
	}

	dt, err := pop.Marshal()
	if err != nil {
		return "", nil, nil, err
	}

	b.Store(dt, md, c)
	return b.Load()
}

func (b *BuildOp) Output() Output {
	return b
}

func (b *BuildOp) Inputs() []Output {
	return b.inputs
}
