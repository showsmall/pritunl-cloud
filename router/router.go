package router

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/tls"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dropbox/godropbox/errors"
	"github.com/gin-gonic/gin"
	"github.com/pritunl/mongo-go-driver/bson"
	"github.com/pritunl/pritunl-cloud/acme"
	"github.com/pritunl/pritunl-cloud/ahandlers"
	"github.com/pritunl/pritunl-cloud/balancer"
	"github.com/pritunl/pritunl-cloud/constants"
	"github.com/pritunl/pritunl-cloud/database"
	"github.com/pritunl/pritunl-cloud/errortypes"
	"github.com/pritunl/pritunl-cloud/event"
	"github.com/pritunl/pritunl-cloud/node"
	"github.com/pritunl/pritunl-cloud/proxy"
	"github.com/pritunl/pritunl-cloud/settings"
	"github.com/pritunl/pritunl-cloud/uhandlers"
	"github.com/pritunl/pritunl-cloud/utils"
	"github.com/sirupsen/logrus"
)

type Router struct {
	nodeHash         []byte
	singleType       bool
	adminType        bool
	userType         bool
	balancerType     bool
	port             int
	noRedirectServer bool
	protocol         string
	adminDomain      string
	userDomain       string
	stateLock        sync.Mutex
	balancers        []*balancer.Balancer
	certificates     *Certificates
	aRouter          *gin.Engine
	uRouter          *gin.Engine
	waiter           sync.WaitGroup
	lock             sync.Mutex
	redirectServer   *http.Server
	webServer        *http.Server
	proxy            *proxy.Proxy
	stop             bool
}

func (r *Router) ServeHTTP(w http.ResponseWriter, re *http.Request) {
	if node.Self.ForwardedProtoHeader != "" &&
		strings.ToLower(re.Header.Get(
			node.Self.ForwardedProtoHeader)) == "http" {

		re.URL.Host = utils.StripPort(re.Host)
		re.URL.Scheme = "https"

		http.Redirect(w, re, re.URL.String(),
			http.StatusMovedPermanently)
		return
	}

	if r.singleType {
		if r.adminType {
			r.aRouter.ServeHTTP(w, re)
		} else if r.userType {
			r.uRouter.ServeHTTP(w, re)
		} else if r.balancerType {
			r.proxy.ServeHTTP(utils.StripPort(re.Host), w, re)
		} else {
			utils.WriteStatus(w, 520)
		}
		return
	} else {
		hst := utils.StripPort(re.Host)
		if r.adminType && hst == r.adminDomain {
			r.aRouter.ServeHTTP(w, re)
			return
		} else if r.userType && hst == r.userDomain {
			r.uRouter.ServeHTTP(w, re)
			return
		} else if r.balancerType {
			r.proxy.ServeHTTP(hst, w, re)
			return
		}
	}

	if re.URL.Path == "/check" {
		utils.WriteText(w, 200, "ok")
		return
	}

	utils.WriteStatus(w, 404)
}

func (r *Router) initRedirect() (err error) {
	r.redirectServer = &http.Server{
		Addr:           ":80",
		ReadTimeout:    1 * time.Minute,
		WriteTimeout:   1 * time.Minute,
		IdleTimeout:    1 * time.Minute,
		MaxHeaderBytes: 8192,
		Handler: http.HandlerFunc(func(
			w http.ResponseWriter, req *http.Request) {

			if strings.HasPrefix(req.URL.Path, acme.AcmePath) {
				token := acme.ParsePath(req.URL.Path)
				token = utils.FilterStr(token, 96)
				if token != "" {
					chal, err := acme.GetChallenge(token)
					if err != nil {
						utils.WriteStatus(w, 400)
					} else {
						logrus.WithFields(logrus.Fields{
							"token": token,
						}).Info("router: Acme challenge requested")
						utils.WriteText(w, 200, chal.Resource)
					}
					return
				}
			} else if req.URL.Path == "/check" {
				utils.WriteText(w, 200, "ok")
				return
			}

			newHost := utils.StripPort(req.Host)
			if r.port != 443 {
				newHost += fmt.Sprintf(":%d", r.port)
			}

			req.URL.Host = newHost
			req.URL.Scheme = "https"

			http.Redirect(w, req, req.URL.String(),
				http.StatusMovedPermanently)
		}),
	}

	return
}

func (r *Router) startRedirect() {
	defer r.waiter.Done()

	if r.port == 80 || r.noRedirectServer {
		return
	}

	logrus.WithFields(logrus.Fields{
		"production": constants.Production,
		"protocol":   "http",
		"port":       80,
	}).Info("router: Starting redirect server")

	err := r.redirectServer.ListenAndServe()
	if err != nil {
		if err == http.ErrServerClosed {
			err = nil
		} else {
			err = &errortypes.UnknownError{
				errors.Wrap(err, "router: Server listen failed"),
			}
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("router: Redirect server error")
		}
	}
}

func (r *Router) initWeb() (err error) {
	r.adminType = node.Self.IsAdmin()
	r.userType = node.Self.IsUser()
	r.balancerType = node.Self.IsBalancer()
	r.adminDomain = node.Self.AdminDomain
	r.userDomain = node.Self.UserDomain
	r.noRedirectServer = node.Self.NoRedirectServer

	if r.adminType && !r.userType && !r.balancerType {
		r.singleType = true
	} else if r.userType && !r.balancerType && !r.adminType {
		r.singleType = true
	} else if r.balancerType && !r.adminType && !r.userType {
		r.singleType = true
	} else {
		r.singleType = false
	}

	r.port = node.Self.Port
	if r.port == 0 {
		r.port = 443
	}

	r.protocol = node.Self.Protocol
	if r.protocol == "" {
		r.protocol = "https"
	}

	if r.adminType {
		r.aRouter = gin.New()

		if !constants.Production {
			r.aRouter.Use(gin.Logger())
		}

		ahandlers.Register(r.aRouter)
	}

	if r.userType {
		r.uRouter = gin.New()

		if !constants.Production {
			r.uRouter.Use(gin.Logger())
		}

		uhandlers.Register(r.uRouter)
	}

	readTimeout := time.Duration(settings.Router.ReadTimeout) * time.Second
	readHeaderTimeout := time.Duration(
		settings.Router.ReadHeaderTimeout) * time.Second
	writeTimeout := time.Duration(settings.Router.WriteTimeout) * time.Second
	idleTimeout := time.Duration(settings.Router.IdleTimeout) * time.Second

	r.webServer = &http.Server{
		Addr:              fmt.Sprintf(":%d", r.port),
		Handler:           r,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    settings.Router.MaxHeaderBytes,
	}

	return
}

func (r *Router) startWeb() {
	defer r.waiter.Done()

	logrus.WithFields(logrus.Fields{
		"production":          constants.Production,
		"protocol":            r.protocol,
		"port":                r.port,
		"read_timeout":        settings.Router.ReadTimeout,
		"write_timeout":       settings.Router.WriteTimeout,
		"idle_timeout":        settings.Router.IdleTimeout,
		"read_header_timeout": settings.Router.ReadHeaderTimeout,
	}).Info("router: Starting web server")

	if r.protocol == "http" {
		err := r.webServer.ListenAndServe()
		if err != nil {
			if err == http.ErrServerClosed {
				err = nil
			} else {
				err = &errortypes.UnknownError{
					errors.Wrap(err, "router: Server listen failed"),
				}
				logrus.WithFields(logrus.Fields{
					"error": err,
				}).Error("router: Web server error")
				return
			}
		}
	} else {
		tlsConfig := &tls.Config{
			MinVersion:     tls.VersionTLS12,
			MaxVersion:     tls.VersionTLS13,
			GetCertificate: r.certificates.GetCertificate,
		}

		listener, err := tls.Listen("tcp", r.webServer.Addr, tlsConfig)
		if err != nil {
			err = &errortypes.UnknownError{
				errors.Wrap(err, "router: TLS listen failed"),
			}
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("router: Web server TLS error")
			return
		}

		err = r.webServer.Serve(listener)
		if err != nil {
			if err == http.ErrServerClosed {
				err = nil
			} else {
				err = &errortypes.UnknownError{
					errors.Wrap(err, "router: Server listen failed"),
				}
				logrus.WithFields(logrus.Fields{
					"error": err,
				}).Error("router: Web server error")
				return
			}
		}
	}

	return
}

func (r *Router) initServers() (err error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	err = r.certificates.Init()
	if err != nil {
		return
	}

	err = r.updateState()
	if err != nil {
		return
	}

	err = r.initRedirect()
	if err != nil {
		return
	}

	err = r.initWeb()
	if err != nil {
		return
	}

	return
}

func (r *Router) startServers() {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.redirectServer == nil || r.webServer == nil {
		return
	}

	r.waiter.Add(2)
	go r.startRedirect()
	go r.startWeb()

	time.Sleep(250 * time.Millisecond)

	return
}

func (r *Router) Restart() {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.redirectServer != nil {
		redirectCtx, redirectCancel := context.WithTimeout(
			context.Background(),
			1*time.Second,
		)
		defer redirectCancel()
		r.redirectServer.Shutdown(redirectCtx)
	}
	if r.webServer != nil {
		webCtx, webCancel := context.WithTimeout(
			context.Background(),
			1*time.Second,
		)
		defer webCancel()
		r.webServer.Shutdown(webCtx)
	}

	func() {
		defer func() {
			recover()
		}()
		if r.redirectServer != nil {
			r.redirectServer.Close()
		}
		if r.webServer != nil {
			r.webServer.Close()
		}
	}()

	event.WebSocketsStop()

	r.redirectServer = nil
	r.webServer = nil

	time.Sleep(250 * time.Millisecond)
}

func (r *Router) Shutdown() {
	r.stop = true
	r.Restart()
	time.Sleep(1 * time.Second)
	r.Restart()
	time.Sleep(1 * time.Second)
	r.Restart()
}

func (r *Router) hashNode() []byte {
	hash := md5.New()
	for _, typ := range node.Self.Types {
		io.WriteString(hash, typ)
	}
	io.WriteString(hash, node.Self.AdminDomain)
	io.WriteString(hash, node.Self.UserDomain)
	io.WriteString(hash, strconv.Itoa(node.Self.Port))
	io.WriteString(hash, fmt.Sprintf("%t", node.Self.NoRedirectServer))
	io.WriteString(hash, node.Self.Protocol)

	io.WriteString(hash, strconv.Itoa(settings.Router.ReadTimeout))
	io.WriteString(hash, strconv.Itoa(settings.Router.ReadHeaderTimeout))
	io.WriteString(hash, strconv.Itoa(settings.Router.WriteTimeout))
	io.WriteString(hash, strconv.Itoa(settings.Router.IdleTimeout))

	return hash.Sum(nil)
}

func (r *Router) watchNode() {
	for {
		time.Sleep(1 * time.Second)

		hash := r.hashNode()
		if bytes.Compare(r.nodeHash, hash) != 0 {
			r.nodeHash = hash
			time.Sleep(time.Duration(rand.Intn(3)) * time.Second)
			r.Restart()
			time.Sleep(2 * time.Second)
		}
	}
}

func (r *Router) updateState() (err error) {
	db := database.GetDatabase()
	defer db.Close()

	if node.Self.IsBalancer() {
		dcId, e := node.Self.GetDatacenter(db)
		if e != nil {
			err = e
			return
		}

		balncs, e := balancer.GetAll(db, &bson.M{
			"datacenter": dcId,
		})
		if e != nil {
			r.balancers = []*balancer.Balancer{}
			return
		}

		r.balancers = balncs
	} else {
		r.balancers = []*balancer.Balancer{}
	}

	r.stateLock.Lock()
	defer r.stateLock.Unlock()

	err = r.certificates.Update(db, r.balancers)
	if err != nil {
		return
	}

	err = r.proxy.Update(db, r.balancers)
	if err != nil {
		return
	}

	return
}

func (r *Router) watchState() {
	for {
		time.Sleep(4 * time.Second)

		err := r.updateState()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("proxy: Failed to load proxy state")
		}
	}
}

func (r *Router) Run() (err error) {
	r.nodeHash = r.hashNode()
	go r.watchNode()
	go r.watchState()

	for {
		if !node.Self.IsAdmin() && !node.Self.IsUser() &&
			!node.Self.IsBalancer() {

			time.Sleep(500 * time.Millisecond)
			continue
		}

		err = r.initServers()
		if err != nil {
			logrus.WithFields(logrus.Fields{
				"error": err,
			}).Error("router: Failed to init web servers")
			time.Sleep(1 * time.Second)
			continue
		}

		r.waiter = sync.WaitGroup{}
		r.startServers()
		r.waiter.Wait()

		if r.stop {
			break
		}
	}

	return
}

func (r *Router) Init() {
	if constants.Production {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	r.certificates = &Certificates{}
	r.proxy = &proxy.Proxy{}
	r.proxy.Init()
}
