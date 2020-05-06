package drawful

// PictureLine represents a Drawful Picture Line entry
type PictureLine struct {
	Thickness int     `json:"thickness"`
	Color     string  `json:"color"`
	Points    []Point `json:"points"`
}

// Point represents a Point in the Drawful canvas
type Point struct {
	X int `json:"x"`
	Y int `json:"y"`
}
