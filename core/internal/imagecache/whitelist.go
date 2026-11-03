package imagecache

import "strings"

type DomainWhitelist struct {
	allowed map[string]struct{}
}

func NewDomainWhitelist(domains []string) *DomainWhitelist {
	allowed := make(map[string]struct{}, len(domains))
	for _, domain := range domains {
		normalized := strings.ToLower(strings.TrimSpace(domain))
		if normalized != "" {
			allowed[normalized] = struct{}{}
		}
	}
	return &DomainWhitelist{allowed: allowed}
}

func ParseDomainWhitelist(commaSeparated string) *DomainWhitelist {
	return NewDomainWhitelist(strings.Split(commaSeparated, ","))
}

func (w *DomainWhitelist) IsAllowed(hostname string) bool {
	_, ok := w.allowed[strings.ToLower(hostname)]
	return ok
}
