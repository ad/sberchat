package chat

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

type Client struct {
	client   *http.Client
	config   *Config
	token    *string
	exiresAt *int64
}

type Config struct {
	AuthUrl      string
	BaseUrl      string
	ClientId     string
	ClientSecret string
	Scope        string
	Insecure     bool
}

type OAuthResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

type ModelListResponse struct {
	Models []Model `json:"data"`
	Type   string  `json:"object"`
}

type Model struct {
	Id      string `json:"id"`
	Type    string `json:"object"`
	OwnedBy string `json:"owned_by"`
}

type ChatRequest struct {
	Model             string    `json:"model"`
	Messages          []Message `json:"messages"`
	Temperature       *float64  `json:"temperature"`
	TopP              *float64  `json:"top_p"`
	N                 *int64    `json:"n"`
	Stream            *bool     `json:"stream"`
	MaxTokens         int64     `json:"max_tokens"`
	RepetitionPenalty *float64  `json:"repetition_penalty"`
	UpdateInterval    *int64    `json:"update_interval"`
}

type ChatResponse struct {
	Model   string   `json:"model"`
	Created int64    `json:"created"`
	Method  string   `json:"object"`
	Choices []Choice `json:"choices"`
	Usage   Usage    `json:"usage"`
}

type Choice struct {
	Index        int64  `json:"index"`
	FinishReason string `json:"finish_reason"`
	Message      Message
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Usage struct {
	PromptTokens     int64 `json:"prompt_tokens"`
	CompletionTokens int64 `json:"completion_tokens"`
	TotalTokens      int64 `json:"total_tokens"`
}

// NewInsecureClient creates a new GigaChat client with InsecureSkipVerify because GigaChat uses a weird certificate authority.
func NewInsecureClient(clientId string, clientSecret string) (*Client, error) {
	var conf = &Config{
		AuthUrl:      AuthUrl,
		BaseUrl:      BaseUrl,
		ClientId:     clientId,
		ClientSecret: clientSecret,
		Scope:        ScopeApiIndividual,
		Insecure:     true,
	}
	return NewClientWithConfig(conf)
}

// NewClientWithConfig creates a new GigaChat client with the specified configuration.
func NewClientWithConfig(config *Config) (*Client, error) {
	var client *http.Client
	if config.Insecure {
		customTransport := http.DefaultTransport.(*http.Transport).Clone()
		customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
		client = &http.Client{Transport: customTransport}
	} else {
		client = &http.Client{}
	}

	return &Client{
		client: client,
		config: config,
	}, nil
}

func (c *Client) Auth() error {
	return c.AuthWithContext(context.Background())
}

func (c *Client) AuthWithContext(ctx context.Context) error {
	if c.token != nil {
		return nil
	}

	payload := strings.NewReader("scope=" + c.config.Scope)
	req, err := http.NewRequestWithContext(ctx, "POST", c.config.AuthUrl+OAuthPath, payload)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Add("RqUID", uuid.NewString())
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.config.ClientId+":"+c.config.ClientSecret)))

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d %v", resp.StatusCode, resp.Status)
	}

	var oauth OAuthResponse
	err = json.NewDecoder(resp.Body).Decode(&oauth)
	if err != nil {
		return err
	}

	c.token = &oauth.AccessToken
	c.exiresAt = &oauth.ExpiresAt

	return nil
}

func (c *Client) sendRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *c.token))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		var errMessage interface{}
		if err := json.NewDecoder(res.Body).Decode(&errMessage); err != nil {
			return nil, err
		}

		return nil, fmt.Errorf("GigaCode API request failed: status Code: %d %s %s Message: %+v", res.StatusCode, res.Status, res.Request.URL, errMessage)
	}

	return res, nil
}

func (c *Client) Model(model string) (*Model, error) {
	return c.ModelWithContext(context.Background(), model)
}

func (c *Client) ModelWithContext(ctx context.Context, model string) (*Model, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ModelsPath+"/"+model, nil)
	if err != nil {
		return nil, err
	}

	res, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var modelResponse Model
	if err := json.NewDecoder(res.Body).Decode(&modelResponse); err != nil {
		return nil, err
	}

	return &modelResponse, nil
}

func (c *Client) Chat(in *ChatRequest) (*ChatResponse, error) {
	return c.ChatWithContext(context.Background(), in)
}

func (c *Client) ChatWithContext(ctx context.Context, in *ChatRequest) (*ChatResponse, error) {

	reqBytes, _ := json.Marshal(in)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseUrl+ChatPath, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, err
	}

	res, err := c.sendRequest(ctx, req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var chatResponse ChatResponse
	if err := json.NewDecoder(res.Body).Decode(&chatResponse); err != nil {
		return nil, err
	}

	return &chatResponse, nil
}
