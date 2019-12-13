package llb

// Gateway returns a State representing the result of solving a frontend image
// using the gateway frontend. By default, it assumes it has a binary at
// `/run` that implements a frontend GRPC client talking over stdio, but this is
// overridable by providing an OCI image config.
//
// For example, you can define an image with the default `imagemetaresolver`:
// ```
// st := llb.Gateway(llb.Image("docker.io/library/dockerfile:latest", imagemetaresolver.WithDefault))
// ```
//
// If there are states that depend on the result of a frontend, it is more
// efficient to use Gateway to represent a lazy solve than using read or stat
// file APIs on a solved result.
func Gateway(root State, opts ...FrontendOption) State {
	var info FrontendInfo
	for _, opt := range opts {
		opt.SetFrontendOption(&info)
	}

	frontend := NewFrontend(root, info.Constraints)
	return NewState(frontend.Output())
}

// FrontendOption is an option for a frontend-based build state.
type FrontendOption interface {
	SetFrontendOption(*FrontendInfo)
}

type frontendOptionFunc func(*FrontendInfo)

func (fn frontendOptionFunc) SetFrontendOption(fi *FrontendInfo) {
	fn(fi)
}

// FrontendInfo contains options for a frontend-based build state.
type FrontendInfo struct {
	constraintsWrapper
}

func (fi *FrontendInfo) SetFrontendOption(fi2 *FrontendInfo) {
	*fi2 = *fi
}

var _ FrontendOption = &FrontendInfo{}
