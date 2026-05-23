package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"hdu-ride/backend/app"

	"github.com/gin-gonic/gin"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "hash-password" {
		hash, err := app.HashPassword(os.Args[2])
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(hash)
		return
	}
	if len(os.Args) >= 2 && os.Args[1] == "ops" {
		if err := runOps(os.Args[2:]); err != nil {
			log.Fatal(err)
		}
		return
	}

	ctx := context.Background()
	cfg, err := app.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}

	db, err := app.OpenDB(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := app.InitSchema(ctx, db, cfg); err != nil {
		log.Fatal(err)
	}

	content, err := app.LoadCourses(cfg.ContentRoot, cfg.WorkspaceImageDefault)
	if err != nil && content == nil {
		log.Fatal(err)
	}
	if err != nil {
		log.Println("WARNING:", err)
	}

	objects, err := app.OpenObjectStore(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}

	workspaces, err := app.NewWorkspaceManager(cfg)
	if err != nil {
		log.Fatal(err)
	}

	serverApp := app.NewApp(cfg, db, content, objects, workspaces)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	app.RegisterRoutes(router, serverApp)

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
