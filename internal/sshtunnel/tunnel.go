// Package sshtunnel 通过 SSH 跳板机把远端数据库端口转发到本机回环地址。
// 数据库驱动照常连接 127.0.0.1:<随机端口>，因此 MySQL/PG 等驱动无需任何改动。
package sshtunnel

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// Config SSH 跳板机配置
type Config struct {
	Host     string
	Port     int
	User     string
	AuthType string // "password"（默认）| "key"
	KeyPath  string // 私钥文件路径（AuthType=key）
	Secret   string // 密码，或私钥口令
	// KnownHostsPath 主机公钥记录文件。首次连接某主机时自动信任并记录（TOFU），
	// 之后公钥变化则拒绝连接，防止中间人。
	KnownHostsPath string
	DialTimeout    time.Duration
}

// Tunnel 一条已建立的本地端口转发
type Tunnel struct {
	client   *ssh.Client
	listener net.Listener
	remote   string

	closeOnce sync.Once
	done      chan struct{}
}

// ErrHostKeyChanged 主机公钥与已记录的不一致
var ErrHostKeyChanged = errors.New("SSH 主机公钥与已记录的不一致，可能存在中间人攻击；如确认服务器已重装，请从 known_hosts 中删除对应记录后重试")

// Open 连接跳板机并在本机开一个随机端口，转发到 remoteHost:remotePort。
func Open(cfg Config, remoteHost string, remotePort int) (*Tunnel, error) {
	if cfg.Host == "" || cfg.User == "" {
		return nil, errors.New("SSH 主机和用户名不能为空")
	}
	if cfg.Port == 0 {
		cfg.Port = 22
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 8 * time.Second
	}
	auth, err := authMethods(cfg)
	if err != nil {
		return nil, err
	}
	hostKeyCb, err := hostKeyCallback(cfg.KnownHostsPath)
	if err != nil {
		return nil, err
	}
	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: hostKeyCb,
		Timeout:         cfg.DialTimeout,
	}
	addr := net.JoinHostPort(cfg.Host, fmt.Sprint(cfg.Port))
	client, err := ssh.Dial("tcp", addr, sshCfg)
	if err != nil {
		if errors.Is(err, ErrHostKeyChanged) {
			return nil, ErrHostKeyChanged
		}
		return nil, fmt.Errorf("SSH 连接 %s 失败: %w", addr, err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("监听本地端口失败: %w", err)
	}
	t := &Tunnel{
		client:   client,
		listener: ln,
		remote:   net.JoinHostPort(remoteHost, fmt.Sprint(remotePort)),
		done:     make(chan struct{}),
	}
	go t.acceptLoop()
	go t.keepAlive()
	go func() { // SSH 会话断开时一并关闭本地监听，让上层的探活发现并重建
		_ = client.Wait()
		t.Close()
	}()
	return t, nil
}

// LocalAddr 返回本地监听地址 "127.0.0.1:port"
func (t *Tunnel) LocalAddr() string { return t.listener.Addr().String() }

// LocalPort 返回本地监听端口
func (t *Tunnel) LocalPort() int { return t.listener.Addr().(*net.TCPAddr).Port }

// Close 关闭转发与 SSH 连接，可重复调用。
func (t *Tunnel) Close() {
	t.closeOnce.Do(func() {
		close(t.done)
		_ = t.listener.Close()
		_ = t.client.Close()
	})
}

// maxForwards 一条隧道同时转发的连接数上限。数据库连接池通常只有几十个连接，
// 远多于此多半是本机上某个程序在失控地重连；不设上限会把 goroutine 和文件描述符耗光。
const maxForwards = 128

func (t *Tunnel) acceptLoop() {
	slots := make(chan struct{}, maxForwards)
	for {
		local, err := t.listener.Accept()
		if err != nil {
			return
		}
		select {
		case slots <- struct{}{}:
		default:
			_ = local.Close() // 满了：直接拒绝这个新连接，不影响已有的
			continue
		}
		go func() {
			defer func() { <-slots }()
			t.forward(local)
		}()
	}
}

func (t *Tunnel) forward(local net.Conn) {
	remote, err := t.client.Dial("tcp", t.remote)
	if err != nil {
		_ = local.Close()
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	pipe := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		// 一侧结束就关闭两侧，避免另一方向的 Copy 永久阻塞
		_ = dst.Close()
		_ = src.Close()
	}
	go pipe(remote, local)
	go pipe(local, remote)
	wg.Wait()
}

// keepAlive 周期性发送 SSH 保活，避免空闲时被跳板机/防火墙断开。
func (t *Tunnel) keepAlive() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-t.done:
			return
		case <-ticker.C:
			if _, _, err := t.client.SendRequest("keepalive@openssh.com", true, nil); err != nil {
				t.Close()
				return
			}
		}
	}
}

func authMethods(cfg Config) ([]ssh.AuthMethod, error) {
	switch cfg.AuthType {
	case "key":
		if cfg.KeyPath == "" {
			return nil, errors.New("请选择 SSH 私钥文件")
		}
		pem, err := os.ReadFile(expandHome(cfg.KeyPath))
		if err != nil {
			return nil, fmt.Errorf("读取 SSH 私钥失败: %w", err)
		}
		var signer ssh.Signer
		if cfg.Secret != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(pem, []byte(cfg.Secret))
		} else {
			signer, err = ssh.ParsePrivateKey(pem)
		}
		if err != nil {
			var missing *ssh.PassphraseMissingError
			if errors.As(err, &missing) {
				return nil, errors.New("SSH 私钥已加密，请填写私钥口令")
			}
			return nil, fmt.Errorf("解析 SSH 私钥失败: %w", err)
		}
		return []ssh.AuthMethod{ssh.PublicKeys(signer)}, nil
	default:
		return []ssh.AuthMethod{ssh.Password(cfg.Secret)}, nil
	}
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

var knownHostsMu sync.Mutex

// hostKeyCallback 实现首次信任（TOFU）：未知主机写入记录并放行，已知主机公钥不符则拒绝。
func hostKeyCallback(path string) (ssh.HostKeyCallback, error) {
	if path == "" {
		return nil, errors.New("未配置 known_hosts 路径")
	}
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		knownHostsMu.Lock()
		defer knownHostsMu.Unlock()

		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
		// knownhosts.New 要求文件存在
		f, err := os.OpenFile(path, os.O_CREATE|os.O_RDONLY, 0o600)
		if err != nil {
			return err
		}
		f.Close()

		check, err := knownhosts.New(path)
		if err != nil {
			return fmt.Errorf("读取 known_hosts 失败: %w", err)
		}
		err = check(hostname, remote, key)
		if err == nil {
			return nil
		}
		var keyErr *knownhosts.KeyError
		if errors.As(err, &keyErr) {
			if len(keyErr.Want) > 0 {
				return ErrHostKeyChanged
			}
			// 未知主机：记录并信任
			line := knownhosts.Line([]string{knownhosts.Normalize(hostname)}, key)
			af, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
			if err != nil {
				return err
			}
			defer af.Close()
			_, err = af.WriteString(line + "\n")
			return err
		}
		return err
	}, nil
}
