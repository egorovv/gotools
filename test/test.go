package main

import (
	"fmt"
	"gotools/rest"
)

func main() {

	r := rest.NewRest("https://jenkins2.eng.velocloud.net",
		"vadim", "8cca0fe4b8e0151b2c65fe0500acab09", true)

	resp, err := r.Get("/job/Release-2.4/lastBuild/api/json", nil, nil)

	fmt.Printf("%s, %s\n", err, resp)
}
