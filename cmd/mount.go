/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/dhnt/nomad/internal/fuse/fs"
	"github.com/spf13/cobra"
)

type mountConfig struct {
	debug             bool
	allowOther        bool
	directMount       bool
	directMountStrict bool

	readOnly bool
	quiet    bool

	cpuProfile string
	memProfile string

	mountPoint string
	remote     string
}

func mount(cfg *mountConfig) {
	// log.SetFlags(log.Lmicroseconds)
	// Scans the arg list and sets up flags
	// debug := flag.Bool("debug", false, "print debugging messages.")
	// other := flag.Bool("allow-other", false, "mount with -o allowother.")
	// quiet := flag.Bool("q", false, "quiet")
	// ro := flag.Bool("ro", false, "mount read-only")
	// directmount := flag.Bool("directmount", false, "try to call the mount syscall instead of executing fusermount")
	// directmountstrict := flag.Bool("directmountstrict", false, "like directmount, but don't fall back to fusermount")
	// cpuprofile := flag.String("cpuprofile", "", "write cpu profile to this file")
	// memprofile := flag.String("memprofile", "", "write memory profile to this file")
	// flag.Parse()
	// if flag.NArg() < 2 {
	// 	fmt.Printf("usage: %s MOUNTPOINT ORIGINAL\n", path.Base(os.Args[0]))
	// 	fmt.Printf("\noptions:\n")
	// 	flag.PrintDefaults()
	// 	os.Exit(2)
	// }

	if cfg.cpuProfile != "" {
		if !cfg.quiet {
			fmt.Printf("Writing cpu profile to %s\n", cfg.cpuProfile)
		}
		f, err := os.Create(cfg.cpuProfile)
		if err != nil {
			log.Fatal(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	if cfg.memProfile != "" {
		if !cfg.quiet {
			log.Printf("send SIGUSR1 to %d to dump memory profile", os.Getpid())
		}
		profSig := make(chan os.Signal, 1)
		signal.Notify(profSig, syscall.SIGUSR1)
		go writeMemProfile(cfg.memProfile, profSig)
	}
	if cfg.cpuProfile != "" || cfg.memProfile != "" {
		if !cfg.quiet {
			log.Printf("Note: You must unmount gracefully, otherwise the profile file(s) will stay empty!\n")
		}
	}

	root, err := fs.NewWebRoot(cfg.remote)
	if err != nil {
		log.Fatalln(err)
	}

	sec := time.Second
	opts := &fs.Options{
		// The timeout options are to be compatible with libfuse defaults,
		// making benchmarking easier.
		AttrTimeout:  &sec,
		EntryTimeout: &sec,

		NullPermissions: true, // Leave file permissions on "000" files as-is

		MountOptions: fs.MountOptions{
			AllowOther:        cfg.allowOther,
			Debug:             cfg.debug,
			DirectMount:       cfg.directMount,
			DirectMountStrict: cfg.directMountStrict,
			FsName:            cfg.remote, // First column in "df -T": original dir
			Name:              "nomad",    // Second column in "df -T" will be shown as "fuse." + Name
		},
	}
	if opts.AllowOther {
		// Make the kernel check file permissions for us
		opts.MountOptions.Options = append(opts.MountOptions.Options, "default_permissions")
	}
	if cfg.readOnly {
		opts.MountOptions.Options = append(opts.MountOptions.Options, "ro")
	}
	// Enable diagnostics logging
	if !cfg.quiet {
		opts.Logger = log.New(os.Stderr, "", 0)
	}

	server, err := fs.Mount(cfg.mountPoint, root, opts)
	if err != nil {
		log.Fatalf("Mount fail: %v\n", err)
	}

	if !cfg.quiet {
		log.Println("Mounted!")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		server.Unmount()
	}()

	server.Wait()
}

func writeMemProfile(fn string, sigs <-chan os.Signal) {
	i := 0
	for range sigs {
		fn := fmt.Sprintf("%s-%d.memprof", fn, i)
		i++

		log.Printf("Writing mem profile to %s\n", fn)
		f, err := os.Create(fn)
		if err != nil {
			log.Printf("Create: %v", err)
			continue
		}
		pprof.WriteHeapProfile(f)
		if err := f.Close(); err != nil {
			log.Printf("close %v", err)
		}
	}
}

// mountCmd represents the mount command
var mountCmd = &cobra.Command{
	Use:   "mount",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) != 2 {
			fmt.Printf("usage: %s MOUNTPOINT ORIGINAL\n", filepath.Base(os.Args[0]))
			fmt.Printf("\noptions:\n")
			cmd.Flags().PrintDefaults()
			os.Exit(2)
		}

		var err error
		var c mountConfig

		c.mountPoint = args[0]
		c.remote = args[1]

		c.debug, err = cmd.Flags().GetBool("debug")
		if err != nil {
			log.Fatal(err)
		}
		c.allowOther, err = cmd.Flags().GetBool("allow-other")
		if err != nil {
			log.Fatal(err)
		}
		c.quiet, err = cmd.Flags().GetBool("read-only")
		if err != nil {
			log.Fatal(err)
		}
		c.directMount, err = cmd.Flags().GetBool("direct-mount")
		if err != nil {
			log.Fatal(err)
		}
		c.directMountStrict, err = cmd.Flags().GetBool("direct-mount-strict")
		if err != nil {
			log.Fatal(err)
		}

		c.cpuProfile, _ = cmd.Flags().GetString("cpu-profile")
		if err != nil {
			log.Fatal(err)
		}
		c.memProfile, _ = cmd.Flags().GetString("direct-mount-strict")
		if err != nil {
			log.Fatal(err)
		}

		mount(&c)
	},
}

func init() {
	rootCmd.AddCommand(mountCmd)

	mountCmd.Flags().Bool("debug", false, "Print debugging messages")
	mountCmd.Flags().Bool("allow-other", false, "Mount with -o allowother")
	mountCmd.Flags().BoolP("quiet", "q", false, "Quiet")
	mountCmd.Flags().BoolP("read-only", "r", false, " Mount the file system read-only")
	mountCmd.Flags().Bool("direct-mount", false, "Try to call the mount syscall instead of executing fusermount")
	mountCmd.Flags().Bool("direct-mount-strict", false, "Like direct mount, but don't fall back to fusermount")
	mountCmd.Flags().String("cpu-profile", "", "Write cpu profile to this file")
	mountCmd.Flags().String("mem-profile", "", "Write memory profile to this file")

}
