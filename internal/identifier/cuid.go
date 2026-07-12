package identifier

import "github.com/nrednav/cuid2"

type CUID2Generator struct {
	gen func() string
}

func NewCUID2Generator() *CUID2Generator {
	return &CUID2Generator{
		gen: cuid2.Generate,
	}
}

func (c *CUID2Generator) Generate() string {
	return c.gen()
}
