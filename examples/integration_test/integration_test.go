package integration_test

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"

	"github.com/lucap9056/magic-conch-shell/core/v2/structs"
	"github.com/lucap9056/magic-conch-shell/core/v2/structs/languages"
	"github.com/stretchr/testify/require"
)

const (
	gatewayURL  = "https://localhost"
	caCertFile  = "../certs/gatewayCA.cert"
	testImgFile = "test.png"
)

var (
	httpClient *http.Client
	noRedirect *http.Client
)

func TestMain(m *testing.M) {
	caPEM, err := os.ReadFile(caCertFile)
	if err != nil {
		log.Fatalf("read CA cert: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caPEM) {
		log.Fatal("failed to append CA cert")
	}
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{RootCAs: pool},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if addr == "gateway:443" {
				addr = "localhost:443"
			}
			dialer := &net.Dialer{}
			return dialer.DialContext(ctx, network, addr)
		},
	}
	httpClient = &http.Client{Transport: tr}
	noRedirect = &http.Client{
		Transport: tr,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	os.Exit(m.Run())
}

type apiResponse[T any] struct {
	Success bool `json:"success"`
	Message T    `json:"message"`
}

type loginResponse struct {
	Verifier string `json:"verifier"`
	URL      string `json:"url"`
}

type tokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type uploadResult struct {
	Key      string `json:"key"`
	MimeType string `json:"mime_type"`
}

func TestFlow(t *testing.T) {
	t.Logf("gateway: %s", gatewayURL)

	t.Log("=== step 1: health check ===")
	func() {
		resp, err := httpClient.Get(gatewayURL + "/health")
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		t.Logf("GET /health → %d OK", resp.StatusCode)
		bodyBytes, err := io.ReadAll(resp.Body)
		require.Equal(t, "OK", string(bodyBytes))
	}()

	t.Log("=== step 2: unauthenticated request → 401 ===")
	func() {
		body, err := json.Marshal(&structs.Request{
			Language: languages.EN,
			CurrentMessage: &structs.PromptMessage{
				Parts: []*structs.PromptPart{structs.NewTextPart("A or B")},
			},
		})
		require.NoError(t, err)
		resp, err := httpClient.Post(gatewayURL+"/api/", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			body, _ := io.ReadAll(resp.Body)
			t.Log(string(body))
		}
		require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		result := &apiResponse[string]{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(result))
		require.False(t, result.Success)
		t.Logf("POST /api/ (no token) → %d", resp.StatusCode)
	}()

	t.Log("=== step 3: login → PKCE authorize URL ===")
	loginResp := func() *loginResponse {
		resp, err := httpClient.Get(gatewayURL + "/auth/login")
		require.NoError(t, err)
		defer resp.Body.Close()
		result := &apiResponse[loginResponse]{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(result))
		require.True(t, result.Success)
		t.Logf("authorize URL: %s", result.Message.URL)
		return &result.Message
	}()

	t.Log("=== step 4: browser GET authorize → code ===")
	type authCode struct{ code, state string }
	authRes := func() authCode {
		t.Helper()
		resp, err := noRedirect.Get(loginResp.URL)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusFound, resp.StatusCode)

		location := resp.Header.Get("Location")
		require.NotEmpty(t, location, "no Location header in authorization response")
		redirectURL, err := url.Parse(location)
		require.NoError(t, err)

		code := redirectURL.Query().Get("code")
		state := redirectURL.Query().Get("state")
		require.NotEmpty(t, code, "no code in redirect URL: %s", location)
		t.Logf("redirect → %s  code=%s…  state=%s", redirectURL.Host+redirectURL.Path, code[:8], state)
		return authCode{code: code, state: state}
	}()

	t.Log("=== step 5: /auth/callback → JWT ===")
	tokenRes := func() *tokenPair {
		t.Helper()
		q := url.Values{}
		q.Set("code", authRes.code)
		q.Set("state", authRes.state)

		resp, err := httpClient.Get(gatewayURL + "/auth/callback?" + q.Encode())
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("callback returned %d: %s", resp.StatusCode, bodyBytes)
		}

		result := &apiResponse[tokenPair]{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(result))
		require.True(t, result.Success)
		require.NotEmpty(t, result.Message.AccessToken)
		require.NotEmpty(t, result.Message.RefreshToken)
		t.Logf("access_token=%s…  refresh_token=%s…",
			result.Message.AccessToken[:8], result.Message.RefreshToken[:8])
		return &result.Message
	}()

	t.Log("=== step 6: upload image ===")
	imgRes := func() *uploadResult {
		t.Helper()

		fileBytes, err := os.ReadFile(testImgFile)
		require.NoError(t, err)

		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		part, err := w.CreateFormFile("file", testImgFile)
		require.NoError(t, err)
		_, err = part.Write(fileBytes)
		require.NoError(t, err)
		require.NoError(t, w.Close())

		httpReq, err := http.NewRequest(http.MethodPost, gatewayURL+"/api/upload", &buf)
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", w.FormDataContentType())
		httpReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)

		resp, err := httpClient.Do(httpReq)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST /api/upload returned %d: %s", resp.StatusCode, bodyBytes)
		}

		result := &apiResponse[uploadResult]{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(result))
		require.True(t, result.Success)
		require.NotEmpty(t, result.Message.Key)
		require.NotEmpty(t, result.Message.MimeType)
		t.Logf("uploaded %s → key=%s  mime_type=%s", testImgFile, result.Message.Key, result.Message.MimeType)
		return &result.Message
	}()

	t.Log("=== step 7: authenticated chat request ===")
	func() {
		t.Helper()
		body, err := json.Marshal(&structs.Request{
			Language: languages.EN,
			CurrentMessage: &structs.PromptMessage{
				Parts: []*structs.PromptPart{
					structs.NewImagePart(imgRes.MimeType, imgRes.Key),
					structs.NewTextPart("Which one?"),
				},
			},
		})
		require.NoError(t, err)

		httpReq, err := http.NewRequest(http.MethodPost, gatewayURL+"/api/", bytes.NewReader(body))
		require.NoError(t, err)
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+tokenRes.AccessToken)

		resp, err := httpClient.Do(httpReq)
		require.NoError(t, err)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyBytes, _ := io.ReadAll(resp.Body)
			t.Fatalf("POST /api/ (with token) returned %d: %s", resp.StatusCode, bodyBytes)
		}

		result := &apiResponse[string]{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(result))
		require.True(t, result.Success)
		t.Logf("AI response: %s", result.Message)
	}()
}
