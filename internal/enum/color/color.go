package color

type Color string

const (
	Red   Color = "red"
	Green Color = "green"
	Blue  Color = "blue"
)

func (c Color) String() string {
	return string(c)
}
