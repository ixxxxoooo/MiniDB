package sshtunnel

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

type testServer struct {
	addr       string
	port       int
	hostSigner ssh.Signer
	authorized ssh.PublicKey
	forwarded  atomic.Int32
	ln         net.Listener
}

func newSigner(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	s, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	return s, priv
}

// startServer 起一个只支持 password=pw 或指定公钥、并处理 direct-tcpip 的进程内 SSH 服务器。
func startServer(t *testing.T, hostSigner ssh.Signer, authorized ssh.PublicKey) *testServer {
	t.Helper()
	return startServerOn(t, "127.0.0.1:0", hostSigner, authorized)
}

func startServerOn(t *testing.T, listenAddr string, hostSigner ssh.Signer, authorized ...ssh.PublicKey) *testServer {
	t.Helper()
	var auth ssh.PublicKey
	if len(authorized) > 0 {
		auth = authorized[0]
	}
	authorizedKey := auth
	srv := &testServer{hostSigner: hostSigner, authorized: authorizedKey}
	cfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pw []byte) (*ssh.Permissions, error) {
			if c.User() == "u" && string(pw) == "pw" {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
		PublicKeyCallback: func(c ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			if srv.authorized != nil && string(key.Marshal()) == string(srv.authorized.Marshal()) {
				return nil, nil
			}
			return nil, errors.New("denied")
		},
	}
	cfg.AddHostKey(hostSigner)
	ln, err := net.Listen("tcp", listenAddr)
	if err != nil {
		t.Fatal(err)
	}
	srv.ln = ln
	srv.addr = ln.Addr().String()
	srv.port = ln.Addr().(*net.TCPAddr).Port
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				conn, chans, reqs, err := ssh.NewServerConn(c, cfg)
				if err != nil {
					return
				}
				defer conn.Close()
				go func() {
					for r := range reqs {
						if r.WantReply {
							r.Reply(false, nil)
						}
					}
				}()
				for nc := range chans {
					if nc.ChannelType() != "direct-tcpip" {
						nc.Reject(ssh.UnknownChannelType, "no")
						continue
					}
					var req struct {
						Host     string
						Port     uint32
						OrigHost string
						OrigPort uint32
					}
					if err := ssh.Unmarshal(nc.ExtraData(), &req); err != nil {
						nc.Reject(ssh.ConnectionFailed, "bad")
						continue
					}
					target, err := net.Dial("tcp", net.JoinHostPort(req.Host, itoa(req.Port)))
					if err != nil {
						nc.Reject(ssh.ConnectionFailed, err.Error())
						continue
					}
					ch, chReqs, err := nc.Accept()
					if err != nil {
						target.Close()
						continue
					}
					go ssh.DiscardRequests(chReqs)
					srv.forwarded.Add(1)
					go func() { io.Copy(ch, target); ch.Close() }()
					go func() { io.Copy(target, ch); target.Close() }()
				}
			}()
		}
	}()
	return srv
}

func itoa(u uint32) string { return fmtUint(u) }
func fmtUint(u uint32) string {
	if u == 0 {
		return "0"
	}
	var b []byte
	for u > 0 {
		b = append([]byte{byte('0' + u%10)}, b...)
		u /= 10
	}
	return string(b)
}

// startEcho 起一个目标"数据库"：把收到的内容加前缀回显。
func startEcho(t *testing.T) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer c.Close()
				buf := make([]byte, 64)
				n, _ := c.Read(buf)
				c.Write(append([]byte("echo:"), buf[:n]...))
			}()
		}
	}()
	return "127.0.0.1", ln.Addr().(*net.TCPAddr).Port
}

func roundTrip(t *testing.T, addr string) string {
	t.Helper()
	c, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(3 * time.Second))
	c.Write([]byte("hi"))
	b, _ := io.ReadAll(c)
	return string(b)
}

func TestTunnelPasswordForwardsTraffic(t *testing.T) {
	hostSigner, _ := newSigner(t)
	srv := startServer(t, hostSigner, nil)
	dbHost, dbPort := startEcho(t)
	kh := filepath.Join(t.TempDir(), "known_hosts")

	tun, err := Open(Config{Host: "127.0.0.1", Port: srv.port, User: "u", Secret: "pw", KnownHostsPath: kh}, dbHost, dbPort)
	if err != nil {
		t.Fatal(err)
	}
	defer tun.Close()
	if got := roundTrip(t, tun.LocalAddr()); got != "echo:hi" {
		t.Fatalf("got %q", got)
	}
	if srv.forwarded.Load() != 1 {
		t.Fatal("流量应经 SSH 通道转发")
	}
	if data, _ := os.ReadFile(kh); len(data) == 0 {
		t.Fatal("首次连接应记录主机公钥")
	}
}

func TestTunnelWrongPasswordFails(t *testing.T) {
	hostSigner, _ := newSigner(t)
	srv := startServer(t, hostSigner, nil)
	_, err := Open(Config{Host: "127.0.0.1", Port: srv.port, User: "u", Secret: "bad", KnownHostsPath: filepath.Join(t.TempDir(), "kh")}, "127.0.0.1", 1)
	if err == nil {
		t.Fatal("错误密码应失败")
	}
}

func TestTunnelKeyAuthWithAndWithoutPassphrase(t *testing.T) {
	hostSigner, _ := newSigner(t)
	userSigner, userPriv := newSigner(t)
	srv := startServer(t, hostSigner, userSigner.PublicKey())
	dbHost, dbPort := startEcho(t)
	dir := t.TempDir()

	plainBlock, _ := ssh.MarshalPrivateKey(userPriv, "")
	plain := filepath.Join(dir, "id_plain")
	os.WriteFile(plain, pem.EncodeToMemory(plainBlock), 0o600)
	encBlock, _ := ssh.MarshalPrivateKeyWithPassphrase(userPriv, "", []byte("pp"))
	enc := filepath.Join(dir, "id_enc")
	os.WriteFile(enc, pem.EncodeToMemory(encBlock), 0o600)
	kh := filepath.Join(dir, "kh")

	tun, err := Open(Config{Host: "127.0.0.1", Port: srv.port, User: "any", AuthType: "key", KeyPath: plain, KnownHostsPath: kh}, dbHost, dbPort)
	if err != nil {
		t.Fatal(err)
	}
	if got := roundTrip(t, tun.LocalAddr()); got != "echo:hi" {
		t.Fatalf("got %q", got)
	}
	tun.Close()

	if _, err := Open(Config{Host: "127.0.0.1", Port: srv.port, User: "any", AuthType: "key", KeyPath: enc, KnownHostsPath: kh}, dbHost, dbPort); err == nil || !strings.Contains(err.Error(), "口令") {
		t.Fatalf("加密私钥缺口令应提示，err=%v", err)
	}
	tun, err = Open(Config{Host: "127.0.0.1", Port: srv.port, User: "any", AuthType: "key", KeyPath: enc, Secret: "pp", KnownHostsPath: kh}, dbHost, dbPort)
	if err != nil {
		t.Fatal(err)
	}
	tun.Close()
}

func TestTunnelRejectsChangedHostKey(t *testing.T) {
	signerA, _ := newSigner(t)
	signerB, _ := newSigner(t)
	kh := filepath.Join(t.TempDir(), "kh")

	srvA := startServer(t, signerA, nil)
	tun, err := Open(Config{Host: "127.0.0.1", Port: srvA.port, User: "u", Secret: "pw", KnownHostsPath: kh}, "127.0.0.1", 1)
	if err != nil {
		t.Fatal(err)
	}
	tun.Close()

	// 同一地址换一把主机密钥：关闭 A，在同一端口起 B
	srvA.ln.Close()
	ln, err := net.Listen("tcp", srvA.addr)
	if err != nil {
		t.Skipf("无法复用端口: %v", err)
	}
	ln.Close()
	srvB := startServerOn(t, srvA.addr, signerB)
	_ = srvB
	_, err = Open(Config{Host: "127.0.0.1", Port: srvA.port, User: "u", Secret: "pw", KnownHostsPath: kh}, "127.0.0.1", 1)
	if !errors.Is(err, ErrHostKeyChanged) {
		t.Fatalf("主机公钥变化应被拒绝，err=%v", err)
	}
}

func TestTunnelCloseStopsListener(t *testing.T) {
	hostSigner, _ := newSigner(t)
	srv := startServer(t, hostSigner, nil)
	tun, err := Open(Config{Host: "127.0.0.1", Port: srv.port, User: "u", Secret: "pw", KnownHostsPath: filepath.Join(t.TempDir(), "kh")}, "127.0.0.1", 1)
	if err != nil {
		t.Fatal(err)
	}
	addr := tun.LocalAddr()
	tun.Close()
	tun.Close() // 幂等
	if c, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		c.Close()
		t.Fatal("关闭后不应再接受连接")
	}
}
