package main

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/mux"
	"github.com/rancher/remotedialer"
	"github.com/rancher/remotedialer/api/proxy"
	"github.com/rancher/remotedialer/dummy"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

var (
	clients = map[string]*http.Client{}
	l       sync.Mutex
	counter int64
)

func DefaultErrorWriter(rw http.ResponseWriter, req *http.Request, code int, err error) {
	rw.WriteHeader(code)
	rw.Write([]byte(err.Error()))
}

func Client(server *remotedialer.Server, rw http.ResponseWriter, req *http.Request) {
	timeout := req.URL.Query().Get("timeout")
	if timeout == "" {
		timeout = "15"
	}

	vars := mux.Vars(req)
	clientKey := vars["id"]
	url := fmt.Sprintf("%s://%s%s", vars["scheme"], vars["host"], vars["path"])
	client := getClient(server, clientKey, timeout)

	id := atomic.AddInt64(&counter, 1)
	logrus.Infof("[%03d] REQ t=%s %s", id, timeout, url)

	resp, err := client.Get(url)
	if err != nil {
		logrus.Errorf("[%03d] REQ ERR t=%s %s: %v", id, timeout, url, err)
		DefaultErrorWriter(rw, req, 500, err)
		return
	}
	defer resp.Body.Close()

	logrus.Infof("[%03d] REQ OK t=%s %s", id, timeout, url)
	rw.WriteHeader(resp.StatusCode)
	io.Copy(rw, resp.Body)
	logrus.Infof("[%03d] REQ DONE t=%s %s", id, timeout, url)
}

func getClient(server *remotedialer.Server, clientKey, timeout string) *http.Client {
	l.Lock()
	defer l.Unlock()

	key := fmt.Sprintf("%s/%s", clientKey, timeout)
	client := clients[key]
	if client != nil {
		return client
	}

	dialer := server.Dialer(clientKey)
	client = &http.Client{
		Transport: &http.Transport{
			DialContext: dialer,
		},
	}
	if timeout != "" {
		t, err := strconv.Atoi(timeout)
		if err == nil {
			client.Timeout = time.Duration(t) * time.Second
		}
	}

	clients[key] = client
	return client
}

type proxyService struct {
	remoteDialerServer *remotedialer.Server
	proxy.UnimplementedProxyServiceServer
}

func newProxyServer(server *remotedialer.Server) *proxyService {
	return &proxyService{
		remoteDialerServer: server,
	}
}

func (s *proxyService) ProxyStream(stream proxy.ProxyService_ProxyStreamServer) error {
	clientKey := "foo"

	fmt.Println("get connect")
	defer fmt.Println("lose connect")

	conn := dummy.NewGrpcStreamTunnel(stream)

	session := s.remoteDialerServer.SessionAdd(clientKey, conn)
	defer s.remoteDialerServer.SessionRemove(session)

	err := session.Serve(stream.Context())
	if err != nil {
		return err
	}

	return nil
}

func runGrpc(server *remotedialer.Server) {
	lis, err := net.Listen("tcp", ":8123")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	proxy.RegisterProxyServiceServer(grpcServer, newProxyServer(server))
	grpcServer.Serve(lis)
}

func runHttp(remoteDialerServer *remotedialer.Server) {
	router := mux.NewRouter()
	router.HandleFunc("/client/{id}/{scheme}/{host}{path:.*}", func(rw http.ResponseWriter, req *http.Request) {
		Client(remoteDialerServer, rw, req)
	})

	http.ListenAndServe(":8080", router)
}

func main() {
	remoteDialerServer := remotedialer.New()

	go runGrpc(remoteDialerServer)

	runHttp(remoteDialerServer)
}
