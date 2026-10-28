package webservice

import (
	"context"
	"crypto/hmac"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/wangwanjie/gately/internal/thirdauth"
)

// OAuthProvider 是一个自建的第三方登录配置（使用用户自己申请的 OAuth 应用，不经过任何中转服务器）。
type OAuthProvider = thirdauth.Provider

// 第三方登录相关的保留路径
const (
	oauthStart    = authPath + "/oauth/start"
	oauthCallback = authPath + "/oauth/callback"
	oauthFinish   = authPath + "/oauth/finish"
	// oauthNonceCookie 把登录流程绑定到发起登录的浏览器
	oauthNonceCookie = "gately_oauth_nonce"
	stateTTL         = 10 * time.Minute
	ticketTTL        = 2 * time.Minute
)

// signBlob 生成带过期时间的签名数据，用于 state 与回跳票据。
func (a *authenticator) signBlob(kind string, v any, ttl time.Duration) string {
	return thirdauth.Sign(a.secret, kind, v, ttl)
}

func (a *authenticator) verifyBlob(kind, token string, out any) error {
	return thirdauth.Verify(a.secret, kind, token, out)
}

// verifyBlobExp 与 verifyBlob 相同，另外返回过期时间（Unix 秒）。
func (a *authenticator) verifyBlobExp(kind, token string, out any) (int64, error) {
	return thirdauth.VerifyExp(a.secret, kind, token, out)
}

// oauthState 在跳转身份提供方时携带，回调时用于定位原站点。
type oauthState struct {
	Provider string `json:"p"`
	Origin   string `json:"o"` // 原站点的 scheme://host
	Scope    string `json:"s"`
	Next     string `json:"n"`
	Nonce    string `json:"r"`
}

// oauthTicket 由认证域名签发，原站点凭它建立会话。
type oauthTicket struct {
	Identity string `json:"i"`
	// Aliases 为同一账号的其他可匹配标识（如 GitHub 数字 ID）
	Aliases []string `json:"a,omitempty"`
	Host    string   `json:"h"`
	Scope   string   `json:"s"`
	Next    string   `json:"n"`
	Nonce   string   `json:"r"`
}

// oauthEnv 是处理第三方登录需要的全局配置。
type oauthEnv struct {
	providers map[string]OAuthProvider
	// callbackBase 为认证域名的外部地址，例如 https://auth.example.com:4434
	callbackBase string
}

func (e oauthEnv) callbackURL() string {
	return strings.TrimRight(e.callbackBase, "/") + oauthCallback
}

func requestOrigin(r *http.Request) string {
	scheme := "http"
	if requestIsHTTPS(r) {
		scheme = "https"
	}
	return scheme + "://" + r.Host
}

func randomNonce() string {
	return thirdauth.RandomNonce()
}

// identityAllowed 判断身份是否在站点允许列表中，格式见 thirdauth.ParseEntry。
func identityAllowed(s Site, id thirdauth.Identity) bool {
	return thirdauth.Allowed(s.OAuthUsers, id)
}

// handleOAuthStart 在站点域名上发起第三方登录。
func (a *authenticator) handleOAuthStart(w http.ResponseWriter, r *http.Request, s Site, env oauthEnv) {
	pk := r.URL.Query().Get("p")
	p, ok := env.providers[pk]
	if !ok || env.callbackBase == "" {
		http.Error(w, "第三方登录未配置", http.StatusBadRequest)
		return
	}
	nonce := randomNonce()
	state := a.signBlob("state", oauthState{Provider: pk, Origin: requestOrigin(r), Scope: s.scope, Next: safeNext(r.URL.Query().Get("next")), Nonce: nonce}, stateTTL)
	target, err := p.AuthorizeURL(r.Context(), env.callbackURL(), state)
	if err != nil {
		http.Error(w, "无法连接身份提供方: "+err.Error(), http.StatusBadGateway)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: oauthNonceCookie, Value: nonce, Path: authPath, MaxAge: int(stateTTL / time.Second),
		HttpOnly: true, Secure: requestIsHTTPS(r), SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, target, http.StatusFound)
}

// handleOAuthCallback 在认证域名上处理身份提供方的回调，然后带票据跳回原站点。
func (a *authenticator) handleOAuthCallback(w http.ResponseWriter, r *http.Request, env oauthEnv) {
	var st oauthState
	rawState := r.URL.Query().Get("state")
	if err := a.verifyBlob("state", rawState, &st); err != nil {
		http.Error(w, "登录状态无效: "+err.Error(), http.StatusBadRequest)
		return
	}
	if !a.consumeOnce("state|"+rawState, stateTTL) {
		http.Error(w, "登录状态已使用，请重新登录", http.StatusBadRequest)
		return
	}
	p, ok := env.providers[st.Provider]
	if !ok {
		http.Error(w, "第三方登录配置已变化，请重试", http.StatusBadRequest)
		return
	}
	if e := r.URL.Query().Get("error"); e != "" {
		http.Error(w, "身份提供方拒绝登录: "+e, http.StatusForbidden)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), thirdauth.HTTPTimeout)
	defer cancel()
	id, err := p.Exchange(ctx, r.URL.Query().Get("code"), env.callbackURL())
	if err != nil {
		http.Error(w, "登录失败: "+err.Error(), http.StatusBadGateway)
		return
	}
	u, err := url.Parse(st.Origin)
	if err != nil {
		http.Error(w, "原站点地址无效", http.StatusBadRequest)
		return
	}
	ticket := a.signBlob("ticket", oauthTicket{Identity: st.Provider + ":" + id.Primary, Aliases: id.Aliases, Host: hostOnly(u.Host), Scope: st.Scope, Next: st.Next, Nonce: st.Nonce}, ticketTTL)
	http.Redirect(w, r, st.Origin+oauthFinish+"?ticket="+url.QueryEscape(ticket), http.StatusFound)
}

// handleOAuthFinish 在原站点上校验票据与允许列表，建立会话。
func (a *authenticator) handleOAuthFinish(w http.ResponseWriter, r *http.Request, s Site) {
	var t oauthTicket
	raw := r.URL.Query().Get("ticket")
	if err := a.verifyBlob("ticket", raw, &t); err != nil {
		http.Error(w, "登录票据无效: "+err.Error(), http.StatusBadRequest)
		return
	}
	if t.Host != hostOnly(r.Host) || t.Scope != s.scope {
		http.Error(w, "登录票据与站点不匹配", http.StatusBadRequest)
		return
	}
	// 票据必须由发起登录的同一浏览器使用，防止把他人的登录结果注入受害者浏览器
	c, err := r.Cookie(oauthNonceCookie)
	if err != nil || !hmac.Equal([]byte(c.Value), []byte(t.Nonce)) {
		http.Error(w, "登录状态与当前浏览器不匹配，请重新登录", http.StatusBadRequest)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: oauthNonceCookie, Value: "", Path: authPath, MaxAge: -1})
	if !a.consumeOnce("ticket|"+raw, ticketTTL) {
		http.Error(w, "登录票据已使用", http.StatusBadRequest)
		return
	}
	provider, primary, _ := strings.Cut(t.Identity, ":")
	if !identityAllowed(s, thirdauth.Identity{Provider: provider, Primary: primary, Aliases: t.Aliases}) {
		http.Error(w, fmt.Sprintf("账号 %s 没有访问此站点的权限（如需授权，把它加入站点的第三方身份列表）", t.Identity), http.StatusForbidden)
		return
	}
	a.issueSession(w, r, s, t.Identity, t.Aliases...)
	http.Redirect(w, r, safeNext(t.Next), http.StatusFound)
}
