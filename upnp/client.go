package upnp

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/cookiejar"
	"strings"
)

// Client is used to comunicate with remote device
type Client struct {
	IP            string
	url           string
	client        http.Client
	controlsTable map[string]string
}

// NewClient instantiates new device remote controller client
func NewClient(ip string) Client {
	cookieJar, _ := cookiejar.New(nil)
	return Client{
		IP:            ip,
		url:           strings.Replace("http://{ip}/sony/{endpoint}", "{ip}", ip, -1),
		client:        http.Client{Jar: cookieJar},
		controlsTable: make(map[string]string),
	}
}

var authRequestBody = []byte(`{
    "id": 1,
    "version": "1.0",
    "method": "actRegister",
    "params": [
        {
            "clientid": "GoRemoteController",
            "nickname": "go-remote-controller",
            "level": "private"
        },
        [{
            "value": "yes",
            "function": "WOL"
        }]
    ]
}`)

// Handshake with remote device
func (c Client) Handshake() (response *http.Response, e error) {
	request, _ := c.newJSONRequest("POST", "accessControl", authRequestBody)
	response, e = c.client.Do(request)
	return
}

// Authorize with remote device providing an "Authorization: Basic *" header
func (c Client) Authorize(pin string) (response *http.Response, e error) {
	request, _ := c.newJSONRequest("POST", "accessControl", authRequestBody)
	request.Header.Set("authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(":"+pin)))
	response, e = c.client.Do(request)
	return
}

type envelope struct {
	ID     uint64            `json:"id"`
	Result []json.RawMessage `json:"result"`
}
type control struct {
	Name  string
	Value string
}
type controls []control

var constrolsRequestBody = []byte(`{
    "id": 2,
    "version": "1.0",
    "method": "getRemoteControllerInfo",
    "params": []
}`)

// RequestControlsList gets UPnP controller description from remote device
func (c Client) RequestControlsList() (response *http.Response, e error) {
	request, _ := c.newJSONRequest("POST", "system", constrolsRequestBody)
	response, e = c.client.Do(request)
	if e == nil {
		envelope := new(envelope)
		json.NewDecoder(response.Body).Decode(envelope)
		if 0 == len(envelope.Result) {
			e = errors.New("Could not retrieve UPnP control list from device.")
		} else {
			var controls controls
			json.Unmarshal(envelope.Result[1], &controls)
			for _, control := range controls {
				c.controlsTable[string(control.Name)] = control.Value
			}
		}
	}
	return
}

var commandRequestBody = `<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
    <s:Body>
        <u:X_SendIRCC xmlns:u="urn:schemas-sony-com:service:IRCC:1">
            <IRCCCode>{IRCC}</IRCCCode>
        </u:X_SendIRCC>
    </s:Body>
</s:Envelope>`

// SendCommand sends a signal to activate a remote device function
func (c Client) SendCommand(command string) (signal string, ok bool, response *http.Response, e error) {
	signal, ok = c.controlsTable[command]
    response, e = c.SendIRCC(signal)
	return
}

// SendIRCC sends a raw IRCC signal to device
func (c Client) SendIRCC(IRCC string) (*http.Response, error) {
    body := strings.Replace(commandRequestBody, "{IRCC}", IRCC, -1)
    request, _ := c.newSOAPRequest("POST", "IRCC", []byte(body))
    return c.client.Do(request)
}

func (c Client) newJSONRequest(method string, endpoint string, body []byte) (request *http.Request, e error) {
	request, e = http.NewRequest(method, c.getURL(endpoint), bytes.NewBuffer(body))
	request.Header.Set("content-type", "application/json")
	return
}

func (c Client) newSOAPRequest(method string, endpoint string, body []byte) (request *http.Request, e error) {
	request, e = http.NewRequest(method, c.getURL(endpoint), bytes.NewBuffer(body))
	request.Header.Set("content-type", "text/xml; charset=UTF-8")
	request.Header.Set("soapaction", "urn:schemas-sony-com:service:IRCC:1#X_SendIRCC")
	return
}

func (c Client) getURL(endpoint string) string {
	return strings.Replace(c.url, "{endpoint}", endpoint, -1)
}