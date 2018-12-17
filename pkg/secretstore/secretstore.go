package secretstore

// Client ist the interface implemented by all secret stores
type Client interface {
	GetSecretValue(name string) (string, error)
	GetSecretValueForVersion(name, version string) (string, error)
}
