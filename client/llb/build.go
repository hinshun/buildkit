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

	op := NewBuild(source, &info, info.Constraints)
	return NewState(op.Output())
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
	var inputs []Output
	if source.Output() != nil {
		inputs = append(inputs, source.Output())
	}
	return &BuildOp{builder: pb.LLBBuilder, root: source, inputs: inputs, constraints: c, bi: info}
}

// NewFrontend returns a new BuildOp that will solve using a frontend.
func NewFrontend(frontend State, info *FrontendInfo, c Constraints) *BuildOp {
	var inputs []Output
	if frontend.Output() != nil {
		inputs = append(inputs, frontend.Output())
	}
	return &BuildOp{builder: pb.FrontendBuilder, root: frontend, inputs: inputs, constraints: c, fi: info}
}

// BuildOp is an Op implementation that represents a lazy solve using a
// definition or frontend.
type BuildOp struct {
	MarshalCache
	builder     pb.InputIndex
	root        State
	inputs      []Output
	constraints Constraints

	bi *BuildInfo

	fi   *FrontendInfo
	defs map[string]*pb.Definition
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
	if b.fi != nil {
		if len(b.fi.Inputs) > 0 && b.defs == nil {
			b.defs = make(map[string]*pb.Definition)
			for key, input := range b.fi.Inputs {
				def, err := input.Marshal()
				if err != nil {
					return err
				}
				b.defs[key] = def.ToPB()
			}
		}
	}
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
		Inputs:  make(map[string]*pb.BuildInput),
		Attrs:   make(map[string]string),
	}

	if b.builder == pb.LLBBuilder {
		pbo.Inputs[pb.LLBDefinitionInput] = &pb.BuildInput{Input: pb.InputIndex(0)}

		if b.bi.DefinitionFilename != "" {
			pbo.Attrs[pb.AttrLLBDefinitionFilename] = b.bi.DefinitionFilename
		}
	} else {
		pbo.Args = b.root.GetArgs()
		pbo.Env = b.root.Env()
		pbo.Cwd = b.root.GetDir()
		pbo.Attrs = b.fi.Opts

		pbo.Defs = make(map[string]*pb.Definition)
		for key, pbDef := range b.defs {
			pbo.Defs[key] = pbDef
		}
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
	if b.root.Output() == nil {
		return nil
	}
	return b
}

func (b *BuildOp) Inputs() []Output {
	return b.inputs
}
