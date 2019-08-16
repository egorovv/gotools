package main

// Forward from local port 9000 to remote port 9999

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"encoding/binary"

	"golang.org/x/crypto/ssh"
)

func ssh_cb(password string) ssh.KeyboardInteractiveChallenge {
	return func(user, instruction string, questions []string, echos []bool) (answers []string, err error) {
		answers = []string{}
		err = nil
		for _, _ = range questions {
			answers = append(answers, password)
		}
		return
	}
}

func ssh_run(c *ssh.Client, prog string, args ...string) error {
	session, err := c.NewSession()
	if err != nil {
		panic("Failed to create session: " + err.Error())
	}
	defer session.Close()

	var o bytes.Buffer
	session.Stdout = &o
	session.Stderr = &o

	cmd := fmt.Sprintf("%s %s ", prog, strings.Join(args, ` `))

	log.Printf("running %s", cmd)
	err = session.Run(cmd)
	fmt.Println(o.String())
	return err
}

func merge(x *map[string]interface{}, y *map[string]interface{}) {

	for k, v := range *y {
		z, ok := (*x)[k]
		if !ok {
			(*x)[k] = v
			continue
		}

		switch z := z.(type) {
		case map[string]interface{}:
			switch v := v.(type) {
			case map[string]interface{}:
				x := map[string]interface{}(z)
				y := map[string]interface{}(v)
				merge(&x, &y)
				continue
				break
			default:
				(*x)[k] = v
				break
			}
		case []interface{}:
			// array assign
			if v, ok := v.([]interface{}); ok {
				for i, _ := range v {
					if i >= len(z) {
						// append the rest
						z = append(z, v[i:]...)
						break
					}
					if x, ok := z[i].(map[string]interface{}); ok {
						if y, ok := v[i].(map[string]interface{}); ok {
							merge(&x, &y)
							z[i] = x
							continue
						}
					}
					z[i] = v[i]
				}
				(*x)[k] = z
			} else {
				(*x)[k] = v
			}
		default:
			(*x)[k] = v
			break
		}

	}
}

func LinePosFromOff(r io.Reader, off int64) (lineno int, pos int64, err error) {
	br := bufio.NewReader(r)
	lineno = 1
	pos = off - 1

	var line []byte
	var trunc bool

	for {
		line, trunc, err = br.ReadLine()
		if err != nil {
			return
		}

		if int64(len(line)) > pos {
			return
		}

		pos -= int64(len(line) + 1)

		if !trunc {
			lineno += 1
		}
	}
	return
}

func json_error(r io.Reader, err error) error {
	if serr, ok := err.(*json.SyntaxError); ok {
		line, col, _ := LinePosFromOff(r, serr.Offset)
		return fmt.Errorf("%d:%d: %v", line, col, err)
	}
	return err
}

func load_json(fn string, val interface{}) {
	text, err := ioutil.ReadFile(fn)
	if err != nil {
		log.Panicf("%s", err)
	}
	err = json.NewDecoder(strings.NewReader(string(text))).Decode(val)
	if err != nil {
		log.Panicf("%s:%s", fn,
			json_error(strings.NewReader(string(text)), err))
	}
}

type JsonRpcReq struct {
	Params struct {
		Config map[string]interface{} `json:"config"`
	} `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Id      int    `json:"id"`
}

type JsonRpcReqGet struct {
	Params struct {
		Modules []string `json:"modules"`
	} `json:"params"`
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Id      int    `json:"id"`
}

type JsonRpcRespGet struct {
	Jsonrpc string `json:"jsonrpc"`
	Method  string `json:"method"`
	Id      int    `json:"id"`
	Result  []struct {
		SchemaVersion string                 `json:"schemaVersion"`
		Version       string                 `json:"version"`
		Module        string                 `json:"module"`
		Data          map[string]interface{} `json:"data"`
	} `json:"result"`
}

func rpc_write(c io.Writer, rpc []byte) {
	var data [4]byte

	binary.BigEndian.PutUint32(data[:], uint32(len(rpc)))
	c.Write(data[:])
	c.Write(rpc)
}

func rpc_read(c io.Reader) []byte {
	var data [4]byte

	c.Read(data[:])
	sz := binary.BigEndian.Uint32(data[:])

	resp := make([]byte, sz)
	_, err := c.Read(resp)
	if err != nil {
		log.Fatalf("error: %s", err)
	}
	return resp
}

func main() {

	var args struct {
		host     string
		user     string
		password string
		file     string
		set      string
		verbose  bool
	}

	flag.StringVar(&args.host, "host", "52.175.205.196", "target host")
	flag.StringVar(&args.user, "user", "root", "ssh user")
	flag.StringVar(&args.password, "passwd", "velocloud", "ssh password")
	flag.StringVar(&args.file, "file", "", "rpc file")
	flag.StringVar(&args.set, "json", "", "set specific value {\"foo\":\"bar\"}")
	flag.BoolVar(&args.verbose, "verbose", false, "verbose")

	flag.Parse()

	data := map[string]interface{}{}
	if args.file != "" {
		log.Printf("reading %s", args.file)
		load_json(args.file, &data)
	}

	if args.set != "" {
		val := map[string]interface{}{}
		err := json.NewDecoder(strings.NewReader(args.set)).Decode(&val)
		if err != nil {
			log.Fatalf("%s", err)
		}
		merge(&data, &val)
	}

	if args.verbose {
		str, _ := json.Marshal(&data)
		log.Printf("json: %s", str)
	}

	config := &ssh.ClientConfig{
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		User:            args.user,
		Auth: []ssh.AuthMethod{
			ssh.Password(args.password),
			ssh_cb(args.password),
		},
	}

	idRsaBuf, err := ioutil.ReadFile(
		filepath.Join(os.Getenv("HOME"), "/.ssh/id_rsa"))

	var idRsa ssh.Signer
	if err == nil {
		idRsa, err = ssh.ParsePrivateKey(idRsaBuf)
	}

	if err == nil {
		config.Auth = append(config.Auth, ssh.PublicKeys(idRsa))
	}

	addr := args.host
	if !strings.Contains(addr, ":") {
		addr = addr + ":22"
	}

	log.Printf("connecting %s", addr)
	c, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		log.Fatalf("ssh.Dial failed: %s", err)
	}

	log.Printf("port forwarding")
	sshConn, err := c.Dial("tcp", "localhost:5002")

	if err != nil {
		log.Printf("%s bootstrapping", err)
		ssh_run(c, "sed -i 's/AllowTcpForwarding no/AllowTcpForwarding yes/g' /etc/ssh/sshd_config")
		ssh_run(c, "/etc/init.d/sshd restart")
		c.Close()

		time.Sleep(time.Second)
		c, err = ssh.Dial("tcp", addr, config)
		if err != nil {
			log.Fatalf("ssh.Dial failed: %s", err)
		}
		log.Printf("port forwarding")
		sshConn, err = c.Dial("tcp", "localhost:5002")
	}

	req1 := JsonRpcReqGet{
		Jsonrpc: "2.0",
		Method:  "getConfiguration",
		Id:      1,
	}
	req1.Params.Modules = []string{"deviceSettings"}
	rpc, _ := json.Marshal(&req1)
	rpc_write(sshConn, rpc)
	resp := rpc_read(sshConn)

	conf := JsonRpcRespGet{}
	json.Unmarshal(resp, &conf)

	sshConn.Close()

	if len(conf.Result) < 1 {
		log.Panicf("bad response: %s", resp)
	}
	if args.verbose {
		log.Printf("response: %s", resp)
	}

	log.Printf("reopening port forwarding")
	sshConn, err = c.Dial("tcp", "localhost:5002")

	req := JsonRpcReq{
		Jsonrpc: "2.0",
		Method:  "updateConfiguration",
		Id:      1,
	}

	settings := conf.Result[0]
	merge(&settings.Data, &data)

	req.Params.Config = map[string]interface{}{
		settings.Module: settings,
	}

	rpc, _ = json.MarshalIndent(&req.Params.Config, "", "\t")

	rpc, _ = json.Marshal(&req)

	log.Printf("sending update")
	rpc_write(sshConn, rpc)

	log.Printf("getting response")
	resp = rpc_read(sshConn)

	if args.verbose {
		log.Printf("response: %s", resp)
	}

}
