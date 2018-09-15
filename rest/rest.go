package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type Rest struct {
	auth    *auth
	url     string
	verbose bool
}

type auth struct {
	app_id, secret string
	user, password string
}

func NewRest(url, user, pass string, verbose bool) *Rest {
	return &Rest{
		url: url,
		auth: &auth{
			user:     user,
			password: pass,
		},
		verbose: verbose,
	}
}

func (c *Rest) request(method string, url string, query url.Values,
	data interface{}) ([]byte, http.Header, error) {

	b, err := json.Marshal(data)
	if err != nil {
		return nil, nil, err
	}
	body := strings.NewReader(string(b))
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Panic(err)
	}
	if len(b) != 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("PRIVATE-TOKEN", c.auth.password)

	if err != nil {
		return nil, nil, err
	}

	if c.auth != nil {
		req.SetBasicAuth(c.auth.user, c.auth.password)
	}

	if query != nil {
		req.URL.RawQuery = query.Encode()
	}

	if c.verbose {
		log.Printf("rest: %s %s <= %s ", method, url, string(b))
	}

	client := new(http.Client)
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	if resp.Body == nil {
		return nil, nil, fmt.Errorf("response body is nil")
	}
	defer resp.Body.Close()

	resBodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	if c.verbose {
		log.Printf("rest: %d => %s %s %s ", url, resp.StatusCode,
			resp.Header.Get("Link"), resBodyBytes)
	}

	if (resp.StatusCode != http.StatusOK) && (resp.StatusCode != http.StatusCreated) {
		return nil, nil, fmt.Errorf(resp.Status)
	}

	return resBodyBytes, resp.Header, nil
}

func (c *Rest) Do(method string, url string, query url.Values,
	data interface{}) ([]map[string]interface{}, error) {

	var ret []map[string]interface{}

	for url != "" {
		res, h, err := c.request(method, url, query, data)

		var result []map[string]interface{}
		err = json.Unmarshal(res, &result)
		if err != nil {
			return nil, err
		}
		ret = append(ret, result...)

		url = ""
		if l, ok := h["Link"]; ok {
			for _, ll := range l {
				for _, rel := range strings.Split(ll, ",") {
					log.Printf("rel = %s\n", rel)
					parts := strings.SplitN(rel, ";", 2)
					if strings.TrimSpace(parts[1]) == `rel="next"` {
						url = strings.TrimSpace(parts[0])
						url = url[1 : len(url)-1]
					}
				}
			}
		}
	}

	return ret, nil
}

func (c *Rest) execute(method string, url string, query url.Values,
	data interface{}) (map[string]interface{}, error) {

	res, _, err := c.request(method, url, query, data)

	var result map[string]interface{}
	err = json.Unmarshal(res, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Rest) executeAll(method string, url string, query url.Values,
	data interface{}) (interface{}, error) {

	result, err := c.execute(method, url, query, data)
	if err != nil {
		return nil, err
	}

	next, ok := result["next"]
	for ok {
		url := next.(string)
		nr, err := c.execute(method, url, nil, data)
		if err != nil {
			return nil, err
		}
		values := result["values"].([]interface{})
		result["values"] = append(values, nr["values"].([]interface{}))
		next, ok = nr["next"]
	}
	return result["values"], nil
}

func (c *Rest) Get(path string, query url.Values, data interface{}) (interface{}, error) {
	return c.executeAll("GET", c.url+path, query, nil)
}

func (c *Rest) Post(path string, data interface{}) (interface{}, error) {
	result, err := c.execute("POST", c.url+path, nil, data)
	if err != nil {
		return nil, err
	}
	if values, ok := result["values"]; ok {
		return values, nil
	}
	return result, nil
}
