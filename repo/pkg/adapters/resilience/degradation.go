package resilience

type DependencyMode string

const (
	DependencyStrong DependencyMode = "strong"
	DependencyWeak   DependencyMode = "weak"
)

func DependencyModeFor(name string) DependencyMode {
	switch name {
	case "object-store", "vector-store":
		return DependencyWeak
	default:
		return DependencyStrong
	}
}

func (m DependencyMode) IsWeak() bool {
	return m == DependencyWeak
}
