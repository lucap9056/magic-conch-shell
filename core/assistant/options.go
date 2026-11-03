package assistant

import "github.com/lucap9056/magic-conch-shell/filetoken"

type Options struct {
	cachePath         string
	fileTokenVerifier *filetoken.Verifier
}

type Option = func(*Options)

func defaultOptions() *Options {
	return &Options{
		cachePath: "image_cache",
	}
}

func WithCachePath(path string) Option {
	return func(opts *Options) {
		opts.cachePath = path
	}
}

func WithFileTokenVerifier(verifier *filetoken.Verifier) Option {
	return func(opts *Options) {
		opts.fileTokenVerifier = verifier
	}
}
