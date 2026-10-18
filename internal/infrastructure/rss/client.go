package rss

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"syscall"
	"time"
)

const (
	requestTimeout = 10 * time.Second
	connectTimeout = 5 * time.Second
	maxRedirects   = 5
)

// newHTTPClient создаёт клиент для чтения произвольных пользовательских лент.
// Он не пускает запросы на localhost, в частные сети и служебные диапазоны,
// чтобы адрес ленты нельзя было использовать для SSRF.
func newHTTPClient() *http.Client {
	dialer := &net.Dialer{
		Timeout: connectTimeout,
		Control: func(_, address string, _ syscall.RawConn) error {
			return checkDialAddress(address)
		},
	}

	transport := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSHandshakeTimeout:   connectTimeout,
		ResponseHeaderTimeout: requestTimeout,
		MaxIdleConns:          20,
		IdleConnTimeout:       30 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("слишком много перенаправлений")
			}
			return checkFeedURL(request.URL)
		},
	}
}

// checkFeedURL проверяет схему и наличие хоста у адреса ленты.
func checkFeedURL(parsed *neturl.URL) error {
	if parsed == nil {
		return errors.New("пустой адрес ленты")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("схема %q не поддерживается", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return errors.New("в адресе ленты нет хоста")
	}
	return nil
}

// checkDialAddress вызывается после разрешения имени: проверяется уже
// фактический IP, поэтому DNS rebinding не помогает обойти защиту.
func checkDialAddress(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("не удалось определить адрес %q", address)
	}
	if forbiddenIP(ip) {
		return fmt.Errorf("адрес %s запрещён", ip)
	}
	return nil
}

func forbiddenIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsMulticast() {
		return true
	}

	for _, block := range forbiddenBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

var forbiddenBlocks = parseBlocks(
	"0.0.0.0/8",
	"100.64.0.0/10",
	"127.0.0.0/8",
	"169.254.0.0/16",
	"192.0.0.0/24",
	"198.18.0.0/15",
	"240.0.0.0/4",
	"::1/128",
	"fc00::/7",
	"fe80::/10",
)

func parseBlocks(values ...string) []*net.IPNet {
	blocks := make([]*net.IPNet, 0, len(values))
	for _, value := range values {
		_, block, err := net.ParseCIDR(value)
		if err == nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}
