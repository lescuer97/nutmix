package cashu

type ProtectedRoute struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}
type Nut21Info struct {
	OpenIdDiscovery string           `json:"openid_discovery"`
	ClientId        string           `json:"client-id"`
	ProtectedRoutes []ProtectedRoute `json:"protected_endpoints"`
}

// type 
