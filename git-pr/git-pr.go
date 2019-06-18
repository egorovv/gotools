package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"gotools/util"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"
)

type Args struct {
	Owner    string `json:"owner,omitempty"`
	Repo     string `json:"repo,omitempty"`
	User     string `json:"user,omitempty"`
	Password string `json:"password,omitempty"`
	Branch   string `json:"branch,omitempty"`
	Upstream string `json:"upstream,omitempty"`
	Team     string `json:"team,omitempty"`
	Label    string `json:"label,omitempty"`
	Remove   bool   `json:"remove,omitempty"`
	Verbose  bool   `json:"verbose"`
	remote   string
	args     []string
}

type User struct {
	Name string
	Id   string
}

type Git interface {
	create()
	merge()
	test()
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

func prepare(args Args, m []User) (fn string) {

	text := util.Sh(`git`, `log`, `--reverse`, `@{u}..`, `--pretty= - %B`)
	text = text[2:]

	t, err := template.New("PR").Parse(`#
# Edit pull request title and description, remove starting '!'.
#
# All lines starting with # will be removed. Of the remaining the first
# line will be used as a title and the rest as description.
# Subject starting with ! will cause the oeration to abort.
#
# User: {{ .Args.User }}
# Branch: {{ .Args.Branch }}
# Upstream: {{ .Args.Upstream }}
# Owner/Repo: {{ .Args.Owner }}/{{ .Args.Repo }}
# Label: {{ .Args.Label }}
# Label: {{ .Args.Label }}
# Remove: {{ .Args.Remove }}
#
{{.Body}}

@{{.Args.Team}}

# This PR will be sent to the following recipients:
{{range .Members }}#Review-By: {{ .Id }} <{{ .Name }}>
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
	fn = f.Name()

	err = t.Execute(f, data)
	f.Close()
	return
}

func edit(fn string) (subj, desc string) {
	editor, ok := os.LookupEnv("GIT_EDITOR")
	if !ok {
		editor = "/usr/bin/editor"
	}
	cmd := exec.Command(editor, fn)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	err := cmd.Run()
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
	util.Sh(`git`, `config`, `--global`, `alias.pr`, `!`+exe)
	util.SaveGitFlags("pr")
}

func git_detect(args *Args) {

	upstream := strings.SplitN(
		util.Sh(`git`, `rev-parse`, `--abbrev-ref`, `@{u}`), "/", 2)

	remote := strings.Split(
		util.Sh(`git`, `remote`, `get-url`, upstream[0]), ":")

	repo := strings.SplitN(remote[len(remote)-1], "/", 2)

	args.remote = upstream[0]
	args.Upstream = upstream[1]
	args.Owner = repo[0]
	args.Repo = strings.TrimSuffix(repo[1], ".git")
}

func main() {
	args := Args{
		Team:   "velocloud/dp",
		Branch: "{{.Branch}}",
	}

	git_detect(&args)

	util.GetFlags(&args, "pr")

	t, _ := template.New("pr").Parse(args.Branch)
	b := bytes.NewBufferString("")
	args.Branch = util.Sh(`git`, `symbolic-ref`, `--short`, `HEAD`)
	t.Execute(b, args)
	args.Branch = b.String()

	if len(flag.Args()) > 0 {
		args.args = flag.Args()[1:]
	}
	git := NewGitlab(args)

	switch flag.Arg(0) {
	case "install":
		install(args)
	case "merge":
		git.merge()
	case "test":
		git.test()
	case "", "create":
		util.Sh(`git`, `push`, `-f`, args.remote, fmt.Sprintf("HEAD:%s", args.Branch))
		git.create()
	default:
		log.Panic(flag.Args())
	}

}
