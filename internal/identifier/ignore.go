package identifier

type IgnoreGenerator struct{}

func NewIgnoreGenerator() *IgnoreGenerator {
	return &IgnoreGenerator{}
}

func (g *IgnoreGenerator) Generate() string {
	return ""
}
