package main

import (
	"encoding/json"
)

func main() {
	dirs, entries, err := RecurseDirs("path_to_file")
	if err != nil {
		return
	}
	print("Done")
	marshal, err := json.Marshal(dirs)
	if err != nil {
		return
	}
	print(string(marshal))
	marshall, err := json.Marshal(entries)
	if err != nil {
		return
	}
	print(string(marshall))
}

