package redaction

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/matrix-org/gomatrixserverlib/spec"
)

type Client struct {
	homeserverUrl string
	accessToken   string
	userId        string
	domain        string
}

type whoamiLimitedResponse struct {
	UserId string `json:"user_id"`
}

func NewClient(homeserverUrl string, accessToken string) (*Client, error) {
	c := &Client{
		homeserverUrl: homeserverUrl,
		accessToken:   accessToken,
	}

	whoami := whoamiLimitedResponse{}
	err := c.doJsonRequest("GET", "/_matrix/client/v3/account/whoami", nil, &whoami)
	if err != nil {
		return nil, err
	}
	c.userId = whoami.UserId
	parsedId, err := spec.NewUserID(c.userId, true)
	if err != nil {
		return nil, err
	}
	c.domain = string(parsedId.Domain())
	log.Println("Client user ID is", c.userId, " (homeserver URL is", c.homeserverUrl, " and domain is", c.domain, ")")

	return c, nil
}

func (c *Client) RedactEvent(roomId string, eventId string) error {
	body := map[string]any{
		"reason": "Detected as probable spam",
	}
	consistentTxnId := fmt.Sprintf("policyserv_%s", eventId)
	return c.doJsonRequest("PUT", fmt.Sprintf("/_matrix/client/v3/rooms/%s/redact/%s/%s", url.PathEscape(roomId), url.PathEscape(eventId), url.PathEscape(consistentTxnId)), body, nil)
}

func (c *Client) doJsonRequest(method string, path string, body any, resp any) error {
	req, err := http.NewRequest(method, c.homeserverUrl+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Body = io.NopCloser(bytes.NewReader(b))
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return fmt.Errorf("unexpected status code %d", res.StatusCode)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}
	if resp != nil {
		err = json.Unmarshal(b, &resp)
		if err != nil {
			return err
		}
	}
	return nil
}
