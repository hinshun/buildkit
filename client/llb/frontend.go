package llb

// Frontend returns a State representing the result of solving a frontend image
// using the gateway frontend. By default, it assumes it has a binary at
// `/run` that implements a frontend GRPC client talking over stdio, but this is
// overridable by providing an OCI image config.
//
// For example, you can define an image with the default `imagemetaresolver`:
// ```
// st := llb.Frontend(llb.Image("docker.io/library/dockerfile:latest", imagemetaresolver.WithDefault))
// ```
//
// If there are states that depend on the result of a frontend, it is more
// efficient to use Gateway to represent a lazy solve than using read or stat
// file APIs on a solved result.
func Frontend(frontend State, opts ...FrontendOption) State {
	info := FrontendInfo{
		Inputs: make(map[string]State),
		Opts: make(map[string]string),
	}
	for _, opt := range opts {
		opt.SetFrontendOption(&info)
	}

	op := NewFrontend(frontend, &info, info.Constraints)
	return NewState(op.Output())
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
	Inputs map[string]State
	Opts map[string]string
}

func (fi *FrontendInfo) SetFrontendOption(fi2 *FrontendInfo) {
	*fi2 = *fi
}

var _ FrontendOption = &FrontendInfo{}

func WithFrontendInput(key string, input State) FrontendOption {
	return frontendOptionFunc(func(fi *FrontendInfo) {
		fi.Inputs[key] = input
	})
}

func WithFrontendOpt(key, value string) FrontendOption {
	return frontendOptionFunc(func(fi *FrontendInfo) {
		fi.Opts[key] = value
	})
}
