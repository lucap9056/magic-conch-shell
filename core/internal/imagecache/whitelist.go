package imagecache

type DomainWhitelist struct {
	allowed map[string]struct{}
}

func NewDomainWhitelist(domains []string) *DomainWhitelist {
	if len(domains) == 0 {
		return nil
	}
	allowed := make(map[string]struct{})
	for _, d := range domains {
		allowed[d] = struct{}{}
	}
	return &DomainWhitelist{allowed: allowed}
}

func (w *DomainWhitelist) IsAllowed(hostname string) bool {
	if w == nil {
		return true
	}
	_, ok := w.allowed[hostname]
	return ok
}
