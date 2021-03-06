package application

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gimme-cdn/gimme/api"
	"github.com/gimme-cdn/gimme/configs"
	"github.com/gimme-cdn/gimme/internal/auth"
	"github.com/gimme-cdn/gimme/internal/content"
	gimmeerr "github.com/gimme-cdn/gimme/internal/errors"
	"github.com/gimme-cdn/gimme/internal/storage"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

type Application struct {
	config         *configs.Configuration
	authManager    auth.AuthManager
	contentService content.ContentService
	server         *http.Server
}

// NewApplication create an application instance
func NewApplication() Application {
	return Application{}
}

// loadConfig load app config
func (app *Application) loadConfig() {
	var err *gimmeerr.GimmeError
	app.config, err = configs.NewConfig()
	if err != nil {
		log.Fatalln(err.String())
	}
}

// loadModules load app modules
func (app *Application) loadModules() {
	var err *gimmeerr.GimmeError
	app.authManager = auth.NewAuthManager(app.config.Secret)

	osmClient, err := storage.NewObjectStorageClient(app.config)
	if err != nil {
		log.Fatalln(err.String())
	}
	objectStorageManager := storage.NewObjectStorageManager(osmClient)
	app.contentService = content.NewContentService(objectStorageManager)

	err = objectStorageManager.CreateBucket(app.config.S3BucketName, app.config.S3Location)
	if err != nil {
		log.Fatalln(err.String())
	}
}

func prometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()

	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// loadHttpServer load http (go gin) server
func (app *Application) setupServer() {
	router := gin.Default()
	router.Use(cors.Default())
	router.Static("/docs", "./docs")
	router.LoadHTMLGlob("templates/*.tmpl")

	api.NewRootController(router)
	api.NewAuthController(router, app.authManager, app.config)
	api.NewPackageController(router, app.authManager, app.contentService)

	if app.config.EnableMetrics {
		router.GET("/metrics", prometheusHandler())
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", app.config.AppPort),
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && errors.Is(err, http.ErrServerClosed) {
			logrus.Error(err)
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logrus.Info("Shutting down server...")

	// The context is used to inform the server it has 5 seconds to finish
	// the request it is currently handling
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		logrus.Fatal("Server forced to shutdown")
	}

	logrus.Info("Server exiting.")
}

// Run - run application
func (app *Application) Run() {
	app.loadConfig()
	app.loadModules()
	app.setupServer()
}
