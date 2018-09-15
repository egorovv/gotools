package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"reflect"
	"regexp"
	"strings"
	"text/template"
	"unsafe"
)

type Args struct {
	Owner    string `json:"owner,omitempty"`
	Repo     string `json:"repo,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Team     string `json:"team,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Upstream string `json:"upstream,omitempty"`
	Verbose  bool   `json:"verbose"`
}

type User struct {
	Id   string
	Name string
}

type Git interface {
	create()
	merge()
}

func load(a *Args, fn string) {
	user, err := user.Current()
	if err != nil {
		return
	}
	a.User = user.Username
	path := path.Join(user.HomeDir, fn)

	if f, err := os.Open(path); err == nil {
		defer f.Close()
		p := json.NewDecoder(f)
		p.Decode(a)
	}
}

func load_git() {
	git := map[string]string{}
	config := sh(`git`, `config`, `-l`)
	for _, line := range strings.Split(config, "\n") {
		parts := strings.SplitN(line, `=`, 2)
		git[parts[0]] = parts[1]
	}

	f := func(f *flag.Flag) {
		if val, ok := git[`pr.`+f.Name]; ok {
			flag.Set(f.Name, val)
		}
	}
	flag.VisitAll(f)
}

func parse(a *Args) {
	v := reflect.ValueOf(a).Elem()
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		name := f.Name
		tag := f.Tag.Get("json")
		if tag != "" {
			name = strings.Split(tag, ",")[0]
		}
		vf := v.Field(i)
		switch vf.Type().Kind() {
		case reflect.Bool:
			p := (*bool)(unsafe.Pointer(vf.UnsafeAddr()))
			flag.BoolVar(p, name, *p, "")
		case reflect.String:
			p := (*string)(unsafe.Pointer(vf.UnsafeAddr()))
			flag.StringVar(p, name, *p, "")
		}

	}
	flag.Parse()
}

func sh(cmd string, arg ...string) string {
	out, err := exec.Command(cmd, arg...).Output()
	if err != nil {
		log.Panicf("%s %s : %s", cmd, arg, err)
	}
	return strings.TrimSpace(string(out))
}

func dump(prefix string, v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")

	fmt.Printf("%s: %s\n", prefix, b)
}

func unpack(src, dst interface{}) {
	b, err := json.Marshal(src)
	if err != nil {
		log.Panic(err)
	}
	err = json.Unmarshal(b, dst)
	if err != nil {
		log.Panic(err)
	}
}

func strip(s string) string {
	regex, err := regexp.Compile(`(?m)^#.*$`)
	if err != nil {
		log.Panic(err)
	}
	return regex.ReplaceAllString(s, "")
}

func reviewers(s string) (m []User, desc string) {
	regex, err := regexp.Compile(`(?m)^Review-By:.*$`)
	if err != nil {
		log.Panic(err)
	}
	users := regex.FindAllString(s, -1)
	for _, user := range users {
		u := User{Id: strings.Fields(user)[1]}
		m = append(m, u)
	}
	desc = strings.TrimSpace(regex.ReplaceAllString(s, ""))
	return
}

func edit(args Args, text string, m []User) (subj, desc string) {
	t, err := template.New("PR").Parse(`#
# Edit pull request title and description.
# All lines starting with # will be removed. Of the remaining the first
# line will be used as a title and the rest as description.
# Subject starting with ! will cause the oeration to abort.
#
# User: {{ .Args.User }}
# Branch: {{ .Args.Branch }}
# Upstream: {{ .Args.Upstream }}
# Owner/Repo: {{ .Args.Owner }}/{{ .Args.Repo }}
#
{{.Body}}

# This PR will be sent to the following recipients:
# Review-By: add as a reviewer
# Notify: list of @references to receive notifications
Notify:{{range .Members }} @{{ .Id }}{{end}}
{{range .Members }}Review-By: {{ .Id }} <{{ .Name }}>
{{end}}

`)

	data := struct {
		Body    string
		Members []User
		Args    Args
	}{
		Body:    text,
		Members: m,
		Args:    args,
	}

	f, err := ioutil.TempFile("/tmp", ".bbpr")
	if err != nil {
		log.Panic(err)
	}
	fn := f.Name()
	defer os.Remove(fn)

	err = t.Execute(f, data)
	f.Close()
	cmd := exec.Command("/usr/bin/editor", fn)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	err = cmd.Run()
	if err != nil {
		log.Panic(err)
	}
	db, err := ioutil.ReadFile(fn)
	if err != nil {
		log.Panic(err)
	}

	parts := strings.SplitN(strings.TrimSpace(strip(string(db))), "\n", 2)
	subj = strings.TrimSpace(parts[0])
	desc = strings.TrimSpace(parts[1])
	return
}

func install(args Args) {
	exe := os.Args[0]
	sh(`git`, `config`, `--global`, `alias.pr`, `!`+exe)
}

func main() {
	args := Args{
		Owner:  "velocloud",
		Repo:   "velocloud.src",
		Team:   "vcdp",
		Branch: "{{.Branch}}",
	}

	load(&args, ".git-pr")
	parse(&args)
	load_git()

	t, _ := template.New("PR").Parse(args.Branch)
	b := bytes.NewBufferString("")
	args.Branch = sh(`git`, `symbolic-ref`, `--short`, `HEAD`)
	t.Execute(b, args)
	args.Branch = b.String()

	if args.Upstream == "" {
		args.Upstream = strings.SplitN(
			sh(`git`, `rev-parse`, `--abbrev-ref`, `@{u}`),
			"/", 2)[1]
	}

	git := bb()

	switch flag.Arg(0) {
	case "install":
		install(args)
	case "merge":
		merge(args)
	case "gitlab":
		gitlab_req(args, flag.Arg(1))
	case "", "create":
		git.create(args)
	default:
		log.Panic("error")
	}

}
