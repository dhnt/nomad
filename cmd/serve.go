/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	"github.com/dhnt/nomad/api/handler"
	"github.com/dhnt/nomad/internal/server"

	"github.com/spf13/cobra"
)

func health(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func serve(cfg *server.ServerConfig) {
	log.Printf("config: %v", cfg)

	if err := os.Chdir(cfg.Root); err != nil {
		log.Fatalf("serve Chdir: %v", err)
	}

	mux := http.NewServeMux()

	addr := fmt.Sprintf(":%v", cfg.Port)
	hs := &http.Server{Addr: addr, Handler: mux}
	connsClosed := make(chan struct{})

	// shutdown signal and handler
	shutdown := func() {
		log.Println("server shutting down...")
		if err := hs.Shutdown(context.Background()); err != nil {
			log.Printf("Shutdown: %v", err)
		}
		close(connsClosed)
	}
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		sig := <-sigChan
		log.Printf("caught signal: %v", sig)
		shutdown()
	}()
	mux.HandleFunc("/shutdown", func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK\n"))
		go shutdown()
	})

	mux.HandleFunc("/health", health)

	ph := server.NewProcHandler(cfg)
	mux.Handle("/procs", ph)
	mux.Handle("/procs/", ph)

	vh := server.NewVolHandler(cfg.Root)
	mux.Handle("/volumes/", vh)

	bh := handler.NewBlobHandler("/blob/", cfg.Root)
	mux.Handle("/blob/", bh)

	fh, err := handler.NewFileHandler("/fs/", cfg.Root, cfg.Url)
	if err != nil {
		log.Fatalf("could not create fs handler: %v", err)
	}
	mux.Handle("/fs/", fh)

	// make files available for browsing
	mux.Handle("/root/", http.StripPrefix("/root/", http.FileServer(http.Dir(cfg.Root))))
	mux.Handle("/", http.RedirectHandler(cfg.Url.JoinPath("/root/").String(), http.StatusSeeOther))

	ver := server.Version

	log.Printf("Server %v listening at: %s", ver, addr)
	if err := hs.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("serve ListenAndServe: %v", err)
	}

	<-connsClosed
}

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("port")
		root, _ := cmd.Flags().GetString("root")

		s, _ := cmd.Flags().GetString("url")
		url, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}

		serve(&server.ServerConfig{
			Port: port,
			Root: root,
			Url:  url,
		})
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// serveCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// serveCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	home, err := os.UserHomeDir()
	if err != nil {
		home = "/"
	}

	serveCmd.Flags().IntP("port", "p", 58080, "Specifies the port on which the server listens for connections")
	serveCmd.Flags().String("root", home, "Specifies the base directory for resolving file path")

	serveCmd.Flags().String("url", "http://localhost:58080/", "Specifies the service url for file upload/download")
}
