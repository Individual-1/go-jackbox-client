package drawful

import (
	"encoding/json"
	"io/ioutil"
)

// LoadDrawing reads a json file and loads the results into a PictureLine slice
func LoadDrawing(fn string) ([]PictureLine, error) {
	var pl []PictureLine

	contents, err := ioutil.ReadFile(fn)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(contents, &pl)
	if err != nil {
		return nil, err
	}

	return pl, nil
}
