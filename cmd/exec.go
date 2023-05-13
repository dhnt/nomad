/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dhnt/nomad/api"
	"github.com/dhnt/nomad/internal/shell"
)

type execConfig struct {
	cmd     string
	args    []string
	bg      bool
	timeout int64
	outfile string
	errfile string

	wait     bool
	interval int64
}

func exec(baseUrl *url.URL, cfg *execConfig) {
	log.Printf("%v %v", baseUrl, cfg)

	sh, err := shell.New(baseUrl.String())
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v", err)
		os.Exit(1)
	}

	log.Printf("config: %v", cfg)

	//
	showError := func(status int, err error) {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(status)
	}

	showResult := func(result interface{}) {
		b, err := json.Marshal(result)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			os.Exit(-1)
		}
		fmt.Fprintf(os.Stdout, "%v", string(b))
		os.Exit(0)
	}

	cmd := cfg.cmd

	switch cmd {
	case "ps":
		result, err := sh.Ps()
		if err != nil {
			showError(1, err)
		}
		showResult(result)
	case "kill":
		err = sh.Kill(cfg.args...)
		if err != nil {
			showError(1, err)
		}
		os.Exit(0)
	case "killall":
		err = sh.KillAll()
		if err != nil {
			showError(1, err)
		}
		os.Exit(0)
	default:
		r, err := sh.Exec(api.RunReq{
			Command:    cmd,
			Args:       cfg.args,
			Background: cfg.bg,
			Timeout:    cfg.timeout,
			Outfile:    cfg.outfile,
			Errfile:    cfg.errfile,
		})

		cleanup := func() {
			if r != nil && cfg.bg {
				sh.Kill(r.ID)
			}
		}

		log.Printf("%v err: %v", r, err)

		if err != nil || r.Status != 0 {
			status := 1
			if r.Status != 0 {
				status = r.Status
			}
			fmt.Fprintf(os.Stderr, "%v %v", r.Error, err)
			cleanup()
			os.Exit(status)
		}

		if !cfg.bg || !cfg.wait {
			showResult(r)
		}

		// running in backgroud and wait
		done := []api.RunState{api.Done, api.Failed}
		result, err := sh.Wait(r.ID, done, cfg.timeout, cfg.interval)

		log.Printf("%v err: %v", result, err)

		if err != nil || result.Status != 0 {
			status := 1
			if result.Status != 0 {
				status = result.Status
			}
			fmt.Fprintf(os.Stderr, "%v %v", result.Error, err)
			cleanup()
			os.Exit(status)
		}

		cleanup()
		showResult(result)
	}
}

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a command in a running instance",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			fmt.Printf("usage: %s run command [args ...]\n", filepath.Base(os.Args[0]))
			os.Exit(-1)
		}

		host, _ := cmd.Flags().GetString("host")
		port, _ := cmd.Flags().GetInt("port")
		s := fmt.Sprintf("http://%s:%v", host, port)

		u, err := url.Parse(s)
		if err != nil {
			log.Fatal(err)
		}

		bg, _ := cmd.Flags().GetBool("bg")
		wait, _ := cmd.Flags().GetBool("wait")
		timeout, _ := cmd.Flags().GetInt64("timeout")
		interval, _ := cmd.Flags().GetInt64("interval")

		outfile, _ := cmd.Flags().GetString("out")
		errfile, _ := cmd.Flags().GetString("err")

		exec(u, &execConfig{
			bg:       bg,
			wait:     wait,
			cmd:      args[0],
			args:     args[1:],
			timeout:  timeout,
			interval: interval,
			outfile:  outfile,
			errfile:  errfile,
		})
	},
}

func init() {
	rootCmd.AddCommand(execCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// execCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// execCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	execCmd.Flags().String("host", "localhost", "Host to connect to on the remote host")
	execCmd.Flags().Int("port", 58080, "Port to connect to on the remote host")

	execCmd.Flags().Bool("bg", false, "Run command in the background")
	execCmd.Flags().Bool("wait", false, "Wait for the specified command and report its termination status")
	execCmd.Flags().Int64("timeout", 30, "Timeout in seconds")
	execCmd.Flags().Int64("interval", 1, "Time interval for wait in seconds")

	execCmd.Flags().String("out", "", "Write output to the file if provided")
	execCmd.Flags().String("err", "", "Write error to the file if provided")
}
