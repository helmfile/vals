package httpjson

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/antchfx/jsonquery"
	"github.com/helmfile/vals/pkg/api"
	"github.com/helmfile/vals/pkg/log"
)

type provider struct {
	// Keeping track of httpjson services since we need a service per url
	protocol   string
	log        *log.Logger
	docs       map[string]*jsonquery.Node
	floatAsInt bool
}

func New(l *log.Logger, cfg api.StaticConfig) *provider {
	p := &provider{
		log: l,
	}

	// Should the protocol be insecure i.e. http
	insecureArg := cfg.String("insecure")
	p.protocol = "https"
	if insecureArg == "true" {
		p.protocol = "http"
	}

	// By default JSON will return large integers as float64
	floatAsIntArg := cfg.String("floatAsInt")
	p.floatAsInt = false
	if floatAsIntArg == "true" {
		p.floatAsInt = true
	}

	// Initialize docs map to store the json object for use multiple times
	if len(p.docs) == 0 {
		p.docs = make(map[string]*jsonquery.Node)
	}

	return p
}

func GetXpathFromUri(uri string) (xpath string, err error) {
	found := strings.Split(uri, "mode=singleparam#")[1]
	found = strings.Split(found, "&")[0]
	xpath = strings.TrimPrefix(found, "/")

	return xpath, nil
}

func getUrlFromUri(uri string, protocol string) (string, error) {
	uriParts := strings.Split(uri, "?")
	if len(uriParts) < 2 {
		return "", fmt.Errorf("error getting url from uri: %v, ensure xpath singleparam is set as a query parameter", uri)
	}
	url := strings.Replace(uriParts[0], "httpjson", protocol, 1)

	return url, nil
}

func (p *provider) GetJsonDoc(url string) error {
	if _, ok := p.docs[url]; !ok {
		doc, err := jsonquery.LoadURL(url)
		if err != nil {
			return fmt.Errorf("error fetching json document at %v: %v", url, err)
		}
		p.docs[url] = doc
	}

	return nil
}

func (p *provider) GetString(uri string) (string, error) {
	url, err := getUrlFromUri(uri, p.protocol)
	if err != nil {
		return "", err
	}
	err = p.GetJsonDoc(url)
	if err != nil {
		return "", err
	}
	xpathQuery, err := GetXpathFromUri(uri)
	if err != nil {
		return "", err
	}

	returnValue := ""
	var values []string
	node, err := jsonquery.Query(p.docs[url], xpathQuery)
	if err != nil || node == nil {
		return "", fmt.Errorf("unable to query doc for value with xpath query using %v", uri)
	}

	if node.FirstChild.Data != node.LastChild.Data {
		return "", fmt.Errorf("location %v has child nodes at %v, please use a more granular query", xpathQuery, url)
	}

	childNodesLength := countChildNodes(node)

	if childNodesLength > 1 {
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			values = append(values, child.Value().(string))
		}
		returnValue = strings.Join(values, ",")

	} else {
		returnValue = node.FirstChild.Value().(string)
	}

	if p.floatAsInt {
		intValue, err := strconv.ParseFloat(returnValue, 64)
		if err != nil {
			return "", fmt.Errorf("unable to convert possible float to int for value: %v", returnValue)
		}
		returnValue = fmt.Sprintf("%.0f", intValue)
	}

	return returnValue, nil
}

func countChildNodes(node *jsonquery.Node) int {
	count := 0
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		count++
	}
	return count
}

func (p *provider) GetStringMap(key string) (map[string]interface{}, error) {
	return nil, fmt.Errorf("we should not be in the GetStringMap method")
}
