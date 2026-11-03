package assistant

type Options struct {
	cachePath string
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
