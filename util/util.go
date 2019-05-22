package util

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path"
	"reflect"
	"strings"
	"unsafe"
)

func LoadJsonFlags(a interface{}, fn string) {
	user, err := user.Current()
	if err != nil {
		return
	}
	path := path.Join(user.HomeDir, fn)
	fmt.Printf("%s\n", path)

	f := flag.Lookup("user")
	if f != nil {
		f.Value.Set(user.Username)
	}

	if f, err := os.Open(path); err == nil {
		defer f.Close()
		p := json.NewDecoder(f)
		p.Decode(a)
		Dump("json:", a)
	}
}

func LoadGitFlags(s string) {
	git := map[string]string{}
	config := Sh(`git`, `config`, `-l`)
	for _, line := range strings.Split(config, "\n") {
		parts := strings.SplitN(line, `=`, 2)
		git[parts[0]] = parts[1]
	}

	f := func(f *flag.Flag) {
		if val, ok := git[s+`.`+f.Name]; ok {
			flag.Set(f.Name, val)
		}
	}
	flag.VisitAll(f)
}

func SaveGitFlags(s string) {
	git := map[string]string{}
	config := Sh(`git`, `config`, `-l`)
	for _, line := range strings.Split(config, "\n") {
		parts := strings.SplitN(line, `=`, 2)
		git[parts[0]] = parts[1]
	}

	f := func(f *flag.Flag) {
		Sh(`git`, `config`, `--global`,
			s+`.`+f.Name, f.Value.String())
	}
	flag.Visit(f)
}

func ParseFlags(a interface{}) {
	v := reflect.ValueOf(a).Elem().Elem().Elem()
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}
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
		case reflect.Int:
			p := (*int)(unsafe.Pointer(vf.UnsafeAddr()))
			flag.IntVar(p, name, *p, "")
		case reflect.String:
			p := (*string)(unsafe.Pointer(vf.UnsafeAddr()))
			flag.StringVar(p, name, *p, "")
		}

	}
	flag.Parse()
}

func GetFlags(a interface{}, name string) {
	LoadJsonFlags(&a, "."+name)
	ParseFlags(&a)
	LoadGitFlags(name)
}

func Sh(cmd string, arg ...string) string {
	out, err := exec.Command(cmd, arg...).Output()
	if err != nil {
		log.Panicf("%s %s : %s", cmd, arg, err)
	}
	return strings.TrimSpace(string(out))
}

func Dump(prefix string, v interface{}) {
	b, _ := json.MarshalIndent(v, "", "  ")

	fmt.Printf("%s: %s\n", prefix, b)
}

func Unpack(src, dst interface{}) {
	b, err := json.Marshal(src)
	if err != nil {
		log.Panic(err)
	}
	err = json.Unmarshal(b, dst)
	if err != nil {
		log.Panic(err)
	}
}
