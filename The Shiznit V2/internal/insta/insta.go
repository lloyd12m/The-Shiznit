package insta

import (
	"automg-go/internal/utils"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrSessionInvalid = errors.New("instagram session is invalid")

func IsSessionInvalid(err error) bool {
	return errors.Is(err, ErrSessionInvalid)
}

var GlobalTransport = &http.Transport{
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          1000,
	MaxIdleConnsPerHost:   500,
	MaxConnsPerHost:       1000,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

var GlobalClient = &http.Client{
	Transport: GlobalTransport,
	Timeout:   10 * time.Second,
	CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

type Response struct {
	StatusCode int
	Body       string
	JSON       map[string]interface{}
}

type InstaClient struct {
	SessionID   string
	CSRFToken   string
	Username    string
	FullName    string
	Email       string
	Phone       string
	ExternalURL string
	Bio         string
	Target      string
	Client      *http.Client
	Stats       *Stats
	ProxySelect func() string
	transportMu sync.Mutex
	clients     map[string]*http.Client
}

type CheckerStats struct {
	Checked       int
	WindowChecked int
	Errors        int
	Lock          sync.Mutex
}

type ClaimerStats struct {
	Attempts       int
	WindowAttempts int
	Errors         int
	Success        int
	Lock           sync.Mutex
}

type Stats struct {
	Checker CheckerStats
	Claimer ClaimerStats
}

func NewInstaClient(sessionID string, stats *Stats, proxySelect func() string) *InstaClient {
	jar, _ := cookiejar.New(nil)
	base := GlobalTransport.Clone()
	client := &http.Client{Transport: base, Jar: jar, Timeout: 10 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	instagramURL, _ := url.Parse("https://www.instagram.com/")
	jar.SetCookies(instagramURL, []*http.Cookie{{Name: "sessionid", Value: sessionID, Path: "/"}, {Name: "ds_user_id", Value: userIDFromSession(sessionID), Path: "/"}})
	return &InstaClient{SessionID: sessionID, Client: client, Stats: stats, ProxySelect: proxySelect, clients: make(map[string]*http.Client)}
}

func userIDFromSession(session string) string {
	if i := strings.IndexByte(session, '%'); i >= 0 {
		return session[:i]
	}
	return session
}

func (c *InstaClient) GetHeaders() map[string]string {
	return map[string]string{
		"accept": "*/*", "accept-language": "en-US,en;q=0.9",
		"content-type": "application/x-www-form-urlencoded",
		"origin":       "https://www.instagram.com", "referer": "https://www.instagram.com/accounts/edit/",
		"sec-ch-ua":        `"Google Chrome";v="111", "Not(A:Brand";v="8", "Chromium";v="111"`,
		"sec-ch-ua-mobile": "?0", "sec-ch-ua-platform": `"Windows"`,
		"sec-fetch-dest": "empty", "sec-fetch-mode": "cors", "sec-fetch-site": "same-origin",
		"user-agent":     "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
		"viewport-width": "786", "x-asbd-id": "198387", "x-csrftoken": c.CSRFToken,
		"x-ig-app-id": "936619743392459", "x-ig-www-claim": "hmac.AR2H2vXUvQPS5uwDBHWhRCGmvFXDC9bmwl",
		"x-instagram-ajax": "1007164808", "x-requested-with": "XMLHttpRequest",
	}
}

func (c *InstaClient) GetUserID() string {
	parts := strings.Split(c.SessionID, "%")
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func (c *InstaClient) requestClient(proxy string) *http.Client {
	if proxy == "" {
		return c.Client
	}
	c.transportMu.Lock()
	defer c.transportMu.Unlock()
	if cached := c.clients[proxy]; cached != nil {
		return cached
	}
	if !strings.HasPrefix(proxy, "http://") && !strings.HasPrefix(proxy, "https://") {
		proxy = "http://" + proxy
	}
	proxyURL, err := url.Parse(proxy)
	if err != nil {
		return c.Client
	}
	base, ok := c.Client.Transport.(*http.Transport)
	if !ok || base == nil {
		base = GlobalTransport
	}
	transport := base.Clone()
	transport.Proxy = http.ProxyURL(proxyURL)
	client := &http.Client{Transport: transport, Jar: c.Client.Jar, Timeout: c.Client.Timeout, CheckRedirect: c.Client.CheckRedirect}
	c.clients[proxy] = client
	return client
}

func (c *InstaClient) SendRequest(method, targetURL string, data string) (*Response, error) {
	// The reference chooses one proxy for the logical request and keeps retrying
	// on that proxy until a response is received (status_code != 0).
	proxy := ""
	if c.ProxySelect != nil {
		proxy = c.ProxySelect()
	}
	client := c.requestClient(proxy)

	for {
		var requestBody io.Reader
		if data != "" {
			requestBody = strings.NewReader(data)
		}
		req, err := http.NewRequest(method, targetURL, requestBody)
		if err != nil {
			return nil, err
		}
		for k, v := range c.GetHeaders() {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		c.Stats.Claimer.Lock.Lock()
		c.Stats.Claimer.Attempts++
		c.Stats.Claimer.WindowAttempts++
		if err != nil {
			c.Stats.Claimer.Errors++
		}
		c.Stats.Claimer.Lock.Unlock()
		if err != nil {
			continue
		}

		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			c.Stats.Claimer.Lock.Lock()
			c.Stats.Claimer.Errors++
			c.Stats.Claimer.Lock.Unlock()
			continue
		}
		var jsonResp map[string]interface{}
		_ = json.Unmarshal(body, &jsonResp)
		return &Response{StatusCode: resp.StatusCode, Body: string(body), JSON: jsonResp}, nil
	}
}

func (c *InstaClient) FetchSharedData() error {
	resp, err := c.SendRequest("GET", "https://www.instagram.com/data/shared_data/", "")
	if err != nil {
		return err
	}
	if config, ok := resp.JSON["config"].(map[string]interface{}); ok {
		if csrf, ok := config["csrf_token"].(string); ok {
			c.CSRFToken = csrf
			return nil
		}
	}
	return fmt.Errorf("failed to get CSRF token")
}

func (c *InstaClient) ProfileInfo() error {
	resp, err := c.SendRequest("GET", "https://www.instagram.com/api/v1/accounts/edit/web_form_data/", "")
	if err != nil {
		return err
	}
	if formData, ok := resp.JSON["form_data"].(map[string]interface{}); ok {
		c.FullName, _ = formData["first_name"].(string)
		c.Email, _ = formData["email"].(string)
		c.Username, _ = formData["username"].(string)
		c.Phone, _ = formData["phone_number"].(string)
		c.ExternalURL, _ = formData["external_url"].(string)
		c.Bio, _ = formData["biography"].(string)
		return nil
	}
	return fmt.Errorf("failed to get profile info")
}

func (c *InstaClient) ClaimUser(target string) (bool, error) {
	c.Target = target
	data := url.Values{}
	data.Set("username", target)
	data.Set("first_name", c.FullName)
	data.Set("email", c.Email)
	data.Set("phone_number", c.Phone)
	data.Set("biography", c.Bio)
	data.Set("external_url", c.ExternalURL)
	data.Set("chaining_enabled", "on")

	resp, err := c.SendRequest("POST", "https://www.instagram.com/api/v1/web/accounts/edit/", data.Encode())
	if err != nil {
		return false, err
	}
	// Delete only when Instagram explicitly indicates that authentication is invalid.
	bodyLower := strings.ToLower(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized ||
		(resp.StatusCode == http.StatusForbidden && (strings.Contains(bodyLower, "login_required") ||
			strings.Contains(bodyLower, "checkpoint_required") ||
			strings.Contains(bodyLower, "challenge_required") ||
			strings.Contains(bodyLower, "user_has_logged_out") ||
			strings.Contains(bodyLower, "session_invalid"))) {
		return false, fmt.Errorf("%w: HTTP %d", ErrSessionInvalid, resp.StatusCode)
	}

	// Match the reference: success requires HTTP 200 and a parsed JSON body.
	if resp.StatusCode == http.StatusOK && resp.JSON != nil {
		c.Stats.Claimer.Lock.Lock()
		c.Stats.Claimer.Success++
		c.Stats.Claimer.Lock.Unlock()
		return true, nil
	}
	return false, nil
}

func (c *InstaClient) RemoveSelf(success bool) {
	if success {
		if err := utils.SaveClaim(c.Target, c.SessionID); err != nil {
			return
		}
		_ = utils.SendClaimToTelegram(fmt.Sprintf("%s: %s", c.Target, c.SessionID))
		_ = utils.RemoveFromFile("sessions.txt", c.SessionID)
		_ = utils.RemoveFromFile("users.txt", c.Target)
	}
}
