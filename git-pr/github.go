package main

import (
	"gotools/rest"
	"log"
	"net/url"
)

type Github struct {
	args *Args
	r    *rest.Rest
	url  string
}

func NewGithub(args *Args) Git {
	g := Github{}
	g.args = args
	g.url = "https://api.github.com"
	g.r = rest.NewRest(g.url,
		args.User, args.Password, args.Verbose)
	return &g
}

func (g *Github) request(method, path string, data interface{}) ([]map[string]interface{}, error) {
	x, err := g.r.Do(method, g.url+path, nil, data)
	return x, err
}

func (g *Github) Get(path string) []map[string]interface{} {
	x, err := g.request("GET", path, nil)
	if err != nil {
		log.Panic(err)
	}
	return x
}

func (g *Github) Query(path string, query url.Values) ([]map[string]interface{}, error) {
	x, err := g.r.Do("GET", g.url+path, query, nil)
	if err != nil {
		log.Panic(err)
	}
	return x, err
}

func (g *Github) Post(path string, data interface{}) ([]map[string]interface{}, error) {
	return g.request("POST", path, data)
}

func (g *Github) Put(path string, data interface{}) ([]map[string]interface{}, error) {
	return g.request("PUT", path, data)
}

func (g *Github) test() {
	x := g.Get(g.args.args[0])
	dump("result", x)
}

func (g *Github) create() {
}

func (g *Github) merge() {
}
