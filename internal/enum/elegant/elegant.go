package elegant

type Color interface {
	protected()
}

type color string

func (c color) protected() {}

func (c color) String() string {
	return string(c)
}

const (
	Red   color = "red"
	Green color = "green"
	Blue  color = "blue"
)
