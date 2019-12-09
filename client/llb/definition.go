package llb

import (
	"github.com/moby/buildkit/solver/pb"
	digest "github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// DefinitionOp implements llb.Vertex using a marshalled definition.
//
// For example, after marshalling a LLB state and sending over the wire, the
// LLB state can be reconstructed from the definition.
type DefinitionOp struct {
	MarshalCache
	ops      map[digest.Digest]*pb.Op
	metadata map[digest.Digest]pb.OpMetadata
	dgst     digest.Digest
	index    pb.OutputIndex
}

// NewDefinitionOp returns a new operation from a marshalled definition.
func NewDefinitionOp(def *pb.Definition) (*DefinitionOp, error) {
	ops := make(map[digest.Digest]*pb.Op)

	var dgst digest.Digest

	for _, dt := range def.Def {
		var op pb.Op
		if err := (&op).Unmarshal(dt); err != nil {
			return nil, errors.Wrap(err, "failed to parse llb proto op")
		}
		dgst = digest.FromBytes(dt)
		ops[dgst] = &op
	}

	return &DefinitionOp{
		ops:      ops,
		metadata: def.Metadata,
		dgst:     ops[dgst].Inputs[0].Digest,
	}, nil
}

func (d *DefinitionOp) Validate() error {
	if len(d.ops) == 0 || len(d.metadata) == 0 {
		return errors.Errorf("invalid definition op with no ops")
	}

	op, ok := d.ops[d.dgst]
	if !ok {
		return errors.Errorf("invalid definition op with unknown op %q", d.dgst)
	}

	_, ok = d.metadata[d.dgst]
	if !ok {
		return errors.Errorf("invalid definition op with unknown metadata %q", d.dgst)
	}

	if d.index < 0 || int(d.index) >= len(op.Inputs) {
		return errors.Errorf("invalid definition op with invalid index %d", d.index)
	}

	return nil
}

func (d *DefinitionOp) Marshal(c *Constraints) (digest.Digest, []byte, *pb.OpMetadata, error) {
	if d.Cached(c) {
		return d.Load()
	}
	if err := d.Validate(); err != nil {
		return "", nil, nil, err
	}

	op := d.ops[d.dgst]
	platform := op.Platform.Spec()

	override := Constraints{
		Platform:          &platform,
		WorkerConstraints: op.Constraints.Filter,
		Metadata:          d.metadata[d.dgst],
	}

	pop, md := MarshalConstraints(c, &override)

	pop.Op = op.Op
	pop.Inputs = op.Inputs

	dt, err := pop.Marshal()
	if err != nil {
		return "", nil, nil, err
	}
	d.Store(dt, md, c)
	return d.Load()
}

func (d *DefinitionOp) Output() Output {
	platform := d.ops[d.dgst].Platform.Spec()
	return &output{vertex: d, platform: &platform, getIndex: func() (pb.OutputIndex, error) {
		return d.index, nil
	}}
}

func (d *DefinitionOp) Inputs() []Output {
	op := d.ops[d.dgst]
	platform := op.Platform.Spec()

	var inputs []Output
	for _, input := range op.Inputs {
		vtx := &DefinitionOp{
			ops:   d.ops,
			dgst:  input.Digest,
			index: input.Index,
		}
		inputs = append(inputs, &output{vertex: vtx, platform: &platform, getIndex: func() (pb.OutputIndex, error) {
			return pb.OutputIndex(vtx.index), nil
		}})
	}

	return inputs
}
